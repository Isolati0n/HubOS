import os
B = "/home/user/HubOS/.claude/worktrees/agent-a23944a7cde3c4db0/tools/bench/init/candidates/"

def w(cand, path, text, exe=True):
    p = os.path.join(B, cand, path)
    os.makedirs(os.path.dirname(p), exist_ok=True)
    open(p, "w").write(text)
    if exe:
        os.chmod(p, 0o755)

# ---------------- runit ----------------
c = "runit"
w(c, "build.sh", """# runit 2.1.2 (Ubuntu 24.04 binaries, unpacked with dpkg -x, not installed): PID 1 = runit, stage 2 = runsvdir.
for b in /sbin/runit /usr/bin/runsvdir /usr/bin/runsv /usr/bin/sv /usr/bin/svlogd /usr/bin/chpst; do addbin "$T$b" "$b"; done
""", False)
w(c, "rootfs/cand/boot.sh", """echo "CH CAND runit pid1=runit stage2=runsvdir"
cp -a /etc/sv /run/service
touch /run/runit.reboot; chmod +x /run/runit.reboot   # if stage 2 ever returns, stage 3 ends in a reboot (like the s6 image's finish script)
echo "CH SUP-START up=$(cut -d' ' -f1 /proc/uptime)"
exec /sbin/runit
""", False)
w(c, "rootfs/etc/runit/1", "#!/bin/sh\nexit 0\n")
w(c, "rootfs/etc/runit/2", "#!/bin/sh\nexec runsvdir -P /run/service\n")
w(c, "rootfs/etc/runit/3", "#!/bin/sh\necho 'CH runit stage 3 (stage 2 returned or shutdown)'\n")
def rsvc(name, body):
    w(c, f"rootfs/etc/sv/{name}/run", "#!/bin/sh\nexec 2>&1\n" + body)
    if name != "guard":
        w(c, f"rootfs/etc/sv/{name}/log/run", f"#!/bin/sh\nmkdir -p /var/log/{name}\nprintf 's100000\\nn5\\n' > /var/log/{name}/config\nexec svlogd -tt /var/log/{name}\n")
rsvc("seatd", "exec seatd -log 2\n")
rsvc("udevd", "exec udevd -log 2\n")
rsvc("dbus", "exec dbus -log 2\n")
rsvc("driftwm", "echo -900 > /proc/self/oom_score_adj\nexec driftwm --backend udev -mem 100 -wait /run/seatd.sock,/run/udev/ready\n")
rsvc("waybar", "exec follow-driftwm waybar -wait /run/dw/wayland-1,/run/dw/bus\n")
rsvc("hubd", "echo -900 > /proc/self/oom_score_adj\nexec follow-driftwm hubd -wait /run/dw/wayland-1\n")
rsvc("guard", "echo -1000 > /proc/self/oom_score_adj\nexec hubsim guard /dev/watchdog\n")
w(c, "rootfs/ops/fix", "#!/bin/sh\nsv -w 3 force-restart /run/service/$1 2>/dev/null; sv up /run/service/$1\n")
w(c, "rootfs/ops/status", "#!/bin/sh\nfor d in /run/service/*/; do sv status $d; done\n")
w(c, "cmdline", "hub.cand=runit hub.sup=runsv hub.scan=runsvdir\n", False)

# ---------------- dinit ----------------
c = "dinit"
w(c, "build.sh", """# dinit 0.21.0 built from the Alpine source mirror (static, g++), see README.md.
mkdir -p "$R/opt/dinit/bin"; cp "$W/dinit-root/usr/bin/dinit" "$W/dinit-root/usr/bin/dinitctl" "$W/dinit-root/usr/bin/dinit-check" "$R/opt/dinit/bin/"
cp "$HERE/../../../image/machines/hub/rootfs/usr/lib/hubos/follow-driftwm" "$R/bin/follow-driftwm" 2>/dev/null || true
# the graph is checked at build time with dinit-check (host run)
"$W/dinit-root/usr/bin/dinit-check" -d "$HERE/candidates/dinit/rootfs/etc/dinit.d" boot || echo "dinit-check reported a problem"
""", False)
w(c, "rootfs/cand/boot.sh", """echo "CH CAND dinit pid1=dinit"
echo "CH SUP-START up=$(cut -d' ' -f1 /proc/uptime)"
exec /opt/dinit/bin/dinit -d /etc/dinit.d boot
""", False)
def dsvc(name, cmd, deps=(), oom=None, extra=""):
    t = "type = process\ncommand = " + cmd + "\nrestart = true\nlog-type = buffer\n"
    if name != "guard":
        t += "ready-notification = pipefd:3\n"
    for d in deps:
        t += f"depends-on = {d}\n"
    if oom is not None:
        t += f"oom-score-adj = {oom}\n"
    t += extra
    w(c, f"rootfs/etc/dinit.d/{name}", t, False)
