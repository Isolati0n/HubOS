package inventory

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func problemLines(ps []Problem) []string {
	var out []string
	for _, p := range ps {
		out = append(out, p.String())
	}
	return out
}

func mustLoad(t *testing.T, path string) *Inventory {
	t.Helper()
	inv, problems, err := Load(path)
	if err != nil {
		t.Fatalf("Load(%s): %v", path, err)
	}
	if len(problems) != 0 {
		t.Fatalf("%s should be valid, got:\n%s", path, strings.Join(problemLines(problems), "\n"))
	}
	return inv
}

func TestExampleAndFakeInventoriesAreValid(t *testing.T) {
	for _, path := range []string{
		"../../examples/inventory.example.toml",
		"../../testdata/inventory.fake.toml",
	} {
		inv := mustLoad(t, path)
		if inv.Format != 1 || len(inv.Machines) == 0 {
			t.Errorf("%s: unexpected result %+v", path, inv)
		}
	}
}

func TestFakeInventoryCoversEveryRole(t *testing.T) {
	inv := mustLoad(t, "../../testdata/inventory.fake.toml")
	seen := map[string]bool{}
	for _, m := range inv.Machines {
		seen[m.Role] = true
	}
	for _, r := range Roles {
		if !seen[r] {
			t.Errorf("testdata/inventory.fake.toml has no machine with role %q", r)
		}
	}
}

// Every file in testdata/broken must produce exactly the problems listed in
// its "# EXPECT:" lines, and every rule file must have at least one.
func TestBrokenFiles(t *testing.T) {
	paths, err := filepath.Glob("../../testdata/broken/*.toml")
	if err != nil || len(paths) == 0 {
		t.Fatalf("no broken files found: %v", err)
	}
	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			var want []string
			for _, line := range strings.Split(string(raw), "\n") {
				if rest, ok := strings.CutPrefix(line, "# EXPECT: "); ok {
					want = append(want, rest)
				}
			}
			if len(want) == 0 {
				t.Fatal("file has no # EXPECT: line")
			}
			_, problems, err := Load(path)
			if err != nil {
				t.Fatal(err)
			}
			if got := problemLines(problems); !reflect.DeepEqual(got, want) {
				t.Errorf("wrong messages\n got: %q\nwant: %q", got, want)
			}
		})
	}
}

func TestLoadMissingFile(t *testing.T) {
	_, _, err := Load(filepath.Join(t.TempDir(), "nope.toml"))
	if err != ErrNotFound {
		t.Errorf("got %v, want ErrNotFound", err)
	}
}

const validHub = `[[machine]]
id = "hub"
name = "Hub"
role = "hub"
address = "127.0.0.1"
open = ["none"]
home = { x = 0, y = 0 }
`

func TestFormatChecksStopEverythingElse(t *testing.T) {
	// A bad format must be the only message even if the rest is also wrong.
	_, ps := Parse([]byte("format = 2\n[[machine]]\nbogus = 1\n"))
	want := []string{"file: format version 2 is not supported; supported format versions: 1"}
	if got := problemLines(ps); !reflect.DeepEqual(got, want) {
		t.Errorf("got %q want %q", got, want)
	}

	for _, tc := range []struct{ name, line, found string }{
		{"text", `format = "1"`, `"1"`},
		{"zero", `format = 0`, `0`},
		{"decimal one", `format = 1.0`, `1.0 (a decimal number)`},
		{"decimal", `format = 1.5`, `1.5 (a decimal number)`},
		{"exponent", `format = 1e3`, `1000.0 (a decimal number)`},
	} {
		_, ps := Parse([]byte(tc.line + "\n" + validHub))
		if len(ps) != 1 || !strings.Contains(ps[0].Msg, "format version "+tc.found+" is not supported") {
			t.Errorf("%s: got %q", tc.name, problemLines(ps))
		}
	}
}

