package hub

import (
	"context"
	"fmt"
	"math"
	"net"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"hubos/internal/driftwm"
)

// Direct restore (docs/proposals/hub-stability.md section 9.6, run A6).
//
// hubd sees every change of every window (driftwm's event stream), so it
// always knows where each machine's window is. When the stream ends while
// hubd keeps running, the compositor has gone (a crash, or restart-desktop).
// hubd then:
//
//  1. keeps the list of machines that had a window, with the place of each
//     (centre and size) and the view (zoom and camera);
//  2. waits for the SIGNAL that the new compositor is ready: driftwm's control
//     socket answers a "state" request (that is how hubd gets its event stream
//     again) AND its Wayland socket accepts a connection (the viewers need it);
//  3. starts the viewer of every machine on that list, all at once, and when
//     each window appears puts it at its saved place and size;
//  4. puts the stacking order back (which window is on top of which), then the view.
//
// Nothing is restored after a reboot: the list lives only in hubd's memory.
// The sessions on the machines are not touched, so the viewers reconnect to
// sessions that are still there.

// rememberLocked notes the latest place of every machine window and the
// view from a snapshot of driftwm. Caller holds h.mu.
func (h *Hub) rememberLocked(st *driftwm.State) {
	for _, s := range h.ms {
		if s.win == nil {
			continue
		}
		if cur, ok := st.Window(s.win.Window); ok && cur.AppID == s.win.AppID {
			h.places[s.m.ID] = Place{cur.Position[0], cur.Position[1], cur.Size[0], cur.Size[1]}
		}
	}
	// While windows are being brought back the view is not the owner's: every new window pans it. It is
	// not remembered then, so a second crash during a restore still restores the view from before the first.
	if st.Zoom > 0 && !h.restoringLocked() {
		h.lastView = viewSnap{cam: st.Camera, zoom: st.Zoom, ok: true}
	}
	// The same holds for the stacking order: every window that comes back is raised, so the order seen while
	// windows are being restored is not the owner's.
	if !h.restoringLocked() {
		h.lastOrder = h.machineOrderLocked(st)
	}
}

// machineOrderLocked returns the machine ids that have a window in the snapshot, bottom of the stack first. Windows
// that are not a machine's (a terminal started by hand, say) are left out: only the order between the machines' windows
// is kept. Caller holds h.mu.
func (h *Hub) machineOrderLocked(st *driftwm.State) []string {
	byWin := make(map[int]string, len(h.ms))
	for _, s := range h.ms {
		if s.win == nil {
			continue
		}
		if cur, ok := st.Window(s.win.Window); ok && cur.AppID == s.win.AppID {
			byWin[s.win.Window] = s.m.ID
		}
	}
	var out []string
	for _, id := range st.StackOrder() {
		if m, ok := byWin[id]; ok {
			out = append(out, m)
		}
	}
	return out
}

// restoringLocked says whether windows are waiting to be brought back or are being brought back.
func (h *Hub) restoringLocked() bool { return len(h.restoreSet) > 0 || h.restoreRun }

// noteGoneLocked is called when the event stream ends: every machine that had
// a window goes onto the restore list. Caller holds h.mu.
func (h *Hub) noteGoneLocked() {
	// The view first: once the list is filled the restore counts as pending and the view is no longer taken.
	if h.lastView.ok && !h.restoringLocked() {
		h.restoreView = h.lastView
	}
	if !h.restoringLocked() {
		h.restoreOrder = append([]string(nil), h.lastOrder...)
	}
	for _, s := range h.ms {
		if s.win != nil {
			h.restoreSet[s.m.ID] = h.places[s.m.ID]
		}
	}
}

// RestartPending says whether windows are waiting to be brought back.
func (h *Hub) RestartPending() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.restoreSet) > 0 || h.restoreRun
}

func (h *Hub) takeRestore() (map[string]Place, viewSnap, []string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.restoreSet) == 0 {
		return nil, viewSnap{}, nil
	}
	out := make(map[string]Place, len(h.restoreSet))
	for k, v := range h.restoreSet {
		out[k] = v
	}
	return out, h.restoreView, append([]string(nil), h.restoreOrder...)
}

