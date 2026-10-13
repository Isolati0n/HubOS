package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// Proc is the identity of a process: its id AND its start time (field 22 of /proc/PID/stat, clock ticks since boot), so that a
// reused id is not mistaken for the same process. The zero value means "no process".
type Proc struct {
	PID   int
	Start uint64
}

func (p Proc) None() bool { return p.PID == 0 }
func (p Proc) String() string {
	if p.None() {
		return "none"
	}
	return fmt.Sprintf("pid=%d start=%d", p.PID, p.Start)
}

// procStart reads the start time of a process; ok is false when the process is gone or /proc cannot be parsed.
func procStart(pid int) (uint64, bool) {
	st, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return 0, false
	}
	str := string(st)
	i := strings.LastIndex(str, ")")
	if i < 0 {
		return 0, false
	}
	f := strings.Fields(str[i+2:])
	if len(f) < 20 {
		return 0, false
	}
	v, err := strconv.ParseUint(f[19], 10, 64)
	return v, err == nil
}

// findProcs: the processes named name, with their start times, in order of process id.
func findProcs(name string) []Proc {
	var out []Proc
	for _, p := range pidsNamed(name) {
		if st, ok := procStart(p); ok {
			out = append(out, Proc{p, st})
		}
	}
	return out
}

// Watcher is the whole decision of one round: the rule (4 rounds and 30 s), the identity rule (follow-up 5, decision 3), the
// budget (3 kills in 10 minutes), the latch (decision 1) and the state file (decision 2). Everything outside the process
// (finding the compositor, killing it, the log, the wall clock) is a field so that the tests can replace it.
type Watcher struct {
	R     *Rule
	B     *Budget
	Latch Latch
	State StateFile

	Find     func() []Proc
	Kill     func(Proc) error
	Evidence func(Proc) string
	Wall     func() time.Time
	Log      func(format string, a ...any)
	Target   string

	ident     Proc   // the compositor process at the first failed round of the streak
	lastDamge string // the last latch damage text logged
}

// Result is what Step did, for the log and the tests.
type Result int

const (
	ResIdle    Result = iota // nothing to do (answered, or the streak is still counting)
	ResKilled                // the failing process was killed
	ResCapped                // the rule wants a kill but the latch or the full budget blocks it (state file written)
	ResSkipped               // the rule wants a kill but the process is gone or another one: no kill, no count, a new streak
)

func (r Result) String() string { return [...]string{"idle", "killed", "capped", "skipped"}[r] }

func first(ps []Proc) Proc {
	if len(ps) == 0 {
		return Proc{}
	}
	return ps[0]
}

func has(ps []Proc, p Proc) bool {
	for _, q := range ps {
		if q == p {
			return true
		}
	}
	return false
}

// refresh: pick up what a person did to the files since the last round (`reset` empties the budget file and removes the latch
// and the state file while the watcher runs), and report damage of the latch file once.
func (w *Watcher) refresh(now time.Duration) (latched bool) {
	if w.B.Changed() {
		w.Log("budget file changed by someone else: %s", w.B.Reload(now))
	}
	latched, dmg := w.Latch.State()
	if dmg != w.lastDamge {
		if dmg != "" {
			w.Log("LATCH %s: treated as latched (the safe side), nothing is killed", dmg)
		}
		w.lastDamge = dmg
	}
	return latched
}

