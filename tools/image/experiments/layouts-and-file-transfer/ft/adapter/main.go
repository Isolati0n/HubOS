// adapter: REFERENCE implementation of the proposed per-node "hubos-files" adapter, in fixture mode only (it answers from a
// folder named by HUBOS_FILES_FIXTURE instead of from a real GUI), with deliberate faults that the conformance test must
// catch (BAD=<fault>).  TEST PROTOTYPE, not the production feature.
//
//	adapter capabilities
//	adapter selection
//	adapter destination [X Y]
//	adapter refresh            (JSON on standard input: {"paths":[...]})
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"
)

type item struct {
	Path  string `json:"path"`
	Name  string `json:"name"`
	Kind  string `json:"kind"`
	Bytes int64  `json:"bytes"`
}

type rect struct {
	X0, Y0, X1, Y1 int
	Dir            string
}

var bad = os.Getenv("BAD")

func out(v any) {
	if bad == "noise" {
		fmt.Println("debug: starting up")
	}
	b, _ := json.Marshal(v)
	fmt.Println(string(b))
	if bad == "exitcode" {
		os.Exit(3)
	}
}

func fail(code int, c, msg string) {
	b, _ := json.Marshal(map[string]string{"code": c, "error": msg})
	fmt.Println(string(b))
	os.Exit(code)
}

func dirSize(p string) int64 {
	var n int64
	filepath.Walk(p, func(_ string, i os.FileInfo, err error) error {
		if err == nil && i.Mode().IsRegular() {
			n += i.Size()
		}
		return nil
	})
	return n
}

func main() {
	fx := os.Getenv("HUBOS_FILES_FIXTURE")
	if fx == "" {
		fail(1, "no_fixture", "this reference adapter works only in fixture mode")
	}
	if len(os.Args) < 2 {
		fail(2, "usage", "verb missing")
	}
	if bad == "slow" {
		time.Sleep(5 * time.Second)
	}
	switch os.Args[1] {
	case "capabilities":
		out(map[string]any{"api": 1, "verbs": []string{"selection", "destination", "destination-at", "refresh"}})
	case "selection":
		sel := filepath.Join(fx, "selection")
		ents, _ := os.ReadDir(sel)
		names := []string{}
		for _, e := range ents {
			names = append(names, e.Name())
		}
		sort.Strings(names)
		var items []item
		for _, n := range names {
			p := filepath.Join(sel, n)
			li, err := os.Lstat(p)
			if err != nil {
				continue
			}
			if li.Mode()&os.ModeSymlink != 0 && bad != "symleak" {
				continue // symlinks are not offered
			}
			it := item{Path: p, Name: n}
			st, err := os.Stat(p)
			if err != nil {
				continue
			}
			if st.IsDir() {
				it.Kind, it.Bytes = "dir", dirSize(p)
			} else {
				it.Kind, it.Bytes = "file", st.Size()
			}
			items = append(items, it)
		}
		switch bad {
		case "relpath":
			for i := range items {
				items[i].Path = filepath.Join("selection", items[i].Name)
			}
		case "dotdot":
			for i := range items {
				items[i].Path = sel + "/../selection/" + items[i].Name
			}
		case "missing":
			items = append(items, item{Path: filepath.Join(sel, "ghost.txt"), Name: "ghost.txt", Kind: "file", Bytes: 1})
		case "bytes":
			for i := range items {
				if items[i].Kind == "file" {
					items[i].Bytes++
				}
			}
		case "nonutf8":
			if len(items) > 0 {
				items[0].Path = sel + "/\xff\xfe"
				items[0].Name = "\xff\xfe"
			}
		case "sideeffect":
			os.WriteFile(filepath.Join(sel, ".lastused"), []byte("x"), 0o644)
		}
		if items == nil && bad != "nullitems" {
			items = []item{}
		}
		out(map[string]any{"items": items})
	case "destination":
		if _, err := os.Stat(filepath.Join(fx, "no-destination")); err == nil {
			if bad == "nodest-error" {
				fail(1, "no_folder", "no folder is open")
			}
			out(map[string]string{"dir": ""})
			return
		}
		dir := filepath.Join(fx, "destination")
		if bad == "dirnotdir" {
			dir = filepath.Join(fx, "selection", "plain.txt")
		}
		if len(os.Args) == 4 && bad != "rect" {
			var x, y int
			fmt.Sscan(os.Args[2], &x)
			fmt.Sscan(os.Args[3], &y)
			var rs []rect
			if b, err := os.ReadFile(filepath.Join(fx, "destination-at.json")); err == nil {
				json.Unmarshal(b, &rs)
			}
			for _, r := range rs {
				if x >= r.X0 && x < r.X1 && y >= r.Y0 && y < r.Y1 {
					dir = r.Dir
					break
				}
			}
		}
		out(map[string]string{"dir": dir})
	case "refresh":
		io.Copy(io.Discard, os.Stdin)
		out(map[string]bool{"ok": true})
	default:
		fail(2, "unknown_verb", "unknown verb "+os.Args[1])
	}
}
