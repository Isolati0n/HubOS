package main

import (
	"bufio"
	"flag"
	"fmt"
	"net"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

// Svc is one managed service: what it needs, how to probe it, its ladder.
type Svc struct {
	Name       string
	Needs      []string
	Prober     *Prober
	L          *Ladder
	lastUp     bool
	expectDown bool
	held       bool // stopped because a dependency is down
	pid        int
	started    time.Time
	changed    time.Time            // when a new instance of this service was first seen
	syncedTo   map[string]time.Time // dependency instance this service was already restarted for
}

var (
	be     Backend
	graph  []*Svc
	byName = map[string]*Svc{}
	mu     sync.Mutex
	t0     = time.Now()
	nCrash int
)

// bench policy: far quicker than a real one would be, so a 25 s crash storm reaches the end of the ladder.
var pol = Policy{Window: 60 * time.Second, BackoffAfter: 2, DepsAfter: 4, GiveUpAfter: 6, Pause: time.Second}

func mk(name string, needs ...string) *Svc {
	s := &Svc{Name: name, Needs: needs, L: &Ladder{P: pol}, syncedTo: map[string]time.Time{}}
	byName[name] = s
	graph = append(graph, s)
	return s
}

func dependents(name string) (r []*Svc) {
	for _, s := range graph {
		for _, n := range s.Needs {
			if n == name {
				r = append(r, s)
			}
		}
	}
	return
}

func logf(f string, a ...any) { fmt.Printf("brain: "+f+"\n", a...) }

// crashRecord keeps the last words of a failed service where the recovery agent can serve them (capped to 20 files).
func crashRecord(s *Svc, cause, last string) {
	nCrash++
	os.MkdirAll("/var/log/crash", 0o755)
	tail := ""
	if b, err := os.ReadFile("/var/log/" + s.Name + "/current"); err == nil {
		l := strings.Split(strings.TrimSpace(string(b)), "\n")
		if len(l) > 8 {
			l = l[len(l)-8:]
		}
		tail = strings.Join(l, "\n")
	}
	body := fmt.Sprintf("service=%s time_since_start=%.0fs cause=%q backend_says=%q failures_in_window=%d\n%s\n", s.Name, time.Since(t0).Seconds(), cause, last, len(s.L.fails), tail)
	os.WriteFile(fmt.Sprintf("/var/log/crash/%s-%04d.txt", s.Name, nCrash%20), []byte(body), 0o644)
}

func writeAlert() {
	var l []string
	for _, s := range graph {
		if s.L.Step == Degraded {
			l = append(l, fmt.Sprintf("%s degraded: %s (retrying stopped; the owner must act)", s.Name, s.L.LastCause))
		}
	}
	if len(l) == 0 {
		os.Remove("/run/hubos/alert")
		return
	}
	os.WriteFile("/run/hubos/alert", []byte(strings.Join(l, "\n")+"\n"), 0o644)
}

func fail(s *Svc, now time.Time, cause, last string) {
	act := s.L.Fail(now, cause)
	crashRecord(s, cause, last)
	logf("%s failed (%s), failures=%d action=%d", s.Name, cause, len(s.L.fails), act)
	switch act {
	case StopForBackoff:
		be.Stop(s.Name)
		s.expectDown = true
	case RestartDeps:
		be.Stop(s.Name)
		s.expectDown = true
		for _, d := range dependents(s.Name) {
			be.Restart(d.Name)
		}
	case GiveUp:
		be.Stop(s.Name)
		s.expectDown = true
		writeAlert()
		logf("%s is DEGRADED: alert raised, everything else keeps running", s.Name)
	}
}

func tick(now time.Time) {
	for _, s := range graph {
		needsOK := true
		for _, n := range s.Needs {
			needsOK = needsOK && be.Ready(n)
		}
		up, pid, last := be.Status(s.Name)
		if up && pid != s.pid {
			if s.pid != 0 && s.lastUp && !s.held {
				fail(s, now, "restarted unseen", last)
			}
			s.pid, s.started, s.changed = pid, now, now
		}
		if !needsOK { // a dependency is down: hold this service (its failures are not its own); no polling script needed
			if up {
				logf("holding %s: a dependency is down", s.Name)
				be.Stop(s.Name)
			}
			s.held, s.expectDown, s.lastUp = true, true, up
			continue
		}
		if s.held {
			s.held, s.expectDown, s.lastUp = false, false, up
			if s.L.Step == Running {
				logf("starting %s: its dependencies are ready", s.Name)
				be.Start(s.Name)
			}
			continue
		}
		if !up && s.lastUp && !s.expectDown {
			fail(s, now, last, last)
		}
		s.lastUp = up
		if up && s.Prober != nil && s.Prober.Wedged(now.Sub(s.started)) {
			fail(s, now, "health probe: no answer", "")
			be.Kill(s.Name)
		}
		if s.L.Tick(now) {
			s.expectDown = false
			be.Start(s.Name)
		}
		if !be.Auto() && !up && s.L.Step == Running {
			s.expectDown = false
			be.Start(s.Name)
		}
	}
	// a dependent that is older than its dependency's newest instance is restarted (once per instance), unless it
	// restarted itself in the meantime. This replaces the polling script follow-driftwm.
	for _, s := range graph {
		up := s.lastUp && !s.started.IsZero()
		for _, n := range s.Needs {
			d := byName[n]
			if up && !s.held && !d.changed.IsZero() && s.started.Before(d.changed) && now.Sub(d.changed) > 3*time.Second && s.syncedTo[n] != d.changed && d.L.Step == Running {
				s.syncedTo[n] = d.changed
				logf("%s restarts because %s was restarted", s.Name, n)
				be.Restart(s.Name)
			}
		}
	}
	writeAlert()
}

func serve(path string) {
	os.Remove(path)
	l, err := net.Listen("unix", path)
	if err != nil {
		logf("status socket: %v", err)
		return
	}
	for {
		c, err := l.Accept()
		if err != nil {
			continue
		}
		go func() {
			defer c.Close()
			line, _ := bufio.NewReader(c).ReadString('\n')
			f := strings.Fields(line)
			mu.Lock()
			defer mu.Unlock()
			switch {
			case len(f) > 0 && f[0] == "status":
				names := make([]string, 0)
				for n := range byName {
					names = append(names, n)
				}
				sort.Strings(names)
				for _, n := range names {
					s := byName[n]
					ago := "never"
					if !s.L.LastFail.IsZero() {
						ago = fmt.Sprintf("%.0fs", time.Since(s.L.LastFail).Seconds())
					}
					fmt.Fprintf(c, "%s state=%s up=%v pid=%d restarts=%d last_failure=%s needs=%v cause=%q\n", n, s.L.Step, s.lastUp, s.pid, s.L.Restarts, ago, s.Needs, s.L.LastCause)
				}
			case len(f) == 2 && f[0] == "crashtest": // TEST HOOK: a bug in the policy code. In Go an unrecovered panic in any goroutine ends the whole program.
				go func() { panic("test hook: bug in the policy code") }()
				fmt.Fprintln(c, "ok (panicking)")
			case len(f) == 2 && f[0] == "retry" && byName[f[1]] != nil:
				s := byName[f[1]]
				s.L.Retry()
				s.expectDown = false
				be.Start(s.Name)
				writeAlert()
				fmt.Fprintln(c, "ok")
			default:
				fmt.Fprintln(c, "error: status | retry NAME")
			}
		}()
	}
}

func ctl(path string, args []string) {
	c, err := net.DialTimeout("unix", path, 2*time.Second)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	fmt.Fprintln(c, strings.Join(args, " "))
	bufio.NewReader(c).WriteTo(os.Stdout)
}

func main() {
	mode := flag.String("mode", "s6", "s6: command s6; direct: start and watch the services itself")
	scan := flag.String("scan", "/run/service", "s6 scan directory")
	sock := flag.String("sock", "/run/hubos/brain.sock", "status socket")
	flag.Parse()
	if flag.Arg(0) == "ctl" {
		ctl(*sock, flag.Args()[1:])
		return
	}
	mk("seatd")
	mk("udevd")
	mk("dbus")
	dw := mk("driftwm", "seatd", "udevd")
	dw.Prober = &Prober{Sock: "/run/dw/wayland-1", Request: "state", Grace: 8 * time.Second, Need: 3}
	mk("waybar", "driftwm", "dbus")
	mk("hubd", "driftwm")
	os.MkdirAll("/run/hubos", 0o755)
	if *mode == "direct" {
		d := newDirect(map[string][]string{"seatd": {"-log", "2", "-notify", "3"}, "udevd": {"-log", "2", "-notify", "3"}, "dbus": {"-log", "2", "-notify", "3"},
			"driftwm": {"--backend", "udev", "-mem", "100", "-notify", "3"}, "waybar": {"-notify", "3"}, "hubd": {"-notify", "3"}})
		var names []string
		for _, s := range graph {
			names = append(names, s.Name)
		}
		d.adopt(names)
		be = d
	} else {
		be = s6Backend{scan: *scan}
	}
	logf("started mode=%s pid=%d", *mode, os.Getpid())
	go serve(*sock)
	for range time.Tick(time.Second) {
		mu.Lock()
		tick(time.Now())
		mu.Unlock()
	}
}
