#!/bin/bash
# lib.sh: helpers for the black box recorder experiments. Source it after setting WORK (see scenarios.sh).
# Only QEMU 8.2.2 + OVMF from tools/image/fetch-tools.sh. Every QEMU started here is remembered by its PID and only that PID is ever killed.
. "$(dirname "${BASH_SOURCE[0]}")/../../common.sh"
set +e   # common.sh sets -e; the helpers below return non-zero on purpose
LAB=$WORK/lab
CODE=$T/usr/share/OVMF/OVMF_CODE_4M.fd
VARS_TEMPLATE=$T/usr/share/OVMF/OVMF_VARS_4M.fd

# newscenario NAME: a fresh directory with a fresh OVMF variable file, a 1 MiB raw "disk" (boot counter) -> D
newscenario() {
  D=$WORK/sc-$1; rm -rf "$D"; mkdir -p "$D"
  cp "$VARS_TEMPLATE" "$D/vars.fd"; : > "$D/disk.img"; truncate -s 1M "$D/disk.img"; SEQ=0
  # a small ext4 "config partition" for the save prototype (made with the host's mke2fs; it only exists for scenarios that use bb.save=1)
  : > "$D/cfg.img"; truncate -s 16M "$D/cfg.img"; mke2fs -q -t ext4 -F "$D/cfg.img" >/dev/null 2>&1
  echo "### scenario $1: fresh vars.fd ($(stat -c %s "$D/vars.fd") bytes) and fresh disk.img in $D"
}

# qrun APPEND [MEM_MB] [EXTRA QEMU ARGS...]: start QEMU, serial to $D/serial-NN.log; sets QPID and LOG. Does not wait.
qrun() {
  local append=$1 mem=${2:-512}; shift; shift
  SEQ=$((SEQ+1)); LOG=$D/serial-$(printf %02d $SEQ).log
  QARGS=(-L "$T/usr/share/qemu" -L "$T/usr/share/seabios" -machine q35,smm=off -accel tcg -smp 2 -m "$mem" -nographic -display none
    -drive "if=pflash,format=raw,unit=0,readonly=on,file=$CODE" -drive "if=pflash,format=raw,unit=1,file=$D/vars.fd"
    -drive "file=$D/disk.img,if=none,id=d0,format=raw" -device virtio-blk-pci,drive=d0
    -nic none -drive "file=$D/cfg.img,if=none,id=d1,format=raw" -device virtio-blk-pci,drive=d1 -device i6300esb -watchdog-action reset
    -kernel "$LAB/bzImage" -append "$append" -serial "file:$LOG" -monitor none "$@")
  echo "\$ qemu-system-x86_64 ${QARGS[*]}" > "$D/cmd-$(printf %02d $SEQ).txt"
  qemu-system-x86_64 "${QARGS[@]}" </dev/null >"$D/qemu-stdout-$SEQ.txt" 2>&1 &
  QPID=$!
  echo "[run $SEQ] QEMU pid $QPID, serial log $LOG"; echo "$QPID" >> "$WORK/my-qemu-pids.txt"
}

# qwait PATTERN SECONDS: wait until PATTERN (fixed string) shows up in $LOG, or QEMU exits, or the time is up. Returns 0 if seen.
qwait() {
  local i=0
  while [ $i -lt "$2" ]; do
    grep -a -q -F -- "$1" "$LOG" 2>/dev/null && return 0
    kill -0 "$QPID" 2>/dev/null || { grep -a -q -F -- "$1" "$LOG" 2>/dev/null; return $?; }
    sleep 1; i=$((i+1))
  done
  return 1
}

# qkill: kill -9 ONLY the QEMU this script started (by PID), like a power cut / process kill
qkill() { kill -9 "$QPID" 2>/dev/null; wait "$QPID" 2>/dev/null; echo "[run $SEQ] QEMU pid $QPID killed with SIGKILL (exit status $?)"; }
# qend: wait for a QEMU that powers itself off (up to $1 s); kill it by PID if it does not
qend() { local i=0; while kill -0 "$QPID" 2>/dev/null && [ $i -lt "${1:-30}" ]; do sleep 1; i=$((i+1)); done
  if kill -0 "$QPID" 2>/dev/null; then qkill; else wait "$QPID"; echo "[run $SEQ] QEMU pid $QPID exited by itself (status $?)"; fi; }

# show [LOG]: print only the lines of the guest log that matter
show() { grep -a -E "^BB:|pstore|ramoops|persistent|watchdog|i6300|Kernel panic|Rebooting|efi: EFI v|reserve_mem|user-defined physical RAM" "${1:-$LOG}" | tr -d '\r' | cut -c1-200; }
