package hub

import (
	"fmt"
	"strings"
	"testing"
	"time"
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
	r.h.mu.Lock()
	r.h.byID["a"].checkedAt = time.Now().Add(-10 * time.Minute)
	r.h.mu.Unlock()
	if st := r.h.Status(); st.Class != "alert" || !strings.Contains(st.Tooltip, "stale") {
		t.Errorf("stale: %+v", st)
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
	if got[0] != backLine || got[1] != `! 11 machines match "GUEST 1"` || len(got) != 2+11 {
		t.Errorf("%d lines: %q", len(got), got)
	}
	if got := r.h.List(false, "vmhost"); len(got) != 3 || !strings.Contains(got[2], "vmhost-1") {
		t.Errorf("by id: %q", got)
	}
	if got := r.h.List(false, "no such thing"); len(got) != 2 || got[1] != `! 0 machines match "no such thing"` {
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
