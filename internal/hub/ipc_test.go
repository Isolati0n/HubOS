package hub

import (
	"os"
	"path/filepath"
	"strings"
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
