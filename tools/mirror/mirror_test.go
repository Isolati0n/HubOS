package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

// server is a local HTTP server that serves fixed files and counts requests. No test talks to a real upstream site.
type server struct {
	*httptest.Server
	hits atomic.Int64
	data map[string][]byte
}

func newServer(t *testing.T, files map[string]string) *server {
	t.Helper()
	s := &server{data: map[string][]byte{}}
	for k, v := range files {
		s.data[k] = []byte(v)
	}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.hits.Add(1)
		b, ok := s.data[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Write(b)
	}))
	t.Cleanup(s.Close)
	return s
}

func sum(s string) string { h := sha256.Sum256([]byte(s)); return hex.EncodeToString(h[:]) }

func newMirror(t *testing.T) *Mirror { t.Helper(); return New(t.TempDir()) }

func stored(m *Mirror, name, version, file string) string { return filepath.Join(m.dir(name, version), file) }

// 1. fetch a file: placed, .meta.toml correct.
func TestFetchPlacesFileAndMeta(t *testing.T) {
	srv := newServer(t, map[string]string{"/a-1.0.tar": "alpha"})
	m := newMirror(t)
	res, err := m.Fetch("alpha", "1.0", srv.URL+"/a-1.0.tar", sum("alpha"), FetchOpts{})
	if err != nil || res != "fetched" {
		t.Fatalf("fetch: %q %v", res, err)
	}
	p := stored(m, "alpha", "1.0", "a-1.0.tar")
	if b, _ := os.ReadFile(p); string(b) != "alpha" {
		t.Fatalf("stored file content %q", b)
	}
	mt, err := readMeta(p)
	if err != nil {
		t.Fatal(err)
	}
	if mt.Name != "alpha" || mt.Version != "1.0" || mt.SHA256 != sum("alpha") || mt.UpstreamURL != srv.URL+"/a-1.0.tar" || mt.FetchedAt == "" || mt.SignatureState != "none upstream" {
		t.Fatalf("meta: %+v", mt)
	}
	// no temporary files are left behind
	ents, _ := os.ReadDir(m.dir("alpha", "1.0"))
	for _, e := range ents {
		if strings.HasPrefix(e.Name(), ".part-") || strings.HasPrefix(e.Name(), ".tmp-") || strings.HasPrefix(e.Name(), ".sig-") {
			t.Errorf("leftover %s", e.Name())
		}
	}
}

// 2. fetch it again: reports present, nothing is downloaded.
func TestFetchAgainIsPresent(t *testing.T) {
	srv := newServer(t, map[string]string{"/a.tar": "alpha"})
	m := newMirror(t)
	if _, err := m.Fetch("alpha", "1", srv.URL+"/a.tar", sum("alpha"), FetchOpts{}); err != nil {
		t.Fatal(err)
	}
	h := srv.hits.Load()
	res, err := m.Fetch("alpha", "1", srv.URL+"/a.tar", sum("alpha"), FetchOpts{})
	if err != nil || res != "present" {
		t.Fatalf("second fetch: %q %v", res, err)
	}
	if srv.hits.Load() != h {
		t.Fatalf("second fetch made a request")
	}
}

// 3. corrupt a stored byte, then verify: marked corrupt, status file says so, not ok.
func TestVerifyMarksCorrupt(t *testing.T) {
	srv := newServer(t, map[string]string{"/a.tar": "alpha"})
	m := newMirror(t)
	m.Fetch("alpha", "1", srv.URL+"/a.tar", sum("alpha"), FetchOpts{})
	p := stored(m, "alpha", "1", "a.tar")
	os.WriteFile(p, []byte("alpHa"), 0o644)
	es, ok, err := m.Verify()
	if err != nil || ok {
		t.Fatalf("verify: ok=%v err=%v", ok, err)
	}
	if len(es) != 1 || es[0].State != StateCorrupt {
		t.Fatalf("entries: %+v", es)
	}
	if _, err := os.Stat(p + ".corrupt"); err != nil {
		t.Fatalf("no corrupt marker")
	}
	var sf statusFile
	b, _ := os.ReadFile(filepath.Join(m.Root, "status.json"))
	if err := json.Unmarshal(b, &sf); err != nil || sf.AllOK || sf.Entries[0].State != StateCorrupt {
		t.Fatalf("status file: %s", b)
	}
	st, _ := m.Status()
	if st[0].State != StateCorrupt {
		t.Fatalf("status: %+v", st)
	}
	var out, errb bytes.Buffer
	if code := run([]string{"--root", m.Root, "verify"}, &out, &errb); code != 1 {
		t.Fatalf("verify exit code %d, want 1", code)
	}
}

