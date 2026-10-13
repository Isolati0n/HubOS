package main

import (
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"time"
)

// Configuration comes from the kernel command line (so the host chooses it per run):
//
//	hub.cand=NAME   candidate name (only printed)
//	hub.suite=faults|soak|boot       what to do
//	hub.dur=SECONDS soak length
//	hub.seed=N      random seed
//	hub.reps=N      repetitions in the faults suite
//	hub.sup=NAME    process name of the per-service supervisor (matched together with the service name), if any
//	hub.scan=NAME   process name of the scanner/manager that is NOT PID 1, if any
//	hub.brain=NAME  process name of the policy layer, if any
var cfg = map[string]string{}

func loadCfg() {
	b, _ := os.ReadFile("/proc/cmdline")
	for _, f := range strings.Fields(string(b)) {
		if strings.HasPrefix(f, "hub.") {
			kv := strings.SplitN(f[4:], "=", 2)
			if len(kv) == 2 {
				cfg[kv[0]] = kv[1]
			}
		}
	}
}

func cfgInt(k string, d int) int {
	if v, err := strconv.Atoi(cfg[k]); err == nil {
		return v
	}
	return d
}

var t0 = time.Now()
var allSvc = []string{"seatd", "udevd", "dbus", "driftwm", "waybar", "hubd"}

func say(f string, a ...any) {
	fmt.Printf("CH t=%.1f "+f+"\n", append([]any{time.Since(t0).Seconds()}, a...)...)
}

func uptime() float64 {
	b, _ := os.ReadFile("/proc/uptime")
	var u float64
	fmt.Sscan(string(b), &u)
	return u
}

// ---------- process helpers ----------

func procs() []int {
	d, _ := os.ReadDir("/proc")
	var r []int
	for _, e := range d {
		if n, err := strconv.Atoi(e.Name()); err == nil {
			r = append(r, n)
		}
	}
	return r
}

func comm(p int) string {
	b, _ := os.ReadFile(fmt.Sprintf("/proc/%d/comm", p))
	return strings.TrimSpace(string(b))
}

func cmdline(p int) string {
	b, _ := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", p))
	return strings.ReplaceAll(strings.TrimRight(string(b), "\x00"), "\x00", " ")
}

func state(p int) byte {
	b, _ := os.ReadFile(fmt.Sprintf("/proc/%d/stat", p))
	s := string(b)
	i := strings.LastIndex(s, ")")
	if i < 0 || len(s) < i+3 {
		return '?'
	}
	return s[i+2]
}

func pidOf(name string) int {
	for _, p := range procs() {
		if comm(p) == name {
			return p
		}
	}
	return 0
}

func pidsNamed(name string) []int {
	var r []int
	for _, p := range procs() {
		if comm(p) == name {
			r = append(r, p)
		}
	}
	return r
}

// supOf: the per-service supervisor of a service (e.g. "s6-supervise driftwm"), 0 if the candidate has none.
func supOf(svc string) int {
	n := cfg["sup"]
	if n == "" {
		return 0
	}
	for _, p := range procs() {
		if comm(p) == n || strings.HasPrefix(cmdline(p), n+" ") || cmdline(p) == n {
			fl := strings.Fields(cmdline(p))
			if len(fl) < 2 { // the process exited between the listing and the read: empty command line
				continue
			}
			for _, w := range fl[1:] {
				if w == svc || strings.HasSuffix(w, "/"+svc) || strings.HasSuffix(w, "-"+svc) {
					return p
				}
			}
		}
	}
	return 0
}

func supGroup() []int { // PID 1, scanner, all supervisors, brain: the init side of the machine
	g := []int{1}
	for _, n := range []string{cfg["sup"], cfg["scan"], cfg["brain"]} {
		if n != "" {
			// The kernel cuts a process name at 15 characters, so "supervise-daemon" (OpenRC) is "supervise-daemo" in
			// /proc/PID/comm and an exact name match finds nothing; the command line is matched as well (follow-up 2: until then
			// the OpenRC memory numbers held PID 1 only).
			for _, p := range procs() {
				if c := comm(p); c == n || (len(n) > 15 && c == n[:15]) || strings.HasPrefix(cmdline(p), n+" ") || cmdline(p) == n {
					g = append(g, p)
				}
			}
		}
	}
	return g
}

func cpuTicks(pids []int) int {
	t := 0
	for _, p := range pids {
		b, _ := os.ReadFile(fmt.Sprintf("/proc/%d/stat", p))
		s := string(b)
		i := strings.LastIndex(s, ")")
		if i < 0 {
			continue
		}
		f := strings.Fields(s[i+2:])
		if len(f) > 13 {
			u, _ := strconv.Atoi(f[11])
			k, _ := strconv.Atoi(f[12])
			t += u + k
		}
	}
	return t
}

// ---------- the invariant ----------

// why returns "" when the desktop and bar are fully back, else the first thing that is wrong.
func why() string {
	for _, s := range allSvc {
		p := pidOf(s)
		if p == 0 {
			return s + " not running"
		}
		if c := state(p); c == 'T' || c == 't' {
			return s + " stopped"
		}
	}
	if !exists(udevFlag) {
		return "udev not ready"
	}
	out, ok := probe(waySock, "state", time.Second)
	if !ok {
		return "driftwm does not answer (" + out + ")"
	}
	if !strings.Contains(out, "hubd") || !strings.Contains(out, "waybar") {
		return "clients missing: " + out
	}
	if o, ok := probe(hubdSock, "ping", time.Second); !ok {
		return "hubd " + o
	}
	return ""
}

// waitOK waits for two good checks in a row; returns seconds waited and success.
func waitOK(max time.Duration) (float64, bool) {
	st := time.Now()
	good := 0
	for time.Since(st) < max {
		if why() == "" {
			good++
			if good >= 2 {
				return time.Since(st).Seconds() - 0.2, true
			}
		} else {
			good = 0
		}
		time.Sleep(200 * time.Millisecond)
	}
	return max.Seconds(), false
}

func sh(c string) string {
	out, _ := exec.Command("/bin/sh", "-c", c).CombinedOutput()
	return strings.TrimSpace(string(out))
}

// operator: what a person with a shell would do after looking at what is wrong. Only used after the candidate failed
// to recover alone: kill a stopped or silent process (it is restarted by whatever supervises it), restart a client that
// lost its compositor, or run the candidate's own /ops/fix.
func operator(target string) {
	w := why()
	say("operator-action target=%s why=%q", target, w)
	kill := func(n string) {
		if p := pidOf(n); p != 0 {
			killp(p, syscall.SIGKILL)
		}
	}
	switch {
	case strings.HasPrefix(w, "driftwm does not answer"):
		kill("driftwm")
	case strings.HasSuffix(w, " stopped"):
		kill(strings.TrimSuffix(w, " stopped"))
	case strings.HasPrefix(w, "clients missing"):
		for _, c := range []string{"waybar", "hubd"} {
			if !strings.Contains(w, c) {
				kill(c)
			}
		}
	case strings.HasPrefix(w, "hubd "):
		kill("hubd")
	}
	if _, err := os.Stat("/ops/fix"); err == nil {
		sh("/bin/sh /ops/fix " + target + " >/dev/console 2>&1")
	}
}

type result struct {
	rec    float64
	ok     bool
	fixed  bool
	reason string
}

var evn int
var step int

