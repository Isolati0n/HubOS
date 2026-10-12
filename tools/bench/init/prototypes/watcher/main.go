// TEST STAND-IN. NOT the product watcher. The hang rule is not built.
//
// A tiny watcher for the init comparison (follow-up 4). It implements the owner's rule of docs/proposals/hub-stability.md
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
	"syscall"
	"time"
)

// Probe is copied unchanged from ../go/probe.go (the brain prototype): ask a service a question on a unix socket and want an
// answer starting with "ok" within a second. Used here for the compositor's "state" request (probe 1) and for the hubd "ping"
// that is only logged (information, no part in the decision).
func Probe(sock, request string) bool {
	c, err := net.DialTimeout("unix", sock, time.Second)
	if err != nil {
		return false
	}
	defer c.Close()
	c.SetDeadline(time.Now().Add(time.Second))
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
// (opcode 0) on object 2. Limit: 1 s (the same as the other probe; the document names 2 s, a setting).
func ProbeSync(sock string) bool {
	c, err := net.DialTimeout("unix", sock, time.Second)
	if err != nil {
		return false
	}
	defer c.Close()
	c.SetDeadline(time.Now().Add(time.Second))
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

// Round records one probe round at time now (state = the "state" probe answered, sync = the wl_display.sync probe answered)
// and returns the decision. A round fails only when BOTH probes fail.
func (r *Rule) Round(now time.Duration, state, sync bool) (failed bool, d Decision) {
	failed = !state && !sync
	if !failed {
		r.streak = 0
		return false, Nothing
	}
	if r.streak == 0 {
		r.first = now
	}
	r.streak++
	if r.streak < r.Rounds || now-r.first < r.Window {
		return true, Nothing
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

func main() {
	comp := flag.String("comp", "/run/dw/wayland-1", "compositor socket (the \"state\" request and the Wayland sync both go here in the test image)")
	hubd := flag.String("hubd", "/run/hubos/hubd.sock", "hubd socket (ping, logged for information only)")
	round := flag.Duration("round", time.Second, "time between rounds")
	target := flag.String("target", "driftwm", "process name of the compositor to kill")
	budgetPath := flag.String("budget", "/run/hubos/watcher.budget", "file with the kill times (in the in-memory /run, so a restarted watcher keeps its budget)")
	statePath := flag.String("state", "/run/hubos/watcher.state", "one-line state file: \"capped <time>\" while the cap blocks a kill (the hook a bar would read)")
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
	r := &Rule{Rounds: 4, Window: 30 * time.Second, B: bud}
	sf := StateFile{Path: *statePath}
	logf("started pid=%d probes=state+wl_display.sync, hubd ping is information only (TEST STAND-IN, not the product watcher)", os.Getpid())
	logf("%s (%s)", note, *budgetPath)
	capped := sf.Exists()
	if capped && !bud.Full(BootClock()) {
		sf.Clear()
		capped = false
		logf("state file %s was left over and the budget has room: removed", *statePath)
	}
	if capped {
		logf("state file %s exists and the budget is full: kept", *statePath)
	}
	n := 0
	for range time.Tick(*round) {
		n++
		now := BootClock()
		if capped && !bud.Full(now) { // kills are allowed again: the state file goes (before this round's decision)
			if err := sf.Clear(); err != nil {
				logf("state file not removed: %v", err)
			}
			capped = false
			logf("STATE kills are allowed again: removed %s", *statePath)
		}
		var st, sy, hb bool
		var wg sync.WaitGroup
		wg.Add(3)
		go func() { defer wg.Done(); st = Probe(*comp, "state") }()
		go func() { defer wg.Done(); sy = ProbeSync(*comp) }()
		go func() { defer wg.Done(); hb = Probe(*hubd, "ping") }()
		wg.Wait()
		failed, d := r.Round(now, st, sy)
		s, first := r.Streak()
		since := 0.0
		if s > 0 {
			since = (now - first).Seconds()
		}
		logf("round %d state=%s sync=%s hubd(info only)=%s failed_round=%v streak=%d since_first=%.1fs", n, ok(st), ok(sy), ok(hb), failed, s, since)
		switch d {
		case Kill:
			if bud.SaveErr != nil {
				logf("budget file not written: %v (the kill still counts in memory)", bud.SaveErr)
			}
			logf("BUDGET kill %d of %d in the last 10 minutes recorded in %s", bud.Len(), bud.Max, *budgetPath)
			pids := pidsNamed(*target)
			for _, p := range pids {
				err := syscall.Kill(p, syscall.SIGKILL)
				logf("KILL %s pid=%d err=%v", *target, p, err)
			}
			if len(pids) == 0 {
				logf("KILL %s: no process of that name exists at this moment (the kill counts against the budget)", *target)
			}
		case Capped:
			logf("CAP: would kill %s, but 3 kills in the last 10 minutes were already made; not killing", *target)
			if !capped {
				if err := sf.SetCapped(time.Now()); err != nil {
					logf("state file not written: %v", err)
				}
				capped = true
				logf("STATE wrote %s (capped)", *statePath)
			}
		}
	}
}

func ok(b bool) string {
	if b {
		return "ok"
	}
	return "FAIL"
}
