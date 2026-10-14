package fdpass

import (
	"bufio"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// TestPassFdToErlang: Go makes a socketpair, sends one end to an Erlang process over a Unix SOCK_SEQPACKET socket as an
// SCM_RIGHTS control message, closes its own copy of that end, and reads what Erlang writes to the passed descriptor.
// Skipped when no `escript` is on PATH (set ESCRIPT to point at one).
func TestPassFdToErlang(t *testing.T) {
	esc := os.Getenv("ESCRIPT")
	if esc == "" {
		p, err := exec.LookPath("escript")
		if err != nil {
			t.Skip("no escript on PATH (set ESCRIPT)")
		}
		esc = p
	}
	dir := t.TempDir()
	sock := filepath.Join(dir, "rx.sock")
	cmd := exec.Command(esc, "receiver.escript", sock)
	out, _ := cmd.StdoutPipe()
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer cmd.Process.Kill()
	rd := bufio.NewReader(out)
	line, err := rd.ReadString('\n')
	if err != nil || strings.TrimSpace(line) != "READY" {
		t.Fatalf("receiver did not say READY: %q %v", line, err)
	}

	pair, err := syscall.Socketpair(syscall.AF_UNIX, syscall.SOCK_STREAM|syscall.SOCK_CLOEXEC, 0)
	if err != nil {
		t.Fatal(err)
	}
	mine, theirs := pair[0], pair[1]
	conn, err := net.DialUnix("unixpacket", nil, &net.UnixAddr{Name: sock, Net: "unixpacket"})
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, _, err := conn.WriteMsgUnix([]byte("fd follows"), syscall.UnixRights(theirs), nil); err != nil {
		t.Fatal(err)
	}
	syscall.Close(theirs) // the other copy now lives only in the Erlang process

	f := os.NewFile(uintptr(mine), "mine")
	defer f.Close()
	done := make(chan string, 1)
	go func() {
		s, _ := bufio.NewReader(f).ReadString('\n')
		done <- s
	}()
	select {
	case s := <-done:
		if strings.TrimSpace(s) != "hello over the passed fd" {
			t.Fatalf("unexpected data from Erlang: %q", s)
		}
	case <-time.After(20 * time.Second):
		t.Fatal("nothing came over the passed descriptor")
	}
	// Erlang closes the passed socket after writing, so we must now see end-of-file on our side.
	f.SetReadDeadline(time.Now().Add(5 * time.Second))
	rest := make([]byte, 8)
	if n, err := f.Read(rest); n != 0 || err == nil {
		t.Fatalf("expected end-of-file after Erlang closed its copy, got %d bytes, err %v", n, err)
	}
	for {
		l, err := rd.ReadString('\n')
		if err != nil {
			t.Fatalf("receiver output ended before DONE: %v", err)
		}
		if strings.TrimSpace(l) == "DONE" {
			break
		}
	}
	if err := cmd.Wait(); err != nil {
		t.Fatalf("receiver exited with error: %v", err)
	}
}