// begin: every fault function calls it first; with hub.skip=N the first N steps are skipped (the host restarts the
// virtual machine after a STUCK state, so the run can go on).
func begin(name string) bool {
	step++
	if step <= cfgInt("skip", 0) {
		return false
	}
	say("STEP %d %s", step, name)
	return true
}

// settle waits for recovery after an injected fault. If the candidate has not recovered by 60 s a person acts.
func settle(fault, target string, extra string) result {
	evn++
	rec, ok := waitOK(60 * time.Second)
	r := result{rec: rec, ok: ok}
	if !ok {
		r.reason = why()
		say("UNRECOVERED ev=%d fault=%s target=%s alert=%d why=%q", evn, fault, target, b2i(exists("/run/hubos/alert")), r.reason)
		status()
		for _, l := range strings.Split(sh("tail -n 12 /var/log/brain/current 2>/dev/null"), "\n") {
			if l != "" {
				say("BRAIN-LOG %s", l)
			}
		}
		operator(target)
		r2, ok2 := waitOK(60 * time.Second)
		r.fixed = ok2
		r.rec = 60 + r2
		if !ok2 {
			say("STUCK ev=%d step=%d fault=%s target=%s why=%q (a reboot is the only way out; the host restarts the machine)", evn, step, fault, target, why())
			time.Sleep(time.Second)
			syscall.Sync()
			syscall.Reboot(syscall.LINUX_REBOOT_CMD_POWER_OFF)
		}
	}
	say("EV ev=%d fault=%s target=%s rec=%.2f ok=%d fixed=%d %s", evn, fault, target, r.rec, b2i(ok), b2i(r.fixed), extra)
	return r
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}

func starts(s string) int {
	st, err := os.Stat(faultDir + "/" + s + ".starts")
	if err != nil {
		return 0
	}
	return int(st.Size())
}

func status() {
	if _, err := os.Stat("/ops/status"); err == nil {
		for _, l := range strings.Split(sh("/bin/sh /ops/status 2>&1 | head -40"), "\n") {
			say("STATUS %s", l)
		}
	}
	if b, err := os.ReadFile("/run/hubos/alert"); err == nil {
		say("ALERT-FILE %q", strings.TrimSpace(string(b)))
	}
}

// ---------- faults ----------

func fKill(s string) result {
	if !begin("fKill") {
		return result{ok: true}
	}
	old := pidOf(s)
	killp(old, syscall.SIGKILL)
	return settle("kill", s, "")
}

func fHang(mode string) result { return fHangAfter(mode, 0) }

// fHangAfter injects a hang of the compositor after a pause (so that the fault falls at a random point of the watcher's
// one-second round) and times it (follow-up 4): fault to kill (the compositor's pid changes or disappears), fault to desktop
// back (the whole invariant holds twice in a row, with waybar and hubd registered again at the new compositor), and whether a
// person was needed (the harness's 60-s rule in settle). The line HANG carries the numbers; the EV line is the old one.
func fHangAfter(mode string, pause time.Duration) result {
	if !begin("fHang") {
		return result{ok: true}
	}
	p := pidOf("driftwm")
	time.Sleep(pause)
	t0 := time.Now()
	killAt := make(chan time.Time, 1)
	go func() {
		for end := time.Now().Add(130 * time.Second); time.Now().Before(end); time.Sleep(50 * time.Millisecond) {
			if q := pidOf("driftwm"); q != p {
				killAt <- time.Now()
				return
			}
		}
		killAt <- time.Time{}
	}()
	switch mode {
	case "stop":
		killp(p, syscall.SIGSTOP)
	case "deadlock":
		os.WriteFile(faultDir+"/driftwm.deadlock", nil, 0o644)
	default:
		os.WriteFile(faultDir+"/driftwm.spin", nil, 0o644)
	}
	// The busy-loop and deadlock faults are armed by the stand-in's heartbeat tick (every 500 ms): it consumes the fault file
	// and then stops answering. Until then the desktop still answers and settle would return at once (follow-up 3 had such
	// 0.11-s "recoveries", steps 22 of s6rc-par-wd and 20 of runit-wd: the fault was not yet in effect). Wait for the fault to
	// take effect; the times below count from that moment, and the arming delay is printed.
	arm := 0.0
	if mode != "stop" {
		for exists(faultDir+"/driftwm.spin") || exists(faultDir+"/driftwm.deadlock") {
			time.Sleep(10 * time.Millisecond)
			if time.Since(t0) > 5*time.Second {
				break
			}
		}
		arm = time.Since(t0).Seconds()
		t0 = time.Now()
	}
	wst := ""
	r := settle("hang-"+mode, "driftwm", "")
	os.Remove(faultDir + "/driftwm.spin")
	os.Remove(faultDir + "/driftwm.deadlock")
	k := -1.0
	select {
	case at := <-killAt:
		if !at.IsZero() {
			k = at.Sub(t0).Seconds()
		}
	default:
	}
	wst = watcherFiles()
	note := ""
	if !r.ok {
		note = "person=1"
	}
	say("HANG n=%d mode=%s arm_delay=%.2f old_pid=%d new_pid=%d fault_to_pidchange=%.2f fault_to_desktop=%.2f windows_back=%d needed_person=%d %s", evn, mode, arm, p, pidOf("driftwm"), k, r.rec, b2i(r.ok || r.fixed), b2i(!r.ok), wst+" "+note)
	return r
}

// watcherFiles: the kill budget and the state file of the test watcher (follow-up 4, item 2), for the log.
func watcherFiles() string {
	st, _ := os.ReadFile("/run/hubos/watcher.state")
	bg, _ := os.ReadFile("/run/hubos/watcher.budget")
	return fmt.Sprintf("watcher_state=%q budget_entries=%d latch=%d", strings.TrimSpace(string(st)), strings.Count(string(bg), "kill "), b2i(exists("/run/hubos/watcher.latch")))
}

func fStorm(s string, dur time.Duration) result {
	if !begin("fStorm") {
		return result{ok: true}
	}
	g := supGroup()
	c0, s0 := cpuTicks(g), starts(s)
	os.WriteFile(faultDir+"/"+s+".crash", nil, 0o644)
	killp(pidOf(s), syscall.SIGKILL)
	time.Sleep(dur - 10*time.Second)
	m := starts(s)
	time.Sleep(10 * time.Second)
	n0 := starts(s)
	c1 := cpuTicks(g)
	other := "bar-up"
	if s != "waybar" && pidOf("waybar") == 0 {
		other = "bar-down"
	}
	status()
	os.Remove(faultDir + "/" + s + ".crash")
	say("STORM target=%s dur=%.0f starts=%d starts_last10s=%d cpu_ticks_init_side=%d %s", s, dur.Seconds(), n0-s0, n0-m, c1-c0, other)
	return settle("storm", s, fmt.Sprintf("starts=%d last10=%d cpu=%d %s", n0-s0, n0-m, c1-c0, other))
}

func fDep(mode string) result {
	if !begin("fDep") {
		return result{ok: true}
	}
	st := map[string]int{}
	for _, s := range []string{"driftwm", "waybar", "hubd", "seatd"} {
		st[s] = starts(s)
	}
	if mode == "slow" {
		os.WriteFile(faultDir+"/seatd.slow", []byte("15"), 0o644)
	} else {
		os.WriteFile(faultDir+"/seatd.crash", nil, 0o644)
	}
	killp(pidOf("seatd"), syscall.SIGKILL)
	if mode == "fail" {
		time.Sleep(20 * time.Second)
		os.Remove(faultDir + "/seatd.crash")
	}
	r := settle("dep-"+mode, "seatd", "")
	os.Remove(faultDir + "/seatd.slow")
	os.Remove(faultDir + "/seatd.crash")
	say("DEPSTARTS mode=%s seatd=%d driftwm=%d waybar=%d hubd=%d", mode, starts("seatd")-st["seatd"], starts("driftwm")-st["driftwm"], starts("waybar")-st["waybar"], starts("hubd")-st["hubd"])
	return r
}

