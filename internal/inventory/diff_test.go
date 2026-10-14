package inventory

import "testing"

func parseOK(t *testing.T, s string) *Inventory {
	t.Helper()
	inv, ps := Parse([]byte(s))
	if len(ps) > 0 {
		t.Fatalf("problems: %v", ps)
	}
	return inv
}

const diffBase = `format = 1
[[machine]]
id = "hub"
name = "Hub"
role = "hub"
address = "192.0.2.10"
open = ["none"]
home = { x = 0, y = 0 }
[[machine]]
id = "nas-1"
name = "NAS"
role = "nas"
address = "192.0.2.20"
open = ["ssh"]
home = { x = 1, y = 0 }
`

func TestDiffAddedRemovedChanged(t *testing.T) {
	cur := parseOK(t, diffBase)
	next := parseOK(t, `format = 1
[[machine]]
id = "hub"
name = "Hub"
role = "hub"
address = "192.0.2.10"
open = ["none"]
home = { x = 0, y = 0 }
[[machine]]
id = "nas-1"
name = "NAS"
role = "nas"
address = "192.0.2.99"
open = ["ssh"]
home = { x = 1, y = 0 }
[[machine]]
id = "ai-1"
name = "AI"
role = "ai"
address = "192.0.2.30"
open = ["vnc"]
home = { x = 2, y = 0 }
`)
	c := Diff(cur, next)
	if len(c.Added) != 1 || c.Added[0] != "ai-1" || len(c.Removed) != 0 || len(c.Changed) != 1 || c.Changed[0].ID != "nas-1" || c.Changed[0].Fields[0] != "address" {
		t.Fatalf("changes: %+v", c)
	}
	if !Diff(cur, cur).Empty() {
		t.Fatal("same inventory is not empty")
	}
	r := Diff(next, cur)
	if len(r.Removed) != 1 || r.Removed[0] != "ai-1" {
		t.Fatalf("reverse: %+v", r)
	}
}
