// Command recoveryagent is an EXPERIMENT (docs/proposals/recovery-and-out-of-band.md):
// the smallest piece of a network recovery agent for the Hub OS recovery kernel.
// It is not part of any image. It serves a small JSON API and hands the real work
// to a Backend (a fake one here). Requests are NOT signed (owner decision): the
// only signature that matters is the IMAGE signature, which hubos-ctl checks in
// the install request (a bundle not signed by the owner's update key is refused).
package main

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

// Backend is what the agent does on the machine. In the recovery kernel it would
// call hubos-ctl; in the experiment it is a fake.
type Backend interface {
	Machine() string // this machine's name (the machine id shown in the status)
	Status() Status
	ClearFailures() error
	Install(slot, baseURL string) (string, error)
	Logs() string
}

// Status is the answer to GET /v1/status.
type Status struct {
	API          int    `json:"api"`     // version of this API (add-only inside a major)
	MinHub       int    `json:"min_hub"` // the lowest hub API version this agent works with
	Machine      string `json:"machine"`
	State        string `json:"state"` // always "recovery" in the recovery kernel
	Release      string `json:"recovery_release"`
	BootFailures int    `json:"boot_failures"`
	FailureLimit int    `json:"failure_limit"`
}

const (
	apiVersion = 1
	minHub     = 1
	maxBody    = 4096
)

// Agent is the HTTP handler.
type Agent struct {
	Backend Backend
}

// NewAgent makes an agent on a backend.
func NewAgent(b Backend) *Agent {
	return &Agent{Backend: b}
}

func (a *Agent) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == "GET" && r.URL.Path == "/v1/status":
		st := a.Backend.Status()
		st.API, st.MinHub = apiVersion, minHub
		writeJSON(w, 200, st)
	case r.Method == "GET" && r.URL.Path == "/v1/logs":
		io.WriteString(w, a.Backend.Logs())
	case r.Method == "POST" && r.URL.Path == "/v1/clear-failures":
		if err := a.Backend.ClearFailures(); err != nil {
			writeJSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, 200, map[string]string{"result": "cleared"})
	case r.Method == "POST" && r.URL.Path == "/v1/install":
		body, err := io.ReadAll(io.LimitReader(r.Body, maxBody+1))
		if err != nil || len(body) > maxBody {
			writeJSON(w, 400, map[string]string{"error": "body missing or too large"})
			return
		}
		var req struct{ Slot, BaseURL string }
		if json.Unmarshal(body, &req) != nil || (req.Slot != "a" && req.Slot != "b") || !strings.HasPrefix(req.BaseURL, "http") {
			writeJSON(w, 400, map[string]string{"error": "need JSON {\"Slot\":\"a|b\",\"BaseURL\":\"http...\"}"})
			return
		}
		// hubos-ctl checks the bundle's signature, floor and hashes; the agent adds nothing to that.
		out, err := a.Backend.Install(req.Slot, req.BaseURL)
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": err.Error(), "output": out})
			return
		}
		writeJSON(w, 200, map[string]string{"result": "installed", "output": out})
	default:
		writeJSON(w, 404, map[string]string{"error": "no such endpoint"})
	}
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}
