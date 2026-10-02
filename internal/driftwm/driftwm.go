// Package driftwm talks to the driftwm compositor over its local Unix
// socket (one JSON request per line, one JSON reply per line; see
// docs/driftwm-findings.md section 4). It runs no program. The socket is
// only for the same user (mode 0600).
package driftwm

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

// Window is one window as `state` reports it. Position is the centre of the
// visible frame with Y pointing up; Size is the visible frame.
type Window struct {
	ID       int    `json:"id"`
	AppID    string `json:"app_id"`
	Title    string `json:"title"`
	Position [2]int `json:"position"`
	Size     [2]int `json:"size"`
	Focused  bool   `json:"is_focused"`
	Mode     string `json:"mode"`
}

// Output is one screen.
type Output struct {
	Name   string     `json:"name"`
	Camera [2]float64 `json:"camera"`
	Size   [2]int     `json:"size"`
	Active bool       `json:"active"`
}

// State is the part of driftwm's snapshot hubd uses.
type State struct {
	Camera  [2]float64 `json:"camera"`
	Windows []Window   `json:"windows"`
	Outputs []Output   `json:"outputs"`
}

// Window returns the window with this id.
func (s *State) Window(id int) (Window, bool) {
	for _, w := range s.Windows {
		if w.ID == id {
			return w, true
		}
	}
	return Window{}, false
}

// Viewport is the size of the active output, or 0, 0 if none is listed.
func (s *State) Viewport() (w, h int) {
	for _, o := range s.Outputs {
		if o.Active || len(s.Outputs) == 1 {
			return o.Size[0], o.Size[1]
		}
	}
	return 0, 0
}

// Client talks to one driftwm socket.
type Client struct {
	Path string
}

// SocketPath is where driftwm's socket is: DRIFTWM_SOCKET if set, else
// $XDG_RUNTIME_DIR/driftwm/ipc-$WAYLAND_DISPLAY.sock.
func SocketPath() string {
	if p := os.Getenv("DRIFTWM_SOCKET"); p != "" {
		return p
	}
	return filepath.Join(os.Getenv("XDG_RUNTIME_DIR"), "driftwm", "ipc-"+os.Getenv("WAYLAND_DISPLAY")+".sock")
}

// MaxSocketPath is the longest Unix socket path Linux accepts (107 bytes
// plus the final zero).
const MaxSocketPath = 107

// Identity names this driftwm instance: its socket file is made anew each
// time driftwm starts, so inode and change time differ between runs.
// (BELIEVED, not proven; see docs/hubd-slice2.md.)
func (c *Client) Identity() (string, error) {
	fi, err := os.Stat(c.Path)
	if err != nil {
		return "", err
	}
	if st, ok := fi.Sys().(*syscall.Stat_t); ok {
		return fmt.Sprintf("%d:%d.%d", st.Ino, st.Ctim.Sec, st.Ctim.Nsec), nil
	}
	return fi.ModTime().String(), nil
}

type reply struct {
	Ok  json.RawMessage `json:"Ok"`
	Err *string         `json:"Err"`
}

func (c *Client) dial() (net.Conn, error) {
	return net.DialTimeout("unix", c.Path, 2*time.Second)
}

func (c *Client) call(req any, out any) error {
	conn, err := c.dial()
	if err != nil {
		return err
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(5 * time.Second))
	b, _ := json.Marshal(req)
	if _, err := conn.Write(append(b, '\n')); err != nil {
		return err
	}
	line, err := bufio.NewReaderSize(conn, 1<<20).ReadBytes('\n')
	if err != nil {
		return err
	}
	var r reply
	if err := json.Unmarshal(line, &r); err != nil {
		return fmt.Errorf("driftwm sent something unreadable: %w", err)
	}
	if r.Err != nil {
		return errors.New("driftwm: " + *r.Err)
	}
	if out != nil {
		return json.Unmarshal(r.Ok, out)
	}
	return nil
}

// State reads the whole snapshot.
func (c *Client) State() (*State, error) {
	var r struct {
		State State `json:"State"`
	}
	if err := c.call("State", &r); err != nil {
		return nil, err
	}
	return &r.State, nil
}

// Move puts a window's centre at (x, y), Y up.
func (c *Client) Move(id, x, y int) error {
	return c.call(map[string]any{"Move": map[string]any{"window": id, "to": [2]int{x, y}}}, nil)
}

// Resize asks for a visible-frame size (clamped by the client's limits).
func (c *Client) Resize(id, w, h int) error {
	return c.call(map[string]any{"Resize": map[string]any{"window": id, "to": [2]int{w, h}}}, nil)
}

// Focus focuses and raises a window and pans the view to it unless it is
// already fully visible.
func (c *Client) Focus(id int) error { return c.call(map[string]any{"Focus": id}, nil) }

// Close asks the window's program to close it.
func (c *Client) Close(id int) error { return c.call(map[string]any{"Close": id}, nil) }

// Subscribe streams a snapshot on every change. The channel is closed when
// the connection ends (driftwm quit or restarted) or ctx ends.
func (c *Client) Subscribe(ctx context.Context) (<-chan *State, error) {
	conn, err := c.dial()
	if err != nil {
		return nil, err
	}
	if _, err := conn.Write([]byte("\"Subscribe\"\n")); err != nil {
		conn.Close()
		return nil, err
	}
	br := bufio.NewReaderSize(conn, 1<<20)
	line, err := br.ReadBytes('\n') // the "Ok" reply
	if err != nil {
		conn.Close()
		return nil, err
	}
	var r reply
	if json.Unmarshal(line, &r) != nil || r.Err != nil {
		conn.Close()
		return nil, errors.New("driftwm refused to subscribe")
	}
	ch := make(chan *State, 1)
	go func() {
		<-ctx.Done()
		conn.Close()
	}()
	go func() {
		defer close(ch)
		defer conn.Close()
		for {
			line, err := br.ReadBytes('\n')
			if err != nil {
				return
			}
			var ev struct {
				State *State `json:"State"`
			}
			if json.Unmarshal(line, &ev) != nil || ev.State == nil {
				continue
			}
			// Keep only the newest snapshot if the reader is slow.
			select {
			case ch <- ev.State:
			default:
				select {
				case <-ch:
				default:
				}
				select {
				case ch <- ev.State:
				default:
				}
			}
		}
	}()
	return ch, nil
}
