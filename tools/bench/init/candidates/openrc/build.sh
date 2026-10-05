# OpenRC 0.63.3 built from the Debian source tarball (meson), openrc-init as PID 1 and supervise-daemon as supervisor.
mkdir -p "$R/usr" "$R/etc"
cp -a "$W/openrc-root/." "$R/"
rm -rf "$R/etc/init.d" "$R/etc/runlevels" "$R/etc/conf.d" "$R/etc/local.d" "$R/etc/sysctl.d" "$R/usr/share"   # no stock services: only the six
mkdir -p "$R/etc/init.d"
for b in /sbin/openrc-init /sbin/openrc /sbin/openrc-run /sbin/supervise-daemon /sbin/rc-service /sbin/rc-update /bin/rc-status /sbin/rc-sstat; do [ -e "$W/openrc-root$b" ] && addbin "$W/openrc-root$b" "$b"; done
for l in libcap.so.2; do cp -L "$W/capdev/lib/x86_64-linux-gnu/$l" "$R/lib/" 2>/dev/null || cp -L "$W/capdev/usr/lib/x86_64-linux-gnu/$l" "$R/lib/"; done
mkdir -p "$R/lib/x86_64-linux-gnu"; cp -a "$W/openrc-root/usr/lib/x86_64-linux-gnu/"* "$R/lib/x86_64-linux-gnu/"
copy_s6   # only s6-log, used as the output logger

mkdir -p "$R/etc/runlevels/default" "$R/etc/runlevels/boot" "$R/etc/runlevels/sysinit"
for s in seatd udevd dbus driftwm waybar hubd; do ln -sf /etc/init.d/$s "$R/etc/runlevels/default/$s"; done
