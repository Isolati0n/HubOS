package hub

import (
	"strings"
	"testing"
	"time"
)

// The open marker: every machine line starts with a filled dot when hubd holds
// a window for the machine and an empty dot when it does not. The marker sits
// inside the indent; the id, name and status columns do not move.

// lineOf returns the machine line of an id in a list, or "".
func lineOf(lines []string, id string) string {
	for _, l := range lines {
		f := strings.Fields(l)
		if len(f) >= 2 && (f[0] == MarkerOpen || f[0] == MarkerClosed) && f[1] == id {
			return l
		}
	}
	return ""
}

func markerRig(t *testing.T) *rig {
	r := newRig(t,
		machineDoc("a", "A Box", "ai", "moonlight", 1, 1, 1, ""),
		machineDoc("b", "B Box", "ai", "moonlight", 1, 2, 2, ""),
		machineDoc("c", "C Desk", "desktop", "moonlight", 1, 3, 3, ""))
	for _, id := range []string{"a", "b", "c"} {
		r.setStatus(id, statusUp)
	}
	return r
}

func TestMarkerOpenIsFilledClosedIsEmpty(t *testing.T) {
	r := markerRig(t)
	before := r.h.List(false, "")
	for _, id := range []string{"a", "b", "c"} {
		if l := lineOf(before, id); !strings.HasPrefix(l, " "+MarkerClosed+" "+id) {
			t.Errorf("%s before opening: %q", id, l)
		}
	}
	// The hub's own line has no dot at all (owner decision), and its columns stay where they were.
	hubLine := ""
	for _, l := range before {
		if strings.HasPrefix(l, "   hub ") {
			hubLine = l
		}
	}
	if hubLine == "" || strings.ContainsAny(hubLine, MarkerOpen+MarkerClosed) {
		t.Errorf("the hub line (three spaces, no dot) is missing or has a dot: %q\n%s", hubLine, strings.Join(before, "\n"))
	}
	r.h.Open("a")
	r.h.Open("c")
	after := r.h.List(false, "")
	if l := lineOf(after, "a"); !strings.HasPrefix(l, " "+MarkerOpen+" a ") || strings.Contains(l, "[open]") {
		t.Errorf("a open: %q", l)
	}
	if l := lineOf(after, "c"); !strings.HasPrefix(l, " "+MarkerOpen+" c ") {
		t.Errorf("c open: %q", l)
	}
	if l := lineOf(after, "b"); !strings.HasPrefix(l, " "+MarkerClosed+" b ") {
		t.Errorf("b closed: %q", l)
	}
	// The columns did not move: id at column 3, name at column 20 (as before).
	got := []rune(lineOf(after, "a"))
	if string(got[3:4]) != "a" || !strings.HasPrefix(string(got[20:]), "A Box") {
		t.Errorf("columns moved: %q", string(got))
	}
	// The marker does not change the order inside a group: a and b are both up,
	// so inventory order stays (a before b) although a is open.
	joined := strings.Join(after, "\n")
	if strings.Index(joined, " a ") > strings.Index(joined, " b ") {
		t.Errorf("order changed:\n%s", joined)
	}
	// The flat list carries the marker too.
	if l := lineOf(r.h.List(true, ""), "a"); !strings.HasPrefix(l, " "+MarkerOpen+" a ") {
		t.Errorf("flat: %q", l)
	}
}

func TestMarkerWindowClosedByHandTurnsEmpty(t *testing.T) {
	r := markerRig(t)
	r.h.Open("a")
	w, _ := r.h.WindowOf("a")
	if l := lineOf(r.h.List(false, ""), "a"); !strings.HasPrefix(l, " "+MarkerOpen+" ") {
		t.Fatalf("not open: %q", l)
	}
	r.f.remove(w) // the owner closed the window with the viewer's own close button
	st, _ := r.f.State()
	r.h.syncWindows(st)
	if l := lineOf(r.h.List(false, ""), "a"); !strings.HasPrefix(l, " "+MarkerClosed+" a ") {
		t.Errorf("after closing by hand: %q", l)
	}
	// hubd end closes it too: the marker changes the same way.
	r.h.Open("a")
	if l := lineOf(r.h.List(false, ""), "a"); !strings.HasPrefix(l, " "+MarkerOpen+" ") {
		t.Fatalf("not open again: %q", l)
	}
	if res := r.h.End("a"); res.Action != "end" {
		t.Fatalf("end: %+v", res)
	}
	if l := lineOf(r.h.List(false, ""), "a"); !strings.HasPrefix(l, " "+MarkerClosed+" a ") {
		t.Errorf("after hubd end: %q", l)
	}
}