func TestUnknownFieldsAreAllReported(t *testing.T) {
	doc := "format = 1\nflavour = 3\n" + validHub + "colour = \"red\"\nhome2 = 1\n"
	_, ps := Parse([]byte(doc))
	got := problemLines(ps)
	want := []string{
		`file: line 2: unknown field "flavour"`,
		`file: line 10: unknown field "colour" in a [[machine]] block`,
		`file: line 11: unknown field "home2" in a [[machine]] block`,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got  %q\nwant %q", got, want)
	}
}

func TestUnknownFieldInsideHome(t *testing.T) {
	// For a field inside home = { ... } the library gives the path
	// "machine z" and leaves out "home", so the message names the field and
	// the line and cannot say which inline table it was in.
	doc := strings.Replace("format = 1\n"+validHub, "{ x = 0, y = 0 }", "{ x = 0, y = 0, z = 1 }", 1)
	_, ps := Parse([]byte(doc))
	want := []string{`file: line 8: unknown field "z" in a [[machine]] block`}
	if got := problemLines(ps); !reflect.DeepEqual(got, want) {
		t.Errorf("got %q want %q", got, want)
	}
}

func TestWrongKindOfValueIsReportedInPlainWords(t *testing.T) {
	for _, tc := range []struct{ from, to, want string }{
		{`id = "hub"`, `id = 5`, `file: line 3, column 6: "id" has the wrong kind of value: found TOML integer, expected text in quotes`},
		{`open = ["none"]`, `open = "none"`, `file: line 7, column 8: "open" has the wrong kind of value: found TOML string, expected a list of text`},
		{`home = { x = 0, y = 0 }`, `home = 3`, `file: line 8, column 8: "home" has the wrong kind of value: found TOML integer, expected a table like { x = 1, y = 2 }`},
	} {
		doc := "format = 1\n" + strings.Replace(validHub, tc.from, tc.to, 1)
		_, ps := Parse([]byte(doc))
		if got := problemLines(ps); !reflect.DeepEqual(got, []string{tc.want}) {
			t.Errorf("got  %q\nwant %q", got, tc.want)
		}
	}
}

func TestMachineWithoutIDIsNamedByPosition(t *testing.T) {
	doc := "format = 1\n" + validHub + "\n[[machine]]\nname = \"x\"\nrole = \"nas\"\naddress = \"nas.lan\"\nopen = [\"ssh\"]\nhome = { x = 1, y = 1 }\n"
	_, ps := Parse([]byte(doc))
	want := []string{`machine #2: required field "id" is missing or empty`}
	if got := problemLines(ps); !reflect.DeepEqual(got, want) {
		t.Errorf("got %q want %q", got, want)
	}
}

func TestSeveralProblemsAreAllReported(t *testing.T) {
	doc := `format = 1
[[machine]]
id = "a"
name = "A"
role = "nas"
address = "a.lan"
open = []
home = { x = 1, y = 1 }
[[machine]]
id = "a"
name = "B"
role = "nas"
address = "b.lan"
open = ["none", "ssh"]
home = { x = 1, y = 1 }
`
	_, ps := Parse([]byte(doc))
	// Order is by machine (file order), then file-wide problems last.
	want := []string{
		`a: open must not be empty`,
		`a: id is not unique: machine #1 has the same id`,
		`a: "none" must be the only entry in open (found: none, ssh)`,
		`a: home (x = 1, y = 1) is already used by a`,
		`file: no machine has role "hub"; exactly one is required`,
	}
	if got := problemLines(ps); !reflect.DeepEqual(got, want) {
		t.Errorf("got  %q\nwant %q", got, want)
	}
}

func TestOpenEmptyArrayIsNotTreatedAsMissing(t *testing.T) {
	_, ps := Parse([]byte("format = 1\n" + strings.Replace(validHub, `open = ["none"]`, `open = []`, 1)))
	want := []string{"hub: open must not be empty"}
	if got := problemLines(ps); !reflect.DeepEqual(got, want) {
		t.Errorf("got %q want %q", got, want)
	}
}

