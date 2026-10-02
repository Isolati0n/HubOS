package hub

import (
	"context"
	"strconv"
	"time"

	"hubos/internal/probe"
)

// ProbeRound checks every machine that can be checked, with at most
// Settings.ProbeCap checks in flight. Each result is applied as soon as it
// arrives, so a slow machine never holds up the others.
func (h *Hub) ProbeRound(ctx context.Context) {
	began := h.now()
	type target struct {
		s *mstate
		t probe.Target
	}
	var ts []target
	var targets []probe.Target
	for _, s := range h.ms {
		if s.m.Role == "hub" || s.m.Open[0] == "none" || s.m.Port == nil {
			continue
		}
		t := probe.Target{Address: s.m.Address, Port: *s.m.Port}
		ts = append(ts, target{s, t})
		targets = append(targets, t)
	}
	prober := h.set.Prober
	if prober == nil {
		prober = probe.CheckLimited
	}
	prober(ctx, targets, h.set.ProbeTimeout, h.set.ProbeCap, func(i int, r probe.Result) {
		s := ts[i].s
		if ctx.Err() != nil && !r.Up {
			return // the round was cancelled; learn nothing
		}
		h.mu.Lock()
		old := s.status
		switch {
		case r.Up:
			s.status, s.reason = statusUp, ""
		case r.Unchecked:
			s.status, s.reason = statusNoHandles, r.Reason
		default:
			s.status, s.reason = statusDown, r.Reason
		}
		s.checkedAt = h.now()
		h.lastResult = s.checkedAt
		if h.rounds == 0 {
			h.firstDone++
		}
		if s.status != old {
			h.notifyLocked()
		}
		h.mu.Unlock()
	})
	h.mu.Lock()
	h.rounds++
	h.lastRound = h.now()
	h.lastResult = h.lastRound
	h.lastTook = h.now().Sub(began)
	h.notifyLocked()
	c, _ := h.countLocked()
	h.mu.Unlock()
	if h.set.OnRound != nil {
		h.set.OnRound(h.lastTookCopy(began), c)
	}
}

// RunProbes runs rounds until ctx ends. A round starts one interval after
// the previous round finished.
func (h *Hub) RunProbes(ctx context.Context) {
	for {
		h.ProbeRound(ctx)
		select {
		case <-ctx.Done():
			return
		case <-time.After(h.set.ProbeInterval):
		}
	}
}

// RoundCount is how many full rounds have finished.
func (h *Hub) RoundCount() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.rounds
}

func itoa(n int) string { return strconv.Itoa(n) }

func (h *Hub) lastTookCopy(began time.Time) time.Duration { return h.now().Sub(began) }
