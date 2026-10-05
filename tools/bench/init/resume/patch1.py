p='/home/user/HubOS/.claude/worktrees/agent-a23944a7cde3c4db0/tools/bench/init/hubsim/chaos.go'
s=open(p).read()
a=s.index('// operator: what a person')
b=s.index('type result struct')
new='''// operator: what a person with a shell would do after looking at what is wrong. Only used after the candidate failed
// to recover alone: kill a stopped or silent process (it is restarted by whatever supervises it), restart a client that
// lost its compositor, or run the candidate's own /ops/fix.
func operator(target string) {
	w := why()
	say("operator-action target=%s why=%q", target, w)
	kill := func(n string) {
		if p := pidOf(n); p != 0 {
			syscall.Kill(p, syscall.SIGKILL)
		}
	}
	switch {
	case strings.HasPrefix(w, "driftwm does not answer"):
		kill("driftwm")
	case strings.HasSuffix(w, " stopped"):
		kill(strings.TrimSuffix(w, " stopped"))
	case strings.HasPrefix(w, "clients missing"):
		for _, c := range []string{"waybar", "hubd"} {
			if !strings.Contains(w, c) {
				kill(c)
			}
		}
	case strings.HasPrefix(w, "hubd "):
		kill("hubd")
	}
	if _, err := os.Stat("/ops/fix"); err == nil {
		sh("/bin/sh /ops/fix " + target + " >/dev/console 2>&1")
	}
}

'''
s=s[:a]+new+s[b:]
open(p,'w').write(s)
