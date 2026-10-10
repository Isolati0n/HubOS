package main

import (
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"strconv"
	"strings"
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
			g = append(g, pidsNamed(n)...)
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
			syscall.Kill(p, syscall.SIGKILL)
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
	syscall.Kill(old, syscall.SIGKILL)
	return settle("kill", s, "")
}

func fHang(mode string) result {
	if !begin("fHang") {
		return result{ok: true}
	}
	p := pidOf("driftwm")
	if mode == "stop" {
		syscall.Kill(p, syscall.SIGSTOP)
	} else {
		os.WriteFile(faultDir+"/driftwm.spin", nil, 0o644)
	}
	r := settle("hang-"+mode, "driftwm", "")
	os.Remove(faultDir + "/driftwm.spin")
	return r
}

func fStorm(s string, dur time.Duration) result {
	if !begin("fStorm") {
		return result{ok: true}
	}
	g := supGroup()
	c0, s0 := cpuTicks(g), starts(s)
	os.WriteFile(faultDir+"/"+s+".crash", nil, 0o644)
	syscall.Kill(pidOf(s), syscall.SIGKILL)
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
	syscall.Kill(pidOf("seatd"), syscall.SIGKILL)
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
			syscall.Kill(pidOf("waybar"), syscall.SIGKILL) // a restart while the disk is full
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
	syscall.Kill(sp, syscall.SIGKILL)
	time.Sleep(1500 * time.Millisecond)
	kept := pidOf(s) == old
	r1 := settle("supervisor-killed", s, fmt.Sprintf("child_kept_running=%d", b2i(kept)))
	time.Sleep(2 * time.Second)
	back := supOf(s) != 0
	// does anybody notice the service now? kill it and see
	syscall.Kill(pidOf(s), syscall.SIGKILL)
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
		syscall.Kill(p, syscall.SIGKILL)
	}
	time.Sleep(1500 * time.Millisecond)
	kept := pidOf("driftwm") == old
	back := pidOf(n) != 0
	r := settle("scanner-killed", n, fmt.Sprintf("child_kept_running=%d scanner_back=%d", b2i(kept), b2i(back)))
	syscall.Kill(pidOf("hubd"), syscall.SIGKILL)
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
		syscall.Kill(p, syscall.SIGKILL)
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
	syscall.Kill(old, syscall.SIGKILL)
	setClock(delta)
	r := settle(fmt.Sprintf("clock%+d", delta), "driftwm", "")
	setClock(-delta)
	time.Sleep(time.Second)
	// after the clock is put back: is the supervisor still sane?
	syscall.Kill(pidOf("hubd"), syscall.SIGKILL)
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
		syscall.Kill(pidOf("udevd"), syscall.SIGKILL)
		waitOK(60 * time.Second)
		time.Sleep(5 * time.Second)
		for i := 0; i < 5; i++ {
			fPolicyBug()
			between()
		}
	case "faults":
		faultSuite(cfgInt("reps", 5))
	case "soak":
		soak(rng, time.Duration(cfgInt("dur", 600))*time.Second)
	}
	say("SUITE-DONE")
	time.Sleep(2 * time.Second)
	syscall.Sync()
	syscall.Reboot(syscall.LINUX_REBOOT_CMD_POWER_OFF)
}

func between() { time.Sleep(3 * time.Second) }

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
	for time.Now().Before(end) {
		pick()
		time.Sleep(time.Duration(5+rng.Intn(15)) * time.Second)
	}
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
	hung := false
	sawOK := time.Time{}
	for time.Since(start) < 60*time.Second {
		if why() == "" && sawOK.IsZero() {
			sawOK = time.Now()
		}
		if scn == "hang" && !hung && !sawOK.IsZero() && time.Since(sawOK) > 2*time.Second {
			syscall.Kill(pidOf("driftwm"), syscall.SIGSTOP)
			hung = true
			say("TRIAL-INJECT driftwm stopped (wedged)")
		}
		if _, err := os.Stat("/ops/healthy"); err == nil && exec.Command("/bin/sh", "/ops/healthy").Run() == nil {
			good++
		} else {
			good = 0
		}
		if good >= 15 {
			say("TRIAL CONFIRMED scenario=%s after=%.1fs true_state=%q", scn, time.Since(start).Seconds(), why())
			status()
			return
		}
		time.Sleep(time.Second)
	}
	say("TRIAL ROLLBACK scenario=%s after=60s true_state=%q guard_still_feeding=%v", scn, why(), guardState() != '-')
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
