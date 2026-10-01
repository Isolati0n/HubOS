package main

import (
	"net"
	"testing"
	"time"
)

func canConnect(addr string) bool {
	c, err := net.DialTimeout("tcp", addr, time.Second)
	if err != nil {
		return false
	}
	c.Close()
	return true
}

func TestListenAllAnswersUntilClosed(t *testing.T) {
	ls, err := listenAll([]string{"127.0.0.31:0", "127.0.0.32:0"})
	if err != nil {
		t.Fatal(err)
	}
	for _, l := range ls {
		go serve(l)
	}
	a, b := ls[0].Addr().String(), ls[1].Addr().String()
	if !canConnect(a) || !canConnect(b) {
		t.Fatal("both fake nodes should answer")
	}
	ls[0].Close()
	if canConnect(a) {
		t.Error("stopped fake node should not answer")
	}
	if !canConnect(b) {
		t.Error("the other fake node should still answer")
	}
	ls[1].Close()
}

func TestListenAllIsAllOrNothing(t *testing.T) {
	first, err := net.Listen("tcp", "127.0.0.33:0")
	if err != nil {
		t.Fatal(err)
	}
	taken := first.Addr().String()
	defer first.Close()

	// Find a free port for the first address, then release it.
	probe, err := net.Listen("tcp", "127.0.0.34:0")
	if err != nil {
		t.Fatal(err)
	}
	free := probe.Addr().String()
	probe.Close()

	// The second address is already in use, so listenAll must fail and
	// must not leave the first address open.
	if _, err := listenAll([]string{free, taken}); err == nil {
		t.Fatal("expected an error for an address already in use")
	}
	again, err := net.Listen("tcp", free)
	if err != nil {
		t.Fatalf("first address was left open after the failure: %v", err)
	}
	again.Close()
}
