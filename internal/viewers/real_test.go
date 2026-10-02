package viewers

import (
	"os"
	"os/exec"
	"reflect"
	"strings"
	"testing"

	"hubos/internal/inventory"
)

// The real-viewer example is only checked here: that it loads, and that each
// command renders the argument list the owner chose. Nothing is run against a
// real machine.

func realTable(t *testing.T) *Table {
	t.Helper()
	tab, ps, err := Load("../../examples/viewers.real.example.toml")
	if err != nil || len(ps) != 0 {
		t.Fatalf("err=%v problems=%q", err, ps)
	}
	return tab
}

func TestRealExampleLoadsAndRendersTheChosenCommandLines(t *testing.T) {
	tab := realTable(t)
	m := func(id, prog string, port *int, user, session string) inventory.Machine {
		return inventory.Machine{ID: id, Name: id + " name", Address: "192.0.2.30", Open: []string{prog}, Port: port, User: user, Session: session}
	}
	for _, tc := range []struct {
		m    inventory.Machine
		want []string
	}{
		{m("nas-1", "ssh", nil, "alice", ""), []string{"foot", "--app-id=hubos-nas-1", "--title=nas-1 name", "--hold", "--", "ssh", "-p", "22", "-l", "alice", "-t", "--", "192.0.2.30", "tmux", "new-session", "-A", "-s", "hubos"}},
		{m("nas-1", "ssh", port(2222), "alice", ""), []string{"foot", "--app-id=hubos-nas-1", "--title=nas-1 name", "--hold", "--", "ssh", "-p", "2222", "-l", "alice", "-t", "--", "192.0.2.30", "tmux", "new-session", "-A", "-s", "hubos"}},
		{m("g1", "spice", port(5901), "", ""), []string{"remote-viewer", "--name=hubos-g1", "--title=g1 name", "--", "spice://192.0.2.30:5901"}},
		{m("g2", "vnc", port(5902), "", ""), []string{"remote-viewer", "--name=hubos-g2", "--title=g2 name", "--", "vnc://192.0.2.30:5902"}},
		{m("ai-1", "moonlight", nil, "", "Desktop"), []string{"moonlight", "stream", "192.0.2.30", "Desktop", "--display-mode", "windowed", "--no-quit-after", "--capture-system-keys", "never"}},
	} {
		v := tab.For(tc.m.Open[0])
		if v == nil {
			t.Fatalf("no viewer for %s", tc.m.Open[0])
		}
		got, err := v.Args(tc.m)
		if err != nil || !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s: got %q err %v\nwant %q", tc.m.Open[0], got, err, tc.want)
		}
	}
	// The defaults: ssh 22 and moonlight 47989; spice and vnc have none.
	for prog, want := range map[string]int{"ssh": 22, "moonlight": 47989} {
		if p, ok := tab.CheckPort(inventory.Machine{Open: []string{prog}}); !ok || p != want {
			t.Errorf("%s default port = %d %v, want %d", prog, p, ok, want)
		}
	}
	for _, prog := range []string{"spice", "vnc"} {
		if _, ok := tab.CheckPort(inventory.Machine{Open: []string{prog}}); ok {
			t.Errorf("%s must have no default port", prog)
		}
		// and a guest without a port cannot be opened
		if _, err := tab.For(prog).Args(inventory.Machine{ID: "g", Address: "a", Open: []string{prog}}); err == nil {
			t.Errorf("%s: no port anywhere must be an error", prog)
		}
	}
	// Moonlight: matched by title, not by name; the title is from the template.
	mv := tab.For("moonlight")
	title, ok, err := mv.MatchTitle(inventory.Machine{ID: "ai-1"})
	if mv.SetsName || !ok || err != nil || title != "ai-1 - Moonlight" || mv.WindowWait.String() != "30s" || mv.LateGrace.String() != "1m0s" {
		t.Errorf("moonlight: sets_name=%v title=%q ok=%v err=%v wait=%v grace=%v", mv.SetsName, title, ok, err, mv.WindowWait, mv.LateGrace)
	}
	// A moonlight machine with no session cannot be opened, with the plain message.
	_, err = mv.Args(inventory.Machine{ID: "ai-1", Address: "a", Open: []string{"moonlight"}})
	if err == nil || err.Error() != "ai-1 has no session in the inventory, and its viewer command needs one" {
		t.Errorf("missing session: %v", err)
	}
	// Every entry says it is unverified.
	data, _ := os.ReadFile("../../examples/viewers.real.example.toml")
	if n := strings.Count(string(data), "UNVERIFIED on hardware"); n < 4 {
		t.Errorf("only %d entries say UNVERIFIED on hardware", n)
	}
}

// `ssh -G` prints the settings ssh would use without connecting. It runs here
// only if an ssh program is found: SSH_BIN (for example an unpacked openssh
// package) or ssh on the PATH. It proves how ssh reads the arguments, not
// that the tmux part works.
func TestRealExampleSshArgumentsAreReadByOpenSSH(t *testing.T) {
	ssh := os.Getenv("SSH_BIN")
	if ssh == "" {
		p, err := exec.LookPath("ssh")
		if err != nil {
			t.Skip("no ssh found (set SSH_BIN to an unpacked openssh-client's ssh)")
		}
		ssh = p
	}
	tab := realTable(t)
	m := inventory.Machine{ID: "nas-1", Name: "NAS", Address: "192.0.2.30", Open: []string{"ssh"}, User: "alice"}
	args, err := tab.For("ssh").Args(m)
	if err != nil {
		t.Fatal(err)
	}
	i := 0
	for i < len(args) && args[i] != "ssh" {
		i++ // the part after "foot ... --" is the ssh command
	}
	sshArgs := append([]string{"-G"}, args[i+1:]...)
	out, err := exec.Command(ssh, sshArgs...).CombinedOutput()
	if err != nil {
		t.Fatalf("ssh -G %q: %v\n%s", sshArgs, err, out)
	}
	t.Logf("ssh %s\n(effective settings)\n%s", strings.Join(sshArgs, " "), pick(string(out), "hostname", "port", "user", "requesttty"))
	got := map[string]string{}
	for _, l := range strings.Split(string(out), "\n") {
		if k, v, ok := strings.Cut(l, " "); ok {
			got[k] = v
		}
	}
	for k, want := range map[string]string{"hostname": "192.0.2.30", "port": "22", "user": "alice", "requesttty": "true"} {
		if got[k] != want {
			t.Errorf("ssh -G %s = %q, want %q", k, got[k], want)
		}
	}
	// A hostile address cannot become an option, because of the "--" (the
	// inventory refuses such an address anyway; this is the second lock).
	m.Address = "-oProxyCommand=touch-nothing"
	args, _ = tab.For("ssh").Args(m)
	i = 0
	for args[i] != "ssh" {
		i++
	}
	out, _ = exec.Command(ssh, append([]string{"-G"}, args[i+1:]...)...).CombinedOutput()
	// With "--" ssh takes the text as a host name and refuses it as one; it
	// is not read as the option -oProxyCommand.
	if !strings.Contains(string(out), "hostname contains invalid characters") || strings.Contains(strings.ToLower(string(out)), "proxycommand touch") {
		t.Errorf("the address was not treated as a host name:\n%s", out)
	}
}

func pick(out string, keys ...string) string {
	var b strings.Builder
	for _, l := range strings.Split(out, "\n") {
		for _, k := range keys {
			if strings.HasPrefix(l, k+" ") {
				b.WriteString("  " + l + "\n")
			}
		}
	}
	return b.String()
}
