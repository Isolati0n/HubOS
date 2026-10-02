package hub

import (
	"context"
	"fmt"
	"strings"
	"time"

	"hubos/internal/driftwm"
	"hubos/internal/viewers"
)

// RunWatch follows driftwm until ctx ends: it adopts windows when it
// connects, forgets windows that close, and clears the record when driftwm
// goes away (every window id is meaningless after that).
func (h *Hub) RunWatch(ctx context.Context) {
	backoff := 500 * time.Millisecond
	for ctx.Err() == nil {
		ch, err := h.comp.Subscribe(ctx)
		if err != nil {
			h.setDriftwmUp(false)
			select {
			case <-ctx.Done():
				return
			case <-time.After(backoff):
			}
			if backoff < 4*time.Second {
				backoff *= 2
			}
			continue
		}
		backoff = 500 * time.Millisecond
		if st, err := h.comp.State(); err == nil {
			h.adopt(st)
		}
		h.setDriftwmUp(true)
		for st := range ch {
			h.syncWindows(st)
		}
		// The stream ended: driftwm quit or restarted.
		h.setDriftwmUp(false)
		h.clearAll()
	}
}

func (h *Hub) setDriftwmUp(up bool) {
	h.mu.Lock()
	if h.driftwmUp != up {
		h.driftwmUp = up
		h.notifyLocked()
	}
	h.mu.Unlock()
}

// clearAll forgets every window. Used when driftwm has gone away.
func (h *Hub) clearAll() {
	h.mu.Lock()
	for _, s := range h.ms {
		s.win = nil
		h.leaveLateLocked(s, phaseIdle)
		if s.phase == phaseUnmatched {
			s.phase = phaseIdle
		}
	}
	if h.recPath != "" {
		writeRecord(h.recPath, record{})
	}
	h.notifyLocked()
	h.mu.Unlock()
}

// syncWindows forgets windows that are gone or have been replaced by another
// program under the same id.
func (h *Hub) syncWindows(st *driftwm.State) {
	h.mu.Lock()
	defer h.mu.Unlock()
	changed := false
	for _, s := range h.ms {
		if s.win == nil {
			continue
		}
		cur, ok := st.Window(s.win.Window)
		if !ok || cur.AppID != s.win.AppID {
			s.win = nil
			changed = true
		}
	}
	// Two or more windows with the same hubos- name: tell the owner.
	counts := map[string]int{}
	for _, w := range st.Windows {
		if id, ok := strings.CutPrefix(w.AppID, viewers.AppIDPrefix); ok && h.byID[id] != nil {
			counts[id]++
		}
	}
	dups := map[string]int{}
	for id, n := range counts {
		if n >= 2 {
			dups[id] = n
		}
	}
	if !sameCounts(dups, h.dups) {
		h.dups = dups
		changed = true
	}
	if changed {
		h.saveLocked(h.identityLocked())
		h.notifyLocked()
	}
}

func sameCounts(a, b map[string]int) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

// dupWarningsLocked has one plain sentence for each machine that has two or
// more windows carrying its hubos- name, in machine order.
func (h *Hub) dupWarningsLocked() []string {
	var out []string
	for _, s := range h.ms {
		n := h.dups[s.m.ID]
		if n < 2 {
			continue
		}
		known := "hubd knows none of them"
		if s.win != nil {
			known = fmt.Sprintf("hubd knows window #%d", s.win.Window)
		}
		out = append(out, fmt.Sprintf("WARNING: %d windows are named %s (%s); %s; close the extra one by hand", n, viewers.AppID(s.m.ID), s.m.Name, known))
	}
	return out
}

// adopt picks up windows hubd started before it was restarted. It opens
// nothing. Two sources, both checked against driftwm's own list:
//   - the record file, if it belongs to this same driftwm instance;
//   - windows whose name is hubos-<machine id>, if exactly one has that name.
func (h *Hub) adopt(st *driftwm.State) {
	identity := h.identityLocked()
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.recPath != "" {
		if r, ok := readRecord(h.recPath); ok && r.Driftwm == identity && identity != "" {
			for _, rec := range r.Windows {
				s := h.byID[rec.Machine]
				cur, found := st.Window(rec.Window)
				if s != nil && s.win == nil && found && cur.AppID == rec.AppID && cur.Title == rec.Title {
					rec := rec
					s.win = &rec
				}
			}
		}
	}
	byName := map[string][]driftwm.Window{}
	for _, w := range st.Windows {
		if id, ok := strings.CutPrefix(w.AppID, viewers.AppIDPrefix); ok {
			byName[id] = append(byName[id], w)
		}
	}
	for id, ws := range byName {
		s := h.byID[id]
		if s == nil || s.win != nil || len(ws) != 1 {
			continue // unknown machine, already known, or two windows: not guessed
		}
		s.win = &winRec{Machine: id, Window: ws[0].ID, AppID: ws[0].AppID, Title: ws[0].Title, By: "name"}
	}
	h.saveLocked(identity)
	h.notifyLocked()
}

// WindowOf reports the window id hubd holds for a machine, for tests.
func (h *Hub) WindowOf(id string) (int, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if s := h.byID[id]; s != nil && s.win != nil {
		return s.win.Window, true
	}
	return 0, false
}

// Adopt is adopt for tests and for hubd serve at start-up.
func (h *Hub) Adopt() error {
	st, err := h.comp.State()
	if err != nil {
		return err
	}
	h.adopt(st)
	return nil
}
