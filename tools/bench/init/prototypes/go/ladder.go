// Package main is a PROTOTYPE of the "policy brain" (bench only, never for the image): the escalation ladder, the
// health probe, a status socket and a crash record, in Go. It has two backends: s6 (the brain commands s6, architecture
// C) and direct (the brain starts and watches the services itself, as a normal service under a tiny PID 1,
// architecture B). The ladder never reboots anything: its last step is "stop retrying, mark degraded, raise an alert".
package main

import "time"

// Step is where a service is on its ladder.
type Step int

const (
	Running   Step = iota // restarts are immediate (the backend does them)
	BackedOff             // stopped on purpose, comes back after a growing pause
	DepsHit               // its dependents were restarted as well
	Degraded              // retrying has stopped; an alert is raised; only the owner can resume it
)

func (s Step) String() string {
	return [...]string{"running", "backoff", "restarted-dependents", "degraded"}[s]
}

// Policy is declared per service. The numbers are bench values; the real ones are the owner's to choose.
type Policy struct {
	Window       time.Duration // failures older than this are forgotten
	BackoffAfter int           // failures in the window before the first pause
	DepsAfter    int           // failures before the dependents are restarted too
	GiveUpAfter  int           // failures before retrying stops (degraded)
	Pause        time.Duration // first pause; doubles each further failure
}

type Ladder struct {
	P         Policy
	fails     []time.Time
	Step      Step
	Until     time.Time // end of the current pause
	Restarts  int
	LastFail  time.Time
	LastCause string
}

// Action tells the caller what to do now. The ladder itself does nothing.
type Action int

const (
	Nothing        Action = iota
	StopForBackoff        // stop the service; start it again at Until
	RestartDeps           // restart the dependents, then stop for backoff
	GiveUp                // stop for good, raise the alert
)

// Fail records one failure (process died, or probe failed) at time now and returns the next action.
func (l *Ladder) Fail(now time.Time, cause string) Action {
	l.Restarts++
	l.LastFail, l.LastCause = now, cause
	keep := l.fails[:0]
	for _, t := range l.fails {
		if now.Sub(t) < l.P.Window {
			keep = append(keep, t)
		}
	}
	l.fails = append(keep, now)
	n := len(l.fails)
	switch {
	case l.Step == Degraded:
		return Nothing
	case n >= l.P.GiveUpAfter:
		l.Step = Degraded
		return GiveUp
	case n >= l.P.DepsAfter:
		l.Step, l.Until = DepsHit, now.Add(l.pause(n))
		return RestartDeps
	case n >= l.P.BackoffAfter:
		l.Step, l.Until = BackedOff, now.Add(l.pause(n))
		return StopForBackoff
	}
	return Nothing
}

func (l *Ladder) pause(n int) time.Duration {
	d := l.P.Pause
	for i := l.P.BackoffAfter; i < n && d < time.Minute; i++ {
		d *= 2
	}
	return d
}

// Tick: true when a pause is over and the service should be started again.
func (l *Ladder) Tick(now time.Time) bool {
	if (l.Step == BackedOff || l.Step == DepsHit) && !now.Before(l.Until) {
		l.Step = Running
		return true
	}
	if l.Step == Running && len(l.fails) > 0 && now.Sub(l.fails[len(l.fails)-1]) > l.P.Window {
		l.fails = l.fails[:0] // quiet for a whole window: forgiven
	}
	return false
}

// Retry is the owner's action after a service was marked degraded.
func (l *Ladder) Retry() { l.fails, l.Step = nil, Running }
