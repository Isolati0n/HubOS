package main

import (
	"bufio"
	"encoding/binary"
	"flag"
	"fmt"
	"io"
	"math/rand"
	"net"
	"os"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"
)

const (
	seatSock = "/run/seatd.sock"
	udevFlag = "/run/udev/ready"
	dbusSock = "/run/dw/bus"
	waySock  = "/run/dw/wayland-1"
	hubdSock = "/run/hubos/hubd.sock"
	faultDir = "/run/fault"
)

var held []byte

func exists(p string) bool { _, err := os.Stat(p); return err == nil }

// STAND-IN CHANGE (follow-up 5, item 2g): a slow but alive compositor. While the file <faultDir>/driftwm.slow exists, an answer
// is delayed by 3 to 4 seconds: the file says "both" (the "state" answer and the sync answer are late) or "sync" (only the sync
// answer is late). The process keeps running and every connection has its own goroutine, so a late answer does not block the
// other probe. Nothing else about the stand-in changes.
func slowDelay(kind string) {
	b, err := os.ReadFile(faultDir + "/driftwm.slow")
	if err != nil {
		return
	}
	if m := strings.TrimSpace(string(b)); m == "both" || m == kind {
		time.Sleep(3*time.Second + time.Duration(rand.Intn(1000))*time.Millisecond)
	}
}

func logf(name, f string, a ...any) {
	fmt.Printf("%s: %s\n", name, fmt.Sprintf(f, a...))
}

// notify writes the readiness line to fd (s6 notification-fd, dinit pipefd, our own contract).
func notify(fd int) {
	if fd > 0 {
		syscall.Write(fd, []byte("\n"))
		syscall.Close(fd)
	}
}

func waitFor(name string, paths []string, max time.Duration) bool {
	end := time.Now().Add(max)
	for _, p := range paths {
		for p != "" && !exists(p) {
			if time.Now().After(end) {
				return false
			}
			time.Sleep(200 * time.Millisecond)
		}
	}
	return true
}

func listen(p string) net.Listener {
	os.MkdirAll(p[:strings.LastIndex(p, "/")], 0o755)
	os.Remove(p)
	l, err := net.Listen("unix", p)
	if err != nil {
		fmt.Println("listen:", err)
		os.Exit(1)
	}
	return l
}

