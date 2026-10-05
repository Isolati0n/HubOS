package hub

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

// Place is where a machine's window is: the centre and the size of the
// visible frame, Y up. W and H of 0 mean "size not known".
type Place struct{ X, Y, W, H int }

// viewSnap is a view: the camera (the canvas point at the centre of the
// screen, Y up) and the zoom.
type viewSnap struct {
	cam  [2]float64
	zoom float64
	ok   bool
}

// LayoutResult is the answer to a layout command.
type LayoutResult struct {
	OK      bool
	Message string
	Lines   []string
}

func (h *Hub) step() time.Duration {
	if h.set.SettleStep > 0 {
		return h.set.SettleStep
	}
	return 60 * time.Millisecond
}

func (h *Hub) settleMax() time.Duration {
	if h.set.SettleMax > 0 {
		return h.set.SettleMax
	}
	return 6 * time.Second
}

// loadLayouts reads the remembered active layout and the list of layouts at
// start. Anything damaged is moved aside and reported as a warning; nothing
// here can stop hubd.
func (h *Hub) loadLayouts() {
	if h.lay == nil {
		return
	}
	h.layMu.Lock()
	defer h.layMu.Unlock()
	h.lay.cleanTemps()
	var warns []string
	name, err := h.lay.readActive()
	if err != nil {
		warns = append(warns, "the remembered active layout could not be used: "+err.Error())
		name = ""
	}
	if name != "" {
		l, err := h.lay.load(name)
		if err != nil {
			warns = append(warns, fmt.Sprintf("the remembered active layout %s could not be used: %v", name, err))
			h.lay.writeActive("") // so the warning does not repeat at every start
		} else {
			h.mu.Lock()
			h.active = &l
			h.mu.Unlock()
		}
	}
	infos, lw := h.lay.list()
	warns = append(warns, lw...)
	h.mu.Lock()
	h.layoutInfos = infos
	h.warnings = warns
	if len(warns) > 0 {
		h.setMessageLocked("WARNING: " + warns[0])
	}
	h.mu.Unlock()
}

// Warnings are the problems found with the layout files at start-up.
func (h *Hub) Warnings() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]string(nil), h.warnings...)
}

// ActiveLayout is the name of the active layout, or "".
func (h *Hub) ActiveLayout() string {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.active == nil {
		return ""
	}
	return h.active.Name
}

func (h *Hub) layoutFail(format string, args ...any) LayoutResult {
	return LayoutResult{Message: fmt.Sprintf(format, args...)}
}

// Layout runs one layout command: save, apply, list, delete or clear.
func (h *Hub) Layout(sub, name string, replace bool) LayoutResult {
	if h.lay == nil {
		return h.layoutFail("layouts are not set up: hubd was started without a layout folder")
	}
	h.layMu.Lock()
	defer h.layMu.Unlock()
	switch sub {
	case "save":
		return h.saveLayout(name, replace)
	case "apply":
		return h.applyLayout(name)
	case "list":
		return h.listLayouts()
	case "delete":
		return h.deleteLayout(name)
	case "clear":
		return h.clearLayout()
	}
	return h.layoutFail("unknown layout command %q (use save, apply, list, delete or clear)", sub)
}

// layoutErr turns a store error into a plain sentence.
func layoutErr(name string, err error) string {
	switch {
	case errors.Is(err, ErrNoLayout):
		return fmt.Sprintf("there is no layout named %s", name)
	}
	return err.Error()
}

func (h *Hub) refreshInfos() []string {
	infos, warns := h.lay.list()
	h.mu.Lock()
	h.layoutInfos = infos
	h.notifyLocked()
	h.mu.Unlock()
	return warns
}

