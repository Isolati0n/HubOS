// Command recoveryagent is an EXPERIMENT (docs/proposals/recovery-and-out-of-band.md):
// the smallest piece of a network recovery agent for the Hub OS recovery kernel.
// It is not part of any image. It serves a small JSON API, checks signed
// requests against a keyring of management public keys (signify format), and
// hands the real work to a Backend (a fake one here).
package main

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Backend is what the agent does on the machine. In the recovery kernel it would
// call hubos-ctl; in the experiment it is a fake.
type Backend interface {
	Machine() string // this machine's name (the machine id the hub signs for)
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
	apiVersion  = 1
	minHub      = 1
	proto       = "hubos-recovery-v1"
	maxBody     = 4096
	maxNonces   = 16
	nonceMaxAge = 30 * time.Second
)

// Agent is the HTTP handler.
type Agent struct {
	Keys    []PublicKey
	Backend Backend
	Name    string // this machine's name: the signed text names it, so a request signed for another machine is refused
	Now     func() time.Time

	mu     sync.Mutex
	nonces map[string]time.Time
}

// NewAgent makes an agent with the given management keyring.
func NewAgent(keys []PublicKey, b Backend) *Agent {
	return &Agent{Keys: keys, Backend: b, Name: b.Machine(), Now: time.Now, nonces: map[string]time.Time{}}
}

func (a *Agent) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == "GET" && r.URL.Path == "/v1/status":
		st := a.Backend.Status()
		st.API, st.MinHub = apiVersion, minHub
		writeJSON(w, 200, st)
	case r.Method == "GET" && r.URL.Path == "/v1/challenge":
		n, ok := a.newNonce()
		if !ok {
			writeJSON(w, 503, map[string]string{"error": "too many open challenges"})
			return
		}
		writeJSON(w, 200, map[string]string{"nonce": n})
	case r.Method == "GET" && r.URL.Path == "/v1/logs":
		if !a.authorize(w, r) {
			return
		}
		io.WriteString(w, a.Backend.Logs())
	case r.Method == "POST" && r.URL.Path == "/v1/clear-failures":
		if !a.authorize(w, r) {
			return
		}
		if err := a.Backend.ClearFailures(); err != nil {
			writeJSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, 200, map[string]string{"result": "cleared"})
	case r.Method == "POST" && r.URL.Path == "/v1/install":
		body, ok := a.authorizeBody(w, r)
		if !ok {
			return
		}
		var req struct{ Slot, BaseURL string }
		if json.Unmarshal(body, &req) != nil || (req.Slot != "a" && req.Slot != "b") || !strings.HasPrefix(req.BaseURL, "http") {
			writeJSON(w, 400, map[string]string{"error": "need JSON {\"Slot\":\"a|b\",\"BaseURL\":\"http...\"}"})
			return
		}
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

func (a *Agent) newNonce() (string, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	now := a.Now()
	for n, t := range a.nonces {
		if now.Sub(t) > nonceMaxAge {
			delete(a.nonces, n)
		}
	}
	if len(a.nonces) >= maxNonces {
		return "", false
	}
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", false
	}
	n := hex.EncodeToString(b)
	a.nonces[n] = now
	return n, true
}

// takeNonce uses a nonce up: it works once, and only within nonceMaxAge. The
// agent needs no clock that is correct, only one that runs forward.
func (a *Agent) takeNonce(n string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	t, ok := a.nonces[n]
	delete(a.nonces, n)
	return ok && a.Now().Sub(t) <= nonceMaxAge
}

// SignedMessage is the exact text the hub signs: six lines joined by one "\n", no newline at the end. It binds the protocol
// name, the TARGET MACHINE ("machine=ID"), the method, the request path and query, the one-time nonce and the body's hash.
// The machine line stops a request meant for one machine (or a challenge relayed from another) being accepted by this one.
func SignedMessage(machine, method, requestURI, nonce string, body []byte) []byte {
	h := sha256.Sum256(body)
	return []byte(proto + "\nmachine=" + machine + "\n" + method + "\n" + requestURI + "\n" + nonce + "\n" + hex.EncodeToString(h[:]))
}

func (a *Agent) authorize(w http.ResponseWriter, r *http.Request) bool {
	_, ok := a.authorizeBody(w, r)
	return ok
}

// authorizeBody reads the body (at most maxBody bytes), checks the header
//
//	Authorization: HubOS-Sig nonce=HEX, sig=BASE64            (machine=ID may be added: if it is not this machine's name,
//	                                                            the request is refused with 403 and a clear message)
//
// and returns the body. The nonce is used up whatever the outcome.
func (a *Agent) authorizeBody(w http.ResponseWriter, r *http.Request) ([]byte, bool) {
	deny := func(why string) ([]byte, bool) {
		writeJSON(w, 401, map[string]string{"error": why})
		return nil, false
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, maxBody+1))
	if err != nil || len(body) > maxBody {
		return deny("body missing or too large")
	}
	h := r.Header.Get("Authorization")
	if !strings.HasPrefix(h, "HubOS-Sig ") {
		return deny("signed request required (GET /v1/challenge first)")
	}
	var nonce, sigB64, machine string
	for _, p := range strings.Split(strings.TrimPrefix(h, "HubOS-Sig "), ",") {
		k, v, _ := strings.Cut(strings.TrimSpace(p), "=")
		switch k {
		case "nonce":
			nonce = v
		case "sig":
			sigB64 = v
		case "machine":
			machine = v
		}
	}
	if !a.takeNonce(nonce) {
		return deny("unknown, used or expired nonce")
	}
	if machine != "" && machine != a.Name {
		writeJSON(w, 403, map[string]string{"error": "this request is for another machine"})
		return nil, false
	}
	sig, err := ParseSignatureBlob(sigB64)
	if err != nil {
		return deny("bad signature encoding")
	}
	msg := SignedMessage(a.Name, r.Method, r.URL.RequestURI(), nonce, body)
	for _, k := range a.Keys {
		if k.Verify(msg, sig) {
			return body, true
		}
	}
	return deny("signature not accepted")
}
