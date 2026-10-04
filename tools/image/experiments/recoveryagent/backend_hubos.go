package main

import (
	"context"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// HubosBackend is the real backend for the TEST recovery kernel: the work is done by hubos-ctl, the same program the
// recovery shell uses (so the checks are the same: update key keyring, floor, hashes, recovery kernel hash).
type HubosBackend struct {
	Ctl     string // path of hubos-ctl
	Release string // path of /etc/hubos-release
	LogFile string // the agent's own log
	mu      sync.Mutex
}

func NewHubosBackend() *HubosBackend {
	return &HubosBackend{Ctl: "/usr/sbin/hubos-ctl", Release: "/etc/hubos-release", LogFile: "/run/recovery-agent.log"}
}

func (b *HubosBackend) run(timeout time.Duration, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, b.Ctl, args...).CombinedOutput()
	return string(out), err
}

var failuresRe = regexp.MustCompile(`boot-failures=(\d+) limit=(\d+)`)

func (b *HubosBackend) Status() Status {
	host, _ := os.Hostname()
	st := Status{Machine: host, State: "recovery", BootFailures: -1, FailureLimit: -1}
	if raw, err := os.ReadFile(b.Release); err == nil {
		for _, l := range strings.Split(string(raw), "\n") {
			if v, ok := strings.CutPrefix(l, "version="); ok {
				st.Release = v
			}
			for _, f := range strings.Fields(l) { // the file may be one line "version=... flavor=..."
				if v, ok := strings.CutPrefix(f, "version="); ok {
					st.Release = v
				}
			}
		}
	}
	if out, err := b.run(20*time.Second, "status"); err == nil {
		if m := failuresRe.FindStringSubmatch(out); m != nil {
			st.BootFailures, _ = strconv.Atoi(m[1])
			st.FailureLimit, _ = strconv.Atoi(m[2])
		}
	}
	return st
}

func (b *HubosBackend) ClearFailures() error {
	_, err := b.run(30*time.Second, "clear-failures")
	return err
}

// Install runs `hubos-ctl update BASE SLOT`; one install at a time. hubos-ctl checks the bundle's own signature against the
// update keyring of this recovery kernel, so the agent cannot install anything the update key did not sign.
func (b *HubosBackend) Install(slot, baseURL string) (string, error) {
	if !b.mu.TryLock() {
		return "", errBusy
	}
	defer b.mu.Unlock()
	out, err := b.run(20*time.Minute, "update", baseURL, slot)
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) > 12 {
		lines = lines[len(lines)-12:]
	}
	return strings.Join(lines, "\n"), err
}

func (b *HubosBackend) Logs() string {
	raw, err := os.ReadFile(b.LogFile)
	if err != nil {
		return "no log: " + err.Error() + "\n"
	}
	if len(raw) > 4096 {
		raw = raw[len(raw)-4096:]
	}
	return string(raw)
}

type busyError struct{}

func (busyError) Error() string { return "an install is already running" }

var errBusy error = busyError{}
