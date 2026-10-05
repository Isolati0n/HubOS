// conform: TEST PROTOTYPE of the conformance test for the proposed per-node "hubos-files" adapter.
// It runs ADAPTER (a program) many times against a fixture folder it builds itself (HUBOS_FILES_FIXTURE) and checks
// every answer.  It checks the adapter's input/output rules; it does NOT prove that a real GUI really reports what its
// user selected (that is the manual checklist in docs/proposals/file-transfer.md).
//
//	conform ADAPTER [ARGS...]      exit 0 only if every check passed
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

var (
	adapter []string
	fixture string
	failed  int
	passed  int
)

func pass(id, msg string) { passed++; fmt.Printf("PASS %-4s %s\n", id, msg) }
func fail(id, msg string) { failed++; fmt.Printf("FAIL %-4s %s\n", id, msg) }
func check(id string, ok bool, good, badmsg string) {
	if ok {
		pass(id, good)
	} else {
		fail(id, badmsg)
	}
}

type result struct {
	stdout, stderr []byte
	code           int
	took           time.Duration
	timedOut       bool
}

func run(stdin string, args ...string) result {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, adapter[0], append(append([]string{}, adapter[1:]...), args...)...)
	cmd.Env = append(os.Environ(), "HUBOS_FILES_FIXTURE="+fixture)
	var so, se bytes.Buffer
	cmd.Stdout, cmd.Stderr = &so, &se
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}
	t0 := time.Now()
	err := cmd.Run()
	r := result{so.Bytes(), se.Bytes(), 0, time.Since(t0), ctx.Err() != nil}
	if ee, ok := err.(*exec.ExitError); ok {
		r.code = ee.ExitCode()
	} else if err != nil {
		r.code = -1
	}
	return r
}

type item struct {
	Path  string `json:"path"`
	Name  string `json:"name"`
	Kind  string `json:"kind"`
	Bytes int64  `json:"bytes"`
}

func mk(p, content string) {
	os.MkdirAll(filepath.Dir(p), 0o755)
	os.WriteFile(p, []byte(content), 0o644)
}

