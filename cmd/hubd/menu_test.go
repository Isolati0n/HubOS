package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"hubos/internal/hub"
)

// A stand-in for hubd that answers `hubd menu`, and a stand-in for wofi that
// prints the next line from a script file and records how it was started.
func TestMenuLoopSearchAskAndLauncherOptions(t *testing.T) {
	dir, err := os.MkdirTemp("", "mt")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	sock := filepath.Join(dir, "h.sock")
	l, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	var mu sync.Mutex
	var reqs []hub.Request
	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}
			line, _ := bufio.NewReader(c).ReadBytes('\n')
			var r hub.Request
			json.Unmarshal(line, &r)
			mu.Lock()
			reqs = append(reqs, r)
			mu.Unlock()
			resp := hub.Response{OK: true, Lines: []string{"line one", "line two"}}
			if r.Cmd == "pick" {
				switch {
				case strings.HasPrefix(r.Line, "? search"):
					resp = hub.Response{OK: true, Action: "search", Reopen: true, Flat: true, Ask: true}
				default:
					resp = hub.Response{OK: true, Action: "open", Message: "opened X"}
				}
			}
			b, _ := json.Marshal(resp)
			c.Write(append(b, '\n'))
			c.Close()
		}
	}()

	script := filepath.Join(dir, "answers")
	os.WriteFile(script, []byte("? search by id or name...\nguest-7\n   guest-7  Guest 7  UP\n"), 0o600)
	argsLog := filepath.Join(dir, "args")
	fake := filepath.Join(dir, "fakewofi")
	os.WriteFile(fake, []byte("#!/bin/sh\necho \"$@\" >> "+argsLog+"\ncat > /dev/null\nhead -n 1 "+script+"\nsed -i 1d "+script+"\n"), 0o700)
	style := filepath.Join(dir, "style.css")
	os.WriteFile(style, []byte("* {}"), 0o600)

	var out, errb bytes.Buffer
	code := dispatch([]string{"menu", "--socket", sock, "--wofi", fake, "--style", style, "--width", "640", "--single-click"}, &out, &errb)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errb.String())
	}
	mu.Lock()
	// list, pick(search), list(flat+filter), pick(machine)
	if len(reqs) != 4 || reqs[0].Cmd != "list" || reqs[1].Cmd != "pick" || reqs[2].Cmd != "list" || !reqs[2].Flat || reqs[2].Filter != "guest-7" || reqs[3].Cmd != "pick" {
		mu.Unlock()
		t.Fatalf("requests: %+v", reqs)
	}
	if !strings.Contains(out.String(), "opened X") {
		t.Errorf("output: %q", out.String())
	}
	mu.Unlock()
	a, _ := os.ReadFile(argsLog)
	lines := strings.Split(strings.TrimSpace(string(a)), "\n")
	if len(lines) != 3 {
		t.Fatalf("launcher started %d times: %q", len(lines), a)
	}
	for _, l := range lines {
		for _, want := range []string{"--dmenu", "--cache-file /dev/null", "--width 640", "--style " + style, "-D single_click=true"} {
			if !strings.Contains(l, want) {
				t.Errorf("launcher args %q lack %q", l, want)
			}
		}
	}
	// Without the style file and flag, neither option is passed.
	os.WriteFile(script, []byte("   x  X  UP\n"), 0o600)
	os.WriteFile(argsLog, nil, 0o600)
	errb.Reset()
	dispatch([]string{"menu", "--socket", sock, "--wofi", fake, "--style", filepath.Join(dir, "missing.css")}, &out, &errb)
	b, _ := os.ReadFile(argsLog)
	if strings.Contains(string(b), "--style") || strings.Contains(string(b), "single_click") || !strings.Contains(string(b), "--width 720") {
		t.Errorf("defaults: %q", b)
	}
}
