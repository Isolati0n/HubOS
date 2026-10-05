p='/home/user/HubOS/.claude/worktrees/agent-a23944a7cde3c4db0/tools/bench/init/hubsim/svc.go'
s=open(p).read()
s=s.replace('''	os.MkdirAll(faultDir, 0o755)
	if f, err := os.OpenFile(faultDir+"/"+name+".starts"''','''	os.MkdirAll(faultDir, 0o755)
	// The real programs are single-instance (one compositor can own the seat and the display, a second udevd, seatd or
	// dbus-daemon refuses to start, hubd cannot bind its socket twice). A second copy of a stand-in exits at once, so an
	// init that starts a service again while the old copy still runs (after its supervisor died) is not rewarded with a
	// harmless duplicate.
	lock, err := os.OpenFile(faultDir+"/"+name+".lock", os.O_CREATE|os.O_RDWR, 0o644)
	if err == nil && syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB) != nil {
		logf(name, "already running, exiting")
		os.Exit(1)
	}
	if f, err := os.OpenFile(faultDir+"/"+name+".starts"''')
open(p,'w').write(s)
