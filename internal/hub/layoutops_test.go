package hub

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// layoutRig is a hub with three machines (a and b at homes (100,100) and
// (300,300), c far away) and a layout folder.
func newLayoutRig(t *testing.T) *rig {
	t.Helper()
	r := newRig(t,
		machineDoc("a", "A Box", "ai", "moonlight", 1, 100, 100, ""),
		machineDoc("b", "B Box", "ai", "moonlight", 1, 300, 300, ""),
		machineDoc("c", "C Desk", "desktop", "moonlight", 1, 3000, 0, ""))
	r.set.LayoutDir = filepath.Join(t.TempDir(), "layouts")
	r.h = New(r.inv, r.vt, r.f, r.l.launch, r.set, r.rec)
	r.h.driftwmUp = true
	for _, id := range []string{"a", "b", "c"} {
		r.setStatus(id, statusUp)
	}
	return r
}

// restartHub is hubd started again over the same layout folder and the same
// compositor (the compositor keeps its windows; the new hubd adopts them).
func (r *rig) restartHub(t *testing.T) {
	t.Helper()
	r.h = New(r.inv, r.vt, r.f, r.l.launch, r.set, r.rec)
	r.h.driftwmUp = true
	for _, id := range []string{"a", "b", "c"} {
		r.setStatus(id, statusUp)
	}
	if err := r.h.Adopt(); err != nil {
		t.Fatal(err)
	}
}

func (r *rig) winOf(t *testing.T, id string) (x, y, w, h int) {
	t.Helper()
	wid, ok := r.h.WindowOf(id)
	if !ok {
		t.Fatalf("no window for %s", id)
	}
	cur, ok := mustState(t, r.f).Window(wid)
	if !ok {
		t.Fatalf("driftwm has no window %d for %s", wid, id)
	}
	return cur.Position[0], cur.Position[1], cur.Size[0], cur.Size[1]
}

func (r *rig) place(t *testing.T, id string, x, y, w, h int) {
	t.Helper()
	wid, _ := r.h.WindowOf(id)
	r.f.Move(wid, x, y)
	r.f.Resize(wid, w, h)
}

func TestLayoutSaveRecordsOpenWindowsAndTheView(t *testing.T) {
	r := newLayoutRig(t)
	r.h.Open("a")
	r.h.Open("b") // c stays closed
	r.place(t, "a", -500, 40, 800, 600)
	r.place(t, "b", 500, 40, 640, 480)
	r.f.camera, r.f.zoom = [2]float64{12.5, -7}, 0.75
	res := r.h.Layout("save", "work", false)
	if !res.OK || !strings.Contains(res.Message, "2 windows") || !strings.Contains(res.Message, "zoom 0.75") {
		t.Fatalf("%+v", res)
	}
	l, err := r.h.lay.load("work")
	if err != nil {
		t.Fatal(err)
	}
	if len(l.Windows) != 2 || l.Windows[0] != (LayoutWindow{"a", -500, 40, 800, 600}) || l.Windows[1] != (LayoutWindow{"b", 500, 40, 640, 480}) {
		t.Errorf("windows: %+v", l.Windows)
	}
	if l.View == nil || l.View.Zoom != 0.75 || l.View.Cam != [2]float64{12.5, -7} {
		t.Errorf("view: %+v", l.View)
	}
	// an existing name needs --replace
	if res := r.h.Layout("save", "work", false); res.OK || !strings.Contains(res.Message, "--replace") {
		t.Errorf("second save: %+v", res)
	}
	if res := r.h.Layout("save", "work", true); !res.OK {
		t.Errorf("replace: %+v", res)
	}
	if res := r.h.Layout("save", "Bad Name", false); res.OK {
		t.Errorf("bad name accepted: %+v", res)
	}
}

func TestLayoutSaveWithoutDriftwmChangesNothing(t *testing.T) {
	r := newLayoutRig(t)
	r.h.Open("a")
	if res := r.h.Layout("save", "work", false); !res.OK {
		t.Fatal(res.Message)
	}
	before, _ := os.ReadFile(r.h.lay.path("work"))
	r.f.mu.Lock()
	r.f.failState = true
	r.f.mu.Unlock()
	res := r.h.Layout("save", "work", true)
	if res.OK || !strings.Contains(res.Message, "nothing was saved") {
		t.Errorf("%+v", res)
	}
	if after, _ := os.ReadFile(r.h.lay.path("work")); string(after) != string(before) {
		t.Error("the file changed although driftwm was gone")
	}
	if ents, _ := os.ReadDir(r.h.lay.dir); len(ents) != 1 {
		t.Errorf("the folder holds %d entries: %v", len(ents), ents)
	}
}

