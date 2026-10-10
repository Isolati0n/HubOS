// Package machinefile reads one small TOML file per machine and generates the
// hub's inventory.toml from them (docs/proposals/generated-inventory.md).
//
// The generator does not repeat the inventory rules. It writes an inventory in
// the current format (format = 1) and then hands the text to the existing
// validator, internal/inventory.Parse. It adds only the rules that the
// validator cannot know: the machine file's own fields, and "direct guests
// only" (a guest's host must not itself be a guest).
//
// Differences from the format-1 inventory, which the owner's five fields do not cover,
// are listed in the pull request: role, open, home, hub address, friendly name, lifetime.
package machinefile

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode"

	toml "github.com/pelletier/go-toml/v2"

	"hubos/internal/inventory"
)

// Kinds a machine file may name.
const (
	KindHub  = "hub"
	KindNode = "node"
)

// File is one machine file.
//
// The first five fields are the owner's. The rest pass through to the inventory unchanged,
// because format 1 requires some of them (open, home) and has places for the others.
// Role is a COMPATIBILITY BRIDGE: format 1 requires a role and hubd groups its menu by it,
// but the owner is dropping roles in favour of the display-only Group. Role is used only for
// plain nodes (the hub and guests get theirs from Kind and GuestOf). Remove it when hubd's
// inventory format drops role.
type File struct {
	Name    string `toml:"name"`     // unique; becomes the inventory id and name
	Kind    string `toml:"kind"`     // "hub" or "node"
	Address string `toml:"address"`  // how the hub reaches the machine, "host" or "host:port"; the hub may give a host only
	GuestOf string `toml:"guest_of"` // empty, or the name of the host machine
	Group   string `toml:"group"`    // free text, display only; format 1 has no place for it, so it is not written

	Role     string           `toml:"role"` // bridge, see above
	Open     []string         `toml:"open"`
	Home     *inventory.Point `toml:"home"`
	User     string           `toml:"user"`
	Share    string           `toml:"share"`
	Session  string           `toml:"session"`
	Lifetime string           `toml:"lifetime"`
}

// Source is a machine file as read: its path and bytes (the bytes go into the hash of the inputs).
type Source struct {
	Path string
	Data []byte
	File File
}

// Problem is the inventory package's problem type, so both layers report in one list.
type Problem = inventory.Problem

// Load reads every *.toml file in dir, in name order. Unknown fields and wrong types are problems.
func Load(dir string) ([]Source, []Problem, error) {
	paths, err := filepath.Glob(filepath.Join(dir, "*.toml"))
	if err != nil {
		return nil, nil, err
	}
	sort.Strings(paths)
	var srcs []Source
	var ps []Problem
	for _, p := range paths {
		b, err := os.ReadFile(p)
		if err != nil {
			return nil, nil, err
		}
		s, sp := Parse(p, b)
		ps = append(ps, sp...)
		if len(sp) == 0 {
			srcs = append(srcs, s)
		}
	}
	return srcs, ps, nil
}

// Parse reads one machine file. The label of a problem is the file name.
func Parse(path string, data []byte) (Source, []Problem) {
	var f File
	dec := toml.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&f); err != nil {
		return Source{}, []Problem{{Where: filepath.Base(path), Msg: describe(err)}}
	}
	return Source{Path: path, Data: data, File: f}, nil
}

func describe(err error) string {
	if se, ok := err.(*toml.StrictMissingError); ok {
		var parts []string
		for _, e := range se.Errors {
			row, _ := e.Position()
			parts = append(parts, fmt.Sprintf("line %d: unknown field %q", row, strings.Join([]string(e.Key()), ".")))
		}
		return strings.Join(parts, "; ")
	}
	if de, ok := err.(*toml.DecodeError); ok {
		row, col := de.Position()
		return fmt.Sprintf("line %d, column %d: %s", row, col, strings.TrimPrefix(de.Error(), "toml: "))
	}
	return err.Error()
}

func hasControl(s string) bool {
	for _, r := range s {
		if unicode.IsControl(r) || r == ' ' || r == ' ' {
			return true
		}
	}
	return false
}

