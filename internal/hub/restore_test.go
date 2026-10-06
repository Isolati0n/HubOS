package hub

import (
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

// watchRig is a hub following the fake compositor, with machines a, b, c up.
func watchRig(t *testing.T) *rig {
	t.Helper()
	r := newRig(t,
		machineDoc("a", "A Box", "ai", "moonlight", 1, 100, 100, ""),
		machineDoc("b", "B Box", "ai", "moonlight", 1, 300, 300, ""),
		machineDoc("c", "C Desk", "desktop", "moonlight", 1, 3000, 0, ""))
	for _, id := range []string{"a", "b", "c"} {
		r.setStatus(id, statusUp)
	}
	r.h.driftwmUp = false
	go r.h.RunWatch(r.ctx)
	waitFor(t, "hubd to follow the compositor", 2*time.Second, func() bool { return r.h.Status().Tooltip != "" && r.h.driftwmUpNow() })
	return r
}

func (h *Hub) driftwmUpNow() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.driftwmUp
}

// moveAndTell moves a window and lets hubd see the change, as driftwm's event stream does.
func (r *rig) moveAndTell(t *testing.T, id string, x, y, w, h int) {
	t.Helper()
	r.place(t, id, x, y, w, h)
	r.f.emit()
	want := Place{x, y, w, h}
	waitFor(t, "hubd to see the move", 2*time.Second, func() bool {
		r.h.mu.Lock()
		defer r.h.mu.Unlock()
		return r.h.places[id] == want
	})
}

func (r *rig) restored(t *testing.T) {
	t.Helper()
	waitFor(t, "the windows to come back", 5*time.Second, func() bool { return !r.h.RestartPending() && r.h.driftwmUpNow() })
}

func TestDirectRestoreBringsEveryOpenWindowBackToItsPlaceAfterACompositorRestart(t *testing.T) {
	r := watchRig(t)
	for _, id := range []string{"a", "b", "c"} {
		if res := r.h.Open(id); res.Action != "open" {
			t.Fatal(res.Message)
		}
	}
	r.moveAndTell(t, "a", -500, 40, 800, 600)
	r.moveAndTell(t, "b", 500, 40, 640, 480)
	r.moveAndTell(t, "c", 77, -900, 500, 400)
	r.h.End("b") // closed on purpose: must stay closed
	r.f.camera, r.f.zoom = [2]float64{12, -7}, 0.75
	r.f.emit()
	waitFor(t, "hubd to see the view", 2*time.Second, func() bool {
		r.h.mu.Lock()
		defer r.h.mu.Unlock()
		return r.h.lastView.zoom == 0.75
	})
	launches := r.l.count()

	r.f.crash()
	waitFor(t, "hubd to notice the crash", 2*time.Second, func() bool { return !r.h.driftwmUpNow() })
	if _, ok := r.h.WindowOf("a"); ok {
		t.Error("hubd still holds a window of the dead compositor")
	}
	if got := r.h.List(false, ""); strings.Contains(strings.Join(got, "\n"), MarkerOpen) {
		t.Errorf("a dot is filled while no window exists:\n%s", strings.Join(got, "\n"))
	}
	r.f.restart()
	r.restored(t)

	if n := r.l.count() - launches; n != 2 {
		t.Errorf("%d viewers started by the restore, want 2 (a and c; b was closed on purpose)", n)
	}
	if x, y, w, h := r.winOf(t, "a"); [4]int{x, y, w, h} != [4]int{-500, 40, 800, 600} {
		t.Errorf("a: %d %d %d %d", x, y, w, h)
	}
	if x, y, w, h := r.winOf(t, "c"); [4]int{x, y, w, h} != [4]int{77, -900, 500, 400} {
		t.Errorf("c: %d %d %d %d", x, y, w, h)
	}
	if _, ok := r.h.WindowOf("b"); ok {
		t.Error("b came back although it was closed on purpose")
	}
	if r.f.camera != [2]float64{12, -7} || r.f.zoom != 0.75 {
		t.Errorf("view %v %v", r.f.camera, r.f.zoom)
	}
	st := r.h.Status()
	if !strings.Contains(st.Tooltip, "2 of 2 windows are back at their places") {
		t.Errorf("tooltip: %q", st.Tooltip)
	}
	// and the dots are filled again
	if got := strings.Join(r.h.List(false, ""), "\n"); strings.Count(got, MarkerOpen) != 2 {
		t.Errorf("list:\n%s", got)
	}
}