func TestLayoutApplyMovesOpenWindowsOpensNothingAndSetsTheView(t *testing.T) {
	r := newLayoutRig(t)
	r.h.Open("a")
	r.h.Open("b")
	r.place(t, "a", -500, 40, 800, 600)
	r.place(t, "b", 500, 40, 640, 480)
	r.f.camera, r.f.zoom = [2]float64{12, -7}, 0.75
	r.h.Layout("save", "work", false)
	// the owner rearranges everything, opens c, and changes the view
	r.place(t, "a", 1, 1, 700, 525)
	r.place(t, "b", 2, 2, 700, 525)
	r.f.camera, r.f.zoom = [2]float64{0, 0}, 1
	launches := r.l.count()
	res := r.h.Layout("apply", "work", false)
	if !res.OK || !strings.Contains(res.Message, "moved 2 windows") {
		t.Fatalf("%+v", res)
	}
	if x, y, w, h := r.winOf(t, "a"); [4]int{x, y, w, h} != [4]int{-500, 40, 800, 600} {
		t.Errorf("a: %d %d %d %d", x, y, w, h)
	}
	if x, y, w, h := r.winOf(t, "b"); [4]int{x, y, w, h} != [4]int{500, 40, 640, 480} {
		t.Errorf("b: %d %d %d %d", x, y, w, h)
	}
	if r.f.camera != [2]float64{12, -7} || r.f.zoom != 0.75 {
		t.Errorf("view: %v %v", r.f.camera, r.f.zoom)
	}
	if r.l.count() != launches {
		t.Error("apply started a viewer")
	}
	if r.h.ActiveLayout() != "work" {
		t.Errorf("active: %q", r.h.ActiveLayout())
	}
	// a machine in the layout that is not open is not touched and not opened
	r.h.Layout("delete", "work", false)
	r.h.Open("c")
	r.place(t, "c", 77, 77, 500, 400)
	r.h.Layout("save", "withc", false)
	r.h.End("c")
	res = r.h.Layout("apply", "withc", false)
	if !res.OK || !strings.Contains(res.Message, "not open") || !strings.Contains(res.Message, "c") {
		t.Errorf("%+v", res)
	}
	if _, ok := r.h.WindowOf("c"); ok {
		t.Error("apply opened c")
	}
}

// driftwm refuses a resize while the window is still settling an earlier move or resize ("under an interactive
// move or resize, or still settling one"). That answer is not the end: hubd asks again (seen in the hub image
// test: one of 20 restored windows kept the viewer's first size because the first answer was taken as final).
func TestResizeRefusedWhileTheCompositorIsSettlingIsAskedAgain(t *testing.T) {
	r := newLayoutRig(t)
	r.set.SettleMax = 2 * time.Second
	r.h.set.SettleMax = 2 * time.Second
	r.h.Open("a")
	r.place(t, "a", -500, 40, 800, 600)
	r.h.Layout("save", "one", false)
	r.place(t, "a", 1, 1, 700, 525)
	r.f.mu.Lock()
	r.f.resizeErrors = 3
	r.f.mu.Unlock()
	res := r.h.Layout("apply", "one", false)
	if !res.OK || !strings.Contains(res.Message, "moved 1 window") || strings.Contains(res.Message, "WARNING") {
		t.Fatalf("%+v", res)
	}
	if x, y, w, h := r.winOf(t, "a"); [4]int{x, y, w, h} != [4]int{-500, 40, 800, 600} {
		t.Errorf("a: %d %d %d %d", x, y, w, h)
	}
}

func TestLayoutApplyOrderZoomWaitsThenCamera(t *testing.T) {
	// Setting the camera right after the zoom gives a wrong camera in driftwm (section 2.9): the zoom must be set first and the
	// camera after, both ending as saved.
	r := newLayoutRig(t)
	r.f.camera, r.f.zoom = [2]float64{300, -200}, 0.8
	r.h.Layout("save", "v", false)
	r.f.camera, r.f.zoom = [2]float64{0, 0}, 1
	res := r.h.Layout("apply", "v", false)
	if !res.OK || !strings.Contains(res.Message, "the view is set") {
		t.Fatalf("%+v", res)
	}
	if r.f.camera != [2]float64{300, -200} || r.f.zoom != 0.8 {
		t.Errorf("view %v %v", r.f.camera, r.f.zoom)
	}
}

