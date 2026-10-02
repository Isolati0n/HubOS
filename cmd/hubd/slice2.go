package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"hubos/internal/driftwm"
	"hubos/internal/hub"
	"hubos/internal/inventory"
	"hubos/internal/probe"
	"hubos/internal/viewers"
)

// subcommands of the second slice. "check" is the first slice, unchanged.
var subcommands = map[string]bool{"serve": true, "feed": true, "list": true, "menu": true, "pick": true, "open": true, "end": true, "forget": true}

func dispatch(args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 {
		switch {
		case args[0] == "check":
			return run(args[1:], stdout, stderr, defaultConfig)
		case subcommands[args[0]]:
			return slice2(args[0], args[1:], stdout, stderr)
		}
	}
	return run(args, stdout, stderr, defaultConfig)
}

// socketFlags registers --socket and returns a function that resolves it.
func socketFlags(fs *flag.FlagSet) func() (string, error) {
	p := fs.String("socket", "", "hubd's socket (default $XDG_RUNTIME_DIR/hubos/hubd.sock)")
	return func() (string, error) {
		if *p != "" {
			return *p, hub.CheckSocketPath(*p)
		}
		return hub.SocketPath(os.Getenv("XDG_RUNTIME_DIR"))
	}
}

func slice2(name string, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("hubd "+name, flag.ContinueOnError)
	fs.SetOutput(stderr)
	switch name {
	case "serve":
		return serve(fs, args, stdout, stderr)
	case "menu":
		return menu(fs, args, stdout, stderr)
	}
	flat := fs.Bool("flat", false, "list: machines in one flat list")
	filter := fs.String("filter", "", "list: only machines whose id or name contains this text")
	sock := socketFlags(fs)
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return exitOK
		}
		return exitFailure
	}
	path, err := sock()
	if err != nil {
		fmt.Fprintln(stderr, "hubd:", err)
		return exitFailure
	}
	rest := fs.Args()
	need := func(n int) bool {
		if len(rest) != n {
			fmt.Fprintf(stderr, "usage: hubd %s%s\n", name, map[string]string{"pick": " LINE", "open": " ID", "end": " ID", "forget": " ID"}[name])
			return false
		}
		return true
	}
	switch name {
	case "feed":
		return feed(path, stdout)
	case "list":
		if !need(0) {
			return exitFailure
		}
		r, err := hub.Call(path, hub.Request{Cmd: "list", Flat: *flat, Filter: *filter})
		if err != nil {
			fmt.Fprintln(stderr, "hubd:", err)
			return exitFailure
		}
		for _, l := range r.Lines {
			fmt.Fprintln(stdout, l)
		}
	case "pick":
		if !need(1) {
			return exitFailure
		}
		r, err := hub.Call(path, hub.Request{Cmd: "pick", Line: rest[0]})
		if err != nil {
			fmt.Fprintln(stderr, "hubd:", err)
			return exitFailure
		}
		if r.Message != "" {
			fmt.Fprintln(stdout, r.Message)
		}
	case "open", "end", "forget":
		if !need(1) {
			return exitFailure
		}
		r, err := hub.Call(path, hub.Request{Cmd: name, ID: rest[0]})
		if err != nil {
			fmt.Fprintln(stderr, "hubd:", err)
			return exitFailure
		}
		fmt.Fprintln(stdout, r.Message)
		if !r.OK {
			return exitFailure
		}
	}
	return exitOK
}

// feed copies hubd's status lines to stdout for Waybar. If hubd is not
// running it shows an alert and keeps trying, so the bar recovers by itself.
func feed(socket string, stdout io.Writer) int {
	last := ""
	emit := func(l string) {
		if l != last {
			fmt.Fprintln(stdout, l)
			last = l
		}
	}
	for {
		err := hub.Feed(socket, emit)
		emit(hub.StatusLine{Text: "hubd stopped", Class: "alert", Tooltip: "hubd is not running: " + err.Error()}.JSON())
		time.Sleep(2 * time.Second)
	}
}