// The place hubd restores is the one it saw last, also when the owner moved the
// window an instant before the crash (driftwm's own session file would be a
// second or more behind).
func TestRestoreUsesTheLastMoveSeenRightBeforeTheCrash(t *testing.T) {
	r := watchRig(t)
	r.h.Open("a")
	r.moveAndTell(t, "a", 10, 10, 700, 525)
	r.moveAndTell(t, "a", 11, 12, 701, 526) // the last move, a moment before the kill
	r.f.crash()
	waitFor(t, "the crash to be noticed", 2*time.Second, func() bool { return !r.h.driftwmUpNow() })
	r.f.restart()
	r.restored(t)
	if x, y, w, h := r.winOf(t, "a"); [4]int{x, y, w, h} != [4]int{11, 12, 701, 526} {
		t.Errorf("a: %d %d %d %d", x, y, w, h)
	}
}

// The signal hubd waits for is the new compositor's Wayland socket answering,
// as well as its control socket.
func TestRestoreWaitsForTheWaylandSocket(t *testing.T) {
	dir, err := os.MkdirTemp("", "w")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	sock := filepath.Join(dir, "wayland-1")
	r := newRig(t, machineDoc("a", "A Box", "ai", "moonlight", 1, 100, 100, ""))
	r.setStatus("a", statusUp)
	r.h.set.WaylandSocket = sock
	go r.h.RunWatch(r.ctx)
	waitFor(t, "follow", 2*time.Second, func() bool { return r.h.driftwmUpNow() })
	r.h.Open("a")
	r.moveAndTell(t, "a", 10, 10, 700, 525)
	r.f.crash()
	waitFor(t, "crash", 2*time.Second, func() bool { return !r.h.driftwmUpNow() })
	launches := r.l.count()
	r.f.restart() // the control socket answers, the Wayland socket does not exist yet
	time.Sleep(400 * time.Millisecond)
	if r.l.count() != launches {
		t.Fatal("a viewer was started before the Wayland socket answered")
	}
	l, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}
			c.Close()
		}
	}()
	r.restored(t)
	if r.l.count() != launches+1 {
		t.Errorf("launches %d -> %d", launches, r.l.count())
	}
	if x, y, _, _ := r.winOf(t, "a"); x != 10 || y != 10 {
		t.Errorf("a at %d %d", x, y)
	}
}

func TestRestoreSaysWhichMachineWasNotRestored(t *testing.T) {
	r := watchRig(t)
	r.h.Open("a")
	r.h.Open("b")
	r.moveAndTell(t, "a", 10, 10, 700, 525)
	r.moveAndTell(t, "b", 20, 20, 700, 525)
	r.f.crash()
	waitFor(t, "crash", 2*time.Second, func() bool { return !r.h.driftwmUpNow() })
	r.setStatus("b", statusDown) // b went down in the meantime: a viewer could not connect
	r.f.restart()
	r.restored(t)
	if _, ok := r.h.WindowOf("a"); !ok {
		t.Error("a not restored")
	}
	if _, ok := r.h.WindowOf("b"); ok {
		t.Error("b restored although it is down")
	}
	if tip := r.h.Status().Tooltip; !strings.Contains(tip, "1 of 2 windows are back") || !strings.Contains(tip, "not restored: b (B Box is down, not opened)") {
		t.Errorf("tooltip: %q", tip)
	}
}

// After a reboot nothing comes back: the list lives only in hubd's memory.
func TestNothingIsRestoredWhenHubdStartsAfterTheCrash(t *testing.T) {
	r := newRig(t, machineDoc("a", "A Box", "ai", "moonlight", 1, 100, 100, ""))
	r.setStatus("a", statusUp)
	r.h.Open("a")
	r.f.crash()
	r.f.restart()
	h2 := New(r.inv, r.vt, r.f, r.l.launch, r.set, r.rec)
	h2.setStatus(statusUp, "a")
	go h2.RunWatch(r.ctx)
	waitFor(t, "follow", 2*time.Second, func() bool { return h2.driftwmUpNow() })
	time.Sleep(200 * time.Millisecond)
	if r.l.count() != 1 {
		t.Errorf("a new hubd started viewers: %d launches", r.l.count())
	}
}

func (h *Hub) setStatus(st status, ids ...string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, id := range ids {
		h.byID[id].status = st
	}
	h.rounds = 1
}

