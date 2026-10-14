package machinefile

import (
	"fmt"
	"sort"
	"strings"

	"hubos/internal/inventory"
)

// Migrated is one machine file produced from an old role-based inventory entry.
type Migrated struct {
	File     File
	Friendly string // the old friendly name; it has no field in a machine file, so it is kept as a comment
}

// Migrate turns a valid role-based inventory into machine files. It is read-only on the old file.
// Each role becomes the group label; the hub becomes kind = "hub"; a guest becomes a node with guest_of.
// Because format 1 still requires a role, the role is also kept in the bridge field Role for plain nodes
// (see File). report lists, in plain words, everything that did not carry over as a field.
func Migrate(old []byte) (out []Migrated, report []string, problems []Problem) {
	inv, ps := inventory.Parse(old)
	if len(ps) > 0 {
		return nil, nil, ps
	}
	for _, m := range inv.Machines {
		f := File{
			Name: m.ID, Kind: KindNode, Address: m.Address, GuestOf: m.Host, Group: m.Role,
			Open: m.Open, Home: m.Home, User: m.User, Share: m.Share, Session: m.Session, Lifetime: m.Lifetime,
		}
		if m.Port != nil {
			f.Address = fmt.Sprintf("%s:%d", m.Address, *m.Port)
		}
		switch m.Role {
		case "hub":
			f.Kind = KindHub
		case "guest":
			// role comes back from guest_of
		default:
			f.Role = m.Role // bridge
		}
		if m.Name != m.ID {
			report = append(report, fmt.Sprintf("%s: friendly name %q has no field in a machine file; kept as a comment", m.ID, m.Name))
		}
		out = append(out, Migrated{File: f, Friendly: m.Name})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].File.Name < out[j].File.Name })
	return out, report, nil
}

// Render writes a machine file as TOML text (stable order, one comment line with the old friendly name).
func Render(m Migrated) string {
	f := m.File
	var b strings.Builder
	fmt.Fprintf(&b, "# migrated from a role-based inventory\n")
	if m.Friendly != "" && m.Friendly != f.Name {
		fmt.Fprintf(&b, "# friendly name was: %s\n", strings.ReplaceAll(m.Friendly, "\n", " "))
	}
	fmt.Fprintf(&b, "name  = %s\nkind  = %s\n", q(f.Name), q(f.Kind))
	if f.Address != "" {
		fmt.Fprintf(&b, "address = %s\n", q(f.Address))
	}
	if f.GuestOf != "" {
		fmt.Fprintf(&b, "guest_of = %s\n", q(f.GuestOf))
	}
	if f.Group != "" {
		fmt.Fprintf(&b, "group = %s\n", q(f.Group))
	}
	if f.Role != "" {
		fmt.Fprintf(&b, "role = %s   # compatibility bridge: format 1 still requires a role; remove when hubd drops it\n", q(f.Role))
	}
	if f.Open != nil {
		items := make([]string, len(f.Open))
		for i, o := range f.Open {
			items[i] = q(o)
		}
		fmt.Fprintf(&b, "open = [%s]\n", strings.Join(items, ", "))
	}
	if f.Home != nil && f.Home.X != nil && f.Home.Y != nil {
		fmt.Fprintf(&b, "home = { x = %d, y = %d }\n", *f.Home.X, *f.Home.Y)
	}
	for _, kv := range []struct{ k, v string }{{"user", f.User}, {"share", f.Share}, {"session", f.Session}, {"lifetime", f.Lifetime}} {
		if kv.v != "" {
			fmt.Fprintf(&b, "%s = %s\n", kv.k, q(kv.v))
		}
	}
	return b.String()
}
