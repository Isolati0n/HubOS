p='/home/user/HubOS/.claude/worktrees/agent-a23944a7cde3c4db0/tools/bench/init/hubsim/chaos.go'
s=open(p).read()
s=s.replace('''		exec.Command(os.Args[0], "crash1", "stop").Start()
		time.Sleep(10 * time.Minute)''','''		c := exec.Command(os.Args[0], "crash1", "stop")
		c.Stdout, c.Stderr = os.Stdout, os.Stderr
		c.Start()
		for i := 0; i < 20; i++ {
			time.Sleep(5 * time.Second)
			say("PID1-STATE %c guard=%c watchdog_dev=%v", state(1), guardState(), exists("/dev/watchdog"))
		}''')
s=s.replace('''		exec.Command(os.Args[0], "crash1", "segv").Run()
		time.Sleep(5 * time.Minute)''','''		c := exec.Command(os.Args[0], "crash1", "segv")
		c.Stdout, c.Stderr = os.Stdout, os.Stderr
		c.Run()
		time.Sleep(5 * time.Minute)''')
open(p,'w').write(s)
