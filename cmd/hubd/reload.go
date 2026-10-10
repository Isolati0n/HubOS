package main

import (
	"errors"
	"flag"
	"fmt"
	"io"

	"hubos/internal/inventory"
)

// reloadCmd is `hubd reload --dry-run NEWFILE`: parse and validate the new inventory, compare it with the current one, print
// what would change, and change nothing. It works on files, not on the running hubd: hubd has no live reload today (it reads the
// inventory once at start). Without --dry-run it refuses and says so. See docs/proposals/generated-inventory.md section 6.
//
// Exit codes: exitOK when the new file is valid (even if there are changes), exitBadInventory when the new file is missing,
// unreadable or invalid (the old inventory would be kept), exitFailure for a usage error or when --dry-run is missing.
func reloadCmd(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("hubd reload", flag.ContinueOnError)
	fs.SetOutput(stderr)
	dry := fs.Bool("dry-run", false, "report what would change; change nothing (the only mode there is today)")
	cur := fs.String("inventory", defaultInventory, "path to the current inventory file")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return exitOK
		}
		return exitFailure
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(stderr, "usage: hubd reload --dry-run [--inventory CURRENT] NEWFILE")
		return exitFailure
	}
	if !*dry {
		fmt.Fprintln(stderr, "hubd: live reload is not built yet (hubd reads the inventory once, at start); use --dry-run to see what a reload would do")
		return exitFailure
	}
	next, ps, err := inventory.Load(fs.Arg(0))
	if err != nil {
		fmt.Fprintf(stderr, "hubd: new inventory %s: %v; the current inventory would be kept\n", fs.Arg(0), err)
		return exitBadInventory
	}
	if len(ps) > 0 {
		for _, p := range ps {
			fmt.Fprintf(stderr, "hubd: new inventory: %s\n", p)
		}
		fmt.Fprintln(stderr, "hubd: the new inventory is not valid; the current inventory would be kept")
		return exitBadInventory
	}
	old, ops, err := inventory.Load(*cur)
	if err != nil || len(ops) > 0 {
		// The current file could not be used as a base: say so, and treat every machine of the new file as added.
		fmt.Fprintf(stderr, "hubd: note: the current inventory %s could not be read as a base (%v); showing all machines as added\n", *cur, firstProblem(err, ops))
		old = nil
	}
	ch := inventory.Diff(old, next)
	if ch.Empty() {
		fmt.Fprintln(stdout, "dry run: the new inventory is valid and the same as the current one; nothing would change")
		return exitOK
	}
	fmt.Fprintln(stdout, "dry run: the new inventory is valid; a reload would change:")
	for _, l := range ch.Lines() {
		fmt.Fprintln(stdout, "  "+l)
	}
	return exitOK
}

func firstProblem(err error, ps []inventory.Problem) string {
	if err != nil {
		return err.Error()
	}
	if len(ps) > 0 {
		return ps[0].String()
	}
	return "unknown"
}