func TestLayoutGovernsMachinesOpenedLaterUntilClearedOrAnotherIsApplied(t *testing.T) {
	r := newLayoutRig(t)
	r.h.Open("a")
	r.h.Open("c")
	r.place(t, "a", -500, 40, 800, 600)
	r.place(t, "c", 900, 900, 640, 480)
	r.h.Layout("save", "one", false)
	r.place(t, "a", -111, 111, 800, 600)
	r.place(t, "c", 222, 222, 640, 480)
	r.h.Layout("save", "two", false)
	r.h.End("a")
	r.h.End("c")
	// layout "one" is applied while nothing is open: it governs the next opens
	r.h.Layout("apply", "one", false)
	res := r.h.Open("c")
	if res.Action != "open" || !strings.Contains(res.Message, "its place in layout one (900, 900)") {
		t.Fatalf("%+v", res)
	}
	if x, y, w, h := r.winOf(t, "c"); [4]int{x, y, w, h} != [4]int{900, 900, 640, 480} {
		t.Errorf("c: %d %d %d %d", x, y, w, h)
	}
	// b is not in the layout: it goes home
	res = r.h.Open("b")
	if !strings.Contains(res.Message, "at home (300, 300)") {
		t.Errorf("b: %+v", res)
	}
	// another layout replaces the first
	r.h.End("c")
	r.h.Layout("apply", "two", false)
	r.h.Open("c")
	if x, y, _, _ := r.winOf(t, "c"); x != 222 || y != 222 {
		t.Errorf("c under two: %d %d", x, y)
	}
	// cleared: back to home, and open windows are not moved
	r.h.End("c")
	lres := r.h.Layout("clear", "", false)
	if !lres.OK || r.h.ActiveLayout() != "" {
		t.Fatalf("%+v active %q", lres, r.h.ActiveLayout())
	}
	r.h.Open("c")
	if x, y, _, _ := r.winOf(t, "c"); x != 3000 || y != 0 {
		t.Errorf("c after clear: %d %d", x, y)
	}
}

func TestActiveLayoutIsRememberedAcrossARestartOfHubdButNothingReopens(t *testing.T) {
	r := newLayoutRig(t)
	r.h.Open("c")
	r.place(t, "c", 900, 900, 640, 480)
	r.h.Layout("save", "one", false)
	r.h.End("c")
	r.h.Layout("apply", "one", false)
	// hubd restarts (a new process over the same folder): the layout is still active
	launches := r.l.count()
	r.restartHub(t)
	if r.h.ActiveLayout() != "one" {
		t.Fatalf("active after restart: %q", r.h.ActiveLayout())
	}
	if r.l.count() != launches {
		t.Error("a restart opened a window")
	}
	r.h.Open("c")
	if x, y, _, _ := r.winOf(t, "c"); x != 900 || y != 900 {
		t.Errorf("c: %d %d", x, y)
	}
	// cleared is remembered too
	r.h.Layout("clear", "", false)
	r.restartHub(t)
	if r.h.ActiveLayout() != "" {
		t.Errorf("cleared layout came back: %q", r.h.ActiveLayout())
	}
}

func TestDamagedActiveFileIsIgnoredWithAWarningAndHubdStarts(t *testing.T) {
	r := newLayoutRig(t)
	r.h.Open("c")
	r.h.Layout("save", "one", false)
	r.h.Layout("apply", "one", false)
	os.WriteFile(r.h.lay.activePath(), []byte(`{"format":1,"layout":"one","sha`), 0o644) // half-written
	r.restartHub(t)
	if r.h.ActiveLayout() != "" {
		t.Errorf("active: %q", r.h.ActiveLayout())
	}
	w := r.h.Warnings()
	if len(w) != 1 || !strings.Contains(w[0], "moved aside") {
		t.Errorf("warnings: %q", w)
	}
	if m, _ := filepath.Glob(filepath.Join(r.h.lay.dir, "_active.json.corrupt.*")); len(m) != 1 {
		t.Errorf("quarantined: %v", m)
	}
	if st := r.h.Status(); !strings.Contains(st.Tooltip, "WARNING") {
		t.Errorf("not on the bar: %q", st.Tooltip)
	}
	// and a damaged layout the active file names
	r.h.Layout("apply", "one", false)
	os.WriteFile(r.h.lay.path("one"), []byte("{"), 0o644)
	r.restartHub(t)
	if r.h.ActiveLayout() != "" || len(r.h.Warnings()) < 1 {
		t.Errorf("active %q warnings %q", r.h.ActiveLayout(), r.h.Warnings())
	}
	if m, _ := filepath.Glob(filepath.Join(r.h.lay.dir, "one.layout.json.corrupt.*")); len(m) != 1 {
		t.Errorf("quarantined: %v", m)
	}
	// the warning does not repeat at the next start
	r.restartHub(t)
	if w := r.h.Warnings(); len(w) != 0 {
		t.Errorf("warnings again: %q", w)
	}
}

