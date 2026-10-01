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
	"os/exec"
	"sync"
	"syscall"
	"time"

	"hubos/internal/driftwm"
	"hubos/internal/inventory"
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
}

// Proc is a started viewer. Exited delivers once: nil for a clean exit.
type Proc struct {
	Exited <-chan error
}

// Launcher starts a viewer from an argument list.
type Launcher func(args []string) (*Proc, error)

// ExecLauncher starts the program directly (no shell), in its own process
// group, with no input and output, so it survives hubd and is never tied to
// hubd's terminal.
func ExecLauncher(args []string) (*Proc, error) {
	cmd := exec.Command(args[0], args[1:]...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Stdin, cmd.Stdout, cmd.Stderr = nil, nil, nil
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	ch := make(chan error, 1)
	go func() { ch <- cmd.Wait() }()
	return &Proc{Exited: ch}, nil
}

// Settings are the numbers hubd runs with. The defaults are proposals from
// measurement (docs/hubd-slice2.md), not decisions.
type Settings struct {
	ProbeCap      int                                // most checks in flight at once
	ProbeInterval time.Duration                      // time between the starts of two rounds
	ProbeTimeout  time.Duration                      // limit for one machine's check (slice 1: 2 s)
	WindowWait    time.Duration                      // how long to wait for a started viewer's window
	Settle        time.Duration                      // extra wait after the first window, to catch a second
	CloseWait     time.Duration                      // how long `end` waits for a window to go
	FoldThreshold int                                // groups with more machines than this start folded
	TooltipCap    int                                // most down machines named in the tooltip
	MessageTTL    time.Duration                      // how long a message stays on the bar item
	BarHeight     int                                // pixels the bar reserves; 0 = none or unknown
	FileLimit     uint64                             // soft limit on open files (0 = unknown); caps ProbeCap
	StaleAfter    time.Duration                      // 0 = derived from ProbeInterval
	OnRound       func(took time.Duration, c Counts) // called after each round (optional)
}

// DefaultSettings are the proposed values.
func DefaultSettings() Settings {
	return Settings{
		ProbeCap: 200, ProbeInterval: 10 * time.Second, ProbeTimeout: 2 * time.Second,
		WindowWait: 10 * time.Second, Settle: 500 * time.Millisecond, CloseWait: 3 * time.Second,
		FoldThreshold: 12, TooltipCap: 10, MessageTTL: 15 * time.Second,
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
)

// winRec is one window hubd knows belongs to a machine.
type winRec struct {
	Machine string `json:"machine"`
	Window  int    `json:"window"`
	AppID   string `json:"app_id"`
	Title   string `json:"title"`
	By      string `json:"by"` // "name" or "comparison"
}

type mstate struct {
	m         inventory.Machine
	status    status
	reason    string
	checkedAt time.Time
	phase     phase
	win       *winRec
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

	mu        sync.Mutex
	ms        []*mstate
	byID      map[string]*mstate
	expanded  map[string]bool // explicit fold choices by group key
	msg       string
	msgUntil  time.Time
	driftwmUp bool
	rounds    int
	lastRound time.Time
	subs      map[chan struct{}]struct{}
	now       func() time.Time
}

// New builds a Hub. recPath is where the record file goes ("" = none).
func New(inv *inventory.Inventory, vt *viewers.Table, comp Compositor, launch Launcher, set Settings, recPath string) *Hub {
	h := &Hub{
		inv: inv, vt: vt, comp: comp, launch: launch, set: set, recPath: recPath,
		byID: map[string]*mstate{}, expanded: map[string]bool{},
		subs: map[chan struct{}]struct{}{}, now: time.Now,
	}
	for _, m := range inv.Machines {
		s := &mstate{m: m, status: statusChecking}
		switch {
		case m.Role == "hub":
			s.status = statusUp
		case m.Open[0] == "none":
			s.status, s.reason = statusNotChecked, "nothing to open"
		case m.Port == nil:
			s.status, s.reason = statusNotChecked, "no port in the inventory"
		}
		h.ms = append(h.ms, s)
		h.byID[m.ID] = s
	}
	// Stay under the open-file limit: each check holds one socket.
	if set.FileLimit > 0 && uint64(h.set.ProbeCap) > set.FileLimit-64 {
		if set.FileLimit > 65 {
			h.set.ProbeCap = int(set.FileLimit - 64)
		} else {
			h.set.ProbeCap = 1
		}
	}
	return h
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
