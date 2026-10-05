p='/home/user/HubOS/.claude/worktrees/agent-a23944a7cde3c4db0/tools/bench/init/hubsim/chaos.go'
s=open(p).read()
s=s.replace('''	case "boot":
		time.Sleep(10 * time.Second)''','''	case "boot":
		if cfg["brain"] != "" {
			bs := time.Now()
			for time.Since(bs) < 90*time.Second {
				if n, _ := brainStatus(); n == 6 {
					break
				}
				time.Sleep(time.Second)
			}
			say("BRAIN-READY after %.1fs from BOOT-OK", time.Since(bs).Seconds())
		}
		time.Sleep(10 * time.Second)''')
open(p,'w').write(s)
