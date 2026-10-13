package main

import (
	"os"
	"testing"
	"time"
)

// ---- follow-up 6, Part B: the latch clears by itself after a healthy stretch (fake clock: every Step gets the time as an argument) ----

const hour = 3600

// latchedRig: three kills, then the 4th wanted kill is latched (at about second 340); the compositor is a new process afterwards.
// ClearAfter is set to one hour. Returns the rig; the time of the last failed round is 340.
func latchedRig(t *testing.T, clear time.Duration) *rig {
	r := newRig(t)
	r.w.ClearAfter = clear
	r.threeKills()
	res := r.hang(300, 340)
	if count(res, ResCapped) == 0 {
		t.Fatalf("not latched: %v", res)
	}
	if l, _ := (Latch{r.lp}).State(); !l {
		t.Fatal("no latch")
	}
	return r
}

func (r *rig) latched() bool { l, _ := (Latch{r.lp}).State(); return l }

// healthy feeds rounds a..b (seconds, 1 s apart) in which both probes answer
func (r *rig) healthy(a, b int) {
	for i := a; i <= b; i++ {
		r.w.Step(at(i), true, true)
	}
}

func TestAutoClearNotAt59m59sClearedAt60m(t *testing.T) {
	r := latchedRig(t, hour*time.Second)
	budget, _ := os.ReadFile(r.bp)
	r.healthy(400, 400+hour-1) // the healthy stretch has lasted 59 min 59 s at its last round
	if !r.latched() {
		t.Fatal("cleared at 59 min 59 s")
	}
	r.healthy(400+hour, 400+hour) // 60 min
	if r.latched() {
		t.Fatal("not cleared at 60 min")
	}
	if !r.logged("LATCH CLEARED by itself") {
		t.Fatalf("no clear line: %v", r.logs)
	}
	after, _ := os.ReadFile(r.bp)
	if string(after) != string(budget) {
		t.Fatalf("the clear touched the budget file:\n%s\n---\n%s", budget, after)
	}
}

func TestAutoClearOneFailingProbeRestartsTheTimer(t *testing.T) {
	r := latchedRig(t, hour*time.Second)
	r.healthy(400, 2200)
	r.w.Step(at(2201), true, false) // the sync probe fails once: not a failed round, but not healthy either
	if !r.logged("healthy stretch broken") {
		t.Fatalf("no 'broken' line: %v", r.logs)
	}
	r.healthy(2202, 2202+hour-1)
	if !r.latched() {
		t.Fatal("cleared 59 min 59 s after the failed probe: the timer was not restarted")
	}
	r.healthy(2202+hour, 2202+hour)
	if r.latched() {
		t.Fatal("not cleared 60 min after the failed probe")
	}
	// and the other probe
	r2 := latchedRig(t, hour*time.Second)
	r2.healthy(400, 2200)
	r2.w.Step(at(2201), false, true)
	r2.healthy(2202, 2202+hour-1)
	if !r2.latched() {
		t.Fatal("state-probe variant: cleared too early")
	}
}

func TestAutoClearBothFailingKeepsTheLatchForever(t *testing.T) {
	r := latchedRig(t, hour*time.Second)
	for i := 341; i < 341+3*hour; i++ {
		r.w.Step(at(i), false, false)
	}
	if !r.latched() {
		t.Fatal("a latch with a compositor that is still failing was cleared")
	}
	// healthy for 59 min 59 s, then failing once more: the timer starts again
	r.healthy(20000, 20000+hour-1)
	r.w.Step(at(20000+hour), false, false)
	r.healthy(20001+hour, 20001+hour+hour-1)
	if !r.latched() {
		t.Fatal("cleared although a failed round fell inside the stretch")
	}
}

func TestAutoClearRestartRestartsTheTimer(t *testing.T) {
	r := latchedRig(t, hour*time.Second)
	r.healthy(400, 400+1800) // half an hour healthy, then the watcher is restarted
	r.start(at(2300))
	r.w.ClearAfter = hour * time.Second
	if !r.latched() {
		t.Fatal("latch file lost over the restart")
	}
	r.healthy(2301, 2301+hour-1)
	if !r.latched() {
		t.Fatal("cleared 59 min 59 s after the restart: the old timer was kept")
	}
	r.healthy(2301+hour, 2301+hour)
	if r.latched() {
		t.Fatal("not cleared 60 min after the restart")
	}
}