func TestLayoutListAndDelete(t *testing.T) {
	r := newLayoutRig(t)
	r.h.Open("a")
	r.h.Layout("save", "one", false)
	r.h.Layout("save", "two", false)
	r.h.Layout("apply", "two", false)
	res := r.h.Layout("list", "", false)
	if !res.OK || len(res.Lines) != 2 || !strings.HasPrefix(res.Lines[1], "* two") || !strings.HasPrefix(res.Lines[0], "  one") {
		t.Fatalf("%+v", res)
	}
	// a damaged file shows up as a warning in the list and is set aside
	os.WriteFile(r.h.lay.path("one"), []byte("{"), 0o644)
	res = r.h.Layout("list", "", false)
	if len(res.Lines) != 2 || !strings.HasPrefix(res.Lines[1], "WARNING: layout one") {
		t.Errorf("%+v", res)
	}
	// deleting the active layout clears it
	res = r.h.Layout("delete", "two", false)
	if !res.OK || !strings.Contains(res.Message, "was the active layout") || r.h.ActiveLayout() != "" {
		t.Errorf("%+v", res)
	}
	if res := r.h.Layout("delete", "two", false); res.OK || !strings.Contains(res.Message, "no layout named two") {
		t.Errorf("delete twice: %+v", res)
	}
	if res := r.h.Layout("apply", "nope", false); res.OK || !strings.Contains(res.Message, "no layout named nope") {
		t.Errorf("apply missing: %+v", res)
	}
	if res := r.h.Layout("clear", "", false); !res.OK || !strings.Contains(res.Message, "no layout was active") {
		t.Errorf("clear: %+v", res)
	}
	if res := r.h.Layout("frobnicate", "", false); res.OK {
		t.Errorf("unknown sub: %+v", res)
	}
}

func TestLayoutOverlapIsAWarningNotAnError(t *testing.T) {
	r := newLayoutRig(t)
	r.h.Open("a")
	r.h.Open("b")
	r.place(t, "a", 0, 0, 800, 600)
	r.place(t, "b", 100, 50, 800, 600)
	res := r.h.Layout("save", "pile", false)
	if !res.OK || !strings.Contains(res.Message, "WARNING: these windows overlap: a and b") {
		t.Fatalf("%+v", res)
	}
	res = r.h.Layout("apply", "pile", false)
	if !res.OK || !strings.Contains(res.Message, "WARNING: these windows overlap") {
		t.Errorf("apply: %+v", res)
	}
}

func TestLayoutApplySkipsMachinesThatAreNotInTheInventoryAnymore(t *testing.T) {
	r := newLayoutRig(t)
	r.h.Open("a")
	l := sampleLayout("old")
	l.Windows = []LayoutWindow{{"a", 5, 5, 400, 300}, {"gone", 9, 9, 400, 300}}
	if err := r.h.lay.save(l, false); err != nil {
		t.Fatal(err)
	}
	res := r.h.Layout("apply", "old", false)
	if !res.OK || !strings.Contains(res.Message, "moved 1 window") || !strings.Contains(res.Message, "not in the inventory any more, skipped: gone") {
		t.Errorf("%+v", res)
	}
}

func TestLayoutWithoutADriftwmStillBecomesActive(t *testing.T) {
	r := newLayoutRig(t)
	r.h.Open("c")
	r.place(t, "c", 900, 900, 640, 480)
	r.h.Layout("save", "one", false)
	r.h.End("c")
	r.f.mu.Lock()
	r.f.failState = true
	r.f.mu.Unlock()
	res := r.h.Layout("apply", "one", false)
	if !res.OK || !strings.Contains(res.Message, "driftwm is not reachable") || r.h.ActiveLayout() != "one" {
		t.Errorf("%+v active %q", res, r.h.ActiveLayout())
	}
}