func fDisk(dur time.Duration) result {
	if !begin("fDisk") {
		return result{ok: true}
	}
	f, err := os.Create("/var/ballast")
	if err != nil {
		say("disk: %v", err)
		return result{}
	}
	buf := make([]byte, 65536)
	for {
		if _, err := f.Write(buf); err != nil {
			break
		}
	}
	f.Sync()
	say("DISK-FULL df=%s", sh("df /var | tail -1"))
	okSamples, n := 0, 0
	end := time.Now().Add(dur)
	killed := false
	for time.Now().Before(end) {
		if !killed && time.Until(end) < dur-5*time.Second {
			killp(pidOf("waybar"), syscall.SIGKILL) // a restart while the disk is full
			killed = true
		}
		n++
		if why() == "" {
			okSamples++
		}
		time.Sleep(time.Second)
	}
	status()
	alive := true
	for _, s := range allSvc {
		if pidOf(s) == 0 {
			alive = false
		}
	}
	f.Close()
	os.Remove("/var/ballast")
	r := settle("disk-full", "waybar", fmt.Sprintf("ok_during=%d/%d all_alive_at_end=%d", okSamples, n, b2i(alive)))
	return r
}

func fSupKill(s string) result {
	if !begin("fSupKill") {
		return result{ok: true}
	}
	sp := supOf(s)
	if sp == 0 {
		say("skip supkill %s: no per-service supervisor", s)
		return result{ok: true}
	}
	old := pidOf(s)
	killp(sp, syscall.SIGKILL)
	time.Sleep(1500 * time.Millisecond)
	kept := pidOf(s) == old
	r1 := settle("supervisor-killed", s, fmt.Sprintf("child_kept_running=%d", b2i(kept)))
	time.Sleep(2 * time.Second)
	back := supOf(s) != 0
	// does anybody notice the service now? kill it and see
	killp(pidOf(s), syscall.SIGKILL)
	r2 := settle("after-supervisor-killed", s, fmt.Sprintf("supervisor_back=%d", b2i(back)))
	_ = r1
	return r2
}

func fScanKill() result {
	if !begin("fScanKill") {
		return result{ok: true}
	}
	n := cfg["scan"]
	if n == "" || pidOf(n) == 0 {
		return result{ok: true}
	}
	old := pidOf("driftwm")
	for _, p := range pidsNamed(n) {
		killp(p, syscall.SIGKILL)
	}
	time.Sleep(1500 * time.Millisecond)
	kept := pidOf("driftwm") == old
	back := pidOf(n) != 0
	r := settle("scanner-killed", n, fmt.Sprintf("child_kept_running=%d scanner_back=%d", b2i(kept), b2i(back)))
	killp(pidOf("hubd"), syscall.SIGKILL)
	settle("after-scanner-killed", "hubd", "")
	return r
}

