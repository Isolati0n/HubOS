import os
R = "/home/user/HubOS/.claude/worktrees/agent-a23944a7cde3c4db0/tools/bench/init/candidates/s6rc"

def w(path, text, exe=True):
    p = os.path.join(R, path)
    os.makedirs(os.path.dirname(p), exist_ok=True)
    open(p, "w").write(text)
    if exe:
        os.chmod(p, 0o755)

w("build.sh", """# s6 + s6-rc: PID 1 = s6-svscan, the service graph is a compiled s6-rc database (dependencies, readiness by
# notification-fd, logging pipelines). The database is compiled on the host with s6-rc-compile at build time.
copy_s6
mkdir -p "$R/etc/s6-rc"
"$W/skel/bin/s6-rc-compile" "$R/etc/s6-rc/compiled" "$HERE/candidates/s6rc/source"
""", False)

w("rootfs/cand/boot.sh", """# s6-svscan is PID 1 with a scan directory that holds only a "stage2" service; stage2 runs s6-rc-init and brings up the graph.
echo "CH CAND s6rc pid1=s6-svscan + s6-rc"
mkdir -p /run/service/.s6-svscan /run/service/stage2
printf '#!/bin/sh\\necho "CH s6-svscan finished; rebooting"\\nreboot -f\\n' > /run/service/.s6-svscan/finish
printf '#!/bin/sh\\necho "CH s6-svscan crashed; rebooting"\\nreboot -f\\n' > /run/service/.s6-svscan/crash
cat > /run/service/stage2/run <<'EOS'
#!/bin/sh
exec 2>&1
[ -d /run/s6-rc ] || s6-rc-init -c /etc/s6-rc/compiled -l /run/s6-rc /run/service
s6-rc -l /run/s6-rc -u change top
echo "CH S6RC-UP up=$(cut -d' ' -f1 /proc/uptime)"
exec sleep 2147483647
EOS
chmod +x /run/service/.s6-svscan/finish /run/service/.s6-svscan/crash /run/service/stage2/run
echo "CH SUP-START up=$(cut -d' ' -f1 /proc/uptime)"
exec s6-svscan /run/service
""", False)

svcs = {
    "seatd": ([], "exec seatd -log 2 -notify 3\n"),
    "udevd": ([], "exec udevd -log 2 -notify 3\n"),
    "dbus": ([], "exec dbus -log 2 -notify 3\n"),
    "driftwm": (["seatd", "udevd"], "echo -900 > /proc/self/oom_score_adj\nexec driftwm --backend udev -mem 100 -notify 3\n"),
    "waybar": (["driftwm", "dbus"], "exec waybar -notify 3\n"),
    "hubd": (["driftwm"], "echo -900 > /proc/self/oom_score_adj\nexec hubd -notify 3\n"),
    "guard": ([], "echo -1000 > /proc/self/oom_score_adj\nexec hubsim guard /dev/watchdog\n"),
}
for name, (deps, body) in svcs.items():
    d = f"source/{name}/"
    w(d + "type", "longrun\n", False)
    w(d + "run", "#!/bin/sh\nexec 2>&1\n" + body)
    if name != "guard":
        w(d + "notification-fd", "3\n", False)
    for dep in deps:
        w(d + f"dependencies.d/{dep}", "", False)
    w(d + "producer-for", f"{name}-log\n", False)
    w(f"source/{name}-log/type", "longrun\n", False)
    w(f"source/{name}-log/consumer-for", f"{name}\n", False)
    w(f"source/{name}-log/run", f"#!/bin/sh\nmkdir -p /var/log/{name}\nexec s6-log -b n5 s100000 /var/log/{name}\n")
w("source/top/type", "bundle\n", False)
w("source/top/contents", "\n".join(svcs) + "\n", False)

w("rootfs/ops/fix", """#!/bin/sh
s6-rc -l /run/s6-rc -u change top 2>/dev/null
s6-svc -u /run/service/$1 2>/dev/null; s6-svc -r /run/service/$1 2>/dev/null
""")
w("rootfs/ops/status", """#!/bin/sh
for s in /run/service/*/; do n=$(basename $s); [ -d $s/supervise ] && echo "$n: $(s6-svstat $s)"; done
""")
w("cmdline", "hub.cand=s6rc hub.sup=s6-supervise\n", False)
