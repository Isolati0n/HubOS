package hub

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"hubos/internal/driftwm"
	"hubos/internal/inventory"
	"hubos/internal/viewers"
)

// fakeComp is a pretend driftwm: a list of windows and what was asked of it.
type fakeComp struct {
	mu           sync.Mutex
	next         int
	windows      []driftwm.Window
	moves        [][3]int
	focused      []int
	closed       []int
	identity     string
	viewport     [2]int
	camera       [2]float64
	zoom         float64
	pid          int          // what PeerPID answers (0 = driftwm not reachable)
	resizeErrors int          // the next resizes are refused with driftwm's "still settling" answer
	earlyMoves   int          // moves made while the window had not drawn yet (its frame was only a title bar)
	stateErrors  int          // the next State calls fail with a timeout (driftwm is slow, not gone)
	moveErrors   int          // the next Move calls fail with a timeout
	failState    bool         // State answers with an error (driftwm gone)
	cascade      map[int]int  // window id -> how many of its next moves the compositor ignores, putting the window at its own spot instead
	stuck        map[int]bool // windows that ignore a close request
	subs         []chan *driftwm.State
	failSub      bool
	// modelFocus makes Focus behave like driftwm's: the window gets the focus
	// flag and is raised to the top of the list (last = on top), and the camera
	// pans to it unless its centre is already in view.
	modelFocus bool
	// focusFirst makes State list the windows the way driftwm does: bottom to top, but with the focused window moved
	// to the front of the list (window_inventory in driftwm's src/state/persistence.rs).
	focusFirst bool
}

func newFake() *fakeComp {
	return &fakeComp{identity: "i1", viewport: [2]int{1280, 800}, stuck: map[int]bool{}, zoom: 1, cascade: map[int]int{}}
}

func (f *fakeComp) add(appID, title string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	id := f.next
	f.next++
	f.windows = append(f.windows, driftwm.Window{ID: id, AppID: appID, Title: title, Size: [2]int{700, 525}})
	return id
}

func (f *fakeComp) remove(id int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i, w := range f.windows {
		if w.ID == id {
			f.windows = append(f.windows[:i], f.windows[i+1:]...)
			break
		}
	}
}

func (f *fakeComp) State() (*driftwm.State, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failState {
		return nil, fmt.Errorf("driftwm is gone")
	}
	if f.stateErrors > 0 {
		f.stateErrors--
		return nil, fmt.Errorf("read unix @->/run/dw/driftwm/ipc-wayland-1.sock: i/o timeout")
	}
	wins := append([]driftwm.Window(nil), f.windows...)
	if f.focusFirst {
		for i, w := range wins {
			if w.Focused {
				wins = append([]driftwm.Window{w}, append(wins[:i:i], wins[i+1:]...)...)
				break
			}
		}
	}
	return &driftwm.State{
		Camera:  f.camera,
		Zoom:    f.zoom,
		Windows: wins,
		Outputs: []driftwm.Output{{Name: "o", Size: f.viewport, Active: true}},
	}, nil
}
func (f *fakeComp) Move(id, x, y int) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.moveErrors > 0 {
		f.moveErrors--
		return fmt.Errorf("read unix @->/run/dw/driftwm/ipc-wayland-1.sock: i/o timeout")
	}
	f.moves = append(f.moves, [3]int{id, x, y})
	for _, w := range f.windows {
		if w.ID == id && w.Size[0] < 64 && w.Size[1] < 64 {
			f.earlyMoves++
		}
	}
	if n := f.cascade[id]; n > 0 {
		f.cascade[id] = n - 1
		x, y = 25, -125 // the compositor's own cascade spot
	}
	for i := range f.windows {
		if f.windows[i].ID == id {
			f.windows[i].Position = [2]int{x, y}
		}
	}
	return nil
}
func (f *fakeComp) Resize(id, w, h int) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.resizeErrors > 0 {
		f.resizeErrors--
		return fmt.Errorf("driftwm: this window is under an interactive move or resize, or still settling one")
	}
	for i := range f.windows {
		if f.windows[i].ID == id {
			f.windows[i].Size = [2]int{w, h}
		}
	}
	return nil
}
func (f *fakeComp) Focus(id int) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.focused = append(f.focused, id)
	if f.modelFocus {
		for i := range f.windows {
			f.windows[i].Focused = f.windows[i].ID == id
		}
		for i, w := range f.windows {
			if w.ID != id {
				continue
			}
			f.windows = append(append([]driftwm.Window(nil), f.windows[:i]...), f.windows[i+1:]...)
			f.windows = append(f.windows, w)
			dx, dy := float64(w.Position[0])-f.camera[0], float64(w.Position[1])-f.camera[1]
			if dx < -float64(f.viewport[0])/2 || dx > float64(f.viewport[0])/2 || dy < -float64(f.viewport[1])/2 || dy > float64(f.viewport[1])/2 {
				f.camera = [2]float64{float64(w.Position[0]), float64(w.Position[1])}
			}
			break
		}
	}
	return nil
}
func (f *fakeComp) SetZoom(z float64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.zoom = z
	return nil
}
func (f *fakeComp) SetCamera(x, y float64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.camera = [2]float64{x, y}
	return nil
}
func (f *fakeComp) PeerPID() (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.pid == 0 {
		return 0, fmt.Errorf("no driftwm")
	}
	return f.pid, nil
}
func (f *fakeComp) Close(id int) error {
	f.mu.Lock()
	f.closed = append(f.closed, id)
	stuck := f.stuck[id]
	f.mu.Unlock()
	if !stuck {
		f.remove(id)
	}
	return nil
}
func (f *fakeComp) Identity() (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.identity, nil
}
func (f *fakeComp) Subscribe(ctx context.Context) (<-chan *driftwm.State, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failSub {
		return nil, fmt.Errorf("no driftwm")
	}
	ch := make(chan *driftwm.State, 4)
	f.subs = append(f.subs, ch)
	return ch, nil
}

