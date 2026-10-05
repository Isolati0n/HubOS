package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func newKey(t *testing.T, num byte) (PublicKey, ed25519.PrivateKey) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	k := PublicKey{Key: pub}
	for i := range k.KeyNum {
		k.KeyNum[i] = num
	}
	return k, priv
}

func sigBlob(k PublicKey, priv ed25519.PrivateKey, msg []byte) string {
	raw := append([]byte(pkAlg), k.KeyNum[:]...)
	raw = append(raw, ed25519.Sign(priv, msg)...)
	return base64.StdEncoding.EncodeToString(raw)
}

type rec struct {
	fake
	installs []string
}

func (r *rec) Install(slot, base string) (string, error) {
	r.installs = append(r.installs, slot+" "+base)
	return "ok", nil
}

func challenge(t *testing.T, srv *httptest.Server) string {
	t.Helper()
	resp, err := http.Get(srv.URL + "/v1/challenge")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var c struct{ Nonce string }
	json.NewDecoder(resp.Body).Decode(&c)
	if c.Nonce == "" {
		t.Fatal("no nonce")
	}
	return c.Nonce
}

func signedDo(t *testing.T, srv *httptest.Server, k PublicKey, priv ed25519.PrivateKey, nonce, method, uri, body string) (int, string) {
	t.Helper()
	req, _ := http.NewRequest(method, srv.URL+uri, strings.NewReader(body))
	req.Header.Set("Authorization", "HubOS-Sig nonce="+nonce+", sig="+sigBlob(k, priv, SignedMessage("fake-1", method, uri, nonce, []byte(body))))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b)
}

