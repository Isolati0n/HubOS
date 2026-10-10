# s6 + s6-rc: PID 1 = s6-svscan, the service graph is a compiled s6-rc database (dependencies, readiness by
# notification-fd, logging pipelines). The database is compiled on the host with s6-rc-compile at build time.
copy_s6
cp "$W"/skel/bin/* "$R/opt/s6/bin/"   # all of the suite: s6-rc and s6-svc -w need several helper programs   # s6-rc needs the fdholder and ipcserver programs too
# s6-rc writes the absolute path of execlineb (here the host build prefix) into the run scripts it generates
mkdir -p "$R$W/skel" "$R/opt/s6/libexec"; ln -s /opt/s6/bin "$R$W/skel/bin"; cp "$W"/skel/libexec/* "$R/opt/s6/libexec/"; ln -s /opt/s6/libexec "$R$W/skel/libexec"
mkdir -p "$R/etc/s6-rc"
"$W/skel/bin/s6-rc-compile" "$R/etc/s6-rc/compiled" "$HERE/candidates/s6rc-par/source"
# parity with s6plain (added 2026-10-10): the image's follow-driftwm wrapper
cp "$HERE/../../../image/machines/hub/rootfs/usr/lib/hubos/follow-driftwm" "$R/bin/follow-driftwm"; chmod +x "$R/bin/follow-driftwm"