func TestMarkerWindowAppearsLate(t *testing.T) {
	r := lateRig(t)
	r.l.script = lateWindow(r, 600*time.Millisecond)
	if first := r.h.Open("a"); first.Action != "late" {
		t.Fatalf("first: %+v", first)
	}
	// While the viewer is starting the window is not open: empty marker and the waiting text.
	l := lineOf(r.h.List(false, ""), "a")
	if !strings.HasPrefix(l, " "+MarkerClosed+" a ") || !strings.Contains(l, "[waiting for its window]") {
		t.Errorf("while late: %q", l)
	}
	waitFor(t, "the late window to be adopted", 3*time.Second, func() bool { _, ok := r.h.WindowOf("a"); return ok })
	l = lineOf(r.h.List(false, ""), "a")
	if !strings.HasPrefix(l, " "+MarkerOpen+" a ") || strings.Contains(l, "waiting") {
		t.Errorf("after the window came: %q", l)
	}
}

func TestMarkerWithTheDuplicateTitleWarning(t *testing.T) {
	// hubd knows one of two same-named windows: the marker stays filled and the warning shows.
	r := newRig(t, machineDoc("a", "A name", "ai", "moonlight", 1, 1, 1, ""))
	r.setStatus("a", statusUp)
	r.h.Open("a")
	r.f.add("hubos-a", "A name")
	st, _ := r.f.State()
	r.h.syncWindows(st)
	menu := r.h.List(false, "")
	if !strings.HasPrefix(menu[0], "! WARNING: 2 windows are named hubos-a") {
		t.Fatalf("no warning: %q", menu[0])
	}
	if l := lineOf(menu, "a"); !strings.HasPrefix(l, " "+MarkerOpen+" a ") {
		t.Errorf("known window, duplicate warning: %q", l)
	}

	// hubd knows none of two same-titled windows (title-matched viewer, both
	// already there when hubd started; not guessed): empty marker and the warning.
	tr := moonRig(t, moonMachines()...)
	tr.f.add("com.moonlight_stream.Moonlight", "a - Moonlight")
	tr.f.add("com.moonlight_stream.Moonlight", "a - Moonlight")
	if err := tr.h.Adopt(); err != nil {
		t.Fatal(err)
	}
	st, _ = tr.f.State()
	tr.h.syncWindows(st)
	menu = tr.h.List(false, "")
	if !strings.Contains(menu[0], `WARNING: 2 windows are titled "a - Moonlight"`) {
		t.Fatalf("no title warning: %q", menu[0])
	}
	if l := lineOf(menu, "a"); !strings.HasPrefix(l, " "+MarkerClosed+" a ") {
		t.Errorf("unknown windows, duplicate warning: %q", l)
	}
}

func TestPickAMarkedLineGoesToTheWindowOrOpens(t *testing.T) {
	r := markerRig(t)
	r.h.Open("a")
	launches := r.l.count()
	lines := r.h.List(false, "")
	// The filled line goes to the window; no second viewer.
	if res := r.h.Pick(lineOf(lines, "a")); res.Action != "went" || r.l.count() != launches {
		t.Errorf("filled line: %+v launches %d->%d", res, launches, r.l.count())
	}
	// The empty line opens the machine.
	if res := r.h.Pick(lineOf(lines, "b")); res.Action != "open" || r.l.count() != launches+1 {
		t.Errorf("empty line: %+v", res)
	}
	// A line without the marker (an older menu, a script) still works.
	if res := r.h.Pick("   c                C Desk                   UP"); res.Action != "open" {
		t.Errorf("no marker: %+v", res)
	}
	// Nothing but a marker, a marker with an unknown id, a marker with no indent: ignored.
	for name, line := range map[string]string{
		"marker only":        " " + MarkerOpen + " ",
		"marker only closed": "   " + MarkerClosed,
		"unknown id":         " " + MarkerOpen + " nobody   Nobody   UP",
		"no indent":          MarkerOpen + " a   A Box   UP",
		"marker then name":   " " + MarkerOpen + " A Box",
	} {
		before := r.l.count()
		if res := r.h.Pick(line); res.Action != "ignored" || r.l.count() != before {
			t.Errorf("%s: %+v", name, res)
		}
	}
}