// emit sends the current state to every subscriber, as the compositor does on a change.
func (f *fakeComp) emit() {
	st, err := f.State()
	if err != nil {
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, c := range f.subs {
		select {
		case c <- st:
		default:
		}
	}
}

// crash is a compositor that died: every window and every stream is gone and
// nobody answers until restart.
func (f *fakeComp) crash() {
	f.mu.Lock()
	f.failSub, f.failState = true, true
	f.windows = nil
	f.camera, f.zoom = [2]float64{}, 1
	f.mu.Unlock()
	f.endStreams()
}

// restart is the new compositor: empty, answering, with a new identity.
func (f *fakeComp) restart() {
	f.mu.Lock()
	f.failSub, f.failState = false, false
	f.identity += "n"
	f.next = 0
	f.mu.Unlock()
}

func (f *fakeComp) endStreams() {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, c := range f.subs {
		close(c)
	}
	f.subs = nil
}

// fakeLauncher records launches and makes windows appear in the fake.
type fakeLauncher struct {
	f      *fakeComp
	mu     sync.Mutex
	calls  [][]string
	exits  []chan error        // one per launch: send to make that viewer process exit
	gate   chan struct{}       // if set, a launch waits for it
	script func(args []string) // what windows to create; default: one named by --app-id=
}

func (l *fakeLauncher) launch(id string, args []string) (*Proc, error) {
	l.mu.Lock()
	l.calls = append(l.calls, args)
	l.mu.Unlock()
	if l.gate != nil {
		<-l.gate
	}
	if l.script != nil {
		l.script(args)
	} else {
		var app, title string
		for _, a := range args {
			if v, ok := strings.CutPrefix(a, "--app-id="); ok {
				app = v
			}
			if v, ok := strings.CutPrefix(a, "--title="); ok {
				title = v
			}
		}
		l.f.add(app, title)
	}
	ch := make(chan error, 1)
	l.mu.Lock()
	l.exits = append(l.exits, ch)
	l.mu.Unlock()
	return &Proc{Exited: ch}, nil
}

// exit makes the i-th started viewer process exit (err nil = clean exit).
func (l *fakeLauncher) exit(i int, err error) {
	// the launch is counted before its process exists (a slow machine can be between the two)
	for n := 0; n < 400; n++ {
		l.mu.Lock()
		have := i < len(l.exits)
		l.mu.Unlock()
		if have {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	l.mu.Lock()
	ch := l.exits[i]
	l.mu.Unlock()
	ch <- err
}
func (l *fakeLauncher) count() int { l.mu.Lock(); defer l.mu.Unlock(); return len(l.calls) }

const viewersDoc = `format = 1
[[viewer]]
id = "named"
programs = ["moonlight", "ssh"]
command = ["foot", "--app-id={app_id}", "--title={name}", "--", "sleep", "infinity"]
sets_name = true
[[viewer]]
id = "unnamed"
programs = ["files"]
command = ["foot", "--app-id=ignored", "--", "sleep", "infinity"]
[[viewer]]
id = "needsport"
programs = ["vnc"]
command = ["viewer", "{address}:{port}"]
`

func machineDoc(id, name, role, open string, port int, x, y int, extra string) string {
	s := fmt.Sprintf("[[machine]]\nid = %q\nname = %q\nrole = %q\naddress = \"127.0.0.1\"\nopen = [%q]\nhome = { x = %d, y = %d }\n", id, name, role, open, x, y)
	if port > 0 {
		s += fmt.Sprintf("port = %d\n", port)
	}
	return s + extra
}

type rig struct {
	h    *Hub
	f    *fakeComp
	l    *fakeLauncher
	rec  string
	set  Settings
	inv  *inventory.Inventory
	vt   *viewers.Table
	ctx  context.Context
	stop context.CancelFunc
}

func testSettings() Settings {
	s := DefaultSettings()
	s.WindowWait = 2 * time.Second
	s.Settle = 20 * time.Millisecond
	s.CloseWait = 300 * time.Millisecond
	s.ProbeTimeout = 300 * time.Millisecond
	s.SettleStep = time.Millisecond
	s.SettleMax = 300 * time.Millisecond
	s.SizedWait = 300 * time.Millisecond
	s.FastReconnect = 5 * time.Millisecond
	s.MessageTTL = time.Minute
	return s
}

func newRig(t *testing.T, machines ...string) *rig {
	t.Helper()
	return newRigViewers(t, viewersDoc, machines...)
}

func newRigViewers(t *testing.T, vdoc string, machines ...string) *rig {
	t.Helper()
	doc := "format = 1\n" + machineDoc("hub", "Hub", "hub", "none", 0, 0, 0, "") + strings.Join(machines, "")
	inv, ps := inventory.Parse([]byte(doc))
	if len(ps) > 0 {
		t.Fatalf("test inventory invalid: %q", ps)
	}
	vt, vps := viewers.Parse([]byte(vdoc))
	if len(vps) > 0 {
		t.Fatalf("test viewers invalid: %q", vps)
	}
	f := newFake()
	l := &fakeLauncher{f: f}
	rec := filepath.Join(t.TempDir(), "record.json")
	set := testSettings()
	h := New(inv, vt, f, l.launch, set, rec)
	h.driftwmUp = true
	ctx, stop := context.WithCancel(context.Background())
	t.Cleanup(stop)
	return &rig{h: h, f: f, l: l, rec: rec, set: set, inv: inv, vt: vt, ctx: ctx, stop: stop}
}

func (r *rig) setStatus(id string, st status) {
	r.h.mu.Lock()
	r.h.byID[id].status = st
	r.h.byID[id].checkedAt = time.Now()
	if r.h.rounds == 0 {
		r.h.rounds = 1 // a test that sets a status pretends a round has finished
	}
	r.h.lastResult = time.Now()
	r.h.mu.Unlock()
}

func TestOpenUpMachinePlacesWindowAtHome(t *testing.T) {
	r := newRig(t, machineDoc("ai-1", "AI Box", "ai", "moonlight", 21002, 2000, -1500, ""))
	r.setStatus("ai-1", statusUp)
	res := r.h.Open("ai-1")
	if res.Action != "open" || !strings.Contains(res.Message, "at home (2000, -1500)") || !strings.Contains(res.Message, "matched by name") {
		t.Fatalf("got %+v", res)
	}
	if len(r.f.moves) != 1 || r.f.moves[0][1] != 2000 || r.f.moves[0][2] != -1500 {
		t.Errorf("moves: %v", r.f.moves)
	}
	if got := r.l.calls[0]; strings.Join(got, " ") != "foot --app-id=hubos-ai-1 --title=AI Box -- sleep infinity" {
		t.Errorf("command: %q", got)
	}
	if _, ok := r.h.WindowOf("ai-1"); !ok {
		t.Error("window not recorded")
	}
	if _, err := os.Stat(r.rec); err != nil {
		t.Errorf("record file: %v", err)
	}
}

func TestOpenAgainGoesToExistingWindow(t *testing.T) {
	r := newRig(t, machineDoc("ai-1", "AI Box", "ai", "moonlight", 21002, 10, 10, ""))
	r.setStatus("ai-1", statusUp)
	r.h.Open("ai-1")
	res := r.h.Open("ai-1")
	if res.Action != "went" || r.l.count() != 1 || len(r.f.windows) != 1 {
		t.Errorf("got %+v launches=%d windows=%d", res, r.l.count(), len(r.f.windows))
	}
	// Still allowed when the machine has since gone down.
	r.setStatus("ai-1", statusDown)
	if res := r.h.Open("ai-1"); res.Action != "went" {
		t.Errorf("down machine with an open window: %+v", res)
	}
}

func TestDoubleClickWhileStartingDoesNothing(t *testing.T) {
	r := newRig(t, machineDoc("ai-1", "AI Box", "ai", "moonlight", 21002, 10, 10, ""))
	r.setStatus("ai-1", statusUp)
	r.l.gate = make(chan struct{})
	first := make(chan OpenResult)
	go func() { first <- r.h.Open("ai-1") }()
	for i := 0; r.l.count() == 0 && i < 200; i++ {
		time.Sleep(10 * time.Millisecond)
	}
	second := r.h.Open("ai-1")
	if second.Action != "busy" || !strings.Contains(second.Message, "already opening") {
		t.Errorf("second open: %+v", second)
	}
	close(r.l.gate)
	if res := <-first; res.Action != "open" {
		t.Errorf("first open: %+v", res)
	}
	if r.l.count() != 1 || len(r.f.windows) != 1 {
		t.Errorf("launches=%d windows=%d", r.l.count(), len(r.f.windows))
	}
}

func TestDownMachineRefusedAndMessageShown(t *testing.T) {
	r := newRig(t, machineDoc("ai-1", "AI Box", "ai", "moonlight", 21002, 10, 10, ""))
	r.setStatus("ai-1", statusDown)
	res := r.h.Open("ai-1")
	if res.Action != "refused" || res.Message != "AI Box is down, not opened" {
		t.Errorf("got %+v", res)
	}
	if r.l.count() != 0 {
		t.Error("a viewer was started for a down machine")
	}
	st := r.h.Status()
	if st.Class != "alert" || !strings.HasPrefix(st.Tooltip, "AI Box is down, not opened") {
		t.Errorf("status: %+v", st)
	}
	if l := r.h.List(false, ""); l[0] != "! AI Box is down, not opened" {
		t.Errorf("first menu line: %q", l[0])
	}
}

func TestNotCheckedAndNothingToOpen(t *testing.T) {
	r := newRig(t,
		machineDoc("d-1", "Desk", "desktop", "moonlight", 0, 1, 1, ""), // no port: viewer needs none
		machineDoc("v-1", "VNC box", "desktop", "vnc", 0, 2, 2, ""),    // viewer needs {port}
		machineDoc("n-1", "Plain", "nas", "files", 5, 3, 3, ""),
	)
	// d-1: no port, command needs none -> allowed.
	if res := r.h.Open("d-1"); res.Action != "open" {
		t.Errorf("d-1: %+v", res)
	}
	res := r.h.Open("v-1")
	if res.Action != "refused" || !strings.Contains(res.Message, "has no port") {
		t.Errorf("v-1: %+v", res)
	}
	if res := r.h.Open("hub"); res.Action != "refused" || !strings.Contains(res.Message, "nothing to open") {
		t.Errorf("hub: %+v", res)
	}
	// Not yet checked (first round pending) is refused, not guessed.
	if res := r.h.Open("n-1"); res.Action != "refused" || !strings.Contains(res.Message, "not been checked yet") {
		t.Errorf("n-1: %+v", res)
	}
}

func TestEndClosesOnlyTheLocalWindow(t *testing.T) {
	r := newRig(t,
		machineDoc("a", "A", "ai", "moonlight", 1, 1, 1, ""),
		machineDoc("b", "B", "ai", "moonlight", 1, 2, 2, ""))
	r.setStatus("a", statusUp)
	r.setStatus("b", statusUp)
	r.h.Open("a")
	r.h.Open("b")
	wa, _ := r.h.WindowOf("a")
	res := r.h.End("a")
	if res.Action != "end" || !strings.Contains(res.Message, "machine and its session were not touched") {
		t.Errorf("got %+v", res)
	}
	if len(r.f.closed) != 1 || r.f.closed[0] != wa || len(r.f.windows) != 1 {
		t.Errorf("closed=%v windows=%v", r.f.closed, r.f.windows)
	}
	if _, ok := r.h.WindowOf("b"); !ok {
		t.Error("b's window was forgotten")
	}
	if res := r.h.End("a"); res.Action != "refused" {
		t.Errorf("end of a closed one: %+v", res)
	}
	// A window that ignores the close request is left alone and reported.
	wb, _ := r.h.WindowOf("b")
	r.f.stuck[wb] = true
	if res := r.h.End("b"); res.Action != "failed" || !strings.Contains(res.Message, "still open") {
		t.Errorf("stuck: %+v", res)
	}
	if _, ok := r.h.WindowOf("b"); !ok {
		t.Error("record dropped although the window is still there")
	}
}

func TestRestartAdoptsAndOpensNothing(t *testing.T) {
	r := newRig(t,
		machineDoc("a", "A", "ai", "moonlight", 1, 1, 1, ""),
		machineDoc("f", "F", "nas", "files", 1, 2, 2, "")) // comparison viewer
	r.setStatus("a", statusUp)
	r.setStatus("f", statusUp)
	r.h.Open("a")
	r.h.Open("f")
	wf, _ := r.h.WindowOf("f")
	launches := r.l.count()

	// hubd restarts: a new Hub over the same driftwm and the same record file.
	l2 := &fakeLauncher{f: r.f}
	h2 := New(r.inv, r.vt, r.f, l2.launch, r.set, r.rec)
	if err := h2.Adopt(); err != nil {
		t.Fatal(err)
	}
	if _, ok := h2.WindowOf("a"); !ok {
		t.Error("named window not adopted")
	}
	if got, ok := h2.WindowOf("f"); !ok || got != wf {
		t.Error("comparison window not adopted from the record file")
	}
	if l2.count() != 0 || r.l.count() != launches {
		t.Error("adopting started a viewer")
	}

	// A record from another driftwm instance is not trusted.
	r.f.identity = "i2"
	h3 := New(r.inv, r.vt, r.f, l2.launch, r.set, r.rec)
	h3.Adopt()
	if _, ok := h3.WindowOf("f"); ok {
		t.Error("record of an older driftwm was trusted")
	}
	if _, ok := h3.WindowOf("a"); !ok {
		t.Error("a window named hubos-a should still be adopted by name")
	}
}

func TestDriftwmRestartClearsRecord(t *testing.T) {
	r := newRig(t, machineDoc("a", "A", "ai", "moonlight", 1, 1, 1, ""))
	r.setStatus("a", statusUp)
	r.h.Open("a")
	go r.h.RunWatch(r.ctx)
	for i := 0; i < 200; i++ {
		r.f.mu.Lock()
		n := len(r.f.subs)
		r.f.mu.Unlock()
		if n > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	r.f.mu.Lock()
	r.f.windows, r.f.identity = nil, "i2" // a new driftwm has no windows
	r.f.mu.Unlock()
	r.f.endStreams() // driftwm went away
	for i := 0; i < 200; i++ {
		if _, ok := r.h.WindowOf("a"); !ok {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if _, ok := r.h.WindowOf("a"); ok {
		t.Error("record not cleared when driftwm went away")
	}
	b, _ := os.ReadFile(r.rec)
	if strings.Contains(string(b), `"machine"`) {
		t.Errorf("record file still lists windows: %s", b)
	}
}

func TestWindowClosedByTheUserIsForgotten(t *testing.T) {
	r := newRig(t, machineDoc("a", "A", "ai", "moonlight", 1, 1, 1, ""))
	r.setStatus("a", statusUp)
	r.h.Open("a")
	w, _ := r.h.WindowOf("a")
	r.f.remove(w)
	st, _ := r.f.State()
	r.h.syncWindows(st)
	if _, ok := r.h.WindowOf("a"); ok {
		t.Error("closed window still recorded")
	}
	// Opening now starts a new viewer rather than "going to" a ghost.
	if res := r.h.Open("a"); res.Action != "open" || r.l.count() != 2 {
		t.Errorf("%+v launches=%d", res, r.l.count())
	}
}

func TestAmbiguousMatchDoesNothing(t *testing.T) {
	r := newRig(t, machineDoc("f", "F", "nas", "files", 1, 5, 5, ""))
	r.setStatus("f", statusUp)
	r.l.script = func([]string) { r.f.add("x", "one"); r.f.add("y", "two") }
	res := r.h.Open("f")
	if res.Action != "failed" || !strings.Contains(res.Message, "cannot tell which one") {
		t.Errorf("got %+v", res)
	}
	if len(r.f.moves) != 0 || len(r.f.focused) != 0 || len(r.f.closed) != 0 {
		t.Errorf("windows were touched: moves=%v focused=%v closed=%v", r.f.moves, r.f.focused, r.f.closed)
	}
	if _, ok := r.h.WindowOf("f"); ok {
		t.Error("recorded although ambiguous")
	}
	// Clicking again must not start a third window.
	if res := r.h.Open("f"); res.Action != "refused" || r.l.count() != 1 {
		t.Errorf("second try: %+v launches=%d", res, r.l.count())
	}
	if res := r.h.End("f"); res.Action != "refused" || !strings.Contains(res.Message, "hubd forget f") || len(r.f.closed) != 0 {
		t.Errorf("end must not close an unidentified window: %+v closed=%v", res, r.f.closed)
	}
	// The menu offers the way out, and so does the command.
	menu := strings.Join(r.h.List(false, ""), "\n")
	if !strings.Contains(menu, "x forget unknown window for F (f)") {
		t.Errorf("no forget line:\n%s", menu)
	}
	pr := r.h.Pick("x forget unknown window for F (f)")
	if pr.Action != "forgot" || !pr.Reopen || !strings.Contains(pr.Message, "no window was closed or moved") {
		t.Errorf("pick forget: %+v", pr)
	}
	if len(r.f.moves) != 0 || len(r.f.focused) != 0 || len(r.f.closed) != 0 || len(r.f.windows) != 2 {
		t.Errorf("forgetting touched windows: moves=%v focused=%v closed=%v windows=%d", r.f.moves, r.f.focused, r.f.closed, len(r.f.windows))
	}
	if strings.Contains(strings.Join(r.h.List(false, ""), "\n"), "forget unknown") {
		t.Error("forget line still offered")
	}
	if res := r.h.Forget("f"); res.Action != "refused" {
		t.Errorf("forgetting twice: %+v", res)
	}
	// With the block gone a new open starts a new viewer (and may be ambiguous again).
	if res := r.h.Open("f"); r.l.count() != 2 {
		t.Errorf("open after forget: %+v launches=%d", res, r.l.count())
	}
}

func TestViewerThatIgnoresTheNameUsesComparison(t *testing.T) {
	r := newRig(t, machineDoc("f", "F", "nas", "files", 1, 5, 5, ""))
	r.setStatus("f", statusUp)
	r.f.add("other", "someone elses") // already there before
	res := r.h.Open("f")
	if res.Action != "open" || !strings.Contains(res.Message, "matched by comparison") {
		t.Errorf("got %+v", res)
	}
	if w, _ := r.h.WindowOf("f"); w != 1 {
		t.Errorf("matched window %d, want the new one (1)", w)
	}
}

func TestNamedViewerThatIgnoresItsNameFailsWithAdvice(t *testing.T) {
	r := newRig(t, machineDoc("a", "A", "ai", "moonlight", 1, 1, 1, ""))
	r.setStatus("a", statusUp)
	r.h.set.WindowWait = 400 * time.Millisecond
	r.l.script = func([]string) { r.f.add("not-our-name", "t") }
	res := r.h.Open("a")
	if (res.Action != "late" && res.Action != "failed") || !strings.Contains(res.Message, "sets_name = false") {
		t.Errorf("got %+v", res)
	}
}

func TestNoWindowAppears(t *testing.T) {
	r := newRig(t, machineDoc("a", "A", "ai", "moonlight", 1, 1, 1, ""))
	r.setStatus("a", statusUp)
	r.h.set.WindowWait = 300 * time.Millisecond
	r.h.set.LateGrace = 0 // no grace period: the old behaviour, a plain failure
	r.l.script = func([]string) {}
	res := r.h.Open("a")
	if res.Action != "failed" || !strings.Contains(res.Message, "no window appeared") {
		t.Errorf("got %+v", res)
	}
	if res := r.h.Open("a"); res.Action != "failed" { // may try again
		t.Errorf("retry: %+v", res)
	}
}

func TestWindowTooTallIsShrunkClearOfTheBar(t *testing.T) {
	r := newRig(t, machineDoc("a", "A", "ai", "moonlight", 1, 1, 1, ""))
	r.setStatus("a", statusUp)
	r.h.set.BarHeight = 30
	r.l.script = func([]string) {
		id := r.f.add("hubos-a", "A")
		r.f.mu.Lock()
		r.f.windows[len(r.f.windows)-1].Size = [2]int{1280, 800}
		r.f.mu.Unlock()
		_ = id
	}
	res := r.h.Open("a")
	if !strings.Contains(res.Message, "shrunk from 1280x800 to 1280x770") {
		t.Errorf("got %+v", res)
	}
}

func TestInventoryTextStaysOneArgument(t *testing.T) {
	r := newRig(t, machineDoc("a", "A", "ai", "moonlight", 1, 1, 1, ""))
	r.h.byID["a"].m.Name = `x"; touch /tmp/pwned; echo "`
	r.setStatus("a", statusUp)
	r.h.Open("a")
	got := r.l.calls[0]
	if len(got) != 6 || got[2] != `--title=x"; touch /tmp/pwned; echo "` {
		t.Errorf("args: %q", got)
	}
}

// Viewers that are matched by comparing the window list before and after
// ("unnamed": sets_name false, no title_match) start one at a time.
func TestSecondLaunchWaitsForTheFirst(t *testing.T) {
	r := newRig(t,
		machineDoc("a", "A", "ai", "files", 1, 1, 1, ""),
		machineDoc("b", "B", "ai", "files", 1, 2, 2, ""))
	r.setStatus("a", statusUp)
	r.setStatus("b", statusUp)
	r.l.gate = make(chan struct{})
	done := make(chan OpenResult, 2)
	go func() { done <- r.h.Open("a") }()
	go func() { done <- r.h.Open("b") }()
	time.Sleep(150 * time.Millisecond)
	if r.l.count() != 1 {
		t.Errorf("two viewers started at once: %d", r.l.count())
	}
	close(r.l.gate)
	for i := 0; i < 2; i++ {
		if res := <-done; res.Action != "open" {
			t.Errorf("%+v", res)
		}
	}
}

// Viewers that set their own window name (every machine has its own name) are
// told apart by that name, so they start at the same time: this is what lets
// a restore after a compositor restart start all the viewers at once.
func TestNamedViewersStartAtTheSameTime(t *testing.T) {
	r := newRig(t,
		machineDoc("a", "A", "ai", "moonlight", 1, 1, 1, ""),
		machineDoc("b", "B", "ai", "moonlight", 1, 2, 2, ""))
	r.setStatus("a", statusUp)
	r.setStatus("b", statusUp)
	r.l.gate = make(chan struct{})
	done := make(chan OpenResult, 2)
	go func() { done <- r.h.Open("a") }()
	go func() { done <- r.h.Open("b") }()
	for i := 0; r.l.count() < 2 && i < 100; i++ {
		time.Sleep(10 * time.Millisecond)
	}
	if r.l.count() != 2 {
		t.Errorf("the two named viewers did not start together: %d", r.l.count())
	}
	close(r.l.gate)
	for i := 0; i < 2; i++ {
		if res := <-done; res.Action != "open" {
			t.Errorf("%+v", res)
		}
	}
	for id, pos := range map[string][2]int{"a": {1, 1}, "b": {2, 2}} {
		w, _ := r.h.WindowOf(id)
		cur, _ := mustState(t, r.f).Window(w)
		if cur.Position != pos {
			t.Errorf("%s stands at %v, not at its home %v", id, cur.Position, pos)
		}
	}
}

func mustState(t *testing.T, f *fakeComp) *driftwm.State {
	t.Helper()
	st, err := f.State()
	if err != nil {
		t.Fatal(err)
	}
	return st
}

// (the orphan-viewer tests are in late_test.go)

func recorded(h *Hub, id string) any {
	if w, ok := h.WindowOf(id); ok {
		return w
	}
	return "none"
}

func names(f *fakeComp) []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []string
	for _, w := range f.windows {
		out = append(out, fmt.Sprintf("#%d %s", w.ID, w.AppID))
	}
	return out
}
