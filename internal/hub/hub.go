// Package hub holds what the running hubd knows: the inventory, the latest
// checks, the windows it started, and the rules for opening and ending them.
//
// It never renders anything and never touches a machine. It starts viewers
// (as argument lists, never through a shell) and asks driftwm to move, focus
// and close their windows. The truth about windows stays in driftwm; the
// record kept here is rebuilt from it (see adopt).
package hub

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"hubos/internal/driftwm"
	"hubos/internal/inventory"
	"hubos/internal/probe"
	"hubos/internal/viewers"
)

// Compositor is what hub needs from driftwm. *driftwm.Client satisfies it;
// tests use a fake.
type Compositor interface {
	State() (*driftwm.State, error)
	Move(id, x, y int) error
	Resize(id, w, h int) error
	Focus(id int) error
	Close(id int) error
	Identity() (string, error)
	Subscribe(ctx context.Context) (<-chan *driftwm.State, error)
	SetZoom(z float64) error
	SetCamera(x, y float64) error
	PeerPID() (int, error)
}

// Proc is a started viewer. Exited delivers once: nil for a clean exit.
type Proc struct {
	Exited <-chan error
}

// Launcher starts a viewer for a machine from an argument list.
type Launcher func(machineID string, args []string) (*Proc, error)

// Settings are the numbers hubd runs with. The defaults are proposals from
// measurement (docs/hubd-slice2.md), not decisions.
type Settings struct {
	ProbeCap        int                                                                                                                   // most checks in flight at once
	ProbeInterval   time.Duration                                                                                                         // time between the starts of two rounds
	ProbeTimeout    time.Duration                                                                                                         // limit for one machine's check (slice 1: 2 s)
	WindowWait      time.Duration                                                                                                         // how long to wait for a started viewer's window
	Settle          time.Duration                                                                                                         // extra wait after the first window, to catch a second
	LogDir          string                                                                                                                // where viewer logs go ("" = discard viewer output)
	LogMax          int64                                                                                                                 // most bytes in one viewer log before it is rotated
	LogTotalMax     int64                                                                                                                 // most bytes in all viewer logs together; the oldest go first
	CloseWait       time.Duration                                                                                                         // how long `end` waits for a window to go
	FoldThreshold   int                                                                                                                   // groups with more machines than this start folded
	ListMax         int                                                                                                                   // most machine lines in one menu list (wofi gets very slow far above 1000)
	LateGrace       time.Duration                                                                                                         // how long hubd keeps waiting for a window after the window wait ran out, with the viewer still running
	NoEscape        bool                                                                                                                  // do not escape markup in the status line (for a Waybar that escapes by itself)
	IgnoreAppIDs    []string                                                                                                              // windows with these app-ids are never candidates for comparison matching
	DownMax         int                                                                                                                   // most machine lines in the Down machines group
	TooltipCap      int                                                                                                                   // most down machines named in the tooltip
	MessageTTL      time.Duration                                                                                                         // how long a message stays on the bar item
	BarHeight       int                                                                                                                   // pixels the bar reserves; 0 = none or unknown
	FileLimit       uint64                                                                                                                // soft limit on open files (0 = unknown); caps ProbeCap
	LayoutDir       string                                                                                                                // where saved layouts live ("" = layouts are off), normally /config/hubos/layouts
	WaylandSocket   string                                                                                                                // the compositor's Wayland socket; a restore waits until it accepts connections ("" = do not wait)
	RestoreParallel int                                                                                                                   // most viewers a restore starts at the same time
	SettleStep      time.Duration                                                                                                         // pause between two looks when a window is being put at its place
	SettleMax       time.Duration                                                                                                         // longest a window is watched while it is put at its place
	SizedWait       time.Duration                                                                                                         // longest hubd waits for a new window's first picture before it moves it (a window is listed before it has drawn anything)
	FastReconnect   time.Duration                                                                                                         // how often hubd looks for a compositor that went away while windows were open
	KillWait        time.Duration                                                                                                         // restart-desktop: how long the compositor gets to exit after SIGTERM before SIGKILL
	Log             func(string)                                                                                                          // optional: one line for the log of hubd serve (restore results)
	Signal          func(pid int, sig syscall.Signal) error                                                                               // nil = the real kill
	ProcName        func(pid int) string                                                                                                  // nil = read /proc/PID/comm
	Prober          func(ctx context.Context, targets []probe.Target, timeout time.Duration, limit int, done func(i int, r probe.Result)) // nil = real checks; tests put a fake here
	OnRound         func(took time.Duration, c Counts)                                                                                    // called after each round (optional)
}

