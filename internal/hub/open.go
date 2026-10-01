package hub

import (
	"fmt"
	"strings"
	"time"

	"hubos/internal/driftwm"
	"hubos/internal/viewers"
)

// OpenResult says what `open` did.
type OpenResult struct {
	Action  string `json:"action"` // "open", "went", "refused", "busy", "failed"
	Message string `json:"message"`
}

func (h *Hub) done(action, format string, args ...any) OpenResult {
	return OpenResult{Action: action, Message: fmt.Sprintf(format, args...)}
}

// refuse and fail also put the text on the bar item for a while.
func (h *Hub) refuse(action, format string, args ...any) OpenResult {
	r := h.done(action, format, args...)
	h.mu.Lock()
	h.setMessageLocked(r.Message)
	h.mu.Unlock()
	return r
}

// Open starts the machine's viewer, or goes to its window if one is open.
func (h *Hub) Open(id string) OpenResult {
	return h.open(id, true)
}

func (h *Hub) open(id string, mayRetry bool) OpenResult {
	h.mu.Lock()
	s := h.byID[id]
	if s == nil {
		h.mu.Unlock()
		return h.refuse("refused", "there is no machine %q", id)
	}
	name := s.m.Name
	if s.m.Role == "hub" || s.m.Open[0] == "none" {
		h.mu.Unlock()
		return h.refuse("refused", "%s has nothing to open", name)
	}
	// 1. A window we know of: go to it, even if the machine is down now.
	if s.win != nil {
		w := *s.win
		h.mu.Unlock()
		ok, err := h.goTo(w)
		if ok {
			return h.done("went", "went to the open window of %s", name)
		}
		if err != nil {
			return h.refuse("failed", "cannot reach driftwm: %v", err)
		}
		// The window is gone; forget it and open afresh.
		h.mu.Lock()
		if s.win != nil && *s.win == w {
			s.win = nil
			h.saveLocked(h.identityLocked())
			h.notifyLocked()
		}
		h.mu.Unlock()
		if mayRetry {
			return h.open(id, false)
		}
		return h.refuse("failed", "the window of %s disappeared", name)
	}
	// 2. Already being opened: do nothing.
	switch s.phase {
	case phaseStarting:
		h.mu.Unlock()
		return h.done("busy", "%s is already opening; nothing done", name)
	case phaseUnmatched:
		h.mu.Unlock()
		return h.refuse("refused", "a viewer for %s was started but its window could not be told apart; close it by hand, then run: hubd end %s", name, id)
	}
	// 3. Is the machine fit to open?
	switch s.status {
	case statusDown:
		h.mu.Unlock()
		return h.refuse("refused", "%s is down, not opened", name)
	case statusChecking:
		h.mu.Unlock()
		return h.refuse("refused", "%s has not been checked yet; try again in a moment", name)
	}
	if !h.driftwmUp {
		h.mu.Unlock()
		return h.refuse("failed", "driftwm is not reachable, so no window can be opened")
	}
	prog := s.m.Open[0]
	v := h.vt.For(prog)
	if v == nil {
		h.mu.Unlock()
		return h.refuse("refused", "no viewer is set for %q in viewers.toml, so %s cannot be opened", prog, name)
	}
	args, err := v.Args(s.m)
	if err != nil {
		h.mu.Unlock()
		return h.refuse("refused", "%s cannot be opened: %v", name, err)
	}
	s.phase = phaseStarting
	h.notifyLocked()
	h.mu.Unlock()

	r := h.launchAndPlace(s, v, args)

	h.mu.Lock()
	if s.phase == phaseStarting {
		s.phase = phaseIdle
	}
	h.notifyLocked()
	h.mu.Unlock()
	if r.Action != "open" {
		h.mu.Lock()
		h.setMessageLocked(r.Message)
		h.mu.Unlock()
	}
	return r
}

func (h *Hub) identityLocked() string {
	id, _ := h.comp.Identity()
	return id
}

// goTo focuses the window if driftwm still has it. ok is false when the
// window is gone; err is set when driftwm could not be asked at all.
func (h *Hub) goTo(w winRec) (ok bool, err error) {
	st, err := h.comp.State()
	if err != nil {
		return false, err
	}
	cur, found := st.Window(w.Window)
	if !found || cur.AppID != w.AppID {
		return false, nil
	}
	if err := h.comp.Focus(w.Window); err != nil {
		return false, err
	}
	return true, nil
}

