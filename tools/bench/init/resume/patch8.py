p='/home/user/HubOS/.claude/worktrees/agent-a23944a7cde3c4db0/tools/bench/init/prototypes/go/main.go'
s=open(p).read()
s=s.replace('''		up, _, _ := be.Status(s.Name)
		for _, n := range s.Needs {''','''		up := s.lastUp && !s.started.IsZero()
		for _, n := range s.Needs {''')
open(p,'w').write(s)
