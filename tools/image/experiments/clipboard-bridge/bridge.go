// Command clipboard-bridge is an EXPERIMENT (docs/proposals/node-helper-api.md):
// the smallest pieces of a hub-to-node text clipboard bridge, used to test
// wl-copy and wl-paste between two headless Wayland compositors on loopback.
// It is not part of any image and is not the production node helper. It uses
// only the Go standard library and calls wl-copy and wl-paste from wl-clipboard.
package main

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

// DefaultMaxText is the proposed size limit for one clipboard text, in bytes of
// UTF-8. neatvnc 1.0.3 closes a VNC client that sends more than 10,000,000.
const DefaultMaxText = 1 << 20

// Errors that map to API error codes.
var (
	ErrEmpty    = errors.New("empty text")
	ErrTooBig   = errors.New("text too large")
	ErrNotText  = errors.New("not valid UTF-8 text")
	ErrNoSelect = errors.New("nothing is copied")
)

// CheckText applies the rules every text must pass, on both sides.
func CheckText(text string, max int) error {
	switch {
	case text == "":
		return ErrEmpty
	case len(text) > max:
		return ErrTooBig
	case !utf8.ValidString(text) || strings.ContainsRune(text, 0):
		return ErrNotText
	}
	return nil
}

// Normalize makes two texts that differ only in line endings equal (CRLF to LF),
// because a display protocol or viewer may change them on the way.
func Normalize(text string) string { return strings.ReplaceAll(text, "\r\n", "\n") }

// Hash is what the echo guard remembers: SHA-256 of the normalised text.
func Hash(text string) [32]byte { return sha256.Sum256([]byte(Normalize(text))) }

// Guard stops clipboard echo. It remembers the hash of the last text sent to,
// or received from, the other side, and refuses a change that equals it. It
// also refuses changes that come faster than MinGap (a second line of defence
// that bounds any loop the hash rule misses).
type Guard struct {
	MinGap time.Duration
	Now    func() time.Time

	mu       sync.Mutex
	last     [32]byte
	haveLast bool
	lastAt   time.Time
}

// NewGuard makes a guard.
func NewGuard(minGap time.Duration) *Guard { return &Guard{MinGap: minGap, Now: time.Now} }

// Allow says whether a change may be sent on. The reason is for the log.
func (g *Guard) Allow(text string, max int) (bool, string) {
	if err := CheckText(text, max); err != nil {
		return false, err.Error()
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.haveLast && g.last == Hash(text) {
		return false, "same as last sent or received"
	}
	if g.haveLast && g.Now().Sub(g.lastAt) < g.MinGap {
		return false, "too soon after the last one"
	}
	return true, ""
}

// Remember records a text that was just sent to, or received from, the other side.
func (g *Guard) Remember(text string) {
	g.mu.Lock()
	g.last, g.haveLast, g.lastAt = Hash(text), true, g.Now()
	g.mu.Unlock()
}

// Clipboard is the regular (copy and paste) selection of one Wayland session.
type Clipboard interface {
	// Get returns the text. It returns ErrNoSelect when nothing, or nothing
	// that is text, is copied, and ErrTooBig when the text is over max.
	Get(max int) (string, error)
	// Set makes text the selection.
	Set(text string) error
}

// Wl uses wl-paste and wl-copy. Env is added to the environment, so one program
// can talk to two compositors (WAYLAND_DISPLAY and XDG_RUNTIME_DIR).
type Wl struct {
	Env []string
	Bin string // directory with wl-copy and wl-paste, "" means PATH
}

func (w Wl) cmd(name string, args ...string) *exec.Cmd {
	if w.Bin != "" {
		name = w.Bin + "/" + name
	}
	c := exec.Command(name, args...)
	c.Env = append(os.Environ(), w.Env...)
	return c
}

// Get runs wl-paste. -n keeps it from adding a newline; --type text refuses
// images and files. It reads at most max+1 bytes and stops the program.
func (w Wl) Get(max int) (string, error) {
	c := w.cmd("wl-paste", "-n", "--type", "text")
	out, err := c.StdoutPipe()
	if err != nil {
		return "", err
	}
	if err := c.Start(); err != nil {
		return "", err
	}
	data, _ := io.ReadAll(io.LimitReader(out, int64(max)+1))
	if len(data) > max {
		c.Process.Kill()
		c.Wait()
		return "", ErrTooBig
	}
	if err := c.Wait(); err != nil {
		return "", ErrNoSelect
	}
	return string(data), nil
}

// Set runs wl-copy with the type forced, so the content is never sniffed (the
// sniffing calls xdg-mime and file, which a node image may not have). wl-copy
// forks a background process that serves the selection; its stdout and stderr
// must not be Go pipes, or Wait would never return.
func (w Wl) Set(text string) error {
	c := w.cmd("wl-copy", "--type", "text/plain;charset=utf-8")
	c.Stdin = strings.NewReader(text)
	if err := c.Run(); err != nil {
		return fmt.Errorf("wl-copy: %v", err)
	}
	return nil
}

// SetIfChanged sets text unless the selection already holds it. Setting the
// same text again would make a new selection and wake every watcher, which is
// what starts an echo loop. It reports whether it set anything.
func SetIfChanged(c Clipboard, text string, max int) (bool, error) {
	if err := CheckText(text, max); err != nil {
		return false, err
	}
	cur, err := c.Get(max)
	if err == nil && cur == text {
		return false, nil
	}
	return true, c.Set(text)
}

// Emit is run by "wl-paste --watch". It reads the new selection from stdin and
// writes one line to stdout (which is wl-paste's stdout): "T <base64>" for a
// text within max, "BIG" if over, "EMPTY" if empty. It always drains stdin.
func Emit(in io.Reader, out io.Writer, max int) {
	var buf bytes.Buffer
	n, _ := io.Copy(&buf, io.LimitReader(in, int64(max)+1))
	io.Copy(io.Discard, in)
	switch {
	case n == 0:
		fmt.Fprintln(out, "EMPTY")
	case n > int64(max):
		fmt.Fprintln(out, "BIG")
	default:
		fmt.Fprintln(out, "T "+b64(buf.Bytes()))
	}
}