func serve(fs *flag.FlagSet, args []string, stdout, stderr io.Writer) int {
	def := hub.DefaultSettings()
	invPath := fs.String("inventory", defaultInventory, "path to the inventory file")
	viewersPath := fs.String("viewers", "", "path to viewers.toml (default: next to the inventory)")
	dwSock := fs.String("driftwm-socket", "", "driftwm's socket (default from XDG_RUNTIME_DIR and WAYLAND_DISPLAY)")
	bar := fs.Int("bar-height", def.BarHeight, "pixels the bar reserves at the screen edge (0 = none or unknown)")
	cap_ := fs.Int("probe-cap", def.ProbeCap, "most checks in flight at once")
	interval := fs.Duration("probe-interval", def.ProbeInterval, "time between the starts of two check rounds")
	ptimeout := fs.Duration("probe-timeout", def.ProbeTimeout, "limit for one machine's check")
	wait := fs.Duration("window-wait", def.WindowWait, "how long to wait for a started viewer's window")
	fold := fs.Int("fold", def.FoldThreshold, "groups with more machines than this start folded")
	downMax := fs.Int("down-max", def.DownMax, "most machine lines in the Down machines group")
	listMax := fs.Int("list-max", def.ListMax, "most machine lines in one menu list")
	tipcap := fs.Int("tooltip-cap", def.TooltipCap, "most down machines named in the tooltip")
	ttl := fs.Duration("message-ttl", def.MessageTTL, "how long a message stays on the bar item")
	logRounds := fs.Bool("log-rounds", false, "print one line per check round (how long it took, how many up and down)")
	sock := socketFlags(fs)
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return exitOK
		}
		return exitFailure
	}
	if fs.NArg() > 0 {
		fmt.Fprintln(stderr, "usage: hubd serve [flags]")
		return exitFailure
	}

	inv, problems, err := inventory.Load(*invPath)
	switch {
	case errors.Is(err, inventory.ErrNotFound):
		fmt.Fprintf(stderr, "hubd: there is no inventory file at %s\n", *invPath)
		return exitBadInventory
	case err != nil:
		fmt.Fprintf(stderr, "hubd: cannot read the inventory at %s: %v\n", *invPath, err)
		return exitBadInventory
	case len(problems) > 0:
		fmt.Fprintf(stderr, "hubd: the inventory at %s is not valid (%d problems). Nothing was started.\n", *invPath, len(problems))
		for _, p := range problems {
			fmt.Fprintf(stderr, "  %s\n", p)
		}
		return exitBadInventory
	}
	vp := *viewersPath
	if vp == "" {
		vp = filepath.Join(filepath.Dir(*invPath), "viewers.toml")
	}
	vt, vproblems, err := viewers.Load(vp)
	switch {
	case errors.Is(err, viewers.ErrNotFound):
		fmt.Fprintf(stderr, "hubd: there is no viewers file at %s (give another path with --viewers PATH)\n", vp)
		return exitBadInventory
	case err != nil:
		fmt.Fprintf(stderr, "hubd: cannot read %s: %v\n", vp, err)
		return exitBadInventory
	case len(vproblems) > 0:
		fmt.Fprintf(stderr, "hubd: %s is not valid (%d problems). Nothing was started.\n", vp, len(vproblems))
		for _, p := range vproblems {
			fmt.Fprintf(stderr, "  %s\n", p)
		}
		return exitBadInventory
	}

	socket, err := sock()
	if err != nil {
		fmt.Fprintln(stderr, "hubd:", err)
		return exitFailure
	}
	dwPath := *dwSock
	if dwPath == "" {
		dwPath = driftwm.SocketPath()
	}
	if len(dwPath) > driftwm.MaxSocketPath {
		fmt.Fprintf(stderr, "hubd: warning: driftwm's socket path is %d bytes (limit %d); driftwm fails to start its socket on a path that long, so windows cannot be opened until XDG_RUNTIME_DIR is shorter\n", len(dwPath), driftwm.MaxSocketPath)
	}

	limit := probe.FileLimit()
	set := hub.Settings{
		ProbeCap: *cap_, ProbeInterval: *interval, ProbeTimeout: *ptimeout, WindowWait: *wait,
		Settle: def.Settle, CloseWait: def.CloseWait, FoldThreshold: *fold, ListMax: *listMax, DownMax: *downMax, TooltipCap: *tipcap,
		MessageTTL: *ttl, BarHeight: *bar, FileLimit: limit,
	}
	if *logRounds {
		round := 0
		set.OnRound = func(took time.Duration, c hub.Counts) {
			round++
			fmt.Fprintf(stderr, "hubd: round %d took %s: %d up, %d down, %d not checked\n", round, took.Round(time.Millisecond), c.Up, c.Down, c.NotChecked)
		}
	}
	set.LogDir = strings.TrimSuffix(socket, ".sock") + ".viewer-logs"
	dw := &driftwm.Client{Path: dwPath}
	h := hub.New(inv, vt, dw, hub.NewExecLauncher(set.LogDir, set.LogMax), set, strings.TrimSuffix(socket, ".sock")+".record.json")

	l, err := hub.Listen(socket)
	if err != nil {
		fmt.Fprintln(stderr, "hubd:", err)
		return exitFailure
	}
	defer os.Remove(socket)

	fmt.Fprintf(stderr, "hubd: serving %d machines on %s; driftwm at %s\n", len(inv.Machines), socket, dwPath)
	fmt.Fprintf(stderr, "hubd: open-file limit %d; check cap %d (asked for %d); round every %s; per-machine limit %s\n", limit, h.ProbeCap(), *cap_, *interval, *ptimeout)
	if h.ProbeCap() < *cap_ {
		fmt.Fprintf(stderr, "hubd: the check cap was lowered to stay under the open-file limit (raise it with ulimit -n)\n")
	}
	var noViewer []string
	for _, m := range inv.Machines {
		if m.Role != "hub" && m.Open[0] != "none" && vt.For(m.Open[0]) == nil {
			noViewer = append(noViewer, m.ID)
		}
	}
	if len(noViewer) > 0 {
		if len(noViewer) > 5 {
			noViewer = append(noViewer[:5], "...")
		}
		fmt.Fprintf(stderr, "hubd: no viewer in %s for some machines (they will be listed but cannot be opened): %s\n", vp, strings.Join(noViewer, ", "))
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go h.Serve(l)
	go h.RunProbes(ctx)
	go h.RunWatch(ctx)
	go h.RunLogTrim(ctx)
	<-ctx.Done()
	l.Close()
	fmt.Fprintln(stderr, "hubd: stopped (windows it started were left alone)")
	return exitOK
}

