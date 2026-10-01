package main

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var testConfig = config{perMachine: time.Second, total: 3 * time.Second}

// node is a fake machine: a listener on a port the operating system chose.
type node struct {
	l    net.Listener
	addr string
	port int
}

func startNode(t *testing.T, ip string) *node {
	t.Helper()
	l, err := net.Listen("tcp", net.JoinHostPort(ip, "0"))
	if err != nil {
		t.Fatalf("listen on %s: %v", ip, err)
	}
	t.Cleanup(func() { l.Close() })
	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}
			c.Close()
		}
	}()
	return &node{l: l, addr: ip, port: l.Addr().(*net.TCPAddr).Port}
}

func (n *node) stop() { n.l.Close() }

// machine renders one [[machine]] block. port <= 0 leaves the port out.
func machine(id, role, ip, open string, port int, extra string, x, y int) string {
	s := fmt.Sprintf("[[machine]]\nid = %q\nname = %q\nrole = %q\naddress = %q\nopen = %s\nhome = { x = %d, y = %d }\n",
		id, id+" name", role, ip, open, x, y)
	if port > 0 {
		s += fmt.Sprintf("port = %d\n", port)
	}
	return s + extra
}

func writeInventory(t *testing.T, machines ...string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "inventory.toml")
	body := "format = 1\n\n" + strings.Join(machines, "\n")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func runHubd(args ...string) (code int, stdout, stderr string) {
	var out, errb bytes.Buffer
	code = run(args, &out, &errb, testConfig)
	return code, out.String(), errb.String()
}

// line returns the output line that starts with id.
func line(t *testing.T, out, id string) string {
	t.Helper()
	for _, l := range strings.Split(out, "\n") {
		if strings.HasPrefix(l, id+" ") {
			return l
		}
	}
	t.Fatalf("no line for %q in:\n%s", id, out)
	return ""
}

func TestUpDownAndNotChecked(t *testing.T) {
	gaming := startNode(t, "127.0.0.41")
	ai := startNode(t, "127.0.0.42")
	nas := startNode(t, "127.0.0.43")
	vmhost := startNode(t, "127.0.0.44")
	guest := startNode(t, "127.0.0.45")
	ai.stop()
	guest.stop()

	path := writeInventory(t,
		machine("hub", "hub", "127.0.0.40", `["none"]`, 0, "", 0, 0),
		machine("gaming-1", "gaming", gaming.addr, `["moonlight"]`, gaming.port, "", -2000, -1500),
		machine("ai-1", "ai", ai.addr, `["moonlight", "ssh"]`, ai.port, "", 2000, -1500),
		machine("desktop-1", "desktop", "127.0.0.46", `["moonlight"]`, 0, "", 2000, 0),
		machine("nas-1", "nas", nas.addr, `["files", "ssh"]`, nas.port, "share = \"pool\"\n", -2000, 0),
		machine("vmhost-1", "vm-host", vmhost.addr, `["ssh"]`, vmhost.port, "", 0, 1500),
		machine("scratch-os", "guest", guest.addr, `["spice"]`, guest.port, "host = \"vmhost-1\"\nlifetime = \"ephemeral\"\n", 2000, 1500),
	)

	code, out, errOut := runHubd("--inventory", path)
	if code != 0 || errOut != "" {
		t.Fatalf("exit %d, stderr %q\n%s", code, errOut, out)
	}
	want := map[string]string{
		"hub":        "UP (this machine, not checked)",
		"gaming-1":   "UP",
		"ai-1":       "DOWN (connection refused)",
		"desktop-1":  "NOT CHECKED (no port in the inventory)",
		"nas-1":      "UP",
		"vmhost-1":   "UP",
		"scratch-os": "DOWN (connection refused)",
	}
	for id, status := range want {
		if l := line(t, out, id); !strings.HasSuffix(l, status) {
			t.Errorf("%s: want status %q, got line %q", id, status, l)
		}
	}
	if !strings.Contains(out, "\n3 of 5 up, 1 not checked\n") {
		t.Errorf("wrong summary in:\n%s", out)
	}
}

func TestLineShowsProgramAndTarget(t *testing.T) {
	nas := startNode(t, "127.0.0.47")
	path := writeInventory(t,
		machine("hub", "hub", "127.0.0.40", `["none"]`, 0, "", 0, 0),
		machine("nas-1", "nas", nas.addr, `["files", "ssh"]`, nas.port, "share = \"pool\"\n", 1, 1),
	)
	_, out, _ := runHubd("--inventory", path)
	l := line(t, out, "nas-1")
	if !strings.Contains(l, " files ") || !strings.Contains(l, fmt.Sprintf("%s:%d", nas.addr, nas.port)) {
		t.Errorf("program/target missing: %q", l)
	}
}