// 4. corrupt, then a build: fails naming the input, and nothing is downloaded (the server sees no request).
func TestCorruptThenBuildFailsWithoutDownloading(t *testing.T) {
	srv := newServer(t, map[string]string{"/a.tar": "alpha"})
	m := newMirror(t)
	m.Fetch("alpha", "1", srv.URL+"/a.tar", sum("alpha"), FetchOpts{})
	os.WriteFile(stored(m, "alpha", "1", "a.tar"), []byte("xxxxx"), 0o644)
	m.Verify()
	h := srv.hits.Load()
	_, err := m.Path("alpha", "1", "")
	if err == nil || !strings.Contains(err.Error(), "alpha 1") || !strings.Contains(err.Error(), "corrupt") {
		t.Fatalf("path error: %v", err)
	}
	if _, err := m.Fetch("alpha", "1", srv.URL+"/a.tar", sum("alpha"), FetchOpts{}); err == nil || !strings.Contains(err.Error(), "corrupt") {
		t.Fatalf("fetch of a corrupt entry: %v", err)
	}
	if srv.hits.Load() != h {
		t.Fatalf("a request was made for a corrupt entry")
	}
}

// 5. two different files at once: both placed.
func TestTwoDifferentFilesAtOnce(t *testing.T) {
	srv := newServer(t, map[string]string{"/a.tar": "alpha", "/b.tar": "bravo"})
	m := newMirror(t)
	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i, f := range []struct{ n, c string }{{"a", "alpha"}, {"b", "bravo"}} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, errs[i] = m.Fetch(f.n, "1", srv.URL+"/"+f.n+".tar", sum(f.c), FetchOpts{})
		}()
	}
	wg.Wait()
	for i, e := range errs {
		if e != nil {
			t.Fatalf("fetch %d: %v", i, e)
		}
	}
	for n, c := range map[string]string{"a": "alpha", "b": "bravo"} {
		if b, _ := os.ReadFile(stored(m, n, "1", n+".tar")); string(b) != c {
			t.Errorf("%s content %q", n, b)
		}
	}
}

// 6. the same file twice at once: one file, no partial file, no error, one download.
func TestSameFileTwiceAtOnce(t *testing.T) {
	srv := newServer(t, map[string]string{"/a.tar": strings.Repeat("alpha", 1000)})
	m := newMirror(t)
	var wg sync.WaitGroup
	res := make([]string, 4)
	errs := make([]error, 4)
	for i := range res {
		wg.Add(1)
		go func() {
			defer wg.Done()
			res[i], errs[i] = m.Fetch("a", "1", srv.URL+"/a.tar", sum(strings.Repeat("alpha", 1000)), FetchOpts{})
		}()
	}
	wg.Wait()
	fetched := 0
	for i := range res {
		if errs[i] != nil {
			t.Fatalf("fetch %d: %v", i, errs[i])
		}
		if res[i] == "fetched" {
			fetched++
		}
	}
	if fetched != 1 || srv.hits.Load() != 1 {
		t.Fatalf("fetched=%d hits=%d, want 1 and 1", fetched, srv.hits.Load())
	}
	ents, _ := os.ReadDir(m.dir("a", "1"))
	for _, e := range ents {
		if strings.HasPrefix(e.Name(), ".part-") || strings.HasPrefix(e.Name(), ".tmp-") {
			t.Errorf("partial file %s", e.Name())
		}
	}
}

// 7. a clean verify: everything ok, exit code 0, status file all_ok.
func TestCleanVerify(t *testing.T) {
	srv := newServer(t, map[string]string{"/a.tar": "alpha", "/b.tar": "bravo"})
	m := newMirror(t)
	m.Fetch("a", "1", srv.URL+"/a.tar", sum("alpha"), FetchOpts{})
	m.Fetch("b", "2", srv.URL+"/b.tar", sum("bravo"), FetchOpts{})
	var out, errb bytes.Buffer
	if code := run([]string{"--root", m.Root, "verify"}, &out, &errb); code != 0 {
		t.Fatalf("exit %d: %s %s", code, out.String(), errb.String())
	}
	var sf statusFile
	b, _ := os.ReadFile(filepath.Join(m.Root, "status.json"))
	json.Unmarshal(b, &sf)
	if !sf.AllOK || len(sf.Entries) != 2 {
		t.Fatalf("status file: %s", b)
	}
	st, _ := m.Status()
	for _, e := range st {
		if e.State != StateOK {
			t.Errorf("status %+v", e)
		}
	}
}

