// Package inventory reads the inventory file and checks it against every
// rule in docs/inventory-format.md. It never touches the network.
package inventory

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	toml "github.com/pelletier/go-toml/v2"
)

// SupportedFormats lists the "format" versions this hubd can read.
var SupportedFormats = []int{1}

// Allowed values, straight from the field table in docs/inventory-format.md.
var (
	Roles     = []string{"hub", "gaming", "ai", "desktop", "nas", "backup-nas", "vm-host", "guest"}
	Programs  = []string{"moonlight", "spice", "vnc", "ssh", "files", "none"}
	Lifetimes = []string{"ephemeral", "persistent"}
)

// Point is a machine's fixed spot on the canvas. Both parts are pointers so
// a missing x or y can be told apart from zero.
type Point struct {
	X *int `toml:"x"`
	Y *int `toml:"y"`
}

// Machine is one [[machine]] block. Optional or checked-for-presence fields
// are pointers or nil-able so "missing" can be told apart from "zero".
//
// There is deliberately no field for secrets. Secrets never appear in the
// inventory (docs/inventory-format.md). hubd cannot recognise a secret by
// looking at text, so that rule is not enforced here; the only protection is
// that the format has no field to put one in, and unknown fields are errors.
type Machine struct {
	ID       string   `toml:"id"`
	Name     string   `toml:"name"`
	Role     string   `toml:"role"`
	Address  string   `toml:"address"`
	Open     []string `toml:"open"`
	Home     *Point   `toml:"home"`
	Port     *int     `toml:"port"`
	User     string   `toml:"user"`
	Share    string   `toml:"share"`
	Session  string   `toml:"session"` // the Sunshine app to stream (moonlight only)
	Host     string   `toml:"host"`
	Lifetime string   `toml:"lifetime"`
}

// Inventory is a file that passed (or is being checked against) the rules.
type Inventory struct {
	Format   int
	Machines []Machine
}

// Problem is one thing wrong with the file. Where is a machine id, a
// position like "machine #3" when the id is missing, or "file".
type Problem struct {
	Where string
	Msg   string
}

func (p Problem) String() string { return p.Where + ": " + p.Msg }

// ErrNotFound is returned by Load when the file does not exist.
var ErrNotFound = errors.New("inventory file not found")

// Load reads the file at path and checks it. The error is non-nil only when
// the file could not be read; problems with the contents come back as
// Problems.
func Load(path string) (*Inventory, []Problem, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil, ErrNotFound
		}
		return nil, nil, err
	}
	inv, problems := Parse(data)
	return inv, problems, nil
}

// file is the shape of the whole TOML document.
type file struct {
	Format  any       `toml:"format"`
	Machine []Machine `toml:"machine"`
}

// Parse checks the contents of an inventory file.
//
// Order matters: the format version is checked first and stops everything
// else, because a file in another version may mean something different.
// After that, unknown fields are reported (all of them; the library lists
// every one) and checking stops, as the format doc says. Only then are the
// rules checked, and every problem is collected, not just the first.
func Parse(data []byte) (*Inventory, []Problem) {
	var head struct {
		Format any `toml:"format"`
	}
	if err := toml.Unmarshal(data, &head); err != nil {
		return nil, []Problem{decodeProblem(err)}
	}
	if p := checkFormat(head.Format); p != nil {
		return nil, []Problem{*p}
	}

	var f file
	dec := toml.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&f); err != nil {
		var strict *toml.StrictMissingError
		if errors.As(err, &strict) {
			return nil, unknownFieldProblems(strict)
		}
		return nil, []Problem{decodeProblem(err)}
	}

	inv := &Inventory{Format: SupportedFormats[0], Machines: f.Machine}
	return inv, validate(inv)
}

func checkFormat(v any) *Problem {
	supported := make([]string, len(SupportedFormats))
	for i, n := range SupportedFormats {
		supported[i] = strconv.Itoa(n)
	}
	list := strings.Join(supported, ", ")

	if v == nil {
		return &Problem{"file", fmt.Sprintf(`there is no "format" line; supported format versions: %s`, list)}
	}
	if n, ok := v.(int64); ok {
		for _, s := range SupportedFormats {
			if int64(s) == n {
				return nil
			}
		}
	}
	found := fmt.Sprint(v)
	switch x := v.(type) {
	case string:
		found = strconv.Quote(x)
	case float64:
		found = decimalText(x) + " (a decimal number)"
	}
	return &Problem{"file", fmt.Sprintf("format version %s is not supported; supported format versions: %s", found, list)}
}

// decimalText writes a decimal number so it still looks like one: 1 becomes
// "1.0", 1.5 stays "1.5". Infinity and not-a-number are left as they are.
func decimalText(f float64) string {
	t := strconv.FormatFloat(f, 'f', -1, 64)
	if !strings.ContainsAny(t, ".IN") {
		t += ".0"
	}
	return t
}

// wrongKind matches the library's message for a value of the wrong type, so
// the Go struct names in it can be replaced by plain words.
var wrongKind = regexp.MustCompile(`^cannot decode TOML (.+) into struct field \S+ of type (.+)$`)

func decodeProblem(err error) Problem {
	var de *toml.DecodeError
	if !errors.As(err, &de) {
		return Problem{"file", err.Error()}
	}
	row, col := de.Position()
	msg := strings.TrimPrefix(de.Error(), "toml: ")
	if m := wrongKind.FindStringSubmatch(msg); m != nil {
		key := []string(de.Key())
		if len(key) > 1 && key[0] == "machine" {
			key = key[1:]
		}
		want := "another kind of value"
		switch m[2] {
		case "string":
			want = "text in quotes"
		case "int":
			want = "a whole number"
		case "[]string":
			want = "a list of text"
		default:
			if strings.Contains(m[2], ".") {
				want = "a table like { x = 1, y = 2 }"
			}
		}
		msg = fmt.Sprintf("%q has the wrong kind of value: found TOML %s, expected %s", strings.Join(key, "."), m[1], want)
	}
	return Problem{"file", fmt.Sprintf("line %d, column %d: %s", row, col, msg)}
}

// unknownFieldProblems turns the library's list of unknown fields into
// problems. The library names every unknown field, not only the first. For a
// field inside a [[machine]] block it gives the path "machine <field>"; for a
// field inside an inline table such as home = { ... } it gives the same kind
// of path and leaves out "home", so the message cannot say which table.
func unknownFieldProblems(strict *toml.StrictMissingError) []Problem {
	var ps []Problem
	for _, e := range strict.Errors {
		row, _ := e.Position()
		key := []string(e.Key())
		where := ""
		if len(key) > 1 && key[0] == "machine" {
			key = key[1:]
			where = " in a [[machine]] block"
		}
		ps = append(ps, Problem{"file", fmt.Sprintf("line %d: unknown field %q%s", row, strings.Join(key, "."), where)})
	}
	return ps
}