func TestFirstOpenEntryNoneMeansNotChecked(t *testing.T) {
	n := startNode(t, "127.0.0.48")
	path := writeInventory(t,
		machine("hub", "hub", "127.0.0.40", `["none"]`, 0, "", 0, 0),
		machine("nas-1", "nas", n.addr, `["none"]`, n.port, "", 1, 1),
	)
	_, out, _ := runHubd("--inventory", path)
	if l := line(t, out, "nas-1"); !strings.HasSuffix(l, "NOT CHECKED (nothing to open)") {
		t.Errorf("got %q", l)
	}
}

func TestStoppingANodeChangesTheNextRun(t *testing.T) {
	n := startNode(t, "127.0.0.49")
	path := writeInventory(t,
		machine("hub", "hub", "127.0.0.40", `["none"]`, 0, "", 0, 0),
		machine("ai-1", "ai", n.addr, `["moonlight"]`, n.port, "", 1, 1),
	)
	_, out, _ := runHubd("--inventory", path)
	if !strings.Contains(out, "\n1 of 1 up\n") {
		t.Errorf("first run:\n%s", out)
	}
	n.stop()
	code, out, _ := runHubd("--inventory", path)
	if code != 0 || !strings.Contains(out, "\n0 of 1 up\n") || !strings.HasSuffix(line(t, out, "ai-1"), "DOWN (connection refused)") {
		t.Errorf("second run (exit %d):\n%s", code, out)
	}
}

func TestOverallTimeLimit(t *testing.T) {
	n := startNode(t, "127.0.0.50")
	path := writeInventory(t,
		machine("hub", "hub", "127.0.0.40", `["none"]`, 0, "", 0, 0),
		machine("ai-1", "ai", n.addr, `["moonlight"]`, n.port, "", 1, 1),
	)
	var out, errb bytes.Buffer
	code := run([]string{"--inventory", path}, &out, &errb, config{perMachine: time.Second, total: time.Nanosecond})
	if code != 0 || !strings.Contains(out.String(), "DOWN (overall time limit reached before it answered)") {
		t.Errorf("exit %d\n%s", code, out.String())
	}
}

func TestInvalidInventoryExitsTwoAndChecksNothing(t *testing.T) {
	code, out, errOut := runHubd("--inventory", "../../testdata/broken/15-two-hubs.toml")
	if code != exitBadInventory || out != "" {
		t.Errorf("exit %d, stdout %q", code, out)
	}
	for _, want := range []string{"is not valid (1 problem). No machines were checked.", `more than one machine has role "hub" (hub, hub-2)`} {
		if !strings.Contains(errOut, want) {
			t.Errorf("stderr lacks %q:\n%s", want, errOut)
		}
	}
}

func TestMissingFileExitsTwoAndNeverFallsBack(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "nope.toml")
	code, out, errOut := runHubd("--inventory", missing)
	if code != exitBadInventory || out != "" || !strings.Contains(errOut, "there is no inventory file at "+missing) {
		t.Errorf("exit %d, stdout %q, stderr %q", code, out, errOut)
	}
	if strings.Contains(errOut, "default location") {
		t.Errorf("an explicit path should not mention the default: %q", errOut)
	}
}

func TestUnreadableFileExitsTwo(t *testing.T) {
	// A directory cannot be read as a file.
	code, _, errOut := runHubd("--inventory", t.TempDir())
	if code != exitBadInventory || !strings.Contains(errOut, "cannot read the inventory") {
		t.Errorf("exit %d, stderr %q", code, errOut)
	}
}

func TestCommandLineMistakesExitOne(t *testing.T) {
	for _, args := range [][]string{{"--no-such-flag"}, {"extra"}, {"--inventory", ""}} {
		if code, out, _ := runHubd(args...); code != exitFailure || out != "" {
			t.Errorf("args %q: exit %d, stdout %q", args, code, out)
		}
	}
}

func TestHelpNamesTheDefaultPath(t *testing.T) {
	code, _, errOut := runHubd("-h")
	if code != exitOK || !strings.Contains(errOut, "/etc/hubos/inventory.toml") {
		t.Errorf("exit %d, stderr %q", code, errOut)
	}
}

func TestDefaultPathIsTheApprovedOne(t *testing.T) {
	if defaultInventory != "/etc/hubos/inventory.toml" {
		t.Errorf("default path changed to %q", defaultInventory)
	}
}

func TestHubIsShownButNotCounted(t *testing.T) {
	// Only the hub: shown as UP, but the totals are 0 of 0.
	path := writeInventory(t, machine("hub", "hub", "127.0.0.40", `["none"]`, 0, "", 0, 0))
	code, out, _ := runHubd("--inventory", path)
	if code != 0 || !strings.HasSuffix(line(t, out, "hub"), "UP (this machine, not checked)") || !strings.Contains(out, "\n0 of 0 up\n") {
		t.Errorf("exit %d\n%s", code, out)
	}
}
