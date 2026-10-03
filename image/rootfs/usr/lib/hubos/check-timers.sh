#!/bin/sh
# check-timers.sh CONFIRM_SECONDS WATCHDOG_SECONDS
# The one rule for the two per-machine timeouts: the watchdog must outlast the confirm timeout by more than a
# margin, or a slow unhealthy trial boot is reset by the watchdog before it can fail cleanly and be rolled back.
# Used by stage 0 at every boot (it is inside the kernel's initramfs) and by the build scripts (tools/image).
# Exit 0 = fine, 1 = refused (a message is printed).
c=$1; w=$2; margin=15
case $c in ''|*[!0-9]*) echo "REFUSED: confirm timeout '$c' is not a whole number of seconds"; exit 1;; esac
case $w in ''|*[!0-9]*) echo "REFUSED: watchdog timeout '$w' is not a whole number of seconds"; exit 1;; esac
if [ "$w" -le $((c + margin)) ]; then
  echo "REFUSED: watchdog timeout ${w}s must be more than the confirm timeout ${c}s plus ${margin}s (more than $((c + margin))s); otherwise the watchdog resets a slow unhealthy trial boot before it can fail cleanly"
  exit 1
fi
exit 0
