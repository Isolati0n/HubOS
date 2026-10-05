#!/bin/bash
# scenarios.sh NAME...: run the black box recorder experiments (docs/proposals/black-box-recorder.md section 3).
# WORK must be set (holds tools/root, lab/bzImage from build-lab.sh). Output: the lines the guest printed (BB: ...) plus the QEMU command.
# Each scenario starts QEMU (TCG, no KVM), waits for a line in the serial log, and ends it by PID.
. "$(dirname "$0")/lib.sh"
BASE="console=ttyS0 loglevel=7"
# ramoops: 1 MiB reserved by memmap at 480 MiB (guest RAM is 512 MiB), ecc on
RAMOOPS="memmap=1M\$480M ramoops.mem_address=0x1e000000 ramoops.mem_size=0x100000 ramoops.record_size=0x10000 ramoops.console_size=0x40000 ramoops.pmsg_size=0x10000 ramoops.ecc=1"

run() {
case "$1" in
# ---- efi-pstore (UEFI variables) ----
e1) # kernel panic, the kernel reboots itself (panic=5), efi-pstore backend (default: no ramoops options)
  newscenario e1; qrun "$BASE panic=5 bb.act=panic bb.end=off"; qwait "second or later boot" 240; echo "seen=$?"; qend 60; show;;
e2) # watchdog reset (no panic), efi-pstore backend
  newscenario e2; qrun "$BASE panic=0 i6300esb.heartbeat=15 bb.act=wdt bb.end=off"; qwait "second or later boot" 300; echo "seen=$?"; qend 60; show;;
e3) # panic with panic=0 (halt), then the QEMU process is killed (SIGKILL), then started again with the SAME vars.fd
  newscenario e3; qrun "$BASE panic=0 bb.act=panic"; qwait "Kernel panic" 240; echo "panic seen=$?"; sleep 3; qkill
  qrun "$BASE panic=0 bb.act=none bb.end=off"; qwait "REPORT-DONE" 240; echo "seen=$?"; qend 20; show;;
e4) # cold boot: after e1 (same vars.fd), a clean power-off and a NEW QEMU. Uses e1's directory.
  D=$WORK/sc-e1; SEQ=2; qrun "$BASE panic=0 bb.act=none bb.end=off bb.erase=1"; qwait "REPORT-DONE" 240; echo "seen=$?"; qwait "POWEROFF" 30; qend 30; show;;
e5) # fresh vars.fd: nothing there
  newscenario e5; qrun "$BASE panic=0 bb.act=none bb.end=off"; qwait "REPORT-DONE" 240; echo "seen=$?"; qend 20; show;;
e6) # after e4 erased the pstore files: a new QEMU on the same vars.fd. Are the variables really gone?
  D=$WORK/sc-e1; SEQ=3; qrun "$BASE panic=0 bb.act=none bb.end=off bb.quiet=1"; qwait "second or later" 240; qwait "POWEROFF" 30; qend 30; show;;
e7) # how many 1000-byte variables does OVMF's variable store take? (and can the boot-loop counter still be created afterwards?)
  newscenario e7; qrun "$BASE panic=0 bb.act=fill"; qwait "FILL-DONE" 900; qwait "variable could" 60; echo "seen=$?"; qkill; show | grep "FILL\|variable";;
e8) # a boot loop of 12 panics, efi-pstore, nobody erases. Each boot also tries to create a counter-like variable.
  newscenario e8; qrun "$BASE panic=1 bb.act=panic bb.loop=12 bb.quiet=1 bb.probe=1 bb.end=off"; qwait "second or later" 1500; echo "seen=$?"; qend 60
  grep -a "boot number\|pstore file(s)\|probe\|crash GUID\|Rebooting" "$LOG" | tr -d '\r' | cut -c1-150;;
e9) # a longer boot loop (40 panics) to find where efi-pstore / the variable store stops accepting
  newscenario e9; qrun "$BASE panic=1 bb.act=panic bb.loop=40 bb.quiet=1 bb.probe=1 bb.end=off"; qwait "second or later" 3000; echo "seen=$?"; qend 60
  grep -a "boot number\|pstore file(s)\|probe\|Rebooting" "$LOG" | tr -d '\r' | cut -c1-150 | paste - - - - | cut -c1-200;;
esac
}
for s in "$@"; do run "$s"; done
