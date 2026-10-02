package main

import (
	"bytes"
	"flag"
	"fmt"
	"net"
	"os"
	"os/exec"
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
		"desktop-1":  "NOT CHECKED (no port in the inventory and no default port for moonlight)",
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

// "hubd check" and "hubd" with no command are the same slice 1 program.
func TestCheckSubcommandIsSliceOne(t *testing.T) {
	n := startNode(t, "127.0.0.21")
	inv := writeInventory(t,
		machine("hub", "hub", "127.0.0.10", `["none"]`, 0, "", 0, 0),
		machine("ai-1", "ai", "127.0.0.21", `["moonlight"]`, n.port, "", 1, 1))
	var a, b, e bytes.Buffer
	if code := dispatch([]string{"--inventory", inv}, &a, &e); code != 0 {
		t.Fatalf("plain: %d %s", code, e.String())
	}
	if code := dispatch([]string{"check", "--inventory", inv}, &b, &e); code != 0 {
		t.Fatalf("check: %d %s", code, e.String())
	}
	if a.String() != b.String() || !strings.Contains(a.String(), "1 of 1 up") {
		t.Errorf("differ:\n%s\n---\n%s", a.String(), b.String())
	}
}

// Reproduces the finding of docs/hubd-slice2.md section 7.3: with 5000
// generated machines and "ulimit -n 1024", the first slice used to show
// thousands of working machines as DOWN ("too many open files"). A check that
// cannot be made must be NOT CHECKED, never DOWN, and with the bounded
// checking here every check is made.
func TestCheckWith5000MachinesAndSmallFileLimitHasNoFalseDown(t *testing.T) {
	if testing.Short() {
		t.Skip("builds three programs and starts 4750 fake machines")
	}
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("no sh")
	}
	bin := t.TempDir()
	build := func(name, pkg string) string {
		out := filepath.Join(bin, name)
		cmd := exec.Command("go", "build", "-o", out, pkg)
		if b, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("build %s: %v\n%s", pkg, err, b)
		}
		return out
	}
	hubd := build("hubd", "hubos/cmd/hubd")
	geninv := build("geninv", "hubos/tools/geninv")
	fakenode := build("fakenode", "hubos/tools/fakenode")

	inv, nodes := filepath.Join(bin, "inventory.toml"), filepath.Join(bin, "nodes.txt")
	if b, err := exec.Command(geninv, "-n", "5000", "-inventory", inv, "-nodes", nodes).CombinedOutput(); err != nil {
		t.Fatalf("geninv: %v\n%s", err, b)
	}
	raw, _ := os.ReadFile(nodes)
	addrs := strings.Fields(string(raw))
	if len(addrs) != 4750 {
		t.Fatalf("%d fake machines to start, want 4750", len(addrs))
	}
	// The fake machines need many file handles; they run in their own
	// process with a high limit. Only hubd gets the small limit.
	nodeCmd := exec.Command("sh", append([]string{"-c", "ulimit -n 20000; exec \"$0\" \"$@\"", fakenode}, addrs...)...)
	if err := nodeCmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { nodeCmd.Process.Kill(); nodeCmd.Wait() })
	for i := 0; ; i++ {
		c, err := net.DialTimeout("tcp", addrs[len(addrs)-1], 200*time.Millisecond)
		if err == nil {
			c.Close()
			break
		}
		if i > 100 {
			t.Fatal("fake machines did not start")
		}
		time.Sleep(100 * time.Millisecond)
	}

	out, err := exec.Command("sh", "-c", "ulimit -n 1024; exec \"$0\" check --inventory \"$1\"", hubd, inv).CombinedOutput()
	if err != nil {
		t.Fatalf("hubd check: %v\n%s", err, out)
	}
	text := string(out)
	down := strings.Count(text, "  DOWN (")
	if down != 250 || strings.Contains(text, "too many open files") || strings.Contains(text, "NOT CHECKED (hubd") {
		t.Errorf("DOWN lines: %d (want exactly the 250 machines with no fake node); output tail:\n%s", down, text[max(0, len(text)-300):])
	}
	if !strings.Contains(text, "\n4750 of 5000 up\n") {
		t.Errorf("summary wrong; output tail:\n%s", text[max(0, len(text)-200):])
	}
}

func TestCheckTimeoutFlagIsValidated(t *testing.T) {
	n := startNode(t, "127.0.0.31")
	inv := writeInventory(t,
		machine("hub", "hub", "127.0.0.10", `["none"]`, 0, "", 0, 0),
		machine("ai-1", "ai", "127.0.0.31", `["moonlight"]`, n.port, "", 1, 1))
	for _, tc := range []struct{ arg, want string }{
		{"0s", "must be more than 0"},
		{"-1s", "must be more than 0"},
		{"2m", "must be at most 1m0s"},
		{"4s", "longer than the total limit of this command (3s)"},
	} {
		var out, errb bytes.Buffer
		code := run([]string{"--inventory", inv, "--check-timeout", tc.arg}, &out, &errb, testConfig)
		if code != exitFailure || !strings.Contains(errb.String(), tc.want) {
			t.Errorf("%s: exit %d, %q", tc.arg, code, errb.String())
		}
	}
	var out, errb bytes.Buffer
	if code := run([]string{"--inventory", inv, "--check-timeout", "500ms"}, &out, &errb, testConfig); code != 0 || !strings.Contains(out.String(), "1 of 1 up") {
		t.Errorf("valid value: exit %d, %q %q", code, out.String(), errb.String())
	}
	// The serve command checks the same way.
	errb.Reset()
	if code := dispatch([]string{"serve", "--inventory", inv, "--check-timeout", "0"}, &out, &errb); code != exitFailure || !strings.Contains(errb.String(), "--check-timeout must be more than 0") {
		t.Errorf("serve: exit %d %q", code, errb.String())
	}
}

