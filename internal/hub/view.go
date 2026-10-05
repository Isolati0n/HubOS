package hub

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"hubos/internal/inventory"
)

// StatusLine is what Waybar's custom module reads: one JSON object.
type StatusLine struct {
	Text    string `json:"text"`
	Class   string `json:"class"` // "ok" or "alert"
	Tooltip string `json:"tooltip"`
}

// markupEscaper makes text safe for Pango markup, which Waybar 0.9.24 reads in
// the bar text and in the tooltip: without it a "&" or "<" in a machine name
// makes GTK refuse the text and the tooltip is blank (docs/hubd-slice2.md).
var markupEscaper = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;", "'", "&#39;")

// RawJSON is the one-line form without markup escaping (hubd serve --no-escape).
func (s StatusLine) RawJSON() string {
	b, _ := json.Marshal(s)
	return string(b)
}

// StatusJSON is the line the feed sends: escaped for Waybar unless the hub was
// started with --no-escape.
func (h *Hub) StatusJSON() string {
	st := h.Status()
	if h.set.NoEscape {
		return st.RawJSON()
	}
	return st.JSON()
}

// JSON is the one-line form for Waybar: the text and tooltip are escaped for
// markup, so the module must be configured without Waybar's own "escape".
func (s StatusLine) JSON() string {
	s.Text, s.Tooltip = markupEscaper.Replace(s.Text), markupEscaper.Replace(s.Tooltip)
	b, _ := json.Marshal(s)
	return string(b)
}

// Counts is the numbers behind the status line.
type Counts struct {
	Up, Down, NotChecked, Checking int
	Stale                          bool
}

func (h *Hub) countLocked() (c Counts, down []*mstate) {
	for _, s := range h.ms {
		if s.m.Role == "hub" {
			continue
		}
		switch s.status {
		case statusUp:
			c.Up++
		case statusDown:
			c.Down++
			down = append(down, s)
		case statusNotChecked, statusNoHandles:
			c.NotChecked++
		case statusChecking:
			c.Checking++
		}
	}
	// STALE: no check result has arrived for three intervals.
	c.Stale = h.now().Sub(h.lastResult) > 3*h.set.ProbeInterval
	return
}

// Status builds the status line for the bar.
func (h *Hub) Status() StatusLine {
	h.mu.Lock()
	defer h.mu.Unlock()
	c, down := h.countLocked()
	var tip []string
	if m := h.activeMessageLocked(); m != "" {
		tip = append(tip, m)
	}
	warnings := h.dupWarningsLocked()
	tip = append(tip, warnings...)
	tip = append(tip, h.waitingLinesLocked()...)
	if !h.driftwmUp {
		tip = append(tip, "driftwm is not reachable: windows cannot be opened")
	}
	if c.Stale {
		// Ages are shown in 10 s steps so the tooltip changes at most every 10 s.
		age := h.now().Sub(h.lastResult).Truncate(10 * time.Second)
		round := "no check round has finished yet"
		if h.rounds > 0 {
			round = fmt.Sprintf("the last check round finished %s ago", h.now().Sub(h.lastRound).Truncate(10*time.Second))
		}
		tip = append(tip, fmt.Sprintf("STALE: no check result for %s (limit: 3 intervals = %s); %s", age, 3*h.set.ProbeInterval, round))
	}
	line := StatusLine{Class: "ok"}
	first := h.rounds == 0 && h.checkable > 0
	if first {
		// No partial up and down numbers while the first round is running.
		line.Text = fmt.Sprintf("checking... %d of %d done", h.firstDone, h.checkable)
		tip = append(tip, "first check round in progress")
	} else {
		line.Text = fmt.Sprintf("%d of %d up", c.Up, c.Up+c.Down)
	}
	if c.Stale {
		line.Text = "STALE: " + line.Text
	}
	if c.Stale || len(warnings) > 0 || h.activeMessageLocked() != "" || (!first && c.Down > 0) {
		line.Class = "alert"
	}
	if c.Down > 0 && !first {
		sort.SliceStable(down, func(i, j int) bool { return down[i].m.ID < down[j].m.ID })
		names := make([]string, 0, h.set.TooltipCap)
		for i, s := range down {
			if i == h.set.TooltipCap {
				break
			}
			names = append(names, s.m.Name)
		}
		t := fmt.Sprintf("%d down: %s", c.Down, strings.Join(names, ", "))
		if c.Down > len(names) {
			t += fmt.Sprintf(" and %d more", c.Down-len(names))
		}
		tip = append(tip, t)
	}
	if c.NotChecked > 0 {
		tip = append(tip, fmt.Sprintf("%d not checked", c.NotChecked))
	}
	if len(tip) == 0 {
		tip = append(tip, "all machines up")
	}
	if h.rounds > 0 {
		tip = append(tip, fmt.Sprintf("last check round took %s", h.lastTook.Round(time.Millisecond)))
		if h.lastTook > 2*h.set.ProbeInterval {
			// A plain note, not an alert: results are still arriving.
			tip = append(tip, fmt.Sprintf("note: that is longer than two intervals (%s); a bigger check cap or interval may fit this many machines better", 2*h.set.ProbeInterval))
		}
	}
	line.Tooltip = strings.Join(tip, "\n")
	return line
}

