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
	for _, p := range []string{"../../testdata/viewers/ambiguous.toml", "../../testdata/viewers/ignores-name.toml", "../../testdata/viewers/chatty.toml", "../../testdata/viewers/late.toml", "../../testdata/viewers/handover.toml"} {
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

func TestLateGrace(t *testing.T) {
	doc := func(v string) string {
		return "format = 1\n[[viewer]]\nid=\"a\"\nprograms=[\"ssh\"]\ncommand=[\"x\"]\n" + v
	}
	tab, ps := Parse([]byte(doc(`late_grace = "90s"`)))
	if len(ps) != 0 || tab.Viewers[0].LateGrace != 90*time.Second {
		t.Fatalf("%q", ps)
	}
	tab, ps = Parse([]byte(doc(`late_grace = "2m"` + "\n" + `window_wait = "5s"`)))
	if len(ps) != 0 || tab.Viewers[0].LateGrace != 2*time.Minute || tab.Viewers[0].WindowWait != 5*time.Second {
		t.Fatalf("%q", ps)
	}
	if tab, ps = Parse([]byte(doc(""))); len(ps) != 0 || tab.Viewers[0].LateGrace != 0 {
		t.Errorf("not given must stay 0: %q", ps)
	}
	for _, bad := range []string{`late_grace = "0s"`, `late_grace = "-1m"`, `late_grace = "long"`, `late_grace = "60"`} {
		if _, ps := Parse([]byte(doc(bad))); len(ps) != 1 || !strings.Contains(ps[0], `late_grace`) || !strings.Contains(ps[0], "positive duration") {
			t.Errorf("%s: %q", bad, ps)
		}
	}
	if _, ps := Parse([]byte(doc(`late_grace = 60`))); len(ps) == 0 {
		t.Error("a bare number must be refused")
	}
}

func TestDefaultPortSessionAndTitleMatch(t *testing.T) {
	doc := func(v string) string {
		return "format = 1\n[[viewer]]\nid=\"a\"\nprograms=[\"moonlight\"]\ncommand=[\"x\",\"{address}:{port}\",\"{session}\"]\n" + v
	}
	// default_port: used for {port} only when the machine has none.
	tab, ps := Parse([]byte(doc("default_port = 47989")))
	if len(ps) != 0 {
		t.Fatalf("%q", ps)
	}
	v := &tab.Viewers[0]
	m := inventory.Machine{ID: "m", Address: "h", Session: "Desktop"}
	if got, err := v.Args(m); err != nil || !reflect.DeepEqual(got, []string{"x", "h:47989", "Desktop"}) {
		t.Errorf("default port: %q %v", got, err)
	}
	m.Port = port(1234)
	if got, _ := v.Args(m); got[1] != "h:1234" {
		t.Errorf("the machine's own port must win: %q", got)
	}
	// Neither a port nor a default: an error, never a guess.
	tab, _ = Parse([]byte(doc("")))
	if _, err := tab.Viewers[0].Args(inventory.Machine{ID: "m", Address: "h", Session: "s"}); err == nil || !strings.Contains(err.Error(), "m has no port in the inventory") {
		t.Errorf("no port anywhere: %v", err)
	}
	// {session} missing is an error with the plain sentence.
	if _, err := v.Args(inventory.Machine{ID: "m", Address: "h"}); err == nil || err.Error() != "m has no session in the inventory, and its viewer command needs one" {
		t.Errorf("no session: %v", err)
	}
	for _, bad := range []string{"default_port = 0", "default_port = 65536", "default_port = -1"} {
		if _, ps := Parse([]byte(doc(bad))); len(ps) != 1 || !strings.Contains(ps[0], "default_port") || !strings.Contains(ps[0], "out of range (1 to 65535)") {
			t.Errorf("%s: %q", bad, ps)
		}
	}
	if _, ps := Parse([]byte(doc(`default_port = "22"`))); len(ps) == 0 {
		t.Error("text instead of a number must be refused")
	}

	// title_match.
	tab, ps = Parse([]byte(doc(`title_match = "{id} - Moonlight"`)))
	if len(ps) != 0 {
		t.Fatalf("%q", ps)
	}
	title, ok, err := tab.Viewers[0].MatchTitle(inventory.Machine{ID: "ai-1"})
	if !ok || err != nil || title != "ai-1 - Moonlight" {
		t.Errorf("title %q %v %v", title, ok, err)
	}
	if _, ok, _ := tab.Viewers[0].MatchTitle(inventory.Machine{}); !ok {
		t.Error("a title template that needs nothing must render")
	}
	if _, ok, err := (&Viewer{}).MatchTitle(inventory.Machine{ID: "x"}); ok || err != nil {
		t.Error("no title_match means no title")
	}
	// A title that needs a value the machine lacks is an error.
	tab, _ = Parse([]byte(doc(`title_match = "{session} - Moonlight"`)))
	if _, _, err := tab.Viewers[0].MatchTitle(inventory.Machine{ID: "m"}); err == nil || !strings.Contains(err.Error(), "no session") {
		t.Errorf("title needing a session: %v", err)
	}
	// title_match together with sets_name = true is refused; bad placeholders too.
	bad := "format = 1\n[[viewer]]\nid=\"a\"\nprograms=[\"moonlight\"]\ncommand=[\"x\",\"{app_id}\"]\nsets_name=true\ntitle_match=\"{id}\"\n"
	if _, ps := Parse([]byte(bad)); len(ps) != 1 || !strings.Contains(ps[0], "title_match cannot be used with sets_name = true") {
		t.Errorf("title_match with sets_name: %q", ps)
	}
	if _, ps := Parse([]byte(doc(`title_match = "{nope}"`))); len(ps) != 1 || (!strings.Contains(ps[0], "unknown placeholder {nope}") || !strings.Contains(ps[0], "in title_match")) {
		t.Errorf("unknown placeholder in title_match: %q", ps)
	}
}

func TestCheckPortRules(t *testing.T) {
	tab, ps := Parse([]byte("format = 1\n[[viewer]]\nid=\"s\"\nprograms=[\"ssh\"]\ncommand=[\"x\"]\ndefault_port=22\n[[viewer]]\nid=\"v\"\nprograms=[\"vnc\"]\ncommand=[\"x\"]\n"))
	if len(ps) != 0 {
		t.Fatal(ps)
	}
	cases := []struct {
		m    inventory.Machine
		port int
		ok   bool
	}{
		{inventory.Machine{Open: []string{"ssh"}}, 22, true},
		{inventory.Machine{Open: []string{"ssh"}, Port: port(2222)}, 2222, true},
		{inventory.Machine{Open: []string{"vnc"}}, 0, false},
		{inventory.Machine{Open: []string{"vnc"}, Port: port(5901)}, 5901, true},
		{inventory.Machine{Open: []string{"files"}}, 0, false},      // no viewer for it
		{inventory.Machine{Open: []string{"ssh", "vnc"}}, 22, true}, // the first entry decides
		{inventory.Machine{Open: []string{"vnc", "ssh"}}, 0, false},
	}
	for _, c := range cases {
		if p, ok := tab.CheckPort(c.m); p != c.port || ok != c.ok {
			t.Errorf("%v port %v: got %d %v, want %d %v", c.m.Open, c.m.Port, p, ok, c.port, c.ok)
		}
	}
	var none *Table
	if _, ok := none.CheckPort(inventory.Machine{Open: []string{"ssh"}}); ok {
		t.Error("a missing table has no defaults")
	}
	if got := NoPortReason(inventory.Machine{Open: []string{"vnc"}}); got != "no port in the inventory and no default port for vnc" {
		t.Errorf("reason: %q", got)
	}
}
