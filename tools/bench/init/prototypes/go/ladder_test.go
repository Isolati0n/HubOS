package main

import (
	"testing"
	"time"
)

func TestLadderEndsInAlertNotReboot(t *testing.T) {
	l := &Ladder{P: Policy{Window: time.Minute, BackoffAfter: 3, DepsAfter: 5, GiveUpAfter: 7, Pause: 2 * time.Second}}
	now := time.Unix(1000, 0)
	var got []Action
	for i := 0; i < 8; i++ {
		got = append(got, l.Fail(now, "exit 1"))
		now = now.Add(time.Second)
	}
	want := []Action{Nothing, Nothing, StopForBackoff, StopForBackoff, RestartDeps, RestartDeps, GiveUp, Nothing}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("failure %d: got %v want %v", i+1, got[i], want[i])
		}
	}
	if l.Step != Degraded {
		t.Fatal("not degraded")
	}
	if l.Tick(now.Add(time.Hour)) {
		t.Fatal("a degraded service must not be started again by itself")
	}
	l.Retry()
	if l.Step != Running {
		t.Fatal("owner retry did not resume")
	}
}

func TestPauseGrowsAndForgives(t *testing.T) {
	l := &Ladder{P: Policy{Window: 10 * time.Second, BackoffAfter: 2, DepsAfter: 9, GiveUpAfter: 10, Pause: time.Second}}
	now := time.Unix(0, 0)
	l.Fail(now, "x")
	l.Fail(now, "x")
	if !l.Until.Equal(now.Add(time.Second)) {
		t.Fatalf("first pause %v", l.Until.Sub(now))
	}
	if !l.Tick(now.Add(time.Second)) {
		t.Fatal("pause should be over")
	}
	l.Fail(now.Add(time.Second), "x")
	if l.Until.Sub(now.Add(time.Second)) != 2*time.Second {
		t.Fatalf("second pause %v", l.Until.Sub(now.Add(time.Second)))
	}
	l.Tick(now.Add(5 * time.Second))
	l.Tick(now.Add(time.Minute))
	if len(l.fails) != 0 {
		t.Fatal("quiet window should forgive")
	}
}