// runService is the stand-in. Faults are files in /run/fault named NAME.crash, NAME.slow (seconds), NAME.spin.
func runService(name string, args []string) {
	fs := flag.NewFlagSet(name, flag.ExitOnError)
	nfd := fs.Int("notify", 0, "readiness fd")
	wait := fs.String("wait", "", "comma list of paths to wait for before starting (simulates a run-script wait loop)")
	rate := fs.Int("log", 2, "log lines per second")
	fs.String("backend", "", "ignored; lets the stand-in be started like the real driftwm (--backend udev)")
	mem := fs.Int("mem", 0, "MB of memory to hold (touched), so the service has a realistic size for the OOM test")
	fs.Parse(args)
	if *mem > 0 {
		held = make([]byte, *mem<<20)
		for i := 0; i < len(held); i += 4096 {
			held[i] = 1
		}
	}

	os.MkdirAll(faultDir, 0o755)
	// The real programs are single-instance (one compositor can own the seat and the display, a second udevd, seatd or
	// dbus-daemon refuses to start, hubd cannot bind its socket twice). A second copy of a stand-in exits at once, so an
	// init that starts a service again while the old copy still runs (after its supervisor died) is not rewarded with a
	// harmless duplicate.
	lock, err := os.OpenFile(faultDir+"/"+name+".lock", os.O_CREATE|os.O_RDWR, 0o644)
	if err == nil && syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB) != nil {
		logf(name, "already running, exiting")
		os.Exit(1)
	}
	if f, err := os.OpenFile(faultDir+"/"+name+".starts", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644); err == nil {
		f.WriteString("x")
		f.Close()
	}
	logf(name, "starting pid %d", os.Getpid())
	if exists(faultDir + "/" + name + ".crash") {
		logf(name, "fault: crash on start")
		time.Sleep(50 * time.Millisecond)
		os.Exit(1)
	}
	if *wait != "" && !waitFor(name, strings.Split(*wait, ","), 300*time.Second) {
		logf(name, "gave up waiting")
		os.Exit(1)
	}
	if b, err := os.ReadFile(faultDir + "/" + name + ".slow"); err == nil {
		var s int
		fmt.Sscan(strings.TrimSpace(string(b)), &s)
		logf(name, "fault: slow start %ds", s)
		time.Sleep(time.Duration(s) * time.Second)
	}
	hung := &hangState{}
	go func() { // heartbeat log lines, and the spin fault
		t := time.NewTicker(time.Second / time.Duration(max(*rate, 1)))
		seq := 0
		for range t.C {
			seq++
			// STAND-IN CHANGE (follow-up 4, item 4): pid and sequence number in the log line, so that a lost or doubled
			// line in the log service can be counted. Nothing else about the heartbeat changed.
			logf(name, "heartbeat pid=%d seq=%d", os.Getpid(), seq)
			if exists(faultDir+"/"+name+".deadlock") && !hung.get() { // follow-up 4: a hang with no CPU use (a deadlock), State S
				hung.set()
				os.Remove(faultDir + "/" + name + ".deadlock")
				logf(name, "fault: deadlock, no longer answers")
			}
			if exists(faultDir+"/"+name+".spin") && !hung.get() {
				hung.set()
				os.Remove(faultDir + "/" + name + ".spin") // one-shot: it belongs to this process only
				logf(name, "fault: busy loop, no longer answers")
				go func() {
					for {
					}
				}()
			}
		}
	}()
	switch name {
	case "seatd":
		l := listen(seatSock)
		notify(*nfd)
		for {
			c, err := l.Accept()
			if err == nil {
				go func() {
					buf := make([]byte, 64)
					for {
						if _, e := c.Read(buf); e != nil {
							return
						}
					}
				}()
			}
		}
	case "udevd":
		os.MkdirAll("/run/udev", 0o755)
		os.Remove(udevFlag)
		time.Sleep(1500 * time.Millisecond) // trigger + settle
		os.WriteFile(udevFlag, nil, 0o644)
		logf(name, "devices triggered")
		notify(*nfd)
		select {}
	case "dbus":
		l := listen(dbusSock)
		notify(*nfd)
		for {
			c, err := l.Accept()
			if err == nil {
				go func() {
					buf := make([]byte, 64)
					for {
						if _, e := c.Read(buf); e != nil {
							return
						}
					}
				}()
			}
		}
	case "driftwm":
		compositor(name, *nfd, hung)
	case "waybar", "hubd":
		client(name, *nfd, hung)
	}
}

type hangState struct {
	mu sync.Mutex
	v  bool
}

func (h *hangState) get() bool { h.mu.Lock(); defer h.mu.Unlock(); return h.v }
func (h *hangState) set()      { h.mu.Lock(); h.v = true; h.mu.Unlock() }

