package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// ---- follow-up 5: latch, identity rule, probe limits ----

// rig is a Watcher on temporary files with a fake compositor process list and a fake kill.
type rig struct {
	t               *testing.T
	dir, bp, lp, sp string
	w               *Watcher
	procs           []Proc // what Find returns
	killed          []Proc
	killErr         error
	logs            []string
}

func newRig(t *testing.T) *rig {
	d := t.TempDir()
	r := &rig{t: t, dir: d, bp: filepath.Join(d, "watcher.budget"), lp: filepath.Join(d, "watcher.latch"), sp: filepath.Join(d, "watcher.state"),
		procs: []Proc{{100, 5000}}}
	r.start(at(0))
	return r
}

// start makes a new watcher process object on the same files (a restart: streak and identity are lost, files are kept)
func (r *rig) start(now time.Duration) {
	b, _ := LoadBudget(r.bp, now, 3, 10*time.Minute)
	r.w = &Watcher{
		R: &Rule{Rounds: 4, Window: 30 * time.Second, B: b}, B: b, Latch: Latch{r.lp}, State: StateFile{r.sp},
		Find: func() []Proc { return r.procs },
		Kill: func(p Proc) error {
			if r.killErr != nil {
				return r.killErr
			}
			r.killed = append(r.killed, p)
			return nil
		},
		Wall:   func() time.Time { return time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC) },
		Log:    func(f string, a ...any) { r.logs = append(r.logs, fmt.Sprintf(f, a...)) },
		Target: "driftwm",
	}
}

func (r *rig) logged(sub string) bool {
	for _, l := range r.logs {
		if strings.Contains(l, sub) {
			return true
		}
	}
	return false
}

// hang feeds failed rounds from second a to second b (inclusive) and returns the results seen
func (r *rig) hang(a, b int) (res []Result) {
	for i := a; i <= b; i++ {
		res = append(res, r.w.Step(at(i), false, false))
	}
	return
}

func count(res []Result, x Result) (n int) {
	for _, y := range res {
		if y == x {
			n++
		}
	}
	return
}

// makeNewProc: the init restarted the compositor: another pid and start time
func (r *rig) newProc(pid int, start uint64) { r.procs = []Proc{{pid, start}} }

// three hangs, each killed (the process is replaced after each kill, as the init does), then the 4th is latched
func (r *rig) threeKills() {
	for k := 0; k < 3; k++ {
		base := k * 100
		res := r.hang(base, base+35)
		if count(res, ResKilled) != 1 {
			r.t.Fatalf("hang %d: results %v", k+1, res)
		}
		r.newProc(200+k, uint64(6000+k))
		r.w.Step(at(base+40), true, true) // answers again
	}
}

func TestLatchSetWhenFourthKillWanted(t *testing.T) {
	r := newRig(t)
	r.threeKills()
	if l, _ := (Latch{r.lp}).State(); l {
		t.Fatal("latch set before the 4th kill was wanted")
	}
	res := r.hang(300, 340)
	if count(res, ResKilled) != 0 || count(res, ResCapped) == 0 {
		t.Fatalf("4th hang: %v", res)
	}
	if l, dmg := (Latch{r.lp}).State(); !l || dmg != "" {
		t.Fatalf("latch %v %q", l, dmg)
	}
	if !r.logged("LATCH set") {
		t.Fatal("no log line")
	}
	if len(r.killed) != 3 {
		t.Fatalf("%d kills", len(r.killed))
	}
	b, err := os.ReadFile(r.sp)
	if err != nil || !strings.HasPrefix(string(b), "capped 2026-10-13T12:00:00Z") {
		t.Fatalf("state file %q %v", b, err)
	}
}

