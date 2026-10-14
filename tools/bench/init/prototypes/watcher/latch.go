package main

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// Latch (follow-up 5, owner decision 1): when the cap of 3 kills in 10 minutes is reached and the rule wants a 4th kill, the
// watcher stops killing and stays stopped until a person clears the latch with one command (`watcher reset`; for this test only
// the hook, the product's command is not designed). The latch is a file in /run/hubos/ (in-memory area), written atomically
// (temporary file, flush, rename), read at start and again in every round, so it survives a restart of the watcher and a
// `reset` takes effect within one round. A file that is there but damaged counts as LATCHED (the safe side, nothing is killed).
type Latch struct{ Path string }

const latchHeader = "hubos-watcher-latch v1"

// Set writes the latch file: header, the boot-clock reading and the wall time (information only).
func (l Latch) Set(boot time.Duration, wall time.Time) error {
	if l.Path == "" {
		return nil
	}
	return writeAtomic(l.Path, fmt.Sprintf("%s\nboot_ns %d\nutc %s\nend\n", latchHeader, int64(boot), wall.UTC().Format("2006-01-02T15:04:05Z")))
}

// State: latched is true when the file exists (or cannot be examined: the safe side); damage is non-empty when the file is
// there but not what Set writes (the caller logs it, once per change of the text).
func (l Latch) State() (latched bool, damage string) {
	if l.Path == "" {
		return false, ""
	}
	fi, err := os.Stat(l.Path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, ""
		}
		return true, "cannot examine the latch file: " + err.Error()
	}
	if fi.IsDir() {
		return true, "the latch path is a directory"
	}
	b, err := os.ReadFile(l.Path)
	if err != nil {
		return true, "cannot read the latch file: " + err.Error()
	}
	lines := strings.Split(strings.TrimRight(string(b), "\n"), "\n")
	if len(lines) != 4 || lines[0] != latchHeader || !strings.HasPrefix(lines[1], "boot_ns ") || !strings.HasPrefix(lines[2], "utc ") || lines[3] != "end" {
		return true, "the latch file is damaged"
	}
	return true, ""
}

// Clear removes the latch; no error if there is none.
func (l Latch) Clear() error {
	if l.Path == "" {
		return nil
	}
	if err := os.Remove(l.Path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
