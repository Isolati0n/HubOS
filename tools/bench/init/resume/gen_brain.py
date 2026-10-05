import os
B = "/home/user/HubOS/.claude/worktrees/agent-a23944a7cde3c4db0/tools/bench/init/candidates/"

def w(cand, path, text, exe=True):
    p = os.path.join(B, cand, path)
    os.makedirs(os.path.dirname(p), exist_ok=True)
    open(p, "w").write(text)
    if exe:
        os.chmod(p, 0o755)

# ---------------- c-go: s6 + s6-rc + Go brain ----------------
c = "c-go"
w(c, "build.sh", """# architecture C with the Go brain: the s6rc candidate plus one extra s6 service, the brain, started when the graph is up.
. "$HERE/candidates/s6rc/build.sh"
cp -a "$HERE/candidates/s6rc/rootfs/." "$R/"
cp "$W/brain-go" "$R/bin/brain"
""", False)
w(c, "rootfs/cand/boot.sh", """echo "CH CAND c-go pid1=s6-svscan + s6-rc + brain(go)"
mkdir -p /run/service/.s6-svscan /run/service/stage2 /run/service/brain
printf '#!/bin/sh\\necho "CH s6-svscan finished; rebooting"\\nreboot -f\\n' > /run/service/.s6-svscan/finish
printf '#!/bin/sh\\necho "CH s6-svscan crashed; rebooting"\\nreboot -f\\n' > /run/service/.s6-svscan/crash
touch /run/service/brain/down
printf '#!/bin/sh\\nexec 2>&1\\nexec brain -mode s6 -scan /run/service\\n' > /run/service/brain/run
mkdir -p /run/service/brain/log
printf '#!/bin/sh\\nmkdir -p /var/log/brain\\nexec s6-log -b n5 s100000 /var/log/brain\\n' > /run/service/brain/log/run
cat > /run/service/stage2/run <<'EOS'
#!/bin/sh
exec 2>&1
[ -d /run/s6-rc ] || s6-rc-init -c /etc/s6-rc/compiled -l /run/s6-rc /run/service
s6-rc -l /run/s6-rc -u change top
echo "CH S6RC-UP up=$(cut -d' ' -f1 /proc/uptime)"
s6-svc -u /run/service/brain
exec sleep 2147483647
EOS
chmod +x /run/service/.s6-svscan/finish /run/service/.s6-svscan/crash /run/service/stage2/run /run/service/brain/run /run/service/brain/log/run
echo "CH SUP-START up=$(cut -d' ' -f1 /proc/uptime)"
exec s6-svscan /run/service
""", False)
w(c, "rootfs/ops/fix", """#!/bin/sh
brain ctl retry $1 2>/dev/null
s6-rc -l /run/s6-rc -u change top 2>/dev/null
s6-svc -u /run/service/$1 2>/dev/null
""")
w(c, "rootfs/ops/status", "#!/bin/sh\nbrain ctl status\n")
w(c, "cmdline", "hub.cand=c-go hub.sup=s6-supervise hub.brain=brain\n", False)

# ---------------- b-go: s6-svscan as the tiny PID 1, Go supervisor as a normal service ----------------
c = "b-go"
w(c, "build.sh", """# architecture B approximated: s6-svscan is the tiny PID 1 (it only reaps and restarts the supervisor and the guard); the Go
# prototype in "direct" mode starts and watches the six services itself. NOT an init: nothing here is PID 1 except s6-svscan.
copy_s6
cp "$W/brain-go" "$R/bin/brain"
""", False)
w(c, "rootfs/cand/boot.sh", """echo "CH CAND b-go pid1=s6-svscan, supervisor=brain(go, direct)"
mkdir -p /run/service/.s6-svscan /run/service/brain /run/service/guard
printf '#!/bin/sh\\necho "CH s6-svscan finished; rebooting"\\nreboot -f\\n' > /run/service/.s6-svscan/finish
printf '#!/bin/sh\\necho "CH s6-svscan crashed; rebooting"\\nreboot -f\\n' > /run/service/.s6-svscan/crash
printf '#!/bin/sh\\nexec 2>&1\\nexec brain -mode direct\\n' > /run/service/brain/run
printf '#!/bin/sh\\nexec 2>&1\\necho -1000 > /proc/self/oom_score_adj\\nexec hubsim guard /dev/watchdog\\n' > /run/service/guard/run
chmod +x /run/service/.s6-svscan/finish /run/service/.s6-svscan/crash /run/service/brain/run /run/service/guard/run
echo "CH SUP-START up=$(cut -d' ' -f1 /proc/uptime)"
exec s6-svscan /run/service
""", False)
w(c, "rootfs/ops/fix", "#!/bin/sh\nbrain ctl retry $1 2>/dev/null\n")
w(c, "rootfs/ops/status", "#!/bin/sh\nbrain ctl status\n")
w(c, "cmdline", "hub.cand=b-go hub.brain=brain\n", False)
