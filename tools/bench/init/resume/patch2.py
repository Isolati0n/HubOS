D='/home/user/HubOS/.claude/worktrees/agent-a23944a7cde3c4db0/tools/bench/init/prototypes/go/'
s=open(D+'main.go').read()
a=s.index('func tick(now time.Time) {')
b=s.index('func serve(path string)')
new='''func tick(now time.Time) {
	for _, s := range graph {
		needsOK := true
		for _, n := range s.Needs {
			needsOK = needsOK && be.Ready(n)
		}
		up, pid, last := be.Status(s.Name)
		if up && pid != s.pid {
			if s.pid != 0 && s.lastUp && !s.held {
				fail(s, now, "restarted unseen", last)
			}
			s.pid, s.started, s.changed = pid, now, now
		}
		if !needsOK { // a dependency is down: hold this service (its failures are not its own); no polling script needed
			if up {
				logf("holding %s: a dependency is down", s.Name)
				be.Stop(s.Name)
			}
			s.held, s.expectDown, s.lastUp = true, true, up
			continue
		}
		if s.held {
			s.held, s.expectDown, s.lastUp = false, false, up
			if s.L.Step == Running {
				logf("starting %s: its dependencies are ready", s.Name)
				be.Start(s.Name)
			}
			continue
		}
		if !up && s.lastUp && !s.expectDown {
			fail(s, now, last, last)
		}
		s.lastUp = up
		if up && s.Prober != nil && s.Prober.Wedged(now.Sub(s.started)) {
			fail(s, now, "health probe: no answer", "")
			be.Kill(s.Name)
		}
		if s.L.Tick(now) {
			s.expectDown = false
			be.Start(s.Name)
		}
		if !be.Auto() && !up && s.L.Step == Running {
			s.expectDown = false
			be.Start(s.Name)
		}
	}
	// a dependent that is older than its dependency's newest instance is restarted (once per instance), unless it
	// restarted itself in the meantime. This replaces the polling script follow-driftwm.
	for _, s := range graph {
		up, _, _ := be.Status(s.Name)
		for _, n := range s.Needs {
			d := byName[n]
			if up && !s.held && !d.changed.IsZero() && s.started.Before(d.changed) && now.Sub(d.changed) > 3*time.Second && s.syncedTo[n] != d.changed && d.L.Step == Running {
				s.syncedTo[n] = d.changed
				logf("%s restarts because %s was restarted", s.Name, n)
				be.Restart(s.Name)
			}
		}
	}
	writeAlert()
}

'''
s=s[:a]+new+s[b:]
s=s.replace("	expectDown bool\n","	expectDown bool\n	held       bool // stopped because a dependency is down\n")
open(D+'main.go','w').write(s)
b=open(D+'backend.go').read()
b=b.replace('''func (b s6Backend) Ready(name string) bool {
	up, _, _ := b.Status(name)
	return up
}''','''func (b s6Backend) Ready(name string) bool { // "up (pid N) S seconds, ready R seconds" once the notification arrived
	out, err := exec.Command("s6-svstat", b.scan+"/"+name).Output()
	return err == nil && strings.HasPrefix(string(out), "up") && strings.Contains(string(out), ", ready ")
}''')
open(D+'backend.go','w').write(b)