// Check applies the rules the inventory validator cannot know. It does not repeat the inventory rules.
func Check(srcs []Source) []Problem {
	var ps []Problem
	add := func(w, f string, a ...any) { ps = append(ps, Problem{Where: w, Msg: fmt.Sprintf(f, a...)}) }
	byName := map[string]File{}
	for _, s := range srcs {
		if s.File.Name != "" {
			if _, dup := byName[s.File.Name]; !dup {
				byName[s.File.Name] = s.File
			}
		}
	}
	for _, s := range srcs {
		f := s.File
		w := f.Name
		if w == "" {
			w = filepath.Base(s.Path)
		}
		switch f.Kind {
		case KindHub, KindNode:
		case "":
			add(w, `required field "kind" is missing (it is "hub" or "node")`)
		default:
			add(w, `kind %q is not one of: hub, node`, f.Kind)
		}
		if hasControl(f.Group) {
			add(w, "group must not contain control characters or line breaks")
		}
		if f.Kind == KindHub && f.GuestOf != "" {
			add(w, "the hub cannot be a guest (guest_of is set)")
		}
		if f.Kind == KindNode && f.Address == "" {
			add(w, `required field "address" is missing (a machine other than the hub needs the address the hub reaches it by)`)
		}
		if f.GuestOf != "" {
			if f.GuestOf == f.Name {
				add(w, "guest_of names the machine itself")
			} else if h, ok := byName[f.GuestOf]; ok && h.GuestOf != "" {
				// direct guests only (owner decision 2026-10-10): the host must not itself be a guest
				add(w, "guest_of %q is itself a guest of %q; only direct guests are allowed (no guest of a guest)", f.GuestOf, h.GuestOf)
			}
		}
		if f.Role != "" && f.Kind == KindHub && f.Role != "hub" {
			add(w, "role %q conflicts with kind %q", f.Role, f.Kind)
		}
		if f.Role != "" && f.GuestOf != "" && f.Role != "guest" {
			add(w, "role %q conflicts with guest_of (a guest has role guest)", f.Role)
		}
	}
	return ps
}

// roleOf is the role the inventory entry gets: the hub and guests are known from kind and guest_of.
func roleOf(f File) string {
	switch {
	case f.Kind == KindHub:
		return "hub"
	case f.GuestOf != "":
		return "guest"
	}
	return f.Role
}

// splitAddress splits "host" or "host:port" (also "[v6]:port"). A host without a port gives port 0.
func splitAddress(a string) (host string, port int, err error) {
	if a == "" {
		return "", 0, nil
	}
	if h, p, e := net.SplitHostPort(a); e == nil {
		n, e := strconv.Atoi(p)
		if e != nil {
			return "", 0, fmt.Errorf("port %q is not a number", p)
		}
		return h, n, nil
	}
	return a, 0, nil
}

func q(s string) string { return strconv.Quote(s) }

// Generate writes inventory.toml text from the machine files. It is a pure function of the files:
// the same bytes in give the same bytes out (machines sorted by name, no clock, no environment).
// On any problem it returns the problems and no text.
func Generate(srcs []Source) (string, []Problem) {
	if ps := Check(srcs); len(ps) > 0 {
		return "", ps
	}
	sorted := append([]Source(nil), srcs...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].File.Name < sorted[j].File.Name })

	h := sha256.New()
	for _, s := range sorted {
		h.Write([]byte(filepath.Base(s.Path)))
		h.Write([]byte{0})
		h.Write(s.Data)
		h.Write([]byte{0})
	}

	var b strings.Builder
	fmt.Fprintf(&b, "# GENERATED by invgen from machine_files/*.toml. Do not edit; edit the machine files and generate again.\n")
	fmt.Fprintf(&b, "# inputs sha256: %s\n\nformat = 1\n", hex.EncodeToString(h.Sum(nil)))
	var ps []Problem
	for _, s := range sorted {
		f := s.File
		host, port, err := splitAddress(f.Address)
		if err != nil {
			ps = append(ps, Problem{Where: f.Name, Msg: "address: " + err.Error()})
			continue
		}
		fmt.Fprintf(&b, "\n[[machine]]\nid   = %s\nname = %s\n", q(f.Name), q(f.Name))
		if r := roleOf(f); r != "" {
			fmt.Fprintf(&b, "role = %s\n", q(r))
		}
		if host != "" {
			fmt.Fprintf(&b, "address = %s\n", q(host))
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
		if port != 0 {
			fmt.Fprintf(&b, "port = %d\n", port)
		}
		for _, kv := range []struct{ k, v string }{{"user", f.User}, {"share", f.Share}, {"session", f.Session}} {
			if kv.v != "" {
				fmt.Fprintf(&b, "%s = %s\n", kv.k, q(kv.v))
			}
		}
		if f.GuestOf != "" {
			fmt.Fprintf(&b, "host = %s\n", q(f.GuestOf))
		}
		if f.Lifetime != "" {
			fmt.Fprintf(&b, "lifetime = %s\n", q(f.Lifetime))
		}
	}
	if len(ps) > 0 {
		return "", ps
	}
	text := b.String()
	// Every other rule is the existing validator's job. A bad generator or bad input is caught here, before hubd sees the file.
	if _, vps := inventory.Parse([]byte(text)); len(vps) > 0 {
		return "", vps
	}
	return text, nil
}
