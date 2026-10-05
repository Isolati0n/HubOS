package main

import (
	"bufio"
	"net"
	"strings"
	"time"
)

// Probe asks a service a question on a unix socket and wants an answer starting with "ok" within a second. A process
// that is alive but stopped or looping does not answer, which is exactly what s6 cannot see.
func Probe(sock, request string) bool {
	c, err := net.DialTimeout("unix", sock, time.Second)
	if err != nil {
		return false
	}
	defer c.Close()
	c.SetDeadline(time.Now().Add(time.Second))
	if _, err := c.Write([]byte(request + "\n")); err != nil {
		return false
	}
	line, err := bufio.NewReader(c).ReadString('\n')
	return err == nil && strings.HasPrefix(line, "ok")
}

// Prober counts consecutive failures so one slow answer does not cause a restart.
type Prober struct {
	Sock, Request string
	Grace         time.Duration // no probing for this long after a (re)start
	Need          int           // consecutive failures before the service counts as wedged
	bad           int
}

func (p *Prober) Wedged(sinceStart time.Duration) bool {
	if p.Sock == "" || sinceStart < p.Grace {
		p.bad = 0
		return false
	}
	if Probe(p.Sock, p.Request) {
		p.bad = 0
		return false
	}
	p.bad++
	if p.bad >= p.Need {
		p.bad = 0
		return true
	}
	return false
}
