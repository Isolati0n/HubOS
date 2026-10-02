package hub

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"hubos/internal/driftwm"
	"hubos/internal/probe"
)

// Title-matched viewers: foot stands in for Moonlight. Moonlight's window name
// is fixed (com.moonlight_stream.Moonlight) and its title is "<host name> -
// Moonlight", so the fake viewer sets exactly those two and nothing else.

const moonDoc = `format = 1
[default_ports]
moonlight = 47989
[[viewer]]
id = "moon"
programs = ["moonlight"]
command = ["foot", "--app-id=com.moonlight_stream.Moonlight", "--title={id} - Moonlight", "--", "sleep", "infinity"]
sets_name = false
title_match = "{id} - Moonlight"
[[viewer]]
id = "other"
programs = ["files"]
command = ["foot", "--app-id=ignored", "--", "sleep", "infinity"]
`

func moonRig(t *testing.T, machines ...string) *rig {
	r := newRigViewers(t, moonDoc, machines...)
	for _, id := range []string{"a", "b", "f"} {
		if r.h.byID[id] != nil {
			r.setStatus(id, statusUp)
		}
	}
	return r
}

func moonMachines() []string {
	return []string{
		machineDoc("a", "A", "ai", "moonlight", 0, 1, 1, ""),
		machineDoc("b", "B", "ai", "moonlight", 0, 2, 2, ""),
	}
}

func winOf(r *rig, id string) (winRec, bool) {
	r.h.mu.Lock()
	defer r.h.mu.Unlock()
	if s := r.h.byID[id]; s != nil && s.win != nil {
		return *s.win, true
	}
	return winRec{}, false
}

func titleOfWindow(r *rig, id int) string {
	r.f.mu.Lock()
	defer r.f.mu.Unlock()
	for _, w := range r.f.windows {
		if w.ID == id {
			return w.Title
		}
	}
	return ""
}

func TestTitleMatchTwoMachinesOpenedAtOnceGetTheirOwnWindows(t *testing.T) {
	r := moonRig(t, moonMachines()...)
	// Both windows carry the same fixed app-id. B's window comes first.
	var mu sync.Mutex
	r.l.script = func(args []string) {
		title := ""
		for _, a := range args {
			if v, ok := strings.CutPrefix(a, "--title="); ok {
				title = v
			}
		}
		delay := 300 * time.Millisecond
		if title == "b - Moonlight" {
			delay = 60 * time.Millisecond
		}
		go func() {
			time.Sleep(delay)
			mu.Lock()
			defer mu.Unlock()
			r.f.add("com.moonlight_stream.Moonlight", title)
		}()
	}
	var ra, rb OpenResult
	var wg sync.WaitGroup
	wg.Add(2)
	start := time.Now()
	go func() { defer wg.Done(); ra = r.h.Open("a") }()
	go func() { defer wg.Done(); rb = r.h.Open("b") }()
	wg.Wait()
	t.Logf("a: %+v\nb: %+v\nboth took %s", ra, rb, time.Since(start))
	if ra.Action != "open" || rb.Action != "open" || !strings.Contains(ra.Message, "matched by title") {
		t.Fatalf("a=%+v b=%+v", ra, rb)
	}
	for id, want := range map[string]string{"a": "a - Moonlight", "b": "b - Moonlight"} {
		w, ok := winOf(r, id)
		if !ok || w.By != "title" || titleOfWindow(r, w.Window) != want {
			t.Errorf("%s: recorded %+v (window title %q), want a window titled %q", id, w, titleOfWindow(r, w.Window), want)
		}
	}
	wa, _ := winOf(r, "a")
	wb, _ := winOf(r, "b")
	if wa.Window == wb.Window {
		t.Error("both machines got the same window")
	}
	// Each window was placed at its own machine's home.
	homes := map[int][2]int{}
	for _, m := range r.f.moves {
		homes[m[0]] = [2]int{m[1], m[2]}
	}
	if homes[wa.Window] != [2]int{1, 1} || homes[wb.Window] != [2]int{2, 2} {
		t.Errorf("placement: %v", r.f.moves)
	}
	// They did not wait for each other: B's window came first, and the whole
	// thing took about as long as the slowest window, not the sum.
	if time.Since(start) > 2*time.Second {
		t.Errorf("slow: %s", time.Since(start))
	}
}