func TestStatusNeedsNoSignatureAndShowsRecovery(t *testing.T) {
	srv := httptest.NewServer(NewAgent(nil, &fake{failures: 3}))
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

func TestMutatingCallsNeedASignature(t *testing.T) {
	r := &rec{}
	srv := httptest.NewServer(NewAgent(nil, r))
	defer srv.Close()
	for _, ep := range []struct{ m, p string }{{"GET", "/v1/logs"}, {"POST", "/v1/clear-failures"}, {"POST", "/v1/install"}} {
		req, _ := http.NewRequest(ep.m, srv.URL+ep.p, strings.NewReader(`{"Slot":"a","BaseURL":"http://x"}`))
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != 401 {
			t.Errorf("%s %s without signature: %d, want 401", ep.m, ep.p, resp.StatusCode)
		}
	}
	if len(r.installs) != 0 {
		t.Fatal("install ran without a signature")
	}
}

func TestSignedInstallWorksOnceAndIsBound(t *testing.T) {
	k, priv := newKey(t, 1)
	r := &rec{}
	srv := httptest.NewServer(NewAgent([]PublicKey{k}, r))
	defer srv.Close()
	body := `{"Slot":"b","BaseURL":"http://10.0.0.1/bundle"}`

	n := challenge(t, srv)
	if code, out := signedDo(t, srv, k, priv, n, "POST", "/v1/install", body); code != 200 {
		t.Fatalf("good request: %d %s", code, out)
	}
	if len(r.installs) != 1 || r.installs[0] != "b http://10.0.0.1/bundle" {
		t.Fatalf("installs: %v", r.installs)
	}
	// replay of the same nonce
	if code, _ := signedDo(t, srv, k, priv, n, "POST", "/v1/install", body); code != 401 {
		t.Errorf("replayed nonce: %d, want 401", code)
	}
	// a signature for one path/body does not work for another
	n = challenge(t, srv)
	req, _ := http.NewRequest("POST", srv.URL+"/v1/install", strings.NewReader(`{"Slot":"a","BaseURL":"http://evil/"}`))
	req.Header.Set("Authorization", "HubOS-Sig nonce="+n+", sig="+sigBlob(k, priv, SignedMessage("fake-1", "POST", "/v1/install", n, []byte(body))))
	resp, _ := http.DefaultClient.Do(req)
	if resp.StatusCode != 401 {
		t.Errorf("changed body: %d, want 401", resp.StatusCode)
	}
	n = challenge(t, srv)
	req, _ = http.NewRequest("POST", srv.URL+"/v1/clear-failures", nil)
	req.Header.Set("Authorization", "HubOS-Sig nonce="+n+", sig="+sigBlob(k, priv, SignedMessage("fake-1", "POST", "/v1/install", n, nil)))
	resp, _ = http.DefaultClient.Do(req)
	if resp.StatusCode != 401 {
		t.Errorf("changed path: %d, want 401", resp.StatusCode)
	}
	if len(r.installs) != 1 {
		t.Fatalf("installs after attacks: %v", r.installs)
	}
}

func TestWrongKeyAndMadeUpNonceRefused(t *testing.T) {
	good, _ := newKey(t, 1)
	bad, badPriv := newKey(t, 1) // same key number, different key
	srv := httptest.NewServer(NewAgent([]PublicKey{good}, &rec{}))
	defer srv.Close()
	n := challenge(t, srv)
	if code, _ := signedDo(t, srv, bad, badPriv, n, "POST", "/v1/clear-failures", ""); code != 401 {
		t.Errorf("wrong key: %d", code)
	}
	if code, _ := signedDo(t, srv, bad, badPriv, "00112233445566778899aabbccddeeff", "POST", "/v1/clear-failures", ""); code != 401 {
		t.Errorf("made-up nonce: %d", code)
	}
}

func TestSecondKeyInKeyringWorks(t *testing.T) {
	k1, _ := newKey(t, 1)
	k2, p2 := newKey(t, 2)
	srv := httptest.NewServer(NewAgent([]PublicKey{k1, k2}, &rec{}))
	defer srv.Close()
	if code, out := signedDo(t, srv, k2, p2, challenge(t, srv), "POST", "/v1/clear-failures", ""); code != 200 {
		t.Fatalf("%d %s", code, out)
	}
}

func TestNonceExpires(t *testing.T) {
	k, priv := newKey(t, 1)
	a := NewAgent([]PublicKey{k}, &rec{})
	now := time.Unix(1000, 0)
	a.Now = func() time.Time { return now }
	srv := httptest.NewServer(a)
	defer srv.Close()
	n := challenge(t, srv)
	now = now.Add(nonceMaxAge + time.Second)
	if code, _ := signedDo(t, srv, k, priv, n, "POST", "/v1/clear-failures", ""); code != 401 {
		t.Errorf("expired nonce: %d, want 401", code)
	}
}

func TestChallengesAreLimited(t *testing.T) {
	srv := httptest.NewServer(NewAgent(nil, &rec{}))
	defer srv.Close()
	for i := 0; i < maxNonces; i++ {
		challenge(t, srv)
	}
	resp, _ := http.Get(srv.URL + "/v1/challenge")
	if resp.StatusCode != 503 {
		t.Errorf("challenge %d: %d, want 503", maxNonces+1, resp.StatusCode)
	}
}

func TestBadInstallRequestRefusedAfterAuth(t *testing.T) {
	k, priv := newKey(t, 1)
	r := &rec{}
	srv := httptest.NewServer(NewAgent([]PublicKey{k}, r))
	defer srv.Close()
	for _, b := range []string{`{"Slot":"c","BaseURL":"http://x"}`, `{"Slot":"a","BaseURL":"file:///x"}`, `nonsense`} {
		if code, _ := signedDo(t, srv, k, priv, challenge(t, srv), "POST", "/v1/install", b); code != 400 {
			t.Errorf("%s: %d, want 400", b, code)
		}
	}
	if len(r.installs) != 0 {
		t.Fatal("bad request reached the backend")
	}
}

// Interop with the real program the recovery kernel carries. It runs only when
// signify-openbsd is on PATH; otherwise it is skipped (and says so).
func TestRealSignifyInterop(t *testing.T) {
	sig, err := exec.LookPath("signify-openbsd")
	if err != nil {
		t.Skip("signify-openbsd is not on PATH; interop with the real program NOT tested")
	}
	d := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		if out, err := exec.Command(sig, args...).CombinedOutput(); err != nil {
			t.Fatalf("signify %v: %v: %s", args, err, out)
		}
	}
	run("-G", "-n", "-p", filepath.Join(d, "k.pub"), "-s", filepath.Join(d, "k.sec"))
	pubText, _ := os.ReadFile(filepath.Join(d, "k.pub"))
	k, err := ParsePublicKey(pubText)
	if err != nil {
		t.Fatal(err)
	}
	msg := SignedMessage("fake-1", "POST", "/v1/install", "00ff", []byte(`{}`))
	os.WriteFile(filepath.Join(d, "m"), msg, 0o600)
	run("-S", "-s", filepath.Join(d, "k.sec"), "-m", filepath.Join(d, "m"), "-x", filepath.Join(d, "m.sig"))
	sigText, _ := os.ReadFile(filepath.Join(d, "m.sig"))
	s, err := ParseSignatureBlob(strings.Split(strings.TrimRight(string(sigText), "\n"), "\n")[1])
	if err != nil {
		t.Fatal(err)
	}
	if !k.Verify(msg, s) {
		t.Fatal("Go did not accept a signature made by signify-openbsd")
	}
	if k.Verify(append(msg, 'x'), s) {
		t.Fatal("Go accepted a changed message")
	}
}

