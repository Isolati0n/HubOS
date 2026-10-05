package main

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// probeCmd: one health probe of the compositor, the stand-in for "driftwm msg state". Exit 0 = answered ok.
func probeCmd(args []string) int {
	sock := waySock
	if len(args) > 0 {
		sock = args[0]
	}
	out, ok := probe(sock, "state", time.Second)
	fmt.Println(out)
	if ok {
		return 0
	}
	return 1
}

func probe(sock, req string, to time.Duration) (string, bool) {
	c, err := net.DialTimeout("unix", sock, to)
	if err != nil {
		return err.Error(), false
	}
	defer c.Close()
	c.SetDeadline(time.Now().Add(to))
	fmt.Fprintln(c, req)
	b := make([]byte, 256)
	n, err := c.Read(b)
	if err != nil {
		return "no answer", false
	}
	s := strings.TrimSpace(string(b[:n]))
	return s, strings.HasPrefix(s, "ok")
}

// hog allocates and touches memory until it is killed. ARG: chunk MB (default 8).
func hog(args []string) {
	os.WriteFile("/proc/self/oom_score_adj", []byte("0"), 0o644)
	var keep [][]byte
	for {
		b := make([]byte, 8<<20)
		for i := 0; i < len(b); i += 4096 {
			b[i] = 1
		}
		keep = append(keep, b)
		time.Sleep(20 * time.Millisecond)
	}
}

// crash1: act on PID 1 through ptrace. "segv" sets the instruction pointer to 0 (PID 1 dies like a buggy init);
// "stop" holds PID 1 stopped (a wedged init) until this process is killed.
func crash1(args []string) {
	runtime.LockOSThread()
	mode := "segv"
	if len(args) > 0 {
		mode = args[0]
	}
	if err := syscall.PtraceAttach(1); err != nil {
		fmt.Println("attach:", err)
		os.Exit(1)
	}
	var ws syscall.WaitStatus
	syscall.Wait4(1, &ws, 0, nil)
	if mode == "stop" {
		fmt.Println("PID 1 held stopped")
		time.Sleep(24 * time.Hour) // not select{}: the runtime would call that a deadlock and exit, releasing PID 1
	}
	var regs syscall.PtraceRegs
	if err := syscall.PtraceGetRegs(1, &regs); err != nil {
		fmt.Println("getregs:", err)
		os.Exit(1)
	}
	regs.Rip = 0
	syscall.PtraceSetRegs(1, &regs)
	syscall.PtraceDetach(1)
	time.Sleep(time.Second)
}

// guard: the minimal watchdog guard. It feeds /dev/watchdog only while the kernel runs (it is scheduled) and PID 1 is
// alive: not stopped or traced, and still reaping an orphaned zombie. It never looks at any service.
func guard(args []string) {
	dev := "/dev/watchdog"
	if len(args) > 0 {
		dev = args[0]
	}
	if len(args) > 1 && args[1] == "zombie" { // internal helper: start a grandchild that exits at once
		cmd := exec.Command(os.Args[0], "guard", dev, "zombie2")
		cmd.Start()
		os.WriteFile("/run/guard.zpid", []byte(strconv.Itoa(cmd.Process.Pid)), 0o644)
		os.Exit(0)
	}
	if len(args) > 1 && args[1] == "zombie2" {
		os.Exit(0)
	}
	w, err := os.OpenFile(dev, os.O_WRONLY, 0)
	if err != nil {
		fmt.Println("guard: cannot open watchdog:", err)
		os.Exit(1)
	}
	bad := 0
	for {
		if pid1Alive() {
			bad = 0
		} else {
			bad++
			fmt.Printf("guard: PID 1 check failed (%d)\n", bad)
		}
		if bad < 3 {
			w.Write([]byte{0})
		}
		time.Sleep(3 * time.Second)
	}
}

func pid1Alive() bool {
	b, err := os.ReadFile("/proc/1/stat")
	if err != nil {
		return false
	}
	s := string(b)
	i := strings.LastIndex(s, ")")
	if i < 0 || len(s) < i+3 {
		return false
	}
	switch s[i+2] {
	case 'T', 't', 'Z', 'X':
		return false
	}
	os.Remove("/run/guard.zpid")
	if err := exec.Command(os.Args[0], "guard", "-", "zombie").Run(); err != nil {
		return true // cannot judge; do not blame PID 1
	}
	zb, err := os.ReadFile("/run/guard.zpid")
	if err != nil {
		return true
	}
	end := time.Now().Add(3 * time.Second)
	for time.Now().Before(end) {
		if st, err := os.ReadFile("/proc/" + string(zb) + "/stat"); err != nil || !strings.Contains(string(st), ") Z") {
			return true
		}
		time.Sleep(100 * time.Millisecond)
	}
	return false
}

// ctl SOCK WORDS...: send one line to a unix socket and print the reply (used by the candidates' /ops scripts).
func ctl(args []string) {
	if len(args) < 2 {
		fmt.Println("usage: hubsim ctl SOCK LINE...")
		os.Exit(2)
	}
	c, err := net.DialTimeout("unix", args[0], 2*time.Second)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	defer c.Close()
	fmt.Fprintln(c, strings.Join(args[1:], " "))
	c.SetReadDeadline(time.Now().Add(5 * time.Second))
	b := make([]byte, 4096)
	for {
		n, err := c.Read(b)
		os.Stdout.Write(b[:n])
		if err != nil {
			return
		}
	}
}
