package hub

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func waitExit(t *testing.T, p *Proc) {
	t.Helper()
	select {
	case <-p.Exited:
	case <-time.After(5 * time.Second):
		t.Fatal("viewer did not exit")
	}
}

func TestViewerOutputAndErrorGoToTheLog(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "logs")
	l := NewExecLauncher(dir, 1024)
	p, err := l("nas-1", []string{"sh", "-c", "echo to-stdout; echo to-stderr >&2"})
	if err != nil {
		t.Fatal(err)
	}
	waitExit(t, p)
	b, err := os.ReadFile(filepath.Join(dir, "viewer-nas-1.log"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(b)
	if !strings.Contains(text, "to-stdout\n") || !strings.Contains(text, "to-stderr\n") || !strings.Contains(text, "started: sh\n") {
		t.Errorf("log: %q", text)
	}
	if fi, _ := os.Stat(filepath.Join(dir, "viewer-nas-1.log")); fi.Mode().Perm() != 0o600 {
		t.Errorf("log mode %v", fi.Mode().Perm())
	}
	if fi, _ := os.Stat(dir); fi.Mode().Perm() != 0o700 {
		t.Errorf("dir mode %v", fi.Mode().Perm())
	}
	// The header names the program only, never the arguments (inventory text).
	if strings.Contains(text, "echo") {
		t.Errorf("arguments in the log: %q", text)
	}
}

func TestLogOverTheCapIsRotatedAtStart(t *testing.T) {
	dir := t.TempDir()
	old := strings.Repeat("old line\n", 200) // 1800 bytes
	os.WriteFile(filepath.Join(dir, "viewer-a.log"), []byte(old), 0o600)
	os.WriteFile(filepath.Join(dir, "viewer-a.log.1"), []byte("older"), 0o600)
	p, err := NewExecLauncher(dir, 1024)("a", []string{"sh", "-c", "echo fresh"})
	if err != nil {
		t.Fatal(err)
	}
	waitExit(t, p)
	cur, _ := os.ReadFile(filepath.Join(dir, "viewer-a.log"))
	prev, _ := os.ReadFile(filepath.Join(dir, "viewer-a.log.1"))
	if strings.Contains(string(cur), "old line") || !strings.Contains(string(cur), "fresh") || string(prev) != old {
		t.Errorf("current %q\nprevious has %d bytes", cur, len(prev))
	}
	// A log under the cap is appended to.
	p, _ = NewExecLauncher(dir, 1024)("a", []string{"sh", "-c", "echo second"})
	waitExit(t, p)
	cur, _ = os.ReadFile(filepath.Join(dir, "viewer-a.log"))
	if !strings.Contains(string(cur), "fresh") || !strings.Contains(string(cur), "second") {
		t.Errorf("not appended: %q", cur)
	}
}

func TestRunningViewerLogIsTrimmedAndKeepsWriting(t *testing.T) {
	dir := t.TempDir()
	// A viewer that writes about 100 bytes every 20 ms for a while.
	script := `i=0; while [ $i -lt 40 ]; do echo "line $i ......................................................................................"; i=$((i+1)); sleep 0.02; done`
	p, err := NewExecLauncher(dir, 1<<20)("b", []string{"sh", "-c", script})
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(400 * time.Millisecond)
	path := filepath.Join(dir, "viewer-b.log")
	before, _ := os.Stat(path)
	TrimLogs(dir, 500, 0)
	after, _ := os.Stat(path)
	if before.Size() <= 500 || after.Size() >= before.Size() {
		t.Fatalf("not trimmed: %d -> %d", before.Size(), after.Size())
	}
	if prev, _ := os.ReadFile(path + ".1"); len(prev) < int(before.Size()) {
		t.Errorf("the old text was not kept: %d bytes", len(prev))
	}
	waitExit(t, p)
	cur, _ := os.ReadFile(path)
	if !strings.Contains(string(cur), "line 39") {
		t.Errorf("the viewer's later output is missing (it must keep writing after a trim)")
	}
	if strings.Contains(string(cur), "\x00") {
		t.Error("the log has a hole of zero bytes")
	}
	// Under the cap: nothing happens.
	TrimLogs(dir, 1<<20, 0)
}

func TestFailureMessageNamesTheLog(t *testing.T) {
	r := newRig(t, machineDoc("a", "A", "ai", "moonlight", 1, 1, 1, ""))
	r.setStatus("a", statusUp)
	r.h.set.LogDir = t.TempDir()
	r.l.script = nil
	exits := make(chan error, 1)
	exits <- &exitErr{}
	r.h.launch = func(id string, args []string) (*Proc, error) { return &Proc{Exited: exits}, nil }
	res := r.h.Open("a")
	if res.Action != "failed" || !strings.Contains(res.Message, "its output is in "+filepath.Join(r.h.set.LogDir, "viewer-a.log")) {
		t.Errorf("got %+v", res)
	}
}

type exitErr struct{}

func (*exitErr) Error() string { return "exit status 1" }

func TestTotalLogCapRemovesTheOldestFirst(t *testing.T) {
	dir := t.TempDir()
	base := time.Now().Add(-time.Hour)
	mk := func(name string, size int, age time.Duration) string {
		p := filepath.Join(dir, name)
		os.WriteFile(p, []byte(strings.Repeat("x", size)), 0o600)
		mt := base.Add(age)
		os.Chtimes(p, mt, mt)
		return p
	}
	// 6 files of 1000 bytes: ages 0 (oldest) to 5 minutes.
	old1 := mk("viewer-a.log.1", 1000, 0)
	curA := mk("viewer-a.log", 1000, 1*time.Minute)
	old2 := mk("viewer-b.log.1", 1000, 2*time.Minute)
	curB := mk("viewer-b.log", 1000, 3*time.Minute)
	curC := mk("viewer-c.log", 1000, 4*time.Minute)
	curD := mk("viewer-d.log", 1000, 5*time.Minute)
	size := func(p string) int64 {
		fi, err := os.Stat(p)
		if err != nil {
			return -1
		}
		return fi.Size()
	}
	total := func() (n int64) {
		for _, p := range []string{old1, curA, old2, curB, curC, curD} {
			if s := size(p); s > 0 {
				n += s
			}
		}
		return
	}
	TrimLogs(dir, 1<<20, 6000) // exactly at the cap: nothing happens
	if total() != 6000 {
		t.Fatalf("at the cap: %d", total())
	}
	TrimLogs(dir, 1<<20, 4500) // 1500 to remove: the two rotated copies go first, oldest first
	if size(old1) != -1 || size(old2) != -1 || total() != 4000 || size(curA) != 1000 {
		t.Errorf("after 4500: old1=%d old2=%d curA=%d total=%d", size(old1), size(old2), size(curA), total())
	}
	TrimLogs(dir, 1<<20, 2500) // now current logs, oldest first, emptied but kept
	if size(curA) != 0 || size(curB) != 0 || size(curC) != 1000 || size(curD) != 1000 || total() != 2000 {
		t.Errorf("after 2500: a=%d b=%d c=%d d=%d", size(curA), size(curB), size(curC), size(curD))
	}
	TrimLogs(dir, 1<<20, 64<<20) // far under the cap: nothing
	if total() != 2000 {
		t.Errorf("under the cap changed something: %d", total())
	}
}

func TestDefaultLogCaps(t *testing.T) {
	s := DefaultSettings()
	if s.LogMax != 128<<10 || s.LogTotalMax != 64<<20 {
		t.Errorf("%d %d", s.LogMax, s.LogTotalMax)
	}
}