// 8. a missing input at build time: fails and names it (and does not fetch).
func TestMissingInputAtBuildTime(t *testing.T) {
	m := newMirror(t)
	var out, errb bytes.Buffer
	code := run([]string{"--root", m.Root, "path", "ghost", "9.9"}, &out, &errb)
	if code != 1 || !strings.Contains(errb.String(), "ghost 9.9") || !strings.Contains(errb.String(), "never fetch") {
		t.Fatalf("exit %d, stderr %q", code, errb.String())
	}
}

// Extra (not one of the eight): a hash mismatch at fetch places nothing and leaves no temporary file.
func TestFetchHashMismatchPlacesNothing(t *testing.T) {
	srv := newServer(t, map[string]string{"/a.tar": "alpha"})
	m := newMirror(t)
	_, err := m.Fetch("a", "1", srv.URL+"/a.tar", sum("something else"), FetchOpts{})
	if err == nil || !strings.Contains(err.Error(), "expected") {
		t.Fatalf("error: %v", err)
	}
	ents, _ := os.ReadDir(m.dir("a", "1"))
	for _, e := range ents {
		if !strings.HasSuffix(e.Name(), ".lock") {
			t.Errorf("left behind: %s", e.Name())
		}
	}
	if _, err := m.Path("a", "1", ""); err == nil {
		t.Errorf("the input is present after a mismatch")
	}
}

// Extra: an upstream signature is checked with gpgv when gpgv and a keyring exist; a wrong signature refuses the entry.
// Skipped if gpg or gpgv is missing (then signature checking is UNKNOWN on that machine and the mirror records "unchecked").
func TestSignature(t *testing.T) {
	for _, tool := range []string{"gpg", "gpgv"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skip(tool + " not available")
		}
	}
	home := t.TempDir()
	os.Chmod(home, 0o700)
	gpg := func(args ...string) []byte {
		c := exec.Command("gpg", append([]string{"--homedir", home, "--batch", "--yes", "--pinentry-mode", "loopback", "--passphrase", ""}, args...)...)
		out, err := c.Output()
		if err != nil {
			t.Skipf("gpg %v failed here: %v", args, err)
		}
		return out
	}
	gpg("--quick-gen-key", "mirror-test@example.invalid", "default", "default", "never")
	m := newMirror(t)
	os.MkdirAll(filepath.Join(m.Root, "keys"), 0o755)
	ring := gpg("--export", "mirror-test@example.invalid")
	os.WriteFile(filepath.Join(m.Root, "keys", "testkey.gpg"), ring, 0o644)

	data := []byte("signed content")
	tmp := filepath.Join(t.TempDir(), "f")
	os.WriteFile(tmp, data, 0o644)
	gpg("--detach-sign", "--output", tmp+".sig", tmp)
	sig, _ := os.ReadFile(tmp + ".sig")

	srv := newServer(t, map[string]string{"/f.tar": string(data), "/f.tar.sig": string(sig), "/bad.tar": "other content", "/bad.tar.sig": string(sig)})
	if _, err := m.Fetch("f", "1", srv.URL+"/f.tar", sum(string(data)), FetchOpts{SignatureURL: srv.URL + "/f.tar.sig", SignatureKey: "testkey"}); err != nil {
		t.Fatalf("good signature refused: %v", err)
	}
	mt, _ := readMeta(stored(m, "f", "1", "f.tar"))
	if mt.SignatureState != "ok" {
		t.Fatalf("signature state %q", mt.SignatureState)
	}
	if _, err := m.Fetch("bad", "1", srv.URL+"/bad.tar", sum("other content"), FetchOpts{SignatureURL: srv.URL + "/bad.tar.sig", SignatureKey: "testkey"}); err == nil {
		t.Fatalf("wrong signature accepted")
	}
	if _, err := m.Path("bad", "1", ""); err == nil {
		t.Fatalf("entry with a wrong signature was placed")
	}
}