func TestParseRejectsGarbage(t *testing.T) {
	for _, in := range []string{"", "hello", "untrusted comment: x\n!!!notbase64\n", "untrusted comment: x\nRWQ=\n"} {
		if _, err := ParsePublicKey([]byte(in)); err == nil {
			t.Errorf("accepted %q", in)
		}
	}
	if _, err := ParseSignatureBlob("RWQ="); err == nil {
		t.Error("accepted a short signature")
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

// A request signed for another machine id is refused, whoever holds the key: the signed text names the machine.
func TestRequestSignedForAnotherMachineIsRefused(t *testing.T) {
	k, priv := newKey(t, 1)
	r := &rec{}
	srv := httptest.NewServer(NewAgent([]PublicKey{k}, r)) // this machine is "fake-1"
	defer srv.Close()
	body := `{"Slot":"b","BaseURL":"http://10.0.0.1/bundle"}`
	n := challenge(t, srv)
	req, _ := http.NewRequest("POST", srv.URL+"/v1/install", strings.NewReader(body))
	req.Header.Set("Authorization", "HubOS-Sig nonce="+n+", sig="+sigBlob(k, priv, SignedMessage("ai-1", "POST", "/v1/install", n, []byte(body))))
	resp, _ := http.DefaultClient.Do(req)
	if resp.StatusCode != 401 {
		t.Errorf("signed for ai-1, sent to fake-1: %d, want 401", resp.StatusCode)
	}
	// the same request signed for this machine works
	n = challenge(t, srv)
	if code, out := signedDo(t, srv, k, priv, n, "POST", "/v1/install", body); code != 200 {
		t.Fatalf("signed for this machine: %d %s", code, out)
	}
	// a request that says (machine=ai-1) in its header is refused with a clear message, and the nonce is used up
	n = challenge(t, srv)
	req, _ = http.NewRequest("POST", srv.URL+"/v1/install", strings.NewReader(body))
	req.Header.Set("Authorization", "HubOS-Sig machine=ai-1, nonce="+n+", sig="+sigBlob(k, priv, SignedMessage("fake-1", "POST", "/v1/install", n, []byte(body))))
	resp, _ = http.DefaultClient.Do(req)
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 403 || !strings.Contains(string(b), "another machine") {
		t.Errorf("header for another machine: %d %s", resp.StatusCode, b)
	}
	if code, _ := signedDo(t, srv, k, priv, n, "POST", "/v1/install", body); code != 401 {
		t.Errorf("nonce of the refused request must be used up: %d", code)
	}
	if len(r.installs) != 1 {
		t.Fatalf("installs: %v", r.installs)
	}
}