func treeHash(root string) string {
	h := sha256.New()
	var names []string
	filepath.Walk(root, func(p string, i os.FileInfo, err error) error {
		if err == nil {
			names = append(names, fmt.Sprintf("%s|%d|%d|%s", p, i.Size(), i.ModTime().UnixNano(), i.Mode()))
		}
		return nil
	})
	sort.Strings(names)
	for _, n := range names {
		io.WriteString(h, n+"\n")
	}
	return hex.EncodeToString(h.Sum(nil))
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: conform ADAPTER [ARGS...]")
		os.Exit(2)
	}
	adapter = os.Args[1:]
	var err error
	fixture, err = os.MkdirTemp("", "hubos-files-fixture-")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(fixture)
	sel := filepath.Join(fixture, "selection")
	dest := filepath.Join(fixture, "destination")
	sub := filepath.Join(fixture, "dest-sub")
	os.MkdirAll(dest, 0o755)
	os.MkdirAll(sub, 0o755)

	// C1 capabilities
	r := run("", "capabilities")
	var caps struct {
		API   int      `json:"api"`
		Verbs []string `json:"verbs"`
	}
	ok := r.code == 0 && json.Unmarshal(r.stdout, &caps) == nil && caps.API == 1
	check("C1", ok, "capabilities: exit 0, one JSON document, api 1", fmt.Sprintf("capabilities: exit %d, stdout %q", r.code, trunc(r.stdout)))
	has := func(v string) bool {
		for _, x := range caps.Verbs {
			if x == v {
				return true
			}
		}
		return false
	}
	check("C1b", has("selection") && has("destination"), "capabilities lists the two required verbs", "capabilities does not list selection and destination")

	// C2 empty selection
	os.MkdirAll(sel, 0o755)
	r = run("", "selection")
	var sr struct {
		Items []item `json:"items"`
	}
	var raw map[string]json.RawMessage
	okj := json.Unmarshal(r.stdout, &raw) == nil
	check("C2", r.code == 0 && okj && string(raw["items"]) == "[]", "empty selection: exit 0 and \"items\": []", fmt.Sprintf("empty selection: exit %d, stdout %q (items must be [], not null or missing)", r.code, trunc(r.stdout)))

	// C3 awkward names
	mk(filepath.Join(sel, "plain.txt"), "hello\n")
	mk(filepath.Join(sel, "my file (1) ünï 日本.txt"), "uni")
	mk(filepath.Join(sel, "-leading dash.txt"), "dash")
	mk(filepath.Join(sel, "new\nline.txt"), "nl")
	mk(filepath.Join(sel, "empty"), "")
	mk(filepath.Join(sel, "proj", "sub", "a.txt"), "aaaa")
	mk(filepath.Join(sel, "proj", "b.txt"), "bb")
	os.Symlink("/etc/hostname", filepath.Join(sel, "link-out"))
	before := treeHash(fixture)
	r = run("", "selection")
	sr.Items = nil
	err = json.Unmarshal(r.stdout, &sr)
	check("C3a", r.code == 0 && err == nil, "selection: exit 0 and the output is one JSON document", fmt.Sprintf("selection: exit %d, parse error %v, stdout %q", r.code, err, trunc(r.stdout)))
	problems := []string{}
	seen := map[string]bool{}
	for _, it := range sr.Items {
		seen[it.Name] = true
		if !filepath.IsAbs(it.Path) {
			problems = append(problems, "path not absolute: "+q(it.Path))
			continue
		}
		if filepath.Clean(it.Path) != it.Path {
			problems = append(problems, "path not clean (.. or // inside): "+q(it.Path))
		}
		if !utf8.ValidString(it.Path) || !utf8.ValidString(it.Name) {
			problems = append(problems, "not valid UTF-8: "+q(it.Path))
			continue
		}
		if filepath.Base(it.Path) != it.Name {
			problems = append(problems, "name is not the last part of path: "+q(it.Name))
		}
		li, e := os.Lstat(it.Path)
		if e != nil {
			problems = append(problems, "does not exist: "+q(it.Path))
			continue
		}
		if li.Mode()&os.ModeSymlink != 0 {
			problems = append(problems, "is a symbolic link but listed as "+it.Kind+": "+q(it.Path))
			continue
		}
		switch it.Kind {
		case "file":
			if !li.Mode().IsRegular() {
				problems = append(problems, "kind file but not a regular file: "+q(it.Path))
			} else if it.Bytes != li.Size() {
				problems = append(problems, fmt.Sprintf("bytes %d != real size %d: %s", it.Bytes, li.Size(), q(it.Path)))
			}
		case "dir":
			if !li.IsDir() {
				problems = append(problems, "kind dir but not a directory: "+q(it.Path))
			}
		default:
			problems = append(problems, "kind must be file or dir, got "+q(it.Kind))
		}
		if !strings.HasPrefix(it.Path, sel+"/") {
			problems = append(problems, "outside the selection folder: "+q(it.Path))
		}
	}
	check("C3b", len(problems) == 0, fmt.Sprintf("selection: %d items, all absolute, clean, UTF-8, existing, right kind and size", len(sr.Items)), "selection problems: "+strings.Join(problems, "; "))
	want := []string{"plain.txt", "my file (1) ünï 日本.txt", "-leading dash.txt", "new\nline.txt", "empty", "proj"}
	missing := []string{}
	for _, w := range want {
		if !seen[w] {
			missing = append(missing, q(w))
		}
	}
	check("C3c", len(missing) == 0, "selection lists every regular file and folder of the fixture (and no link)", "selection misses: "+strings.Join(missing, ", "))
	check("C3d", !seen["link-out"], "a symbolic link is not offered", "a symbolic link (to /etc/hostname) was offered")
	check("C4", json.Valid(bytes.TrimSpace(r.stdout)) && bytes.Count(bytes.TrimSpace(r.stdout), []byte("\n")) == 0, "standard output is exactly one JSON line (no extra text)", fmt.Sprintf("standard output is not exactly one JSON line: %q", trunc(r.stdout)))
	check("C5", r.took < 2*time.Second && !r.timedOut, fmt.Sprintf("answered in %d ms (limit 2000)", r.took.Milliseconds()), fmt.Sprintf("too slow: %v (limit 2 s)", r.took))
	r2 := run("", "selection")
	check("C6", bytes.Equal(r.stdout, r2.stdout), "two calls give the same answer", "two calls gave different answers")
	check("C7", treeHash(fixture) == before, "selection changed nothing on disk", "selection changed something in the fixture folder")

	// C8 destination
	r = run("", "destination")
	var dr struct {
		Dir string `json:"dir"`
	}
	err = json.Unmarshal(r.stdout, &dr)
	st, e := os.Stat(dr.Dir)
	check("C8", r.code == 0 && err == nil && filepath.IsAbs(dr.Dir) && e == nil && st.IsDir(), "destination: exit 0, an absolute path of an existing folder", fmt.Sprintf("destination: exit %d, stdout %q (must be an absolute existing folder)", r.code, trunc(r.stdout)))
	check("C9", treeHash(fixture) == before, "destination changed nothing on disk", "destination changed something in the fixture folder")
	// no folder open
	os.WriteFile(filepath.Join(fixture, "no-destination"), nil, 0o644)
	r = run("", "destination")
	dr.Dir = "x"
	err = json.Unmarshal(r.stdout, &dr)
	check("C10", r.code == 0 && err == nil && dr.Dir == "", "no folder open: exit 0 and \"dir\": \"\" (an answer, not an error)", fmt.Sprintf("no folder open: exit %d, stdout %q", r.code, trunc(r.stdout)))
	os.Remove(filepath.Join(fixture, "no-destination"))

	// C11 destination-at
	if has("destination-at") {
		rects, _ := json.Marshal([]map[string]any{{"x0": 100, "y0": 100, "x1": 300, "y1": 300, "dir": sub}})
		os.WriteFile(filepath.Join(fixture, "destination-at.json"), rects, 0o644)
		in := run("", "destination", "200", "200")
		out := run("", "destination", "900", "900")
		var a, b struct {
			Dir string `json:"dir"`
		}
		json.Unmarshal(in.stdout, &a)
		json.Unmarshal(out.stdout, &b)
		check("C11", a.Dir == sub && b.Dir != sub && b.Dir != "", "destination at a point: inside the rectangle gives that folder, outside gives the current folder", fmt.Sprintf("destination at a point: inside gave %q (want %q), outside gave %q", a.Dir, sub, b.Dir))
	} else {
		pass("C11", "destination-at not advertised: skipped")
	}

	// C12 refresh
	if has("refresh") {
		r = run(`{"paths":["/tmp/x"]}`, "refresh")
		var rr struct {
			OK bool `json:"ok"`
		}
		err = json.Unmarshal(r.stdout, &rr)
		check("C12", r.code == 0 && err == nil && rr.OK, "refresh: exit 0 and {\"ok\":true}", fmt.Sprintf("refresh: exit %d, stdout %q", r.code, trunc(r.stdout)))
	} else {
		pass("C12", "refresh not advertised: skipped")
	}

	// C13 unknown verb
	r = run("", "frobnicate")
	var er struct {
		Code string `json:"code"`
	}
	err = json.Unmarshal(r.stdout, &er)
	check("C13", r.code == 2 && err == nil && er.Code != "", "unknown verb: exit 2 and a JSON {\"code\", \"error\"}", fmt.Sprintf("unknown verb: exit %d, stdout %q (want exit 2 and a JSON error)", r.code, trunc(r.stdout)))

	// C14 five at once
	var wg sync.WaitGroup
	bads := 0
	var mu sync.Mutex
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			x := run("", "selection")
			mu.Lock()
			if x.code != 0 || !json.Valid(bytes.TrimSpace(x.stdout)) {
				bads++
			}
			mu.Unlock()
		}()
	}
	wg.Wait()
	check("C14", bads == 0, "five calls at once all answered", fmt.Sprintf("%d of 5 parallel calls failed", bads))

	fmt.Printf("\n%d passed, %d failed\n", passed, failed)
	if failed > 0 {
		os.Exit(1)
	}
}

func q(s string) string { return fmt.Sprintf("%q", s) }
func trunc(b []byte) string {
	s := string(b)
	if len(s) > 120 {
		s = s[:120] + "..."
	}
	return s
}
