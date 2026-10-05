#!/bin/sh
# blackbox-save.sh: PROTOTYPE of what stage 0 (or the recovery kernel) could do after a failed boot: copy what pstore holds
# into a small, capped folder on the config partition, then empty pstore. BusyBox sh only (no bash, no GNU tools).
# usage: blackbox-save.sh CONFIG_MOUNTPOINT FAILED_BOOTS_SO_FAR
#   CONFIG_MOUNTPOINT must be mounted READ-WRITE (stage 0 today mounts it read-only; see the proposal).
# Limits (all proposals): at most KEEP boot folders (oldest deleted first), at most MAXFILE bytes per file (the END is kept,
# because the end of a log is the part that explains the failure), at most MAXBOOT bytes per folder.
CFG=$1; FAILS=${2:-0}
KEEP=${BB_KEEP:-8}; MAXFILE=${BB_MAXFILE:-65536}; MAXBOOT=${BB_MAXBOOT:-262144}
P=/sys/fs/pstore; D=$CFG/hubos/blackbox
n=$(ls $P 2>/dev/null | wc -l)
[ "$n" -gt 0 ] || { echo "BLACKBOX: pstore is empty, nothing to save"; exit 0; }
mkdir -p "$D" || { echo "BLACKBOX: cannot make $D"; exit 1; }
# the next sequence number (a counter file; the clock is not trusted on a machine that just crashed)
seq=$(cat "$D/seq" 2>/dev/null); case $seq in ''|*[!0-9]*) seq=0;; esac
seq=$((seq+1)); S=$(printf %04d $seq); B=$D/$S; mkdir -p "$B"
total=0
for f in $P/*; do
  [ -f "$f" ] || continue
  sz=$(wc -c < "$f"); [ "$sz" -gt 0 ] || continue
  left=$((MAXBOOT-total)); [ "$left" -gt 0 ] || break
  cap=$MAXFILE; [ "$left" -lt "$cap" ] && cap=$left
  tail -c "$cap" "$f" > "$B/$(basename "$f")"
  total=$((total + $(wc -c < "$B/$(basename "$f")")))
done
{
  echo "sequence=$S"
  echo "failed_boots_before_this_boot=$FAILS"
  echo "kernel=$(uname -r)"
  echo "pstore_backend=$(cat /sys/module/pstore/parameters/backend 2>/dev/null)"
  echo "pstore_files_found=$n"
  echo "bytes_saved=$total"
  echo "saved_at_uptime_seconds=$(cut -d. -f1 /proc/uptime)"
} > "$B/meta.txt"
echo "$seq" > "$D/seq"
# rotate: keep the newest KEEP folders
ls -d "$D"/[0-9][0-9][0-9][0-9] 2>/dev/null | sort | head -n -"$KEEP" 2>/dev/null | while read -r old; do rm -rf "$old"; done
rm -f $P/* 2>/dev/null
sync
echo "BLACKBOX: saved $n pstore file(s), $total bytes, into $B (keeping the newest $KEEP boots)"
