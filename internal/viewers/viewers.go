// Package viewers reads viewers.toml, the table that says which program
// hubd starts for each kind of machine ("open" entry in the inventory).
//
// Commands are lists of arguments, never one string for a shell. Text from
// the inventory is put into a single argument and nothing else; it is never
// pasted into a command string. The table lives next to the inventory, on the
// hub's own disk, and never holds secrets.
package viewers

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	toml "github.com/pelletier/go-toml/v2"

	"hubos/internal/inventory"
)

// SupportedFormats lists the "format" versions of viewers.toml this hubd reads.
var SupportedFormats = []int64{1}

// AppIDPrefix starts every window name hubd chooses: hubos-<machine id>.
const AppIDPrefix = "hubos-"

// Viewer is one [[viewer]] block.
type Viewer struct {
	ID       string   `toml:"id"`
	Programs []string `toml:"programs"`  // inventory "open" entries this viewer serves
	Command  []string `toml:"command"`   // program, then arguments, with {placeholders}
	SetsName bool     `toml:"sets_name"` // true if the command makes the window carry {app_id}
	// WaitText is window_wait as written ("10s"): how long hubd waits for the
	// window before it starts the late-window grace period. Empty means the
	// hub's default (10 s).
	WaitText string `toml:"window_wait"`
	// WindowWait is WaitText parsed (0 when not given).
	WindowWait time.Duration `toml:"-"`
	// GraceText is late_grace as written ("90s"): how long hubd keeps waiting
	// for the window after the window wait ran out. Empty means the hub's
	// --late-grace.
	GraceText string `toml:"late_grace"`
	// LateGrace is GraceText parsed (0 when not given).
	LateGrace time.Duration `toml:"-"`
}

// Table is a valid viewers.toml.
type Table struct {
	Viewers []Viewer
}

type file struct {
	Format any      `toml:"format"`
	Viewer []Viewer `toml:"viewer"`
}

var placeholders = []string{"id", "name", "address", "port", "user", "share", "app_id", "title"}

// ErrNotFound is returned by Load when the file does not exist.
var ErrNotFound = errors.New("viewers file not found")

// Load reads and checks the file. Problems with the contents come back as
// plain-words messages; the error is only for a file that cannot be read.
func Load(path string) (*Table, []string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil, ErrNotFound
		}
		return nil, nil, err
	}
	t, problems := Parse(data)
	return t, problems, nil
}

// Parse checks the contents of a viewers file.
func Parse(data []byte) (*Table, []string) {
	var f file
	dec := toml.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&f); err != nil {
		var strict *toml.StrictMissingError
		if errors.As(err, &strict) {
			var ps []string
			for _, e := range strict.Errors {
				row, _ := e.Position()
				ps = append(ps, fmt.Sprintf("line %d: unknown field %q", row, strings.Join(e.Key(), ".")))
			}
			return nil, ps
		}
		return nil, []string{err.Error()}
	}
	n, ok := f.Format.(int64)
	if f.Format == nil || !ok || !slices.Contains(SupportedFormats, n) {
		return nil, []string{fmt.Sprintf("format version %v is not supported (or the format line is missing); supported: 1", f.Format)}
	}

	var ps []string
	seenID := map[string]bool{}
	seenProg := map[string]string{}
	for i, v := range f.Viewer {
		w := fmt.Sprintf("viewer #%d", i+1)
		if v.ID != "" {
			w = "viewer " + strconv.Quote(v.ID)
		}
		if v.ID == "" {
			ps = append(ps, w+`: required field "id" is missing or empty`)
		} else if seenID[v.ID] {
			ps = append(ps, w+": id is not unique")
		}
		seenID[v.ID] = true
		if len(v.Programs) == 0 {
			ps = append(ps, w+": programs must not be empty")
		}
		for _, p := range v.Programs {
			switch {
			case !slices.Contains(inventory.Programs, p) || p == "none":
				ps = append(ps, fmt.Sprintf("%s: %q is not a program that can be opened (one of: %s)", w, p, strings.Join(slices.DeleteFunc(slices.Clone(inventory.Programs), func(s string) bool { return s == "none" }), ", ")))
			case seenProg[p] != "":
				ps = append(ps, fmt.Sprintf("%s: program %q is already served by viewer %q", w, p, seenProg[p]))
			default:
				seenProg[p] = v.ID
			}
		}
		if len(v.Command) == 0 || v.Command[0] == "" {
			ps = append(ps, w+": command must name a program as its first entry")
			continue
		}
		if strings.Contains(v.Command[0], "{") {
			ps = append(ps, w+": the program name (first entry of command) must be fixed text, without {placeholders}")
		}
		if v.WaitText != "" {
			d, err := time.ParseDuration(v.WaitText)
			if err != nil || d <= 0 {
				ps = append(ps, fmt.Sprintf(`%s: window_wait %q must be a positive duration like "10s" or "1m30s"`, w, v.WaitText))
			} else {
				f.Viewer[i].WindowWait = d
			}
		}
		if v.GraceText != "" {
			d, err := time.ParseDuration(v.GraceText)
			if err != nil || d <= 0 {
				ps = append(ps, fmt.Sprintf(`%s: late_grace %q must be a positive duration like "60s" or "2m"`, w, v.GraceText))
			} else {
				f.Viewer[i].LateGrace = d
			}
		}
		usesAppID := false
		for _, arg := range v.Command {
			names, bad := scan(arg)
			if bad != "" {
				ps = append(ps, fmt.Sprintf("%s: %s in %q", w, bad, arg))
			}
			if slices.Contains(names, "app_id") {
				usesAppID = true
			}
		}
		if v.SetsName && !usesAppID {
			ps = append(ps, w+": sets_name is true but command never uses {app_id}")
		}
	}
	if len(ps) > 0 {
		return nil, ps
	}
	return &Table{Viewers: f.Viewer}, nil
}

