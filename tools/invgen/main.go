// Command invgen generates the hub's inventory.toml from one machine file per machine, and migrates an old
// role-based inventory into machine files. See docs/proposals/generated-inventory.md.
//
//	invgen generate --machines DIR --out FILE    write FILE (or standard output with --out -)
//	invgen migrate  --inventory OLD --machines DIR   write one DIR/NAME.toml per machine; OLD is not changed
//
// Exit codes: 0 done, 2 the input is not valid (problems are printed, nothing is written), 1 anything else.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"hubos/internal/machinefile"
)

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: invgen generate --machines DIR --out FILE | invgen migrate --inventory OLD --machines DIR")
		return 1
	}
	fs := flag.NewFlagSet("invgen "+args[0], flag.ContinueOnError)
	fs.SetOutput(stderr)
	dir := fs.String("machines", "machine_files", "folder of machine files")
	out := fs.String("out", "", "generate: output file, or - for standard output")
	old := fs.String("inventory", "", "migrate: the old role-based inventory")
	if err := fs.Parse(args[1:]); err != nil {
		return 1
	}
	switch args[0] {
	case "generate":
		if *out == "" {
			fmt.Fprintln(stderr, "invgen: --out is required")
			return 1
		}
		srcs, ps, err := machinefile.Load(*dir)
		if err != nil {
			fmt.Fprintln(stderr, "invgen:", err)
			return 1
		}
		if len(ps) > 0 {
			return problems(stderr, ps)
		}
		if len(srcs) == 0 {
			fmt.Fprintf(stderr, "invgen: no machine files (*.toml) in %s\n", *dir)
			return 2
		}
		text, ps := machinefile.Generate(srcs)
		if len(ps) > 0 {
			return problems(stderr, ps)
		}
		if *out == "-" {
			fmt.Fprint(stdout, text)
			return 0
		}
		if err := writeAtomic(*out, []byte(text)); err != nil {
			fmt.Fprintln(stderr, "invgen:", err)
			return 1
		}
	case "migrate":
		if *old == "" {
			fmt.Fprintln(stderr, "invgen: --inventory is required")
			return 1
		}
		b, err := os.ReadFile(*old)
		if err != nil {
			fmt.Fprintln(stderr, "invgen:", err)
			return 1
		}
		ms, report, ps := machinefile.Migrate(b)
		if len(ps) > 0 {
			return problems(stderr, ps)
		}
		if err := os.MkdirAll(*dir, 0o755); err != nil {
			fmt.Fprintln(stderr, "invgen:", err)
			return 1
		}
		for _, m := range ms {
			if err := writeAtomic(filepath.Join(*dir, m.File.Name+".toml"), []byte(machinefile.Render(m))); err != nil {
				fmt.Fprintln(stderr, "invgen:", err)
				return 1
			}
		}
		fmt.Fprintf(stdout, "migrated %d machines into %s\n", len(ms), *dir)
		for _, l := range report {
			fmt.Fprintln(stdout, "note: "+l)
		}
	default:
		fmt.Fprintln(stderr, "invgen: unknown command", args[0])
		return 1
	}
	return 0
}

func problems(w io.Writer, ps []machinefile.Problem) int {
	for _, p := range ps {
		fmt.Fprintf(w, "invgen: %s\n", p)
	}
	return 2
}

func writeAtomic(path string, b []byte) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".invgen-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if _, err := f.Write(b); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
