// Command gostatus is an EXPERIMENT (docs/proposals/language-comparison.md): the smallest version of the
// recovery agent's status endpoint, GET /v1/status, written in Go with the standard library only, built the
// way hubd is built (CGO_ENABLED=0, static). It is not part of any image.
//
// The answer is a JSON object with the machine's release and slot:
//
//	{"release":"7","flavor":"hub","slot":"a"}
//
// The release and flavor come from the "version=" and "flavor=" lines of a release file (on a machine:
// /etc/hubos-release); the slot comes from the "hubos.slot=" word of the kernel command line (on a machine:
// /proc/cmdline). Both paths can be changed with flags, so the program can be run on a machine that is not Hub OS.
package main

import (
	"encoding/json"
	"flag"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
)

// Status is the answer to GET /v1/status.
type Status struct {
	Release string `json:"release"`
	Flavor  string `json:"flavor"`
	Slot    string `json:"slot"`
}

// parseRelease reads the "version=" and "flavor=" lines of a release file. Missing values are "unknown".
func parseRelease(text string) (release, flavor string) {
	release, flavor = "unknown", "unknown"
	for _, line := range strings.Split(text, "\n") {
		if v, ok := strings.CutPrefix(line, "version="); ok {
			release = strings.TrimSpace(v)
		} else if v, ok := strings.CutPrefix(line, "flavor="); ok {
			flavor = strings.TrimSpace(v)
		}
	}
	return
}

// parseSlot finds the last "hubos.slot=" word of a kernel command line; only "a" and "b" are accepted.
func parseSlot(cmdline string) string {
	slot := "unknown"
	for _, w := range strings.Fields(cmdline) {
		if v, ok := strings.CutPrefix(w, "hubos.slot="); ok {
			slot = "unknown"
			if v == "a" || v == "b" {
				slot = v
			}
		}
	}
	return slot
}

// Server answers the status call. The files are read at every request, so a change shows at once.
type Server struct {
	ReleaseFile string
	CmdlineFile string
	TestCrash   bool // enables /v1/crash and /v1/crash-goroutine (for the experiment only)
}

func (s *Server) status() Status {
	rel, _ := os.ReadFile(s.ReleaseFile)
	cmd, _ := os.ReadFile(s.CmdlineFile)
	var st Status
	st.Release, st.Flavor = parseRelease(string(rel))
	st.Slot = parseSlot(string(cmd))
	return st
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == "GET" && r.URL.Path == "/v1/status":
		b, _ := json.Marshal(s.status())
		w.Header().Set("Content-Type", "application/json")
		w.Write(b)
	case s.TestCrash && r.URL.Path == "/v1/crash":
		panic("test crash in a request handler")
	case s.TestCrash && r.URL.Path == "/v1/crash-goroutine":
		// A panic in a goroutine the handler started is NOT caught by net/http: it ends the whole program.
		go func() { panic("test crash in a goroutine started by a handler") }()
		time.Sleep(time.Second)
	default:
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(404)
		w.Write([]byte(`{"error":"no such endpoint"}`))
	}
}

func main() {
	listen := flag.String("listen", "127.0.0.1:8481", "address to listen on")
	rel := flag.String("release-file", "/etc/hubos-release", "release file (version= and flavor= lines)")
	cmd := flag.String("cmdline-file", "/proc/cmdline", "kernel command line file (hubos.slot=a|b)")
	crash := flag.Bool("test-crash", false, "enable the /v1/crash test endpoints (experiment only)")
	flag.Parse()
	ln, err := net.Listen("tcp", *listen)
	if err != nil {
		log.Fatal(err)
	}
	srv := &http.Server{Handler: &Server{ReleaseFile: *rel, CmdlineFile: *cmd, TestCrash: *crash}, ReadHeaderTimeout: 5 * time.Second}
	log.Printf("gostatus: listening on %s", ln.Addr())
	log.Fatal(srv.Serve(ln))
}
