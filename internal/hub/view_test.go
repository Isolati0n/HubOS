package hub

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"hubos/internal/probe"
)

func big(t *testing.T) *rig {
	t.Helper()
	parts := []string{
		machineDoc("gaming-1", "Gaming Box", "gaming", "moonlight", 1, -1, -1, ""),
		machineDoc("ai-1", "AI Box", "ai", "moonlight", 1, -2, -2, ""),
		machineDoc("ai-2", "AI Box 2", "ai", "moonlight", 1, -3, -3, ""),
		machineDoc("vmhost-1", "VM Host", "vm-host", "ssh", 1, -4, -4, ""),
	}
	for i := 1; i <= 20; i++ {
		parts = append(parts, machineDoc(fmt.Sprintf("g%02d", i), fmt.Sprintf("Guest %d", i), "guest", "vnc", 1, i, 100,
			"host = \"vmhost-1\"\nlifetime = \"ephemeral\"\n"))
	}
	r := newRig(t, parts...)
	for _, s := range r.h.ms {
		if s.m.Role != "hub" && s.status == statusChecking {
			r.setStatus(s.m.ID, statusUp)
		}
	}
	r.setStatus("ai-2", statusDown)
	r.setStatus("g05", statusDown)
	r.setStatus("g06", statusDown)
	return r
}

