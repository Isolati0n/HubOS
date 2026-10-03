#!/bin/bash
# build-base.sh: the Ubuntu 24.04 root for this machine, from the machine's package list, pinned to a
# dated snapshot of the archive, with SOURCE_DATE_EPOCH set and the systemd pin in place. Output: $WORK/out/base.tar (the
# reproducible artefact) and $WORK/base (it unpacked). Usage: build-base.sh [OUT.tar]
. "$(dirname "$0")/common.sh"
load_machine
OUT=${1:-$WORK/out/base.tar}
STAMP=$(cat "$REPO/$MACHINE" "$REPO/$PACKAGES" "$REPO/image/apt/no-systemd.pref" | sha256sum | cut -d' ' -f1)
if [ -z "$1" ] && [ -z "$FORCE" ] && [ -d "$WORK/base" ] && [ "$(cat "$WORK/out/base.stamp" 2>/dev/null)" = "$STAMP" ]; then
  say "base root is up to date"; exit 0
fi
PK=$(grep -v '^#' "$REPO/$PACKAGES" | grep -v '^$' | paste -sd, -)
say "mmdebstrap: packages $PK, snapshot $SNAPSHOT"
s=$(date +%s)
# The apt pin that forbids the systemd programs (image/apt/no-systemd.pref) is in place before the first install.
mmdebstrap --mode=root --variant=minbase --include="$PK" --components=main,universe \
  --setup-hook='mkdir -p "$1/etc/apt/preferences.d" && cp "'"$REPO"'/image/apt/no-systemd.pref" "$1/etc/apt/preferences.d/hubos-no-systemd"' \
  noble "$OUT" "http://snapshot.ubuntu.com/ubuntu/$SNAPSHOT" >/dev/null
say "base.tar in $(( $(date +%s) - s )) s, sha256 $(sha256sum "$OUT" | cut -d' ' -f1)"
if [ "$OUT" = "$WORK/out/base.tar" ]; then
  rm -rf "$WORK/base"; mkdir -p "$WORK/base"
  tar -C "$WORK/base" --numeric-owner -xpf "$OUT"
  echo "$STAMP" > "$WORK/out/base.stamp"
fi
