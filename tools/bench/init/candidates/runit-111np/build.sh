# runit 2.1.2 (Ubuntu 24.04 binaries, unpacked with dpkg -x, not installed): PID 1 = runit, stage 2 = runsvdir.
for b in /sbin/runit /usr/bin/runsvdir /usr/bin/runsv /usr/bin/sv /usr/bin/svlogd /usr/bin/chpst; do addbin "$T$b" "$b"; done
cp "$HERE/../../../image/machines/hub/rootfs/usr/lib/hubos/follow-driftwm" "$R/bin/follow-driftwm"; chmod +x "$R/bin/follow-driftwm"