func TestListGroupsNestingFoldingAndDownFirst(t *testing.T) {
	r := big(t)
	got := strings.Join(r.h.List(false, ""), "\n")
	for _, want := range []string{
		"? search by id or name...",
		"- Hub (1 machine)",
		"   hub              Hub                      THIS HUB",
		"- AI (2 machines, 1 down)",
		"   vmhost-1         VM Host                  UP",
		"   + Guests of vmhost-1 (20 machines, 2 down)", // 20 > fold threshold 12: folded
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
	// Down first inside a group: ai-2 before ai-1.
	if strings.Index(got, "ai-2") > strings.Index(got, "ai-1 ") {
		t.Errorf("down machine not first:\n%s", got)
	}
	if strings.Contains(got, "g01") {
		t.Errorf("folded guests are listed:\n%s", got)
	}
}

func TestPickingAFoldedHeadingOpensItAndAsksForReopen(t *testing.T) {
	r := big(t)
	res := r.h.Pick("   + Guests of vmhost-1 (20 machines, 2 down)")
	if res.Action != "toggle" || !res.Reopen || res.Flat {
		t.Fatalf("got %+v", res)
	}
	got := r.h.List(false, "")
	text := strings.Join(got, "\n")
	if !strings.Contains(text, "   - Guests of vmhost-1 (20 machines, 2 down)") || !strings.Contains(text, "      g01") {
		t.Errorf("group not open:\n%s", text)
	}
	// Down guests come first inside the opened group.
	i05, i01 := strings.Index(text, "g05"), strings.Index(text, "g01")
	if i05 > i01 {
		t.Errorf("down guests not first:\n%s", text)
	}
	// Picking the open heading folds it again.
	r.h.Pick("   - Guests of vmhost-1 (20 machines, 2 down)")
	if strings.Contains(strings.Join(r.h.List(false, ""), "\n"), "g01") {
		t.Error("did not fold again")
	}
}

func TestFlatListHasEveryMachineDownFirst(t *testing.T) {
	r := big(t)
	lines := r.h.List(true, "")
	if lines[0] != backLine || len(lines) != 1+25 { // back + hub + 4 + 20
		t.Fatalf("%d lines, first %q", len(lines), lines[0])
	}
	if !strings.Contains(lines[1], "DOWN") || !strings.Contains(lines[2], "DOWN") || !strings.Contains(lines[3], "DOWN") {
		t.Errorf("down machines not first: %q", lines[1:4])
	}
	if res := r.h.Pick(searchLine); res.Action != "search" || !res.Flat || !res.Reopen || !res.Ask {
		t.Errorf("search: %+v", res)
	}
	if res := r.h.Pick(backLine); res.Action != "back" || res.Flat || !res.Reopen {
		t.Errorf("back: %+v", res)
	}
}

func TestPickIgnoresHeadingsUnknownAndEmptyLines(t *testing.T) {
	r := big(t)
	for name, line := range map[string]string{
		"empty":             "",
		"spaces only":       "     ",
		"newline":           "\n",
		"unknown line":      "   no-such-machine   Nothing   UP",
		"plain text":        "hello",
		"message line":      "! AI Box is down, not opened",
		"id without indent": "ai-1",
		"unknown heading":   "+ Nonsense (3 machines)",
		"name only":         "   AI Box",
	} {
		before := r.l.count()
		res := r.h.Pick(line)
		if res.Action != "ignored" || res.Reopen || res.Message != "" {
			t.Errorf("%s: got %+v", name, res)
		}
		if r.l.count() != before || len(r.f.windows) != 0 {
			t.Errorf("%s: something was started", name)
		}
	}
	// A machine line opens by id and the rest of the line does not matter.
	res := r.h.Pick("   ai-1             whatever the name was    DOWN")
	if res.Action != "open" {
		t.Errorf("machine line: %+v", res)
	}
}

func TestStatusLineCountsHubExcludedAndTooltipCapped(t *testing.T) {
	r := big(t)
	r.h.set.TooltipCap = 2
	st := r.h.Status()
	// 24 non-hub machines, 3 down.
	if st.Text != "21 of 24 up" || st.Class != "alert" {
		t.Errorf("got %+v", st)
	}
	if !strings.Contains(st.Tooltip, "3 down: AI Box 2, Guest 5 and 1 more") {
		t.Errorf("tooltip: %q", st.Tooltip)
	}
	r.setStatus("ai-2", statusUp)
	r.setStatus("g05", statusUp)
	r.setStatus("g06", statusUp)
	st = r.h.Status()
	if st.Text != "24 of 24 up" || st.Class != "ok" || !strings.HasPrefix(st.Tooltip, "all machines up") {
		t.Errorf("all up: %+v", st)
	}
	if strings.Contains(st.JSON(), "\n") || !strings.HasPrefix(st.JSON(), `{"text":"24 of 24 up","class":"ok","tooltip":`) {
		t.Errorf("json: %s", st.JSON())
	}
}

func TestStatusLineBeforeFirstRoundNotCheckedAndStale(t *testing.T) {
	r := newRig(t,
		machineDoc("a", "A", "ai", "moonlight", 1, 1, 1, ""),
		machineDoc("n", "N", "desktop", "moonlight", 0, 2, 2, ""))
	if st := r.h.Status(); st.Text != "checking... 0 of 1 done" || st.Class != "ok" {
		t.Errorf("start: %+v", st)
	}
	r.setStatus("a", statusUp)
	st := r.h.Status()
	if st.Text != "1 of 1 up" || !strings.Contains(st.Tooltip, "1 not checked") {
		t.Errorf("one up: %+v", st)
	}
}

func TestOutOfFileHandlesIsNeverShownAsDown(t *testing.T) {
	r := newRig(t, machineDoc("a", "A", "ai", "moonlight", 1, 1, 1, ""))
	r.h.mu.Lock()
	r.h.byID["a"].status = statusNoHandles
	r.h.mu.Unlock()
	st := r.h.Status()
	if strings.Contains(st.Tooltip, "down") || st.Class != "ok" {
		t.Errorf("%+v", st)
	}
	if res := r.h.Open("a"); res.Action != "open" {
		// "not checked" machines may be opened
		t.Errorf("%+v", res)
	}
}

func TestFileLimitCapsTheProbeCap(t *testing.T) {
	r := newRig(t)
	set := r.set
	set.ProbeCap, set.FileLimit = 200, 100
	if got := New(r.inv, r.vt, r.f, r.l.launch, set, "").ProbeCap(); got != 80 {
		t.Errorf("cap with limit 100 = %d, want 80 (80%%)", got)
	}
	set.FileLimit = 1024
	if got := New(r.inv, r.vt, r.f, r.l.launch, set, "").ProbeCap(); got != 200 {
		t.Errorf("cap with limit 1024 = %d", got)
	}
}

func TestFilterAndListCap(t *testing.T) {
	r := big(t)
	got := r.h.List(false, "GUEST 1")
	// Guest 1, Guest 10..19: case-insensitive match on the name.
	if got[0] != backLine || got[1] != `! 11 match "GUEST 1"` || len(got) != 2+11 {
		t.Errorf("%d lines: %q", len(got), got)
	}
	if got := r.h.List(false, "vmhost"); len(got) != 3 || !strings.Contains(got[2], "vmhost-1") {
		t.Errorf("by id: %q", got)
	}
	if got := r.h.List(false, "no such thing"); len(got) != 2 || got[1] != `! 0 match "no such thing"` {
		t.Errorf("none: %q", got)
	}
	r.h.set.ListMax = 5
	flat := r.h.List(true, "")
	if len(flat) != 1+5+1 || flat[6] != "! 20 more not shown; type more letters to narrow the search" {
		t.Errorf("capped flat list: %q", flat)
	}
	// Opening a group is capped too, and a "more" line is never a pick.
	r.h.Pick("   + Guests of vmhost-1 (20 machines, 2 down)")
	grouped := strings.Join(r.h.List(false, ""), "\n")
	if !strings.Contains(grouped, "      ! 15 more in this group not shown; use the search") {
		t.Errorf("group cap missing:\n%s", grouped)
	}
	if res := r.h.Pick("      ! 15 more in this group not shown; use the search"); res.Action != "ignored" {
		t.Errorf("%+v", res)
	}
}

// A fake clock and a fake checker, so a 5000-machine round that takes 10 s or
// 40 s can be tested without waiting.
type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *fakeClock) now() time.Time { c.mu.Lock(); defer c.mu.Unlock(); return c.t }
func (c *fakeClock) add(d time.Duration) {
	c.mu.Lock()
	c.t = c.t.Add(d)
	c.mu.Unlock()
}

