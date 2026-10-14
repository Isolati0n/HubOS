// TEST STAND-IN. NOT the product watcher. The hang rule is not built.
//
// A tiny watcher for the init comparison (follow-up 3, item 3). It implements ONLY the owner's rule of 2026-10-12:
// probe the stand-in compositor and the stand-in hubd every round; after 4 failed rounds AND 30 seconds since the first failed
// round of the streak, with BOTH probes failing, kill the compositor; at most 3 kills in 10 minutes; never reboot anything.
// It restarts nothing: the init restarts what it killed. It has no restart ladder, no alert and no conductor. It writes one
// line per probe round and one per kill, with timestamps, to its standard error and to the serial console.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// Probe is copied unchanged from ../go/probe.go (the brain prototype): ask a service a question on a unix socket and want an
// answer starting with "ok" within a second. The brain uses it for the compositor ("state"); the same function is used here
// for hubd with the request "ping" that the benchmark harness already sends to hubd (hubsim/chaos.go, why()).
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

// Rule holds the owner's numbers and the state of the current failure streak and of the kills made.
type Rule struct {
	Rounds   int           // failed rounds in a row before a kill (4)
	Window   time.Duration // time since the first failed round of the streak before a kill (30 s)
	MaxKills int           // kills allowed ... (3)
	Per      time.Duration // ... in this period (10 min)
	Both     bool          // true: a round fails only if BOTH probes fail; false: it fails if the compositor probe fails
	streak   int
	first    time.Time
	kills    []time.Time
}

// Decision is what Round tells the caller.
type Decision int

const (
	Nothing Decision = iota
	Kill             // kill the compositor now
	Capped           // would kill, but 3 kills in the last 10 minutes were already made
)

// Round records one probe round at time now (comp = compositor probe ok, hubd = hubd probe ok) and returns the decision.
func (r *Rule) Round(now time.Time, comp, hubd bool) (failed bool, d Decision) {
	failed = !comp && !hubd
	if !r.Both {
		failed = !comp
	}
	if !failed {
		r.streak = 0
		return false, Nothing
	}
	if r.streak == 0 {
		r.first = now
	}
	r.streak++
	if r.streak < r.Rounds || now.Sub(r.first) < r.Window {
		return true, Nothing
	}
	keep := r.kills[:0]
	for _, t := range r.kills {
		if now.Sub(t) < r.Per {
			keep = append(keep, t)
		}
	}
	r.kills = keep
	if len(r.kills) >= r.MaxKills {
		return true, Capped
	}
	r.kills = append(r.kills, now)
	r.streak = 0
	return true, Kill
}

func (r *Rule) Streak() (int, time.Time) { return r.streak, r.first }

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
	comp := flag.String("comp", "/run/dw/wayland-1", "compositor socket")
	hubd := flag.String("hubd", "/run/hubos/hubd.sock", "hubd socket")
	round := flag.Duration("round", time.Second, "time between rounds")
	mode := flag.String("mode", "both", "both: a round fails when BOTH probes fail (the owner's wording); compositor: when the compositor probe fails (a variation, see the README)")
	target := flag.String("target", "driftwm", "process name of the compositor to kill")
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
	r := &Rule{Rounds: 4, Window: 30 * time.Second, MaxKills: 3, Per: 10 * time.Minute, Both: *mode == "both"}
	logf("started pid=%d mode=%s (TEST STAND-IN, not the product watcher)", os.Getpid(), *mode)
	n := 0
	for range time.Tick(*round) {
		n++
		now := time.Now()
		c, h := Probe(*comp, "state"), Probe(*hubd, "ping")
		failed, d := r.Round(now, c, h)
		s, first := r.Streak()
		since := 0.0
		if s > 0 {
			since = now.Sub(first).Seconds()
		}
		logf("round %d comp=%s hubd=%s failed_round=%v streak=%d since_first=%.1fs", n, ok(c), ok(h), failed, s, since)
		switch d {
		case Kill:
			for _, p := range pidsNamed(*target) {
				err := syscall.Kill(p, syscall.SIGKILL)
				logf("KILL %s pid=%d err=%v", *target, p, err)
			}
		case Capped:
			logf("CAP: would kill %s, but 3 kills in the last 10 minutes were already made; not killing", *target)
		}
	}
}

func ok(b bool) string {
	if b {
		return "ok"
	}
	return "FAIL"
}
