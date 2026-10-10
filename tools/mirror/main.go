package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"text/tabwriter"
	"time"
)

const usage = `usage: mirror [--root DIR] COMMAND ...

  fetch   NAME VERSION URL SHA256 [--file F] [--signature-url URL --signature-key ID]
          download into the mirror; the hash is given by you, never taken from the download
  verify  [NAME ...]   re-hash stored files, mark mismatches corrupt, write ROOT/status.json; exit 1 if anything is not ok
  status               list entries (no re-hashing)
  path    NAME VERSION [FILE]   the lookup a build uses: prints the path, never downloads; exit 1 and a message if missing or corrupt
`

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("mirror", flag.ContinueOnError)
	fs.SetOutput(stderr)
	root := fs.String("root", os.Getenv("HUBOS_MIRROR"), "mirror folder (default $HUBOS_MIRROR)")
	fs.Usage = func() { fmt.Fprint(stderr, usage) }
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	rest := fs.Args()
	if len(rest) == 0 || *root == "" {
		fmt.Fprint(stderr, usage)
		if *root == "" {
			fmt.Fprintln(stderr, "mirror: no --root and no $HUBOS_MIRROR")
		}
		return 2
	}
	m := New(*root)
	cmd, rest := rest[0], rest[1:]
	switch cmd {
	case "fetch":
		ff := flag.NewFlagSet("fetch", flag.ContinueOnError)
		ff.SetOutput(stderr)
		file := ff.String("file", "", "stored file name (default: last part of the URL)")
		surl := ff.String("signature-url", "", "URL of the upstream detached signature")
		skey := ff.String("signature-key", "", "key id; keyring is ROOT/keys/ID.gpg")
		pos, err := parseInterspersed(ff, rest)
		if err != nil || len(pos) != 4 {
			fmt.Fprint(stderr, usage)
			return 2
		}
		res, err := m.Fetch(pos[0], pos[1], pos[2], pos[3], FetchOpts{File: *file, SignatureURL: *surl, SignatureKey: *skey})
		if err != nil {
			fmt.Fprintln(stderr, "mirror:", err)
			return 1
		}
		fmt.Fprintln(stdout, res)
	case "verify":
		es, ok, err := m.Verify(rest...)
		if err != nil {
			fmt.Fprintln(stderr, "mirror:", err)
			return 1
		}
		printEntries(stdout, es)
		if !ok {
			fmt.Fprintln(stderr, "mirror: verify found problems; see", *root+"/status.json")
			return 1
		}
	case "status":
		es, err := m.Status()
		if err != nil {
			fmt.Fprintln(stderr, "mirror:", err)
			return 1
		}
		printEntries(stdout, es)
	case "path":
		if len(rest) < 2 || len(rest) > 3 {
			fmt.Fprint(stderr, usage)
			return 2
		}
		f := ""
		if len(rest) == 3 {
			f = rest[2]
		}
		p, err := m.Path(rest[0], rest[1], f)
		if err != nil {
			fmt.Fprintln(stderr, "mirror:", err)
			return 1
		}
		fmt.Fprintln(stdout, p)
	default:
		fmt.Fprint(stderr, usage)
		return 2
	}
	return 0
}

// parseInterspersed lets flags come before or after the positional words.
func parseInterspersed(fs *flag.FlagSet, args []string) ([]string, error) {
	var pos []string
	for {
		if err := fs.Parse(args); err != nil {
			return nil, err
		}
		if fs.NArg() == 0 {
			return pos, nil
		}
		pos = append(pos, fs.Arg(0))
		args = fs.Args()[1:]
	}
}

func printEntries(w io.Writer, es []Entry) {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "NAME\tVERSION\tSTATE\tAGE\tSIGNATURE\tDETAIL")
	for _, e := range es {
		age := ""
		if t, err := time.Parse(time.RFC3339, e.FetchedAt); err == nil {
			age = time.Since(t).Round(time.Minute).String()
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n", e.Name, e.Version, e.State, age, e.SigState, e.Detail)
	}
	tw.Flush()
}
