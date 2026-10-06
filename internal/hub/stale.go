package hub

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

// The last known state, for the time hubd is down.
//
// While it runs, hubd writes the bar's status line and the menu's list to one
// small file in its runtime folder (memory-backed, gone at reboot): at every
// change, at most once a second, and at least every 5 seconds, so the time in
// the file is never far behind. When hubd is not running, `hubd feed` (the
// bar) and `hubd menu` read that file and show what hubd last said, marked
// STALE, instead of an empty "hubd stopped". The marker is the word STALE in
// front of the bar text (the same word hubd already uses when its check
// results are old), the red alert colour, a first tooltip or menu line that
// says how old it is, and a menu whose lines do nothing when picked.

// Snapshot is the file.
type Snapshot struct {
	Time time.Time  `json:"time"`
	Line StatusLine `json:"line"` // exactly the line the feed sent (already escaped for markup)
	List []string   `json:"list"` // the menu's lines (the grouped list)
}

// SnapshotPath is the snapshot file next to hubd's socket.
func SnapshotPath(socket string) string { return strings.TrimSuffix(socket, ".sock") + ".last.json" }

// RunSnapshot keeps the snapshot file up to date until ctx ends.
func (h *Hub) RunSnapshot(ctx context.Context, path string) {
	ch, stop := h.Changes()
	defer stop()
	tick := time.NewTicker(5 * time.Second)
	defer tick.Stop()
	for {
		h.WriteSnapshot(path)
		select {
		case <-ctx.Done():
			return
		case <-ch:
		case <-tick.C:
		}
		select { // at most one write a second
		case <-ctx.Done():
			return
		case <-time.After(time.Second):
		}
	}
}

// WriteSnapshot writes the file once. The write is a temporary file renamed
// over the old one, so a reader never sees half a file; a failure is ignored
// (the file is only a help).
func (h *Hub) WriteSnapshot(path string) {
	if path == "" {
		return
	}
	snap := Snapshot{Time: h.now(), List: h.List(false, "")}
	st := h.Status()
	if !h.set.NoEscape {
		st.Text, st.Tooltip = markupEscaper.Replace(st.Text), markupEscaper.Replace(st.Tooltip)
	}
	snap.Line = st
	b, err := json.Marshal(snap)
	if err != nil {
		return
	}
	tmp := path + ".tmp"
	if os.WriteFile(tmp, b, 0o600) == nil {
		os.Rename(tmp, path)
	}
}

// ReadSnapshot reads the file.
func ReadSnapshot(path string) (Snapshot, error) {
	var s Snapshot
	b, err := os.ReadFile(path)
	if err != nil {
		return s, err
	}
	if err := json.Unmarshal(b, &s); err != nil {
		return Snapshot{}, err
	}
	if s.Time.IsZero() || s.Line.Text == "" {
		return Snapshot{}, fmt.Errorf("the last-state file is empty")
	}
	return s, nil
}

// ago says how old something is in plain words; seconds are rounded down to
// 10 s steps so a tooltip that shows it changes at most every 10 s.
func ago(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%d s", int(d.Truncate(10*time.Second)/time.Second))
	case d < time.Hour:
		return fmt.Sprintf("%d min", int(d/time.Minute))
	}
	return fmt.Sprintf("%d h %d min", int(d/time.Hour), int(d%time.Hour/time.Minute))
}

// StaleLine is the bar line for the time hubd is down: the last text with
// STALE in front, the alert colour, and a first tooltip line that says so.
func (s Snapshot) StaleLine(now time.Time) StatusLine {
	l := s.Line
	if !strings.HasPrefix(l.Text, "STALE: ") {
		l.Text = "STALE: " + l.Text
	}
	l.Class = "alert"
	head := fmt.Sprintf("hubd is not running: this is the last known state, from %s UTC (%s ago), not the current one", s.Time.UTC().Format("15:04:05"), ago(now.Sub(s.Time)))
	l.Tooltip = head + "\n" + l.Tooltip
	return l
}

// StaleList is the menu for the time hubd is down: the last list under a
// first line that says it is STALE. Picking a line does nothing.
func (s Snapshot) StaleList(now time.Time) []string {
	head := fmt.Sprintf("! STALE: hubd is not running; this list is %s old; picking does nothing", ago(now.Sub(s.Time)))
	return append([]string{head}, s.List...)
}
