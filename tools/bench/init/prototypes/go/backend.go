package main

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

// Backend is how the brain reaches the processes. s6Backend commands s6 (the per-service supervision stays in
// s6-supervise: if the brain dies, the services keep running). directBackend starts and watches the processes itself
// (if the brain dies its children become orphans of PID 1: see adopt).
type Backend interface {
	Status(name string) (up bool, pid int, last string)
	Start(name string)
	Stop(name string)
	Kill(name string)
	Restart(name string)
	Ready(name string) bool // up and has said it is ready
	Auto() bool             // true when the backend itself restarts a service that died
}

// ---------- s6 ----------

type s6Backend struct{ scan string }

var s6up = regexp.MustCompile(`^up \(pid (\d+)`)
var s6down = regexp.MustCompile(`^down \(([^)]*)\)`)

func (b s6Backend) Auto() bool { return true }
func (b s6Backend) Ready(name string) bool { // "up (pid N) S seconds, ready R seconds" once the notification arrived
	out, err := exec.Command("s6-svstat", b.scan+"/"+name).Output()
	return err == nil && strings.HasPrefix(string(out), "up") && strings.Contains(string(out), ", ready ")
}
func (b s6Backend) run(opt, name string) {
	exec.Command("s6-svc", opt, b.scan+"/"+name).Run()
}
func (b s6Backend) Status(name string) (bool, int, string) {
	out, err := exec.Command("s6-svstat", b.scan+"/"+name).Output()
	if err != nil {
		return false, 0, "no supervisor"
	}
	s := string(out)
	if m := s6up.FindStringSubmatch(s); m != nil {
		pid, _ := strconv.Atoi(m[1])
		return true, pid, ""
	}
	if m := s6down.FindStringSubmatch(s); m != nil {
		return false, 0, m[1]
	}
	return false, 0, strings.TrimSpace(s)
}
func (b s6Backend) Start(n string)   { b.run("-u", n) }
func (b s6Backend) Stop(n string)    { b.run("-d", n) }
func (b s6Backend) Kill(n string)    { b.run("-k", n) }
func (b s6Backend) Restart(n string) { b.run("-r", n) }

// ---------- direct ----------

type proc struct {
	pid     int
	ready   bool
	last    string
	adopted bool
	cmd     *exec.Cmd
	down    bool // stopped on purpose
}

type directBackend struct {
	mu    sync.Mutex
	procs map[string]*proc
	args  map[string][]string
}

func newDirect(args map[string][]string) *directBackend {
	return &directBackend{procs: map[string]*proc{}, args: args}
}

func (b *directBackend) Auto() bool { return false }
func (b *directBackend) Ready(name string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	p := b.procs[name]
	return p != nil && p.ready && p.last == ""
}

// adopt: after a brain restart the old children are still running. Take them over by name instead of killing them
// (killing driftwm would close every open window). Their exit status is unknown because PID 1 reaps them.
func (b *directBackend) adopt(names []string) {
	d, _ := os.ReadDir("/proc")
	for _, e := range d {
		pid, err := strconv.Atoi(e.Name())
		if err != nil || pid == os.Getpid() {
			continue
		}
		c, _ := os.ReadFile("/proc/" + e.Name() + "/comm")
		for _, n := range names {
			if strings.TrimSpace(string(c)) == n {
				b.procs[n] = &proc{pid: pid, ready: true, adopted: true}
			}
		}
	}
}

func alive(pid int) bool {
	st, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	return err == nil && !strings.Contains(string(st), ") Z")
}

func (b *directBackend) Status(name string) (bool, int, string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	p := b.procs[name]
	if p == nil {
		return false, 0, "not started"
	}
	if p.adopted {
		if alive(p.pid) {
			return true, p.pid, ""
		}
		return false, 0, "exit status unknown (adopted)"
	}
	if p.pid != 0 && p.last == "" {
		return true, p.pid, ""
	}
	return false, 0, p.last
}

func (b *directBackend) Start(name string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if p := b.procs[name]; p != nil && ((p.adopted && alive(p.pid)) || (!p.adopted && p.pid != 0 && p.last == "")) {
		return
	}
	r, w, _ := os.Pipe()
	os.MkdirAll("/var/log/"+name, 0o755)
	logf, _ := os.OpenFile("/var/log/"+name+"/current", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	cmd := exec.Command("/bin/"+name, b.args[name]...)
	cmd.Stdout, cmd.Stderr, cmd.ExtraFiles = logf, logf, []*os.File{w}
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		return
	}
	w.Close()
	p := &proc{pid: cmd.Process.Pid, cmd: cmd}
	b.procs[name] = p
	if name == "driftwm" || name == "hubd" {
		os.WriteFile(fmt.Sprintf("/proc/%d/oom_score_adj", p.pid), []byte("-900"), 0o644)
		os.MkdirAll("/sys/fs/cgroup/hub", 0o755)
		os.WriteFile("/sys/fs/cgroup/hub/cgroup.procs", []byte(fmt.Sprint(p.pid)), 0o644)
		syscall.Setpriority(syscall.PRIO_PROCESS, p.pid, -5)
	}
	go func() { // readiness line
		buf := make([]byte, 8)
		if n, _ := r.Read(buf); n > 0 {
			b.mu.Lock()
			p.ready = true
			b.mu.Unlock()
		}
		r.Close()
	}()
	go func() { // the exit status, as only a parent can see it
		err := cmd.Wait()
		logf.Close()
		b.mu.Lock()
		p.last = "exited"
		if ee, ok := err.(*exec.ExitError); ok {
			p.last = ee.ProcessState.String()
		}
		b.mu.Unlock()
	}()
	go func() { // keep the log file small
		for range time.Tick(10 * time.Second) {
			if st, err := os.Stat("/var/log/" + name + "/current"); err == nil && st.Size() > 100000 {
				os.Truncate("/var/log/"+name+"/current", 0)
			}
			b.mu.Lock()
			done := p.last != ""
			b.mu.Unlock()
			if done {
				return
			}
		}
	}()
}

func (b *directBackend) sig(name string, s syscall.Signal) {
	b.mu.Lock()
	p := b.procs[name]
	b.mu.Unlock()
	if p != nil && p.pid != 0 {
		syscall.Kill(p.pid, s)
	}
}
func (b *directBackend) Kill(n string) { b.sig(n, syscall.SIGKILL) }
func (b *directBackend) Stop(n string) {
	b.mu.Lock()
	if p := b.procs[n]; p != nil {
		p.down = true
	}
	b.mu.Unlock()
	b.sig(n, syscall.SIGKILL)
}
func (b *directBackend) Restart(n string) { b.sig(n, syscall.SIGTERM) }