// ---- restart-desktop ----

type sigRec struct {
	mu   sync.Mutex
	sigs []syscall.Signal
	pids []int
}

func (s *sigRec) send(pid int, sig syscall.Signal) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sigs = append(s.sigs, sig)
	s.pids = append(s.pids, pid)
	return nil
}

func TestRestartDesktopSendsTermToTheCompositorAndNothingElse(t *testing.T) {
	r := newRig(t, machineDoc("a", "A", "ai", "moonlight", 1, 1, 1, ""))
	r.f.pid = 4242
	rec := &sigRec{}
	alive := true
	var mu sync.Mutex
	r.h.set.Signal = func(pid int, sig syscall.Signal) error {
		rec.send(pid, sig)
		mu.Lock()
		alive = false // it exits when asked politely
		mu.Unlock()
		return nil
	}
	r.h.set.ProcName = func(pid int) string {
		mu.Lock()
		defer mu.Unlock()
		if pid == 4242 && alive {
			return "driftwm"
		}
		return ""
	}
	res := r.h.RestartDesktop()
	if res.Action != "restart" || !strings.Contains(res.Message, "pid 4242") {
		t.Fatalf("%+v", res)
	}
	if len(rec.sigs) != 1 || rec.sigs[0] != syscall.SIGTERM || rec.pids[0] != 4242 {
		t.Errorf("signals %v to %v", rec.sigs, rec.pids)
	}
}

func TestRestartDesktopKillsAfterTheWaitIfTheCompositorDoesNotExit(t *testing.T) {
	r := newRig(t, machineDoc("a", "A", "ai", "moonlight", 1, 1, 1, ""))
	r.f.pid = 4242
	rec := &sigRec{}
	r.h.set.KillWait = 150 * time.Millisecond
	r.h.set.Signal = rec.send
	r.h.set.ProcName = func(int) string { return "driftwm" }
	res := r.h.RestartDesktop()
	if res.Action != "restart" || !strings.Contains(res.Message, "was killed") {
		t.Fatalf("%+v", res)
	}
	if len(rec.sigs) != 2 || rec.sigs[0] != syscall.SIGTERM || rec.sigs[1] != syscall.SIGKILL || rec.pids[1] != 4242 {
		t.Errorf("signals %v to %v", rec.sigs, rec.pids)
	}
}

func TestRestartDesktopRefusesWhenItCannotBeSureItIsTheCompositor(t *testing.T) {
	for _, c := range []struct {
		name string
		pid  int
		proc string
		want string
	}{
		{"nobody listens", 0, "", "cannot find the desktop"},
		{"pid 1", 1, "driftwm", "makes no sense"},
		{"another program owns the socket", 4242, "sway", `is "sway"`},
	} {
		t.Run(c.name, func(t *testing.T) {
			r := newRig(t, machineDoc("a", "A", "ai", "moonlight", 1, 1, 1, ""))
			r.f.pid = c.pid
			rec := &sigRec{}
			r.h.set.Signal = rec.send
			r.h.set.ProcName = func(int) string { return c.proc }
			res := r.h.RestartDesktop()
			if res.Action == "restart" || !strings.Contains(res.Message, c.want) {
				t.Errorf("%+v", res)
			}
			if len(rec.sigs) != 0 {
				t.Errorf("signals sent: %v", rec.sigs)
			}
		})
	}
}

// A compositor that closes its windows one by one before it exits must not make hubd forget them.
func TestRestartDesktopKeepsTheWindowsEvenIfTheyVanishBeforeTheStreamEnds(t *testing.T) {
	r := watchRig(t)
	r.h.Open("a")
	r.moveAndTell(t, "a", 10, 10, 700, 525)
	r.f.pid = 4242
	var gone atomic.Bool
	r.h.set.ProcName = func(int) string {
		if gone.Load() {
			return ""
		}
		return "driftwm"
	}
	r.h.set.Signal = func(pid int, sig syscall.Signal) error {
		defer gone.Store(true)
		r.f.mu.Lock()
		r.f.windows = nil // the windows go first ...
		r.f.mu.Unlock()
		r.f.emit()
		time.Sleep(50 * time.Millisecond)
		r.f.crash() // ... then the compositor ends
		return nil
	}
	if res := r.h.RestartDesktop(); res.Action != "restart" {
		t.Fatal(res.Message)
	}
	waitFor(t, "crash", 2*time.Second, func() bool { return !r.h.driftwmUpNow() })
	r.f.restart()
	r.restored(t)
	if x, y, _, _ := r.winOf(t, "a"); x != 10 || y != 10 {
		t.Errorf("a at %d %d", x, y)
	}
}

