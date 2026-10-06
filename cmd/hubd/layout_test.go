package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"hubos/internal/hub"
)

// shortDir is a short temporary folder (a Unix socket path is limited to 107 bytes).
func shortDir(t *testing.T) string {
	t.Helper()
	d, err := os.MkdirTemp("", "hl")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(d) })
	return d
}

// fakeHubd answers every request with reply(req) and records them.
func fakeHubd(t *testing.T, sock string, reply func(hub.Request) hub.Response) (get func() []hub.Request) {
	t.Helper()
	l, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { l.Close() })
	var mu sync.Mutex
	var reqs []hub.Request
	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}
			line, _ := bufio.NewReader(c).ReadBytes('\n')
			var r hub.Request
			json.Unmarshal(line, &r)
			mu.Lock()
			reqs = append(reqs, r)
			mu.Unlock()
			b, _ := json.Marshal(reply(r))
			c.Write(append(b, '\n'))
			c.Close()
		}
	}()
	return func() []hub.Request { mu.Lock(); defer mu.Unlock(); return append([]hub.Request(nil), reqs...) }
}

func TestLayoutCommandLineSendsTheRightRequests(t *testing.T) {
	sock := filepath.Join(shortDir(t), "h.sock")
	get := fakeHubd(t, sock, func(r hub.Request) hub.Response {
		if r.Cmd == "layout" && r.Sub == "list" {
			return hub.Response{OK: true, Message: "2 layouts", Lines: []string{"* work", "  home"}}
		}
		if r.Name == "bad" {
			return hub.Response{OK: false, Message: "no way"}
		}
		return hub.Response{OK: true, Message: "done " + r.Sub}
	})
	run := func(args ...string) (int, string, string) {
		var out, errb bytes.Buffer
		code := dispatch(append([]string{"layout", "--socket", sock}, args...), &out, &errb)
		return code, out.String(), errb.String()
	}
	if code, out, _ := run("save", "work", "--replace"); code != 0 || out != "done save\n" {
		t.Errorf("save: %d %q", code, out)
	}
	if code, _, _ := run("save", "--replace", "work2"); code != 0 {
		t.Errorf("save with the flag first: %d", code)
	}
	run("apply", "work")
	run("delete", "work")
	run("clear")
	if code, out, _ := run("list"); code != 0 || out != "2 layouts\n* work\n  home\n" {
		t.Errorf("list: %d %q", code, out)
	}
	if code, out, _ := run("apply", "bad"); code != 1 || out != "no way\n" {
		t.Errorf("a refused command must exit 1: %d %q", code, out)
	}
	got := get()
	want := []hub.Request{
		{Cmd: "layout", Sub: "save", Name: "work", Replace: true},
		{Cmd: "layout", Sub: "save", Name: "work2", Replace: true},
		{Cmd: "layout", Sub: "apply", Name: "work"},
		{Cmd: "layout", Sub: "delete", Name: "work"},
		{Cmd: "layout", Sub: "clear"},
		{Cmd: "layout", Sub: "list"},
		{Cmd: "layout", Sub: "apply", Name: "bad"},
	}
	if len(got) != len(want) {
		t.Fatalf("requests: %+v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("request %d: %+v, want %+v", i, got[i], want[i])
		}
	}
	// mistakes in the command line are refused before anything is sent
	n := len(get())
	for _, bad := range [][]string{{}, {"save"}, {"save", "a", "b"}, {"list", "x"}, {"clear", "x"}, {"apply", "a", "--replace"}, {"explode"}} {
		if code, _, errs := run(bad...); code != 1 || !strings.Contains(errs, "usage: hubd layout") {
			t.Errorf("%v: %d %q", bad, code, errs)
		}
	}
	if len(get()) != n {
		t.Error("a mistake in the command line sent a request")
	}
}

func TestRestartDesktopCommand(t *testing.T) {
	sock := filepath.Join(shortDir(t), "h.sock")
	ok := true
	get := fakeHubd(t, sock, func(r hub.Request) hub.Response {
		return hub.Response{OK: ok, Message: "asked driftwm to exit"}
	})
	var out, errb bytes.Buffer
	if code := dispatch([]string{"restart-desktop", "--socket", sock}, &out, &errb); code != 0 || !strings.Contains(out.String(), "asked driftwm") {
		t.Errorf("%d %q %q", code, out.String(), errb.String())
	}
	if reqs := get(); len(reqs) != 1 || reqs[0].Cmd != "restart-desktop" {
		t.Errorf("%+v", reqs)
	}
	ok = false
	if code := dispatch([]string{"restart-desktop", "--socket", sock}, &out, &errb); code != 1 {
		t.Errorf("a refused restart must exit 1: %d", code)
	}
	if code := dispatch([]string{"restart-desktop", "--socket", sock, "extra"}, &out, &errb); code != 1 {
		t.Errorf("extra argument: %d", code)
	}
}