func TestHomeNeedsBothParts(t *testing.T) {
	_, ps := Parse([]byte("format = 1\n" + strings.Replace(validHub, `{ x = 0, y = 0 }`, `{ x = 0 }`, 1)))
	want := []string{"hub: home needs both x and y"}
	if got := problemLines(ps); !reflect.DeepEqual(got, want) {
		t.Errorf("got %q want %q", got, want)
	}
}

func TestIPv6AddressIsAllowed(t *testing.T) {
	_, ps := Parse([]byte("format = 1\n" + strings.Replace(validHub, `"127.0.0.1"`, `"::1"`, 1)))
	if len(ps) != 0 {
		t.Errorf("got %q", problemLines(ps))
	}
}

func TestControlCharactersAreRefusedInEveryTextField(t *testing.T) {
	for _, tc := range []struct{ field, from, to string }{
		{"id", `id = "hub"`, `id = "hu\tb"`},
		{"name", `name = "Hub"`, `name = "Hu\nb"`},
		{"address", `address = "127.0.0.1"`, `address = "host\n.lan"`},
		{"name", `name = "Hub"`, `name = "Hu b"`},
		{"name", `name = "Hub"`, `name = "Hu\u0007b"`},
	} {
		doc := "format = 1\n" + strings.Replace(validHub, tc.from, tc.to, 1)
		_, ps := Parse([]byte(doc))
		want := tc.field + " must not contain control characters or line breaks"
		if len(ps) != 1 || ps[0].Msg != want {
			t.Errorf("%s: got %q, want one problem %q", tc.to, problemLines(ps), want)
		}
	}
	for _, field := range []string{"user", "share"} {
		doc := "format = 1\n" + strings.Replace(validHub, `open = ["none"]`, `open = ["files"]`+"\n"+field+` = "a\nb"`, 1)
		_, ps := Parse([]byte(doc))
		found := false
		for _, p := range ps {
			found = found || p.Msg == field+" must not contain control characters or line breaks"
		}
		if !found {
			t.Errorf("%s: got %q", field, problemLines(ps))
		}
	}
}

// BenchmarkParse reads the file named by HUBOS_BENCH_INVENTORY (see
// docs/hubd-slice2.md) and measures reading plus checking it.
func BenchmarkParse(b *testing.B) {
	path := os.Getenv("HUBOS_BENCH_INVENTORY")
	if path == "" {
		b.Skip("set HUBOS_BENCH_INVENTORY to a generated inventory")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		b.Fatal(err)
	}
	b.SetBytes(int64(len(data)))
	for i := 0; i < b.N; i++ {
		if _, ps := Parse(data); len(ps) != 0 {
			b.Fatal(ps[0])
		}
	}
}

func TestLeadingDashRules(t *testing.T) {
	for _, tc := range []struct{ field, from, to, want string }{
		{"user", `open = ["none"]`, "open = [\"none\"]\nuser = \"-oBad\"", `hub: user "-oBad" must not start with a dash`},
		{"share", `open = ["none"]`, "open = [\"files\"]\nshare = \"-pool\"", `hub: share "-pool" must not start with a dash`},
		{"address", `address = "127.0.0.1"`, `address = "-x"`, `hub: address "-x" must not start with a dash`},
		{"id", `id = "hub"`, `id = "-hub"`, `-hub: id "-hub" must start with a letter or digit, not a dash`},
		{"id lone dash", `id = "hub"`, `id = "-"`, `-: id "-" must start with a letter or digit, not a dash`},
	} {
		doc := "format = 1\n" + strings.Replace(validHub, tc.from, tc.to, 1)
		_, ps := Parse([]byte(doc))
		if got := problemLines(ps); !reflect.DeepEqual(got, []string{tc.want}) {
			t.Errorf("%s: got %q want %q", tc.field, got, tc.want)
		}
	}
	// A dash inside, or an id that starts with a digit, is fine.
	for _, id := range []string{"a-b", "9-lives", "x-"} {
		doc := "format = 1\n" + strings.Replace(validHub, `id = "hub"`, `id = "`+id+`"`, 1)
		if _, ps := Parse([]byte(doc)); len(ps) != 0 {
			t.Errorf("id %q: %q", id, problemLines(ps))
		}
	}
}
