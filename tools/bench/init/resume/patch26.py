p='/home/user/HubOS/.claude/worktrees/agent-a23944a7cde3c4db0/tools/bench/init/hubsim/chaos.go'
s=open(p).read()
s=s.replace('''	case "bug":
		syscall.Kill(pidOf("udevd"), syscall.SIGKILL)''','''	case "bug":
		bs := time.Now()
		for time.Since(bs) < 90*time.Second { // the policy layer starts after the desktop; wait until it answers
			if n, _ := brainStatus(); n == 6 {
				break
			}
			time.Sleep(time.Second)
		}
		say("BRAIN-READY after %.1fs from BOOT-OK", time.Since(bs).Seconds())
		syscall.Kill(pidOf("udevd"), syscall.SIGKILL)''')
open(p,'w').write(s)