// ---- the home-position race ----

// A window that has only just appeared can be put at the compositor's own
// cascade spot by the compositor after hubd's first move (seen once in nine
// runs, docs/hubd-slice2.md section 17.6). hubd now looks again until the window
// stays, and asks again.
func TestWindowIsPutAtItsHomeAgainWhenTheCompositorMovesItAfterTheFirstMove(t *testing.T) {
	r := newRig(t, machineDoc("a", "A", "ai", "moonlight", 1, 100, 100, ""))
	r.setStatus("a", statusUp)
	r.f.cascade[0] = 2 // the first two moves are overridden by the compositor
	res := r.h.Open("a")
	if res.Action != "open" || strings.Contains(res.Message, "WARNING") {
		t.Fatalf("%+v", res)
	}
	if x, y, _, _ := r.winOf(t, "a"); x != 100 || y != 100 {
		t.Errorf("a at %d %d, not at home", x, y)
	}
	if len(r.f.moves) < 3 {
		t.Errorf("moves: %v", r.f.moves)
	}
}

// The real cause of the race: driftwm lists a window as soon as its toplevel
// exists (a frame of only a title bar and border) and places it, at its own
// cascade spot, only when the first picture arrives, overwriting any move made
// before that. hubd waits for the first picture before it moves the window.
func TestWindowIsNotMovedBeforeItHasDrawnItsFirstPicture(t *testing.T) {
	r := newRig(t, machineDoc("a", "A", "ai", "moonlight", 1, 100, 100, ""))
	r.setStatus("a", statusUp)
	r.h.set.SizedWait = 3 * time.Second
	r.l.script = func(args []string) {
		id := r.f.add("hubos-a", "A")
		r.f.mu.Lock()
		for i := range r.f.windows {
			if r.f.windows[i].ID == id {
				r.f.windows[i].Size = [2]int{16, 41} // listed, nothing drawn yet
			}
		}
		r.f.mu.Unlock()
		go func() { // the first picture arrives late (a slow machine): the compositor places the window at its own spot
			time.Sleep(1200 * time.Millisecond)
			r.f.mu.Lock()
			for i := range r.f.windows {
				if r.f.windows[i].ID == id {
					r.f.windows[i].Size = [2]int{700, 525}
					r.f.windows[i].Position = [2]int{25, -125}
				}
			}
			r.f.mu.Unlock()
		}()
	}
	res := r.h.Open("a")
	if res.Action != "open" || strings.Contains(res.Message, "WARNING") {
		t.Fatalf("%+v", res)
	}
	if x, y, _, _ := r.winOf(t, "a"); x != 100 || y != 100 {
		t.Errorf("a at %d %d, not at home", x, y)
	}
	if r.f.earlyMoves != 0 {
		t.Errorf("%d moves were made before the window had drawn", r.f.earlyMoves)
	}
}

func TestWindowThatNeverDrawsIsMovedAnywayWithAWarning(t *testing.T) {
	r := newRig(t, machineDoc("a", "A", "ai", "moonlight", 1, 100, 100, ""))
	r.setStatus("a", statusUp)
	r.l.script = func(args []string) {
		id := r.f.add("hubos-a", "A")
		r.f.mu.Lock()
		r.f.windows[len(r.f.windows)-1].Size = [2]int{16, 41}
		r.f.mu.Unlock()
		_ = id
	}
	res := r.h.Open("a")
	if !strings.Contains(res.Message, "WARNING: the window had drawn nothing after") {
		t.Errorf("%+v", res)
	}
}

func TestWindowThatNeverStaysGetsAWarningNotASilentWrongPlace(t *testing.T) {
	r := newRig(t, machineDoc("a", "A", "ai", "moonlight", 1, 100, 100, ""))
	r.setStatus("a", statusUp)
	r.f.cascade[0] = 1 << 20
	res := r.h.Open("a")
	if !strings.Contains(res.Message, "WARNING: it did not stay at (100, 100)") {
		t.Errorf("%+v", res)
	}
}

