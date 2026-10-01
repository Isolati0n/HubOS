// Command fakenode pretends to be machines for testing hubd on one computer.
//
// Give it one or more addresses (host:port). It listens on each and, for
// every connection that arrives, closes it straight away. It sends and
// reads nothing, so it is not a protocol of any kind. Start it to make
// machines "up"; stop it (Ctrl+C) to make them "down".
//
//	fakenode 127.0.0.11:21001 127.0.0.12:21002
//
// This is a test helper. It is never part of the Hub OS image.
package main

import (
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: fakenode HOST:PORT [HOST:PORT ...]")
		os.Exit(1)
	}
	listeners, err := listenAll(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, "fakenode:", err)
		os.Exit(1)
	}
	for _, l := range listeners {
		fmt.Println("fakenode: listening on", l.Addr())
		go serve(l)
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	closeAll(listeners)
	fmt.Println("fakenode: stopped")
}

// listenAll opens every address, or none: if one fails, the ones already
// open are closed again.
func listenAll(addrs []string) ([]net.Listener, error) {
	var ls []net.Listener
	for _, a := range addrs {
		l, err := net.Listen("tcp", a)
		if err != nil {
			closeAll(ls)
			return nil, err
		}
		ls = append(ls, l)
	}
	return ls, nil
}

func closeAll(ls []net.Listener) {
	for _, l := range ls {
		l.Close()
	}
}

// serve accepts connections and closes each one at once, until the listener
// is closed.
func serve(l net.Listener) {
	for {
		c, err := l.Accept()
		if err != nil {
			return
		}
		c.Close()
	}
}
