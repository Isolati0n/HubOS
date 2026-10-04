//go:build qemu

package image

// Helpers for the recovery agent test (T18 in TestImage): signed requests to the agent that runs inside the TEST recovery
// kernel (tools/image/experiments/recoveryagent). The request format is the one of the agent's SignedMessage:
//   Authorization: HubOS-Sig nonce=HEX, sig=BASE64   over   "hubos-recovery-v1\nMETHOD\nREQUEST-URI\nNONCE\nsha256(body) in hex"
// signed with signify-openbsd (an Ed25519 signature in signify's format), by a management key whose public half is in the
// test recovery kernel (/etc/hubos/mgmt).

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type agentClient struct {
	t    *testing.T
	r    *rig
	url  string // http://127.0.0.1:PORT (QEMU forwards it to port 8480 of the guest)
	http *http.Client
}

func (a *agentClient) do(method, uri, auth string, body []byte) (int, string) {
	req, _ := http.NewRequest(method, a.url+uri, bytes.NewReader(body))
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}
	resp, err := a.http.Do(req)
	if err != nil {
		return -1, err.Error()
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, strings.TrimSpace(string(b))
}

func (a *agentClient) nonce() string {
	code, body := a.do("GET", "/v1/challenge", "", nil)
	var v struct{ Nonce string }
	if code != 200 || json.Unmarshal([]byte(body), &v) != nil || v.Nonce == "" {
		a.t.Fatalf("challenge: %d %s", code, body)
	}
	return v.Nonce
}

// sign makes the Authorization header value for one request with the secret key file sec.
func (a *agentClient) sign(sec, method, uri, nonce string, body []byte) string {
	h := sha256.Sum256(body)
	msg := "hubos-recovery-v1\n" + method + "\n" + uri + "\n" + nonce + "\n" + hex.EncodeToString(h[:])
	dir := a.t.TempDir()
	mf, sf := filepath.Join(dir, "msg"), filepath.Join(dir, "msg.sig")
	os.WriteFile(mf, []byte(msg), 0o644)
	if out, err := exec.Command(a.r.tool("bin/signify-openbsd"), "-S", "-s", sec, "-m", mf, "-x", sf).CombinedOutput(); err != nil {
		a.t.Fatalf("signify -S: %v %s", err, out)
	}
	raw, _ := os.ReadFile(sf)
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	return fmt.Sprintf("HubOS-Sig nonce=%s, sig=%s", nonce, strings.TrimSpace(lines[1]))
}

// signed does a whole signed request: challenge, signature, request.
func (a *agentClient) signed(sec, method, uri string, body []byte) (int, string) {
	n := a.nonce()
	return a.do(method, uri, a.sign(sec, method, uri, n, body), body)
}

func newAgentClient(t *testing.T, r *rig) *agentClient {
	return &agentClient{t: t, r: r, url: fmt.Sprintf("http://127.0.0.1:%d", r.fwd), http: &http.Client{Timeout: 25 * time.Minute}}
}
