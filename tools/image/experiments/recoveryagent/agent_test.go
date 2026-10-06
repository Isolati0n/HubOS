package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type rec struct {
	fake
	installs []string
}

func (r *rec) Install(slot, base string) (string, error) {
	r.installs = append(r.installs, slot+" "+base)
	return "ok", nil
}

func TestStatusShowsRecovery(t *testing.T) {
	srv := httptest.NewServer(NewAgent(&fake{failures: 3}))
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/v1/status")
	if err != nil {
		t.Fatal(err)
	}
	var s Status
	json.NewDecoder(resp.Body).Decode(&s)
	if resp.StatusCode != 200 || s.State != "recovery" || s.BootFailures != 3 {
		t.Fatalf("got %d %+v", resp.StatusCode, s)
	}
	if s.API != 1 || s.MinHub != 1 {
		t.Fatalf("api/min_hub missing: %+v", s)
	}
	// the raw JSON carries the two add-only fields under their documented names
	resp, _ = http.Get(srv.URL + "/v1/status")
	raw, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(raw), `"api":1`) || !strings.Contains(string(raw), `"min_hub":1`) {
		t.Fatalf("status JSON: %s", raw)
	}
}

func do(t *testing.T, method, url, body string) (int, string) {
	t.Helper()
	req, _ := http.NewRequest(method, url, strings.NewReader(body))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b)
}

// Requests are not signed: the plain calls work without any header, and there is no challenge endpoint any more.
func TestCallsNeedNoSignature(t *testing.T) {
	r := &rec{}
	srv := httptest.NewServer(NewAgent(r))
	defer srv.Close()
	if code, out := do(t, "GET", srv.URL+"/v1/logs", ""); code != 200 || !strings.Contains(out, "FAKE log") {
		t.Errorf("logs: %d %q", code, out)
	}
	if code, out := do(t, "POST", srv.URL+"/v1/clear-failures", ""); code != 200 || !strings.Contains(out, "cleared") {
		t.Errorf("clear-failures: %d %q", code, out)
	}
	if code, out := do(t, "POST", srv.URL+"/v1/install", `{"Slot":"b","BaseURL":"http://10.0.0.1/bundle"}`); code != 200 {
		t.Errorf("install: %d %q", code, out)
	}
	if len(r.installs) != 1 || r.installs[0] != "b http://10.0.0.1/bundle" {
		t.Fatalf("installs: %v", r.installs)
	}
	if code, _ := do(t, "GET", srv.URL+"/v1/challenge", ""); code != 404 {
		t.Errorf("/v1/challenge: %d, want 404 (removed)", code)
	}
}

func TestBadInstallRequestRefused(t *testing.T) {
	r := &rec{}
	srv := httptest.NewServer(NewAgent(r))
	defer srv.Close()
	for _, b := range []string{`{"Slot":"c","BaseURL":"http://x"}`, `{"Slot":"a","BaseURL":"file:///x"}`, `nonsense`} {
		if code, _ := do(t, "POST", srv.URL+"/v1/install", b); code != 400 {
			t.Errorf("%s: %d, want 400", b, code)
		}
	}
	if len(r.installs) != 0 {
		t.Fatal("bad request reached the backend")
	}
}

// The install request refuses what the image check refuses: when hubos-ctl (the program that verifies the bundle's signature)
// exits with an error, the agent answers 500 with the program's output and reports no success. The real signature checks are
// tested in tools/image (test 4, R3 and T18); here hubos-ctl is a fake script.
func TestInstallRefusedByHubosCtlIsAnError(t *testing.T) {
	dir := t.TempDir()
	ctl := filepath.Join(dir, "hubos-ctl")
	if err := os.WriteFile(ctl, []byte("#!/bin/sh\necho 'REFUSED: bad signature'; exit 2\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(NewAgent(&HubosBackend{Ctl: ctl, Release: filepath.Join(dir, "r"), LogFile: filepath.Join(dir, "l"), NodeConf: filepath.Join(dir, "n")}))
	defer srv.Close()
	code, out := do(t, "POST", srv.URL+"/v1/install", `{"Slot":"b","BaseURL":"http://x/y"}`)
	if code != 500 || !strings.Contains(out, "REFUSED: bad signature") || strings.Contains(out, "installed") {
		t.Fatalf("%d %s", code, out)
	}
}

// The real backend, with a fake hubos-ctl script standing in for the program of the recovery kernel.
func TestHubosBackend(t *testing.T) {
	dir := t.TempDir()
	ctl := filepath.Join(dir, "hubos-ctl")
	script := "#!/bin/sh\ncase $1 in\n status) echo 'slot=none'; echo 'boot-failures=2 limit=3';;\n clear-failures) echo cleared;;\n update) echo \"update: $2 into $3\"; [ \"$3\" = b ] || exit 2;;\nesac\n"
	if err := os.WriteFile(ctl, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	rel := filepath.Join(dir, "release")
	os.WriteFile(rel, []byte("version=recovery-7\nflavor=recovery\n"), 0o644)
	conf := filepath.Join(dir, "node.conf")
	os.WriteFile(conf, []byte("NAME=\"ai-1\"\nNET=dhcp\n"), 0o644)
	b := &HubosBackend{Ctl: ctl, Release: rel, LogFile: filepath.Join(dir, "log"), NodeConf: conf}
	if b.Machine() != "ai-1" {
		t.Fatalf("machine id %q, want ai-1 (NAME= in node.conf)", b.Machine())
	}
	if st := b.Status(); st.Release != "recovery-7" || st.BootFailures != 2 || st.FailureLimit != 3 || st.State != "recovery" {
		t.Fatalf("status %+v", st)
	}
	if err := b.ClearFailures(); err != nil {
		t.Fatal(err)
	}
	out, err := b.Install("b", "http://x/y")
	if err != nil || !strings.Contains(out, "update: http://x/y into b") {
		t.Fatalf("install: %v %q", err, out)
	}
	if _, err := b.Install("a", "http://x/y"); err == nil {
		t.Fatal("a failing hubos-ctl must be an error")
	}
	if !strings.HasPrefix(b.Logs(), "no log") {
		t.Fatalf("logs %q", b.Logs())
	}
}
