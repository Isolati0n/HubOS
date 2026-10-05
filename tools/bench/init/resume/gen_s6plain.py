import os, stat
R = "/home/user/HubOS/.claude/worktrees/agent-a23944a7cde3c4db0/tools/bench/init/candidates/s6plain"

def w(path, text, exe=True):
    p = os.path.join(R, path)
    os.makedirs(os.path.dirname(p), exist_ok=True)
    open(p, "w").write(text)
    if exe:
        os.chmod(p, 0o755)

w("build.sh", """# s6 plain: what the hub image has today (PID 1 = s6-svscan, one run script per service, follow-driftwm polling script).
copy_s6
cp "$HERE/../../../image/machines/hub/rootfs/usr/lib/hubos/follow-driftwm" "$R/bin/follow-driftwm"; chmod +x "$R/bin/follow-driftwm"
""", False)

w("rootfs/cand/boot.sh", """# s6 plain: copy the service directories to the (RAM) scan directory and become s6-svscan, as image/rootfs/usr/sbin/init does.
echo "CH CAND s6plain pid1=s6-svscan"
cp -a /etc/s6/sv /run/service
mkdir -p /run/service/.s6-svscan
printf '#!/bin/sh\\necho "CH s6-svscan finished; rebooting"\\nreboot -f\\n' > /run/service/.s6-svscan/finish
printf '#!/bin/sh\\necho "CH s6-svscan crashed; rebooting"\\nreboot -f\\n' > /run/service/.s6-svscan/crash
chmod +x /run/service/.s6-svscan/finish /run/service/.s6-svscan/crash
echo "CH SUP-START up=$(cut -d' ' -f1 /proc/uptime)"
exec s6-svscan /run/service
""", False)

def logrun(name):
    w(f"rootfs/etc/s6/sv/{name}/log/run", f"#!/bin/sh\nmkdir -p /var/log/{name}\nexec s6-log -b n5 s100000 /var/log/{name}\n")

def svc(name, body):
    w(f"rootfs/etc/s6/sv/{name}/run", "#!/bin/sh\nexec 2>&1\n" + body)
    if name != "guard":
        logrun(name)

svc("seatd", "exec seatd -log 2\n")
svc("udevd", "exec udevd -log 2\n")
svc("dbus", "exec dbus -log 2\n")
svc("driftwm", "echo -900 > /proc/self/oom_score_adj\nexec driftwm --backend udev -mem 100 -wait /run/seatd.sock,/run/udev/ready\n")
svc("waybar", "exec follow-driftwm waybar -wait /run/dw/wayland-1,/run/dw/bus\n")
svc("hubd", "echo -900 > /proc/self/oom_score_adj\nexec follow-driftwm hubd -wait /run/dw/wayland-1\n")
svc("guard", "echo -1000 > /proc/self/oom_score_adj\nexec hubsim guard /dev/watchdog\n")

w("rootfs/ops/fix", """#!/bin/sh
# what the owner would do by hand: tell s6 to (re)start the service
s6-svc -u /run/service/$1 2>/dev/null; s6-svc -k /run/service/$1 2>/dev/null
""")
w("rootfs/ops/status", """#!/bin/sh
for s in /run/service/*/; do n=$(basename $s); [ -d $s/supervise ] && echo "$n: $(s6-svstat $s)"; done
""")
w("cmdline", "hub.cand=s6plain hub.sup=s6-supervise\n", False)
