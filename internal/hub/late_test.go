package hub

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"hubos/internal/viewers"
)

// The orphan viewer: a viewer whose window appears later than the window
// wait. The cases A, B and C are the ones found in the first review (they are
// shown there as they behaved before the fix, in docs/hubd-slice2.md section
// 11.2). The settle wait is the production value (500 ms).

func lateRig(t *testing.T) *rig {
	r := newRig(t, machineDoc("a", "A", "ai", "moonlight", 1, 1, 1, ""))
	r.setStatus("a", statusUp)
	r.h.set.WindowWait = 300 * time.Millisecond
	r.h.set.LateGrace = 5 * time.Second
	r.h.set.Settle = 500 * time.Millisecond
	return r
}

func lateWindow(r *rig, after time.Duration) func([]string) {
	return func(args []string) {
		go func() {
			time.Sleep(after)
			r.f.add("hubos-a", "A")
		}()
	}
}

func waitFor(t *testing.T, what string, limit time.Duration, ok func() bool) {
	t.Helper()
	deadline := time.Now().Add(limit)
	for time.Now().Before(deadline) {
		if ok() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

func windowCount(r *rig) int { r.f.mu.Lock(); defer r.f.mu.Unlock(); return len(r.f.windows) }

func phaseOf(r *rig, id string) phase {
	r.h.mu.Lock()
	defer r.h.mu.Unlock()
	return r.h.byID[id].phase
}

// Case A: the window shows up after the wait ran out; the owner clicks again.
// Before the fix a second viewer was started and two windows existed.
func TestLateCaseA_NoSecondViewer(t *testing.T) {
	r := lateRig(t)
	r.l.script = lateWindow(r, 600*time.Millisecond)
	first := r.h.Open("a")
	t.Logf("1st click: %+v", first)
	if first.Action != "late" || !strings.Contains(first.Message, "no window appeared for A within 300ms") || !strings.Contains(first.Message, "keeps waiting") {
		t.Fatalf("first: %+v", first)
	}
	if phaseOf(r, "a") != phaseLate {
		t.Fatal("not in the late-window state")
	}
	// A click while it is late starts nothing.
	r.l.script = nil
	mid := r.h.Open("a")
	t.Logf("click during the late state: %+v", mid)
	if mid.Action != "busy" || mid.Message != "the viewer for A is still starting; waiting for its window" {
		t.Errorf("mid: %+v", mid)
	}
	// The window arrives and is adopted by itself, and placed at home.
	waitFor(t, "the late window to be adopted", 3*time.Second, func() bool { _, ok := r.h.WindowOf("a"); return ok })
	if id, _ := r.h.WindowOf("a"); id != 0 {
		t.Errorf("adopted window %d, want 0", id)
	}
	if len(r.f.moves) != 1 || r.f.moves[0] != [3]int{0, 1, 1} {
		t.Errorf("not placed at home: %v", r.f.moves)
	}
	waitFor(t, "idle", time.Second, func() bool { return phaseOf(r, "a") == phaseIdle })
	second := r.h.Open("a")
	t.Logf("click after: %+v", second)
	if second.Action != "went" || r.l.count() != 1 || windowCount(r) != 1 {
		t.Errorf("a second viewer must not start: %+v launches=%d windows=%d", second, r.l.count(), windowCount(r))
	}
	if !strings.Contains(strings.Join(r.h.List(false, ""), "\n"), "a                A") {
		t.Error("machine missing from the list")
	}
}

// Case B: the late window arrives while the second click would have waited.
// Before the fix: two viewers, "cannot tell which one". Now: one viewer.
func TestLateCaseB_SecondClickStartsNothing(t *testing.T) {
	r := lateRig(t)
	r.l.script = lateWindow(r, 450*time.Millisecond)
	first := r.h.Open("a")
	t.Logf("1st click: %+v", first)
	r.l.script = lateWindow(r, 300*time.Millisecond) // would have been the second viewer's window
	second := r.h.Open("a")
	t.Logf("2nd click: %+v", second)
	if second.Action != "busy" {
		t.Errorf("second: %+v", second)
	}
	waitFor(t, "adoption", 3*time.Second, func() bool { _, ok := r.h.WindowOf("a"); return ok })
	time.Sleep(700 * time.Millisecond)
	if r.l.count() != 1 || windowCount(r) != 1 || phaseOf(r, "a") != phaseIdle {
		t.Errorf("launches=%d windows=%d phase=%v", r.l.count(), windowCount(r), phaseOf(r, "a"))
	}
}

// Case C: before the fix the first viewer's late window was taken for the
// second viewer's. Now there is no second viewer, so the window that comes is
// the right one.
func TestLateCaseC_NoWrongWindowAdopted(t *testing.T) {
	r := lateRig(t)
	r.l.script = lateWindow(r, 450*time.Millisecond)
	r.h.Open("a")
	r.l.script = lateWindow(r, 1500*time.Millisecond) // the slow second viewer that must not exist
	second := r.h.Open("a")
	t.Logf("2nd click: %+v", second)
	waitFor(t, "adoption", 3*time.Second, func() bool { _, ok := r.h.WindowOf("a"); return ok })
	time.Sleep(1800 * time.Millisecond) // long enough for the slow second window, if a second viewer had been started
	if r.l.count() != 1 {
		t.Errorf("viewers started: %d", r.l.count())
	}
	if windowCount(r) != 1 {
		t.Errorf("windows: %v", names(r.f))
	}
	if id, ok := r.h.WindowOf("a"); !ok || id != 0 {
		t.Errorf("recorded %d %v", id, ok)
	}
	if second.Action != "busy" {
		t.Errorf("second: %+v", second)
	}
}

func TestLateGraceEndsWithTheProcessAliveAndNoWindow(t *testing.T) {
	r := lateRig(t)
	r.h.set.LateGrace = 600 * time.Millisecond
	r.l.script = func([]string) {}
	first := r.h.Open("a")
	if first.Action != "late" {
		t.Fatalf("first: %+v", first)
	}
	waitFor(t, "the grace to end", 3*time.Second, func() bool { return phaseOf(r, "a") == phaseUnmatched })
	menu := strings.Join(r.h.List(false, ""), "\n")
	if !strings.Contains(menu, "x forget unknown window for A (a)") || !strings.Contains(menu, "stopped waiting and killed nothing") {
		t.Errorf("menu:\n%s", menu)
	}
	res := r.h.Open("a")
	if res.Action != "refused" || !strings.Contains(res.Message, "hubd forget a") || r.l.count() != 1 {
		t.Errorf("after the grace: %+v launches=%d", res, r.l.count())
	}
	// Nothing was killed or closed: the fake has no kill; windows and closes are untouched.
	if len(r.f.closed) != 0 {
		t.Errorf("closed: %v", r.f.closed)
	}
	if f := r.h.Forget("a"); f.Action != "forgot" {
		t.Errorf("forget: %+v", f)
	}
	if res := r.h.Open("a"); res.Action == "refused" {
		t.Errorf("open after forget: %+v", res)
	}
}

func TestLateViewerExitsFirstIsAFailureWithTheLogPath(t *testing.T) {
	r := lateRig(t)
	r.h.set.LogDir = t.TempDir()
	r.l.script = func([]string) {}
	if first := r.h.Open("a"); first.Action != "late" {
		t.Fatalf("first: %+v", first)
	}
	r.l.exit(0, errors.New("exit status 3"))
	waitFor(t, "idle", 2*time.Second, func() bool { return phaseOf(r, "a") == phaseIdle })
	st := r.h.Status()
	want := "its output is in " + filepath.Join(r.h.set.LogDir, "viewer-a.log")
	if !strings.Contains(st.Tooltip, "exit status 3") || !strings.Contains(st.Tooltip, want) || st.Class != "alert" {
		t.Errorf("tooltip: %+v", st)
	}
	// And a new open is allowed again.
	r.l.script = nil
	if res := r.h.Open("a"); res.Action != "open" {
		t.Errorf("open after the failure: %+v", res)
	}
}

func TestLateForgetStopsTheWaitAndTouchesNothing(t *testing.T) {
	r := lateRig(t)
	r.l.script = func([]string) {}
	r.h.Open("a")
	f := r.h.Forget("a")
	if f.Action != "forgot" || !strings.Contains(f.Message, "the viewer was not touched") || phaseOf(r, "a") != phaseIdle {
		t.Errorf("%+v", f)
	}
	r.f.add("hubos-a", "A") // a window after the wait was stopped is not adopted
	time.Sleep(900 * time.Millisecond)
	if _, ok := r.h.WindowOf("a"); ok || len(r.f.moves) != 0 {
		t.Error("adopted after the wait was stopped")
	}
}

func TestLateWindowForViewerThatDoesNotSetItsName(t *testing.T) {
	r := newRig(t,
		machineDoc("f", "F", "nas", "files", 1, 5, 5, ""),
		machineDoc("g", "G", "nas", "files", 1, 6, 6, ""))
	r.setStatus("f", statusUp)
	r.setStatus("g", statusUp)
	r.h.set.WindowWait, r.h.set.LateGrace, r.h.set.Settle = 300*time.Millisecond, 5*time.Second, 200*time.Millisecond
	r.f.add("someone-elses", "not ours") // there before: ignored
	r.l.script = func([]string) {
		go func() { time.Sleep(600 * time.Millisecond); r.f.add("whatever", "F window") }()
	}
	first := r.h.Open("f")
	if first.Action != "late" {
		t.Fatalf("%+v", first)
	}
	// While it is late, another comparison-matched viewer cannot be started.
	other := r.h.Open("g")
	if other.Action != "busy" || !strings.Contains(other.Message, "cannot be told apart") || r.l.count() != 1 {
		t.Errorf("other: %+v launches=%d", other, r.l.count())
	}
	waitFor(t, "adoption", 3*time.Second, func() bool { _, ok := r.h.WindowOf("f"); return ok })
	if id, _ := r.h.WindowOf("f"); id != 1 {
		t.Errorf("adopted %d, want the new window 1 (not the one that was there before)", id)
	}
	waitFor(t, "idle", time.Second, func() bool { return phaseOf(r, "f") == phaseIdle })
	r.l.script = func([]string) { r.f.add("whatever2", "G window") }
	if res := r.h.Open("g"); res.Action != "open" {
		t.Errorf("g after f is done: %+v", res)
	}
}

func TestLateTwoNamedWindowsAreNotGuessed(t *testing.T) {
	r := lateRig(t)
	r.l.script = func([]string) {
		go func() { time.Sleep(500 * time.Millisecond); r.f.add("hubos-a", "A"); r.f.add("hubos-a", "A") }()
	}
	r.h.Open("a")
	waitFor(t, "unidentified", 3*time.Second, func() bool { return phaseOf(r, "a") == phaseUnmatched })
	if len(r.f.moves) != 0 || len(r.f.focused) != 0 {
		t.Errorf("windows touched: %v %v", r.f.moves, r.f.focused)
	}
	if !strings.Contains(r.h.Status().Tooltip, "cannot tell which one") {
		t.Errorf("tooltip: %q", r.h.Status().Tooltip)
	}
}

func TestPerViewerWindowWaitOverridesTheHubDefault(t *testing.T) {
	r := lateRig(t)
	vt, ps := viewers.Parse([]byte("format = 1\n[[viewer]]\nid = \"quick\"\nprograms = [\"moonlight\"]\ncommand = [\"foot\", \"--app-id={app_id}\"]\nsets_name = true\nwindow_wait = \"150ms\"\n"))
	if len(ps) != 0 {
		t.Fatal(ps)
	}
	r.h.vt = vt
	r.h.set.WindowWait = 30 * time.Second // the hub default would be far too long
	r.l.script = func([]string) {}
	start := time.Now()
	res := r.h.Open("a")
	if res.Action != "late" || !strings.Contains(res.Message, "within 150ms") || time.Since(start) > 2*time.Second {
		t.Errorf("%+v after %s", res, time.Since(start))
	}
}

func TestSameNameWindowsGiveAWarningInTheMenuAndTheTooltip(t *testing.T) {
	r := newRig(t,
		machineDoc("a", "A name", "ai", "moonlight", 1, 1, 1, ""),
		machineDoc("b", "B name", "ai", "moonlight", 1, 2, 2, ""))
	r.setStatus("a", statusUp)
	r.setStatus("b", statusUp)
	r.h.Open("a")
	r.f.add("hubos-a", "A") // an extra window with the same name
	st, _ := r.f.State()
	r.h.syncWindows(st)

	menu := r.h.List(false, "")
	want := "! WARNING: 2 windows are named hubos-a (A name); hubd knows window #0; close the extra one by hand"
	if menu[0] != want {
		t.Errorf("menu first line: %q", menu[0])
	}
	tip := r.h.Status()
	if tip.Class != "alert" || !strings.HasPrefix(tip.Tooltip, want[2:]) {
		t.Errorf("tooltip: %+v", tip)
	}
	if strings.Contains(strings.Join(menu, "\n"), "hubos-b") {
		t.Error("warning for a machine without duplicates")
	}
	// A picked warning line does nothing; end and forget keep working on the known window.
	if res := r.h.Pick("! " + want[2:]); res.Action != "ignored" {
		t.Errorf("pick: %+v", res)
	}
	if res := r.h.End("a"); res.Action != "end" {
		t.Errorf("end: %+v", res)
	}
	if len(r.f.closed) != 1 || r.f.closed[0] != 0 {
		t.Errorf("end closed %v, want only the known window 0", r.f.closed)
	}
	// The extra window is still there: the warning goes (one window left) once it is the only one.
	st, _ = r.f.State()
	r.h.syncWindows(st)
	if strings.Contains(strings.Join(r.h.List(false, ""), "\n"), "WARNING") {
		t.Error("warning stays with a single window")
	}
}

func TestStaleTooltipShowsBothAges(t *testing.T) {
	r, clk := rig5000(t, 10*time.Second, true)
	r.h.ProbeRound(r.ctx)
	clk.add(41 * time.Second)
	tip := r.h.Status().Tooltip
	if !strings.Contains(tip, "STALE: no check result for 40s (limit: 3 intervals = 30s); the last check round finished 40s ago") {
		t.Errorf("tooltip: %s", tip)
	}
	// Before any round has finished.
	r2 := newRig(t, machineDoc("a", "A", "ai", "moonlight", 1, 1, 1, ""))
	r2.h.set.ProbeInterval = 100 * time.Millisecond
	time.Sleep(450 * time.Millisecond)
	if tip := r2.h.Status().Tooltip; !strings.Contains(tip, "no check round has finished yet") {
		t.Errorf("tooltip: %s", tip)
	}
}

func TestNoEscapeFlagLeavesTheTextAlone(t *testing.T) {
	r := newRig(t, machineDoc("m", `A<b>&"x"`, "ai", "moonlight", 1, 1, 1, ""))
	r.setStatus("m", statusDown)
	escaped := r.h.StatusJSON()
	r.h.set.NoEscape = true
	raw := r.h.StatusJSON()
	var dec StatusLine
	if err := jsonUnmarshal(escaped, &dec); err != nil || !strings.Contains(dec.Tooltip, `1 down: A&lt;b&gt;&amp;&quot;x&quot;`) {
		t.Errorf("default must escape: %s (%v)", escaped, err)
	}
	var back StatusLine
	if err := jsonUnmarshal(raw, &back); err != nil || !strings.Contains(back.Tooltip, `1 down: A<b>&"x"`) {
		t.Errorf("--no-escape: %s (%v)", raw, err)
	}
	if strings.Contains(raw, "&amp;") || strings.Contains(raw, "u0026amp") {
		t.Errorf("raw still escaped: %s", raw)
	}
}

func jsonUnmarshal(s string, v any) error { return jsonDecode([]byte(s), v) }

var _ = fmt.Sprint
var _ = os.Getenv

func jsonDecode(b []byte, v any) error { return json.Unmarshal(b, v) }

// ---- third round: end message, stop-waiting line, late_grace, clean exit, ignore list ----

func TestEndOnALateMachineRefusesAndSaysWhy(t *testing.T) {
	r := lateRig(t)
	r.l.script = func([]string) {}
	if first := r.h.Open("a"); first.Action != "late" {
		t.Fatalf("%+v", first)
	}
	res := r.h.End("a")
	want := "the viewer for A is still starting, so there is no window to close. hubd end cannot cancel a viewer (hubd never kills one). To stop waiting for its window, run: hubd forget a"
	if res.Action != "refused" || res.Message != want {
		t.Errorf("got %+v", res)
	}
	if len(r.f.closed) != 0 || phaseOf(r, "a") != phaseLate {
		t.Errorf("end changed something: closed=%v phase=%v", r.f.closed, phaseOf(r, "a"))
	}
}

func TestLateMachineShowsStopWaitingLineAndANonRedTooltip(t *testing.T) {
	r := lateRig(t)
	r.h.set.LateGrace = 25 * time.Second
	r.l.script = func([]string) {}
	first := r.h.Open("a")
	if first.Action != "late" {
		t.Fatalf("%+v", first)
	}
	// The bar item is not red, there is no red message, and the tooltip says what is going on.
	st := r.h.Status()
	if st.Class != "ok" || !strings.Contains(st.Tooltip, "waiting for A's window (up to 30 s more)") {
		t.Errorf("status: %+v", st)
	}
	// "up to N s more" moves in 10 s steps (25 s left rounds up to 30, then 20 ...).
	r.h.mu.Lock()
	r.h.byID["a"].lateEnd = time.Now().Add(14 * time.Second)
	r.h.mu.Unlock()
	if tip := r.h.Status().Tooltip; !strings.Contains(tip, "(up to 20 s more)") {
		t.Errorf("14 s left: %s", tip)
	}
	// The menu offers the way out.
	menu := strings.Join(r.h.List(false, ""), "\n")
	if !strings.Contains(menu, "x stop waiting for A's window (a)") || strings.Contains(menu, "forget unknown window") {
		t.Errorf("menu:\n%s", menu)
	}
	if strings.Contains(strings.Join(r.h.List(false, "")[:1], ""), "no window appeared") {
		t.Error("the waiting message was put at the top of the menu as an alert")
	}
	pr := r.h.Pick("x stop waiting for A's window (a)")
	if pr.Action != "forgot" || !pr.Reopen || !strings.Contains(pr.Message, "stopped waiting") {
		t.Errorf("pick: %+v", pr)
	}
	if phaseOf(r, "a") != phaseIdle || r.l.count() != 1 || len(r.f.closed) != 0 {
		t.Errorf("phase=%v launches=%d closed=%v", phaseOf(r, "a"), r.l.count(), r.f.closed)
	}
	st = r.h.Status()
	if strings.Contains(st.Tooltip, "waiting for") || strings.Contains(strings.Join(r.h.List(false, ""), "\n"), "stop waiting") {
		t.Errorf("still shown after stopping: %+v", st)
	}
	// An unknown id on such a line is refused, nothing else happens.
	if pr := r.h.Pick("x stop waiting for Nobody's window (no-such)"); pr.Action == "forgot" {
		t.Errorf("%+v", pr)
	}
}

func TestPerViewerLateGraceOverridesTheHubSetting(t *testing.T) {
	r := lateRig(t)
	r.h.set.LateGrace = time.Minute // the hub setting would wait far too long
	vt, ps := viewers.Parse([]byte("format = 1\n[[viewer]]\nid = \"v\"\nprograms = [\"moonlight\"]\ncommand = [\"foot\", \"--app-id={app_id}\"]\nsets_name = true\nwindow_wait = \"200ms\"\nlate_grace = \"600ms\"\n"))
	if len(ps) != 0 {
		t.Fatal(ps)
	}
	r.h.vt = vt
	r.l.script = func([]string) {}
	res := r.h.Open("a")
	if res.Action != "late" || !strings.Contains(res.Message, "for up to 600ms") {
		t.Fatalf("%+v", res)
	}
	waitFor(t, "the viewer's own grace to end", 3*time.Second, func() bool { return phaseOf(r, "a") == phaseUnmatched })
	// And a viewer without late_grace uses the hub's.
	r2 := lateRig(t)
	r2.h.set.LateGrace = 7 * time.Second
	r2.l.script = func([]string) {}
	if res := r2.h.Open("a"); !strings.Contains(res.Message, "for up to 7s") {
		t.Errorf("hub default: %+v", res)
	}
}

func TestCleanExitKeepsWaitingNonZeroExitFailsAtOnce(t *testing.T) {
	// Status 0 before the window: keep waiting through the grace, adopt the window.
	r := lateRig(t)
	r.h.set.LogDir = t.TempDir()
	r.l.script = lateWindow(r, 700*time.Millisecond) // after the 300 ms wait
	done := make(chan OpenResult, 1)
	go func() { done <- r.h.Open("a") }()
	waitFor(t, "the launch", time.Second, func() bool { return r.l.count() == 1 })
	r.l.exit(0, nil) // the process exits cleanly (a hand-over to a copy that is already running)
	first := <-done
	t.Logf("first click: %+v", first)
	if first.Action != "late" || !strings.Contains(first.Message, "exited cleanly") {
		t.Fatalf("clean exit must keep waiting: %+v", first)
	}
	waitFor(t, "adoption after a clean exit", 3*time.Second, func() bool { _, ok := r.h.WindowOf("a"); return ok })
	if r.l.count() != 1 || windowCount(r) != 1 {
		t.Errorf("launches=%d windows=%d", r.l.count(), windowCount(r))
	}

	// A clean exit and no window at all: the grace ends, nothing is killed.
	r = lateRig(t)
	r.h.set.LateGrace = 500 * time.Millisecond
	r.l.script = func([]string) {}
	go func() { time.Sleep(100 * time.Millisecond); r.l.exit(0, nil) }()
	if res := r.h.Open("a"); res.Action != "late" {
		t.Fatalf("%+v", res)
	}
	waitFor(t, "the grace to end", 3*time.Second, func() bool { return phaseOf(r, "a") == phaseUnmatched })

	// A non-zero exit during the first wait: failure at once, with the log path.
	r = lateRig(t)
	r.h.set.LogDir = t.TempDir()
	r.h.set.WindowWait = 5 * time.Second
	r.l.script = func([]string) {}
	go func() { time.Sleep(150 * time.Millisecond); r.l.exit(0, errors.New("exit status 3")) }()
	start := time.Now()
	res := r.h.Open("a")
	if res.Action != "failed" || !strings.Contains(res.Message, "exit status 3") || !strings.Contains(res.Message, filepath.Join(r.h.set.LogDir, "viewer-a.log")) || time.Since(start) > 2*time.Second {
		t.Errorf("non-zero exit: %+v after %s", res, time.Since(start))
	}
	if phaseOf(r, "a") != phaseIdle {
		t.Error("not idle after the failure")
	}
}

func TestIgnoredAppIDsAreNeverCandidatesForComparisonMatching(t *testing.T) {
	setup := func(ignore []string) *rig {
		r := newRig(t, machineDoc("f", "F", "nas", "files", 1, 5, 5, ""))
		r.setStatus("f", statusUp)
		r.h.set.Settle = 100 * time.Millisecond
		r.h.set.IgnoreAppIDs = ignore
		r.l.script = func([]string) {
			r.f.add("waybar-popup", "a menu") // appears together with the viewer's window
			r.f.add("whatever", "F window")
		}
		return r
	}
	// Without the list: two new windows, not told apart.
	r := setup(nil)
	if res := r.h.Open("f"); res.Action != "failed" || !strings.Contains(res.Message, "cannot tell which one") {
		t.Errorf("without the list: %+v", res)
	}
	// With the list: exactly one candidate.
	r = setup([]string{"waybar-popup"})
	res := r.h.Open("f")
	if res.Action != "open" || !strings.Contains(res.Message, "matched by comparison") {
		t.Fatalf("with the list: %+v", res)
	}
	if id, _ := r.h.WindowOf("f"); id != 1 {
		t.Errorf("recorded window %d, want 1 (not the ignored one)", id)
	}
	if len(r.f.moves) != 1 || r.f.moves[0][0] != 1 {
		t.Errorf("the ignored window was moved: %v", r.f.moves)
	}
	// The same in the late state: an ignored window does not end the wait early or get adopted.
	r = newRig(t, machineDoc("f", "F", "nas", "files", 1, 5, 5, ""))
	r.setStatus("f", statusUp)
	r.h.set.WindowWait, r.h.set.LateGrace, r.h.set.Settle = 200*time.Millisecond, 5*time.Second, 100*time.Millisecond
	r.h.set.IgnoreAppIDs = []string{"waybar-popup"}
	r.l.script = func([]string) {
		go func() {
			time.Sleep(300 * time.Millisecond)
			r.f.add("waybar-popup", "a menu")
			time.Sleep(400 * time.Millisecond)
			r.f.add("whatever", "F window")
		}()
	}
	if res := r.h.Open("f"); res.Action != "late" {
		t.Fatalf("%+v", res)
	}
	waitFor(t, "adoption", 3*time.Second, func() bool { _, ok := r.h.WindowOf("f"); return ok })
	if id, _ := r.h.WindowOf("f"); id != 1 {
		t.Errorf("late path adopted window %d, want 1", id)
	}
	// A named viewer is not affected by the list.
	r = newRig(t, machineDoc("a", "A", "ai", "moonlight", 1, 1, 1, ""))
	r.setStatus("a", statusUp)
	r.h.set.IgnoreAppIDs = []string{"hubos-a"}
	if res := r.h.Open("a"); res.Action != "open" {
		t.Errorf("named viewer with its own name in the list: %+v", res)
	}
}
