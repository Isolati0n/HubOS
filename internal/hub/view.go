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

// JSON is the one-line form.
func (s StatusLine) JSON() string {
	b, _ := json.Marshal(s)
	return string(b)
}

// Counts is the numbers behind the status line.
type Counts struct {
	Up, Down, NotChecked, Checking int
	Stale                          bool
}

func (h *Hub) staleAfterLocked() time.Duration {
	if h.set.StaleAfter > 0 {
		return h.set.StaleAfter
	}
	return 3*h.set.ProbeInterval + h.set.ProbeTimeout
}

func (h *Hub) countLocked() (c Counts, down []*mstate) {
	var oldest time.Time
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
		if !s.checkedAt.IsZero() && (oldest.IsZero() || s.checkedAt.Before(oldest)) {
			oldest = s.checkedAt
		}
	}
	// Stale: the oldest answer is older than the rounds should allow. A
	// round that is itself slow counts too, because answers arrive during it.
	if !oldest.IsZero() && h.now().Sub(oldest) > h.staleAfterLocked() {
		c.Stale = true
	}
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
	if !h.driftwmUp {
		tip = append(tip, "driftwm is not reachable: windows cannot be opened")
	}
	if c.Stale {
		tip = append(tip, "status is stale: checks are not finishing in time")
	}
	line := StatusLine{Class: "ok"}
	if h.rounds == 0 && c.Up == 0 && c.Down == 0 {
		line.Text = "checking..."
	} else {
		line.Text = fmt.Sprintf("%d of %d up", c.Up, c.Up+c.Down)
	}
	if c.Down > 0 || c.Stale || h.activeMessageLocked() != "" {
		line.Class = "alert"
	}
	if c.Down > 0 {
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
//	"? search ..."     switch to the flat list of every machine
//	"< back ..."       switch back to the groups
//	"- Label (...)"    an open group heading; picking it folds the group
//	"+ Label (...)"    a folded group heading; picking it opens the group
//	"   id  name  S"   a machine; the first word is the machine id
const (
	searchLine = "? search all machines..."
	backLine   = "< back to groups"
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
	if s.win != nil {
		t += " [open]"
	}
	if s.phase == phaseStarting {
		t += " [opening]"
	}
	if s.phase == phaseUnmatched {
		t += " [window not identified]"
	}
	return t
}

func (h *Hub) entryLocked(s *mstate, indent int) string {
	return fmt.Sprintf("%s%-16s %-24s %s", strings.Repeat(" ", indent), s.m.ID, s.m.Name, h.statusTextLocked(s))
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

// List returns the menu lines. flat lists every machine in one run, down
// first, so typing in the launcher searches all of them.
func (h *Hub) List(flat bool) []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	var lines []string
	if m := h.activeMessageLocked(); m != "" {
		lines = append(lines, "! "+m)
	}
	if flat {
		lines = append(lines, backLine)
		all := append([]*mstate(nil), h.ms...)
		sortMembers(all)
		for _, s := range all {
			lines = append(lines, h.entryLocked(s, 3))
		}
		return lines
	}
	lines = append(lines, searchLine)
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
		for _, s := range g.members {
			lines = append(lines, h.entryLocked(s, 3))
			if c := g.children[s.m.ID]; c != nil {
				copen := h.isOpenLocked(c.key, len(c.members))
				cmark := "+"
				if copen {
					cmark = "-"
				}
				lines = append(lines, strings.Repeat(" ", c.indent)+cmark+" "+headingText(c.label, c.members))
				if copen {
					for _, gs := range c.members {
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
}

// Pick handles one line chosen in the launcher. It acts only on a line that
// is exactly a machine line (indented, first word an id that exists), a
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
		return PickResult{Action: "search", Reopen: true, Flat: true}
	case trim == backLine && !indented:
		return PickResult{Action: "back", Reopen: true}
	case strings.HasPrefix(trim, "+ ") || strings.HasPrefix(trim, "- "):
		label := strings.TrimSpace(trim[2:])
		if i := strings.LastIndex(label, " ("); i >= 0 {
			label = label[:i]
		}
		h.mu.Lock()
		defer h.mu.Unlock()
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
	id := strings.Fields(trim)[0]
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