// waitWayland waits until the compositor's Wayland socket accepts a
// connection (the second half of the signal), up to d.
func (h *Hub) waitWayland(ctx context.Context, d time.Duration) bool {
	if h.set.WaylandSocket == "" {
		return true
	}
	deadline := time.Now().Add(d)
	for ctx.Err() == nil {
		c, err := net.DialTimeout("unix", h.set.WaylandSocket, time.Second)
		if err == nil {
			c.Close()
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		select {
		case <-ctx.Done():
		case <-time.After(h.fast()):
		}
	}
	return false
}

func (h *Hub) fast() time.Duration {
	if h.set.FastReconnect > 0 {
		return h.set.FastReconnect
	}
	return 100 * time.Millisecond
}

// runRestore brings the windows back. ctx ends when the compositor goes away
// again; what was not finished stays on the list for the next time.
func (h *Hub) runRestore(ctx context.Context, set map[string]Place, view viewSnap, order []string) {
	start := time.Now()
	h.mu.Lock()
	h.restoreRun = true
	h.mu.Unlock()
	defer func() {
		h.mu.Lock()
		h.restoreRun = false
		h.notifyLocked()
		h.mu.Unlock()
	}()
	if !h.waitWayland(ctx, 30*time.Second) {
		if ctx.Err() == nil {
			h.mu.Lock()
			h.setMessageLocked("the desktop is back but its Wayland socket did not answer, so the windows were not restored; open them from the list")
			for id := range set {
				delete(h.restoreSet, id)
			}
			h.mu.Unlock()
		}
		return
	}
	ids := make([]string, 0, len(set))
	for id := range set {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	h.mu.Lock()
	h.restoring, h.restoreOf = len(ids), len(ids)
	h.notifyLocked()
	h.mu.Unlock()
	var wg sync.WaitGroup
	var rmu sync.Mutex
	why := map[string]string{} // why a machine's open did not give a window at once, or not at its place
	misplaced := map[string]bool{}
	parallel := h.set.RestoreParallel
	if parallel <= 0 {
		parallel = 10
	}
	sem := make(chan struct{}, parallel)
	for _, id := range ids {
		wg.Add(1)
		sem <- struct{}{}
		go func(id string, pl Place) {
			defer wg.Done()
			defer func() { <-sem }()
			r := h.openRestore(id, pl)
			h.logf("restore %s: %s: %s", id, r.Action, r.Message)
			h.mu.Lock()
			if h.restoring > 0 {
				h.restoring--
			}
			h.notifyLocked()
			h.mu.Unlock()
			if r.Action != "open" || r.Problem != "" {
				rmu.Lock()
				why[id] = r.Message
				if r.Problem != "" {
					why[id] = "open, but not at its place: " + r.Problem
					misplaced[id] = true
				}
				rmu.Unlock()
			}
		}(id, set[id])
	}
	wg.Wait()
	// A viewer that was slow to show its window is in the late-window state:
	// its window is still placed at the saved place when it comes (restoreAt
	// stays set), so wait for those too, as long as hubd itself would.
	late := h.set.LateGrace
	if late <= 0 {
		late = 60 * time.Second
	}
	for deadline := time.Now().Add(late + 5*time.Second); ctx.Err() == nil && time.Now().Before(deadline) && h.anyLate(ids); {
		time.Sleep(h.step())
	}
	if ctx.Err() != nil {
		h.mu.Lock()
		h.restoring = 0
		h.mu.Unlock()
		return
	}
	// The stacking order goes back before the view: focusing a window pans the view to it unless it is fully in
	// view, and the view is put back afterwards.
	orderNote := ""
	if err := h.applyStackOrder(ctx, order); err != nil {
		orderNote = "; the stacking order could not be put back: " + err.Error()
	}
	viewNote := ""
	if view.ok {
		if err := h.setView(view); err != nil {
			viewNote = "; the view could not be put back: " + err.Error()
		}
	}
	h.mu.Lock()
	h.restoring = 0
	back := 0
	var failed []string
	for _, id := range ids {
		delete(h.restoreSet, id) // this attempt is over
		if s := h.byID[id]; s != nil {
			s.restoreAt = nil
			if s.win != nil && !misplaced[id] {
				back++
				continue
			}
		}
		m := why[id]
		if m == "" {
			m = "no window came"
		}
		failed = append(failed, fmt.Sprintf("%s (%s)", id, m))
	}
	msg := fmt.Sprintf("the desktop restarted: %d of %d windows are back at their places in %s%s%s", back, len(ids), time.Since(start).Round(100*time.Millisecond), orderNote, viewNote)
	if len(failed) > 0 {
		sort.Strings(failed)
		msg += "; not restored: " + strings.Join(failed, "; ")
	}
	h.setMessageLocked(msg)
	h.mu.Unlock()
	h.logf("%s", msg)
}

// applyStackOrder puts the windows of the machines in order (bottom of the stack first) by focusing them one after the
// other from the bottom: driftwm raises a window it focuses, so the last one ends on top, which is where the
// window that had the focus was. A machine without a window now is skipped. The only way to raise a window in driftwm's
// socket is Focus (there is no separate raise request), so the keyboard focus ends on the top window and the view may
// pan; setView puts the view back afterwards.
func (h *Hub) applyStackOrder(ctx context.Context, order []string) error {
	var failed []string
	var last error
	for _, id := range order {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		h.mu.Lock()
		s := h.byID[id]
		win := -1
		if s != nil && s.win != nil {
			win = s.win.Window
		}
		h.mu.Unlock()
		if win < 0 {
			continue
		}
		if err := h.comp.Focus(win); err != nil {
			failed = append(failed, id)
			last = err
		}
	}
	if len(failed) > 0 {
		return fmt.Errorf("%s: %v", strings.Join(failed, ", "), last)
	}
	return nil
}

// anyLate says whether any of the machines is still waiting for a late window.
func (h *Hub) anyLate(ids []string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, id := range ids {
		if s := h.byID[id]; s != nil && s.phase == phaseLate {
			return true
		}
	}
	return false
}

// openRestore opens one machine's viewer and puts its window at the saved
// place (a size of 0 means the place has no size: the machine's home is used).
func (h *Hub) openRestore(id string, pl Place) OpenResult {
	h.mu.Lock()
	s := h.byID[id]
	if s != nil {
		cp := pl
		s.restoreAt = &cp
	}
	h.mu.Unlock()
	r := h.open(id, true)
	if r.Action != "late" { // a late window is still placed at the saved place when it comes
		h.mu.Lock()
		if s != nil {
			s.restoreAt = nil
		}
		h.mu.Unlock()
	}
	return r
}

// setView sets the zoom, waits until the camera has stopped (a zoom change
// moves the camera while it animates, docs/proposals/driftwm-layouts.md
// section 2.9), and then sets the camera.
func (h *Hub) setView(v viewSnap) error {
	st, err := h.state()
	if err != nil {
		return err
	}
	if math.Abs(st.Zoom-v.zoom) > 0.001 {
		if err := h.comp.SetZoom(v.zoom); err != nil {
			return err
		}
		h.waitCameraStill(15 * time.Second)
	}
	for try := 0; try < 3; try++ {
		if err := h.comp.SetCamera(v.cam[0], v.cam[1]); err != nil {
			return err
		}
		cam, _ := h.waitCameraStill(15 * time.Second)
		if math.Abs(cam[0]-v.cam[0]) <= 1 && math.Abs(cam[1]-v.cam[1]) <= 1 {
			return nil
		}
	}
	return fmt.Errorf("the camera did not reach %.0f, %.0f", v.cam[0], v.cam[1])
}

// waitCameraStill reads the camera until three reads in a row agree, for at
// most max.
func (h *Hub) waitCameraStill(max time.Duration) ([2]float64, bool) {
	deadline := time.Now().Add(max)
	var prev [2]float64
	same := 0
	first := true
	for {
		st, err := h.state()
		if err != nil {
			return prev, false
		}
		if !first && st.Camera == prev {
			same++
			if same >= 3 {
				return prev, true
			}
		} else {
			same = 0
		}
		first, prev = false, st.Camera
		if time.Now().After(deadline) {
			return prev, false
		}
		time.Sleep(h.pause(2 * h.step()))
	}
}

// settleWindow puts a window at a place and watches it until two looks in a
// row agree: a window that has only just appeared can still be put somewhere
// else by the compositor after the first move (see docs/hubd-slice2.md
// section 18.5), so the position is checked and asked for again. It reports
// false if the window is gone or did not stay within SettleMax.
func (h *Hub) settleWindow(id int, appID string, p Place) (bool, error) {
	deadline := time.Now().Add(h.settleMax())
	good, resizes := 0, 0
	var lastResize time.Time
	var lastErr error
	wantSize := p.W > 0 && p.H > 0
	for {
		st, err := h.state()
		if err != nil {
			if goneErr(err) {
				return false, err
			}
			// slow, not gone: ask again until the time is up
			lastErr = err
			if time.Now().After(deadline) {
				return false, lastErr
			}
			time.Sleep(h.pause(h.step()))
			continue
		}
		cur, found := st.Window(id)
		if !found || (appID != "" && cur.AppID != appID) {
			return false, nil
		}
		sizeOK := true
		moved := false
		// driftwm refuses a resize or a move while the window is still settling an earlier one ("under an
		// interactive move or resize, or still settling one"): such an answer is not the end, it is tried again.
		if wantSize && cur.Size != [2]int{p.W, p.H} {
			sizeOK = false
			switch {
			case resizes < 3 && time.Since(lastResize) >= 7*h.step():
				// Ask for the size, then for the place again (a resize keeps the centre or a corner fixed).
				// Not more often than every 7 steps (about 400 ms): the program needs time to draw the new size.
				lastResize = time.Now()
				if err := h.comp.Resize(id, p.W, p.H); err != nil {
					lastErr = err
				} else {
					resizes++
					if err := h.comp.Move(id, p.X, p.Y); err != nil {
						lastErr = err
					}
					moved = true
				}
			case resizes >= 3 && time.Since(lastResize) >= 16*h.step():
				sizeOK = true // the program does not take this size; its place is what counts
			}
		}
		if !moved && cur.Position != [2]int{p.X, p.Y} {
			if err := h.comp.Move(id, p.X, p.Y); err != nil {
				lastErr = err
			}
			moved = true
		}
		if moved || !sizeOK {
			good = 0
		} else {
			good++
		}
		if good >= 2 {
			return true, nil
		}
		if time.Now().After(deadline) {
			return false, lastErr
		}
		time.Sleep(h.pause(h.step()))
	}
}

// ---- restart-desktop ----

// RestartDesktop restarts ONLY the compositor: it sends SIGTERM to the
// process that owns driftwm's control socket (SIGKILL after Settings.KillWait
// if it is still there). The init system's supervisor starts driftwm again
// (it is a service that is meant to be up), and hubd then brings the windows
// back by the direct restore above. It never reboots the machine and never
// touches another service; Waybar restarts by itself because it follows
// driftwm (image/machines/hub/rootfs/usr/lib/hubos/follow-driftwm).
func (h *Hub) RestartDesktop() OpenResult {
	pid, err := h.comp.PeerPID()
	if err != nil {
		return h.refuse("failed", "cannot find the desktop (driftwm is not reachable: %v); nothing was restarted", err)
	}
	if pid <= 1 {
		return h.refuse("failed", "the desktop's process id (%d) makes no sense; nothing was restarted", pid)
	}
	name := h.procName(pid)
	if name != "driftwm" {
		return h.refuse("failed", "the process that owns driftwm's socket is %q (pid %d), not driftwm; nothing was killed", name, pid)
	}
	// Remember the windows now, so that a compositor that closes its windows
	// one by one before it exits does not make hubd forget them.
	h.mu.Lock()
	h.noteGoneLocked()
	h.mu.Unlock()
	sig := h.set.Signal
	if sig == nil {
		sig = func(pid int, s syscall.Signal) error { return syscall.Kill(pid, s) }
	}
	if err := sig(pid, syscall.SIGTERM); err != nil {
		return h.refuse("failed", "could not stop driftwm (pid %d): %v; nothing was restarted", pid, err)
	}
	wait := h.set.KillWait
	if wait <= 0 {
		wait = 10 * time.Second
	}
	deadline := time.Now().Add(wait)
	for time.Now().Before(deadline) {
		if h.procName(pid) != "driftwm" {
			return h.done("restart", "asked driftwm to exit (pid %d); the init system starts it again and the windows come back by themselves", pid)
		}
		time.Sleep(50 * time.Millisecond)
	}
	if h.procName(pid) == "driftwm" {
		if err := sig(pid, syscall.SIGKILL); err != nil {
			return h.refuse("failed", "driftwm (pid %d) did not exit after %s and SIGKILL failed: %v", pid, wait, err)
		}
		return h.done("restart", "driftwm (pid %d) did not exit within %s and was killed; the init system starts it again and the windows come back by themselves", pid, wait)
	}
	return h.done("restart", "asked driftwm to exit (pid %d); the init system starts it again and the windows come back by themselves", pid)
}

func (h *Hub) procName(pid int) string {
	if h.set.ProcName != nil {
		return h.set.ProcName(pid)
	}
	b, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/comm")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}
