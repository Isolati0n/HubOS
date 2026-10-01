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
	if st.Text != "24 of 24 up" || st.Class != "ok" || st.Tooltip != "all machines up" {
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
	if st := r.h.Status(); st.Text != "checking..." || st.Class != "ok" {
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
	if got := New(r.inv, r.vt, r.f, r.l.launch, set, "").ProbeCap(); got != 36 {
		t.Errorf("cap with limit 100 = %d, want 36", got)
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

func rig5000(t *testing.T, roundLen time.Duration) (*rig, *fakeClock) {
	t.Helper()
	var parts []string
	for i := 1; i <= 5000; i++ {
		parts = append(parts, machineDoc(fmt.Sprintf("m%04d", i), fmt.Sprintf("Machine %d", i), "desktop", "moonlight", 21000, i, 7, ""))
	}
	r := newRig(t, parts...)
	clk := &fakeClock{t: time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)}
	r.h.now = clk.now
	r.h.set.ProbeInterval = 10 * time.Second
	// Every machine silent: all checks run into their limit, so the round
	// takes roundLen (10 s at cap 1000, about 40 s at cap 200: docs/hubd-slice2.md).
	r.h.set.Prober = func(ctx context.Context, ts []probe.Target, to time.Duration, limit int, done func(int, probe.Result)) {
		for i := range ts {
			done(i, probe.Result{Reason: "no answer"})
		}
		clk.add(roundLen)
	}
	return r, clk
}

func TestStaleRuleAt5000MachinesWithSlowAndFastRounds(t *testing.T) {
	// Fast rounds (cap 1000: 10 s). Rounds finish 20 s apart (10 s round + 10 s wait).
	r, clk := rig5000(t, 10*time.Second)
	r.h.ProbeRound(r.ctx)
	st := r.h.Status()
	if st.Text != "0 of 5000 up" || !strings.Contains(st.Tooltip, "last check round took 10s") || strings.Contains(st.Tooltip, "STALE") {
		t.Fatalf("after the first round: %+v", st)
	}
	clk.add(19 * time.Second) // the second round is still running, 19 s since the last one ended
	if st := r.h.Status(); strings.Contains(st.Text, "STALE") {
		t.Errorf("19 s after a finished round is not stale: %+v", st)
	}
	clk.add(12 * time.Second) // 31 s > 3 intervals
	st = r.h.Status()
	if !strings.HasPrefix(st.Text, "STALE: ") || st.Class != "alert" || !strings.Contains(st.Tooltip, "STALE: the last finished check round ended 31s ago (limit: 3 intervals = 30s)") {
		t.Errorf("31 s later: %+v", st)
	}

	// Slow rounds (cap 200: about 40 s): by the time the second round ends,
	// the last finished one is 50 s old, so the status is STALE while hubd is
	// working. The tooltip says why: the round took 40 s.
	r, clk = rig5000(t, 40*time.Second)
	r.h.ProbeRound(r.ctx)
	clk.add(45 * time.Second)
	st = r.h.Status()
	if !strings.HasPrefix(st.Text, "STALE: ") || !strings.Contains(st.Tooltip, "last check round took 40s") {
		t.Errorf("slow rounds: %+v", st)
	}
	// And when a round has finished, the status is fresh again.
	r.h.ProbeRound(r.ctx)
	if st := r.h.Status(); strings.Contains(st.Text, "STALE") {
		t.Errorf("right after a round: %+v", st)
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

func TestDownMachinesGroupAtTheTopWithCapAndMoreLine(t *testing.T) {
	r := big(t) // ai-2, g05, g06 are down
	lines := r.h.List(false, "")
	if lines[0] != searchLine || lines[1] != "- Down machines (3)" {
		t.Fatalf("top of the list: %q", lines[:3])
	}
	for i, id := range []string{"ai-2", "g05", "g06"} {
		if !strings.Contains(lines[2+i], id) || !strings.Contains(lines[2+i], "DOWN") {
			t.Errorf("line %d: %q", 2+i, lines[2+i])
		}
	}
	if !strings.HasPrefix(lines[5], "- Hub") {
		t.Errorf("after the down group: %q", lines[5])
	}
	// Cap: with DownMax 2 the third is replaced by a visible line.
	r.h.set.DownMax = 2
	lines = r.h.List(false, "")
	if lines[4] != "   ! and 1 more down machines; use the search" {
		t.Errorf("more line: %q", lines[4])
	}
	// Folding it leaves its heading with the count.
	r.h.Pick("- Down machines (3)")
	lines = r.h.List(false, "")
	if lines[1] != "+ Down machines (3)" || strings.Contains(lines[2], "DOWN") {
		t.Errorf("folded: %q", lines[:3])
	}
	// No down machines: no group.
	for _, id := range []string{"ai-2", "g05", "g06"} {
		r.setStatus(id, statusUp)
	}
	if l := r.h.List(false, ""); strings.Contains(strings.Join(l, "\n"), "Down machines") {
		t.Errorf("group shown with nothing down: %q", l[:3])
	}
}

func TestDownGroupAt5000Machines(t *testing.T) {
	var parts []string
	for i := 1; i <= 5000; i++ {
		parts = append(parts, machineDoc(fmt.Sprintf("m%04d", i), fmt.Sprintf("Machine %d", i), "desktop", "moonlight", 21000, i, 7, ""))
	}
	r := newRig(t, parts...)
	for i, s := range r.h.ms {
		if s.m.Role != "hub" {
			st := statusUp
			if i%20 == 0 {
				st = statusDown // 250 down
			}
			r.setStatus(s.m.ID, st)
		}
	}
	lines := r.h.List(false, "")
	text := strings.Join(lines, "\n")
	if !strings.Contains(text, "- Down machines (250)") || !strings.Contains(text, "   ! and 200 more down machines; use the search") {
		t.Errorf("down group:\n%s", strings.Join(lines[:4], "\n"))
	}
	// 50 down lines + the more line; the big desktop group is folded and says how many it hides.
	if !strings.Contains(text, "+ Desktop (5000 machines, 250 down)") || len(lines) > 60 {
		t.Errorf("%d lines; folded heading missing", len(lines))
	}
}