dsvc("seatd", "/bin/seatd -log 2 -notify 3")
dsvc("udevd", "/bin/udevd -log 2 -notify 3")
dsvc("dbus", "/bin/dbus -log 2 -notify 3")
dsvc("driftwm", "/bin/driftwm --backend udev -mem 100 -notify 3", ["seatd", "udevd"], -900)
dsvc("waybar", "/bin/waybar -notify 3", ["driftwm", "dbus"])
dsvc("hubd", "/bin/hubd -notify 3", ["driftwm"], -900)
dsvc("guard", "/bin/hubsim guard /dev/watchdog", [], -1000)
w(c, "rootfs/etc/dinit.d/boot", "type = internal\n" + "".join(f"depends-on = {s}\n" for s in ["guard", "seatd", "udevd", "dbus", "driftwm", "waybar", "hubd"]), False)
w(c, "rootfs/ops/fix", "#!/bin/sh\n/opt/dinit/bin/dinitctl start boot 2>/dev/null; /opt/dinit/bin/dinitctl restart $1 2>/dev/null\n")
w(c, "rootfs/ops/status", "#!/bin/sh\n/opt/dinit/bin/dinitctl list\n")
w(c, "cmdline", "hub.cand=dinit\n", False)

# ---------------- openrc ----------------
c = "openrc"
w(c, "build.sh", """# OpenRC 0.63.3 built from the Debian source tarball (meson), openrc-init as PID 1 and supervise-daemon as supervisor.
mkdir -p "$R/usr" "$R/etc"
cp -a "$W/openrc-root/." "$R/"
for b in /sbin/openrc-init /sbin/openrc /sbin/openrc-run /sbin/supervise-daemon /sbin/rc-service /sbin/rc-update /bin/rc-status /sbin/rc-sstat; do [ -e "$W/openrc-root$b" ] && addbin "$W/openrc-root$b" "$b"; done
for l in libcap.so.2; do cp -L "$W/capdev/lib/x86_64-linux-gnu/$l" "$R/lib/" 2>/dev/null || cp -L "$W/capdev/usr/lib/x86_64-linux-gnu/$l" "$R/lib/"; done
mkdir -p "$R/lib/x86_64-linux-gnu"; cp -a "$W/openrc-root/usr/lib/x86_64-linux-gnu/"* "$R/lib/x86_64-linux-gnu/"
copy_s6   # only s6-log, used as the output logger
""", False)
w(c, "rootfs/cand/boot.sh", """echo "CH CAND openrc pid1=openrc-init"
export LD_LIBRARY_PATH=/lib/x86_64-linux-gnu:/lib
# the guard is started here, not by OpenRC: supervise-daemon sets itself as a child subreaper (BELIEVED), which would hide an unreaped orphan from the PID 1 check
echo -1000 > /proc/self/oom_score_adj
/bin/hubsim guard /dev/watchdog &
echo 0 > /proc/self/oom_score_adj
echo "CH SUP-START up=$(cut -d' ' -f1 /proc/uptime)"
exec /sbin/openrc-init
""", False)
w(c, "rootfs/etc/rc.conf", 'rc_parallel="YES"\nrc_logger="NO"\nrc_controller_cgroups="NO"\nrc_cgroup_mode="none"\nrc_depend_strict="YES"\n', False)
def osvc(name, cmd, args, need="", health=False, oom=None):
    t = "#!/sbin/openrc-run\nsupervisor=supervise-daemon\nname=" + name + "\n"
    if oom is not None:
        t += f"command=/bin/sh\ncommand_args=\"-c 'echo {oom} > /proc/self/oom_score_adj; exec {cmd} {args}'\"\n"
    else:
        t += f"command={cmd}\ncommand_args=\"{args}\"\n"
    t += "notify=fd:3\nrespawn_delay=1\nrespawn_max=10\nrespawn_period=60\n"
    t += f"output_logger=\"s6-log -b n5 s100000 /var/log/{name}\"\nerror_logger=\"s6-log -b n5 s100000 /var/log/{name}\"\n"
    if health:
        t += "healthcheck_delay=10\nhealthcheck_timer=2\nhealthcheck() { /bin/hubsim probe /run/dw/wayland-1 >/dev/null; }\nunhealthy() { echo \"driftwm failed its health check\"; }\n"
    t += "depend() {\n\tneed " + need + "\n}\n" if need else "depend() {\n\t:\n}\n"
    w(c, f"rootfs/etc/init.d/{name}", t)
osvc("seatd", "/bin/seatd", "-log 2 -notify 3")
osvc("udevd", "/bin/udevd", "-log 2 -notify 3")
osvc("dbus", "/bin/dbus", "-log 2 -notify 3")
osvc("driftwm", "/bin/driftwm", "--backend udev -mem 100 -notify 3", "seatd udevd", True, -900)
osvc("waybar", "/bin/waybar", "-notify 3", "driftwm dbus")
osvc("hubd", "/bin/hubd", "-notify 3", "driftwm", False, -900)
w(c, "build.sh", open(B + c + "/build.sh").read() + """
mkdir -p "$R/etc/runlevels/default" "$R/etc/runlevels/boot" "$R/etc/runlevels/sysinit"
for s in seatd udevd dbus driftwm waybar hubd; do ln -sf /etc/init.d/$s "$R/etc/runlevels/default/$s"; done
""", False)
w(c, "rootfs/ops/fix", "#!/bin/sh\nrc-service $1 restart 2>/dev/null; openrc default 2>/dev/null\n")
w(c, "rootfs/ops/status", "#!/bin/sh\nrc-status -a 2>&1\n")
w(c, "cmdline", "hub.cand=openrc hub.sup=supervise-daemon\n", False)
