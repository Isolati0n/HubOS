package main

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"sync"
	"time"
	"unicode/utf8"
)

// hashHex is the SHA-256 of the normalised text, in hex. The hub's guard can
// remember it without keeping the text.
func hashHex(text string) string {
	h := Hash(text)
	return hex.EncodeToString(h[:])
}

func b64(b []byte) string { return base64.StdEncoding.EncodeToString(b) }

// NodeAPI is the clipboard part of the node helper's API, with the signing left
// out (the signed message is described in the proposal and was tested in the
// recovery agent experiment, not here).
//
//	GET /v1/clipboard   -> {"present":true,"text":"..."}
//	PUT /v1/clipboard   <- {"text":"...","origin":"hub"}  -> {"result":"set"|"unchanged"}
type NodeAPI struct {
	Clip Clipboard
	Max  int
	// AlwaysSet turns off the "unchanged" rule, to show the echo it prevents.
	AlwaysSet bool
	// RateMax sets per RateWindow are allowed; more get 429.
	RateMax    int
	RateWindow time.Duration
	Now        func() time.Time

	mu    sync.Mutex
	times []time.Time
}

type errBody struct {
	Code  string `json:"code"`
	Error string `json:"error"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func fail(w http.ResponseWriter, status int, code, msg string) {
	writeJSON(w, status, errBody{Code: code, Error: msg})
}

func (a *NodeAPI) rateOK() bool {
	if a.RateMax <= 0 {
		return true
	}
	now := a.Now()
	a.mu.Lock()
	defer a.mu.Unlock()
	keep := a.times[:0]
	for _, t := range a.times {
		if now.Sub(t) < a.RateWindow {
			keep = append(keep, t)
		}
	}
	a.times = keep
	if len(a.times) >= a.RateMax {
		return false
	}
	a.times = append(a.times, now)
	return true
}

// ServeHTTP implements the two calls.
func (a *NodeAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/v1/clipboard" {
		fail(w, http.StatusNotFound, "not_found", "no such endpoint")
		return
	}
	switch r.Method {
	case http.MethodGet:
		text, err := a.Clip.Get(a.Max)
		switch {
		case errors.Is(err, ErrNoSelect):
			writeJSON(w, 200, map[string]any{"present": false, "text": ""})
		case errors.Is(err, ErrTooBig):
			fail(w, http.StatusRequestEntityTooLarge, "too_large", "clipboard text is over the limit")
		case err != nil:
			fail(w, http.StatusServiceUnavailable, "clipboard_unavailable", err.Error())
		default:
			writeJSON(w, 200, map[string]any{"present": true, "text": text, "bytes": len(text), "sha256": hashHex(text)})
		}
	case http.MethodPut:
		if !a.rateOK() {
			fail(w, http.StatusTooManyRequests, "rate_limited", "too many clipboard sets")
			return
		}
		// JSON can write one byte as six (\u0001), so the body may be six times the text.
		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, int64(6*a.Max+4096)))
		if err != nil {
			fail(w, http.StatusRequestEntityTooLarge, "too_large", "request body too large")
			return
		}
		if !utf8.Valid(body) {
			// encoding/json would silently turn bad bytes into U+FFFD
			fail(w, http.StatusUnprocessableEntity, "bad_text", ErrNotText.Error())
			return
		}
		var req struct {
			Text   string `json:"text"`
			Origin string `json:"origin"`
		}
		if json.Unmarshal(body, &req) != nil {
			fail(w, http.StatusBadRequest, "bad_request", "need JSON {\"text\":\"...\"}")
			return
		}
		switch err := CheckText(req.Text, a.Max); {
		case errors.Is(err, ErrTooBig):
			fail(w, http.StatusRequestEntityTooLarge, "too_large", err.Error())
			return
		case err != nil:
			fail(w, http.StatusUnprocessableEntity, "bad_text", err.Error())
			return
		}
		changed := true
		if a.AlwaysSet {
			err = a.Clip.Set(req.Text)
		} else {
			changed, err = SetIfChanged(a.Clip, req.Text, a.Max)
		}
		if err != nil {
			fail(w, http.StatusServiceUnavailable, "clipboard_unavailable", err.Error())
			return
		}
		result := "set"
		if !changed {
			result = "unchanged"
		}
		writeJSON(w, 200, map[string]any{"result": result, "bytes": len(req.Text), "sha256": hashHex(req.Text)})
	default:
		fail(w, http.StatusMethodNotAllowed, "method_not_allowed", "GET or PUT")
	}
}
