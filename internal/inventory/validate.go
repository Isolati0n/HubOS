package inventory

import (
	"fmt"
	"net"
	"regexp"
	"slices"
	"strings"
	"unicode"
)

var idPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

// idCharsPattern is the old character rule, kept so a wrong character and a
// leading dash get different messages.
var idCharsPattern = regexp.MustCompile(`^[a-z0-9-]+$`)

// label names a machine in messages: its id, or its position if it has none.
func label(i int, m Machine) string {
	if m.ID != "" {
		return m.ID
	}
	return fmt.Sprintf("machine #%d", i+1)
}

// hasControl reports whether s has a control character (this includes tabs
// and line breaks) or the Unicode line and paragraph separators.
func hasControl(s string) bool {
	for _, r := range s {
		if unicode.IsControl(r) || r == '\u2028' || r == '\u2029' {
			return true
		}
	}
	return false
}

func quoteAll(items []string) string {
	return strings.Join(items, ", ")
}

// validate checks every rule in docs/inventory-format.md and returns all the
// problems it finds, machines first (in file order), then file-wide ones.
func validate(inv *Inventory) []Problem {
	var ps []Problem
	add := func(where, format string, args ...any) {
		ps = append(ps, Problem{where, fmt.Sprintf(format, args...)})
	}

	// First pass: who has which id and role, so host checks can look
	// forward and backward. The first machine with an id wins a lookup.
	byID := map[string]int{}
	for i, m := range inv.Machines {
		if _, seen := byID[m.ID]; m.ID != "" && !seen {
			byID[m.ID] = i
		}
	}
	homes := map[[2]int]int{}

	for i, m := range inv.Machines {
		w := label(i, m)

		// No control characters or line breaks in text that reaches the
		// panel, the logs and viewer command lines.
		for _, f := range []struct{ name, val string }{
			{"id", m.ID}, {"name", m.Name}, {"user", m.User}, {"share", m.Share}, {"session", m.Session}, {"address", m.Address},
		} {
			if hasControl(f.val) {
				add(w, "%s must not contain control characters or line breaks", f.name)
			}
		}

		// A text that starts with a dash could be read as an option by a
		// program it is passed to.
		for _, f := range []struct{ name, val string }{{"address", m.Address}, {"user", m.User}, {"share", m.Share}, {"session", m.Session}} {
			if strings.HasPrefix(f.val, "-") {
				add(w, "%s %q must not start with a dash", f.name, f.val)
			}
		}

		// id: present, well-formed, unique.
		if m.ID == "" {
			add(w, `required field "id" is missing or empty`)
		} else {
			switch {
			case hasControl(m.ID):
				// already reported above
			case !idCharsPattern.MatchString(m.ID):
				add(w, "id %q must use only lowercase letters, digits and dashes", m.ID)
			case !idPattern.MatchString(m.ID):
				add(w, "id %q must start with a letter or digit, not a dash", m.ID)
			}
			if first := byID[m.ID]; first != i {
				add(w, "id is not unique: machine #%d has the same id", first+1)
			}
		}

		// Required fields.
		if m.Name == "" {
			add(w, `required field "name" is missing or empty`)
		}
		if m.Role == "" {
			add(w, `required field "role" is missing or empty`)
		} else if !slices.Contains(Roles, m.Role) {
			add(w, "role %q is not one of: %s", m.Role, quoteAll(Roles))
		}
		if m.Address == "" {
			add(w, `required field "address" is missing or empty`)
		} else if net.ParseIP(m.Address) == nil && !hasControl(m.Address) && strings.ContainsAny(m.Address, ": /") {
			add(w, `address %q is not a plain name or IP address (no port, slash or spaces; the port goes in "port")`, m.Address)
		}

		// open: required, not empty, known programs, "none" only alone.
		switch {
		case m.Open == nil:
			add(w, `required field "open" is missing`)
		case len(m.Open) == 0:
			add(w, "open must not be empty")
		default:
			for _, p := range m.Open {
				if !slices.Contains(Programs, p) {
					add(w, "open has %q, which is not one of: %s", p, quoteAll(Programs))
				}
			}
			if slices.Contains(m.Open, "none") && len(m.Open) > 1 {
				add(w, `"none" must be the only entry in open (found: %s)`, quoteAll(m.Open))
			}
		}

		// share only with "files".
		if m.Share != "" && !slices.Contains(m.Open, "files") {
			add(w, `share is set but open does not include "files"`)
		}

		// session only with "moonlight".
		if m.Session != "" && !slices.Contains(m.Open, "moonlight") {
			add(w, `session is set but open does not include "moonlight"`)
		}

		// home: present, both parts, not shared with another machine.
		switch {
		case m.Home == nil:
			add(w, `required field "home" is missing`)
		case m.Home.X == nil || m.Home.Y == nil:
			add(w, "home needs both x and y")
		default:
			key := [2]int{*m.Home.X, *m.Home.Y}
			if first, taken := homes[key]; taken {
				add(w, "home (x = %d, y = %d) is already used by %s", key[0], key[1], label(first, inv.Machines[first]))
			} else {
				homes[key] = i
			}
		}

		// port range.
		if m.Port != nil && (*m.Port < 1 || *m.Port > 65535) {
			add(w, "port %d is out of range (1 to 65535)", *m.Port)
		}

		// guests need host and lifetime; everyone else must not have them.
		switch {
		case m.Role == "guest":
			if m.Host == "" {
				add(w, `required field "host" is missing or empty (a guest must name its vm-host)`)
			} else if hostIdx, ok := byID[m.Host]; !ok {
				add(w, "host %q does not exist", m.Host)
			} else if role := inv.Machines[hostIdx].Role; role != "vm-host" {
				add(w, `host %q has role %q, but a guest's host must have role "vm-host"`, m.Host, role)
			}
			if m.Lifetime == "" {
				add(w, `required field "lifetime" is missing or empty`)
			} else if !slices.Contains(Lifetimes, m.Lifetime) {
				add(w, "lifetime %q is not one of: %s", m.Lifetime, quoteAll(Lifetimes))
			}
		case m.Role != "":
			if m.Host != "" {
				add(w, "host is only allowed on a guest (this machine's role is %q)", m.Role)
			}
			if m.Lifetime != "" {
				add(w, "lifetime is only allowed on a guest (this machine's role is %q)", m.Role)
			}
		}
	}

	// Exactly one hub.
	var hubs []string
	for i, m := range inv.Machines {
		if m.Role == "hub" {
			hubs = append(hubs, label(i, m))
		}
	}
	switch {
	case len(hubs) == 0:
		add("file", `no machine has role "hub"; exactly one is required`)
	case len(hubs) > 1:
		add("file", `more than one machine has role "hub" (%s); exactly one is required`, quoteAll(hubs))
	}

	return ps
}