func TestNoKillWhileLatchedEvenAfterTenMinutes(t *testing.T) {
	r := newRig(t)
	r.threeKills()
	r.hang(300, 340)             // latched
	res := r.hang(341, 341+2400) // 40 more minutes of failing rounds; the budget would be free again after 10
	if count(res, ResKilled) != 0 {
		t.Fatalf("killed %d times while latched", count(res, ResKilled))
	}
	if len(r.killed) != 3 {
		t.Fatalf("%d kills", len(r.killed))
	}
	if !r.w.State.Exists() {
		t.Fatal("state file gone while latched and failing")
	}
	if r.w.B.Full(at(341 + 2400)) {
		t.Fatal("budget should have room by now (the point of the test: the latch, not the budget, blocks)")
	}
}

func TestLatchSurvivesRestart(t *testing.T) {
	r := newRig(t)
	r.threeKills()
	r.hang(300, 340)
	r.start(at(341)) // the watcher is killed and restarted: same files
	if r.w.B.Len() != 3 {
		t.Fatalf("budget %d", r.w.B.Len())
	}
	res := r.hang(342, 342+700)
	if count(res, ResKilled) != 0 || len(r.killed) != 3 {
		t.Fatalf("killed after restart: %v", res)
	}
	if !r.w.State.Exists() {
		t.Fatal("state file lost over restart")
	}
	if l, _ := (Latch{r.lp}).State(); !l {
		t.Fatal("latch lost")
	}
}

func TestResetClearsLatchAndBudgetAndTheWatcherKillsThen(t *testing.T) {
	r := newRig(t)
	r.threeKills()
	r.hang(300, 340)
	if err := Reset(r.bp, r.lp, r.sp); err != nil {
		t.Fatal(err)
	}
	if l, _ := (Latch{r.lp}).State(); l || r.w.State.Exists() {
		t.Fatal("latch or state file still there after reset")
	}
	b, _ := LoadBudget(r.bp, at(341), 3, 10*time.Minute)
	if b.Len() != 0 {
		t.Fatalf("budget has %d entries after reset", b.Len())
	}
	// the running watcher notices within one round, and kills (the streak is long enough already)
	if res := r.w.Step(at(341), false, false); res != ResKilled {
		t.Fatalf("round after reset: %v", res)
	}
	if !r.logged("budget file changed by someone else") || len(r.killed) != 4 {
		t.Fatalf("reload not logged or kills=%d", len(r.killed))
	}
	// a second reset when nothing is latched is harmless
	if err := Reset(r.bp, r.lp, r.sp); err != nil {
		t.Fatal(err)
	}
}

func TestStateFileAppearsAndDisappears(t *testing.T) {
	r := newRig(t)
	r.threeKills()
	if r.w.State.Exists() {
		t.Fatal("state file before the cap")
	}
	r.hang(300, 340)
	if !r.w.State.Exists() {
		t.Fatal("no state file while capped")
	}
	// one probe answering is not "both answer": file stays, and the streak is over
	r.w.Step(at(341), true, false)
	if !r.w.State.Exists() {
		t.Fatal("state file removed by a round in which only one probe answered")
	}
	r.w.Step(at(342), false, false)
	// both answer: gone in that very round, latch stays
	if res := r.w.Step(at(343), true, true); res != ResIdle || r.w.State.Exists() {
		t.Fatal("state file not removed in the first round with both probes answering")
	}
	if l, _ := (Latch{r.lp}).State(); !l {
		t.Fatal("latch cleared by the state file's removal")
	}
	// hang again: not killed, state file comes back (the rule wants a kill after its 30 s)
	res := r.hang(400, 440)
	if count(res, ResKilled) != 0 || count(res, ResCapped) == 0 || !r.w.State.Exists() {
		t.Fatalf("hang while latched: %v exists=%v", res, r.w.State.Exists())
	}
	// reset removes the file
	Reset(r.bp, r.lp, r.sp)
	if r.w.State.Exists() {
		t.Fatal("state file after reset")
	}
}

