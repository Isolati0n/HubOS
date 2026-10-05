p='/home/user/HubOS/.claude/worktrees/agent-a23944a7cde3c4db0/tools/bench/init/hubsim/chaos.go'
s=open(p).read()
s=s.replace('''	_, r1 := brainStatus()
	kept := 1''','''	if back < 0 {
		for _, l := range strings.Split(sh("tail -n 25 /var/log/brain/current 2>/dev/null"), "\\n") {
			say("BRAIN-LOG %s", l)
		}
	}
	_, r1 := brainStatus()
	kept := 1''')
open(p,'w').write(s)
