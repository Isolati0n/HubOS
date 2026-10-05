p='/home/user/HubOS/.claude/worktrees/agent-a23944a7cde3c4db0/tools/bench/init/hubsim/chaos.go'
s=open(p).read()
s=s.replace('''	case "boot":
		time.Sleep(20 * time.Second)''','''	case "boot":
		time.Sleep(10 * time.Second)
		b0, i0 := cpuStat()
		g0 := cpuTicks(supGroup())
		time.Sleep(30 * time.Second)
		b1, i1 := cpuStat()
		say("IDLE-CPU window=30s whole_vm_busy_ticks=%d whole_vm_idle_ticks=%d init_side_ticks=%d (1 tick = 10 ms, %s vCPUs)", b1-b0, i1-i0, cpuTicks(supGroup())-g0, sh("nproc"))
		memReport()''')
s+='''
func cpuStat() (busy, idle int) {
	b, _ := os.ReadFile("/proc/stat")
	f := strings.Fields(strings.SplitN(string(b), "\\n", 2)[0])
	for i, v := range f[1:] {
		n, _ := strconv.Atoi(v)
		if i == 3 || i == 4 { // idle, iowait
			idle += n
		} else {
			busy += n
		}
	}
	return
}

// memReport: resident and proportional memory of PID 1, every supervisor and the policy layer.
func memReport() {
	seen := map[int]bool{}
	for _, p := range supGroup() {
		if seen[p] {
			continue
		}
		seen[p] = true
		pss, rss := 0, 0
		b, _ := os.ReadFile(fmt.Sprintf("/proc/%d/smaps_rollup", p))
		for _, l := range strings.Split(string(b), "\\n") {
			f := strings.Fields(l)
			if len(f) >= 2 && f[0] == "Pss:" {
				pss, _ = strconv.Atoi(f[1])
			}
			if len(f) >= 2 && f[0] == "Rss:" {
				rss, _ = strconv.Atoi(f[1])
			}
		}
		say("MEM pid=%d comm=%s rss_kb=%d pss_kb=%d", p, comm(p), rss, pss)
	}
}
'''
open(p,'w').write(s)
