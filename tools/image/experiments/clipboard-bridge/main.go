package main

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

func usage() {
	fmt.Fprintln(os.Stderr, `usage: clipboard-bridge COMMAND [flags]
  node-api   serve the node side (uses WAYLAND_DISPLAY and XDG_RUNTIME_DIR of the node)
  hub-watch  watch the hub clipboard; log changes; with -auto push them to the node
  hub-push   push the hub clipboard to the node once (what a button or chord would run)
  hub-pull   copy the node clipboard to the hub once
  relay      stand-in for wayvnc plus a viewer: copy node changes to the hub clipboard
  emit       used by wl-paste --watch; not for people`)
	os.Exit(2)
}

func main() {
	if len(os.Args) < 2 {
		usage()
	}
	cmd, args := os.Args[1], os.Args[2:]
	fs := flag.NewFlagSet(cmd, flag.ExitOnError)
	max := fs.Int("max", DefaultMaxText, "size limit in bytes")
	bin := fs.String("bin", "", "directory holding wl-copy and wl-paste")
	node := fs.String("node", "http://127.0.0.1:8480", "node API base URL")
	listen := fs.String("listen", "127.0.0.1:8480", "node-api: address")
	always := fs.Bool("always-set", false, "node-api: no 'unchanged' rule (shows the echo)")
	auto := fs.Bool("auto", false, "hub-watch: push every change")
	noGuard := fs.Bool("no-guard", false, "hub-watch: no hash rule (shows the echo)")
	minGap := fs.Duration("min-gap", 0, "hub-watch: least time between pushes")
	hubEnv := fs.String("hub-env", "", "hub, relay: XDG_RUNTIME_DIR of the hub compositor (WAYLAND_DISPLAY=wayland-1)")
	nodeEnv := fs.String("node-env", "", "relay: XDG_RUNTIME_DIR of the node compositor (WAYLAND_DISPLAY=wayland-1)")
	rate := fs.Int("rate", 20, "node-api: clipboard sets allowed per 10 s (0 = no limit)")
	crlf := fs.Bool("crlf", false, "relay: turn LF into CRLF on the way")
	fs.Parse(args)
	env := func(dir string) []string {
		if dir == "" {
			return nil
		}
		return []string{"XDG_RUNTIME_DIR=" + dir, "WAYLAND_DISPLAY=wayland-1"}
	}
	hubClip := Wl{Env: env(*hubEnv), Bin: *bin}

	switch cmd {
	case "emit":
		Emit(os.Stdin, os.Stdout, *max)
	case "node-api":
		api := &NodeAPI{Clip: Wl{Bin: *bin}, Max: *max, AlwaysSet: *always, RateMax: *rate, RateWindow: 10 * time.Second, Now: time.Now}
		log.Fatal(http.ListenAndServe(*listen, api))
	case "hub-push":
		text, err := hubClip.Get(*max)
		if err != nil {
			log.Fatalf("hub clipboard: %v", err)
		}
		fmt.Println(push(*node, text))
	case "hub-pull":
		var got struct {
			Present bool
			Text    string
		}
		if err := getJSON(*node+"/v1/clipboard", &got); err != nil || !got.Present {
			log.Fatalf("pull: present=%v err=%v", got.Present, err)
		}
		if err := hubClip.Set(got.Text); err != nil {
			log.Fatal(err)
		}
	case "relay":
		relay(Wl{Env: env(*nodeEnv), Bin: *bin}, hubClip, *max, *crlf)
	case "hub-watch":
		hubWatch(hubClip, *node, *max, *auto, !*noGuard, *minGap)
	default:
		usage()
	}
}

func push(base, text string) string {
	body, _ := json.Marshal(map[string]string{"text": text, "origin": "hub"})
	req, _ := http.NewRequest(http.MethodPut, base+"/v1/clipboard", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "error: " + err.Error()
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return fmt.Sprintf("%d %s", resp.StatusCode, strings.TrimSpace(string(b)))
}

func getJSON(url string, v any) error {
	r, err := http.Get(url)
	if err != nil {
		return err
	}
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(v)
}

// watchLines starts "wl-paste --watch clipboard-bridge emit" and calls fn for
// every text that arrives. wl-paste runs the command once at start, for what is
// already copied, then once per change.
func watchLines(w Wl, max int, fn func(text string, note string)) (*exec.Cmd, error) {
	self, _ := os.Executable()
	c := w.cmd("wl-paste", "-n", "--type", "text", "--watch", self, "emit", fmt.Sprintf("-max=%d", max))
	out, err := c.StdoutPipe()
	if err != nil {
		return nil, err
	}
	c.Stderr = os.Stderr
	if err := c.Start(); err != nil {
		return nil, err
	}
	// when we are stopped, stop wl-paste too (it would otherwise keep watching)
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sig
		c.Process.Kill()
		os.Exit(0)
	}()
	go func() {
		sc := bufio.NewScanner(out)
		sc.Buffer(make([]byte, 0, 64<<10), 8*max)
		for sc.Scan() {
			line := sc.Text()
			switch {
			case strings.HasPrefix(line, "T "):
				b, err := base64.StdEncoding.DecodeString(line[2:])
				if err != nil {
					fn("", "bad line")
					continue
				}
				fn(string(b), "")
			default:
				fn("", line)
			}
		}
	}()
	return c, nil
}

func hubWatch(hub Wl, node string, max int, auto, guard bool, minGap time.Duration) {
	g := NewGuard(minGap)
	n := 0
	c, err := watchLines(hub, max, func(text, note string) {
		n++
		if note != "" {
			fmt.Printf("event %d: %s (not sent)\n", n, note)
			return
		}
		if guard {
			if ok, why := g.Allow(text, max); !ok {
				fmt.Printf("event %d: %d bytes ignored: %s\n", n, len(text), why)
				return
			}
		} else if err := CheckText(text, max); err != nil {
			fmt.Printf("event %d: ignored: %v\n", n, err)
			return
		}
		if !auto {
			fmt.Printf("event %d: %d bytes changed (not pushed, manual mode)\n", n, len(text))
			return
		}
		g.Remember(text)
		fmt.Printf("event %d: %d bytes pushed -> %s\n", n, len(text), push(node, text))
	})
	if err != nil {
		log.Fatal(err)
	}
	c.Wait()
}

// relay stands in for wayvnc on the node plus the viewer on the hub: a change of
// the node's clipboard is copied to the hub's clipboard. Like wayvnc
// (data-control.c, ignores empty text from a client) it never copies empty text.
func relay(nodeClip, hubClip Wl, max int, crlf bool) {
	n := 0
	c, err := watchLines(nodeClip, max, func(text, note string) {
		n++
		if note != "" || text == "" {
			fmt.Printf("relay %d: skipped (%s)\n", n, note)
			return
		}
		if crlf {
			text = strings.ReplaceAll(Normalize(text), "\n", "\r\n")
		}
		fmt.Printf("relay %d: node text (%d bytes) -> hub clipboard\n", n, len(text))
		if err := hubClip.Set(text); err != nil {
			fmt.Printf("relay %d: %v\n", n, err)
		}
	})
	if err != nil {
		log.Fatal(err)
	}
	c.Wait()
}
