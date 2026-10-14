package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

// BootClock returns CLOCK_BOOTTIME: the time since the kernel started, in a form that can be compared between two runs of the
// watcher on the same boot. Why this clock: it keeps counting while the machine sleeps; it is not changed by anyone setting the
// wall clock (the fault suite sets it back an hour and forward a day, and a restart of the watcher between those would
// otherwise see kill times in the future or a day old); a restarted watcher process reads the same clock (CLOCK_MONOTONIC would
// do as well on a machine that never sleeps; the process's own Go monotonic reading would not survive a restart). Its limit:
// it starts again at zero after a reboot, but so does /run, where the file lives, so a file and a clock never disagree about
// which boot they belong to.
func BootClock() time.Duration {
	var ts syscall.Timespec
	const clockBoottime = 7
	if _, _, e := syscall.Syscall(syscall.SYS_CLOCK_GETTIME, clockBoottime, uintptr(unsafe.Pointer(&ts)), 0); e != 0 {
		return 0
	}
	return time.Duration(ts.Sec)*time.Second + time.Duration(ts.Nsec)
}

const budgetHeader = "hubos-watcher-budget v1 clock=boottime unit=ns"

// Budget is the kill budget: the times of the kills made in the last Per, at most Max of them. It is kept in a file (in /run, the
// in-memory area) so that a restarted watcher does not get a fresh budget. Every change is written at once, by writing a
// temporary file next to it and renaming it over the real one (a reader sees the old file or the new one, never half of one).
type Budget struct {
	Path  string
	Max   int
	Per   time.Duration
	kills []time.Duration
	// SaveErr is the error of the last write (nil when it worked); the kill budget still counts in memory.
	SaveErr error
	sig     string // fileSig after our last write or read
}

// LoadBudget reads the file. now is the current BootClock. A file that is missing or damaged gives an empty budget and a note
// saying so (the caller logs it); it never fails. Damaged means: wrong header, a line that is not a time, a time in the future
// (it cannot belong to this boot), a missing or wrong "end N" line, anything after it.
func LoadBudget(path string, now time.Duration, max int, per time.Duration) (*Budget, string) {
	b := &Budget{Path: path, Max: max, Per: per}
	f, err := os.Open(path)
	if err != nil {
		b.sig = b.fileSig()
		if os.IsNotExist(err) {
			return b, "budget file missing: starting with an empty budget"
		}
		return b, fmt.Sprintf("budget file unreadable (%v): starting with an empty budget", err)
	}
	defer f.Close()
	damaged := func(why string) (*Budget, string) {
		b.sig = b.fileSig()
		b.kills = nil
		return b, "budget file damaged (" + why + "): starting with an empty budget"
	}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 4096), 4096)
	if !sc.Scan() || sc.Text() != budgetHeader {
		return damaged("bad header")
	}
	var all []time.Duration
	ended := false
	for sc.Scan() {
		l := sc.Text()
		if ended {
			return damaged("text after the end line")
		}
		if strings.HasPrefix(l, "end ") {
			n, err := strconv.Atoi(strings.TrimPrefix(l, "end "))
			if err != nil || n != len(all) {
				return damaged("end line does not match the number of entries")
			}
			ended = true
			continue
		}
		if !strings.HasPrefix(l, "kill ") {
			return damaged("unknown line")
		}
		v, err := strconv.ParseInt(strings.TrimPrefix(l, "kill "), 10, 64)
		if err != nil || v < 0 {
			return damaged("entry is not a time")
		}
		t := time.Duration(v)
		if t > now+time.Second {
			return damaged("entry is in the future of this boot")
		}
		all = append(all, t)
		if len(all) > 1000 {
			return damaged("too many entries")
		}
	}
	if sc.Err() != nil || !ended {
		return damaged("file ends early")
	}
	dropped := 0
	for _, t := range all {
		if now-t < per {
			b.kills = append(b.kills, t)
		} else {
			dropped++
		}
	}
	b.sig = b.fileSig()
	return b, fmt.Sprintf("budget file read: %d kills in the last %v kept, %d older dropped", len(b.kills), per, dropped)
}