func TestSearchStillMatchesByNameAndIdWithTheMarker(t *testing.T) {
	r := markerRig(t)
	r.h.Open("a")
	// By name, in any case: both Boxes are found, each with its own marker.
	lines := r.h.List(false, "box")
	if l := lineOf(lines, "a"); !strings.HasPrefix(l, " "+MarkerOpen+" a ") {
		t.Errorf("a: %q", l)
	}
	if l := lineOf(lines, "b"); !strings.HasPrefix(l, " "+MarkerClosed+" b ") {
		t.Errorf("b: %q", l)
	}
	if lineOf(lines, "c") != "" {
		t.Error("C Desk matched 'box'")
	}
	// By id too.
	if lineOf(r.h.List(false, "B"), "b") == "" {
		t.Error("id search broke")
	}
	// The marker is not part of what hubd searches: typing a dot finds nothing.
	for _, m := range []string{MarkerOpen, MarkerClosed} {
		for _, l := range r.h.List(false, m) {
			if lineOf([]string{l}, "a") != "" || lineOf([]string{l}, "b") != "" || lineOf([]string{l}, "c") != "" {
				t.Errorf("the marker %q matched a machine: %q", m, l)
			}
		}
	}
	// A line picked from the filtered list opens that machine.
	if res := r.h.Pick(lineOf(lines, "b")); res.Action != "open" {
		t.Errorf("pick from search: %+v", res)
	}
}

func TestWindowsOfNoMachineNeverAppearInTheList(t *testing.T) {
	r := markerRig(t)
	// The menu program, the bar, a stray terminal, and a window named like a
	// machine that does not exist.
	r.f.add("wofi", "hub")
	r.f.add("waybar", "waybar")
	r.f.add("foot", "foot")
	r.f.add("hubos-nobody", "Nobody")
	if err := r.h.Adopt(); err != nil {
		t.Fatal(err)
	}
	st, _ := r.f.State()
	r.h.syncWindows(st)
	text := strings.Join(r.h.List(false, ""), "\n") + "\n" + strings.Join(r.h.List(true, ""), "\n")
	for _, bad := range []string{"wofi", "waybar", "foot", "nobody", "Nobody", MarkerOpen} {
		if strings.Contains(text, bad) {
			t.Errorf("%q is in the list:\n%s", bad, text)
		}
	}
	// And an unrelated window never makes a machine look open: opening "a"
	// starts its own viewer instead of "going" to one of those windows.
	if res := r.h.Open("a"); res.Action != "open" {
		t.Errorf("open: %+v", res)
	}
}

func TestPickingAnOpenMachineFocusesItsWindowAndMovesTheView(t *testing.T) {
	r := markerRig(t)
	r.f.modelFocus = true
	r.h.Open("a")
	wa, _ := r.h.WindowOf("a")
	r.f.Move(wa, 0, 0)
	r.h.Open("c")
	wc, _ := r.h.WindowOf("c")
	r.f.Move(wc, 5000, 0) // c far to the right
	r.f.Focus(wc)
	if r.f.camera[0] != 5000 {
		t.Fatalf("setup: the view should be at c, camera %v", r.f.camera)
	}
	// a is off screen. Pick its filled line.
	if res := r.h.Pick(lineOf(r.h.List(false, ""), "a")); res.Action != "went" {
		t.Fatalf("pick: %+v", res)
	}
	if r.f.camera[0] != 0 {
		t.Errorf("the view did not move to a: camera %v", r.f.camera)
	}
	// a is behind c's window: put c on top of it, then pick a.
	r.f.Move(wc, 0, 0)
	r.f.Focus(wc)
	st, _ := r.f.State()
	if top := st.Windows[len(st.Windows)-1]; top.ID != wc {
		t.Fatalf("setup: c should be on top, top is #%d", top.ID)
	}
	if res := r.h.Pick(lineOf(r.h.List(false, ""), "a")); res.Action != "went" {
		t.Fatalf("pick: %+v", res)
	}
	st, _ = r.f.State()
	top := st.Windows[len(st.Windows)-1]
	cur, _ := st.Window(wa)
	if top.ID != wa || !cur.Focused {
		t.Errorf("a is not raised and focused: top #%d, a focused %v", top.ID, cur.Focused)
	}
	if r.l.count() != 2 {
		t.Errorf("a viewer was started (%d launches)", r.l.count())
	}
}
