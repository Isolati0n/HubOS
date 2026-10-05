# s6 plain: what the hub image has today (PID 1 = s6-svscan, one run script per service, follow-driftwm polling script).
copy_s6
cp "$HERE/../../../image/machines/hub/rootfs/usr/lib/hubos/follow-driftwm" "$R/bin/follow-driftwm"; chmod +x "$R/bin/follow-driftwm"