// DefaultSettings are the proposed values.
func DefaultSettings() Settings {
	return Settings{
		ProbeCap: 0, ProbeInterval: 10 * time.Second, ProbeTimeout: 2 * time.Second,
		WindowWait: 10 * time.Second, LateGrace: 60 * time.Second, Settle: 500 * time.Millisecond, CloseWait: 30 * time.Second,
		SettleStep: 60 * time.Millisecond, SettleMax: 30 * time.Second, RestoreParallel: 10, SizedWait: 60 * time.Second, FastReconnect: 100 * time.Millisecond, KillWait: 10 * time.Second,
		FoldThreshold: 12, ListMax: 1000, DownMax: 50, TooltipCap: 10, MessageTTL: 15 * time.Second, LogMax: 128 << 10, LogTotalMax: 64 << 20,
	}
}

type status int

const (
	statusChecking status = iota
	statusUp
	statusDown
	statusNotChecked // no port, or nothing to open
	statusNoHandles  // could not check: out of file handles
)

type phase int

const (
	phaseIdle phase = iota
	phaseStarting
	phaseUnmatched // a viewer was started but its window could not be told apart
	phaseLate      // the window wait ran out with the viewer still running; hubd keeps waiting for its window (grace period)
)

// winRec is one window hubd knows belongs to a machine.
type winRec struct {
	Machine string `json:"machine"`
	Window  int    `json:"window"`
	AppID   string `json:"app_id"`
	Title   string `json:"title"`
	By      string `json:"by"` // "name" or "comparison"
}

// openTicket belongs to one open. It is stale when the compositor it started under has gone away (h.epoch
// moved on) or when a newer open has taken the machine's place: a stale open records and moves nothing.
type openTicket struct{ epoch int }

type mstate struct {
	m         inventory.Machine
	status    status
	reason    string
	checkedAt time.Time
	phase     phase
	win       *winRec
	// While phase is phaseLate: closing lateCancel stops the wait; lateCmp
	// says the viewer is matched by comparison (not by name).
	lateCancel chan struct{}
	lateCmp    bool
	lateEnd    time.Time   // when the wait ends
	port       int         // the port the check uses: the machine's own, else default_ports[its first open entry] (0 = none)
	ticket     *openTicket // the open that is under way for this machine
	restoreAt  *Place      // set while the window is being restored after a compositor restart: where it goes (consumed when the window is recorded)
}

// Hub is the running state.
type Hub struct {
	inv      *inventory.Inventory
	vt       *viewers.Table
	comp     Compositor
	launch   Launcher
	set      Settings
	recPath  string
	launchMu sync.Mutex // one launch at a time, whole hub

	mu             sync.Mutex
	ms             []*mstate
	byID           map[string]*mstate
	expanded       map[string]bool // explicit fold choices by group key
	msg            string
	msgUntil       time.Time
	driftwmUp      bool
	rounds         int
	lastRound      time.Time
	subs           map[chan struct{}]struct{}
	now            func() time.Time
	lateComparison int                 // machines in the late-window state whose viewer is matched by comparison
	dups           map[string]int      // machine id -> how many windows carry its hubos- name (only when 2 or more)
	lastResult     time.Time           // when a check result last arrived (or the round last ended, or hubd started)
	firstDone      int                 // machines answered during the first round
	checkable      int                 // machines that can be checked at all
	lastTook       time.Duration       // how long the latest check round took
	titleOf        map[string]string   // machine id -> the exact window title its viewer's title_match gives (title-matched machines only)
	titleIDs       map[string][]string // the same, the other way round

	// layouts (layoutops.go) and direct restore (restore.go)
	lay         *layoutStore
	layMu       sync.Mutex // one layout command at a time
	active      *Layout    // the active layout (nil = none); guarded by h.mu
	layoutInfos []LayoutInfo
	warnings    []string         // problems found while loading layouts, for the log
	places      map[string]Place // the latest place of every machine window hubd knows (from driftwm's events)
	lastView    viewSnap         // the latest view
	restoreSet  map[string]Place // machines whose windows must come back after the compositor restarted
	restoreView viewSnap         // the view to put back with them
	stateNanos  atomic.Int64     // how long the latest answers of driftwm's "state" took (smoothed); hubd's loops wait longer when driftwm is slow
	epoch       int              // counts the compositors that went away; an open that started under an earlier one is dropped
	restoring   int              // windows still being brought back
	restoreRun  bool             // a restore is running
	restoreOf   int              // how many windows the running restore started with
}

