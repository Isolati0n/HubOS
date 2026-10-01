package driftwm

import (
	"bufio"
	"context"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// fakeServer answers each request line from a table keyed by the request text.
func fakeServer(t *testing.T, replies map[string]string) *Client {
	t.Helper()
	dir, err := os.MkdirTemp("", "dwt")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	path := filepath.Join(dir, "s.sock")
	l, err := net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { l.Close() })
	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}
			go func() {
				defer c.Close()
				br := bufio.NewReader(c)
				line, err := br.ReadString('\n')
				if err != nil {
					return
				}
				r, ok := replies[strings.TrimSpace(line)]
				if !ok {
					r = `{"Err":"unexpected request"}`
				}
				c.Write([]byte(r + "\n"))
				if strings.TrimSpace(line) == `"Subscribe"` {
					c.Write([]byte(`{"State":{"camera":[1,2],"windows":[{"id":7,"app_id":"a","title":"t","position":[3,4],"size":[5,6]}],"outputs":[]}}` + "\n"))
				}
			}()
		}
	}()
	return &Client{Path: path}
}

const state = `{"Ok":{"State":{"camera":[0.0,-0.5],"zoom":1.0,"windows":[{"id":0,"app_id":"hubos-x","title":"X Box","position":[10,20],"size":[700,525],"is_focused":true,"mode":"Normal"}],"outputs":[{"name":"winit","camera":[0.0,-0.5],"zoom":1.0,"size":[1280,800],"active":true}]}}}`

func TestRequestsAndReplies(t *testing.T) {
	c := fakeServer(t, map[string]string{
		`"State"`:                                state,
		`{"Move":{"to":[300,-100],"window":4}}`:  `{"Ok":{"Position":{"x":300,"y":-100}}}`,
		`{"Resize":{"to":[500,400],"window":4}}`: `{"Ok":{"Size":{"width":500,"height":400}}}`,
		`{"Focus":4}`:                            `{"Ok":{"Focused":{"id":4,"app_id":"a"}}}`,
		`{"Close":4}`:                            `{"Ok":"Ok"}`,
	})
	s, err := c.State()
	if err != nil {
		t.Fatal(err)
	}
	w, ok := s.Window(0)
	if !ok || w.AppID != "hubos-x" || w.Position != [2]int{10, 20} || w.Size != [2]int{700, 525} {
		t.Errorf("window: %+v", w)
	}
	if vw, vh := s.Viewport(); vw != 1280 || vh != 800 {
		t.Errorf("viewport %d x %d", vw, vh)
	}
	for name, err := range map[string]error{
		"move": c.Move(4, 300, -100), "resize": c.Resize(4, 500, 400), "focus": c.Focus(4), "close": c.Close(4),
	} {
		if err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
	if err := c.Focus(99); err == nil || !strings.Contains(err.Error(), "unexpected request") {
		t.Errorf("an error reply must become an error, got %v", err)
	}
}

func TestSubscribeDeliversAndCloses(t *testing.T) {
	c := fakeServer(t, map[string]string{`"Subscribe"`: `{"Ok":"Ok"}`})
	ch, err := c.Subscribe(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	select {
	case s := <-ch:
		if len(s.Windows) != 1 || s.Windows[0].ID != 7 {
			t.Errorf("got %+v", s)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no snapshot")
	}
	select {
	case _, open := <-ch:
		if open {
			t.Error("expected the channel to close when the server ends the connection")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("channel did not close")
	}
}

func TestNoSocket(t *testing.T) {
	if _, err := (&Client{Path: "/nonexistent/x.sock"}).State(); err == nil {
		t.Error("want an error")
	}
}

// Needs a running driftwm: set HUBOS_DRIFTWM_TEST=1 (see docs/hubd-slice2.md).
func TestRealDriftwm(t *testing.T) {
	if os.Getenv("HUBOS_DRIFTWM_TEST") == "" {
		t.Skip("set HUBOS_DRIFTWM_TEST=1 with a running driftwm")
	}
	c := &Client{Path: SocketPath()}
	s, err := c.State()
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("windows: %d, viewport: %v", len(s.Windows), s.Outputs)
	if _, err := c.Identity(); err != nil {
		t.Error(err)
	}
}