// A viewer that is slow to show its window (it goes to the late-window state)
// still gets its window put at the saved place when the window comes.
func TestRestoreOfASlowViewerPlacesItsLateWindowAtTheSavedPlace(t *testing.T) {
	r := newRig(t, machineDoc("a", "A Box", "ai", "moonlight", 1, 100, 100, ""))
	r.setStatus("a", statusUp)
	r.h.set.WindowWait = 100 * time.Millisecond
	r.h.set.LateGrace = 5 * time.Second
	go r.h.RunWatch(r.ctx)
	waitFor(t, "follow", 2*time.Second, func() bool { return r.h.driftwmUpNow() })
	r.h.Open("a")
	r.moveAndTell(t, "a", 10, 10, 640, 480)
	r.f.crash()
	waitFor(t, "crash", 2*time.Second, func() bool { return !r.h.driftwmUpNow() })
	r.l.script = func(args []string) { // the window comes 600 ms after the viewer started
		go func() {
			time.Sleep(600 * time.Millisecond)
			r.f.add("hubos-a", "A Box")
		}()
	}
	r.f.restart()
	r.restored(t)
	if x, y, w, h := r.winOf(t, "a"); [4]int{x, y, w, h} != [4]int{10, 10, 640, 480} {
		t.Errorf("a: %d %d %d %d", x, y, w, h)
	}
	if tip := r.h.Status().Tooltip; !strings.Contains(tip, "1 of 1 windows are back") {
		t.Errorf("tooltip: %q", tip)
	}
}

// An open that was under way when the compositor went away must not record or move anything afterwards: its
// window is gone, and in the new compositor the same window number can belong to another machine's window.
func TestAnOpenUnderWayWhenTheCompositorRestartedRecordsAndMovesNothing(t *testing.T) {
	r := watchRig(t)
	r.l.gate = make(chan struct{})
	done := make(chan OpenResult, 1)
	go func() { done <- r.h.Open("a") }()
	waitFor(t, "the viewer to be started", 2*time.Second, func() bool { return r.l.count() == 1 })
	r.f.crash()
	waitFor(t, "crash", 2*time.Second, func() bool { return !r.h.driftwmUpNow() })
	r.f.restart()
	waitFor(t, "follow", 2*time.Second, func() bool { return r.h.driftwmUpNow() })
	close(r.l.gate) // the viewer of the old compositor now "shows" a window in the new one
	res := <-done
	if res.Action != "failed" || !strings.Contains(res.Message, "restarted while") {
		t.Errorf("%+v", res)
	}
	if _, ok := r.h.WindowOf("a"); ok {
		t.Error("a window of the dead compositor was recorded")
	}
	r.f.mu.Lock()
	moves := len(r.f.moves)
	r.f.mu.Unlock()
	if moves != 0 {
		t.Errorf("%d moves were made for an open that belonged to the old compositor", moves)
	}
}

func TestWindowThatClosedBeforeItWasPlacedIsNotRecorded(t *testing.T) {
	r := newRig(t, machineDoc("a", "A", "ai", "moonlight", 1, 100, 100, ""))
	r.setStatus("a", statusUp)
	r.h.set.SizedWait = 3 * time.Second
	r.l.script = func(args []string) {
		id := r.f.add("hubos-a", "A")
		r.f.mu.Lock()
		r.f.windows[len(r.f.windows)-1].Size = [2]int{16, 41}
		r.f.mu.Unlock()
		go func() { time.Sleep(700 * time.Millisecond); r.f.remove(id) }()
	}
	res := r.h.Open("a")
	if res.Action != "failed" || !strings.Contains(res.Message, "closed") {
		t.Errorf("%+v", res)
	}
	if _, ok := r.h.WindowOf("a"); ok {
		t.Error("the closed window was recorded")
	}
}

