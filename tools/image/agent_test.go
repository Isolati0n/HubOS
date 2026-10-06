//go:build qemu

package image

// Helper for the recovery agent test (T18 in TestImage): plain (unsigned) requests to the agent that runs inside the TEST
// recovery kernel (tools/image/experiments/recoveryagent). Requests are not signed (owner decision); only the IMAGE
// signature of the bundle named in an install request is checked, by hubos-ctl inside the recovery kernel.

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
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

func (a *agentClient) do(method, uri string, body []byte) (int, string) {
	req, _ := http.NewRequest(method, a.url+uri, bytes.NewReader(body))
	resp, err := a.http.Do(req)
	if err != nil {
		return -1, err.Error()
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, strings.TrimSpace(string(b))
}

func newAgentClient(t *testing.T, r *rig) *agentClient {
	return &agentClient{t: t, r: r, url: fmt.Sprintf("http://127.0.0.1:%d", r.fwd), http: &http.Client{Timeout: 25 * time.Minute}}
}