func TestTitleMatchTwoWindowsWithTheSameTitleGiveAWarningAndNoGuess(t *testing.T) {
	r := moonRig(t, moonMachines()...)
	r.l.script = func(args []string) {
		r.f.add("com.moonlight_stream.Moonlight", "a - Moonlight")
		r.f.add("com.moonlight_stream.Moonlight", "a - Moonlight")
	}
	res := r.h.Open("a")
	t.Logf("%+v", res)
	if res.Action != "failed" || !strings.Contains(res.Message, `2 new windows are titled "a - Moonlight"`) || !strings.Contains(res.Message, "cannot tell which one") {
		t.Fatalf("%+v", res)
	}
	if _, ok := winOf(r, "a"); ok || len(r.f.moves) != 0 {
		t.Errorf("a guess was made: moves %v", r.f.moves)
	}
	if phaseOf(r, "a") != phaseUnmatched {
		t.Error("the machine must be unidentified")
	}
	// The warning in the menu and the tooltip.
	st, _ := r.f.State()
	r.h.syncWindows(st)
	menu := strings.Join(r.h.List(false, ""), "\n")
	if !strings.Contains(menu, `WARNING: 2 windows are titled "a - Moonlight" (A)`) {
		t.Errorf("menu:\n%s", menu)
	}
	if !strings.Contains(r.h.Status().Tooltip, `2 windows are titled "a - Moonlight"`) {
		t.Errorf("tooltip: %q", r.h.Status().Tooltip)
	}
	// Restart: a second hubd adopts nothing for A (two windows), and does not guess.
	h2 := New(r.inv, r.vt, r.f, r.l.launch, r.set, "")
	h2.driftwmUp = true
	if err := h2.Adopt(); err != nil {
		t.Fatal(err)
	}
	if _, ok := h2.WindowOf("a"); ok {
		t.Error("adopted one of two same-titled windows")
	}
}

func TestTitleMatchAdoptionAfterHubdIsKilled(t *testing.T) {
	r := moonRig(t, moonMachines()...)
	if res := r.h.Open("a"); res.Action != "open" || !strings.Contains(res.Message, "matched by title") {
		t.Fatalf("%+v", res)
	}
	if res := r.h.Open("b"); res.Action != "open" {
		t.Fatalf("%+v", res)
	}
	wa, _ := winOf(r, "a")
	wb, _ := winOf(r, "b")
	// A stranger's window with another title is left alone.
	other := r.f.add("com.moonlight_stream.Moonlight", "something else - Moonlight")
	launches := r.l.count()
	// kill -9: no record file survives for the new hubd (a different driftwm
	// instance would also ignore it), the windows are all that is left.
	h2 := New(r.inv, r.vt, r.f, r.l.launch, r.set, "")
	h2.driftwmUp = true
	if err := h2.Adopt(); err != nil {
		t.Fatal(err)
	}
	for id, want := range map[string]int{"a": wa.Window, "b": wb.Window} {
		got, ok := h2.WindowOf(id)
		if !ok || got != want {
			t.Errorf("%s: adopted %d %v, want %d", id, got, ok, want)
		}
	}
	h2.mu.Lock()
	by := h2.byID["a"].win.By
	h2.mu.Unlock()
	if by != "title" {
		t.Errorf("adopted by %q", by)
	}
	if r.l.count() != launches || len(r.f.closed) != 0 {
		t.Error("adoption started or closed something")
	}
	_ = other
	// And opening A now goes to its window instead of starting a viewer.
	h2.mu.Lock()
	h2.byID["a"].status = statusUp
	h2.mu.Unlock()
	if res := h2.Open("a"); res.Action != "went" || r.l.count() != launches {
		t.Errorf("open after adoption: %+v launches %d->%d", res, launches, r.l.count())
	}
	// With the record file kept (same driftwm), the record path adopts too.
	h3 := New(r.inv, r.vt, r.f, r.l.launch, r.set, r.rec)
	h3.driftwmUp = true
	h3.Adopt()
	if _, ok := h3.WindowOf("b"); !ok {
		t.Error("record-based adoption failed")
	}
}

