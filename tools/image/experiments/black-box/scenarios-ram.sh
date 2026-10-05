#!/bin/bash
# scenarios-ram.sh NAME...: the RAM backend (ramoops) experiments, docs/proposals/black-box-recorder.md section 4.
# Same helpers as scenarios.sh. WORK must be set.
. "$(dirname "$0")/lib.sh"
BASE="console=ttyS0 loglevel=7"
# 1 MiB reserved by memmap at 256 MiB (guest RAM is 512 MiB; 480 MiB did NOT survive, see scenario ra); records 64 KiB, console 256 KiB, pmsg 64 KiB, software ECC on
MM="memmap=1M\$256M"
RO="ramoops.mem_address=0x10000000 ramoops.mem_size=0x100000 ramoops.record_size=0x10000 ramoops.console_size=0x40000 ramoops.pmsg_size=0x10000 ramoops.ecc=1"

run() {
case "$1" in
r1) # kernel panic, kernel reboots itself (panic=5): warm reset inside the same QEMU process
  newscenario r1; qrun "$BASE panic=5 $MM $RO bb.act=panic bb.end=off"; qwait "second or later boot" 240; echo "seen=$?"; qend 60; show;;
r2) # watchdog reset, no panic, same QEMU process
  newscenario r2; qrun "$BASE panic=0 i6300esb.heartbeat=15 $MM $RO bb.act=wdt bb.end=off"; qwait "second or later boot" 300; echo "seen=$?"; qend 60; show;;
r3) # panic (panic=0: halt), then the QEMU process is killed (SIGKILL) and a NEW QEMU is started with the same vars.fd and disk
  newscenario r3; qrun "$BASE panic=0 $MM $RO bb.act=panic"; qwait "Kernel panic" 240; echo "panic seen=$?"; sleep 3; qkill
  qrun "$BASE panic=0 $MM $RO bb.act=none bb.end=off"; qwait "REPORT-DONE" 240; echo "seen=$?"; qend 20; show;;
r4) # cold boot: after r1 (not erased), a clean power-off and a NEW QEMU. Uses r1's directory.
  D=$WORK/sc-r1; SEQ=2; qrun "$BASE panic=0 $MM $RO bb.act=none bb.end=off"; qwait "REPORT-DONE" 240; echo "seen=$?"; qwait "POWEROFF" 30; qend 30; show;;
r5) # same as r1 but the region comes from reserve_mem (no address on the command line)
  newscenario r5; qrun "$BASE panic=5 reserve_mem=1M:4096:oops ramoops.mem_name=oops ramoops.mem_size=0x100000 ramoops.record_size=0x10000 ramoops.console_size=0x40000 ramoops.pmsg_size=0x10000 ramoops.ecc=1 bb.act=panic bb.end=off"; qwait "second or later boot" 240; echo "seen=$?"; qend 60; show;;
r6) # both backends wanted: ramoops AND efi-pstore at the same time? (r1 command line; look at which one registers)
  newscenario r6; qrun "$BASE panic=5 $MM $RO bb.act=none bb.end=off"; qwait "REPORT-DONE" 240; qwait "POWEROFF" 5; qkill; grep -a -i "pstore\|ramoops" "$LOG" | tr -d '\r' | cut -c1-200;;
r7) # a clean reboot (no crash) inside one QEMU: does the console record of the healthy boot show up on the next boot?
  newscenario r7; qrun "$BASE panic=5 $MM $RO bb.act=reboot bb.end=off"; qwait "second or later boot" 240; echo "seen=$?"; qend 60; show;;
s1) # the save prototype: ramoops crash with warm reset; the next boot copies pstore to the (second, ext4) config disk and prints what /v1/logs would send
  newscenario s1; qrun "$BASE panic=5 $MM $RO bb.act=panic bb.save=1 bb.logs=1 bb.end=off"; qwait "second or later boot" 240; echo "seen=$?"; qend 60
  grep -a "BB:\|BLACKBOX\|pstore" "$LOG" | tr -d '\r' | cut -c1-190 | sed -n '/boot number 2/,$p';;
s2) # cold boot after s1: a NEW QEMU on the same disks. RAM is gone; the saved folder on the config disk is still there.
  D=$WORK/sc-s1; SEQ=2; qrun "$BASE panic=0 $MM $RO bb.act=none bb.save=1 bb.logs=1 bb.end=off"; qwait "second or later boot" 240; qwait "POWEROFF" 30; qend 30
  grep -a "BB:\|BLACKBOX\|pstore" "$LOG" | tr -d '\r' | cut -c1-190 | grep -v "^BB:   L| #\|ELF" | head -60;;
ra) # which memory addresses survive a warm reset? one run per address (RAM is 512 MiB). ADDRS="0x... 0x..." can be set in the environment.
  for A in ${ADDRS:-0x08000000 0x10000000 0x18000000 0x1c000000 0x1e000000}; do
    newscenario ra-$A >/dev/null; MMA="memmap=1M\$$((A))"; MMA="memmap=1M\$${A}"
    RA="ramoops.mem_address=$A ramoops.mem_size=0x100000 ramoops.record_size=0x10000 ramoops.console_size=0x40000 ramoops.pmsg_size=0x10000 ramoops.ecc=1"
    qrun "$BASE panic=5 memmap=1M\$${A} $RA bb.act=panic bb.end=off bb.quiet=1" >/dev/null; qwait "second or later boot" 240; qend 60 >/dev/null
    echo "== address $A: boot 2 sees:"; grep -a "ramoops:\|pstore file(s)" "$LOG" | tr -d '\r' | sed -n '/uncorrectable\|found\|ECC\|pstore file/p' | sort | uniq -c | sort -rn | head -4
  done;;
esac
}
for s in "$@"; do run "$s"; done
