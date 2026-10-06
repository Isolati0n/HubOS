package hub

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"hubos/internal/driftwm"
	"hubos/internal/viewers"
)

// OpenResult says what `open` did.
type OpenResult struct {
	Action  string `json:"action"` // "open", "went", "refused", "busy", "failed"
	Message string `json:"message"`
	Problem string `json:"-"` // set when the window is open but could not be put at its place (the restore says so)
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
		if s.ticket != nil && s.ticket.epoch == h.epoch { // an open that began under a compositor that is gone does not count
			h.mu.Unlock()
			return h.done("busy", "%s is already opening; nothing done", name)
		}
	case phaseLate:
		h.mu.Unlock()
		return h.done("busy", "the viewer for %s is still starting; waiting for its window", name)
	case phaseUnmatched:
		h.mu.Unlock()
		return h.refuse("refused", "a viewer for %s was started but its window could not be told apart; close it by hand, then run: hubd forget %s", name, id)
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
	if err == nil {
		_, _, err = v.MatchTitle(s.m)
	}
	if err != nil {
		h.mu.Unlock()
		return h.refuse("refused", "%s cannot be opened: %v", name, err)
	}
	// A title-matched viewer's window is told apart by its title, so the
	// late-window comparison lock does not apply to it.
	if !v.SetsName && v.TitleMatch == "" && h.lateComparison > 0 {
		// A window that shows up late cannot be told from the new viewer's.
		var other string
		for _, o := range h.ms {
			if o.phase == phaseLate && o.lateCmp {
				other = o.m.Name
			}
		}
		h.mu.Unlock()
		return h.refuse("busy", "the viewer for %s is still starting and its window cannot be told apart from this one's; wait for it (or run: hubd forget), then open %s", other, name)
	}
	s.phase = phaseStarting
	tk := &openTicket{epoch: h.epoch}
	s.ticket = tk
	h.notifyLocked()
	h.mu.Unlock()

	r := h.launchAndPlace(s, v, args, tk)

	h.mu.Lock()
	if s.phase == phaseStarting && s.ticket == tk {
		s.phase = phaseIdle
	}
	h.notifyLocked()
	h.mu.Unlock()
	if r.Action != "open" && r.Action != "late" { // waiting is not an alert
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
	st, err := h.state()
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
func (h *Hub) launchAndPlace(s *mstate, v *viewers.Viewer, args []string, tk *openTicket) OpenResult {
	name := s.m.Name
	appID := viewers.AppID(s.m.ID)
	title, _, _ := v.MatchTitle(s.m)
	if title == "" && !v.SetsName {
		// One launch at a time, so "the window that is new" is never
		// ambiguous. A title-matched window is told apart by its title and
		// a viewer that sets its name by that name (every machine has its
		// own), so those launches do not wait for each other (this is what
		// lets a restore start all the viewers at once, docs/hubd-slice2.md
		// section 18).
		h.launchMu.Lock()
		defer h.launchMu.Unlock()
	}

	before, err := h.state()
	if err != nil {
		return h.done("failed", "cannot reach driftwm: %v", err)
	}
	known := map[int]bool{}
	for _, w := range before.Windows {
		known[w.ID] = true
	}
	proc, err := h.launch(s.m.ID, args)
	if err != nil {
		return h.done("failed", "could not start %s for %s: %v", args[0], name, err)
	}

	wait := v.WindowWait
	if wait <= 0 {
		wait = h.set.WindowWait
	}
	deadline := time.Now().Add(wait)
	var cands, others []driftwm.Window
	cleanExit := false
	for {
		select {
		case err := <-proc.Exited:
			proc.Exited = nil
			if err != nil {
				return h.done("failed", "%s", h.exitMessage(s, args[0], err))
			}
			// A clean exit (status 0) may be a hand-over to a copy that is
			// already running: the window can still come, so keep waiting.
			cleanExit = true
		default:
		}
		st, err := h.state()
		if err == nil {
			cands, others = h.candidates(st, known, v, appID, title)
			if len(cands) > 0 {
				time.Sleep(h.set.Settle)
				if st, err = h.state(); err == nil {
					cands, others = h.candidates(st, known, v, appID, title)
				}
				break
			}
		}
		if time.Now().After(deadline) {
			msg := fmt.Sprintf("no window appeared for %s within %s", name, wait)
			if len(others) > 0 {
				w := others[0]
				if title != "" {
					msg += fmt.Sprintf("; a new window appeared but its title is %q, not %q: check title_match in viewers.toml", w.Title, title)
				} else {
					msg += fmt.Sprintf("; a new window appeared but is named %q, not %q: this viewer may ignore the chosen name (set sets_name = false in viewers.toml to match by comparison)", w.AppID, appID)
				}
			}
			if grace := h.graceFor(v); grace > 0 {
				// The viewer is still running (or exited cleanly): keep
				// waiting for its window, and let nothing start a second
				// viewer meanwhile.
				h.beginLate(s, v, proc, cleanExit, grace, known, appID, title, args[0], tk)
				state := "the viewer is still running"
				if cleanExit {
					state = "the viewer process has exited cleanly (it may have handed over to a copy that is already running)"
				}
				return h.done("late", "%s; %s, so hubd keeps waiting for its window for up to %s (the machine is in the \"late window\" state; hubd forget %s stops the wait)", msg, state, grace, s.m.ID)
			}
			if lp := h.LogPath(s.m.ID); lp != "" {
				msg += "; the viewer's output is in " + lp
			}
			return h.done("failed", "%s", msg)
		}
		time.Sleep(h.pause(100 * time.Millisecond))
	}
	switch len(cands) {
	case 0:
		return h.done("failed", "the window for %s went away at once", name)
	case 1:
	default:
		h.mu.Lock()
		s.phase = phaseUnmatched
		h.mu.Unlock()
		return h.done("failed", "%s", ambiguousMessage(name, cands, title))
	}
	return h.recordWindow(s, v, cands[0], "", tk)
}

func ambiguousMessage(name string, cands []driftwm.Window, title string) string {
	titles := make([]string, len(cands))
	for i, w := range cands {
		titles[i] = fmt.Sprintf("#%d %q", w.ID, w.AppID)
	}
	if title != "" {
		return fmt.Sprintf("%d new windows are titled %q while opening %s (%s); I cannot tell which one is its window, so I left them all alone and placed and recorded nothing", len(cands), title, name, strings.Join(titles, ", "))
	}
	return fmt.Sprintf("%d new windows appeared while opening %s (%s); I cannot tell which one is its window, so I left them all alone and placed and recorded nothing", len(cands), name, strings.Join(titles, ", "))
}

func (h *Hub) exitMessage(s *mstate, prog string, err error) string {
	msg := fmt.Sprintf("%s exited (%v) before showing a window for %s", prog, err, s.m.Name)
	if lp := h.LogPath(s.m.ID); lp != "" {
		msg += "; its output is in " + lp
	}
	return msg
}

// candidates are the new windows nobody has claimed: for a viewer that sets
// its name, only those with that name; for a title-matched viewer (title is
// not empty), only those whose title is exactly that (others lists the rest).
func (h *Hub) candidates(st *driftwm.State, known map[int]bool, v *viewers.Viewer, appID, title string) (cands, others []driftwm.Window) {
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
		if title != "" {
			if w.Title != title {
				others = append(others, w)
			} else {
				cands = append(cands, w)
			}
			continue
		}
		if !v.SetsName && slices.Contains(h.set.IgnoreAppIDs, w.AppID) {
			continue // never a candidate for comparison matching
		}
		if !v.SetsName && len(h.titleIDs[w.Title]) > 0 {
			continue // belongs to a title-matched machine, not to this viewer
		}
		if v.SetsName && w.AppID != appID {
			others = append(others, w)
			continue
		}
		cands = append(cands, w)
	}
	return
}

// recordWindow places a found window at home and records it. late is "" for
// a normal open, or a few words for a window that came late.
func (h *Hub) recordWindow(s *mstate, v *viewers.Viewer, w driftwm.Window, late string, tk *openTicket) OpenResult {
	by := "comparison"
	switch {
	case v.SetsName:
		by = "name"
	case v.TitleMatch != "":
		by = "title"
	}
	h.mu.Lock()
	stale := tk.epoch != h.epoch || s.ticket != tk
	h.mu.Unlock()
	if stale {
		// The compositor went away while this viewer was starting: the window is gone with it, and its
		// number may belong to another machine's window in the new compositor. Nothing is moved or recorded.
		return h.done("failed", "the desktop restarted while %s was opening, so its window was dropped", s.m.Name)
	}
	target, src := h.targetFor(s)
	note, gone := h.place(s, w, target, src)
	if gone {
		return h.done("failed", "the window of %s closed (or the desktop went away) before it could be placed", s.m.Name)
	}
	h.mu.Lock()
	if tk.epoch != h.epoch || s.ticket != tk {
		h.mu.Unlock()
		return h.done("failed", "the desktop restarted while %s was opening, so its window was dropped", s.m.Name)
	}
	s.restoreAt = nil
	s.win = &winRec{Machine: s.m.ID, Window: w.ID, AppID: w.AppID, Title: w.Title, By: by}
	h.places[s.m.ID] = target
	h.saveLocked(h.identityLocked())
	h.notifyLocked()
	h.mu.Unlock()
	var msg string
	switch {
	case src == "home":
		msg = fmt.Sprintf("opened %s at home (%d, %d), matched by %s", s.m.Name, target.X, target.Y, by)
	case src == "saved":
		msg = fmt.Sprintf("restored %s at its saved place (%d, %d), matched by %s", s.m.Name, target.X, target.Y, by)
	default:
		msg = fmt.Sprintf("opened %s at its place in %s (%d, %d), matched by %s", s.m.Name, src, target.X, target.Y, by)
	}
	if late != "" {
		msg += " (" + late + ")"
	}
	if note != "" {
		msg += "; " + note
	}
	if st, err := h.state(); err == nil {
		h.syncWindows(st) // also looks for windows with the same name
	}
	r := h.done("open", "%s", msg)
	if strings.Contains(note, "WARNING") || strings.Contains(note, "could not move") {
		r.Problem = note
	}
	return r
}

// ---- the late-window state ----

// graceFor is how long to keep waiting for this viewer's late window.
func (h *Hub) graceFor(v *viewers.Viewer) time.Duration {
	if v.LateGrace > 0 {
		return v.LateGrace
	}
	return h.set.LateGrace
}

// beginLate puts the machine in the late-window state and starts watching.
func (h *Hub) beginLate(s *mstate, v *viewers.Viewer, proc *Proc, cleanExit bool, grace time.Duration, known map[int]bool, appID, title, prog string, tk *openTicket) {
	cancel := make(chan struct{})
	h.mu.Lock()
	s.phase, s.lateCancel, s.lateCmp = phaseLate, cancel, !v.SetsName && v.TitleMatch == ""
	s.lateEnd = h.now().Add(grace)
	if s.lateCmp {
		h.lateComparison++
	}
	h.notifyLocked()
	h.mu.Unlock()
	exitCh := proc.Exited
	if cleanExit {
		exitCh = nil // already gone; a nil channel never fires
	}
	go h.watchLate(s, v, exitCh, grace, known, appID, title, prog, cancel, tk)
}

// leaveLateLocked ends the late-window state (caller holds h.mu).
func (h *Hub) leaveLateLocked(s *mstate, next phase) {
	if s.lateCancel == nil {
		return
	}
	close(s.lateCancel)
	s.lateCancel = nil
	s.restoreAt = nil
	if s.lateCmp {
		h.lateComparison--
	}
	s.lateCmp = false
	s.phase = next
	h.notifyLocked()
}

// finishLate moves the machine on, if this watch is still the current one.
func (h *Hub) finishLate(s *mstate, cancel chan struct{}, next phase, message string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	if s.lateCancel != cancel || s.phase != phaseLate {
		return false
	}
	h.leaveLateLocked(s, next)
	if message != "" {
		h.setMessageLocked(message)
	}
	return true
}

// watchLate waits, for up to Settings.LateGrace, for the window of a viewer
// that is still running. A window that shows up is adopted and placed. If the
// process exits with an error first, that is a failure (with the log path); a
// clean exit (status 0) does not end the wait. If the time
// runs out with the process alive and no window, the machine becomes
// "unidentified" (hubd forget is the way out). It never kills anything.
func (h *Hub) watchLate(s *mstate, v *viewers.Viewer, exitedCh <-chan error, grace time.Duration, known map[int]bool, appID, title, prog string, cancel chan struct{}, tk *openTicket) {
	name := s.m.Name
	end := time.Now().Add(grace)
	for {
		select {
		case <-cancel:
			return
		case err := <-exitedCh:
			if err == nil {
				// A clean exit (status 0) may be a hand-over to a copy that
				// is already running: the window can still come, so keep
				// waiting until the grace ends or a window appears.
				exitedCh = nil
				continue
			}
			h.finishLate(s, cancel, phaseIdle, h.exitMessage(s, prog, err))
			return
		case <-time.After(h.pause(100 * time.Millisecond)):
		}
		if st, err := h.state(); err == nil {
			if cands, _ := h.candidates(st, known, v, appID, title); len(cands) > 0 {
				select {
				case <-cancel:
					return
				case <-time.After(h.set.Settle):
				}
				st, err = h.state()
				if err != nil {
					continue
				}
				cands, _ = h.candidates(st, known, v, appID, title)
				switch len(cands) {
				case 0:
					continue
				case 1:
					// Stay in the late state until the window is recorded, so
					// that nothing can start a second viewer in between.
					h.mu.Lock()
					current := s.lateCancel == cancel && s.phase == phaseLate
					h.mu.Unlock()
					if !current {
						return
					}
					r := h.recordWindow(s, v, cands[0], "its window came late", tk)
					h.finishLate(s, cancel, phaseIdle, r.Message)
					return
				default:
					h.finishLate(s, cancel, phaseUnmatched, ambiguousMessage(name, cands, title))
					return
				}
			}
		}
		if time.Now().After(end) {
			h.finishLate(s, cancel, phaseUnmatched, fmt.Sprintf("the viewer for %s showed no window during the %s of waiting after the window wait; hubd stopped waiting and killed nothing. Close the viewer by hand (if it is still running), then run: hubd forget %s", name, grace, s.m.ID))
			return
		}
	}
}

// targetFor says where a machine's new window goes and why: "saved" (it is
// being restored after a compositor restart), "layout NAME" (a layout is
// active and has the machine), or "home".
func (h *Hub) targetFor(s *mstate) (Place, string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if s.restoreAt != nil && s.restoreAt.W > 0 && s.restoreAt.H > 0 {
		return *s.restoreAt, "saved"
	}
	if h.active != nil {
		if w, ok := h.active.byMachine(s.m.ID); ok {
			return Place{w.X, w.Y, w.W, w.H}, "layout " + h.active.Name
		}
	}
	return Place{X: *s.m.Home.X, Y: *s.m.Home.Y}, "home"
}

// place moves the window to its target (window centre, Y up), checks that it
// stays there (and asks again if it does not, also when driftwm is slow to
// answer), shrinks it if it could not fit below the bar (a home target only:
// a saved or layout size is used as it is), and brings the view to it (not
// when the window is being restored: the view is put back after all the
// windows). The returned note is empty when all went as expected; gone says
// the window (or driftwm) went away first.
func (h *Hub) place(s *mstate, w driftwm.Window, p Place, src string) (note string, gone bool) {
	var notes []string
	x, y := p.X, p.Y
	// The compositor lists a window as soon as its toplevel exists, but it
	// places it only when the first picture arrives (its first commit with a
	// size); a move made before that is overwritten by its own placement
	// (the home-position race, docs/hubd-slice2.md section 18.5). So wait for
	// the first picture before moving.
	switch drawn, gone := h.waitFirstPicture(w.ID, w.AppID); {
	case gone:
		return "", true
	case !drawn:
		notes = append(notes, fmt.Sprintf("WARNING: the window had drawn nothing after %s, so it was moved before the compositor placed it and may stand at the compositor's own spot", h.sizedWait()))
	}
	want := p
	if !(p.W > 0 && p.H > 0) {
		// A home place has no size: keep the window clear of the bar, so shrink it if it is taller than
		// the space below the bar (or wider than the screen).
		if st, err := h.state(); err == nil {
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
					want.W, want.H = nw, nh
					notes = append(notes, fmt.Sprintf("shrunk from %dx%d to %dx%d to stay clear of the bar", cur.Size[0], cur.Size[1], nw, nh))
				}
			}
		}
	}
	// Put it there and look until it stays: a window that has only just appeared can be put somewhere else
	// by the compositor after the first move, and an answer "refused" or "too slow" from driftwm is not the end.
	ok, err := h.settleWindow(w.ID, w.AppID, want)
	switch {
	case err != nil && goneErr(err):
		return "", true
	case err != nil:
		notes = append(notes, fmt.Sprintf("could not move it to its place: %v", err))
		return strings.Join(notes, "; "), false
	case !ok:
		notes = append(notes, fmt.Sprintf("WARNING: it did not stay at (%d, %d)", x, y))
	}
	if src == "saved" {
		return strings.Join(notes, "; "), false
	}
	camBefore := [2]float64{}
	if st2, err2 := h.state(); err2 == nil {
		camBefore = st2.Camera
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
	return strings.Join(notes, "; "), false
}

