// TEST STAND-IN. NOT the product watcher. The hang rule is not built.
//
// A tiny watcher for the init comparison (follow-up 4; follow-up 5 added the latch, the identity rule and the probe limits). It implements the owner's rule of docs/proposals/hub-stability.md
// section 9.1: two probes, BOTH aimed at the compositor and both answered by its single event-loop thread (driftwm's socket
// "state" request, and a Wayland wl_display.sync round trip). A round fails only when BOTH probes fail. After 4 failed rounds
// in a row AND at least 30 seconds since the first failed round of the streak, the compositor is killed; at most 3 kills in
// 10 minutes; nothing is ever rebooted. It restarts nothing: the init restarts what it killed. It has no restart ladder, no
// alert and no conductor. The hubd ping is logged for information only and takes no part in the decision. It writes one line
// per probe round and one per kill, with timestamps, to its standard error and to the serial console.
package main

import (
	"bufio"
	"encoding/binary"
	"flag"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Probe is copied unchanged from ../go/probe.go (the brain prototype): ask a service a question on a unix socket and want an
// answer starting with "ok" within a second. Used here for the compositor's "state" request (probe 1) and for the hubd "ping"
// that is only logged (information, no part in the decision).
func Probe(sock, request string) bool { return ProbeLimit(sock, request, time.Second) }

// Probe limits of the owner's rule (hub-stability.md 9.1; follow-up 5, decision 4): the driftwm "state" request has a 5 s
// deadline, the Wayland wl_display.sync round trip 2 s. (The follow-up 4 watcher used 1 s for both.)
const (
	StateLimit = 5 * time.Second
	SyncLimit  = 2 * time.Second
)

// ProbeLimit is Probe with a limit: the connect and the whole question-and-answer must finish within limit.
func ProbeLimit(sock, request string, limit time.Duration) bool {
	c, err := net.DialTimeout("unix", sock, limit)
	if err != nil {
		return false
	}
	defer c.Close()
	c.SetDeadline(time.Now().Add(limit))
	if _, err := c.Write([]byte(request + "\n")); err != nil {
		return false
	}
	line, err := bufio.NewReader(c).ReadString('\n')
	return err == nil && strings.HasPrefix(line, "ok")
}

// ProbeSync is probe 2: a Wayland wl_display.sync round trip on the compositor's Wayland socket (hub-stability.md 9.1: connect,
// send wl_display.sync, wait for the wl_callback.done event). It speaks the Wayland wire format by hand: a message is
// object id (u32), then (size << 16 | opcode) (u32), then the arguments, all little-endian. wl_display is object 1, its
// request sync is opcode 0 with one argument, the new object id for the callback (we use 2); the answer is the event done
// (opcode 0) on object 2. Limit: SyncLimit (2 s, as hub-stability.md 9.1 names it).
func ProbeSync(sock string) bool { return ProbeSyncLimit(sock, SyncLimit) }

// ProbeSyncLimit is ProbeSync with a limit.
func ProbeSyncLimit(sock string, limit time.Duration) bool {
	c, err := net.DialTimeout("unix", sock, limit)
	if err != nil {
		return false
	}
	defer c.Close()
	c.SetDeadline(time.Now().Add(limit))
	var req [12]byte
	binary.LittleEndian.PutUint32(req[0:], 1)        // object: wl_display
	binary.LittleEndian.PutUint32(req[4:], 12<<16|0) // size 12, opcode 0 = sync
	binary.LittleEndian.PutUint32(req[8:], 2)        // new_id of the callback
	if _, err := c.Write(req[:]); err != nil {
		return false
	}
	r := bufio.NewReader(c)
	for i := 0; i < 8; i++ { // events before done (for example delete_id) are skipped
		var h [8]byte
		if _, err := readFull(r, h[:]); err != nil {
			return false
		}
		obj, w := binary.LittleEndian.Uint32(h[0:]), binary.LittleEndian.Uint32(h[4:])
		size, op := int(w>>16), int(w&0xffff)
		if size < 8 || size > 4096 {
			return false
		}
		if size > 8 {
			if _, err := readFull(r, make([]byte, size-8)); err != nil {
				return false
			}
		}
		if obj == 2 && op == 0 {
			return true
		}
	}
	return false
}

func readFull(r *bufio.Reader, b []byte) (int, error) {
	n := 0
	for n < len(b) {
		m, err := r.Read(b[n:])
		n += m
		if err != nil {
			return n, err
		}
	}
	return n, nil
}

// Rule holds the owner's numbers and the state of the current failure streak. The kills made live in the Budget (a file).
// All times are BootClock readings.
type Rule struct {
	Rounds int           // failed rounds in a row before a kill (4)
	Window time.Duration // time since the first failed round of the streak before a kill (30 s)
	B      *Budget       // 3 kills in 10 minutes
	streak int
	first  time.Duration
}

// Decision is what Round tells the caller.
type Decision int

const (
	Nothing Decision = iota
	Kill             // kill the compositor now (the kill is already written into the budget)
	Capped           // would kill, but 3 kills in the last 10 minutes were already made
)

// Want records one probe round at time now (state = the "state" probe answered, sync = the wl_display.sync probe answered).
// A round fails only when BOTH probes fail. want is true when the rule asks for a kill: 4 failed rounds in a row and at least
// 30 s since the first failed round. It touches no budget and no file (the Watcher of watcher.go decides what follows).
func (r *Rule) Want(now time.Duration, state, sync bool) (failed, want bool) {
	failed = !state && !sync
	if !failed {
		r.streak = 0
		return false, false
	}
	if r.streak == 0 {
		r.first = now
	}
	r.streak++
	return true, r.streak >= r.Rounds && now-r.first >= r.Window
}

// Restart begins a new streak whose first failed round is the one at time now.
func (r *Rule) Restart(now time.Duration) { r.streak, r.first = 1, now }

// Reset ends the streak.
func (r *Rule) Reset() { r.streak = 0 }

// Round is the follow-up 4 behaviour, the "window" version: Want, then the budget alone decides (3 kills in 10 minutes; when
// the budget is full the round is Capped, and the first round with room kills). It is kept for the tests of that version and
// for the record; the program uses Watcher.Step (the latch version).
func (r *Rule) Round(now time.Duration, state, sync bool) (failed bool, d Decision) {
	failed, want := r.Want(now, state, sync)
	if !want {
		return failed, Nothing
	}
	if r.B.Full(now) {
		return true, Capped
	}
	r.B.Add(now)
	r.streak = 0
	return true, Kill
}

func (r *Rule) Streak() (int, time.Duration) { return r.streak, r.first }

// pidsNamed: processes whose /proc/PID/comm is name.
func pidsNamed(name string) []int {
	var out []int
	d, _ := os.ReadDir("/proc")
	for _, e := range d {
		p, err := strconv.Atoi(e.Name())
		if err != nil {
			continue
		}
		b, _ := os.ReadFile(fmt.Sprintf("/proc/%d/comm", p))
		if strings.TrimSpace(string(b)) == name {
			out = append(out, p)
		}
	}
	return out
}

// evidence: what /proc says about the process that is about to be killed: State, wchan, CPU ticks (user+system), and its age. The
// age tells a process that has just been started (a crash loop) from one that has been hung for a long time.
func evidence(pid int) string {
	st, _ := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	str := string(st)
	i := strings.LastIndex(str, ")")
	if i < 0 || len(str) < i+3 {
		return fmt.Sprintf("pid=%d (no /proc entry)", pid)
	}
	f := strings.Fields(str[i+2:])
	state, ticks, age := f[0], 0, -1.0
	if len(f) > 19 {
		u, _ := strconv.Atoi(f[11])
		k, _ := strconv.Atoi(f[12])
		ticks = u + k
		start, _ := strconv.ParseFloat(f[19], 64) // clock ticks since boot (100 per second)
		age = BootClock().Seconds() - start/100
	}
	wchan, _ := os.ReadFile(fmt.Sprintf("/proc/%d/wchan", pid))
	return fmt.Sprintf("pid=%d state=%s wchan=%s cpu_ticks=%d age=%.2fs", pid, state, strings.TrimSpace(string(wchan)), ticks, age)
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "reset" { // the person-command of the test (only the hook; the product's is not designed)
		fs := flag.NewFlagSet("reset", flag.ExitOnError)
		bp := fs.String("budget", "/run/hubos/watcher.budget", "budget file")
		lp := fs.String("latch", "/run/hubos/watcher.latch", "latch file")
		sp := fs.String("state", "/run/hubos/watcher.state", "state file")
		fs.Parse(os.Args[2:])
		if err := Reset(*bp, *lp, *sp); err != nil {
			fmt.Fprintln(os.Stderr, "watcher reset:", err)
			os.Exit(1)
		}
		fmt.Println("watcher reset: budget emptied, latch and state file removed")
		return
	}
	comp := flag.String("comp", "/run/dw/wayland-1", "compositor socket (the \"state\" request and the Wayland sync both go here in the test image)")
	hubd := flag.String("hubd", "/run/hubos/hubd.sock", "hubd socket (ping, logged for information only)")
	round := flag.Duration("round", time.Second, "time between rounds")
	target := flag.String("target", "driftwm", "process name of the compositor to kill")
	budgetPath := flag.String("budget", "/run/hubos/watcher.budget", "file with the kill times (in the in-memory /run, so a restarted watcher keeps its budget)")
	latchPath := flag.String("latch", "/run/hubos/watcher.latch", "latch file: present while the watcher is stopped until a person clears it (`watcher reset`)")
	statePath := flag.String("state", "/run/hubos/watcher.state", "one-line state file: \"capped <time>\" while the rule wants a kill and the latch blocks it (the hook a bar would read)")
	stateLimit := flag.Duration("state-limit", StateLimit, "deadline of the \"state\" probe")
	syncLimit := flag.Duration("sync-limit", SyncLimit, "limit of the wl_display.sync probe")
	clearAfter := flag.Duration("latch-clear-after", 3600*time.Second, "the latch is removed by itself after this long with BOTH probes answering in every round (0 = never; the timer is in memory only)")
	console := flag.String("console", "/dev/console", "also write the log here (empty: standard error only)")
	flag.Parse()
	var cons *os.File
	if *console != "" {
		cons, _ = os.OpenFile(*console, os.O_WRONLY|os.O_APPEND, 0)
	}
	logf := func(f string, a ...any) {
		l := fmt.Sprintf("watcher: "+f+"\n", a...)
		fmt.Fprint(os.Stderr, l)
		if cons != nil {
			cons.WriteString(l)
		}
	}
	os.MkdirAll(filepath.Dir(*budgetPath), 0o755)
	bud, note := LoadBudget(*budgetPath, BootClock(), 3, 10*time.Minute)
	w := &Watcher{
		R: &Rule{Rounds: 4, Window: 30 * time.Second, B: bud}, B: bud,
		Latch: Latch{*latchPath}, State: StateFile{*statePath},
		Find:     func() []Proc { return findProcs(*target) },
		Kill:     killProc,
		Evidence: func(p Proc) string { return evidence(p.PID) },
		Wall:     time.Now, Log: logf, Target: *target, ClearAfter: *clearAfter,
	}
	logf("started pid=%d probes=state(%v)+wl_display.sync(%v), hubd ping is information only (TEST STAND-IN, not the product watcher; latch version + auto-clear after %v)", os.Getpid(), *stateLimit, *syncLimit, *clearAfter)
	logf("%s (%s)", note, *budgetPath)
	if l, dmg := w.Latch.State(); l {
		logf("LATCH is set at start (%s): nothing is killed until a person clears it %s", *latchPath, dmg)
	}
	if w.State.Exists() {
		logf("state file %s exists at start: kept until both probes answer", *statePath)
	}
	n := 0
	for range time.Tick(*round) {
		n++
		now := BootClock()
		var st, sy, hb bool
		var wg sync.WaitGroup
		wg.Add(3)
		go func() { defer wg.Done(); st = ProbeLimit(*comp, "state", *stateLimit) }()
		go func() { defer wg.Done(); sy = ProbeSyncLimit(*comp, *syncLimit) }()
		go func() { defer wg.Done(); hb = Probe(*hubd, "ping") }()
		wg.Wait()
		res := w.Step(now, st, sy)
		s, f1 := w.R.Streak()
		since := 0.0
		if s > 0 {
			since = (now - f1).Seconds()
		}
		logf("round %d state=%s sync=%s hubd(info only)=%s failed_round=%v streak=%d since_first=%.1fs result=%s", n, ok(st), ok(sy), ok(hb), !st && !sy, s, since, res)
	}
}

func ok(b bool) string {
	if b {
		return "ok"
	}
	return "FAIL"
}
