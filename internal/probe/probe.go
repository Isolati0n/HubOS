// Package probe decides whether a machine is up. It knows nothing about the
// inventory: it is given an address and a port and tries one TCP connection.
//
// "Up" here means only that something accepted the connection. It does not
// prove a desktop is logged in or that a session server is healthy. That is
// the simplest honest check; anything deeper is unverified (see HUB-OS.md).
package probe

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"sync"
	"syscall"
	"time"
)

// Target is where to knock.
type Target struct {
	Address string
	Port    int
}

// Result is the outcome of one check. Reason is empty when Up.
type Result struct {
	Up     bool
	Reason string
}

// Check opens one TCP connection to the target and closes it again without
// sending anything. timeout limits this one check; ctx may end it sooner.
// There are no retries.
func Check(ctx context.Context, t Target, timeout time.Duration) Result {
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var d net.Dialer
	conn, err := d.DialContext(cctx, "tcp", net.JoinHostPort(t.Address, strconv.Itoa(t.Port)))
	if err != nil {
		if ctx.Err() != nil {
			return Result{Reason: "overall time limit reached before it answered"}
		}
		return Result{Reason: reason(err, timeout)}
	}
	_ = conn.Close()
	return Result{Up: true}
}

// CheckAll checks every target at the same time and returns the results in
// the same order. ctx carries the overall time limit.
func CheckAll(ctx context.Context, targets []Target, timeout time.Duration) []Result {
	results := make([]Result, len(targets))
	var wg sync.WaitGroup
	for i, t := range targets {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results[i] = Check(ctx, t, timeout)
		}()
	}
	wg.Wait()
	return results
}

// reason turns a connection error into a short plain-words reason.
func reason(err error, timeout time.Duration) string {
	var dns *net.DNSError
	var netErr net.Error
	switch {
	case errors.Is(err, syscall.ECONNREFUSED):
		return "connection refused"
	case errors.Is(err, syscall.ENETUNREACH), errors.Is(err, syscall.EHOSTUNREACH):
		return "no route to the machine"
	case errors.As(err, &dns) && dns.IsNotFound:
		return "name not found"
	case errors.As(err, &dns):
		return "name lookup failed"
	case errors.Is(err, context.DeadlineExceeded), errors.As(err, &netErr) && netErr.Timeout():
		return fmt.Sprintf("no answer within %s", timeout)
	}
	return err.Error()
}
