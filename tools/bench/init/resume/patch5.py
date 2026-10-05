p='/home/user/HubOS/.claude/worktrees/agent-a23944a7cde3c4db0/tools/bench/init/hubsim/tools.go'
s=open(p).read()
s+='''
// ctl SOCK WORDS...: send one line to a unix socket and print the reply (used by the candidates' /ops scripts).
func ctl(args []string) {
	if len(args) < 2 {
		fmt.Println("usage: hubsim ctl SOCK LINE...")
		os.Exit(2)
	}
	c, err := net.DialTimeout("unix", args[0], 2*time.Second)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	defer c.Close()
	fmt.Fprintln(c, strings.Join(args[1:], " "))
	c.SetReadDeadline(time.Now().Add(5 * time.Second))
	b := make([]byte, 4096)
	for {
		n, err := c.Read(b)
		os.Stdout.Write(b[:n])
		if err != nil {
			return
		}
	}
}
'''
open(p,'w').write(s)
p='/home/user/HubOS/.claude/worktrees/agent-a23944a7cde3c4db0/tools/bench/init/hubsim/main.go'
s=open(p).read()
s=s.replace('	case "crash1":\n		crash1(args)\n','	case "crash1":\n		crash1(args)\n	case "ctl":\n		ctl(args)\n')
s=s.replace('//	hubsim crash1  make PID 1 segfault (ptrace) or hold it stopped\n','//	hubsim crash1  make PID 1 segfault (ptrace) or hold it stopped\n//	hubsim ctl     send one line to a unix socket and print the reply\n')
open(p,'w').write(s)