func fBrainKill() result {
	if !begin("fBrainKill") {
		return result{ok: true}
	}
	n := cfg["brain"]
	if n == "" || pidOf(n) == 0 {
		return result{ok: true}
	}
	old := pidOf("driftwm")
	oldBar := pidOf("waybar")
	st := time.Now()
	for _, p := range pidsNamed(n) {
		killp(p, syscall.SIGKILL)
	}
	back := 0.0
	for time.Since(st) < 30*time.Second {
		if pidOf(n) != 0 {
			back = time.Since(st).Seconds()
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	time.Sleep(2 * time.Second)
	kept := pidOf("driftwm") == old && pidOf("waybar") == oldBar
	return settle("brain-killed", n, fmt.Sprintf("services_kept_running=%d brain_back_after=%.1f", b2i(kept), back))
}

func fClock(delta int64) result {
	if !begin("fClock") {
		return result{ok: true}
	}
	old := pidOf("driftwm")
	killp(old, syscall.SIGKILL)
	setClock(delta)
	r := settle(fmt.Sprintf("clock%+d", delta), "driftwm", "")
	setClock(-delta)
	time.Sleep(time.Second)
	// after the clock is put back: is the supervisor still sane?
	killp(pidOf("hubd"), syscall.SIGKILL)
	r2 := settle(fmt.Sprintf("after-clock%+d", delta), "hubd", "")
	_ = r2
	return r
}

func setClock(delta int64) {
	var tv syscall.Timeval
	syscall.Gettimeofday(&tv)
	tv.Sec += delta
	syscall.Settimeofday(&tv)
}

func fOOM() result {
	if !begin("fOOM") {
		return result{ok: true}
	}
	say("OOMSCORE driftwm=%s hubd=%s waybar=%s", sh("cat /proc/"+strconv.Itoa(pidOf("driftwm"))+"/oom_score_adj"), sh("cat /proc/"+strconv.Itoa(pidOf("hubd"))+"/oom_score_adj"), sh("cat /proc/"+strconv.Itoa(pidOf("waybar"))+"/oom_score_adj"))
	before := map[string]int{}
	for _, s := range allSvc {
		before[s] = pidOf(s)
	}
	pid1 := state(1)
	cmd := exec.Command(os.Args[0], "hog")
	cmd.Start()
	end := time.Now().Add(90 * time.Second)
	died := false
	var ws syscall.WaitStatus
	for time.Now().Before(end) {
		if w, _ := syscall.Wait4(cmd.Process.Pid, &ws, syscall.WNOHANG, nil); w == cmd.Process.Pid {
			died = true
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	if !died {
		cmd.Process.Kill()
		cmd.Wait()
	}
	var lost []string
	for _, s := range allSvc {
		if p := pidOf(s); p != before[s] {
			lost = append(lost, s)
		}
	}
	return settle("oom", "hog", fmt.Sprintf("hog_killed=%d services_lost=%v pid1_state=%c", b2i(died), lost, pid1))
}

// ---------- runs ----------

func sampler() {
	for {
		z, n := 0, 0
		for _, p := range procs() {
			n++
			if state(p) == 'Z' {
				z++
			}
		}
		g := supGroup()
		fd, rss := 0, 0
		for _, p := range g {
			d, _ := os.ReadDir(fmt.Sprintf("/proc/%d/fd", p))
			fd += len(d)
			b, _ := os.ReadFile(fmt.Sprintf("/proc/%d/statm", p))
			f := strings.Fields(string(b))
			if len(f) > 1 {
				v, _ := strconv.Atoi(f[1])
				rss += v * 4
			}
		}
		var avail int
		mb, _ := os.ReadFile("/proc/meminfo")
		for _, l := range strings.Split(string(mb), "\n") {
			if strings.HasPrefix(l, "MemAvailable:") {
				fmt.Sscan(strings.TrimPrefix(l, "MemAvailable:"), &avail)
			}
		}
		say("SAMPLE zombies=%d nproc=%d init_side_fds=%d init_side_rss_kb=%d memavail_kb=%d init_side_cpu_ticks=%d", z, n, fd, rss, avail, cpuTicks(g))
		time.Sleep(10 * time.Second)
	}
}

func chaos(args []string) {
	loadCfg()
	os.WriteFile("/proc/self/oom_score_adj", []byte("-1000"), 0o644)
	os.MkdirAll(faultDir, 0o755)
	seed := int64(cfgInt("seed", 1))
	rng := rand.New(rand.NewSource(seed))
	say("CHAOS start cand=%s suite=%s seed=%d cpus=%s", cfg["cand"], cfg["suite"], seed, sh("nproc"))
	if cfg["suite"] == "trial" {
		trial(cfg["trial"])
		return
	}
	boot, ok := waitOK(300 * time.Second)
	say("BOOT-OK up=%.2f ok=%d (seconds since kernel start; waited %.1f)", uptime(), b2i(ok), boot)
	if !ok {
		say("BOOT-FAILED why=%q", why())
		status()
		return
	}
	go sampler()
	time.Sleep(5 * time.Second)
	switch cfg["suite"] {
	case "boot":
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
		time.Sleep(10 * time.Second)
		b0, i0 := cpuStat()
		g0 := cpuTicks(supGroup())
		time.Sleep(30 * time.Second)
		b1, i1 := cpuStat()
		say("IDLE-CPU window=30s whole_vm_busy_ticks=%d whole_vm_idle_ticks=%d init_side_ticks=%d (1 tick = 10 ms, %s vCPUs)", b1-b0, i1-i0, cpuTicks(supGroup())-g0, sh("nproc"))
		memReport()
		policyReport()
		listenReport()
	case "pid1crash":
		say("INJECT pid1-segv")
		c := exec.Command(os.Args[0], "crash1", "segv")
		c.Stdout, c.Stderr = os.Stdout, os.Stderr
		c.Run()
		time.Sleep(5 * time.Minute)
	case "pid1wedge":
		say("INJECT pid1-wedge (held stopped by ptrace)")
		c := exec.Command(os.Args[0], "crash1", "stop")
		c.Stdout, c.Stderr = os.Stdout, os.Stderr
		c.Start()
		for i := 0; i < 20; i++ {
			time.Sleep(5 * time.Second)
			say("PID1-STATE %c guard=%c watchdog_dev=%v", state(1), guardState(), exists("/dev/watchdog"))
		}
	case "bug":
		bs := time.Now()
		for time.Since(bs) < 90*time.Second { // the policy layer starts after the desktop; wait until it answers
			if n, _ := brainStatus(); n == 6 {
				break
			}
			time.Sleep(time.Second)
		}
		say("BRAIN-READY after %.1fs from BOOT-OK", time.Since(bs).Seconds())
		killp(pidOf("udevd"), syscall.SIGKILL)
		waitOK(60 * time.Second)
		time.Sleep(5 * time.Second)
		for i := 0; i < 5; i++ {
			fPolicyBug()
			between()
		}
	case "faults":
		faultSuite(cfgInt("reps", 5))
	case "hangs":
		hangSuite(rng, cfgInt("reps", 9))
	case "budget":
		budgetSuite()
	case "latch":
		latchSuite(cfgInt("hold", 960))
	case "latchc":
		latchCSuite()
	case "hangsP":
		hangSuiteP(rng, cfgInt("reps", 9))
	case "storm3":
		storm3Suite(cfgInt("reps", 3))
	case "slow":
		slowSuite()
	case "lockprobe":
		lockProbeSuite(cfgInt("reps", 3))
	case "oom":
		for i := 0; i < cfgInt("reps", 3); i++ {
			fOOM()
			between()
		}
	case "storm":
		for i := 0; i < cfgInt("reps", 3); i++ {
			fStorm("driftwm", 40*time.Second)
			between()
		}
	case "soak":
		soak(rng, time.Duration(cfgInt("dur", 600))*time.Second)
	}
	say("SUITE-DONE")
	time.Sleep(2 * time.Second)
	syscall.Sync()
	syscall.Reboot(syscall.LINUX_REBOOT_CMD_POWER_OFF)
}

func between() { time.Sleep(3 * time.Second) }

// hangSuite (follow-up 4, item 3): n hangs of the compositor, in shuffled groups of three shapes (SIGSTOP, busy loop, deadlock),
// each after a random pause of up to one second (so the fault falls at a random point of the watcher's round). The kill budget
// (3 in 10 minutes) is respected on purpose: before the first hang of each further group of three the harness waits until
// 10 minutes and 15 seconds after the first fault of the previous group, so every hang finds a full budget. (The capped case is
// the "budget" suite.)
func hangSuite(rng *rand.Rand, n int) {
	modes := []string{"stop", "spin", "deadlock"}
	var groupStart float64
	for i := 0; i < n; i++ {
		if i%3 == 0 {
			rng.Shuffle(len(modes), func(a, b int) { modes[a], modes[b] = modes[b], modes[a] })
			if i > 0 {
				for uptime() < groupStart+615 {
					fmt.Printf("CH t=%.1f waiting for the kill budget (group %d)\n", time.Since(t0).Seconds(), i/3)
					time.Sleep(30 * time.Second)
				}
			}
			groupStart = uptime()
		}
		pause := time.Duration(rng.Intn(1000)) * time.Millisecond
		fHangAfter(modes[i%3], pause)
		between()
	}
}

func faultSuite(reps int) {
	for i := 0; i < reps; i++ {
		for _, s := range allSvc {
			fKill(s)
			between()
		}
	}
	for i := 0; i < 3; i++ {
		fHang("stop")
		between()
		fHang("spin")
		between()
	}
	fStorm("hubd", 40*time.Second)
	between()
	fStorm("driftwm", 40*time.Second)
	between()
	fDep("slow")
	between()
	fDep("fail")
	between()
	fDisk(20 * time.Second)
	between()
	fDisk(20 * time.Second)
	between()
	for i := 0; i < 3; i++ {
		fSupKill("driftwm")
		between()
		fSupKill("waybar")
		between()
	}
	fScanKill()
	between()
	fBrainKill()
	between()
	fBrainKill()
	between()
	fClock(-3600)
	between()
	fClock(86400)
	between()
	fOOM()
	between()
	fOOM()
	if cfg["watcher"] != "" { // follow-up 3, item 3: the test watcher is one more service; the init must restart it when it is killed
		between()
		fWatcherKill()
		between()
		fWatcherKill()
	}
}

// fWatcherKill: kill the test watcher itself (SIGKILL). Recovery = a new watcher process exists. If it does not come back
// within 60 s a person acts (the candidate's ops/fix), as for every other fault.
func fWatcherKill() result {
	if !begin("fWatcherKill") {
		return result{ok: true}
	}
	n := cfg["watcher"]
	old := pidOf(n)
	if old == 0 {
		say("skip watcher kill: %s is not running", n)
		return result{ok: true}
	}
	evn++
	st := time.Now()
	killp(old, syscall.SIGKILL)
	wait := func(max time.Duration) (float64, bool) {
		for time.Since(st) < max {
			time.Sleep(200 * time.Millisecond)
			if p := pidOf(n); p != 0 && p != old {
				return time.Since(st).Seconds(), true
			}
		}
		return time.Since(st).Seconds(), false
	}
	rec, ok := wait(60 * time.Second)
	r := result{rec: rec, ok: ok}
	if !ok {
		say("UNRECOVERED ev=%d fault=watcher-killed target=%s alert=%d why=%q", evn, n, b2i(exists("/run/hubos/alert")), n+" not running")
		status()
		operator(n)
		rec2, ok2 := wait(120 * time.Second)
		r.fixed, r.rec = ok2, rec2
		if !ok2 {
			say("STUCK ev=%d step=%d fault=watcher-killed target=%s why=%q (a reboot is the only way out; the host restarts the machine)", evn, step, n, n+" not running")
			time.Sleep(time.Second)
			syscall.Sync()
			syscall.Reboot(syscall.LINUX_REBOOT_CMD_POWER_OFF)
		}
	}
	say("EV ev=%d fault=watcher-killed target=%s rec=%.2f ok=%d fixed=%d old_pid=%d new_pid=%d", evn, n, r.rec, b2i(ok), b2i(r.fixed), old, pidOf(n))
	return r
}

func soak(rng *rand.Rand, dur time.Duration) {
	end := time.Now().Add(dur)
	pick := func() {
		s := allSvc[rng.Intn(len(allSvc))]
		r := rng.Intn(100)
		switch {
		case r < 50:
			fKill(s)
		case r < 56:
			fHang("stop")
		case r < 60:
			fHang("spin")
		case r < 65:
			t := []string{"hubd", "waybar", "driftwm"}[rng.Intn(3)]
			fStorm(t, 25*time.Second)
		case r < 69:
			fDep("slow")
		case r < 72:
			fDep("fail")
		case r < 76:
			fDisk(15 * time.Second)
		case r < 86:
			fSupKill([]string{"driftwm", "waybar", "hubd", "seatd"}[rng.Intn(4)])
		case r < 88:
			fScanKill()
		case r < 92:
			fBrainKill()
		case r < 95:
			d := []int64{-3600, 86400}[rng.Intn(2)]
			fClock(d)
		default:
			fOOM()
		}
	}
	// Memory and restart bookkeeping for the soak: every 60 s the PSS of PID 1, the scanner, every supervisor and the
	// policy layer (smaps_rollup), and the number of service restarts seen so far (a service whose PID changed).
	var restarts, faults int64
	go func() {
		last := map[string]int{}
		for {
			for _, n := range allSvc {
				p := pidOf(n)
				if p > 0 && last[n] > 0 && p != last[n] {
					atomic.AddInt64(&restarts, 1)
				}
				if p > 0 {
					last[n] = p
				}
			}
			time.Sleep(time.Second)
		}
	}()
	go func() {
		for t0 := time.Now(); ; time.Sleep(60 * time.Second) {
			say("SOAKMEM t=%.0f restarts=%d faults=%d", time.Since(t0).Seconds(), atomic.LoadInt64(&restarts), atomic.LoadInt64(&faults))
			if cfg["brain"] == "brain" && cfg["gcstats"] != "" { // item 8: the brain's own counters after a forced collection, BEFORE the PSS is read
				say("BRAINSTAT %s", strings.TrimSpace(sh("hubsim ctl /run/hubos/brain.sock gcstats")))
			}
			memReport()
		}
	}()
	for time.Now().Before(end) {
		pick()
		atomic.AddInt64(&faults, 1)
		time.Sleep(time.Duration(5+rng.Intn(15)) * time.Second)
	}
	say("SOAK-END restarts=%d faults=%d", atomic.LoadInt64(&restarts), atomic.LoadInt64(&faults))
}

func guardState() byte {
	for _, p := range procs() {
		if comm(p) == "hubsim" && strings.Contains(cmdline(p), "guard") && !strings.Contains(cmdline(p), "zombie") {
			return state(p)
		}
	}
	return '-'
}

func cpuStat() (busy, idle int) {
	b, _ := os.ReadFile("/proc/stat")
	f := strings.Fields(strings.SplitN(string(b), "\n", 2)[0])
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
		for _, l := range strings.Split(string(b), "\n") {
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

// policyReport: what resource policy the candidate really applied to the two essential programs.
func policyReport() {
	for _, n := range []string{"driftwm", "hubd", "waybar"} {
		p := pidOf(n)
		cg := strings.TrimSpace(sh(fmt.Sprintf("cat /proc/%d/cgroup", p)))
		st, _ := os.ReadFile(fmt.Sprintf("/proc/%d/stat", p))
		f := strings.Fields(string(st)[strings.LastIndex(string(st), ")")+2:])
		nice := "?"
		if len(f) > 16 {
			nice = f[16]
		}
		say("POLICY %s oom_score_adj=%s nice=%s cgroup=%s", n, strings.TrimSpace(sh(fmt.Sprintf("cat /proc/%d/oom_score_adj", p))), nice, cg)
	}
}

// listenReport: TCP/UDP sockets in LISTEN state on any address and whether epmd runs (the Erlang distribution check).
func listenReport() {
	n := 0
	for _, f := range []string{"/proc/net/tcp", "/proc/net/tcp6"} {
		b, _ := os.ReadFile(f)
		for _, l := range strings.Split(string(b), "\n")[1:] {
			if fl := strings.Fields(l); len(fl) > 3 && fl[3] == "0A" {
				n++
			}
		}
	}
	say("LISTEN tcp_listening_sockets=%d epmd_running=%v", n, pidOf("epmd") != 0)
}

// trial: the update's trial boot. A confirm step (this function, standing in for it) asks the init's OWN view of
// service health once a second and confirms after 15 good answers in a row, or gives up after 60 s and reboots (the
// rollback, one of the two allowed automatic reboots). The watchdog guard keeps running all the time and knows nothing
// about services. Scenarios: healthy; crash (driftwm cannot start at all); hang (driftwm is wedged 2 s after it first
// worked). "true_state" is what the harness itself sees at the moment of confirmation: a wedged compositor that is
// confirmed anyway is a false confirmation.
func trial(scn string) {
	start := time.Now()
	good := 0
	iters, slowest := 0, time.Duration(0)
	hung := false
	sawOK := time.Time{}
	for time.Since(start) < 60*time.Second {
		if why() == "" && sawOK.IsZero() {
			sawOK = time.Now()
		}
		if scn == "hang" && !hung && !sawOK.IsZero() && time.Since(sawOK) > 2*time.Second {
			killp(pidOf("driftwm"), syscall.SIGSTOP)
			hung = true
			say("TRIAL-INJECT driftwm stopped (wedged)")
		}
		t0 := time.Now()
		if _, err := os.Stat("/ops/healthy"); err == nil && exec.Command("/bin/sh", "/ops/healthy").Run() == nil {
			good++
		} else {
			good = 0
		}
		iters++
		if d := time.Since(t0); d > slowest {
			slowest = d
		}
		if good >= 15 {
			say("TRIAL CONFIRMED scenario=%s after=%.1fs true_state=%q", scn, time.Since(start).Seconds(), why())
			status()
			return
		}
		time.Sleep(time.Second)
	}
	say("TRIAL ROLLBACK scenario=%s after=60s true_state=%q guard_still_feeding=%v health_checks_run=%d slowest_check=%.1fs consecutive_good=%d (15 needed)", scn, why(), guardState() != '-', iters, slowest.Seconds(), good)
	status()
	time.Sleep(time.Second)
	syscall.Sync()
	syscall.Reboot(syscall.LINUX_REBOOT_CMD_RESTART)
}

func brainStatus() (lines int, udevdRestarts int) {
	out := sh("hubsim ctl /run/hubos/brain.sock status")
	for _, l := range strings.Split(out, "\n") {
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
	if back < 0 {
		for _, l := range strings.Split(sh("tail -n 25 /var/log/brain/current 2>/dev/null"), "\n") {
			say("BRAIN-LOG %s", l)
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

// ---------- follow-up 4, item 2d: the kill budget across a restart of the watcher and across 10 minutes ----------

func readFile(p string) string { b, _ := os.ReadFile(p); return strings.TrimSpace(string(b)) }

func stateOf(pid int) string {
	b, _ := os.ReadFile(fmt.Sprintf("/proc/%d/status", pid))
	for _, l := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(l, "State:") {
			return strings.TrimSpace(strings.TrimPrefix(l, "State:"))
		}
	}
	return "gone"
}

// budgetSuite: 3 hangs, each killed by the watcher, and the WATCHER killed after each of them (the init restarts it; the new
// watcher must read the kill budget from the file). A 4th hang: it must be capped (state file "capped ...", the compositor stays
// frozen; the watcher is killed once more during the cap and the file and the cap must hold). Nobody helps. Then the budget
// returns 10 minutes after the first kill: the frozen compositor is killed, the state file is removed. Then, more than 10
// minutes after the last kill, one more hang must be killed again at the normal time with no cap.
func budgetSuite() {
	modes := []string{"stop", "spin", "deadlock"}
	for i := 0; i < 3; i++ {
		fHangAfter(modes[i], 0)
		say("BUDGET-TEST after hang %d killed: %s", i+1, watcherFiles())
		between()
		fWatcherKill()
		time.Sleep(4 * time.Second)
		say("BUDGET-TEST after the watcher was killed and restarted (%d): %s", i+1, watcherFiles())
		between()
	}
	say("BUDGET-TEST budget file now: %q", strings.ReplaceAll(readFile("/run/hubos/watcher.budget"), "\n", " | "))
	// hang 4
	if !begin("fHangCapped") {
		return
	}
	evn++
	p := pidOf("driftwm")
	t0 := time.Now()
	killp(p, syscall.SIGSTOP)
	say("BUDGET-TEST hang 4 injected (SIGSTOP of driftwm pid=%d); no help from the harness", p)
	for time.Since(t0) < 150*time.Second && !exists("/run/hubos/watcher.state") {
		time.Sleep(500 * time.Millisecond)
	}
	say("BUDGET-TEST state file after %.1fs: %q (driftwm pid=%d State: %s)", time.Since(t0).Seconds(), readFile("/run/hubos/watcher.state"), pidOf("driftwm"), stateOf(pidOf("driftwm")))
	time.Sleep(20 * time.Second)
	say("BUDGET-TEST 20 s later: pid=%d State: %s state=%q", pidOf("driftwm"), stateOf(pidOf("driftwm")), readFile("/run/hubos/watcher.state"))
	fWatcherKill()
	time.Sleep(4 * time.Second)
	say("BUDGET-TEST after the watcher was killed during the cap: %s budget=%q", watcherFiles(), strings.ReplaceAll(readFile("/run/hubos/watcher.budget"), "\n", " | "))
	k4 := -1.0
	for time.Since(t0) < 900*time.Second {
		time.Sleep(time.Second)
		if q := pidOf("driftwm"); q != p {
			k4 = time.Since(t0).Seconds()
			break
		}
		if int(time.Since(t0).Seconds())%30 == 0 {
			fmt.Printf("CH t=%.1f BUDGET-TEST waiting for the budget to return: driftwm pid=%d State: %s state=%q uptime=%.0f\n", time.Since(t0).Seconds(), pidOf("driftwm"), stateOf(pidOf("driftwm")), readFile("/run/hubos/watcher.state"), uptime())
		}
	}
	say("BUDGET-TEST the frozen compositor was replaced after %.1fs (uptime %.0f); state file now: %q", k4, uptime(), readFile("/run/hubos/watcher.state"))
	rec, ok := waitOK(60 * time.Second)
	gone := false
	for i := 0; i < 20 && !gone; i++ {
		gone = !exists("/run/hubos/watcher.state")
		time.Sleep(250 * time.Millisecond)
	}
	say("BUDGET-TEST desktop back=%v (%.1fs after the replacement), state file removed=%v, budget=%q", ok, rec, gone, strings.ReplaceAll(readFile("/run/hubos/watcher.budget"), "\n", " | "))
	say("EV ev=%d fault=hang-capped target=driftwm rec=%.2f ok=%d fixed=0 k4=%.1f", evn, k4+rec, b2i(ok), k4)
	// more than 10 minutes after the last kill: a further hang must be killed again
	lastKill := uptime()
	for uptime() < lastKill+615 {
		fmt.Printf("CH t=%.1f BUDGET-TEST waiting 10 minutes after the last kill: uptime=%.0f budget_entries=%d\n", time.Since(t0).Seconds(), uptime(), strings.Count(readFile("/run/hubos/watcher.budget"), "kill "))
		time.Sleep(30 * time.Second)
	}
	say("BUDGET-TEST 10 minutes after the last kill: state file=%q", readFile("/run/hubos/watcher.state"))
	fHangAfter("deadlock", 0)
	say("BUDGET-TEST after the further hang: %s", watcherFiles())
}

// ---------- follow-up 4, item 4: the svlogd "unable to lock directory" lines (runit) ----------

// lockSnapshot: the runsv and svlogd processes of one service (pid, parent, state), who holds the flock on its log directory's
// lock file (from /proc/locks, by inode), and the service's own pids. One line.
func lockSnapshot(svc string) string {
	var rs, ls, sv []string
	for _, p := range procs() {
		c := comm(p)
		cl := cmdline(p)
		switch {
		case c == "runsv" && strings.HasSuffix(cl, " "+svc):
			rs = append(rs, fmt.Sprintf("%d", p))
		case c == "svlogd" && strings.Contains(cl, "/var/log/"+svc):
			ls = append(ls, fmt.Sprintf("%d(%c)", p, state(p)))
		case c == svc:
			sv = append(sv, fmt.Sprintf("%d(%c)", p, state(p)))
		}
	}
	holder := "-"
	if st, err := os.Stat("/var/log/" + svc + "/lock"); err == nil {
		ino := st.Sys().(*syscall.Stat_t).Ino
		b, _ := os.ReadFile("/proc/locks")
		for _, l := range strings.Split(string(b), "\n") {
			if strings.HasSuffix(l, fmt.Sprintf(":%d", ino)) || strings.Contains(l, fmt.Sprintf(":%d ", ino)) {
				f := strings.Fields(l)
				if len(f) > 4 {
					holder = f[4]
					break
				}
			}
		}
	}
	return fmt.Sprintf("runsv=%v svlogd=%v lock_holder=%s service=%v", rs, ls, holder, sv)
}

// lockProbeSuite: for each of the supervisor-kill steps (the same as fSupKill: kill the per-service supervisor, then the service),
// sample the processes and the lock holder every 100 ms and print a line whenever anything changed; then dump the service's log
// files to the console so the host can count lost or doubled lines (the stand-in numbers its heartbeat lines: pid and seq).
func lockProbeSuite(reps int) {
	for i := 0; i < reps; i++ {
		for _, svc := range []string{"driftwm", "waybar"} {
			stop := make(chan bool)
			done := make(chan bool)
			go func() {
				last := ""
				st := time.Now()
				for {
					select {
					case <-stop:
						done <- true
						return
					default:
					}
					if s := lockSnapshot(svc); s != last {
						say("LOCKPROBE svc=%s t=%.2f %s", svc, time.Since(st).Seconds(), s)
						last = s
					}
					time.Sleep(100 * time.Millisecond)
				}
			}()
			say("LOCKPROBE-STEP rep=%d svc=%s begin", i+1, svc)
			fSupKill(svc)
			time.Sleep(3 * time.Second)
			close(stop)
			<-done
			say("LOCKPROBE-STEP rep=%d svc=%s end", i+1, svc)
			between()
		}
	}
	for _, svc := range []string{"driftwm", "waybar"} {
		for _, l := range strings.Split(sh("ls /var/log/"+svc+"/ 2>&1; for f in /var/log/"+svc+"/@* /var/log/"+svc+"/current; do [ -f $f ] && cat $f; done"), "\n") {
			fmt.Printf("LOGDUMP %s %s\n", svc, l)
		}
	}
}

// killp sends a signal to one process. A pid of 0 or below is ignored: kill(0, sig) signals the sender's whole process group, kill(-1, sig) every
// process, and pidOf returns 0 when the service has no process at that instant. Follow-up 4: a SIGKILL sent that way ended the injector itself and
// everything in its group in one 3-rep run of s6rc-par-wd2 (the suite stopped at step 36); every earlier run in this follow-up had finished its suite.
func killp(pid int, sig syscall.Signal) {
	if pid <= 0 {
		say("KILLP skipped: no process (pid %d) to send signal %d to", pid, int(sig))
		return
	}
	syscall.Kill(pid, sig)
}

// ---------- follow-up 5: the latch version of the test watcher ----------

const (
	wBudget = "/run/hubos/watcher.budget"
	wLatch  = "/run/hubos/watcher.latch"
	wState  = "/run/hubos/watcher.state"
)

// personReset: THE HARNESS PLAYS THE PERSON. It runs the one command that clears the latch (the test's hook `watcher reset`; the
// product's command is not designed). Every call is marked in the log.
func personReset(why string) {
	say("PERSON-COMMAND *** the harness plays the person: running `watcher reset` (%s) ***", why)
	out := sh("/bin/watcher reset 2>&1")
	say("PERSON-COMMAND output: %q; now %s", out, watcherFiles())
}

// fillBudget: three hangs (SIGSTOP, busy loop, deadlock), each killed by the watcher alone.
func fillBudget() {
	for i, m := range []string{"stop", "spin", "deadlock"} {
		fHangAfter(m, 0)
		say("LATCH-TEST after hang %d: %s", i+1, watcherFiles())
		between()
	}
}

// frozen: the compositor process p is still the same one and in State T (stopped)
func frozen(p int) bool { return pidOf("driftwm") == p && strings.HasPrefix(stateOf(p), "T") }

// freeze4: inject the 4th hang (SIGSTOP) and wait for the state file; returns the pid and the seconds until the file appeared.
func freeze4(tag string) (int, float64, bool) {
	p := pidOf("driftwm")
	t := time.Now()
	killp(p, syscall.SIGSTOP)
	say("%s hang 4 injected (SIGSTOP of driftwm pid=%d); no help from the harness; %s", tag, p, watcherFiles())
	for time.Since(t) < 150*time.Second && !exists(wState) {
		time.Sleep(200 * time.Millisecond)
	}
	d := time.Since(t).Seconds()
	say("%s state file after %.1fs: %q; latch file=%v; driftwm pid=%d State: %s; budget=%q", tag, d, readFile(wState), exists(wLatch), pidOf("driftwm"), stateOf(p), strings.ReplaceAll(readFile(wBudget), "\n", " | "))
	return p, d, exists(wState)
}

// timeToKill: after the person-command, wait for the compositor to be replaced, then for the desktop to be back.
func timeToKill(p int, from time.Time) {
	k := -1.0
	for time.Since(from) < 120*time.Second {
		if q := pidOf("driftwm"); q != p {
			k = time.Since(from).Seconds()
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	rec, ok := waitOK(60 * time.Second)
	say("LATCH-TEST after the person-command: reset to kill %.2fs, reset to desktop back %.2fs (desktop ok=%v); state file present=%v; %s", k, time.Since(from).Seconds(), ok, exists(wState), watcherFiles())
	_ = rec
}

// latchSuite (item 2a and 2b): three hangs killed; a 4th (SIGSTOP) is latched and must stay frozen for `hold` seconds (the window version
// replaced it after about 490 s); the watcher is killed (SIGKILL) two minutes into the freeze and the latch, the state file and the
// frozen state must be unchanged; then the harness plays the person: reset; time from reset to kill and to desktop back.
func latchSuite(hold int) {
	fillBudget()
	if !begin("fHangLatched") {
		return
	}
	evn++
	p, _, got := freeze4("LATCH-TEST")
	if !got {
		say("LATCH-TEST NO STATE FILE after 150 s")
	}
	t := time.Now()
	samples, notFrozen, noLatch, noState := 0, 0, 0, 0
	killed := false
	for time.Since(t) < time.Duration(hold)*time.Second {
		time.Sleep(2 * time.Second)
		samples++
		if !frozen(p) {
			notFrozen++
			say("LATCH-TEST NOT FROZEN at %.0fs: pid now %d State %s", time.Since(t).Seconds(), pidOf("driftwm"), stateOf(p))
		}
		if !exists(wLatch) {
			noLatch++
		}
		if !exists(wState) {
			noState++
		}
		if int(time.Since(t).Seconds())%60 < 2 {
			fmt.Printf("CH t=%.1f LATCH-TEST held %.0fs of %d: frozen=%v latch=%v state=%q uptime=%.0f\n", time.Since(t0).Seconds(), time.Since(t).Seconds(), hold, frozen(p), exists(wLatch), readFile(wState), uptime())
		}
		if !killed && time.Since(t) > 120*time.Second {
			killed = true
			before := fmt.Sprintf("latch=%q state=%q budget=%q frozen=%v", readFile(wLatch), readFile(wState), strings.ReplaceAll(readFile(wBudget), "\n", " | "), frozen(p))
			say("LATCH-TEST (b) killing the WATCHER (SIGKILL) while latched and frozen; before: %s", before)
			r := fWatcherKill()
			time.Sleep(4 * time.Second)
			after := fmt.Sprintf("latch=%q state=%q budget=%q frozen=%v", readFile(wLatch), readFile(wState), strings.ReplaceAll(readFile(wBudget), "\n", " | "), frozen(p))
			say("LATCH-TEST (b) init restarted the watcher in %.2fs (ok=%v); after: %s; unchanged=%v", r.rec, r.ok, after, before == after)
		}
	}
	say("LATCH-TEST (a) held %.0fs, %d samples (every 2 s): not frozen in %d, latch missing in %d, state file missing in %d; uptime now %.0f", time.Since(t).Seconds(), samples, notFrozen, noLatch, noState, uptime())
	tr := time.Now()
	personReset("after the hold")
	timeToKill(p, tr)
	say("EV ev=%d fault=hang-latched target=driftwm rec=%.2f ok=1 fixed=0", evn, time.Since(tr).Seconds())
}

// latchCSuite (item 2c and 2d): a latched, frozen compositor is resumed (SIGCONT) WITHOUT reset: the state file must go within one
// round, the latch must stay; hung again it must NOT be killed, the state file must come back and it must stay frozen; then
// reset. Then (2d): a hang, the watcher killed about 15 s into its streak: time from the restart to the kill and from the fault
// to the desktop.
func latchCSuite() {
	fillBudget()
	if !begin("fHangLatchedC") {
		return
	}
	evn++
	p, _, _ := freeze4("LATCH-TEST(c)")
	time.Sleep(10 * time.Second)
	say("LATCH-TEST(c) still frozen=%v, latch=%v, state=%q", frozen(p), exists(wLatch), readFile(wState))
	tc := time.Now()
	killp(p, syscall.SIGCONT)
	for time.Since(tc) < 30*time.Second && exists(wState) {
		time.Sleep(50 * time.Millisecond)
	}
	say("LATCH-TEST(c) SIGCONT sent; state file removed after %.2fs (present now=%v); latch still set=%v; compositor State: %s pid same=%v", time.Since(tc).Seconds(), exists(wState), exists(wLatch), stateOf(p), pidOf("driftwm") == p)
	_, ok := waitOK(30 * time.Second)
	say("LATCH-TEST(c) desktop ok=%v with the latch still set=%v", ok, exists(wLatch))
	time.Sleep(5 * time.Second)
	th := time.Now()
	killp(p, syscall.SIGSTOP)
	say("LATCH-TEST(c) hung AGAIN (SIGSTOP of the same process pid=%d); no help", p)
	for time.Since(th) < 60*time.Second && !exists(wState) {
		time.Sleep(100 * time.Millisecond)
	}
	say("LATCH-TEST(c) state file came back after %.1fs: %q; latch=%v", time.Since(th).Seconds(), readFile(wState), exists(wLatch))
	for time.Since(th) < 120*time.Second {
		time.Sleep(2 * time.Second)
	}
	say("LATCH-TEST(c) 120 s after the second hang: frozen=%v (pid same=%v, State %s) state=%q latch=%v; NOT killed=%v", frozen(p), pidOf("driftwm") == p, stateOf(p), readFile(wState), exists(wLatch), pidOf("driftwm") == p)
	tr := time.Now()
	personReset("after the second hang")
	timeToKill(p, tr)
	say("EV ev=%d fault=hang-latched-c target=driftwm rec=%.2f ok=1 fixed=0", evn, time.Since(tr).Seconds())
	between()
	// (d): a watcher restart 15 s into a hang's streak
	if !begin("fHangRestartStreak") {
		return
	}
	evn++
	p2 := pidOf("driftwm")
	tf := time.Now()
	killp(p2, syscall.SIGSTOP)
	say("LATCH-TEST(d) hang injected (SIGSTOP pid=%d); the watcher will be killed 15 s into the streak; %s", p2, watcherFiles())
	time.Sleep(15 * time.Second)
	w := pidOf(cfg["watcher"])
	tw := time.Now()
	killp(w, syscall.SIGKILL)
	var restart float64
	for time.Since(tw) < 30*time.Second {
		time.Sleep(50 * time.Millisecond)
		if q := pidOf(cfg["watcher"]); q != 0 && q != w {
			restart = time.Since(tw).Seconds()
			break
		}
	}
	trs := time.Now()
	say("LATCH-TEST(d) watcher killed at %.1fs after the fault; init restarted it in %.2fs", tw.Sub(tf).Seconds(), restart)
	k := -1.0
	for time.Since(tf) < 150*time.Second {
		if q := pidOf("driftwm"); q != p2 {
			k = time.Since(trs).Seconds()
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	kf := time.Since(tf).Seconds()
	rec, ok2 := waitOK(60 * time.Second)
	say("LATCH-TEST(d) from the watcher's restart to the kill: %.2fs (expect about 30 s); fault to kill %.2fs; fault to desktop back %.2fs (ok=%v); %s", k, kf, time.Since(tf).Seconds(), ok2, watcherFiles())
	_ = rec
	say("EV ev=%d fault=hang-restart-streak target=driftwm rec=%.2f ok=%d fixed=0", evn, time.Since(tf).Seconds(), b2i(ok2))
}

// hangSuiteP (item 2e): the spaced hang run for the numbers. Same as hangSuite, but after every third hang the harness plays the
// person (reset) instead of waiting for the 10 minutes, so that the latch never blocks.
func hangSuiteP(rng *rand.Rand, n int) {
	modes := []string{"stop", "spin", "deadlock"}
	for i := 0; i < n; i++ {
		if i%3 == 0 {
			rng.Shuffle(len(modes), func(a, b int) { modes[a], modes[b] = modes[b], modes[a] })
			if i > 0 {
				personReset(fmt.Sprintf("after hang %d, so that the latch never blocks the next group", i))
				time.Sleep(2 * time.Second)
			}
		}
		pause := time.Duration(rng.Intn(1000)) * time.Millisecond
		fHangAfter(modes[i%3], pause)
		between()
	}
}

// storm3Suite (item 2f): crash storms with an empty budget at the start of each; the watcher's SKIP/KILL lines are in the log; here the
// budget and latch files after each storm. Then once after two real hang kills, and a real hang afterwards.
func storm3Suite(n int) {
	for i := 0; i < n; i++ {
		say("STORM-TEST %d start: %s", i+1, watcherFiles())
		fStorm("driftwm", 40*time.Second)
		time.Sleep(3 * time.Second)
		say("STORM-TEST %d end: %s budget=%q", i+1, watcherFiles(), strings.ReplaceAll(readFile(wBudget), "\n", " | "))
		between()
	}
	say("STORM-TEST two real hangs, then a storm, then a real hang")
	fHangAfter("stop", 0)
	between()
	fHangAfter("spin", 0)
	between()
	say("STORM-TEST after two real hang kills: %s budget=%q", watcherFiles(), strings.ReplaceAll(readFile(wBudget), "\n", " | "))
	fStorm("driftwm", 40*time.Second)
	time.Sleep(3 * time.Second)
	say("STORM-TEST storm after two real hang kills, end: %s budget=%q", watcherFiles(), strings.ReplaceAll(readFile(wBudget), "\n", " | "))
	between()
	fHangAfter("deadlock", 0)
	say("STORM-TEST real hang after the storm: %s budget=%q", watcherFiles(), strings.ReplaceAll(readFile(wBudget), "\n", " | "))
}

// slowSuite (item 2g): a slow but alive compositor. Variant 1: every answer (state and sync) 3 to 4 s late for 60 s: state is inside its
// 5-s deadline, sync is over its 2-s limit, so sync fails and state answers. Variant 2: only the sync answer is late. The watcher must not kill.
func slowSuite() {
	for _, v := range []string{"both", "sync"} {
		p := pidOf("driftwm")
		say("SLOW-TEST variant=%s start: driftwm pid=%d %s", v, p, watcherFiles())
		os.WriteFile(faultDir+"/driftwm.slow", []byte(v), 0o644)
		t := time.Now()
		for time.Since(t) < 60*time.Second {
			time.Sleep(5 * time.Second)
			fmt.Printf("CH t=%.1f SLOW-TEST variant=%s at %.0fs: pid=%d State: %s\n", time.Since(t0).Seconds(), v, time.Since(t).Seconds(), pidOf("driftwm"), stateOf(pidOf("driftwm")))
		}
		os.Remove(faultDir + "/driftwm.slow")
		say("SLOW-TEST variant=%s end of the 60 s: driftwm pid=%d (same=%v) %s", v, pidOf("driftwm"), pidOf("driftwm") == p, watcherFiles())
		time.Sleep(10 * time.Second)
		rec, ok := waitOK(30 * time.Second)
		say("SLOW-TEST variant=%s 10 s after the end: pid same=%v, desktop ok=%v (%.1fs) %s", v, pidOf("driftwm") == p, ok, rec, watcherFiles())
		time.Sleep(10 * time.Second)
	}
}