// launchAndPlace starts the viewer, finds its window, puts it at the
// machine's home, and records it. One launch at a time across the hub, so
// "the window that is new" is never ambiguous because of another launch of
// ours.
func (h *Hub) launchAndPlace(s *mstate, v *viewers.Viewer, args []string) OpenResult {
	h.launchMu.Lock()
	defer h.launchMu.Unlock()
	name := s.m.Name
	appID := viewers.AppID(s.m.ID)

	before, err := h.comp.State()
	if err != nil {
		return h.done("failed", "cannot reach driftwm: %v", err)
	}
	known := map[int]bool{}
	for _, w := range before.Windows {
		known[w.ID] = true
	}
	proc, err := h.launch(args)
	if err != nil {
		return h.done("failed", "could not start %s for %s: %v", args[0], name, err)
	}

	newWindows := func(st *driftwm.State) (cands, others []driftwm.Window) {
		h.mu.Lock()
		claimed := map[int]bool{}
		for _, o := range h.ms {
			if o.win != nil {
				claimed[o.win.Window] = true
			}
		}
		h.mu.Unlock()
		for _, w := range st.Windows {
			if known[w.ID] || claimed[w.ID] {
				continue
			}
			if v.SetsName && w.AppID != appID {
				others = append(others, w)
				continue
			}
			cands = append(cands, w)
		}
		return
	}

	deadline := time.Now().Add(h.set.WindowWait)
	var cands, others []driftwm.Window
	for {
		select {
		case err := <-proc.Exited:
			proc.Exited = nil // a clean exit may be a hand-over to a running copy
			if err != nil {
				return h.done("failed", "%s exited (%v) before showing a window for %s", args[0], err, name)
			}
		default:
		}
		st, err := h.comp.State()
		if err == nil {
			cands, others = newWindows(st)
			if len(cands) > 0 {
				time.Sleep(h.set.Settle)
				if st, err = h.comp.State(); err == nil {
					cands, others = newWindows(st)
				}
				break
			}
		}
		if time.Now().After(deadline) {
			msg := fmt.Sprintf("no window appeared for %s within %s", name, h.set.WindowWait)
			if len(others) > 0 {
				w := others[0]
				msg += fmt.Sprintf("; a new window appeared but is named %q, not %q: this viewer may ignore the chosen name (set sets_name = false in viewers.toml to match by comparison)", w.AppID, appID)
			}
			return h.done("failed", "%s", msg)
		}
		time.Sleep(100 * time.Millisecond)
	}
	switch len(cands) {
	case 0:
		return h.done("failed", "the window for %s went away at once", name)
	case 1:
	default:
		h.mu.Lock()
		s.phase = phaseUnmatched
		h.mu.Unlock()
		titles := make([]string, len(cands))
		for i, w := range cands {
			titles[i] = fmt.Sprintf("#%d %q", w.ID, w.AppID)
		}
		return h.done("failed", "%d new windows appeared while opening %s (%s); I cannot tell which one is its window, so I left them all alone and placed and recorded nothing", len(cands), name, strings.Join(titles, ", "))
	}

	w := cands[0]
	by := "comparison"
	if v.SetsName {
		by = "name"
	}
	note := h.place(s, w)
	h.mu.Lock()
	s.win = &winRec{Machine: s.m.ID, Window: w.ID, AppID: w.AppID, Title: w.Title, By: by}
	h.saveLocked(h.identityLocked())
	h.notifyLocked()
	h.mu.Unlock()
	msg := fmt.Sprintf("opened %s at home (%d, %d), matched by %s", name, *s.m.Home.X, *s.m.Home.Y, by)
	if note != "" {
		msg += "; " + note
	}
	return h.done("open", "%s", msg)
}

