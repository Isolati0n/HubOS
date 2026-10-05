import os, re
B='/home/user/HubOS/.claude/worktrees/agent-a23944a7cde3c4db0/tools/bench/init/'
def sub(path, pairs):
    p=B+path
    s=open(p).read()
    for a,b in pairs:
        if a not in s: print("MISSING in", path, ":", a[:50])
        s=s.replace(a,b)
    open(p,'w').write(s)

dw_old='echo -900 > /proc/self/oom_score_adj\nexec driftwm'
for c,base in (('s6plain','rootfs/etc/s6/sv/'),('runit','rootfs/etc/sv/')):
    sub(f'candidates/{c}/{base}driftwm/run',[(dw_old,'exec policy driftwm')])
    sub(f'candidates/{c}/{base}hubd/run',[('echo -900 > /proc/self/oom_score_adj\nexec follow-driftwm hubd','exec follow-driftwm policy hubd')])
sub('candidates/s6rc/source/driftwm/run',[(dw_old,'exec policy driftwm')])
sub('candidates/s6rc/source/hubd/run',[('echo -900 > /proc/self/oom_score_adj\nexec hubd','exec policy hubd')])
sub('candidates/openrc/rootfs/etc/init.d/driftwm',[("echo -900 > /proc/self/oom_score_adj; exec /bin/driftwm","exec /bin/policy driftwm")])
sub('candidates/openrc/rootfs/etc/init.d/hubd',[("echo -900 > /proc/self/oom_score_adj; exec /bin/hubd","exec /bin/policy hubd")])
# dinit: native properties
for n in ('driftwm','hubd'):
    p=B+f'candidates/dinit/rootfs/etc/dinit.d/{n}'
    s=open(p).read()
    if 'nice' not in s:
        s+='nice = -5\nrun-in-cgroup = /hub\n'
    open(p,'w').write(s)
# init.sh: controllers and the hub cgroup
sub('common/init.sh',[('mount -t cgroup2 none /sys/fs/cgroup 2>/dev/null\n','mount -t cgroup2 none /sys/fs/cgroup 2>/dev/null\necho "+memory +cpu +pids" > /sys/fs/cgroup/cgroup.subtree_control 2>/dev/null\nmkdir -p /sys/fs/cgroup/hub\nfor f in $(sed -n \'s/.*hub\\.preflag=\\([^ ]*\\).*/\\1/p\' /proc/cmdline | tr , " "); do mkdir -p /run/fault; : > /run/fault/$f; done\n')])
# mkrootfs: policy helper
sub('mkrootfs.sh',[('cp "$HERE/common/init.sh" "$R/init"; chmod +x "$R/init"','cp "$HERE/common/init.sh" "$R/init"; cp "$HERE/common/policy" "$R/bin/policy"; chmod +x "$R/init" "$R/bin/policy"')])
# b-go direct backend: cgroup and nice for the two essential programs
sub('prototypes/go/backend.go',[('''		os.WriteFile(fmt.Sprintf("/proc/%d/oom_score_adj", p.pid), []byte("-900"), 0o644)''','''		os.WriteFile(fmt.Sprintf("/proc/%d/oom_score_adj", p.pid), []byte("-900"), 0o644)
		os.MkdirAll("/sys/fs/cgroup/hub", 0o755)
		os.WriteFile("/sys/fs/cgroup/hub/cgroup.procs", []byte(fmt.Sprint(p.pid)), 0o644)
		syscall.Setpriority(syscall.PRIO_PROCESS, p.pid, -5)''')])
