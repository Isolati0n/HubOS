package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
)

// fake is the stand-in backend of the experiment: it changes nothing.
type fake struct{ failures int }

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
	keydir := flag.String("keys", "", "directory with management public keys (*.pub, signify format)")
	flag.Parse()
	files, _ := filepath.Glob(filepath.Join(*keydir, "*.pub"))
	var keys []PublicKey
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			log.Fatal(err)
		}
		k, err := ParsePublicKey(b)
		if err != nil {
			log.Fatalf("%s: %v", f, err)
		}
		keys = append(keys, k)
	}
	log.Printf("recoveryagent (FAKE backend): %d management key(s), listening on %s", len(keys), *listen)
	log.Fatal(http.ListenAndServe(*listen, NewAgent(keys, &fake{failures: 3})))
}
