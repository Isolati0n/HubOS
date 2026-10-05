B='/home/user/HubOS/.claude/worktrees/agent-a23944a7cde3c4db0/tools/bench/init/'
# ---- chaos.go: trial suite, policy and listen reports ----
p=B+'hubsim/chaos.go'
s=open(p).read()
s=s.replace('''	boot, ok := waitOK(300 * time.Second)''','''	if cfg["suite"] == "trial" {
		trial(cfg["trial"])
		return
	}
	boot, ok := waitOK(300 * time.Second)''')
s=s.replace('''		memReport()''','''		memReport()
		policyReport()
		listenReport()''',1)
s+='''
// policyReport: what resource policy the candidate really applied to the two essential programs.
func policyReport() {
	for _, n := range []string{"driftwm", "hubd", "waybar"} {
		p := pidOf(n)
		cg := strings.TrimSpace(sh(fmt.Sprintf("cat /proc/%d/cgroup", p)))
		st, _ := os.ReadFile(fmt.Sprintf("/proc/%d/stat", p))
		f := strings.Fields(string(st)[strings.LastIndex(string(st), ")")+2:])
		nice := "?"
		if len(f) > 16 {
			nice = f[16]
		}
		say("POLICY %s oom_score_adj=%s nice=%s cgroup=%s", n, strings.TrimSpace(sh(fmt.Sprintf("cat /proc/%d/oom_score_adj", p))), nice, cg)
	}
}

// listenReport: TCP/UDP sockets in LISTEN state on any address and whether epmd runs (the Erlang distribution check).
func listenReport() {
	n := 0
	for _, f := range []string{"/proc/net/tcp", "/proc/net/tcp6"} {
		b, _ := os.ReadFile(f)
		for _, l := range strings.Split(string(b), "\\n")[1:] {
			if fl := strings.Fields(l); len(fl) > 3 && fl[3] == "0A" {
				n++
			}
		}
	}
	say("LISTEN tcp_listening_sockets=%d epmd_running=%v", n, pidOf("epmd") != 0)
}

// trial: the update's trial boot. A confirm step (this function, standing in for it) asks the init's OWN view of
// service health once a second and confirms after 15 good answers in a row, or gives up after 60 s and reboots (the
// rollback, one of the two allowed automatic reboots). The watchdog guard keeps running all the time and knows nothing
// about services. Scenarios: healthy; crash (driftwm cannot start at all); hang (driftwm is wedged 2 s after it first
// worked). "true_state" is what the harness itself sees at the moment of confirmation: a wedged compositor that is
// confirmed anyway is a false confirmation.
func trial(scn string) {
	start := time.Now()
	good := 0
	hung := false
	sawOK := time.Time{}
	for time.Since(start) < 60*time.Second {
		if why() == "" && sawOK.IsZero() {
			sawOK = time.Now()
		}
		if scn == "hang" && !hung && !sawOK.IsZero() && time.Since(sawOK) > 2*time.Second {
			syscall.Kill(pidOf("driftwm"), syscall.SIGSTOP)
			hung = true
			say("TRIAL-INJECT driftwm stopped (wedged)")
		}
		if _, err := os.Stat("/ops/healthy"); err == nil && exec.Command("/bin/sh", "/ops/healthy").Run() == nil {
			good++
		} else {
			good = 0
		}
		if good >= 15 {
			say("TRIAL CONFIRMED scenario=%s after=%.1fs true_state=%q", scn, time.Since(start).Seconds(), why())
			status()
			return
		}
		time.Sleep(time.Second)
	}
	say("TRIAL ROLLBACK scenario=%s after=60s true_state=%q guard_still_feeding=%v", scn, why(), guardState() != '-')
	status()
	time.Sleep(time.Second)
	syscall.Sync()
	syscall.Reboot(syscall.LINUX_REBOOT_CMD_RESTART)
}
'''
open(p,'w').write(s)
# ---- run-qemu.py: --trial / --preflag, stop on TRIAL line ----
p=B+'run-qemu.py'
s=open(p).read()
s=s.replace('ap.add_argument("--smp", type=int, default=2)','ap.add_argument("--smp", type=int, default=2)\nap.add_argument("--trial", default="healthy"); ap.add_argument("--preflag", default="")')
s=s.replace('hub.skip={skip} {extra}"]','hub.skip={skip} hub.trial={a.trial} hub.preflag={a.preflag} {extra}"]')
s=s.replace('''            if b"BOOT-OK" in line:''','''            if a.suite == "trial" and b" TRIAL " in line and (b"CONFIRMED" in line or b"ROLLBACK" in line):
                trial_seen = time.monotonic()
            if a.suite == "trial" and trial_seen and (b"STAGE1" in line or time.monotonic() - trial_seen > 20):
                f.write(b"H# trial finished\\n"); done = True; break
            if b"BOOT-OK" in line:''')
s=s.replace('        boot_ok = 0\n','        boot_ok = 0\n        trial_seen = None\n',1)
open(p,'w').write(s)