func TestTitleMatchLateWindowIsAdoptedAndDoesNotLockComparisonViewers(t *testing.T) {
	r := moonRig(t, append(moonMachines(), machineDoc("f", "F", "nas", "files", 1, 5, 5, ""))...)
	r.h.set.WindowWait, r.h.set.LateGrace = 200*time.Millisecond, 5*time.Second
	r.l.script = func(args []string) {
		for _, a := range args {
			if v, ok := strings.CutPrefix(a, "--title="); ok {
				go func() { time.Sleep(900 * time.Millisecond); r.f.add("com.moonlight_stream.Moonlight", v) }()
			}
		}
	}
	first := r.h.Open("a")
	t.Logf("%+v", first)
	if first.Action != "late" || phaseOf(r, "a") != phaseLate {
		t.Fatalf("%+v", first)
	}
	// While A is late, a comparison-matched viewer is NOT refused, because a
	// title-matched window can be told apart.
	r.l.script = nil
	if res := r.h.Open("f"); res.Action != "open" {
		t.Errorf("comparison viewer while a title-matched one is late: %+v", res)
	}
	waitFor(t, "late window adopted", 3*time.Second, func() bool { _, ok := winOf(r, "a"); return ok })
	w, _ := winOf(r, "a")
	if w.By != "title" || titleOfWindow(r, w.Window) != "a - Moonlight" {
		t.Errorf("recorded %+v", w)
	}
	fw, ok := winOf(r, "f")
	if !ok || fw.By != "comparison" || fw.Window == w.Window {
		t.Errorf("f: %+v %v", fw, ok)
	}

	// The other way round: a comparison viewer that is late must not take a
	// title-matched machine's window for its own.
	r = moonRig(t, append(moonMachines(), machineDoc("f", "F", "nas", "files", 1, 5, 5, ""))...)
	r.h.set.WindowWait, r.h.set.LateGrace, r.h.set.Settle = 200*time.Millisecond, 5*time.Second, 100*time.Millisecond
	r.l.script = func(args []string) {
		if strings.Contains(strings.Join(args, " "), "ignored") {
			go func() { time.Sleep(900 * time.Millisecond); r.f.add("whatever", "F window") }()
			return
		}
		r.f.add("com.moonlight_stream.Moonlight", "a - Moonlight")
	}
	if res := r.h.Open("f"); res.Action != "late" {
		t.Fatalf("%+v", res)
	}
	if res := r.h.Open("a"); res.Action != "open" {
		t.Fatalf("a: %+v", res)
	}
	waitFor(t, "f adopted", 3*time.Second, func() bool { _, ok := winOf(r, "f"); return ok })
	fw, _ = winOf(r, "f")
	aw, _ := winOf(r, "a")
	if titleOfWindow(r, fw.Window) != "F window" || titleOfWindow(r, aw.Window) != "a - Moonlight" {
		t.Errorf("wrong windows: f=%q a=%q", titleOfWindow(r, fw.Window), titleOfWindow(r, aw.Window))
	}
}

func TestTitleMatchNoWindowWithTheRightTitleSaysSo(t *testing.T) {
	r := moonRig(t, moonMachines()...)
	r.h.set.WindowWait, r.h.set.LateGrace = 300*time.Millisecond, 0
	r.l.script = func([]string) { r.f.add("com.moonlight_stream.Moonlight", "Moonlight") }
	res := r.h.Open("a")
	t.Logf("%+v", res)
	if res.Action != "failed" || !strings.Contains(res.Message, `its title is "Moonlight", not "a - Moonlight": check title_match`) {
		t.Errorf("%+v", res)
	}
	if _, ok := winOf(r, "a"); ok {
		t.Error("a window with another title was taken")
	}
}

func TestMachineWithoutSessionIsRefusedBeforeAnythingStarts(t *testing.T) {
	vdoc := `format = 1
[default_ports]
moonlight = 47989
[[viewer]]
id = "moon"
programs = ["moonlight"]
command = ["foot", "--title={id} - Moonlight", "--", "moonlight", "stream", "{address}", "{session}"]
title_match = "{id} - Moonlight"
`
	r := newRigViewers(t, vdoc, moonMachines()[0], machineDoc("b", "B", "ai", "moonlight", 0, 2, 2, "session = \"Desktop\"\n"))
	r.setStatus("a", statusUp)
	r.setStatus("b", statusUp)
	res := r.h.Open("a")
	if res.Action != "refused" || !strings.Contains(res.Message, "a has no session in the inventory, and its viewer command needs one") || r.l.count() != 0 {
		t.Errorf("%+v launches=%d", res, r.l.count())
	}
	res = r.h.Open("b")
	if res.Action != "open" || r.l.count() != 1 || r.l.calls[0][len(r.l.calls[0])-1] != "Desktop" {
		t.Errorf("%+v %q", res, r.l.calls)
	}
}

// ---- default check ports ----