// Prune drops the entries that are older than Per.
func (b *Budget) Prune(now time.Duration) {
	keep := b.kills[:0]
	for _, t := range b.kills {
		if now-t < b.Per {
			keep = append(keep, t)
		}
	}
	b.kills = keep
}

// Full reports whether Max kills were made in the last Per.
func (b *Budget) Full(now time.Duration) bool {
	b.Prune(now)
	return len(b.kills) >= b.Max
}

func (b *Budget) Len() int { return len(b.kills) }

// Add records a kill and writes the file.
func (b *Budget) Add(now time.Duration) {
	b.Prune(now)
	b.kills = append(b.kills, now)
	b.SaveErr = b.save()
}

func (b *Budget) save() error {
	if b.Path == "" {
		return nil
	}
	var sb strings.Builder
	sb.WriteString(budgetHeader + "\n")
	for _, t := range b.kills {
		fmt.Fprintf(&sb, "kill %d\n", int64(t))
	}
	fmt.Fprintf(&sb, "end %d\n", len(b.kills))
	err := writeAtomic(b.Path, sb.String())
	b.sig = b.fileSig()
	return err
}

// writeAtomic: write a temporary file in the same directory, flush it, then rename it over the target.
func writeAtomic(path, content string) error {
	tmp := path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	if _, err := f.WriteString(content); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, path)
}

// StateFile is the one-line hook a bar would read: "capped <time>" while the cap blocks a kill. This test has no bar or panel.
type StateFile struct{ Path string }

// SetCapped writes "capped <wall time>" (atomically).
func (s StateFile) SetCapped(wall time.Time) error {
	if s.Path == "" {
		return nil
	}
	return writeAtomic(s.Path, "capped "+wall.UTC().Format("2006-01-02T15:04:05Z")+"\n")
}

// Clear removes the file; it is not an error if there is none.
func (s StateFile) Clear() error {
	if s.Path == "" {
		return nil
	}
	if err := os.Remove(s.Path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// Exists reports whether the file is there.
func (s StateFile) Exists() bool {
	if s.Path == "" {
		return false
	}
	_, err := os.Stat(s.Path)
	return err == nil
}

// ---- follow-up 5 additions ----

// Undo takes back the last kill (a kill that found no process is never counted) and writes the file.
func (b *Budget) Undo() {
	if len(b.kills) > 0 {
		b.kills = b.kills[:len(b.kills)-1]
	}
	b.SaveErr = b.save()
}

// fileSig is a cheap signature of the budget file (inode, modification time, size) as it is now; "" when it cannot be read.
func (b *Budget) fileSig() string {
	if b.Path == "" {
		return ""
	}
	fi, err := os.Stat(b.Path)
	if err != nil {
		return "missing"
	}
	st, _ := fi.Sys().(*syscall.Stat_t)
	if st == nil {
		return fmt.Sprintf("%d-%d", fi.ModTime().UnixNano(), fi.Size())
	}
	return fmt.Sprintf("%d-%d-%d", st.Ino, fi.ModTime().UnixNano(), fi.Size())
}

// Changed reports whether someone else wrote the file since this process last wrote or read it (the person-command `reset`
// empties it while the watcher runs). If the last write of this process failed, the memory is kept (no reload).
func (b *Budget) Changed() bool { return b.SaveErr == nil && b.sig != b.fileSig() }

// Reload reads the file again (same rules as LoadBudget) and returns a note.
func (b *Budget) Reload(now time.Duration) string {
	nb, note := LoadBudget(b.Path, now, b.Max, b.Per)
	b.kills, b.SaveErr = nb.kills, nil
	b.sig = b.fileSig()
	return note
}
