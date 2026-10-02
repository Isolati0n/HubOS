package viewers

import (
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"hubos/internal/inventory"
)

func TestExampleFileIsValid(t *testing.T) {
	tab, ps, err := Load("../../examples/viewers.example.toml")
	if err != nil || len(ps) != 0 {
		t.Fatalf("err=%v problems=%q", err, ps)
	}
	if tab.For("moonlight") == nil || tab.For("none") != nil {
		t.Errorf("unexpected table %+v", tab)
	}
	for _, line := range strings.Split(mustRead(t), "\n") {
		l := strings.ToLower(line)
		if strings.HasPrefix(l, "command") && (strings.Contains(l, "moonlight") || strings.Contains(l, "remmina") || strings.Contains(l, "virt-viewer")) {
			t.Errorf("a real viewer command is in the example: %s", line)
		}
	}
}

func TestLoadMissing(t *testing.T) {
	if _, _, err := Load("/nonexistent/viewers.toml"); !errors.Is(err, ErrNotFound) {
		t.Errorf("got %v", err)
	}
}

func TestProblems(t *testing.T) {
	for name, tc := range map[string]struct{ doc, want string }{
		"no format":        {"", "format version"},
		"bad format":       {"format = 2\n", "format version 2"},
		"unknown field":    {"format = 1\n[[viewer]]\nid=\"a\"\nprograms=[\"ssh\"]\ncommand=[\"x\"]\nshell=true\n", `unknown field "viewer.shell"`},
		"no command":       {"format = 1\n[[viewer]]\nid=\"a\"\nprograms=[\"ssh\"]\n", "command must name a program"},
		"program none":     {"format = 1\n[[viewer]]\nid=\"a\"\nprograms=[\"none\"]\ncommand=[\"x\"]\n", `"none" is not a program that can be opened`},
		"unknown holder":   {"format = 1\n[[viewer]]\nid=\"a\"\nprograms=[\"ssh\"]\ncommand=[\"x\",\"{password}\"]\n", "unknown placeholder {password}"},
		"holder in prog":   {"format = 1\n[[viewer]]\nid=\"a\"\nprograms=[\"ssh\"]\ncommand=[\"{name}\"]\n", "must be fixed text"},
		"sets name no use": {"format = 1\n[[viewer]]\nid=\"a\"\nprograms=[\"ssh\"]\ncommand=[\"x\"]\nsets_name=true\n", "never uses {app_id}"},
		"program twice":    {"format = 1\n[[viewer]]\nid=\"a\"\nprograms=[\"ssh\"]\ncommand=[\"x\"]\n[[viewer]]\nid=\"b\"\nprograms=[\"ssh\"]\ncommand=[\"y\"]\n", "already served"},
		"unfinished":       {"format = 1\n[[viewer]]\nid=\"a\"\nprograms=[\"ssh\"]\ncommand=[\"x\",\"{id\"]\n", "no closing }"},
	} {
		_, ps := Parse([]byte(tc.doc))
		if !strings.Contains(strings.Join(ps, "\n"), tc.want) {
			t.Errorf("%s: got %q, want something with %q", name, ps, tc.want)
		}
	}
}

func port(n int) *int { return &n }

func TestArgsOneArgumentEach(t *testing.T) {
	v := Viewer{ID: "v", Command: []string{"prog", "--title={name}", "{address}:{port}", "-x", "{user}@{id}"}}
	// Hostile text stays inside its own argument: no splitting, no second
	// substitution, no shell involved.
	m := inventory.Machine{ID: "a1", Name: `x"; rm -rf / ; {port} $(id) 'q`, Address: "h.lan", Port: port(7), User: "o wner"}
	got, err := v.Args(m)
	want := []string{"prog", `--title=x"; rm -rf / ; {port} $(id) 'q`, "h.lan:7", "-x", "o wner@a1"}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Errorf("got %q err=%v\nwant %q", got, err, want)
	}
}

func TestArgsMissingValue(t *testing.T) {
	v := Viewer{ID: "v", Command: []string{"prog", "{port}"}}
	_, err := v.Args(inventory.Machine{ID: "a", Name: "A", Address: "h"})
	var me *MissingError
	if !errors.As(err, &me) || me.Field != "port" {
		t.Errorf("got %v", err)
	}
	if got, err := (&Viewer{Command: []string{"prog", "{app_id}"}}).Args(inventory.Machine{ID: "a"}); err != nil || got[1] != "hubos-a" {
		t.Errorf("app id: %q %v", got, err)
	}
}

func mustRead(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile("../../examples/viewers.example.toml")
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestTestdataViewerFilesAreValid(t *testing.T) {
	for _, p := range []string{"../../testdata/viewers/ambiguous.toml", "../../testdata/viewers/ignores-name.toml", "../../testdata/viewers/chatty.toml"} {
		if _, ps, err := Load(p); err != nil || len(ps) != 0 {
			t.Errorf("%s: %v %q", p, err, ps)
		}
	}
}

func TestWindowWait(t *testing.T) {
	doc := func(v string) string {
		return "format = 1\n[[viewer]]\nid=\"a\"\nprograms=[\"ssh\"]\ncommand=[\"x\"]\n" + v
	}
	tab, ps := Parse([]byte(doc(`window_wait = "25s"`)))
	if len(ps) != 0 || tab.Viewers[0].WindowWait != 25*time.Second {
		t.Fatalf("%q %+v", ps, tab)
	}
	tab, ps = Parse([]byte(doc(`window_wait = "1m30s"`)))
	if len(ps) != 0 || tab.Viewers[0].WindowWait != 90*time.Second {
		t.Fatalf("%q", ps)
	}
	tab, ps = Parse([]byte(doc("")))
	if len(ps) != 0 || tab.Viewers[0].WindowWait != 0 {
		t.Errorf("not given must stay 0 (the hub default applies): %q", ps)
	}
	for _, bad := range []string{`window_wait = "0s"`, `window_wait = "-5s"`, `window_wait = "soon"`, `window_wait = "10"`} {
		if _, ps := Parse([]byte(doc(bad))); len(ps) != 1 || !strings.Contains(ps[0], "positive duration") {
			t.Errorf("%s: %q", bad, ps)
		}
	}
	// A number instead of text is the wrong kind of value.
	if _, ps := Parse([]byte(doc(`window_wait = 10`))); len(ps) == 0 {
		t.Error("a bare number must be refused")
	}
}