// place moves the window to the machine's home (window centre, Y up),
// shrinks it if it could not fit below the bar, and brings the view to it.
// The returned note is empty when all went as expected.
func (h *Hub) place(s *mstate, w driftwm.Window) string {
	var notes []string
	x, y := *s.m.Home.X, *s.m.Home.Y
	if err := h.comp.Move(w.ID, x, y); err != nil {
		return fmt.Sprintf("could not move it to its home: %v", err)
	}
	st, err := h.comp.State()
	if err == nil {
		vw, vh := st.Viewport()
		if cur, ok := st.Window(w.ID); ok && vw > 0 {
			usableH := vh - h.set.BarHeight
			nw, nh := cur.Size[0], cur.Size[1]
			if nh > usableH {
				nh = usableH
			}
			if nw > vw {
				nw = vw
			}
			if nw != cur.Size[0] || nh != cur.Size[1] {
				if err := h.comp.Resize(w.ID, nw, nh); err != nil {
					notes = append(notes, fmt.Sprintf("it is taller than the space below the bar and could not be shrunk: %v", err))
				} else {
					notes = append(notes, fmt.Sprintf("shrunk from %dx%d to %dx%d to stay clear of the bar", cur.Size[0], cur.Size[1], nw, nh))
					// Resizing keeps the top left fixed or the centre fixed;
					// move again so the centre is at home.
					h.comp.Move(w.ID, x, y)
				}
			}
		}
	}
	camBefore := [2]float64{}
	if err == nil {
		camBefore = st.Camera
	}
	if err := h.comp.Focus(w.ID); err != nil {
		notes = append(notes, fmt.Sprintf("could not bring the view to it: %v", err))
	} else if h.set.BarHeight > 0 {
		// driftwm pans the view with an animation, so wait until the camera
		// has stopped before reading it. It centres the view on the area
		// below the bar, which is half the bar height off the screen centre
		// (docs/bar-findings.md section 7).
		if cam, ok := h.settledCamera(camBefore); ok && cam != camBefore {
			want := float64(y) + float64(h.set.BarHeight)/2
			if d := cam[1] - want; d > 2 || d < -2 {
				notes = append(notes, fmt.Sprintf("the view is at y=%.1f, not %.1f (home plus half the bar height); the bar offset may be different from what hubd expects", cam[1], want))
			}
		}
	}
	return strings.Join(notes, "; ")
}

// settledCamera reads the camera until two reads in a row agree, for at
// most a second.
func (h *Hub) settledCamera(start [2]float64) ([2]float64, bool) {
	prev := start
	deadline := time.Now().Add(time.Second)
	same := 0
	for time.Now().Before(deadline) {
		time.Sleep(60 * time.Millisecond)
		st, err := h.comp.State()
		if err != nil {
			return prev, false
		}
		if st.Camera == prev {
			same++
			if same >= 2 && prev != start {
				return prev, true
			}
			if same >= 3 { // it never moved: the window was already in view
				return prev, false
			}
		} else {
			same = 0
		}
		prev = st.Camera
	}
	return prev, prev != start
}

// End closes the local window of a machine. It asks driftwm to close the
// window; it never kills a process and never contacts the machine, so the
// machine and its session are not touched.
func (h *Hub) End(id string) OpenResult {
	h.mu.Lock()
	s := h.byID[id]
	if s == nil {
		h.mu.Unlock()
		return h.done("refused", "there is no machine %q", id)
	}
	name := s.m.Name
	if s.win == nil {
		if s.phase == phaseUnmatched {
			s.phase = phaseIdle
			h.notifyLocked()
			h.mu.Unlock()
			return h.done("end", "forgot the unidentified viewer of %s; its windows were not touched, close them by hand", name)
		}
		h.mu.Unlock()
		return h.done("refused", "hubd has no window open for %s", name)
	}
	w := *s.win
	h.mu.Unlock()
	if err := h.comp.Close(w.Window); err != nil {
		return h.done("failed", "driftwm would not close the window of %s: %v", name, err)
	}
	deadline := time.Now().Add(h.set.CloseWait)
	for time.Now().Before(deadline) {
		st, err := h.comp.State()
		if err == nil {
			if _, still := st.Window(w.Window); !still {
				h.mu.Lock()
				if s.win != nil && s.win.Window == w.Window {
					s.win = nil
					h.saveLocked(h.identityLocked())
				}
				h.notifyLocked()
				h.mu.Unlock()
				return h.done("end", "closed the local window of %s; the machine and its session were not touched", name)
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	return h.done("failed", "asked the window of %s to close, but it is still open (the program may be asking something); nothing was forced", name)
}
