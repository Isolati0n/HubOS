package hub

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func shortDir(t *testing.T) string {
	t.Helper()
	d, err := os.MkdirTemp("", "hs")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(d) })
	return d
}

func TestSocketServesAllCommands(t *testing.T) {
	r := newRig(t, machineDoc("a", "A", "ai", "moonlight", 1, 1, 1, ""))
	r.setStatus("a", statusUp)
	sock, err := SocketPath(shortDir(t))
	if err != nil {
		t.Fatal(err)
	}
	l, err := Listen(sock)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	go r.h.Serve(l)

	if fi, _ := os.Stat(sock); fi.Mode().Perm() != 0o600 {
		t.Errorf("socket mode %v", fi.Mode().Perm())
	}
	if fi, _ := os.Stat(filepath.Dir(sock)); fi.Mode().Perm() != 0o700 {
		t.Errorf("directory mode %v", fi.Mode().Perm())
	}
	st, err := Call(sock, Request{Cmd: "status"})
	if err != nil || st.Status.Text != "1 of 1 up" {
		t.Errorf("status: %+v %v", st, err)
	}
	ls, _ := Call(sock, Request{Cmd: "list"})
	if len(ls.Lines) < 3 || !strings.Contains(strings.Join(ls.Lines, "\n"), "   a ") {
		t.Errorf("list: %q", ls.Lines)
	}
	op, _ := Call(sock, Request{Cmd: "pick", Line: "   a                A                        UP"})
	if op.Action != "open" || !strings.Contains(op.Message, "opened A") {
		t.Errorf("pick: %+v", op)
	}
	if e, _ := Call(sock, Request{Cmd: "end", ID: "a"}); !e.OK {
		t.Errorf("end: %+v", e)
	}
	if _, err := Call(sock, Request{Cmd: "nonsense"}); err == nil {
		t.Error("an unknown command must be an error")
	}
	// A second hubd on the same socket is refused.
	if _, err := Listen(sock); err == nil || !strings.Contains(err.Error(), "already running") {
		t.Errorf("second listen: %v", err)
	}
}

func TestFeedSendsOnlyWhenTheLineChanges(t *testing.T) {
	r := newRig(t, machineDoc("a", "A", "ai", "moonlight", 1, 1, 1, ""))
	r.setStatus("a", statusUp)
	sock, _ := SocketPath(shortDir(t))
	l, _ := Listen(sock)
	defer l.Close()
	go r.h.Serve(l)

	lines := make(chan string, 10)
	go Feed(sock, func(s string) { lines <- s })
	get := func() string {
		select {
		case s := <-lines:
			return s
		case <-time.After(2 * time.Second):
			t.Fatal("no line")
			return ""
		}
	}
	if first := get(); !strings.Contains(first, `"1 of 1 up"`) || !strings.Contains(first, `"class":"ok"`) {
		t.Errorf("first: %s", first)
	}
	// A change that does not change the line sends nothing...
	r.h.mu.Lock()
	r.h.notifyLocked()
	r.h.mu.Unlock()
	select {
	case s := <-lines:
		t.Errorf("unexpected line %s", s)
	case <-time.After(400 * time.Millisecond):
	}
	// ...and one that does, sends one.
	r.setStatus("a", statusDown)
	r.h.mu.Lock()
	r.h.notifyLocked()
	r.h.mu.Unlock()
	if next := get(); !strings.Contains(next, `"0 of 1 up"`) || !strings.Contains(next, `"class":"alert"`) {
		t.Errorf("after stop: %s", next)
	}
}

func TestLongRuntimeDirIsRefusedPlainly(t *testing.T) {
	long := "/tmp/" + strings.Repeat("a-very-long-directory-name/", 5)
	_, err := SocketPath(long)
	if err == nil || !strings.Contains(err.Error(), "Linux allows at most 107") || !strings.Contains(err.Error(), "shorter XDG_RUNTIME_DIR") {
		t.Fatalf("got %v", err)
	}
	if _, err := SocketPath(""); err == nil || !strings.Contains(err.Error(), "XDG_RUNTIME_DIR is not set") {
		t.Errorf("empty: %v", err)
	}
	// 107 bytes exactly is accepted, 108 is not.
	pad := func(n int) string { return "/tmp/" + strings.Repeat("x", n-len("/tmp/")-len("/hubos/hubd.sock")) }
	if _, err := SocketPath(pad(107)); err != nil {
		t.Errorf("107: %v", err)
	}
	if _, err := SocketPath(pad(108)); err == nil {
		t.Error("108 should fail")
	}
}

func TestStaleSocketFileIsReplaced(t *testing.T) {
	sock, _ := SocketPath(shortDir(t))
	os.MkdirAll(filepath.Dir(sock), 0o700)
	os.WriteFile(sock, []byte("left over"), 0o600)
	l, err := Listen(sock)
	if err != nil {
		t.Fatal(err)
	}
	l.Close()
}

// STALE comes from the clock, not from an event: the feed must notice it.
func TestFeedShowsStaleWhenTimePassesWithNoEvent(t *testing.T) {
	r := newRig(t, machineDoc("a", "A", "ai", "moonlight", 1, 1, 1, ""))
	r.setStatus("a", statusUp)
	r.h.set.ProbeInterval = 200 * time.Millisecond // stale after 600 ms
	r.h.mu.Lock()
	r.h.rounds, r.h.lastResult, r.h.lastTook = 1, time.Now(), 5*time.Millisecond
	r.h.mu.Unlock()
	sock, _ := SocketPath(shortDir(t))
	l, _ := Listen(sock)
	defer l.Close()
	go r.h.Serve(l)
	lines := make(chan string, 10)
	go Feed(sock, func(s string) { lines <- s })
	first := <-lines
	if strings.Contains(first, "STALE") {
		t.Fatalf("stale at once: %s", first)
	}
	select {
	case next := <-lines:
		if !strings.Contains(next, `"text":"STALE: 1 of 1 up"`) || !strings.Contains(next, `"class":"alert"`) {
			t.Errorf("got %s", next)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("no STALE line although no round finished")
	}
}

// During the first round hundreds of results arrive each second; the bar
// must still get at most one line per second.
func TestFeedSendsAtMostOneLinePerSecondDuringTheFirstRound(t *testing.T) {
	var parts []string
	for i := 1; i <= 50; i++ {
		parts = append(parts, machineDoc(fmt.Sprintf("m%02d", i), "M", "desktop", "moonlight", 21000, i, 7, ""))
	}
	r := newRig(t, parts...)
	sock, _ := SocketPath(shortDir(t))
	l, _ := Listen(sock)
	defer l.Close()
	go r.h.Serve(l)
	var mu sync.Mutex
	var got []string
	go Feed(sock, func(s string) { mu.Lock(); got = append(got, s); mu.Unlock() })
	deadline := time.Now().Add(2500 * time.Millisecond)
	for i := 0; time.Now().Before(deadline); i++ {
		r.h.mu.Lock()
		r.h.firstDone = i % 50
		r.h.notifyLocked()
		r.h.mu.Unlock()
		time.Sleep(10 * time.Millisecond) // 100 changes a second
	}
	mu.Lock()
	defer mu.Unlock()
	if len(got) < 2 || len(got) > 4 { // at 0 s, then at most one per second: 0, 1, 2 s
		t.Errorf("%d lines in 2.5 s: %q", len(got), got)
	}
	if !strings.Contains(got[0], "checking... ") {
		t.Errorf("first line: %s", got[0])
	}
}