// Counts returns the numbers behind the status line.
func (h *Hub) Counts() Counts {
	h.mu.Lock()
	defer h.mu.Unlock()
	c, _ := h.countLocked()
	return c
}

// ---- the menu list ----

var roleOrder = []string{"hub", "gaming", "ai", "desktop", "nas", "backup-nas", "vm-host"}

var roleLabel = map[string]string{
	"hub": "Hub", "gaming": "Gaming", "ai": "AI", "desktop": "Desktop",
	"nas": "NAS", "backup-nas": "Backup NAS", "vm-host": "VM host",
}

// Lines of the menu use this grammar (parsed again in Pick):
//
//	"! text"           a message; ignored if picked
//	"? search ..."     ask for text, then show a flat list of the matches
//	"< back ..."       switch back to the groups
//	"x forget unknown window for <name> (<id>)"   drop the block on that machine
//	"- Label (...)"    an open group heading; picking it folds the group
//	"+ Label (...)"    a folded group heading; picking it opens the group
//	" ● id  name  S"   a machine whose window is open (the marker is filled)
//	" ○ id  name  S"   a machine whose window is not open (the marker is empty)
//
// The marker sits inside the indent, so the id, name and status columns stay
// where they were. The machine's first word after the marker is its id.
const (
	forgetPrefix = "x forget unknown window for "
	stopPrefix   = "x stop waiting for "
	searchLine   = "? search by id or name..."
	backLine     = "< back to groups"
	// MarkerOpen and MarkerClosed start every machine line: filled when hubd
	// holds a window for the machine, empty otherwise. Both are one column wide
	// in the menu's fixed-width font (DejaVu Sans Mono has both).
	MarkerOpen   = "\u25cf" // ●
	MarkerClosed = "\u25cb" // ○
	// SearchHint is the one line shown when the menu asks for text to search
	// for. If it comes back unchanged, nothing was typed.
	SearchHint = "type part of an id or name, then press Enter"
)

type group struct {
	key, label string
	indent     int
	members    []*mstate
	children   map[string]*group // guest groups by host id, for vm-host members
}

func (h *Hub) statusTextLocked(s *mstate) string {
	var t string
	switch s.status {
	case statusUp:
		t = "UP"
		if s.m.Role == "hub" {
			t = "THIS HUB"
		}
	case statusDown:
		t = "DOWN"
	case statusChecking:
		t = "checking..."
	case statusNoHandles:
		t = "NOT CHECKED (out of file handles)"
	default:
		t = "NOT CHECKED (" + s.reason + ")"
	}
	if s.phase == phaseStarting {
		t += " [opening]"
	}
	if s.phase == phaseLate {
		t += " [waiting for its window]"
	}
	if s.phase == phaseUnmatched {
		t += " [window not identified]"
	}
	return t
}

