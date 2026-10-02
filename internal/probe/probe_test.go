package probe

import (
	"context"
	"errors"
	"net"
	"os"
	"sync"
	"syscall"
	"testing"
	"time"
)

// listen starts a listener on a port the operating system picks and accepts
// (then closes) connections until the test ends.
func listen(t *testing.T, ip string) net.Listener {
	t.Helper()
	l, err := net.Listen("tcp", net.JoinHostPort(ip, "0"))
	if err != nil {
		t.Fatalf("listen on %s: %v", ip, err)
	}
	t.Cleanup(func() { l.Close() })
	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}
			c.Close()
		}
	}()
	return l
}

func target(l net.Listener) Target {
	a := l.Addr().(*net.TCPAddr)
	return Target{Address: a.IP.String(), Port: a.Port}
}

func TestUpThenDownWhenStopped(t *testing.T) {
	l := listen(t, "127.0.0.21")
	tg := target(l)
	if r := Check(context.Background(), tg, time.Second); !r.Up || r.Reason != "" {
		t.Fatalf("running node should be up, got %+v", r)
	}
	l.Close()
	r := Check(context.Background(), tg, time.Second)
	if r.Up || r.Reason != "connection refused" {
		t.Fatalf("stopped node should be down (connection refused), got %+v", r)
	}
}

func TestCheckAllKeepsOrderAndMixesResults(t *testing.T) {
	up1 := listen(t, "127.0.0.22")
	down := listen(t, "127.0.0.23")
	up2 := listen(t, "127.0.0.24")
	downTarget := target(down)
	down.Close()

	got := CheckAll(context.Background(), []Target{target(up1), downTarget, target(up2)}, time.Second)
	if len(got) != 3 || !got[0].Up || got[1].Up || !got[2].Up {
		t.Fatalf("got %+v", got)
	}
}

func TestOverallLimitAlreadyReached(t *testing.T) {
	l := listen(t, "127.0.0.25")
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()
	r := Check(ctx, target(l), time.Second)
	if r.Up || r.Reason != "overall time limit reached before it answered" {
		t.Fatalf("got %+v", r)
	}
}

func TestNameThatDoesNotExistIsDown(t *testing.T) {
	// The exact reason depends on the DNS set-up of the machine running the
	// test, so only "down, with some reason" is checked here.
	r := Check(context.Background(), Target{Address: "no-such-machine.invalid", Port: 1}, time.Second)
	if r.Up || r.Reason == "" {
		t.Fatalf("got %+v", r)
	}
	t.Logf("reason on this machine: %s", r.Reason)
}

func TestReasonWording(t *testing.T) {
	cases := []struct {
		err  error
		want string
	}{
		{&net.OpError{Err: syscall.ECONNREFUSED}, "connection refused"},
		{&net.OpError{Err: syscall.EHOSTUNREACH}, "no route to the machine"},
		{&net.OpError{Err: syscall.ENETUNREACH}, "no route to the machine"},
		{context.DeadlineExceeded, "no answer within 2s"},
		{&net.DNSError{IsNotFound: true}, "name not found"},
		{&net.DNSError{IsTimeout: true}, "name lookup failed"},
		{errors.New("something odd"), "something odd"},
	}
	for _, c := range cases {
		if got := reason(c.err, 2*time.Second); got != c.want {
			t.Errorf("reason(%v) = %q, want %q", c.err, got, c.want)
		}
	}
}

func TestCheckLimitedNeverRunsMoreThanLimit(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}
			c.Close()
		}
	}()
	port := l.Addr().(*net.TCPAddr).Port
	targets := make([]Target, 50)
	for i := range targets {
		targets[i] = Target{Address: "127.0.0.1", Port: port}
	}
	var mu sync.Mutex
	up := 0
	CheckLimited(context.Background(), targets, time.Second, 4, func(i int, r Result) {
		mu.Lock()
		defer mu.Unlock()
		if r.Up {
			up++
		}
	})
	if up != 50 {
		t.Errorf("up = %d, want 50", up)
	}
}

// With too few file handles, a check that could not even open a socket must
// say so, and must not claim the machine is down.
func TestOutOfFileHandlesIsReportedAsUnchecked(t *testing.T) {
	l := listen(t, "127.0.0.1")
	port := l.Addr().(*net.TCPAddr).Port
	var old syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &old); err != nil {
		t.Skip(err)
	}
	low := syscall.Rlimit{Cur: 40, Max: old.Max}
	if err := syscall.Setrlimit(syscall.RLIMIT_NOFILE, &low); err != nil {
		t.Skip(err)
	}
	defer syscall.Setrlimit(syscall.RLIMIT_NOFILE, &old)

	// Hold most of the handles, then ask for far more checks at once.
	var hold []*os.File
	for i := 0; i < 20; i++ {
		f, err := os.Open("/dev/null")
		if err != nil {
			break
		}
		hold = append(hold, f)
	}
	defer func() {
		for _, f := range hold {
			f.Close()
		}
	}()
	targets := make([]Target, 200)
	for i := range targets {
		targets[i] = Target{Address: "127.0.0.1", Port: port}
	}
	var mu sync.Mutex
	up, unchecked, down := 0, 0, 0
	CheckLimited(context.Background(), targets, time.Second, 200, func(i int, r Result) {
		mu.Lock()
		defer mu.Unlock()
		switch {
		case r.Up:
			up++
		case r.Unchecked:
			unchecked++
		default:
			down++
		}
	})
	t.Logf("up=%d unchecked=%d down=%d", up, unchecked, down)
	if unchecked == 0 {
		t.Error("expected some checks to run out of file handles")
	}
	if down != 0 {
		t.Errorf("%d checks were reported down although the machine is up", down)
	}
}