func TestCheckPortIsTheMachinesOwnOrTheDefaultPortsTable(t *testing.T) {
	vdoc := strings.Replace(moonDoc, "moonlight = 47989\n", "moonlight = 47989\nfiles = 445\n", 1) + `
[[viewer]]
id = "rv"
programs = ["spice", "ssh"]
command = ["foot", "--", "sleep", "infinity"]
`
	r := newRigViewers(t, vdoc,
		machineDoc("a", "A", "ai", "moonlight", 0, 1, 1, ""),    // table: moonlight 47989
		machineDoc("b", "B", "ai", "moonlight", 4000, 2, 2, ""), // own port wins
		machineDoc("n", "NAS", "nas", "files", 0, 3, 3, ""),     // table: files 445
		machineDoc("h", "Host", "vm-host", "ssh", 22, 4, 4, ""),
		machineDoc("g", "G", "guest", "spice", 0, 5, 5, "host = \"h\"\nlifetime = \"ephemeral\"\n"), // no spice default
	)
	r.h.set.ProbeCap = 8
	var mu sync.Mutex
	got := map[string]bool{}
	r.h.set.Prober = func(ctx context.Context, targets []probe.Target, timeout time.Duration, limit int, done func(i int, r probe.Result)) {
		mu.Lock()
		for _, tg := range targets {
			got[tg.Address+":"+itoa(tg.Port)] = true
		}
		mu.Unlock()
		for i := range targets {
			done(i, probe.Result{Up: true})
		}
	}
	r.h.ProbeRound(r.ctx)
	want := map[string]bool{"127.0.0.1:47989": true, "127.0.0.1:4000": true, "127.0.0.1:445": true, "127.0.0.1:22": true}
	if len(got) != len(want) {
		t.Errorf("targets: %v", got)
	}
	for k := range want {
		if !got[k] {
			t.Errorf("missing target %s in %v", k, got)
		}
	}
	r.h.mu.Lock()
	s := r.h.byID["g"]
	st, reason := s.status, s.reason
	r.h.mu.Unlock()
	if st != statusNotChecked || reason != "no port in the inventory and no default port for spice" {
		t.Errorf("guest: status %v reason %q", st, reason)
	}
	// It is counted as not checked, not as down.
	if c := r.h.Counts(); c.NotChecked != 1 || c.Up != 4 || c.Down != 0 {
		t.Errorf("counts: %+v", c)
	}
	// The NAS is opened with the table port in {port}-less commands too: it is checked, so it can be opened.
	r.setStatus("n", statusUp)
	if res := r.h.Open("n"); res.Action == "refused" && strings.Contains(res.Message, "no port") {
		t.Errorf("%+v", res)
	}
}

// ---- the clean-exit bug in watchLate ----

func TestCleanExitDuringTheGraceKeepsWaitingAndAdoptsTheLateWindow(t *testing.T) {
	r := lateRig(t)
	r.h.set.LogDir = t.TempDir()
	r.l.script = lateWindow(r, 1200*time.Millisecond) // after the 300 ms wait, well inside the grace
	first := r.h.Open("a")
	if first.Action != "late" || phaseOf(r, "a") != phaseLate {
		t.Fatalf("first: %+v", first)
	}
	// The viewer was still running when the grace began. Now it exits cleanly.
	r.l.exit(0, nil)
	time.Sleep(400 * time.Millisecond)
	if phaseOf(r, "a") != phaseLate {
		t.Fatal("a clean exit during the grace ended the wait")
	}
	if res := r.h.Open("a"); res.Action != "busy" || r.l.count() != 1 {
		t.Errorf("open while waiting: %+v launches=%d", res, r.l.count())
	}
	waitFor(t, "the late window to be adopted", 4*time.Second, func() bool { _, ok := r.h.WindowOf("a"); return ok })
	if windowCount(r) != 1 || r.l.count() != 1 {
		t.Errorf("windows=%d launches=%d", windowCount(r), r.l.count())
	}
	waitFor(t, "idle", time.Second, func() bool { return phaseOf(r, "a") == phaseIdle })

	// A clean exit and no window at all: the wait runs to the end of the grace
	// (it did not stop at the exit), then the machine is unidentified.
	r = lateRig(t)
	r.h.set.LateGrace = 1500 * time.Millisecond
	r.l.script = func([]string) {}
	if first := r.h.Open("a"); first.Action != "late" {
		t.Fatalf("%+v", first)
	}
	began := time.Now()
	r.l.exit(0, nil)
	time.Sleep(600 * time.Millisecond)
	if phaseOf(r, "a") != phaseLate {
		t.Fatal("the wait ended at the clean exit")
	}
	waitFor(t, "the grace to end", 4*time.Second, func() bool { return phaseOf(r, "a") == phaseUnmatched })
	if time.Since(began) < 1000*time.Millisecond {
		t.Errorf("the wait ended after %s, before the grace", time.Since(began))
	}
}

func TestNonZeroExitDuringTheGraceIsAFailureWithTheLogPath(t *testing.T) {
	r := lateRig(t)
	r.h.set.LogDir = t.TempDir()
	r.l.script = func([]string) {}
	if first := r.h.Open("a"); first.Action != "late" {
		t.Fatalf("%+v", first)
	}
	r.l.exit(0, errors.New("exit status 7"))
	waitFor(t, "failure", 2*time.Second, func() bool { return phaseOf(r, "a") == phaseIdle })
	st := r.h.Status()
	if !strings.Contains(st.Tooltip, "exit status 7") || !strings.Contains(st.Tooltip, "viewer-a.log") || st.Class != "alert" {
		t.Errorf("tooltip: %+v", st)
	}
}

var _ = driftwm.Window{}
