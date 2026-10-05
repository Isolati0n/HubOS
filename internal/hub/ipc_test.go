package hub

import (
	"fmt"
	"net"
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
	if len(ls.Lines) < 3 || !strings.Contains(strings.Join(ls.Lines, "\n"), " ○ a ") {
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

func TestListenNeverRemovesWhatIsNotASocket(t *testing.T) {
	dir := shortDir(t)
	sock, _ := SocketPath(dir)
	os.MkdirAll(filepath.Dir(sock), 0o700)

	// A plain file with precious content.
	os.WriteFile(sock, []byte("precious"), 0o600)
	_, err := Listen(sock)
	if err == nil || !strings.Contains(err.Error(), "is not a socket (it is a plain file)") || !strings.Contains(err.Error(), "will not remove it") {
		t.Fatalf("plain file: %v", err)
	}
	if b, _ := os.ReadFile(sock); string(b) != "precious" {
		t.Errorf("the file was touched: %q", b)
	}
	os.Remove(sock)

	// A link (to something elsewhere) and a folder.
	target := filepath.Join(dir, "target")
	os.WriteFile(target, []byte("keep"), 0o600)
	os.Symlink(target, sock)
	if _, err := Listen(sock); err == nil || !strings.Contains(err.Error(), "a link") {
		t.Errorf("link: %v", err)
	}
	if b, _ := os.ReadFile(target); string(b) != "keep" {
		t.Error("the link target was touched")
	}
	os.Remove(sock)
	os.Mkdir(sock, 0o700)
	if _, err := Listen(sock); err == nil || !strings.Contains(err.Error(), "a folder") {
		t.Errorf("folder: %v", err)
	}
	os.Remove(sock)

	// A leftover socket nobody answers on is replaced.
	l, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	l.(*net.UnixListener).SetUnlinkOnClose(false)
	l.Close()
	if fi, err := os.Lstat(sock); err != nil || fi.Mode()&os.ModeSocket == 0 {
		t.Fatalf("test set-up: %v", err)
	}
	l2, err := Listen(sock)
	if err != nil {
		t.Fatalf("leftover socket: %v", err)
	}
	l2.Close()
}

func TestListenRefusesAFolderThatIsNotOwnedOrNotMode0700(t *testing.T) {
	dir := shortDir(t)
	sock, _ := SocketPath(dir)
	folder := filepath.Dir(sock)
	os.MkdirAll(folder, 0o700)

	os.Chmod(folder, 0o755)
	_, err := Listen(sock)
	if err == nil || !strings.Contains(err.Error(), "has mode 0755, but it must be 0700") || !strings.Contains(err.Error(), "chmod 700") {
		t.Errorf("mode 0755: %v", err)
	}
	if _, serr := os.Lstat(sock); serr == nil {
		t.Error("a socket was made in the loose folder")
	}
	os.Chmod(folder, 0o770)
	if _, err := Listen(sock); err == nil {
		t.Error("mode 0770 accepted")
	}
	os.Chmod(folder, 0o500) // owner can only read: not 0700 either
	if _, err := Listen(sock); err == nil {
		t.Error("mode 0500 accepted")
	}
	os.Chmod(folder, 0o700)
	l, err := Listen(sock)
	if err != nil {
		t.Fatalf("mode 0700: %v", err)
	}
	l.Close()

	// Another owner: only a superuser can give a folder away.
	if os.Geteuid() != 0 {
		t.Skip("needs to run as root to give the folder to another user")
	}
	if err := os.Chown(folder, 12345, 12345); err != nil {
		t.Skip(err)
	}
	_, err = Listen(sock)
	if err == nil || !strings.Contains(err.Error(), "is owned by user 12345, not by the user running hubd (0)") {
		t.Errorf("other owner: %v", err)
	}
}
