package hub

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"
)

// A started viewer's standard output and error go to one file per machine,
// viewer-<id>.log in Settings.LogDir (the runtime folder: memory-backed, gone
// at reboot, mode 0700; files mode 0600). The viewer writes to the file
// directly, not through hubd, so it keeps running if hubd is killed.
//
// Size cap and rotation (Settings.LogMax, proposed 128 KiB):
//   - when a viewer is started and its log is already over the cap, the old
//     log is moved to viewer-<id>.log.1 (replacing an older .1) and a new one starts;
//   - while hubd runs, TrimLogs does the same every few seconds to a log that
//     grew over the cap (the old text is copied to .1 and the log is emptied;
//     the viewer keeps writing, at the start).
//
// So a machine keeps at most about 2 x the cap on disk while hubd runs. While
// hubd is not running nothing trims, so a chatty viewer can grow its log until
// hubd runs again; hubd trims at start-up.

// NewExecLauncher starts the program directly (no shell), in its own process
// group, with no input, and output going to the machine's log (or nowhere if
// logDir is empty).
func NewExecLauncher(logDir string, logMax int64) Launcher {
	return func(machineID string, args []string) (*Proc, error) {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
		if logDir != "" {
			f, err := openLog(logDir, machineID, logMax)
			if err != nil {
				return nil, fmt.Errorf("cannot open the viewer log: %w", err)
			}
			defer f.Close() // the child has its own copy
			fmt.Fprintf(f, "--- %s started: %s\n", time.Now().Format(time.RFC3339), args[0])
			cmd.Stdout, cmd.Stderr = f, f
		}
		if err := cmd.Start(); err != nil {
			return nil, err
		}
		ch := make(chan error, 1)
		go func() { ch <- cmd.Wait() }()
		return &Proc{Exited: ch}, nil
	}
}

func logPath(dir, id string) string { return filepath.Join(dir, "viewer-"+id+".log") }

func openLog(dir, id string, max int64) (*os.File, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	p := logPath(dir, id)
	if fi, err := os.Stat(p); err == nil && fi.Size() > max {
		os.Rename(p, p+".1")
	}
	return os.OpenFile(p, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
}

// LogPath is where a machine's viewer log is, or "" if logs are off.
func (h *Hub) LogPath(id string) string {
	if h.set.LogDir == "" {
		return ""
	}
	return logPath(h.set.LogDir, id)
}

// TrimLogs rotates every viewer log that is over the cap.
func (h *Hub) TrimLogs() {
	if h.set.LogDir == "" {
		return
	}
	TrimLogs(h.set.LogDir, h.set.LogMax)
}

// TrimLogs copies a log that is over max bytes to <log>.1 and empties it.
func TrimLogs(dir string, max int64) {
	files, _ := filepath.Glob(filepath.Join(dir, "viewer-*.log"))
	for _, p := range files {
		fi, err := os.Stat(p)
		if err != nil || fi.Size() <= max {
			continue
		}
		src, err := os.Open(p)
		if err != nil {
			continue
		}
		dst, err := os.OpenFile(p+".1", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
		if err == nil {
			io.Copy(dst, src)
			dst.Close()
			os.Truncate(p, 0) // writers use O_APPEND, so they go on from the start
		}
		src.Close()
	}
}

// RunLogTrim trims at start-up and then every 5 seconds until ctx ends.
func (h *Hub) RunLogTrim(ctx context.Context) {
	h.TrimLogs()
	t := time.NewTicker(5 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			h.TrimLogs()
		}
	}
}