func TestIdentityRule(t *testing.T) {
	cases := []struct {
		name string
		now  []Proc // the process list at the kill moment
		kill bool
	}{
		{"same pid and start time", []Proc{{100, 5000}}, true},
		{"same pid, different start time", []Proc{{100, 5999}}, false},
		{"different pid", []Proc{{101, 5000}}, false},
		{"no process", nil, false},
		{"the failing one among others", []Proc{{7, 1}, {100, 5000}}, true},
	}
	for _, c := range cases {
		r := newRig(t)
		r.hang(0, 5) // first failed round records pid 100 start 5000
		r.procs = c.now
		res := r.hang(6, 30)
		killed := count(res, ResKilled) == 1
		if killed != c.kill {
			t.Errorf("%s: killed=%v want %v (%v)", c.name, killed, c.kill, res)
		}
		if c.kill {
			if len(r.killed) != 1 || r.killed[0] != (Proc{100, 5000}) || r.w.B.Len() != 1 {
				t.Errorf("%s: kills %v budget %d", c.name, r.killed, r.w.B.Len())
			}
			continue
		}
		// not killed, not counted, logged, and a new streak with the new process starts at the kill moment
		if len(r.killed) != 0 || r.w.B.Len() != 0 || count(res, ResSkipped) != 1 || !r.logged("SKIP: process changed") {
			t.Errorf("%s: kills=%d budget=%d skipped=%d", c.name, len(r.killed), r.w.B.Len(), count(res, ResSkipped))
		}
		if b, _ := os.ReadFile(r.bp); strings.Contains(string(b), "kill ") {
			t.Errorf("%s: budget file has a kill", c.name)
		}
		if l, _ := (Latch{r.lp}).State(); l {
			t.Errorf("%s: latch set by a skip", c.name)
		}
		// the new process (if any) gets its own 30 s: no kill for 29 more seconds, then the kill
		if len(c.now) == 0 {
			continue
		}
		want := c.now[len(c.now)-1]
		if c.name == "the failing one among others" {
			continue
		}
		res2 := r.hang(31, 59)
		if count(res2, ResKilled) != 0 {
			t.Errorf("%s: new process killed before its own 30 s", c.name)
		}
		if res3 := r.hang(60, 62); count(res3, ResKilled) != 1 || r.killed[0] != want {
			t.Errorf("%s: new process not killed after its 30 s: %v killed=%v", c.name, res3, r.killed)
		}
	}
}

// the init restarts the compositor every few seconds (a crash storm): nothing is killed, nothing counted
func TestCrashStormKillsNothing(t *testing.T) {
	r := newRig(t)
	for i := 0; i < 40; i++ {
		r.newProc(300+i, uint64(7000+i))
		r.w.Step(at(i), false, false)
	}
	if len(r.killed) != 0 || r.w.B.Len() != 0 {
		t.Fatalf("storm: %d kills, budget %d", len(r.killed), r.w.B.Len())
	}
}

// a kill that finds no process (it died between the check and the signal) is taken back
func TestKillThatFindsNoProcessIsNotCounted(t *testing.T) {
	r := newRig(t)
	r.killErr = syscall.ESRCH
	res := r.hang(0, 30)
	if count(res, ResKilled) != 0 || r.w.B.Len() != 0 || count(res, ResSkipped) != 1 {
		t.Fatalf("%v budget %d", res, r.w.B.Len())
	}
	if b, _ := os.ReadFile(r.bp); strings.Contains(string(b), "kill ") {
		t.Fatal("budget file keeps the kill")
	}
	// the real kill function refuses pid 0 and a wrong start time
	if killProc(Proc{}) == nil || killProc(Proc{-5, 1}) == nil {
		t.Fatal("killProc accepted pid <= 0")
	}
	if killProc(Proc{os.Getpid(), 1}) == nil { // our own pid with a start time that is not ours: must not kill us
		t.Fatal("killProc killed a process with another start time")
	}
}

// the 30-second streak does not survive a restart
func TestStreakNotKeptOverRestart(t *testing.T) {
	r := newRig(t)
	r.hang(0, 20)
	r.start(at(21))
	res := r.hang(21, 50) // 29 s since the new watcher's first failed round
	if count(res, ResKilled) != 0 {
		t.Fatalf("killed %v", res)
	}
	if res := r.hang(51, 52); count(res, ResKilled) != 1 {
		t.Fatalf("no kill 30 s after the restart: %v", res)
	}
}