// While windows are being brought back every new window pans the view, so that view is not the owner's. A second
// crash during a restore must bring back the view from before the first one.
func TestSecondCrashDuringARestoreStillRestoresTheViewFromBeforeTheFirst(t *testing.T) {
	r := watchRig(t)
	r.h.Open("a")
	r.moveAndTell(t, "a", 10, 10, 640, 480)
	r.f.camera, r.f.zoom = [2]float64{12, -7}, 0.75
	r.f.emit()
	waitFor(t, "hubd to see the view", 2*time.Second, func() bool {
		r.h.mu.Lock()
		defer r.h.mu.Unlock()
		return r.h.lastView.zoom == 0.75
	})
	var n int
	var nmu sync.Mutex
	r.l.gate = make(chan struct{})
	r.l.script = func(args []string) {
		nmu.Lock()
		n++
		first := n == 1
		nmu.Unlock()
		if !first { // the first viewer belongs to the first new compositor, which dies
			r.f.add("hubos-a", "A Box")
		}
	}
	r.f.crash()
	waitFor(t, "crash", 2*time.Second, func() bool { return !r.h.driftwmUpNow() })
	r.f.restart() // the restore starts and its viewer waits at the gate
	waitFor(t, "the restore to start a viewer", 2*time.Second, func() bool { return r.l.count() == 2 })
	r.f.mu.Lock()
	r.f.camera, r.f.zoom = [2]float64{777, 777}, 0.5 // what a new window does to the view
	r.f.mu.Unlock()
	r.f.emit()
	time.Sleep(50 * time.Millisecond)
	r.f.crash() // the second crash, in the middle of the restore
	waitFor(t, "second crash", 2*time.Second, func() bool { return !r.h.driftwmUpNow() })
	r.f.restart()
	close(r.l.gate)
	r.restored(t)
	if x, y, w, h := r.winOf(t, "a"); [4]int{x, y, w, h} != [4]int{10, 10, 640, 480} {
		t.Errorf("a: %d %d %d %d", x, y, w, h)
	}
	if r.f.camera != [2]float64{12, -7} || r.f.zoom != 0.75 {
		t.Errorf("view %v %v, want the one from before the first crash", r.f.camera, r.f.zoom)
	}
}

// driftwm can be slow to answer when 20 windows start at once. A timeout is not "the window is gone" and not the
// end of the placing: hubd asks again (seen in the hub image test: a window kept the viewer's first size because
// the first move timed out).
func TestSlowAnswersFromDriftwmAreAskedAgainWhilePlacing(t *testing.T) {
	r := newRig(t, machineDoc("a", "A", "ai", "moonlight", 1, 100, 100, ""))
	r.setStatus("a", statusUp)
	r.h.set.SettleMax = 2 * time.Second
	r.h.set.SizedWait = 2 * time.Second
	r.l.script = func(args []string) {
		r.f.add("hubos-a", "A")
		r.f.mu.Lock()
		r.f.stateErrors, r.f.moveErrors = 3, 2 // the next answers time out
		r.f.mu.Unlock()
	}
	res := r.h.Open("a")
	if res.Action != "open" || res.Problem != "" || strings.Contains(res.Message, "WARNING") || strings.Contains(res.Message, "could not") {
		t.Fatalf("%+v", res)
	}
	if x, y, _, _ := r.winOf(t, "a"); x != 100 || y != 100 {
		t.Errorf("a at %d %d, not at home", x, y)
	}
}

// A window that is open but could not be put at its place is not counted as restored, and the bar says so.
func TestRestoreDoesNotCountAWindowThatCouldNotBePutAtItsPlace(t *testing.T) {
	r := watchRig(t)
	r.h.Open("a")
	r.moveAndTell(t, "a", 10, 10, 640, 480)
	r.f.crash()
	waitFor(t, "crash", 2*time.Second, func() bool { return !r.h.driftwmUpNow() })
	r.f.restart()
	r.f.mu.Lock()
	r.f.moveErrors = 1 << 20 // every move times out
	r.f.mu.Unlock()
	r.restored(t)
	tip := r.h.Status().Tooltip
	if !strings.Contains(tip, "0 of 1 windows are back") || !strings.Contains(tip, "not restored: a (open, but not at its place:") {
		t.Errorf("tooltip: %q", tip)
	}
}

// ---- hubd end ----

func TestEndWaitsForASlowWindowUpToThirtySecondsByDefault(t *testing.T) {
	if got := DefaultSettings().CloseWait; got != 30*time.Second {
		t.Errorf("default close wait %s", got)
	}
	r := newRig(t, machineDoc("a", "A", "ai", "moonlight", 1, 1, 1, ""))
	r.setStatus("a", statusUp)
	r.h.Open("a")
	wid, _ := r.h.WindowOf("a")
	r.f.stuck[wid] = true
	r.h.set.CloseWait = 3 * time.Second
	go func() { time.Sleep(700 * time.Millisecond); r.f.remove(wid) }() // the program takes a while to close
	if res := r.h.End("a"); res.Action != "end" {
		t.Errorf("%+v", res)
	}
}