// compositor: holds a seat connection (exits if seatd goes away, like a real compositor), serves the wayland-like
// socket: "hello NAME" registers a client for as long as the connection lives, "state" answers the client list.
func compositor(name string, nfd int, hung *hangState) {
	seat, err := net.Dial("unix", seatSock)
	if err != nil {
		logf(name, "no seat: %v", err)
		os.Exit(1)
	}
	if !exists(udevFlag) {
		logf(name, "no udev database")
		os.Exit(1)
	}
	go func() {
		buf := make([]byte, 16)
		for {
			if _, e := seat.Read(buf); e != nil {
				logf(name, "seat lost, exiting")
				os.Exit(1)
			}
		}
	}()
	time.Sleep(1500 * time.Millisecond) // renderer start-up
	l := listen(waySock)
	var mu sync.Mutex
	clients := map[net.Conn]string{}
	notify(nfd)
	logf(name, "ready")
	for {
		c, err := l.Accept()
		if err != nil {
			continue
		}
		go func() {
			defer func() { mu.Lock(); delete(clients, c); mu.Unlock(); c.Close() }()
			r := bufio.NewReader(c)
			for {
				// STAND-IN CHANGE (follow-up 4, item 1a): answer a Wayland wl_display.sync on this same socket and in this same
				// handler as "state", behind the same hung check, so that anything that freezes or hangs the "state" handler
				// (SIGSTOP, the busy loop, the deadlock fault) freezes the sync answer too. Only this one request is understood;
				// the stand-in speaks no other Wayland. A Wayland message starts with the object id (1 = wl_display, first byte
				// 0x01); the text requests start with a letter, so the two cannot be confused.
				if b, err := r.Peek(1); err != nil {
					return
				} else if b[0] == 1 {
					var m [12]byte
					if _, err := io.ReadFull(r, m[:]); err != nil {
						return
					}
					if hung.get() {
						select {} // alive but not answering
					}
					slowDelay("sync")
					if binary.LittleEndian.Uint32(m[4:8])&0xffff == 0 { // opcode 0 = sync; m[8:12] is the callback's new id
						var out [24]byte
						binary.LittleEndian.PutUint32(out[0:], 1) // wl_display.delete_id(callback id)
						binary.LittleEndian.PutUint32(out[4:], 12<<16|1)
						copy(out[8:12], m[8:12])
						copy(out[12:16], m[8:12]) // wl_callback.done(serial) on the callback object
						binary.LittleEndian.PutUint32(out[16:], 12<<16|0)
						binary.LittleEndian.PutUint32(out[20:], 1)
						c.Write(out[:])
					}
					continue
				}
				line, err := r.ReadString('\n')
				if err != nil {
					return
				}
				if hung.get() {
					select {} // alive but not answering
				}
				line = strings.TrimSpace(line)
				switch {
				case strings.HasPrefix(line, "hello "):
					mu.Lock()
					clients[c] = strings.TrimPrefix(line, "hello ")
					mu.Unlock()
				case line == "state":
					slowDelay("state")
					mu.Lock()
					var n []string
					for _, v := range clients {
						n = append(n, v)
					}
					mu.Unlock()
					sort.Strings(n)
					fmt.Fprintf(c, "ok clients=%s\n", strings.Join(n, ","))
				}
			}
		}()
	}
}

// client: waybar exits when the compositor connection drops (a Wayland client does); hubd does not: it stays
// running with a stale connection until somebody restarts it (BELIEVED real behaviour, see the proposal).
func client(name string, nfd int, hung *hangState) {
	if !exists(dbusSock) && name == "waybar" {
		logf(name, "no session bus")
		os.Exit(1)
	}
	c, err := net.Dial("unix", waySock)
	if err != nil {
		logf(name, "no compositor: %v", err)
		os.Exit(1)
	}
	fmt.Fprintf(c, "hello %s\n", name)
	time.Sleep(500 * time.Millisecond)
	connected := true
	var mu sync.Mutex
	// STAND-IN CHANGE (follow-up 8): the stand-in hubd now reconnects to the compositor when it comes back (every 500 ms) and registers
	// again, as docs/hubd-slice2.md section 18 says the real hubd does ("brings the windows back by itself"; SOURCE, not tested with the
	// real hubd). Before this change it stayed "stale" for ever, which made the harness's desktop check fail after every compositor
	// restart in the candidates that start hubd the image's way (follow-driftwm --once). waybar is unchanged (it exits).
	go func() {
		for {
			buf := make([]byte, 64)
			for {
				if _, e := c.Read(buf); e != nil {
					break
				}
			}
			if name == "waybar" {
				logf(name, "compositor connection lost, exiting")
				os.Exit(1)
			}
			logf(name, "compositor connection lost (stale, still running)")
			mu.Lock()
			connected = false
			mu.Unlock()
			for {
				time.Sleep(500 * time.Millisecond)
				nc, err := net.Dial("unix", waySock)
				if err != nil {
					continue
				}
				c = nc
				fmt.Fprintf(c, "hello %s\n", name)
				mu.Lock()
				connected = true
				mu.Unlock()
				logf(name, "compositor connection re-established")
				break
			}
		}
	}()
	if name == "hubd" {
		l := listen(hubdSock)
		os.Chmod(hubdSock, 0o666)
		go func() {
			for {
				cc, err := l.Accept()
				if err != nil {
					continue
				}
				go func() {
					defer cc.Close()
					if hung.get() {
						select {}
					}
					mu.Lock()
					ok := connected
					mu.Unlock()
					if ok {
						fmt.Fprintln(cc, "ok")
					} else {
						fmt.Fprintln(cc, "stale")
					}
				}()
			}
		}()
	}
	notify(nfd)
	logf(name, "ready")
	select {}
}
