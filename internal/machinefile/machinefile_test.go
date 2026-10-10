package machinefile

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"hubos/internal/inventory"
)

// write puts machine files (file name -> text) in a new folder and returns it.
func write(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for n, c := range files {
		if err := os.WriteFile(filepath.Join(dir, n), []byte(c), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func gen(t *testing.T, files map[string]string) (string, []Problem) {
	t.Helper()
	srcs, ps, err := Load(write(t, files))
	if err != nil {
		t.Fatal(err)
	}
	if len(ps) > 0 {
		return "", ps
	}
	return Generate(srcs)
}

func problemsText(ps []Problem) string {
	var b strings.Builder
	for _, p := range ps {
		b.WriteString(p.String() + "\n")
	}
	return b.String()
}

// The machine files used by most tests. 192.0.2.x is the range reserved for documentation.
var (
	hubFile = `name = "hub"
kind = "hub"
address = "192.0.2.10"
group = "desk"
open = ["none"]
home = { x = 0, y = 0 }
`
	nasFile = `name = "nas-1"
kind = "node"
address = "192.0.2.20:8480"
group = "storage"
role = "nas"
open = ["ssh", "files"]
home = { x = -2000, y = 0 }
user = "owner"
share = "pool"
`
	aiFile = `name = "ai-1"
kind = "node"
address = "192.0.2.30:8480"
group = "generative"
role = "ai"
open = ["vnc", "ssh"]
home = { x = 2000, y = 0 }
`
	deskFile = `name = "desk-1"
kind = "node"
address = "192.0.2.50:8480"
role = "desktop"
open = ["vnc"]
home = { x = 0, y = 1500 }
`
	vmhostFile = `name = "vmhost-1"
kind = "node"
address = "192.0.2.40:8480"
role = "vm-host"
open = ["ssh"]
home = { x = 0, y = -1500 }
`
)

func guest(name, host string, x int) string {
	return "name = \"" + name + "\"\nkind = \"node\"\naddress = \"192.0.2.41\"\nguest_of = \"" + host +
		"\"\nopen = [\"spice\"]\nlifetime = \"ephemeral\"\nhome = { x = " + itoa(x) + ", y = 3000 }\n"
}

func itoa(n int) string { return strconv.Itoa(n) }

// 1. one hub plus three nodes: a valid inventory that the existing validator accepts, the same bytes on a second run.
func TestOneHubThreeNodes(t *testing.T) {
	files := map[string]string{"hub.toml": hubFile, "nas-1.toml": nasFile, "ai-1.toml": aiFile, "desk-1.toml": deskFile}
	text, ps := gen(t, files)
	if len(ps) > 0 {
		t.Fatal(problemsText(ps))
	}
	inv, vps := inventory.Parse([]byte(text))
	if len(vps) > 0 || len(inv.Machines) != 4 {
		t.Fatalf("machines %d, problems %s", len(inv.Machines), problemsText(vps))
	}
	by := map[string]inventory.Machine{}
	for _, m := range inv.Machines {
		by[m.ID] = m
	}
	if by["hub"].Role != "hub" || by["nas-1"].Role != "nas" || by["nas-1"].Address != "192.0.2.20" || by["nas-1"].Port == nil || *by["nas-1"].Port != 8480 {
		t.Fatalf("hub or nas entry wrong: %+v / %+v", by["hub"], by["nas-1"])
	}
	again, _ := gen(t, files)
	if again != text {
		t.Fatal("two runs gave different bytes")
	}
	if !strings.Contains(text, "# inputs sha256: ") {
		t.Fatal("no hash of the inputs in the header")
	}
}

// 2. a duplicate name is refused (by the existing validator: the id is not unique).
func TestDuplicateName(t *testing.T) {
	_, ps := gen(t, map[string]string{"hub.toml": hubFile, "nas-1.toml": nasFile, "nas-again.toml": nasFile})
	if !strings.Contains(problemsText(ps), "not unique") {
		t.Fatalf("problems: %q", problemsText(ps))
	}
}

// 3. two hubs are refused (existing validator).
func TestTwoHubs(t *testing.T) {
	second := strings.Replace(hubFile, `name = "hub"`, `name = "hub-2"`, 1)
	second = strings.Replace(second, "home = { x = 0, y = 0 }", "home = { x = 9, y = 9 }", 1)
	_, ps := gen(t, map[string]string{"hub.toml": hubFile, "hub-2.toml": second, "nas-1.toml": nasFile})
	if !strings.Contains(problemsText(ps), "more than one machine has role") {
		t.Fatalf("problems: %q", problemsText(ps))
	}
}

// 4. a missing host (guest_of names nobody) is refused (existing validator).
func TestMissingHost(t *testing.T) {
	_, ps := gen(t, map[string]string{"hub.toml": hubFile, "g.toml": guest("g1", "ghost", 1)})
	if !strings.Contains(problemsText(ps), `host "ghost" does not exist`) {
		t.Fatalf("problems: %q", problemsText(ps))
	}
}

// 5. a guest cycle is refused, with a message that says guests of guests are not allowed.
func TestGuestCycle(t *testing.T) {
	_, ps := gen(t, map[string]string{"hub.toml": hubFile, "a.toml": guest("a", "b", 1), "b.toml": guest("b", "a", 2)})
	got := problemsText(ps)
	if !strings.Contains(got, "only direct guests are allowed") {
		t.Fatalf("problems: %q", got)
	}
}

// 9 (extra). no chains (owner decision 2026-10-10): a guest of a guest is refused and named.
func TestNoGuestChains(t *testing.T) {
	files := map[string]string{"hub.toml": hubFile, "vmhost-1.toml": vmhostFile, "g1.toml": guest("g1", "vmhost-1", 1), "g2.toml": guest("g2", "g1", 2)}
	_, ps := gen(t, files)
	got := problemsText(ps)
	if !strings.Contains(got, "g2") || !strings.Contains(got, `"g1" is itself a guest`) {
		t.Fatalf("problems: %q", got)
	}
	// and the direct guests alone are fine
	delete(files, "g2.toml")
	if text, ps := gen(t, files); len(ps) > 0 || !strings.Contains(text, `host = "vmhost-1"`) {
		t.Fatalf("direct guest refused: %q", problemsText(ps))
	}
}

// 8. migration from an old role-based inventory, then generation: the same machines come back, and the inventory is accepted.
func TestMigrationRoundTrip(t *testing.T) {
	old, err := os.ReadFile("../../examples/inventory.example.toml")
	if err != nil {
		t.Fatal(err)
	}
	before, ps := inventory.Parse(old)
	if len(ps) > 0 {
		t.Fatal(problemsText(ps))
	}
	ms, report, mps := Migrate(old)
	if len(mps) > 0 {
		t.Fatal(problemsText(mps))
	}
	dir := t.TempDir()
	for _, m := range ms {
		if err := os.WriteFile(filepath.Join(dir, m.File.Name+".toml"), []byte(Render(m)), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	srcs, lps, err := Load(dir)
	if err != nil || len(lps) > 0 {
		t.Fatalf("load: %v %s", err, problemsText(lps))
	}
	text, gps := Generate(srcs)
	if len(gps) > 0 {
		t.Fatal(problemsText(gps))
	}
	after, aps := inventory.Parse([]byte(text))
	if len(aps) > 0 {
		t.Fatal(problemsText(aps))
	}
	if len(after.Machines) != len(before.Machines) {
		t.Fatalf("%d machines before, %d after", len(before.Machines), len(after.Machines))
	}
	// Everything but the friendly name must come back exactly.
	ch := inventory.Diff(before, after)
	for _, c := range ch.Changed {
		if len(c.Fields) != 1 || c.Fields[0] != "name" {
			t.Errorf("machine %s changed in more than its friendly name: %v", c.ID, c.Fields)
		}
	}
	if len(ch.Added)+len(ch.Removed) != 0 {
		t.Errorf("added %v removed %v", ch.Added, ch.Removed)
	}
	if len(report) == 0 {
		t.Error("the migration did not report the friendly names it could not keep as a field")
	}
	// the hub is kind = "hub" and each role became a group label
	for _, m := range ms {
		if (m.File.Name == "hub") != (m.File.Kind == KindHub) || m.File.Group == "" {
			t.Errorf("machine file %+v", m.File)
		}
	}
}