// Step handles one probe round at boot-clock time now. state and sync tell whether the two compositor probes answered.
func (w *Watcher) Step(now time.Duration, state, sync bool) Result {
	latched := w.refresh(now)
	if state && sync { // both answer: the streak ends, and the "capped" message goes away (the latch stays)
		w.R.Reset()
		w.ident = Proc{}
		if w.State.Exists() {
			if err := w.State.Clear(); err != nil {
				w.Log("state file not removed: %v", err)
			} else {
				w.Log("STATE both probes answer again: removed %s (latch %s)", w.State.Path, setWord(latched))
			}
		}
		return ResIdle
	}
	failed, want := w.R.Want(now, state, sync)
	if !failed { // exactly one probe answered: not a failed round, the streak is over; the message stays
		w.ident = Proc{}
		return ResIdle
	}
	if s, _ := w.R.Streak(); s == 1 { // first failed round of a streak: remember which process is failing
		w.ident = first(w.Find())
		w.Log("STREAK starts; failing process: %s", w.ident)
	}
	if !want {
		return ResIdle
	}
	cur := w.Find()
	if w.ident.None() || !has(cur, w.ident) {
		w.Log("SKIP: process changed (recorded %s, now %s): no kill, nothing counted, new streak with the new process", w.ident, first(cur))
		w.ident = first(cur)
		w.R.Restart(now)
		return ResSkipped
	}
	if !latched && w.B.Full(now) { // the rule wants a 4th kill: latch
		if err := w.Latch.Set(now, w.Wall()); err != nil {
			w.Log("LATCH not written: %v (the full budget still blocks the kill)", err)
		} else {
			w.Log("LATCH set: the rule wants a kill but %d kills were made in the last 10 minutes; stopped until a person clears it", w.B.Len())
		}
		latched = true
	}
	if latched || w.B.Full(now) {
		w.Log("CAP: would kill %s %s, but the watcher is latched; not killing", w.Target, w.ident)
		if !w.State.Exists() {
			if err := w.State.SetCapped(w.Wall()); err != nil {
				w.Log("state file not written: %v", err)
			} else {
				w.Log("STATE wrote %s (capped)", w.State.Path)
			}
		}
		return ResCapped
	}
	// kill: recorded before the signal; taken back if the signal finds no such process
	w.B.Add(now)
	if w.B.SaveErr != nil {
		w.Log("budget file not written: %v (the kill still counts in memory)", w.B.SaveErr)
	}
	if w.Evidence != nil {
		w.Log("EVIDENCE %s", w.Evidence(w.ident))
	}
	if err := w.Kill(w.ident); err != nil {
		w.B.Undo()
		w.Log("SKIP: kill of %s %s failed (%v): nothing counted, new streak", w.Target, w.ident, err)
		w.ident = Proc{}
		w.R.Restart(now)
		return ResSkipped
	}
	w.Log("BUDGET kill %d of %d in the last 10 minutes recorded in %s", w.B.Len(), w.B.Max, w.B.Path)
	w.Log("KILL %s %s", w.Target, w.ident)
	w.R.Reset()
	w.ident = Proc{}
	return ResKilled
}

func setWord(b bool) string {
	if b {
		return "still set"
	}
	return "not set"
}

// killProc sends SIGKILL to the one process whose identity is p, after checking once more that it is still that process. The
// check and the signal are not atomic (the id could be reused in between; pidfd would close that gap, not used in the test).
func killProc(p Proc) error {
	if p.PID <= 0 { // never kill(0) or a negative id: that signals a whole process group
		return fmt.Errorf("no process")
	}
	if st, ok := procStart(p.PID); !ok || st != p.Start {
		return syscall.ESRCH
	}
	return syscall.Kill(p.PID, syscall.SIGKILL)
}

// Reset is the person-command of the test (`watcher reset`): empty the budget file, remove the latch, remove the state file. Only
// the hook; the product's command is not designed. The order matters: the budget is emptied first, so a running watcher never
// sees "no latch and a full budget" (which would latch again at once).
func Reset(budgetPath, latchPath, statePath string) error {
	b := &Budget{Path: budgetPath, Max: 3, Per: 10 * time.Minute}
	if err := b.save(); err != nil {
		return fmt.Errorf("budget file: %v", err)
	}
	if err := (Latch{latchPath}).Clear(); err != nil {
		return fmt.Errorf("latch file: %v", err)
	}
	if err := (StateFile{statePath}).Clear(); err != nil {
		return fmt.Errorf("state file: %v", err)
	}
	return nil
}
