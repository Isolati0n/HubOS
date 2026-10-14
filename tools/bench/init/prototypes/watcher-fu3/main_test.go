package main

import (
	"testing"
	"time"
)

func newRule(both bool) *Rule {
	return &Rule{Rounds: 4, Window: 30 * time.Second, MaxKills: 3, Per: 10 * time.Minute, Both: both}
}

var t0 = time.Date(2026, 10, 12, 12, 0, 0, 0, time.UTC)

// no kill before 4 failed rounds AND 30 s; a kill at the first round that satisfies both
func TestKillNeedsFourRoundsAndThirtySeconds(t *testing.T) {
	r := newRule(true)
	for i := 0; i < 30; i++ {
		if _, d := r.Round(t0.Add(time.Duration(i)*time.Second), false, false); d != Nothing {
			t.Fatalf("decision %v at second %d", d, i)
		}
	}
	if _, d := r.Round(t0.Add(30*time.Second), false, false); d != Kill {
		t.Fatalf("no kill at 30 s: %v", d)
	}
	// four failed rounds within 5 s are not enough
	r = newRule(true)
	for i := 0; i < 5; i++ {
		if _, d := r.Round(t0.Add(time.Duration(i)*time.Second), false, false); d != Nothing {
			t.Fatal("killed too early")
		}
	}
}

// one probe answering resets the streak when both must fail
func TestBothMustFail(t *testing.T) {
	r := newRule(true)
	for i := 0; i < 120; i++ {
		if _, d := r.Round(t0.Add(time.Duration(i)*time.Second), false, true); d != Nothing {
			t.Fatal("killed with hubd answering")
		}
	}
	r = newRule(false) // the variation: the compositor probe alone
	kill := false
	for i := 0; i < 40; i++ {
		if _, d := r.Round(t0.Add(time.Duration(i)*time.Second), false, true); d == Kill {
			kill = true
		}
	}
	if !kill {
		t.Fatal("variation did not kill")
	}
}

func TestAnswerResetsStreak(t *testing.T) {
	r := newRule(true)
	for i := 0; i < 25; i++ {
		r.Round(t0.Add(time.Duration(i)*time.Second), false, false)
	}
	r.Round(t0.Add(25*time.Second), true, true)
	for i := 26; i < 55; i++ {
		if _, d := r.Round(t0.Add(time.Duration(i)*time.Second), false, false); d != Nothing {
			t.Fatalf("kill at %d after a good round", i)
		}
	}
}

// at most 3 kills in 10 minutes; the fourth is capped; after the oldest is older than 10 minutes a kill is allowed again
func TestThreeKillsInTenMinutes(t *testing.T) {
	r := newRule(true)
	now := t0
	kills := 0
	capped := 0
	for i := 0; i < 600; i++ {
		now = t0.Add(time.Duration(i) * time.Second)
		_, d := r.Round(now, false, false)
		switch d {
		case Kill:
			kills++
		case Capped:
			capped++
		}
	}
	if kills != 3 {
		t.Fatalf("kills in 10 minutes: %d", kills)
	}
	if capped == 0 {
		t.Fatal("never capped")
	}
	// the first kill was at t0+30 s: at t0+630 s a new kill is allowed
	var d Decision
	for i := 600; i < 700; i++ {
		_, d = r.Round(t0.Add(time.Duration(i)*time.Second), false, false)
		if d == Kill {
			return
		}
	}
	t.Fatal("no kill after the budget came back")
}