// scan returns the placeholder names in one argument, and a problem text if
// the argument has an unknown or unfinished placeholder.
func scan(arg string) (names []string, problem string) {
	for i := 0; i < len(arg); i++ {
		switch arg[i] {
		case '{':
			j := strings.IndexByte(arg[i:], '}')
			if j < 0 {
				return names, "an opening { with no closing }"
			}
			name := arg[i+1 : i+j]
			if !slices.Contains(placeholders, name) {
				return names, fmt.Sprintf("unknown placeholder {%s} (known: %s)", name, strings.Join(placeholders, ", "))
			}
			names = append(names, name)
			i += j
		case '}':
			return names, "a } with no opening {"
		}
	}
	return names, ""
}

// For returns the viewer that serves a program, or nil.
func (t *Table) For(program string) *Viewer {
	for i := range t.Viewers {
		if slices.Contains(t.Viewers[i].Programs, program) {
			return &t.Viewers[i]
		}
	}
	return nil
}

// AppID is the window name hubd asks a viewer to use for a machine.
func AppID(machineID string) string { return AppIDPrefix + machineID }

// Args builds the argument list for a machine. Each entry of the command
// becomes exactly one argument. A placeholder for a value the machine does
// not have ({port}, {user}, {share}) is an error, so hubd never guesses.
func (v *Viewer) Args(m inventory.Machine) ([]string, error) {
	values := map[string]string{
		"id": m.ID, "name": m.Name, "address": m.Address,
		"app_id": AppID(m.ID), "title": m.Name,
	}
	if m.Port != nil {
		values["port"] = strconv.Itoa(*m.Port)
	}
	if m.User != "" {
		values["user"] = m.User
	}
	if m.Share != "" {
		values["share"] = m.Share
	}
	args := make([]string, 0, len(v.Command))
	for _, tmpl := range v.Command {
		var out strings.Builder
		for i := 0; i < len(tmpl); i++ {
			if tmpl[i] != '{' {
				out.WriteByte(tmpl[i])
				continue
			}
			j := strings.IndexByte(tmpl[i:], '}')
			if j < 0 {
				return nil, fmt.Errorf("viewer %q has an unfinished placeholder", v.ID)
			}
			name := tmpl[i+1 : i+j]
			val, ok := values[name]
			if !ok {
				return nil, &MissingError{Machine: m.ID, Field: name}
			}
			out.WriteString(val)
			i += j
		}
		args = append(args, out.String())
	}
	return args, nil
}

// MissingError says a machine lacks a value the command needs.
type MissingError struct{ Machine, Field string }

func (e *MissingError) Error() string {
	return fmt.Sprintf("%s has no %s in the inventory, and its viewer command needs one", e.Machine, e.Field)
}