// a damaged latch file does not crash the watcher: it counts as latched (nothing is killed) and is logged
func TestDamagedLatchFile(t *testing.T) {
	for name, content := range map[string]string{
		"empty": "", "binary": "\x00\xff\x01", "bad header": "other\nboot_ns 1\nutc x\nend\n", "cut off": latchHeader + "\nboot_ns 5\n",
	} {
		r := newRig(t)
		os.WriteFile(r.lp, []byte(content), 0o644)
		res := r.hang(0, 100)
		if count(res, ResKilled) != 0 || len(r.killed) != 0 {
			t.Errorf("%s: killed while the latch file is damaged", name)
		}
		if !r.logged("damaged") || !r.logged("treated as latched") || count(res, ResCapped) == 0 {
			t.Errorf("%s: not logged or not capped (%v)", name, r.logs[len(r.logs)-1])
		}
	}
	// a directory where the latch should be: also the safe side
	r := newRig(t)
	os.Mkdir(r.lp, 0o755)
	if res := r.hang(0, 40); count(res, ResKilled) != 0 {
		t.Error("killed with a directory as latch")
	}
	// reading never crashes on a big file
	r = newRig(t)
	os.WriteFile(r.lp, []byte(strings.Repeat("x", 1<<20)), 0o644)
	r.hang(0, 40)
}

// probe limits: state answers within 5 s (after 4 s it counts as answered); sync has 2 s (after 3 s it fails)
func TestProbeLimits(t *testing.T) {
	dir := t.TempDir()
	sock := filepath.Join(dir, "slow")
	l, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}
			go func() {
				defer c.Close()
				br := bufio.NewReader(c)
				b, _ := br.Peek(1)
				if len(b) == 1 && b[0] == 0x01 { // wl_display.sync after 3 s
					var m [12]byte
					if _, err := readFull(br, m[:]); err != nil {
						return
					}
					time.Sleep(3 * time.Second)
					c.Write([]byte{2, 0, 0, 0, 0, 0, 12, 0, 9, 0, 0, 0}) // object 2, size 12, opcode 0, serial
					return
				}
				br.ReadString('\n')
				time.Sleep(4 * time.Second) // state after 4 s
				c.Write([]byte("ok slow\n"))
			}()
		}
	}()
	t0 := time.Now()
	if !ProbeLimit(sock, "state", StateLimit) {
		t.Error("a state answer after 4 s must count as answered (deadline 5 s)")
	}
	if d := time.Since(t0); d < 3900*time.Millisecond || d > 5*time.Second {
		t.Errorf("took %v", d)
	}
	t0 = time.Now()
	if ProbeSync(sock) {
		t.Error("a sync answer after 3 s must count as failed (limit 2 s)")
	}
	if d := time.Since(t0); d < 1900*time.Millisecond || d > 2500*time.Millisecond {
		t.Errorf("sync gave up after %v, want about 2 s", d)
	}
	if StateLimit != 5*time.Second || SyncLimit != 2*time.Second {
		t.Error("limits are not the document's numbers")
	}
	// and the same server answers a patient sync (limit 4 s): proves the fake itself works
	if !ProbeSyncLimit(sock, 4*time.Second) {
		t.Error("fake server does not answer the sync")
	}
}

// one probe answering (slow but alive) never kills, even for a long time
func TestSlowButAliveNeverKills(t *testing.T) {
	for _, c := range []struct{ state, sync bool }{{true, false}, {false, true}} {
		r := newRig(t)
		for i := 0; i < 600; i++ {
			if res := r.w.Step(at(i), c.state, c.sync); res != ResIdle {
				t.Fatalf("%+v: %v at %d", c, res, i)
			}
		}
		if len(r.killed) != 0 {
			t.Fatal("killed")
		}
	}
}