// markerLocked is the open marker of a machine: filled when hubd holds a window
// for it (the same fact that makes a pick go to that window), empty otherwise.
// A window that belongs to no machine (the menu, the bar) is never in the list.
func markerLocked(s *mstate) string {
	if s.win != nil {
		return MarkerOpen
	}
	return MarkerClosed
}

// entryLocked is one machine line: the marker takes the last two columns of the
// indent (indent is at least 3), so the columns do not move.
func (h *Hub) entryLocked(s *mstate, indent int) string {
	return fmt.Sprintf("%s%s %-16s %-24s %s", strings.Repeat(" ", indent-2), markerLocked(s), s.m.ID, s.m.Name, h.statusTextLocked(s))
}

// rank puts down machines first, then those not checked, then up.
func rank(s *mstate) int {
	switch s.status {
	case statusDown:
		return 0
	case statusUp:
		return 2
	}
	return 1
}

func sortMembers(ms []*mstate) {
	sort.SliceStable(ms, func(i, j int) bool { return rank(ms[i]) < rank(ms[j]) })
}

func (h *Hub) isOpenLocked(key string, n int) bool {
	if v, ok := h.expanded[key]; ok {
		return v
	}
	return n <= h.set.FoldThreshold
}

func countDown(ms []*mstate) (down int) {
	for _, s := range ms {
		if s.status == statusDown {
			down++
		}
	}
	return
}

func headingText(label string, ms []*mstate) string {
	t := fmt.Sprintf("%s (%d", label, len(ms))
	if len(ms) == 1 {
		t += " machine"
	} else {
		t += " machines"
	}
	if d := countDown(ms); d > 0 {
		t += fmt.Sprintf(", %d down", d)
	}
	return t + ")"
}

// groupsLocked builds the groups in menu order, with guests nested under
// their host. It returns every group (for Pick) in order of appearance.
func (h *Hub) groupsLocked() []*group {
	byRole := map[string][]*mstate{}
	guests := map[string][]*mstate{}
	for _, s := range h.ms {
		if s.m.Role == "guest" {
			guests[s.m.Host] = append(guests[s.m.Host], s)
		} else {
			byRole[s.m.Role] = append(byRole[s.m.Role], s)
		}
	}
	var out []*group
	for _, role := range roleOrder {
		ms := byRole[role]
		if len(ms) == 0 {
			continue
		}
		sortMembers(ms)
		g := &group{key: "role:" + role, label: roleLabel[role], members: ms, children: map[string]*group{}}
		for _, s := range ms {
			if gs := guests[s.m.ID]; len(gs) > 0 {
				sortMembers(gs)
				g.children[s.m.ID] = &group{key: "guests:" + s.m.ID, label: "Guests of " + s.m.ID, indent: 3, members: gs}
			}
		}
		out = append(out, g)
	}
	return out
}

// forgetLinesLocked has one line for each machine whose window could not be
// told apart, so the owner can clear the block from the menu.
func (h *Hub) forgetLinesLocked() []string {
	var out []string
	for _, s := range h.ms {
		switch s.phase {
		case phaseUnmatched:
			out = append(out, fmt.Sprintf("%s%s (%s)", forgetPrefix, s.m.Name, s.m.ID))
		case phaseLate:
			out = append(out, fmt.Sprintf("%s%s's window (%s)", stopPrefix, s.m.Name, s.m.ID))
		}
	}
	return out
}

