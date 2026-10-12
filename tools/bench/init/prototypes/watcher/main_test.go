package main

import (
	"encoding/binary"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func newRule() *Rule {
	return &Rule{Rounds: 4, Window: 30 * time.Second, MaxKills: 3, Per: 10 * time.Minute}
}

var t0 = time.Date(2026, 10, 12, 12, 0, 0, 0, time.UTC)

func at(i int) time.Time { return t0.Add(time.Duration(i) * time.Second) }

// no kill before 4 failed rounds AND 30 s; a kill at the first round that satisfies both
func TestKillNeedsFourRoundsAndThirtySeconds(t *testing.T) {
	r := newRule()
	for i := 0; i < 30; i++ {
		if _, d := r.Round(at(i), false, false); d != Nothing {
			t.Fatalf("decision %v at second %d", d, i)
		}
	}
	if _, d := r.Round(at(30), false, false); d != Kill {
		t.Fatalf("no kill at 30 s: %v", d)
	}
	// four failed rounds within 5 s are not enough
	r = newRule()
	for i := 0; i < 5; i++ {
		if _, d := r.Round(at(i), false, false); d != Nothing {
			t.Fatal("killed too early")
		}
	}
	// 30 s are not enough without 4 rounds (rounds 10 s apart: 4th round at 30 s is the first that has both)
	r = newRule()
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
		r := newRule()
		for i := 0; i < 300; i++ {
			failed, d := r.Round(at(i), c.state, c.sync)
			if failed || d != Nothing {
				t.Fatalf("state=%v sync=%v: failed=%v decision=%v at %d", c.state, c.sync, failed, d, i)
			}
		}
	}
	r := newRule()
	failed, _ := r.Round(at(0), false, false)
	if !failed {
		t.Fatal("both probes failing did not fail the round")
	}
}

// both probes failing for 30 s kills, even if one of them answers once in a while only after the streak is over
func TestBothFailingKills(t *testing.T) {
	r := newRule()
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
		r := newRule()
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
	r := newRule()
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
	_ = os.Remove
}
