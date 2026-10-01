package probe

import (
	"context"
	"errors"
	"net"
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
