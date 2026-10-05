#!/bin/sh
# Stage 1 of every candidate in the benchmark (BENCH CODE, not for the image). Mounts the basics, loads the watchdog
# driver, starts the fault injector, then hands over to the candidate (/cand/boot.sh ends in an exec of its PID 1).
export PATH=/bin:/sbin:/usr/bin:/usr/sbin:/opt/s6/bin:/opt/dinit/bin
mount -t proc proc /proc
mount -t sysfs sysfs /sys
mount -t devtmpfs devtmpfs /dev
mount -t tmpfs tmpfs /run
mount -t tmpfs -o size=4m tmpfs /var       # stands in for the hub's small writable disk (ENOSPC test)
mount -t cgroup2 none /sys/fs/cgroup 2>/dev/null
exec >/dev/console 2>&1
echo "CH STAGE1 up=$(cut -d' ' -f1 /proc/uptime)"
insmod /lib/modules/i6300esb.ko heartbeat=30 nowayout=0 2>&1
ln -s /run /var/run; mkdir -p /var/log /run/fault /run/dw /run/hubos /run/udev
chmod 777 /run/dw /run/hubos
/bin/hubsim chaos &
. /cand/boot.sh
echo "CH BOOT.SH RETURNED (no exec)"