// a latch file left by a previous process (the watcher starts and finds it): cleared after a healthy hour counted from the first
// healthy round of this process, not before.
func TestAutoClearLatchLeftByAPreviousProcess(t *testing.T) {
	d := t.TempDir()
	r := &rig{t: t, dir: d, bp: d + "/watcher.budget", lp: d + "/watcher.latch", sp: d + "/watcher.state", procs: []Proc{{100, 5000}}}
	if err := (Latch{r.lp}).Set(at(5), time.Date(2026, 10, 13, 11, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	r.start(at(100000)) // a long time later, the watcher starts
	r.w.ClearAfter = hour * time.Second
	r.healthy(100000, 100000+hour-1)
	if !r.latched() {
		t.Fatal("cleared before a healthy hour from the watcher's start")
	}
	r.healthy(100000+hour, 100000+hour)
	if r.latched() {
		t.Fatal("not cleared after a healthy hour from the watcher's start")
	}
}

func TestAutoClearResetWorksAndStopsThePendingTimer(t *testing.T) {
	r := latchedRig(t, hour*time.Second)
	r.healthy(400, 2200)
	if err := Reset(r.bp, r.lp, r.sp); err != nil {
		t.Fatal(err)
	}
	r.healthy(2201, 2201)
	if r.latched() {
		t.Fatal("reset did not clear the latch")
	}
	if r.w.healthy {
		t.Fatal("the timer is still pending after reset")
	}
	// a new latch (written here by hand) must wait for its own full hour, counted from its first healthy round, not from 400
	if err := (Latch{r.lp}).Set(at(2202), r.w.Wall()); err != nil {
		t.Fatal(err)
	}
	r.healthy(2202, 2202+hour-1)
	if !r.latched() {
		t.Fatal("a new latch was cleared by the old timer")
	}
	r.healthy(2202+hour, 2202+hour)
	if r.latched() {
		t.Fatal("the new latch was not cleared after its own healthy hour")
	}
}

func TestAutoClearOffWhenZero(t *testing.T) {
	r := latchedRig(t, 0)
	r.healthy(400, 400+5*hour)
	if !r.latched() {
		t.Fatal("cleared although the setting is 0 (off)")
	}
}

// After the clear, with the budget aged out (the default setting: the kills are more than an hour old), a new hang is killed at the
// normal time (4 rounds and 30 s).
func TestAutoClearThenNewHangKilledAtNormalTime(t *testing.T) {
	r := latchedRig(t, hour*time.Second)
	r.healthy(400, 400+hour)
	if r.latched() {
		t.Fatal("not cleared")
	}
	r.newProc(300, 9000)
	base := 400 + hour + 10
	res := r.hang(base, base+35)
	if count(res, ResKilled) != 1 {
		t.Fatalf("new hang after the clear: %v", res)
	}
	// the kill came at the 31st second of the streak (round 31, 4 rounds and 30 s)
	for i, x := range res {
		if x == ResKilled && i != 30 {
			t.Fatalf("killed at streak second %d, not 30", i)
		}
	}
}

// With a short setting the kills are still inside the 10 minutes when the latch clears: the budget is full, so the next wanted kill
// latches again (the clear does not touch the budget). This is the behaviour with the shortened test setting; with the default hour the
// budget has aged out.
func TestAutoClearShortSettingBudgetStillFullLatchesAgain(t *testing.T) {
	r := latchedRig(t, 120*time.Second)
	r.healthy(400, 520)
	if r.latched() {
		t.Fatal("not cleared after 120 s")
	}
	r.newProc(300, 9000)
	res := r.hang(530, 570)
	if count(res, ResKilled) != 0 || count(res, ResCapped) == 0 || !r.latched() {
		t.Fatalf("expected latched again, got %v latched=%v", res, r.latched())
	}
}
