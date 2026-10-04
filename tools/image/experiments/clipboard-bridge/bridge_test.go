package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// fakeClip is a clipboard in memory. Sets counts how many times a new
// selection was made, because every set wakes every watcher.
type fakeClip struct {
	text    string
	present bool
	Sets    int
}

func (f *fakeClip) Get(max int) (string, error) {
	if !f.present {
		return "", ErrNoSelect
	}
	if len(f.text) > max {
		return "", ErrTooBig
	}
	return f.text, nil
}

func (f *fakeClip) Set(t string) error { f.text, f.present = t, true; f.Sets++; return nil }

func TestCheckText(t *testing.T) {
	cases := []struct {
		name string
		text string
		max  int
		want error
	}{
		{"plain", "hello", 10, nil},
		{"non-ASCII", "café ✓ 日本", 100, nil},
		{"empty", "", 10, ErrEmpty},
		{"exactly at the limit", strings.Repeat("a", 10), 10, nil},
		{"one over", strings.Repeat("a", 11), 10, ErrTooBig},
		{"bytes not letters", "日本", 5, ErrTooBig}, // 6 bytes
		{"bad UTF-8", "a\xffb", 10, ErrNotText},
		{"NUL", "a\x00b", 10, ErrNotText},
	}
	for _, c := range cases {
		if got := CheckText(c.text, c.max); got != c.want {
			t.Errorf("%s: got %v want %v", c.name, got, c.want)
		}
	}
}

func TestGuardHashRule(t *testing.T) {
	now := time.Unix(1000, 0)
	g := NewGuard(0)
	g.Now = func() time.Time { return now }
	if ok, _ := g.Allow("A", 100); !ok {
		t.Fatal("first text must pass")
	}
	g.Remember("A")
	if ok, why := g.Allow("A", 100); ok || !strings.Contains(why, "same") {
		t.Fatalf("same text must be ignored, got %v %q", ok, why)
	}
	if ok, _ := g.Allow("A\r", 100); !ok {
		t.Fatal("a lone CR is a different text")
	}
	if ok, _ := g.Allow("B", 100); !ok {
		t.Fatal("a different text must pass")
	}
	g.Remember("x\ny")
	if ok, _ := g.Allow("x\r\ny", 100); ok {
		t.Fatal("CRLF and LF versions must count as the same text")
	}
}

func TestGuardMinGap(t *testing.T) {
	now := time.Unix(1000, 0)
	g := NewGuard(time.Second)
	g.Now = func() time.Time { return now }
	g.Remember("A")
	if ok, why := g.Allow("B", 100); ok || !strings.Contains(why, "too soon") {
		t.Fatalf("B within the gap must wait, got %v %q", ok, why)
	}
	now = now.Add(1100 * time.Millisecond)
	if ok, _ := g.Allow("B", 100); !ok {
		t.Fatal("B after the gap must pass")
	}
}

func TestEmit(t *testing.T) {
	var out bytes.Buffer
	Emit(strings.NewReader("héllo"), &out, 10)
	if got := out.String(); got != "T aMOpbGxv\n" {
		t.Errorf("text: %q", got)
	}
	out.Reset()
	Emit(strings.NewReader(""), &out, 10)
	if out.String() != "EMPTY\n" {
		t.Errorf("empty: %q", out.String())
	}
	out.Reset()
	Emit(strings.NewReader(strings.Repeat("a", 11)), &out, 10)
	if out.String() != "BIG\n" {
		t.Errorf("big: %q", out.String())
	}
}

func newAPI(f *fakeClip, max int) *NodeAPI {
	return &NodeAPI{Clip: f, Max: max, RateMax: 3, RateWindow: time.Minute, Now: time.Now}
}

func do(h http.Handler, method, path, body string) (int, map[string]any) {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	var m map[string]any
	json.Unmarshal(w.Body.Bytes(), &m)
	return w.Code, m
}

func TestAPIPutGet(t *testing.T) {
	f := &fakeClip{}
	api := newAPI(f, 10)
	if c, m := do(api, "GET", "/v1/clipboard", ""); c != 200 || m["present"] != false {
		t.Fatalf("empty node clipboard: %d %v", c, m)
	}
	if c, m := do(api, "PUT", "/v1/clipboard", `{"text":"héllo","origin":"hub"}`); c != 200 || m["result"] != "set" {
		t.Fatalf("set: %d %v", c, m)
	}
	if c, m := do(api, "GET", "/v1/clipboard", ""); c != 200 || m["text"] != "héllo" || m["present"] != true {
		t.Fatalf("get: %d %v", c, m)
	}
	// an unknown field is ignored (the versioning rule)
	if c, m := do(api, "PUT", "/v1/clipboard", `{"text":"héllo","future_field":1}`); c != 200 || m["result"] != "unchanged" {
		t.Fatalf("same text again: %d %v", c, m)
	}
	_, m := do(api, "GET", "/v1/clipboard", "")
	h := Hash("héllo")
	if m["bytes"] != float64(len("héllo")) || m["sha256"] != hex.EncodeToString(h[:]) {
		t.Fatalf("bytes and hash missing or wrong: %v", m)
	}
	if f.Sets != 1 {
		t.Fatalf("the same text must not make a second selection, sets=%d", f.Sets)
	}
}

