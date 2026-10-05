// Command hubsim is the test stand-in for the hub's six services and the fault injector used by
// tools/bench/init. It is BENCH CODE: it never goes into an image and is not a design.
//
// One static binary, many names (like busybox). Called as seatd, udevd, dbus, driftwm, waybar or
// hubd it behaves like that service as far as an init can see (start order, readiness, sockets,
// what happens when a dependency goes away). Called as "hubsim SUBCOMMAND" it is a tool:
//
//	hubsim chaos   fault injector and invariant checker (runs in the virtual machine)
//	hubsim guard   the minimal watchdog guard (kernel and PID 1 only, never service health)
//	hubsim probe   one health probe of the compositor ("driftwm msg state" stand-in)
//	hubsim hog     allocate memory until killed (out-of-memory test)
//	hubsim crash1  make PID 1 segfault (ptrace) or hold it stopped
//	hubsim nice    set the CPU priority (nice value) and exec a program: busybox here has no nice applet
//	hubsim ctl     send one line to a unix socket and print the reply
package main

import (
	"fmt"
	"os"
	"path/filepath"
)

var services = map[string]bool{"seatd": true, "udevd": true, "dbus": true, "driftwm": true, "waybar": true, "hubd": true}

func main() {
	name := filepath.Base(os.Args[0])
	if services[name] {
		runService(name, os.Args[1:])
		return
	}
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: hubsim chaos|guard|probe|hog|crash1 ...")
		os.Exit(2)
	}
	args := os.Args[2:]
	switch os.Args[1] {
	case "chaos":
		chaos(args)
	case "guard":
		guard(args)
	case "probe":
		os.Exit(probeCmd(args))
	case "hog":
		hog(args)
	case "crash1":
		crash1(args)
	case "ctl":
		ctl(args)
	case "nice":
		niceExec(args)
	default:
		fmt.Fprintln(os.Stderr, "unknown subcommand", os.Args[1])
		os.Exit(2)
	}
}