// waitingLinesLocked has one tooltip line for each machine in the late-window
// state: "waiting for X's window (up to N s more)", N rounded up to 10 s steps
// so the tooltip changes at most every 10 s. It is information, not an alert.
func (h *Hub) waitingLinesLocked() []string {
	var out []string
	for _, s := range h.ms {
		if s.phase != phaseLate {
			continue
		}
		left := s.lateEnd.Sub(h.now())
		if left < 0 {
			left = 0
		}
		secs := int((left + 9*time.Second) / (10 * time.Second) * 10)
		out = append(out, fmt.Sprintf("waiting for %s's window (up to %d s more)", s.m.Name, secs))
	}
	return out
}

// DownLabel is the heading of the group of machines that are down.
const DownLabel = "Down machines"

// downGroupLocked is the "Down machines" group at the top of the list: every
// machine that is down (the hub excepted), at most Settings.DownMax lines,
// then a line saying how many more there are. It is open unless the owner
// folded it. Nothing is hidden without a line that says so.
func (h *Hub) downGroupLocked() []string {
	var down []*mstate
	for _, s := range h.ms {
		if s.m.Role != "hub" && s.status == statusDown {
			down = append(down, s)
		}
	}
	if len(down) == 0 {
		return nil
	}
	open := true
	if v, ok := h.expanded["down"]; ok {
		open = v
	}
	mark := "-"
	if !open {
		mark = "+"
	}
	lines := []string{fmt.Sprintf("%s %s (%d)", mark, DownLabel, len(down))}
	if !open {
		return lines
	}
	for i, s := range down {
		if i == h.set.DownMax {
			lines = append(lines, fmt.Sprintf("   ! and %d more down machines; use the search", len(down)-i))
			break
		}
		lines = append(lines, h.entryLocked(s, 3))
	}
	return lines
}

// List returns the menu lines. flat lists machines in one run, down first;
// filter (case-insensitive, matched against id and name) narrows that list.
// No list is longer than Settings.ListMax machine lines: wofi's start-up
// time grows much faster than the number of lines (docs/hubd-slice2.md).
func (h *Hub) List(flat bool, filter string) []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	var lines []string
	if m := h.activeMessageLocked(); m != "" {
		lines = append(lines, "! "+m)
	}
	for _, w := range h.dupWarningsLocked() {
		lines = append(lines, "! "+w)
	}
	lines = append(lines, h.forgetLinesLocked()...)
	if flat || filter != "" {
		lines = append(lines, backLine)
		var all []*mstate
		f := strings.ToLower(filter)
		for _, s := range h.ms {
			if f == "" || strings.Contains(strings.ToLower(s.m.ID), f) || strings.Contains(strings.ToLower(s.m.Name), f) {
				all = append(all, s)
			}
		}
		sortMembers(all)
		if filter != "" {
			lines = append(lines, fmt.Sprintf("! %d match %q", len(all), filter))
		}
		for i, s := range all {
			if i == h.set.ListMax {
				lines = append(lines, fmt.Sprintf("! %d more not shown; type more letters to narrow the search", len(all)-i))
				break
			}
			lines = append(lines, h.entryLocked(s, 3))
		}
		return lines
	}
	lines = append(lines, searchLine)
	lines = append(lines, h.downGroupLocked()...)
	for _, g := range h.groupsLocked() {
		open := h.isOpenLocked(g.key, len(g.members))
		mark := "+"
		if open {
			mark = "-"
		}
		lines = append(lines, mark+" "+headingText(g.label, g.members))
		if !open {
			continue
		}
		for i, s := range g.members {
			if i == h.set.ListMax {
				lines = append(lines, fmt.Sprintf("   ! %d more in this group not shown; use the search", len(g.members)-i))
				break
			}
			lines = append(lines, h.entryLocked(s, 3))
			if c := g.children[s.m.ID]; c != nil {
				copen := h.isOpenLocked(c.key, len(c.members))
				cmark := "+"
				if copen {
					cmark = "-"
				}
				lines = append(lines, strings.Repeat(" ", c.indent)+cmark+" "+headingText(c.label, c.members))
				if copen {
					for j, gs := range c.members {
						if j == h.set.ListMax {
							lines = append(lines, fmt.Sprintf("      ! %d more in this group not shown; use the search", len(c.members)-j))
							break
						}
						lines = append(lines, h.entryLocked(gs, 6))
					}
				}
			}
		}
	}
	return lines
}