// menu shows the list in wofi and acts on the pick, again and again while
// the answer is "show it again" (opening or folding a group, search).
func menu(fs *flag.FlagSet, args []string, stdout, stderr io.Writer) int {
	launcher := fs.String("wofi", "wofi", "the list launcher program")
	style := fs.String("style", defaultWofiStyle, "wofi style file (CSS); used only if the file exists")
	width := fs.Int("width", 720, "menu width in pixels")
	single := fs.Bool("single-click", false, "pick with a single mouse click (wofi's default is a double click)")
	sock := socketFlags(fs)
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return exitOK
		}
		return exitFailure
	}
	path, err := sock()
	if err != nil {
		fmt.Fprintln(stderr, "hubd:", err)
		return exitFailure
	}
	// One menu at a time: a second click on the bar item does nothing.
	lock, err := os.OpenFile(strings.TrimSuffix(path, ".sock")+".menu.lock", os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		fmt.Fprintln(stderr, "hubd:", err)
		return exitFailure
	}
	defer lock.Close()
	if syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB) != nil {
		return exitOK
	}
	opts := launcherOpts{program: *launcher, width: *width, singleClick: *single}
	if _, err := os.Stat(*style); err == nil {
		opts.style = *style
	}
	flat, filter := false, ""
	for {
		r, err := hub.Call(path, hub.Request{Cmd: "list", Flat: flat, Filter: filter})
		if err != nil {
			fmt.Fprintln(stderr, "hubd:", err)
			return exitFailure
		}
		pick := runLauncher(opts, r.Lines, stderr)
		if pick == "" {
			return exitOK
		}
		pr, err := hub.Call(path, hub.Request{Cmd: "pick", Line: pick})
		if err != nil {
			fmt.Fprintln(stderr, "hubd:", err)
			return exitFailure
		}
		if pr.Message != "" {
			fmt.Fprintln(stdout, pr.Message)
		}
		if !pr.Reopen {
			return exitOK
		}
		flat, filter = pr.Flat, ""
		if pr.Ask {
			// wofi gives back what was typed when nothing in the list matches.
			typed := runLauncher(opts, []string{hub.SearchHint}, stderr)
			if typed == "" || typed == hub.SearchHint {
				flat = false
				continue
			}
			filter = typed
		}
	}
}

// defaultWofiStyle is where the menu look is read from if the file exists.
const defaultWofiStyle = "/etc/hubos/wofi.css"

type launcherOpts struct {
	program     string
	style       string
	width       int
	singleClick bool
}

// runLauncher shows the lines and returns the line picked (or the text typed
// when nothing matched). Empty means Escape.
func runLauncher(o launcherOpts, lines []string, stderr io.Writer) string {
	args := []string{"--dmenu", "--cache-file", "/dev/null", "--insensitive",
		"--lines", "12", "--width", strconv.Itoa(o.width), "--location", "top_left", "--yoffset", "0", "--prompt", "hub"}
	if o.style != "" {
		args = append(args, "--style", o.style)
	}
	if o.singleClick {
		args = append(args, "-D", "single_click=true")
	}
	cmd := exec.Command(o.program, args...)
	cmd.Env = append(os.Environ(), "LC_ALL=C.UTF-8")
	cmd.Stdin = strings.NewReader(strings.Join(lines, "\n") + "\n")
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = stderr
	cmd.Run() // a non-zero exit with no output means Escape was pressed
	return strings.TrimRight(out.String(), "\n")
}