// New builds a Hub. recPath is where the record file goes ("" = none).
func New(inv *inventory.Inventory, vt *viewers.Table, comp Compositor, launch Launcher, set Settings, recPath string) *Hub {
	h := &Hub{
		inv: inv, vt: vt, comp: comp, launch: launch, set: set, recPath: recPath,
		byID: map[string]*mstate{}, expanded: map[string]bool{},
		subs: map[chan struct{}]struct{}{}, now: time.Now,
		places: map[string]Place{}, restoreSet: map[string]Place{},
	}
	if set.LayoutDir != "" {
		h.lay = &layoutStore{dir: set.LayoutDir, now: time.Now}
	}
	for _, m := range inv.Machines {
		s := &mstate{m: m, status: statusChecking}
		switch {
		case m.Role == "hub":
			s.status = statusUp
		case m.Open[0] == "none":
			s.status, s.reason = statusNotChecked, "nothing to open"
		default:
			if p, ok := vt.CheckPort(m); ok {
				s.port = p
			} else {
				s.status, s.reason = statusNotChecked, viewers.NoPortReason(m)
			}
		}
		h.ms = append(h.ms, s)
		h.byID[m.ID] = s
	}
	// 0 means automatic. Either way: at most 80% of the open-file limit.
	if h.set.ProbeCap == 0 {
		h.set.ProbeCap = probe.AutoCap(len(inv.Machines))
	}
	h.set.ProbeCap = probe.SafeCap(h.set.ProbeCap, set.FileLimit)
	h.lastResult = h.now()
	for _, s := range h.ms {
		if s.port > 0 && s.m.Role != "hub" && s.m.Open[0] != "none" {
			h.checkable++
		}
		if s.m.Role != "hub" && s.m.Open[0] != "none" {
			if v := vt.For(s.m.Open[0]); v != nil {
				if title, ok, _ := v.MatchTitle(s.m); ok {
					if h.titleOf == nil {
						h.titleOf, h.titleIDs = map[string]string{}, map[string][]string{}
					}
					h.titleOf[s.m.ID] = title
					h.titleIDs[title] = append(h.titleIDs[title], s.m.ID)
				}
			}
		}
	}
	h.loadLayouts()
	return h
}

// state asks driftwm for its state and notes how long the answer took.
func (h *Hub) state() (*driftwm.State, error) {
	t0 := time.Now()
	st, err := h.comp.State()
	d := time.Since(t0).Nanoseconds()
	h.stateNanos.Store((h.stateNanos.Load()*3 + d) / 4)
	return st, err
}

// pause is how long a loop that polls driftwm waits before it asks again: the
// time it wanted, but at least twice the time driftwm needs to answer (at most
// 2 s), so that 20 loops at once do not bury a driftwm that is already slow.
func (h *Hub) pause(base time.Duration) time.Duration {
	if d := 2 * time.Duration(h.stateNanos.Load()); d > base {
		return min(d, 2*time.Second)
	}
	return base
}

// goneErr says the error means driftwm is not there (not just slow): nobody listens, or the socket is gone.
func goneErr(err error) bool {
	return errors.Is(err, syscall.ECONNREFUSED) || errors.Is(err, syscall.ENOENT) || errors.Is(err, io.EOF)
}

// logf writes one line to the log, if there is one.
func (h *Hub) logf(format string, args ...any) {
	if h.set.Log != nil {
		h.set.Log(fmt.Sprintf(format, args...))
	}
}

// ProbeCap is the cap actually used (after the file-limit guard).
func (h *Hub) ProbeCap() int { return h.set.ProbeCap }

// Changes returns a channel that gets a signal when anything shown on the
// bar or in the menu may have changed, and a function that stops it.
func (h *Hub) Changes() (<-chan struct{}, func()) {
	ch := make(chan struct{}, 1)
	h.mu.Lock()
	h.subs[ch] = struct{}{}
	h.mu.Unlock()
	return ch, func() {
		h.mu.Lock()
		delete(h.subs, ch)
		h.mu.Unlock()
	}
}

// notifyLocked wakes every listener. Caller holds h.mu.
func (h *Hub) notifyLocked() {
	for ch := range h.subs {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}

func (h *Hub) setMessageLocked(text string) {
	h.msg = text
	h.msgUntil = h.now().Add(h.set.MessageTTL)
	h.notifyLocked()
	time.AfterFunc(h.set.MessageTTL+50*time.Millisecond, func() {
		h.mu.Lock()
		h.notifyLocked()
		h.mu.Unlock()
	})
}

func (h *Hub) activeMessageLocked() string {
	if h.msg != "" && h.now().Before(h.msgUntil) {
		return h.msg
	}
	return ""
}

// saveLocked writes the record file. Caller holds h.mu.
func (h *Hub) saveLocked(identity string) {
	if h.recPath == "" {
		return
	}
	var wins []winRec
	for _, s := range h.ms {
		if s.win != nil {
			wins = append(wins, *s.win)
		}
	}
	writeRecord(h.recPath, record{Driftwm: identity, Windows: wins})
}
