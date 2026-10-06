package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
)

// fake is the stand-in backend of the experiment: it changes nothing.
type fake struct{ failures int }

func (f *fake) Machine() string { return "fake-1" }
func (f *fake) Status() Status {
	return Status{Machine: "fake-1", State: "recovery", Release: "recovery-1", BootFailures: f.failures, FailureLimit: 3}
}
func (f *fake) ClearFailures() error { f.failures = 0; return nil }
func (f *fake) Install(slot, baseURL string) (string, error) {
	return fmt.Sprintf("FAKE: would run: hubos-ctl update %s %s", baseURL, slot), nil
}
func (f *fake) Logs() string { return "FAKE log line 1\nFAKE log line 2\n" }

func main() {
	listen := flag.String("listen", "127.0.0.1:8480", "address to listen on")
	backend := flag.String("backend", "fake", "fake (changes nothing) or hubos (calls hubos-ctl; for the TEST recovery kernel)")
	flag.Parse()
	var b Backend = &fake{failures: 3}
	if *backend == "hubos" {
		b = NewHubosBackend()
	}
	log.Printf("recoveryagent (%s backend), listening on %s", *backend, *listen)
	log.Fatal(http.ListenAndServe(*listen, NewAgent(b)))
}
