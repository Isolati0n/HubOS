package main

import (
	"encoding/binary"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// newRule: the owner's numbers, with a budget kept in a temporary file (as the program does, with the boot clock).
func newRule(t *testing.T) *Rule {
	b, _ := LoadBudget(filepath.Join(t.TempDir(), "budget"), 0, 3, 10*time.Minute)
	return &Rule{Rounds: 4, Window: 30 * time.Second, B: b}
}

// at(i): i seconds after some boot-clock reading far from zero
func at(i int) time.Duration { return 1000*time.Second + time.Duration(i)*time.Second }

// no kill before 4 failed rounds AND 30 s; a kill at the first round that satisfies both
func TestKillNeedsFourRoundsAndThirtySeconds(t *testing.T) {
	r := newRule(t)
	for i := 0; i < 30; i++ {
		if _, d := r.Round(at(i), false, false); d != Nothing {
			t.Fatalf("decision %v at second %d", d, i)
		}
	}
	if _, d := r.Round(at(30), false, false); d != Kill {
		t.Fatalf("no kill at 30 s: %v", d)
	}
	// four failed rounds within 5 s are not enough
	r = newRule(t)
	for i := 0; i < 5; i++ {
		if _, d := r.Round(at(i), false, false); d != Nothing {
			t.Fatal("killed too early")
		}
	}
	// 30 s are not enough without 4 rounds (rounds 10 s apart: 4th round at 30 s is the first that has both)
	r = newRule(t)
	for _, s := range []int{0, 15, 30} {
		if _, d := r.Round(at(s), false, false); d != Nothing {
			t.Fatalf("killed with fewer than 4 rounds at %d s", s)
		}
	}
	if _, d := r.Round(at(31), false, false); d != Kill {
		t.Fatal("no kill at the 4th round after 30 s")
	}
}

// a round fails only when BOTH compositor probes fail: one probe failing does not fail the round
func TestOneProbeFailingDoesNotFailTheRound(t *testing.T) {
	for _, c := range []struct{ state, sync bool }{{false, true}, {true, false}} {
		r := newRule(t)
		for i := 0; i < 300; i++ {
			failed, d := r.Round(at(i), c.state, c.sync)
			if failed || d != Nothing {
				t.Fatalf("state=%v sync=%v: failed=%v decision=%v at %d", c.state, c.sync, failed, d, i)
			}
		}
	}
	r := newRule(t)
	failed, _ := r.Round(at(0), false, false)
	if !failed {
		t.Fatal("both probes failing did not fail the round")
	}
}

// both probes failing for 30 s kills, even if one of them answers once in a while only after the streak is over
func TestBothFailingKills(t *testing.T) {
	r := newRule(t)
	kill := false
	for i := 0; i < 40; i++ {
		if _, d := r.Round(at(i), false, false); d == Kill {
			kill = true
		}
	}
	if !kill {
		t.Fatal("both probes failing for 40 s did not kill")
	}
}

// an answer from either probe resets the streak
func TestAnswerResetsStreak(t *testing.T) {
	for _, ans := range []struct{ state, sync bool }{{true, true}, {true, false}, {false, true}} {
		r := newRule(t)
		for i := 0; i < 25; i++ {
			r.Round(at(i), false, false)
		}
		r.Round(at(25), ans.state, ans.sync)
		if s, _ := r.Streak(); s != 0 {
			t.Fatalf("streak %d after an answer %+v", s, ans)
		}
		for i := 26; i < 55; i++ {
			if _, d := r.Round(at(i), false, false); d != Nothing {
				t.Fatalf("kill at %d after an answer", i)
			}
		}
	}
}

// at most 3 kills in 10 minutes; the fourth is capped; after the oldest is older than 10 minutes a kill is allowed again
func TestThreeKillsInTenMinutes(t *testing.T) {
	r := newRule(t)
	kills, capped := 0, 0
	var killAt []int
	for i := 0; i < 700; i++ {
		_, d := r.Round(at(i), false, false)
		switch d {
		case Kill:
			kills++
			killAt = append(killAt, i)
		case Capped:
			capped++
		}
	}
	if len(killAt) < 4 || killAt[0] != 30 || killAt[1] != 61 || killAt[2] != 92 {
		t.Fatalf("kills at %v", killAt)
	}
	if killAt[3] != 630 { // the first kill (second 30) is 600 s old at second 630
		t.Fatalf("fourth kill at %d, want 630 (budget back 10 minutes after the first kill)", killAt[3])
	}
	if capped == 0 {
		t.Fatal("never capped")
	}
}

// ProbeSync against a fake server that speaks the wire format: done is answered, silence fails
func TestProbeSyncWire(t *testing.T) {
	dir := t.TempDir()
	good, silent := filepath.Join(dir, "good"), filepath.Join(dir, "silent")
	serve := func(path string, answer bool) {
		l, err := net.Listen("unix", path)
		if err != nil {
			t.Fatal(err)
		}
		go func() {
			for {
				c, err := l.Accept()
				if err != nil {
					return
				}
				go func() {
					defer c.Close()
					var m [12]byte
					if _, err := c.Read(m[:]); err != nil {
						return
					}
					if binary.LittleEndian.Uint32(m[0:]) != 1 || binary.LittleEndian.Uint32(m[4:]) != 12<<16 {
						return
					}
					if !answer {
						time.Sleep(3 * time.Second)
						return
					}
					var out [24]byte
					binary.LittleEndian.PutUint32(out[0:], 1) // delete_id first: must be skipped
					binary.LittleEndian.PutUint32(out[4:], 12<<16|1)
					binary.LittleEndian.PutUint32(out[8:], 2)
					binary.LittleEndian.PutUint32(out[12:], 2) // callback.done
					binary.LittleEndian.PutUint32(out[16:], 12<<16|0)
					binary.LittleEndian.PutUint32(out[20:], 7)
					c.Write(out[:])
				}()
			}
		}()
	}
	serve(good, true)
	serve(silent, false)
	if !ProbeSync(good) {
		t.Fatal("sync not answered")
	}
	if ProbeSync(silent) {
		t.Fatal("silent server counted as answering")
	}
	if ProbeSync(filepath.Join(dir, "none")) {
		t.Fatal("missing socket counted as answering")
	}
}

// ---- item 2: the kill budget in a file ----

func bpath(t *testing.T) string { return filepath.Join(t.TempDir(), "watcher.budget") }

// the budget survives a restart of the watcher: a second program that reads the file knows the kills of the first
func TestBudgetSurvivesRestart(t *testing.T) {
	p := bpath(t)
	b, note := LoadBudget(p, at(0), 3, 10*time.Minute)
	if b.Len() != 0 || !strings.Contains(note, "missing") {
		t.Fatalf("fresh start: %d %q", b.Len(), note)
	}
	r := &Rule{Rounds: 4, Window: 30 * time.Second, B: b}
	kills := 0
	for i := 0; i < 400; i++ {
		if _, d := r.Round(at(i), false, false); d == Kill {
			kills++
			// a restart between the kills: new budget object, new rule, streak lost, kills kept
			b2, note := LoadBudget(p, at(i), 3, 10*time.Minute)
			if b2.Len() != kills || !strings.Contains(note, "kept") {
				t.Fatalf("after kill %d the restarted watcher sees %d kills (%q)", kills, b2.Len(), note)
			}
			r = &Rule{Rounds: 4, Window: 30 * time.Second, B: b2}
		}
	}
	if kills != 3 {
		t.Fatalf("%d kills in 400 s with restarts between them, want 3 and then the cap", kills)
	}
	// the fourth hang after a restart is capped, not killed
	b4, _ := LoadBudget(p, at(400), 3, 10*time.Minute)
	r4 := &Rule{Rounds: 4, Window: 30 * time.Second, B: b4}
	var last Decision
	for i := 400; i < 450; i++ {
		_, last = r4.Round(at(i), false, false)
	}
	if last != Capped {
		t.Fatalf("decision after restart: %v, want Capped", last)
	}
	// and the budget returns: 10 minutes after the first kill (second 30) the restarted watcher may kill again
	b5, _ := LoadBudget(p, at(631), 3, 10*time.Minute)
	if b5.Full(at(631)) {
		t.Fatal("budget still full 601 s after the first kill")
	}
}

// a damaged file does not crash the watcher: empty budget and a note
func TestBudgetDamagedFile(t *testing.T) {
	good := budgetHeader + "\nkill 1000000000000\nend 1\n"
	for name, content := range map[string]string{
		"empty":            "",
		"binary":           "\x00\xff\x01garbage",
		"bad header":       "something else\nend 0\n",
		"cut off":          budgetHeader + "\nkill 1000000000000\n",
		"count wrong":      budgetHeader + "\nkill 1000000000000\nend 2\n",
		"not a time":       budgetHeader + "\nkill abc\nend 1\n",
		"negative":         budgetHeader + "\nkill -5\nend 1\n",
		"future":           budgetHeader + "\nkill 99999999999999999\nend 1\n",
		"text after end":   good + "kill 1000000000000\n",
		"unknown line":     budgetHeader + "\nhello\nend 0\n",
		"very long line":   budgetHeader + "\nkill " + strings.Repeat("9", 100000) + "\nend 1\n",
		"too many entries": budgetHeader + "\n" + strings.Repeat("kill 1000000000000\n", 2000) + "end 2000\n",
	} {
		p := bpath(t)
		os.WriteFile(p, []byte(content), 0o644)
		b, note := LoadBudget(p, 1001*time.Second, 3, 10*time.Minute)
		if b == nil || b.Len() != 0 || !strings.Contains(note, "damaged") || !strings.Contains(note, "empty budget") {
			t.Errorf("%s: budget %v note %q", name, b, note)
		}
	}
	// the good file loads
	p := bpath(t)
	os.WriteFile(p, []byte(good), 0o644)
	if b, note := LoadBudget(p, 1001*time.Second, 3, 10*time.Minute); b.Len() != 1 {
		t.Errorf("good file: %d %q", b.Len(), note)
	}
	// a directory where the file should be, and a file in an unwritable place, do not crash
	if b, _ := LoadBudget(t.TempDir(), 0, 3, time.Minute); b == nil || b.Len() != 0 {
		t.Error("directory as file")
	}
	b, _ := LoadBudget("/proc/nonexistent/budget", 0, 3, time.Minute)
	b.Add(at(0))
	if b.SaveErr == nil || b.Len() != 1 {
		t.Error("a failing write must be reported and still count in memory")
	}
}

// entries older than 10 minutes drop out, at load and while running
func TestBudgetOldEntriesDropOut(t *testing.T) {
	p := bpath(t)
	b, _ := LoadBudget(p, at(0), 3, 10*time.Minute)
	b.Add(at(0))
	b.Add(at(100))
	b.Add(at(200))
	if !b.Full(at(300)) {
		t.Fatal("not full with 3 kills")
	}
	if b.Full(at(601)) { // the first kill (at 0) is 601 s old
		t.Fatal("first kill did not drop out after 10 minutes")
	}
	if b.Len() != 2 {
		t.Fatalf("%d entries, want 2", b.Len())
	}
	// exactly 600 s old is out too (the rule is "in the last 10 minutes")
	b2, _ := LoadBudget(p, at(600), 3, 10*time.Minute)
	if b2.Len() != 2 {
		t.Fatalf("at load, 600 s after the first kill: %d entries, want 2", b2.Len())
	}
	// at load, everything older than 10 minutes is dropped, and the note says how many
	b3, note := LoadBudget(p, at(5000), 3, 10*time.Minute)
	if b3.Len() != 0 || !strings.Contains(note, "3 older dropped") {
		t.Fatalf("%d entries, %q", b3.Len(), note)
	}
	// the file is written without a temporary file left behind
	if _, err := os.Stat(p + ".tmp"); err == nil {
		t.Fatal("temporary file left behind")
	}
}

// the state file: written when the cap blocks a kill, removed when kills are allowed again
func TestStateFile(t *testing.T) {
	sp := filepath.Join(t.TempDir(), "watcher.state")
	sf := StateFile{Path: sp}
	if sf.Exists() {
		t.Fatal("exists before")
	}
	if err := sf.SetCapped(time.Date(2026, 10, 12, 12, 30, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(sp)
	if string(b) != "capped 2026-10-12T12:30:00Z\n" {
		t.Fatalf("content %q", b)
	}
	if err := sf.Clear(); err != nil || sf.Exists() {
		t.Fatal("not removed")
	}
	if err := sf.Clear(); err != nil { // removing what is not there is fine
		t.Fatal(err)
	}
}