func TestAPIErrors(t *testing.T) {
	api := newAPI(&fakeClip{}, 10)
	api.RateMax = 100
	cases := []struct {
		method, body string
		code         int
		errCode      string
	}{
		{"PUT", `{"text":"` + strings.Repeat("a", 11) + `"}`, 413, "too_large"},
		{"PUT", `{"text":""}`, 422, "bad_text"},
		{"PUT", `{"text":"a\u0000b"}`, 422, "bad_text"},
		{"PUT", "{\"text\":\"a\xffb\"}", 422, "bad_text"},
		{"PUT", `not json`, 400, "bad_request"},
		{"DELETE", ``, 405, "method_not_allowed"},
	}
	for _, c := range cases {
		code, m := do(api, c.method, "/v1/clipboard", c.body)
		if code != c.code || m["code"] != c.errCode {
			t.Errorf("%s %q: got %d %v, want %d %s", c.method, c.body, code, m, c.code, c.errCode)
		}
	}
	if code, _ := do(api, "GET", "/v1/other", ""); code != 404 {
		t.Errorf("unknown path: %d", code)
	}
}

func TestAPIBodyLimit(t *testing.T) {
	api := newAPI(&fakeClip{}, 10)
	big := `{"text":"a","pad":"` + strings.Repeat("x", 6*10+5000) + `"}`
	if code, m := do(api, "PUT", "/v1/clipboard", big); code != 413 {
		t.Errorf("huge body: %d %v", code, m)
	}
}

func TestAPIRateLimit(t *testing.T) {
	api := newAPI(&fakeClip{}, 10)
	for i := 0; i < 3; i++ {
		if c, _ := do(api, "PUT", "/v1/clipboard", `{"text":"a"}`); c != 200 {
			t.Fatalf("put %d: %d", i, c)
		}
	}
	if c, m := do(api, "PUT", "/v1/clipboard", `{"text":"a"}`); c != 429 || m["code"] != "rate_limited" {
		t.Fatalf("fourth put: %d %v", c, m)
	}
}

// loop models: hub clipboard -> (push) -> node clipboard -> (wayvnc and viewer)
// -> hub clipboard. Each set on a side wakes that side's watcher. It returns
// how many sets happened on the node before the loop stopped (or hit the cap).
func loop(guard *Guard, alwaysSet bool, startOnHub string) int {
	hub, node := &fakeClip{}, &fakeClip{}
	const cap = 50
	hub.Set(startOnHub)
	hubEvents := []string{startOnHub}
	for len(hubEvents) > 0 && node.Sets < cap {
		text := hubEvents[0]
		hubEvents = hubEvents[1:]
		if guard != nil {
			if ok, _ := guard.Allow(text, 100); !ok {
				continue
			}
			guard.Remember(text)
		}
		before := node.Sets
		if alwaysSet {
			node.Set(text)
		} else {
			SetIfChanged(node, text, 100)
		}
		if node.Sets > before { // the node's watcher fires: wayvnc sends it to the hub
			hub.Set(node.text)
			hubEvents = append(hubEvents, hub.text)
		}
	}
	return node.Sets
}

func TestEchoLoopAndTheRulesThatStopIt(t *testing.T) {
	if n := loop(nil, true, "A"); n < 50 {
		t.Errorf("with no rule the loop must run until the cap, node sets=%d", n)
	}
	if n := loop(NewGuard(0), true, "A"); n != 1 {
		t.Errorf("hash rule on the hub: want 1 node set, got %d", n)
	}
	if n := loop(nil, false, "A"); n != 1 {
		t.Errorf("'unchanged' rule on the node: want 1 node set, got %d", n)
	}
	if n := loop(NewGuard(0), false, "A"); n != 1 {
		t.Errorf("both rules: want 1 node set, got %d", n)
	}
}

// TestRealWlClipboard runs against a real compositor. It is skipped unless
// CB_BIN (directory with wl-copy and wl-paste) and CB_HUB (an XDG_RUNTIME_DIR
// with a Wayland socket "wayland-1") are set; see README.md.
func TestRealWlClipboard(t *testing.T) {
	bin, dir := os.Getenv("CB_BIN"), os.Getenv("CB_HUB")
	if bin == "" || dir == "" {
		t.Skip("CB_BIN and CB_HUB not set: no compositor to test against")
	}
	w := Wl{Bin: bin, Env: []string{"XDG_RUNTIME_DIR=" + dir, "WAYLAND_DISPLAY=wayland-1"}}
	for _, text := range []string{"hello", "café ✓ 日本", "line1\nline2\n", "  spaces  ", strings.Repeat("x", 100000)} {
		if err := w.Set(text); err != nil {
			t.Fatalf("set: %v", err)
		}
		got, err := w.Get(1 << 20)
		if err != nil || got != text {
			t.Errorf("round trip of %d bytes: got %d bytes, err %v", len(text), len(got), err)
		}
	}
	if err := w.Set(strings.Repeat("y", 5000)); err != nil {
		t.Fatal(err)
	}
	if _, err := w.Get(4096); err != ErrTooBig {
		t.Errorf("text over the limit: got %v, want ErrTooBig", err)
	}
	exec.Command(bin+"/wl-copy", "--clear").Run()
}
