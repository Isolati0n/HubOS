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
	// Unchecked is true when the check could not be made at all because hubd
	// ran out of file handles. That says nothing about the machine, so the
	// panel must not show it as down.
	Unchecked bool
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
		if errors.Is(err, syscall.EMFILE) || errors.Is(err, syscall.ENFILE) {
			// The reason text stays what slice 1 always printed; only the
			// new flag tells the daemon this says nothing about the machine.
			return Result{Reason: reason(err, timeout), Unchecked: true}
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

// SafeCap is how many checks may be in flight at once: the wanted number,
// but never more than 80% of the open-file limit (the rest is kept for other
// sockets and files). fileLimit 0 means unknown, and the wanted number is used.
func SafeCap(want int, fileLimit uint64) int {
	if want < 1 {
		want = 1
	}
	if fileLimit == 0 {
		return want
	}
	most := int(fileLimit * 8 / 10)
	if most < 1 {
		most = 1
	}
	if want > most {
		return most
	}
	return want
}

// AutoCap is the default number of checks in flight: 200, or 1000 when there
// are more than 1000 machines. (Proposed from the measurements in
// docs/hubd-slice2.md; always cut down by SafeCap.)
func AutoCap(machines int) int {
	if machines > 1000 {
		return 1000
	}
	return 200
}

// FileLimit is the soft limit on open files (0 if it cannot be read).
func FileLimit() uint64 {
	var rl syscall.Rlimit
	if syscall.Getrlimit(syscall.RLIMIT_NOFILE, &rl) != nil {
		return 0
	}
	return rl.Cur
}

// CheckLimited checks the targets with at most limit checks in flight at
// once. For each finished check it calls done(index, result) (from several
// goroutines at once; done must be safe for that). It returns when all are
// finished or ctx has ended; targets not reached before ctx ended get the
// "overall time limit" result.
func CheckLimited(ctx context.Context, targets []Target, timeout time.Duration, limit int, done func(i int, r Result)) {
	if limit < 1 {
		limit = 1
	}
	next := make(chan int)
	var wg sync.WaitGroup
	for w := 0; w < limit && w < len(targets); w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range next {
				done(i, Check(ctx, targets[i], timeout))
			}
		}()
	}
	for i := range targets {
		next <- i
	}
	close(next)
	wg.Wait()
}