// ---- the last known state while hubd is down ----

type lockedBuf struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (l *lockedBuf) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.Write(p)
}
func (l *lockedBuf) String() string { l.mu.Lock(); defer l.mu.Unlock(); return l.b.String() }

func writeSnap(t *testing.T, sock string, age time.Duration) {
	t.Helper()
	snap := hub.Snapshot{Time: time.Now().Add(-age),
		Line: hub.StatusLine{Text: "3 of 4 up", Class: "ok", Tooltip: "1 down: Storage"},
		List: []string{"? search by id or name...", "- Hub (1 machine)", "   hub              Desk Hub                 THIS HUB", " ● ai-1             AI Box                   UP"}}
	b, _ := json.Marshal(snap)
	if err := os.WriteFile(hub.SnapshotPath(sock), b, 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestFeedShowsTheLastKnownStateMarkedStaleWhenHubdIsDown(t *testing.T) {
	sock := filepath.Join(shortDir(t), "h.sock") // nobody listens
	writeSnap(t, sock, 25*time.Second)
	out := &lockedBuf{}
	go feed(sock, out)
	deadline := time.Now().Add(5 * time.Second)
	for !strings.Contains(out.String(), "\n") && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	var l hub.StatusLine
	if err := json.Unmarshal([]byte(strings.SplitN(out.String(), "\n", 2)[0]), &l); err != nil {
		t.Fatalf("%q: %v", out.String(), err)
	}
	if l.Text != "STALE: 3 of 4 up" || l.Class != "alert" || !strings.Contains(l.Tooltip, "hubd is not running: this is the last known state") || !strings.Contains(l.Tooltip, "1 down: Storage") {
		t.Errorf("%+v", l)
	}
}

func TestFeedWithNoLastStateStillSaysHubdStopped(t *testing.T) {
	sock := filepath.Join(shortDir(t), "h.sock")
	out := &lockedBuf{}
	go feed(sock, out)
	deadline := time.Now().Add(5 * time.Second)
	for !strings.Contains(out.String(), "\n") && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if !strings.Contains(out.String(), `"text":"hubd stopped"`) || !strings.Contains(out.String(), `"class":"alert"`) {
		t.Errorf("%q", out.String())
	}
}

func TestMenuShowsTheLastListMarkedStaleAndPickingDoesNothing(t *testing.T) {
	dir := shortDir(t)
	sock := filepath.Join(dir, "h.sock")
	writeSnap(t, sock, 2*time.Minute)
	shown := filepath.Join(dir, "shown")
	fake := filepath.Join(dir, "fakewofi")
	// the stand-in for wofi saves the lines it is given and picks the machine line
	os.WriteFile(fake, []byte("#!/bin/sh\ncat > "+shown+"\necho ' ● ai-1             AI Box                   UP'\n"), 0o700)
	var out, errb bytes.Buffer
	code := dispatch([]string{"menu", "--socket", sock, "--wofi", fake, "--style", filepath.Join(dir, "none.css")}, &out, &errb)
	if code != 1 || !strings.Contains(errb.String(), "hubd is not running, so nothing was done") {
		t.Errorf("exit %d, stderr %q", code, errb.String())
	}
	b, _ := os.ReadFile(shown)
	lines := strings.Split(strings.TrimRight(string(b), "\n"), "\n")
	if len(lines) != 5 || !strings.HasPrefix(lines[0], "! STALE: hubd is not running") || !strings.Contains(lines[0], "2 min old") || !strings.Contains(lines[4], "ai-1") {
		t.Errorf("the menu showed:\n%s", b)
	}
	// without a last list the old message stays
	os.Remove(hub.SnapshotPath(sock))
	errb.Reset()
	if code := dispatch([]string{"menu", "--socket", sock, "--wofi", fake}, &out, &errb); code != 1 || !strings.Contains(errb.String(), "hubd is not running") {
		t.Errorf("no snapshot: %d %q", code, errb.String())
	}
}