// minRealFrame: a window whose visible frame is smaller than this in both
// directions has drawn nothing yet (its frame is only the title bar and
// border). No real viewer window is that small.
const minRealFrame = 64

func (h *Hub) sizedWait() time.Duration {
	if h.set.SizedWait > 0 {
		return h.set.SizedWait
	}
	return 60 * time.Second
}

// waitFirstPicture waits until the window has drawn its first picture (its
// frame has a real size), or is gone, or SizedWait has passed. drawn says the
// picture is there; gone says the window (or driftwm) is gone. An answer of
// driftwm that is only slow or refused is asked for again.
func (h *Hub) waitFirstPicture(id int, appID string) (drawn, gone bool) {
	deadline := time.Now().Add(h.sizedWait())
	for {
		st, err := h.state()
		switch {
		case err != nil && goneErr(err):
			return false, true
		case err == nil:
			cur, found := st.Window(id)
			if !found || (appID != "" && cur.AppID != appID) {
				return false, true
			}
			if cur.Size[0] >= minRealFrame || cur.Size[1] >= minRealFrame {
				return true, false
			}
		}
		if time.Now().After(deadline) {
			return false, false
		}
		time.Sleep(h.pause(h.step()))
	}
}

// settledCamera reads the camera until two reads in a row agree, for at
// most a second.
func (h *Hub) settledCamera(start [2]float64) ([2]float64, bool) {
	prev := start
	deadline := time.Now().Add(time.Second)
	same := 0
	for time.Now().Before(deadline) {
		time.Sleep(60 * time.Millisecond)
		st, err := h.state()
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
		if s.phase == phaseLate {
			h.mu.Unlock()
			return h.done("refused", "the viewer for %s is still starting, so there is no window to close. hubd end cannot cancel a viewer (hubd never kills one). To stop waiting for its window, run: hubd forget %s", name, id)
		}
		if s.phase == phaseUnmatched {
			h.mu.Unlock()
			return h.done("refused", "hubd does not know which window is the one of %s, so it will not close any; close it by hand, then run: hubd forget %s", name, id)
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
		st, err := h.state()
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

// Forget drops the "window not identified" block of a machine. It closes
// nothing, moves nothing and does not contact the machine: windows the viewer
// left behind stay where they are, and the owner closes them by hand.
func (h *Hub) Forget(id string) OpenResult {
	h.mu.Lock()
	defer h.mu.Unlock()
	s := h.byID[id]
	if s == nil {
		return h.done("refused", "there is no machine %q", id)
	}
	if s.phase == phaseLate {
		h.leaveLateLocked(s, phaseIdle)
		return h.done("forgot", "stopped waiting for the late window of %s; the viewer was not touched (it may still be running) and no window was closed or moved", s.m.Name)
	}
	if s.phase != phaseUnmatched {
		return h.done("refused", "nothing to forget: hubd has no unidentified window for %s", s.m.Name)
	}
	s.phase = phaseIdle
	h.notifyLocked()
	return h.done("forgot", "forgot the unidentified window of %s; no window was closed or moved and the machine was not contacted", s.m.Name)
}