func TestIgnoreAppIDFlagIsRepeatable(t *testing.T) {
	var l appIDList
	fs := flag.NewFlagSet("x", flag.ContinueOnError)
	fs.Var(&l, "ignore-app-id", "")
	if err := fs.Parse([]string{"--ignore-app-id", "waybar", "--ignore-app-id=wofi", "--ignore-app-id", "foot-popup"}); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(l, ","); got != "waybar,wofi,foot-popup" {
		t.Errorf("got %q", got)
	}
	if err := fs.Parse([]string{"--ignore-app-id", ""}); err == nil {
		t.Error("an empty app-id must be refused")
	}
	var none appIDList
	if len(none) != 0 {
		t.Error("default must be none")
	}
}

// A machine without a port is checked at the default_port of its viewer, and
// "not checked" (with the program named) when there is none. Guests with
// spice or vnc have no default.
func TestCheckUsesTheViewersDefaultPort(t *testing.T) {
	sun := startNode(t, "127.0.0.51")
	ssh := startNode(t, "127.0.0.52")
	own := startNode(t, "127.0.0.53")
	path := writeInventory(t,
		machine("hub", "hub", "127.0.0.50", `["none"]`, 0, "", 0, 0),
		machine("pc", "desktop", sun.addr, `["moonlight"]`, 0, "", 1, 1),
		machine("box", "vm-host", ssh.addr, `["ssh"]`, 0, "", 2, 2),
		machine("own", "ai", own.addr, `["moonlight"]`, own.port, "", 3, 3),
		machine("vm", "guest", "127.0.0.54", `["spice"]`, 0, "host = \"box\"\nlifetime = \"ephemeral\"\n", 4, 4),
	)
	viewers := fmt.Sprintf(`format = 1
[[viewer]]
id = "moon"
programs = ["moonlight"]
command = ["moonlight", "stream", "{address}"]
sets_name = false
default_port = %d
[[viewer]]
id = "term"
programs = ["ssh"]
command = ["foot", "--app-id={app_id}", "--", "ssh", "{address}"]
sets_name = true
default_port = %d
[[viewer]]
id = "rv"
programs = ["spice", "vnc"]
command = ["remote-viewer", "--name={app_id}", "spice://{address}:{port}"]
sets_name = true
`, sun.port, ssh.port)
	vpath := filepath.Join(filepath.Dir(path), "viewers.toml")
	if err := os.WriteFile(vpath, []byte(viewers), 0o600); err != nil {
		t.Fatal(err)
	}
	code, out, errOut := runHubd("--inventory", path)
	if code != 0 || errOut != "" {
		t.Fatalf("exit %d, stderr %q\n%s", code, errOut, out)
	}
	t.Log("\n" + out)
	for id, status := range map[string]string{
		"pc":  "UP",
		"box": "UP",
		"own": "UP",
		"vm":  "NOT CHECKED (no port in the inventory and no default port for spice)",
	} {
		if l := line(t, out, id); !strings.HasSuffix(l, status) {
			t.Errorf("%s: want %q, got %q", id, status, l)
		}
	}
	if l := line(t, out, "pc"); !strings.Contains(l, fmt.Sprintf("%s:%d", sun.addr, sun.port)) {
		t.Errorf("pc target should be the default port: %q", l)
	}
	// The machine's own port wins over the default.
	if l := line(t, out, "own"); !strings.Contains(l, fmt.Sprintf(":%d", own.port)) {
		t.Errorf("own: %q", l)
	}
	// A viewers file that was asked for but is missing is an error; a broken one too.
	if code, _, errOut := runHubd("--inventory", path, "--viewers", filepath.Join(t.TempDir(), "nope.toml")); code != exitBadInventory || !strings.Contains(errOut, "there is no viewers file") {
		t.Errorf("missing viewers file: exit %d %q", code, errOut)
	}
	os.WriteFile(vpath, []byte("format = 1\n[[viewer]]\nid = \"x\"\nprograms = [\"ssh\"]\ncommand = [\"foot\"]\ndefault_port = 70000\n"), 0o600)
	if code, _, errOut := runHubd("--inventory", path); code != exitBadInventory || !strings.Contains(errOut, "default_port 70000 is out of range") {
		t.Errorf("broken viewers file: exit %d %q", code, errOut)
	}
}
