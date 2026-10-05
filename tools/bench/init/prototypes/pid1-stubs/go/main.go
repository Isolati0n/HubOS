// MEASUREMENT STUB, not an init and not for the image (see ../stub.c).
package main

import (
	"os"
	"os/signal"
	"syscall"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "exit" {
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "panic" {
		var m map[string]int
		m["x"] = 1 // a Go panic
	}
	ch := make(chan os.Signal, 8)
	signal.Notify(ch, syscall.SIGCHLD)
	for range ch {
		for {
			if p, _ := syscall.Wait4(-1, nil, syscall.WNOHANG, nil); p <= 0 {
				break
			}
		}
	}
}