// rig5000 is 5000 machines with a fake clock and a fake checker. The checker
// delivers its results evenly over roundLen (so a 40 s round has a result
// about every 8 ms), all "down" or all "up".
func rig5000(t *testing.T, roundLen time.Duration, up bool) (*rig, *fakeClock) {
	t.Helper()
	var parts []string
	for i := 1; i <= 5000; i++ {
		parts = append(parts, machineDoc(fmt.Sprintf("m%04d", i), fmt.Sprintf("Machine %d", i), "desktop", "moonlight", 21000, i, 7, ""))
	}
	r := newRig(t, parts...)
	clk := &fakeClock{t: time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)}
	r.h.now = clk.now
	r.h.lastResult = clk.now()
	r.h.set.ProbeInterval = 10 * time.Second
	r.h.set.Prober = func(ctx context.Context, ts []probe.Target, to time.Duration, limit int, done func(int, probe.Result)) {
		step := roundLen / time.Duration(len(ts))
		for i := range ts {
			clk.add(step)
			done(i, probe.Result{Up: up, Reason: "no answer"})
		}
	}
	return r, clk
}

func TestStaleRuleAt5000MachinesCap1000(t *testing.T) {
	// Cap 1000: a round of 10 s, a new one 10 s after it finished.
	r, clk := rig5000(t, 10*time.Second, false)
	if st := r.h.Status(); st.Text != "checking... 0 of 5000 done" {
		t.Fatalf("before any round: %+v", st)
	}
	r.h.ProbeRound(r.ctx)
	st := r.h.Status()
	if st.Text != "0 of 5000 up" || strings.Contains(st.Tooltip, "note:") || strings.Contains(st.Tooltip, "STALE") || !strings.Contains(st.Tooltip, "last check round took 10s") {
		t.Fatalf("after the round: %+v", st)
	}
	clk.add(25 * time.Second) // waiting 10 s + a running round's first results are not in this fake
	if st := r.h.Status(); strings.Contains(st.Text, "STALE") {
		t.Errorf("25 s without a result is under 3 intervals: %+v", st)
	}
	clk.add(6 * time.Second) // 31 s
	st = r.h.Status()
	if !strings.HasPrefix(st.Text, "STALE: ") || st.Class != "alert" || !strings.Contains(st.Tooltip, "STALE: no check result for 30s (limit: 3 intervals = 30s)") {
		t.Errorf("31 s without a result: %+v", st)
	}
	// While STALE the age moves in 10 s steps: the tooltip is the same at
	// 31 s and 38 s, and changes at 41 s.
	at31 := st.Tooltip
	clk.add(7 * time.Second)
	if got := r.h.Status().Tooltip; got != at31 {
		t.Errorf("tooltip changed inside a 10 s step:\n%s\n%s", at31, got)
	}
	clk.add(3 * time.Second)
	if got := r.h.Status().Tooltip; got == at31 || !strings.Contains(got, "no check result for 40s") {
		t.Errorf("tooltip after 41 s: %s", got)
	}
	// A new result clears it.
	r.h.ProbeRound(r.ctx)
	if st := r.h.Status(); strings.Contains(st.Text, "STALE") {
		t.Errorf("after a new round: %+v", st)
	}
}