func (h *Hub) saveLayout(name string, replace bool) LayoutResult {
	if err := ValidLayoutName(name); err != nil {
		return h.layoutFail("%v", err)
	}
	st, err := h.state()
	if err != nil {
		return h.layoutFail("cannot reach driftwm, so nothing was saved: %v", err)
	}
	var ws []LayoutWindow
	h.mu.Lock()
	for _, s := range h.ms {
		if s.win == nil {
			continue
		}
		cur, ok := st.Window(s.win.Window)
		if !ok || cur.AppID != s.win.AppID {
			continue
		}
		ws = append(ws, LayoutWindow{Machine: s.m.ID, X: cur.Position[0], Y: cur.Position[1], W: cur.Size[0], H: cur.Size[1]})
	}
	h.mu.Unlock()
	zoom := st.Zoom
	if zoom <= 0 || zoom > 1 {
		zoom = 1
	}
	l := Layout{Format: layoutFormat, Name: name, Saved: h.now().UTC().Format(time.RFC3339), Windows: ws,
		View: &LayoutView{Zoom: zoom, Cam: st.Camera}}
	if l.Windows == nil {
		l.Windows = []LayoutWindow{}
	}
	if err := h.lay.save(l, replace); err != nil {
		return h.layoutFail("%v", err)
	}
	h.refreshInfos()
	msg := fmt.Sprintf("saved layout %s: %d %s and the view (zoom %.2f, camera %.0f, %.0f)", name, len(ws), plural(len(ws), "window", "windows"), zoom, st.Camera[0], st.Camera[1])
	if len(ws) == 0 {
		msg += "; no machine window was open, so only the view is in it"
	}
	if ov := overlapPairs(ws); len(ov) > 0 {
		msg += "\nWARNING: these windows overlap: " + strings.Join(ov, "; ")
	}
	return LayoutResult{OK: true, Message: msg}
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

func (h *Hub) applyLayout(name string) LayoutResult {
	l, err := h.lay.load(name)
	if err != nil {
		h.refreshInfos()
		return h.layoutFail("%s", layoutErr(name, err))
	}
	// Active first: a machine opened while the windows are being moved already goes to its place.
	h.mu.Lock()
	h.active = &l
	h.notifyLocked()
	h.mu.Unlock()
	var notes []string
	if err := h.lay.writeActive(name); err != nil {
		notes = append(notes, fmt.Sprintf("WARNING: the layout is active now but could not be remembered for the next start: %v", err))
	}
	type job struct {
		id  int
		app string
		w   LayoutWindow
	}
	var jobs []job
	var notOpen, unknown []string
	h.mu.Lock()
	for _, w := range l.Windows {
		s := h.byID[w.Machine]
		switch {
		case s == nil:
			unknown = append(unknown, w.Machine)
		case s.win == nil:
			notOpen = append(notOpen, w.Machine)
		default:
			jobs = append(jobs, job{s.win.Window, s.win.AppID, w})
		}
	}
	h.mu.Unlock()
	if _, err := h.state(); err != nil {
		return LayoutResult{OK: true, Message: fmt.Sprintf("layout %s is active for windows opened later, but driftwm is not reachable, so no window was moved: %v", name, err)}
	}
	// The windows are put at their places at the same time (up to 8): each one is watched until it stays,
	// which takes a while on a slow machine.
	type outcome struct {
		ok  bool
		err error
	}
	outs := make([]outcome, len(jobs))
	var wg sync.WaitGroup
	sem := make(chan struct{}, 8)
	for i, j := range jobs {
		wg.Add(1)
		sem <- struct{}{}
		go func(i int, j job) {
			defer wg.Done()
			defer func() { <-sem }()
			ok, err := h.settleWindow(j.id, j.app, Place{j.w.X, j.w.Y, j.w.W, j.w.H})
			outs[i] = outcome{ok, err}
		}(i, j)
	}
	wg.Wait()
	moved := 0
	for i, j := range jobs {
		switch {
		case outs[i].err != nil:
			notes = append(notes, fmt.Sprintf("WARNING: %s could not be moved: %v", j.w.Machine, outs[i].err))
		case !outs[i].ok:
			notes = append(notes, fmt.Sprintf("WARNING: %s did not stay at its place (its window may have closed)", j.w.Machine))
		default:
			moved++
		}
	}
	viewNote := "the layout has no view"
	if l.View != nil {
		if err := h.setView(viewSnap{cam: l.View.Cam, zoom: l.View.Zoom, ok: true}); err != nil {
			notes = append(notes, "WARNING: the view could not be set: "+err.Error())
			viewNote = "the view was not set"
		} else {
			viewNote = fmt.Sprintf("the view is set (zoom %.2f, camera %.0f, %.0f)", l.View.Zoom, l.View.Cam[0], l.View.Cam[1])
		}
	}
	msg := fmt.Sprintf("applied layout %s: moved %d %s; %s; machines opened later go to their places in it", name, moved, plural(moved, "window", "windows"), viewNote)
	if len(notOpen) > 0 {
		msg += fmt.Sprintf("; not open, so not touched and nothing opened: %s", strings.Join(notOpen, ", "))
	}
	if len(unknown) > 0 {
		msg += fmt.Sprintf("; not in the inventory any more, skipped: %s", strings.Join(unknown, ", "))
	}
	if ov := overlapPairs(l.Windows); len(ov) > 0 {
		msg += "\nWARNING: these windows overlap: " + strings.Join(ov, "; ")
	}
	if len(notes) > 0 {
		msg += "\n" + strings.Join(notes, "\n")
	}
	return LayoutResult{OK: true, Message: msg}
}

func (h *Hub) listLayouts() LayoutResult {
	warns := h.refreshInfos()
	h.mu.Lock()
	infos := append([]LayoutInfo(nil), h.layoutInfos...)
	act := ""
	if h.active != nil {
		act = h.active.Name
	}
	h.mu.Unlock()
	var lines []string
	for _, i := range infos {
		mark := "  "
		if i.Name == act {
			mark = "* "
		}
		lines = append(lines, fmt.Sprintf("%s%-24s %3d %-7s  saved %s", mark, i.Name, i.Windows, plural(i.Windows, "window", "windows"), i.Saved))
	}
	for _, w := range warns {
		lines = append(lines, "WARNING: "+w)
	}
	msg := fmt.Sprintf("%d %s", len(infos), plural(len(infos), "layout", "layouts"))
	if act != "" {
		msg += "; the active one is marked with *"
	} else {
		msg += "; none is active"
	}
	return LayoutResult{OK: true, Message: msg, Lines: lines}
}

func (h *Hub) deleteLayout(name string) LayoutResult {
	if err := h.lay.delete(name); err != nil {
		return h.layoutFail("%s", layoutErr(name, err))
	}
	msg := fmt.Sprintf("deleted layout %s", name)
	h.mu.Lock()
	wasActive := h.active != nil && h.active.Name == name
	if wasActive {
		h.active = nil
	}
	h.mu.Unlock()
	if wasActive {
		if err := h.lay.writeActive(""); err != nil {
			msg += fmt.Sprintf("; it was the active layout, now none is, but that could not be remembered: %v", err)
		} else {
			msg += "; it was the active layout, so now none is"
		}
	}
	h.refreshInfos()
	return LayoutResult{OK: true, Message: msg}
}

func (h *Hub) clearLayout() LayoutResult {
	h.mu.Lock()
	was := h.active
	h.active = nil
	h.notifyLocked()
	h.mu.Unlock()
	if err := h.lay.writeActive(""); err != nil {
		return LayoutResult{OK: true, Message: fmt.Sprintf("no layout is active now, but that could not be remembered for the next start: %v", err)}
	}
	if was == nil {
		return LayoutResult{OK: true, Message: "no layout was active; windows opened later go to their home positions"}
	}
	return LayoutResult{OK: true, Message: fmt.Sprintf("layout %s is no longer active; windows opened later go to their home positions (open windows were not moved)", was.Name)}
}

// layoutLines are the "Layouts" group of the menu. Nothing is shown when
// there are no layouts. Caller holds h.mu.
func (h *Hub) layoutLinesLocked() []string {
	if h.lay == nil || (len(h.layoutInfos) == 0 && h.active == nil) {
		return nil
	}
	act := ""
	if h.active != nil {
		act = h.active.Name
	}
	open := h.isOpenLocked("layouts", len(h.layoutInfos))
	mark := "-"
	if !open {
		mark = "+"
	}
	head := fmt.Sprintf("%s %s (%d", mark, LayoutsLabel, len(h.layoutInfos))
	if act != "" {
		head += ", active: " + act
	}
	lines := []string{head + ")"}
	if !open {
		return lines
	}
	for _, i := range h.layoutInfos {
		tag := ""
		if i.Name == act {
			tag = "  ACTIVE"
		}
		lines = append(lines, fmt.Sprintf("   %s %s %-24s %3d %s%s", layoutMark, layoutApply, i.Name, i.Windows, plural(i.Windows, "window", "windows"), tag))
	}
	if act != "" {
		lines = append(lines, fmt.Sprintf("   %s %s", layoutMark, layoutClear))
	}
	return lines
}

// Menu words for the Layouts group (parsed again in Pick).
const (
	LayoutsLabel = "Layouts"
	layoutMark   = "="
	layoutApply  = "apply"
	layoutClear  = "clear the active layout"
)
