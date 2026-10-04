package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestParseRelease(t *testing.T) {
	r, f := parseRelease("version=7\nflavor=hub\nkernel-version=6.12.1\n")
	if r != "7" || f != "hub" {
		t.Fatalf("got %q %q", r, f)
	}
	r, f = parseRelease("")
	if r != "unknown" || f != "unknown" {
		t.Fatalf("empty file: got %q %q", r, f)
	}
}

func TestParseSlot(t *testing.T) {
	for in, want := range map[string]string{
		"console=ttyS0 root=PARTLABEL=hubos-root-b hubos.slot=b ro\n": "b",
		"console=ttyS0 ro":          "unknown",
		"hubos.slot=c":              "unknown",
		"hubos.slot=a hubos.slot=b": "b",
		"xhubos.slot=a":             "unknown",
	} {
		if got := parseSlot(in); got != want {
			t.Errorf("parseSlot(%q) = %q, want %q", in, got, want)
		}
	}
}

func get(t *testing.T, url string) (int, string) {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b)
}

func TestStatusEndpoint(t *testing.T) {
	dir := t.TempDir()
	rel, cmd := filepath.Join(dir, "release"), filepath.Join(dir, "cmdline")
	os.WriteFile(rel, []byte("version=7\nflavor=hub\n"), 0o644)
	os.WriteFile(cmd, []byte("console=ttyS0 hubos.slot=a ro\n"), 0o644)
	ts := httptest.NewServer(&Server{ReleaseFile: rel, CmdlineFile: cmd})
	defer ts.Close()
	code, body := get(t, ts.URL+"/v1/status")
	if code != 200 || body != `{"release":"7","flavor":"hub","slot":"a"}` {
		t.Fatalf("got %d %q", code, body)
	}
	// the files are read at every request
	os.WriteFile(cmd, []byte("hubos.slot=b\n"), 0o644)
	if _, body = get(t, ts.URL+"/v1/status"); body != `{"release":"7","flavor":"hub","slot":"b"}` {
		t.Fatalf("after change: %q", body)
	}
	// files missing: the answer says unknown, it does not fail
	ts2 := httptest.NewServer(&Server{ReleaseFile: filepath.Join(dir, "none"), CmdlineFile: filepath.Join(dir, "none")})
	defer ts2.Close()
	if code, body = get(t, ts2.URL+"/v1/status"); code != 200 || body != `{"release":"unknown","flavor":"unknown","slot":"unknown"}` {
		t.Fatalf("missing files: %d %q", code, body)
	}
	if code, _ = get(t, ts.URL+"/v1/nothing"); code != 404 {
		t.Fatalf("unknown path: %d", code)
	}
	if code, _ = get(t, ts.URL+"/v1/crash"); code != 404 {
		t.Fatalf("crash endpoint must be off by default: %d", code)
	}
}

func TestHandlerPanicDoesNotStopTheServer(t *testing.T) {
	ts := httptest.NewUnstartedServer(&Server{ReleaseFile: "/nonexistent", CmdlineFile: "/nonexistent", TestCrash: true})
	ts.Config.ErrorLog = nil
	ts.Start()
	defer ts.Close()
	if _, err := http.Get(ts.URL + "/v1/crash"); err == nil {
		t.Fatal("the crashing request should get no answer")
	}
	if code, _ := get(t, ts.URL+"/v1/status"); code != 200 {
		t.Fatalf("server did not survive: %d", code)
	}
}
