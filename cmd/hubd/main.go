// Command hubd is the first slice of the Hub OS broker. It reads the
// inventory, checks it against every rule in docs/inventory-format.md,
// checks whether each machine is up, and prints the result. It opens no
// windows and starts no viewers.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"text/tabwriter"
	"time"

	"hubos/internal/inventory"
	"hubos/internal/probe"
)

// defaultInventory is where the real inventory lives on the hub: its own
// disk, in per-machine config outside the system image. It is never
// committed to the repo. hubd never falls back to the example file.
const defaultInventory = "/etc/hubos/inventory.toml"

// Exit codes.
const (
	exitOK           = 0 // the inventory is valid and the checks ran (machines may be down)
	exitFailure      = 1 // hubd itself failed, for example a bad command line
	exitBadInventory = 2 // the inventory file is missing, unreadable or invalid
)

// config holds the time limits. They are guesses, not measured on any real
// network (see the design notes).
type config struct {
	perMachine time.Duration // limit for one machine's check
	total      time.Duration // limit for the whole run
}

var defaultConfig = config{perMachine: 2 * time.Second, total: 5 * time.Second}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr, defaultConfig))
}

func run(args []string, stdout, stderr io.Writer, cfg config) int {
	fs := flag.NewFlagSet("hubd", flag.ContinueOnError)
	fs.SetOutput(stderr)
	path := fs.String("inventory", defaultInventory, "path to the inventory file")
	fs.Usage = func() {
		fmt.Fprintf(stderr, "usage: hubd [--inventory PATH]\n\n"+
			"Reads the inventory, checks it, checks which machines are up, and prints the result.\n"+
			"Without --inventory, the file is read from %s\n", defaultInventory)
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return exitOK
		}
		return exitFailure
	}
	if fs.NArg() > 0 || *path == "" {
		fs.Usage()
		return exitFailure
	}
	given := false
	fs.Visit(func(f *flag.Flag) { given = given || f.Name == "inventory" })

	inv, problems, err := inventory.Load(*path)
	switch {
	case errors.Is(err, inventory.ErrNotFound):
		fmt.Fprintf(stderr, "hubd: there is no inventory file at %s\n", *path)
		if !given {
			fmt.Fprintln(stderr, "hubd: that is the default location. Put the real inventory there, or give another path with --inventory PATH.")
		}
		return exitBadInventory
	case err != nil:
		fmt.Fprintf(stderr, "hubd: cannot read the inventory at %s: %v\n", *path, err)
		return exitBadInventory
	}
	if len(problems) > 0 {
		noun := "problems"
		if len(problems) == 1 {
			noun = "problem"
		}
		fmt.Fprintf(stderr, "hubd: the inventory at %s is not valid (%d %s). No machines were checked.\n", *path, len(problems), noun)
		for _, p := range problems {
			fmt.Fprintf(stderr, "  %s\n", p)
		}
		return exitBadInventory
	}

	rows := check(inv.Machines, cfg)

	fmt.Fprintf(stdout, "hubd: inventory %s (format %d, %d machines)\n\n", *path, inv.Format, len(inv.Machines))
	tw := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "ID\tNAME\tROLE\tPROGRAM\tTARGET\tSTATUS")
	up, notChecked := 0, 0
	for i, m := range inv.Machines {
		r := rows[i]
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n", m.ID, m.Name, m.Role, m.Open[0], r.target, r.status)
		if m.Role == "hub" {
			continue // the hub is shown, but is not counted in "N of M up"
		}
		switch r.state {
		case stateUp:
			up++
		case stateNotChecked:
			notChecked++
		}
	}
	tw.Flush()

	// The hub is excluded from both numbers, so M is the other machines
	// that were checked.
	summary := fmt.Sprintf("%d of %d up", up, len(inv.Machines)-1-notChecked)
	if notChecked > 0 {
		summary += fmt.Sprintf(", %d not checked", notChecked)
	}
	fmt.Fprintf(stdout, "\n%s\n", summary)
	return exitOK
}

type state int

const (
	stateUp state = iota
	stateDown
	stateNotChecked
)

type row struct {
	state  state
	target string // "address:port", or "-" when there is nothing to connect to
	status string
}

// check decides the state of every machine, in order.
//
// The rules (approved by the owner):
//   - The hub is the machine hubd runs on. It is reported UP without a check.
//   - The check uses the first entry of "open" as the program, and the
//     machine's own address and port. If that entry is "none", or the
//     machine has no port, it is NOT CHECKED. hubd has no default port for
//     any program and never guesses one.
//   - Guests are checked at their own address and port (provisional; see the
//     "Unverified" section of docs/inventory-format.md).
func check(machines []inventory.Machine, cfg config) []row {
	rows := make([]row, len(machines))
	var targets []probe.Target
	var index []int // index[k] is the machine that targets[k] belongs to

	for i, m := range machines {
		switch {
		case m.Role == "hub":
			rows[i] = row{stateUp, "-", "UP (this machine, not checked)"}
		case m.Open[0] == "none":
			rows[i] = row{stateNotChecked, "-", "NOT CHECKED (nothing to open)"}
		case m.Port == nil:
			rows[i] = row{stateNotChecked, "-", "NOT CHECKED (no port in the inventory)"}
		default:
			t := probe.Target{Address: m.Address, Port: *m.Port}
			rows[i].target = t.Address + ":" + strconv.Itoa(t.Port)
			targets = append(targets, t)
			index = append(index, i)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), cfg.total)
	defer cancel()
	for k, res := range probe.CheckAll(ctx, targets, cfg.perMachine) {
		i := index[k]
		if res.Up {
			rows[i].state, rows[i].status = stateUp, "UP"
		} else {
			rows[i].state, rows[i].status = stateDown, "DOWN ("+res.Reason+")"
		}
	}
	return rows
}
