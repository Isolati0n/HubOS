package hub

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSnapshotHoldsTheBarLineAndTheMenuList(t *testing.T) {
	r := newRig(t,
		machineDoc("a", "A & Box", "ai", "moonlight", 1, 1, 1, ""),
		machineDoc("b", "B Box", "ai", "moonlight", 1, 2, 2, ""))
	r.setStatus("a", statusUp)
	r.setStatus("b", statusDown)
	r.h.Open("a")
	path := filepath.Join(t.TempDir(), "hubd.last.json")
	r.h.WriteSnapshot(path)
	snap, err := ReadSnapshot(path)
	if err != nil {
		t.Fatal(err)
	}
	if want := r.h.StatusJSON(); snapJSON(snap.Line) != want {
		t.Errorf("line\n got %s\nwant %s", snapJSON(snap.Line), want)
	}
	if got, want := strings.Join(snap.List, "\n"), strings.Join(r.h.List(false, ""), "\n"); got != want {
		t.Errorf("list\n got %s\nwant %s", got, want)
	}
	if time.Since(snap.Time) > time.Minute {
		t.Errorf("time %v", snap.Time)
	}
	if fi, _ := os.Stat(path); fi.Mode().Perm() != 0o600 {
		t.Errorf("mode %v", fi.Mode())
	}
}

func snapJSON(l StatusLine) string {
	b, _ := json.Marshal(l)
	return string(b)
}

func TestStaleLineIsMarkedStaleAndAlertAndSaysHowOldItIs(t *testing.T) {
	now := time.Date(2026, 10, 5, 12, 0, 40, 0, time.UTC)
	snap := Snapshot{Time: now.Add(-47 * time.Second), Line: StatusLine{Text: "3 of 4 up", Class: "ok", Tooltip: "1 down: Storage"}}
	l := snap.StaleLine(now)
	if l.Text != "STALE: 3 of 4 up" || l.Class != "alert" {
		t.Errorf("%+v", l)
	}
	lines := strings.Split(l.Tooltip, "\n")
	if !strings.Contains(lines[0], "hubd is not running") || !strings.Contains(lines[0], "11:59:53 UTC") || !strings.Contains(lines[0], "(40 s ago)") || lines[1] != "1 down: Storage" {
		t.Errorf("tooltip %q", l.Tooltip)
	}
	// hubd's own STALE (old check results) is not doubled
	snap.Line.Text = "STALE: 3 of 4 up"
	if got := snap.StaleLine(now).Text; got != "STALE: 3 of 4 up" {
		t.Errorf("doubled: %q", got)
	}
	// the age is in 10 s steps, then minutes, then hours
	for d, want := range map[time.Duration]string{5 * time.Second: "0 s", 59 * time.Second: "50 s", 61 * time.Second: "1 min", 3*time.Hour + 5*time.Minute + 9*time.Second: "3 h 5 min"} {
		if got := ago(d); got != want {
			t.Errorf("ago(%v) = %q, want %q", d, got, want)
		}
	}
}

func TestStaleListHasAStaleFirstLineAndTheLastList(t *testing.T) {
	now := time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)
	snap := Snapshot{Time: now.Add(-3 * time.Minute), List: []string{"? search by id or name...", " ● a   A Box   UP"}}
	got := snap.StaleList(now)
	if len(got) != 3 || !strings.HasPrefix(got[0], "! STALE: hubd is not running") || !strings.Contains(got[0], "3 min old") || got[2] != snap.List[1] {
		t.Errorf("%q", got)
	}
	// a line starting with "!" is never acted on by Pick
	r := newRig(t, machineDoc("a", "A", "ai", "moonlight", 1, 1, 1, ""))
	if p := r.h.Pick(got[0]); p.Action != "ignored" {
		t.Errorf("pick of the stale line: %+v", p)
	}
}

func TestReadSnapshotRefusesAnEmptyOrBrokenFile(t *testing.T) {
	dir := t.TempDir()
	for name, content := range map[string]string{"missing": "", "broken": "{", "empty": "{}"} {
		p := filepath.Join(dir, name)
		if name != "missing" {
			os.WriteFile(p, []byte(content), 0o600)
		}
		if _, err := ReadSnapshot(p); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
	if got := SnapshotPath("/run/hubos/hubd.sock"); got != "/run/hubos/hubd.last.json" {
		t.Errorf("path %q", got)
	}
}

func TestRunSnapshotWritesAtOnceAndOnChange(t *testing.T) {
	r := newRig(t, machineDoc("a", "A", "ai", "moonlight", 1, 1, 1, ""))
	r.setStatus("a", statusUp)
	path := filepath.Join(t.TempDir(), "s.json")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go r.h.RunSnapshot(ctx, path)
	waitFor(t, "the first snapshot", 2*time.Second, func() bool { _, err := ReadSnapshot(path); return err == nil })
	r.setStatus("a", statusDown)
	r.h.mu.Lock()
	r.h.notifyLocked()
	r.h.mu.Unlock()
	waitFor(t, "the snapshot to follow a change", 4*time.Second, func() bool {
		s, err := ReadSnapshot(path)
		return err == nil && strings.Contains(strings.Join(s.List, "\n"), "DOWN")
	})
}
