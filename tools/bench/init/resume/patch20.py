B='/home/user/HubOS/.claude/worktrees/agent-a23944a7cde3c4db0/tools/bench/init/'
# ---- Go brain: test hook ----
p=B+'prototypes/go/main.go'
s=open(p).read()
s=s.replace('''			case len(f) == 2 && f[0] == "retry" && byName[f[1]] != nil:''','''			case len(f) == 2 && f[0] == "crashtest": // TEST HOOK: a bug in the policy code. In Go an unrecovered panic in any goroutine ends the whole program.
				go func() { panic("test hook: bug in the policy code") }()
				fmt.Fprintln(c, "ok (panicking)")
			case len(f) == 2 && f[0] == "retry" && byName[f[1]] != nil:''')
open(p,'w').write(s)
# ---- Elixir brain: test hook ----
p=B+'prototypes/elixir/brain.ex'
s=open(p).read()
s=s.replace('''  def handle_call(:retry, _f, s) do''','''  # TEST HOOK: a bug in the policy code of one service's watcher. OTP ends only this process and restarts it.
  def handle_cast(:boom, _s), do: raise("test hook: bug in the policy code")

  def handle_call(:retry, _f, s) do''')
s=s.replace('''        _ -> "error: status | retry NAME\\n"''','''        ["crashtest", n] -> GenServer.cast(Brain.Watcher.via(n), :boom); "ok (raising)\\n"
        _ -> "error: status | retry NAME\\n"''')
open(p,'w').write(s)
# ---- chaos: the bug suite ----
p=B+'hubsim/chaos.go'
s=open(p).read()
s=s.replace('''	case "faults":
		faultSuite(cfgInt("reps", 5))''','''	case "bug":
		syscall.Kill(pidOf("udevd"), syscall.SIGKILL)
		waitOK(60 * time.Second)
		time.Sleep(5 * time.Second)
		for i := 0; i < 5; i++ {
			fPolicyBug()
			between()
		}
	case "faults":
		faultSuite(cfgInt("reps", 5))''')
s+='''
func brainStatus() (lines int, udevdRestarts int) {
	out := sh("hubsim ctl /run/hubos/brain.sock status")
	for _, l := range strings.Split(out, "\\n") {
		if strings.Contains(l, "state=") {
			lines++
		}
		if strings.HasPrefix(l, "udevd ") {
			for _, w := range strings.Fields(l) {
				if strings.HasPrefix(w, "restarts=") {
					udevdRestarts, _ = strconv.Atoi(strings.TrimPrefix(w, "restarts="))
				}
			}
		}
	}
	return
}

// fPolicyBug: a bug in the policy layer's own code (test hook "crashtest" on its status socket). What is lost, how fast does
// the layer answer again, and do the managed services keep running untouched?
func fPolicyBug() result {
	if !begin("fPolicyBug") {
		return result{ok: true}
	}
	old := map[string]int{}
	for _, s := range allSvc {
		old[s] = pidOf(s)
	}
	b0 := pidOf(cfg["brain"])
	_, r0 := brainStatus()
	sh("hubsim ctl /run/hubos/brain.sock crashtest driftwm")
	st := time.Now()
	back := -1.0
	for time.Since(st) < 30*time.Second {
		time.Sleep(300 * time.Millisecond)
		if n, _ := brainStatus(); n == 6 && time.Since(st) > 600*time.Millisecond {
			back = time.Since(st).Seconds()
			break
		}
	}
	_, r1 := brainStatus()
	kept := 1
	for _, s := range allSvc {
		if pidOf(s) != old[s] {
			kept = 0
		}
	}
	return settle("policy-bug", "brain", fmt.Sprintf("brain_answers_again_after=%.1f services_kept_running=%d brain_process_replaced=%d udevd_restart_count_before=%d after=%d", back, kept, b2i(pidOf(cfg["brain"]) != b0), r0, r1))
}
'''
open(p,'w').write(s)