// PickResult tells the menu what to do next.
type PickResult struct {
	Action  string `json:"action"`            // "open", "went", "refused", "failed", "busy", "toggle", "search", "back", "ignored"
	Message string `json:"message,omitempty"` // plain words for the owner
	Reopen  bool   `json:"reopen"`            // show the menu again at once
	Flat    bool   `json:"flat"`              // ... as the flat list
	Ask     bool   `json:"ask,omitempty"`     // ... after asking for text to search for
}

// Pick handles one line chosen in the launcher. It acts only on a line that
// is exactly a machine line (indented, first word an id that exists, after the
// open marker if there is one), a
// group heading, or one of the two switch lines. Anything else is ignored.
func (h *Hub) Pick(line string) PickResult {
	line = strings.TrimRight(line, "\r\n")
	if strings.TrimSpace(line) == "" {
		return PickResult{Action: "ignored"}
	}
	trim := strings.TrimLeft(line, " ")
	indented := len(trim) < len(line)
	switch {
	case trim == searchLine && !indented:
		return PickResult{Action: "search", Reopen: true, Flat: true, Ask: true}
	case trim == backLine && !indented:
		return PickResult{Action: "back", Reopen: true}
	case !indented && (strings.HasPrefix(trim, forgetPrefix) || strings.HasPrefix(trim, stopPrefix)):
		i, j := strings.LastIndex(trim, "("), strings.LastIndex(trim, ")")
		if i < 0 || j != len(trim)-1 || j < i {
			return PickResult{Action: "ignored"}
		}
		r := h.Forget(trim[i+1 : j])
		return PickResult{Action: r.Action, Message: r.Message, Reopen: r.Action == "forgot"}
	case strings.HasPrefix(trim, "+ ") || strings.HasPrefix(trim, "- "):
		label := strings.TrimSpace(trim[2:])
		if i := strings.LastIndex(label, " ("); i >= 0 {
			label = label[:i]
		}
		h.mu.Lock()
		defer h.mu.Unlock()
		if label == DownLabel && len(h.downGroupLocked()) > 0 {
			open := true
			if v, ok := h.expanded["down"]; ok {
				open = v
			}
			h.expanded["down"] = !open
			h.notifyLocked()
			return PickResult{Action: "toggle", Reopen: true}
		}
		for _, g := range h.groupsLocked() {
			for _, gg := range append([]*group{g}, childGroups(g)...) {
				if gg.label == label {
					h.expanded[gg.key] = !h.isOpenLocked(gg.key, len(gg.members))
					h.notifyLocked()
					return PickResult{Action: "toggle", Reopen: true}
				}
			}
		}
		return PickResult{Action: "ignored"}
	case !indented:
		return PickResult{Action: "ignored"} // "! message" lines and anything unknown
	}
	fields := strings.Fields(trim)
	if fields[0] == MarkerOpen || fields[0] == MarkerClosed {
		fields = fields[1:] // the open marker is not part of the id
		if len(fields) == 0 {
			return PickResult{Action: "ignored"}
		}
	}
	id := fields[0]
	h.mu.Lock()
	_, ok := h.byID[id]
	h.mu.Unlock()
	if !ok {
		return PickResult{Action: "ignored"}
	}
	r := h.Open(id)
	return PickResult{Action: r.Action, Message: r.Message}
}

func childGroups(g *group) []*group {
	var out []*group
	for _, c := range g.children {
		out = append(out, c)
	}
	return out
}

var _ = inventory.Roles