func TestLongRoundAt5000MachinesCap200IsANoteNotAnAlert(t *testing.T) {
	// Cap 200: a round of 40 s, longer than two intervals (20 s). Results
	// keep arriving all the time, so it is not STALE, and with every machine
	// up there is nothing to alert about.
	r, clk := rig5000(t, 40*time.Second, true)
	r.h.ProbeRound(r.ctx)
	st := r.h.Status()
	if st.Text != "5000 of 5000 up" || st.Class != "ok" || strings.Contains(st.Text, "STALE") {
		t.Fatalf("got %+v", st)
	}
	if !strings.Contains(st.Tooltip, "last check round took 40s") || !strings.Contains(st.Tooltip, "note: that is longer than two intervals (20s)") {
		t.Errorf("tooltip: %q", st.Tooltip)
	}
	// Ten seconds later the next round starts and results flow again: still fine,
	// even 45 s after the end of the first round, because results arrived in between.
	clk.add(10 * time.Second)
	r.h.ProbeRound(r.ctx)
	clk.add(5 * time.Second)
	if st := r.h.Status(); strings.Contains(st.Text, "STALE") || st.Class != "ok" {
		t.Errorf("%+v", st)
	}
	// A round of 40 s whose results all stop for 31 s in the middle is STALE.
	r, clk = rig5000(t, 40*time.Second, true)
	r.h.set.Prober = func(ctx context.Context, ts []probe.Target, to time.Duration, limit int, done func(int, probe.Result)) {
		for i := range ts[:100] {
			done(i, probe.Result{Up: true})
		}
		clk.add(31 * time.Second) // nothing arrives
		if st := r.h.Status(); !strings.HasPrefix(st.Text, "STALE: ") {
			t.Errorf("31 s of silence in the middle of a round: %+v", st)
		}
		for i := range ts[100:] {
			done(100+i, probe.Result{Up: true})
		}
	}
	r.h.ProbeRound(r.ctx)
	if st := r.h.Status(); strings.Contains(st.Text, "STALE") {
		t.Errorf("after the round: %+v", st)
	}
}

func TestFirstRoundShowsHowManyAreDoneNotPartialCounts(t *testing.T) {
	r, _ := rig5000(t, 40*time.Second, false)
	var seen StatusLine
	r.h.set.Prober = func(ctx context.Context, ts []probe.Target, to time.Duration, limit int, done func(int, probe.Result)) {
		for i := range ts {
			done(i, probe.Result{Reason: "no answer"})
			if i == 1233 {
				seen = r.h.Status() // in the middle of the first round
			}
		}
	}
	r.h.ProbeRound(r.ctx)
	if seen.Text != "checking... 1234 of 5000 done" || seen.Class != "ok" || strings.Contains(seen.Tooltip, "down") || !strings.Contains(seen.Tooltip, "first check round in progress") {
		t.Errorf("during the first round: %+v", seen)
	}
	if st := r.h.Status(); st.Text != "0 of 5000 up" || st.Class != "alert" {
		t.Errorf("after it: %+v", st)
	}
}

func TestRoundStartsOneIntervalAfterTheLastFinished(t *testing.T) {
	r := newRig(t, machineDoc("a", "A", "ai", "moonlight", 1, 1, 1, ""))
	r.h.set.ProbeInterval = 300 * time.Millisecond
	var mu sync.Mutex
	var starts, ends []time.Time
	r.h.set.Prober = func(ctx context.Context, ts []probe.Target, to time.Duration, limit int, done func(int, probe.Result)) {
		mu.Lock()
		starts = append(starts, time.Now())
		mu.Unlock()
		time.Sleep(200 * time.Millisecond)
		for i := range ts {
			done(i, probe.Result{Up: true})
		}
		mu.Lock()
		ends = append(ends, time.Now())
		mu.Unlock()
	}
	ctx, cancel := context.WithTimeout(r.ctx, 1200*time.Millisecond)
	defer cancel()
	r.h.RunProbes(ctx)
	mu.Lock()
	defer mu.Unlock()
	if len(starts) < 2 {
		t.Fatalf("only %d rounds", len(starts))
	}
	if gap := starts[1].Sub(ends[0]); gap < 280*time.Millisecond {
		t.Errorf("second round started %s after the first finished, want about one interval (300ms)", gap)
	}
}
