package hub

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"
)

// MaxSocketPath is the longest Unix socket path Linux accepts (107 bytes
// plus the final zero). driftwm's own socket has the same limit and fails to
// start on a longer path (docs/driftwm-findings.md section 0).
const MaxSocketPath = 107

// SocketPath is where hubd listens: $XDG_RUNTIME_DIR/hubos/hubd.sock. It
// refuses a path the system cannot use, with a message that says so.
func SocketPath(runtimeDir string) (string, error) {
	if runtimeDir == "" {
		return "", errors.New("XDG_RUNTIME_DIR is not set; give --socket PATH")
	}
	p := filepath.Join(runtimeDir, "hubos", "hubd.sock")
	return p, CheckSocketPath(p)
}

// CheckSocketPath says plainly when a socket path is too long.
func CheckSocketPath(p string) error {
	if len(p) > MaxSocketPath {
		return fmt.Errorf("the socket path is %d bytes long (%s); Linux allows at most %d. Use a shorter XDG_RUNTIME_DIR (for example /run/user/1000), or give --socket PATH", len(p), p, MaxSocketPath)
	}
	return nil
}

// Request and Response are the lines spoken on the socket (JSON, one per
// line). A "feed" request is answered with an endless stream of status
// lines, one JSON object per line, exactly what Waybar reads.
type Request struct {
	Cmd    string `json:"cmd"` // status, list, pick, open, end, feed
	ID     string `json:"id,omitempty"`
	Line   string `json:"line,omitempty"`
	Flat   bool   `json:"flat,omitempty"`
	Filter string `json:"filter,omitempty"`
}

type Response struct {
	OK      bool        `json:"ok"`
	Error   string      `json:"error,omitempty"`
	Action  string      `json:"action,omitempty"`
	Message string      `json:"message,omitempty"`
	Lines   []string    `json:"lines,omitempty"`
	Status  *StatusLine `json:"status,omitempty"`
	Reopen  bool        `json:"reopen,omitempty"`
	Flat    bool        `json:"flat,omitempty"`
	Ask     bool        `json:"ask,omitempty"`
}

// Listen opens the socket: directory 0700, socket 0600. If another hubd
// answers on it, that is an error; a leftover file nobody answers on is
// removed.
func Listen(path string) (net.Listener, error) {
	if err := CheckSocketPath(path); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	if c, err := net.DialTimeout("unix", path, time.Second); err == nil {
		c.Close()
		return nil, fmt.Errorf("another hubd is already running (it answers on %s)", path)
	}
	os.Remove(path)
	l, err := net.Listen("unix", path)
	if err != nil {
		return nil, err
	}
	if err := os.Chmod(path, 0o600); err != nil {
		l.Close()
		return nil, err
	}
	return l, nil
}

// Serve answers connections until the listener is closed.
func (h *Hub) Serve(l net.Listener) {
	for {
		c, err := l.Accept()
		if err != nil {
			return
		}
		go h.handle(c)
	}
}

func (h *Hub) handle(c net.Conn) {
	defer c.Close()
	c.SetReadDeadline(time.Now().Add(5 * time.Second))
	line, err := bufio.NewReader(c).ReadBytes('\n')
	if err != nil {
		return
	}
	c.SetReadDeadline(time.Time{})
	var req Request
	if err := json.Unmarshal(line, &req); err != nil {
		reply(c, Response{Error: "unreadable request"})
		return
	}
	switch req.Cmd {
	case "status":
		st := h.Status()
		reply(c, Response{OK: true, Status: &st})
	case "list":
		reply(c, Response{OK: true, Lines: h.List(req.Flat, req.Filter)})
	case "pick":
		r := h.Pick(req.Line)
		reply(c, Response{OK: true, Action: r.Action, Message: r.Message, Reopen: r.Reopen, Flat: r.Flat, Ask: r.Ask})
	case "open":
		r := h.Open(req.ID)
		reply(c, Response{OK: r.Action == "open" || r.Action == "went", Action: r.Action, Message: r.Message})
	case "end":
		r := h.End(req.ID)
		reply(c, Response{OK: r.Action == "end", Action: r.Action, Message: r.Message})
	case "feed":
		h.feed(c)
	default:
		reply(c, Response{Error: fmt.Sprintf("unknown command %q", req.Cmd)})
	}
}

func reply(c net.Conn, r Response) {
	b, _ := json.Marshal(r)
	c.Write(append(b, '\n'))
}

// feed writes a status line now and again whenever it changes. Changes that
// come close together are sent as one (100 ms), and a line identical to the
// last one is not sent again.
func (h *Hub) feed(c net.Conn) {
	ch, stop := h.Changes()
	defer stop()
	gone := make(chan struct{})
	go func() { // notice the reader going away
		buf := make([]byte, 1)
		c.Read(buf)
		close(gone)
	}()
	last := ""
	for {
		if line := h.Status().JSON(); line != last {
			if _, err := c.Write([]byte(line + "\n")); err != nil {
				return
			}
			last = line
		}
		select {
		case <-ch:
			time.Sleep(100 * time.Millisecond)
		case <-gone:
			return
		}
	}
}

// ---- client side ----

// Call sends one request and reads one reply.
func Call(socket string, req Request) (Response, error) {
	c, err := net.DialTimeout("unix", socket, 2*time.Second)
	if err != nil {
		return Response{}, fmt.Errorf("hubd is not running (cannot connect to %s)", socket)
	}
	defer c.Close()
	b, _ := json.Marshal(req)
	c.Write(append(b, '\n'))
	c.SetReadDeadline(time.Now().Add(60 * time.Second))
	line, err := bufio.NewReaderSize(c, 1<<24).ReadBytes('\n')
	if err != nil {
		return Response{}, err
	}
	var r Response
	if err := json.Unmarshal(line, &r); err != nil {
		return Response{}, err
	}
	if r.Error != "" {
		return r, errors.New(r.Error)
	}
	return r, nil
}

// Feed connects and calls out for every status line until the connection
// ends. It returns the error that ended it.
func Feed(socket string, out func(line string)) error {
	c, err := net.DialTimeout("unix", socket, 2*time.Second)
	if err != nil {
		return err
	}
	defer c.Close()
	b, _ := json.Marshal(Request{Cmd: "feed"})
	c.Write(append(b, '\n'))
	sc := bufio.NewScanner(c)
	sc.Buffer(make([]byte, 1<<20), 1<<24)
	for sc.Scan() {
		out(sc.Text())
	}
	if err := sc.Err(); err != nil {
		return err
	}
	return errors.New("hubd closed the connection")
}