func TestLayoutsAreOffWithoutAFolder(t *testing.T) {
	r := newRig(t, machineDoc("a", "A", "ai", "moonlight", 1, 1, 1, ""))
	if res := r.h.Layout("list", "", false); res.OK {
		t.Errorf("%+v", res)
	}
	for _, l := range r.h.List(false, "") {
		if strings.Contains(l, "Layouts") {
			t.Errorf("menu line %q", l)
		}
	}
}

// ---- the Layouts group in the menu ----

func TestMenuLayoutsGroupApplyClearAndFold(t *testing.T) {
	r := newLayoutRig(t)
	r.h.Open("c")
	r.place(t, "c", 900, 900, 640, 480)
	for _, l := range r.h.List(false, "") {
		if strings.Contains(l, "Layouts") {
			t.Errorf("a group with no layouts: %q", l)
		}
	}
	r.h.Layout("save", "one", false)
	r.h.Layout("save", "two", false)
	lines := r.h.List(false, "")
	i := indexOf(lines, "- Layouts (2)")
	if i < 0 || !strings.HasPrefix(lines[i+1], "   = apply one") || !strings.HasPrefix(lines[i+2], "   = apply two") || len(lines) != i+3 {
		t.Fatalf("group:\n%s", strings.Join(lines, "\n"))
	}
	// pick: apply two
	r.place(t, "c", 5, 5, 640, 480)
	p := r.h.Pick(lines[i+2])
	if p.Action != "layout" || r.h.ActiveLayout() != "two" {
		t.Fatalf("%+v active %q", p, r.h.ActiveLayout())
	}
	if x, y, _, _ := r.winOf(t, "c"); x != 900 || y != 900 {
		t.Errorf("c: %d %d", x, y)
	}
	lines = r.h.List(false, "")
	i = indexOf(lines, "- Layouts (2, active: two)")
	if i < 0 || !strings.HasSuffix(lines[i+2], "ACTIVE") || !strings.Contains(lines[len(lines)-1], "= clear the active layout") {
		t.Fatalf("group:\n%s", strings.Join(lines, "\n"))
	}
	// fold
	if p := r.h.Pick(lines[i]); p.Action != "toggle" || !p.Reopen {
		t.Errorf("fold: %+v", p)
	}
	lines = r.h.List(false, "")
	if j := indexOf(lines, "+ Layouts (2, active: two)"); j < 0 || j != len(lines)-1 {
		t.Errorf("folded:\n%s", strings.Join(lines, "\n"))
	}
	r.h.Pick(lines[len(lines)-1]) // open again
	// clear
	lines = r.h.List(false, "")
	if p := r.h.Pick(lines[len(lines)-1]); p.Action != "layout" || r.h.ActiveLayout() != "" {
		t.Errorf("clear: %+v active %q", p, r.h.ActiveLayout())
	}
}

// A layout may be named like a machine. Its menu line must apply the layout and
// never open the machine.
func TestMenuLayoutNamedLikeAMachineAppliesTheLayoutOnly(t *testing.T) {
	r := newLayoutRig(t)
	r.h.Open("a")
	r.h.Layout("save", "c", false) // "c" is a machine id too
	lines := r.h.List(false, "")
	i := indexOf(lines, "- Layouts (1)")
	if i < 0 {
		t.Fatalf("no group:\n%s", strings.Join(lines, "\n"))
	}
	launches := r.l.count()
	p := r.h.Pick(lines[i+1])
	if p.Action != "layout" || r.l.count() != launches || r.h.ActiveLayout() != "c" {
		t.Errorf("%+v launches %d->%d active %q", p, launches, r.l.count(), r.h.ActiveLayout())
	}
	if _, ok := r.h.WindowOf("c"); ok {
		t.Error("the machine c was opened")
	}
	// nonsense lines in the group are ignored
	for _, bad := range []string{"   = apply", "   = ", "   = explode c", "= apply c"} {
		if p := r.h.Pick(bad); p.Action != "ignored" {
			t.Errorf("%q: %+v", bad, p)
		}
	}
}

func indexOf(lines []string, prefix string) int {
	for i, l := range lines {
		if strings.HasPrefix(l, prefix) {
			return i
		}
	}
	return -1
}
