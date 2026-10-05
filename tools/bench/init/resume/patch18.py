B='/home/user/HubOS/.claude/worktrees/agent-a23944a7cde3c4db0/tools/bench/init/'
p=B+'hubsim/main.go'
s=open(p).read()
s=s.replace('	case "ctl":\n		ctl(args)\n','	case "ctl":\n		ctl(args)\n	case "nice":\n		niceExec(args)\n')
s=s.replace('//	hubsim ctl ','//	hubsim nice    set the CPU priority (nice value) and exec a program: busybox here has no nice applet\n//	hubsim ctl ')
open(p,'w').write(s)
p=B+'hubsim/tools.go'
s=open(p).read()
s=s.replace('import (\n','import (\n\t"errors"\n',1)
s+='''
// niceExec N PROGRAM ARGS...: setpriority(N), then exec PROGRAM (found in PATH).
func niceExec(args []string) {
	n, err := strconv.Atoi(args[0])
	if err != nil || len(args) < 2 {
		fmt.Println("usage: hubsim nice N PROGRAM ARGS...")
		os.Exit(2)
	}
	syscall.Setpriority(syscall.PRIO_PROCESS, 0, n)
	path, err := exec.LookPath(args[1])
	if err == nil {
		err = syscall.Exec(path, args[1:], os.Environ())
	}
	fmt.Println("nice:", errors.Unwrap(err), err)
	os.Exit(127)
}
'''
open(p,'w').write(s)
p=B+'common/policy'
s=open(p).read()
s=s.replace('exec nice -n -5 "$@"','exec hubsim nice -5 "$@"')
open(p,'w').write(s)
# s6-rc: only the programs it needs
p=B+'candidates/s6rc/build.sh'
s=open(p).read()
s=s.replace('cp "$W"/skel/bin/* "$R/opt/s6/bin/"','for b in s6-fdholder-daemon s6-fdholderd s6-fdholder-store s6-fdholder-delete s6-fdholder-retrieve s6-fdholder-transferdump s6-fdholder-getdump s6-fdholder-setdump s6-fdholder-list s6-ipcserver s6-ipcserverd s6-ipcserver-socketbinder s6-ipcclient s6-sudo s6-sudod s6-sudoc s6-ioconnect s6-ftrig-listen s6-ftrig-listen1; do [ -x "$W/skel/bin/$b" ] && cp "$W/skel/bin/$b" "$R/opt/s6/bin/"; done')
open(p,'w').write(s)
