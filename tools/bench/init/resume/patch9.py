p='/home/user/HubOS/.claude/worktrees/agent-a23944a7cde3c4db0/tools/bench/init/hubsim/chaos.go'
s=open(p).read()
s=s.replace('''		operator(target)
		r2, ok2 := waitOK(60 * time.Second)''','''		status()
		for _, l := range strings.Split(sh("tail -n 12 /var/log/brain/current 2>/dev/null"), "\\n") {
			if l != "" {
				say("BRAIN-LOG %s", l)
			}
		}
		operator(target)
		r2, ok2 := waitOK(60 * time.Second)''')
open(p,'w').write(s)
