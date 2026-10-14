package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const reloadBase = `format = 1
[[machine]]
id = "hub"
name = "Hub"
role = "hub"
address = "192.0.2.10"
open = ["none"]
home = { x = 0, y = 0 }
`

func reloadFiles(t *testing.T, cur, next string) (string, string) {
	t.Helper()
	d := t.TempDir()
	c, n := filepath.Join(d, "current.toml"), filepath.Join(d, "new.toml")
	if err := os.WriteFile(c, []byte(cur), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(n, []byte(next), 0o644); err != nil {
		t.Fatal(err)
	}
	return c, n
}

// 6. a reload dry run with an added node reports it, exits 0 and changes nothing.
func TestReloadDryRunAddedNode(t *testing.T) {
	added := reloadBase + `[[machine]]
id = "nas-1"
name = "NAS"
role = "nas"
address = "192.0.2.20"
open = ["ssh"]
home = { x = 1, y = 0 }
`
	cur, next := reloadFiles(t, reloadBase, added)
	var out, errb bytes.Buffer
	code := dispatch([]string{"reload", "--dry-run", "--inventory", cur, next}, &out, &errb)
	if code != exitOK || !strings.Contains(out.String(), "added: nas-1") {
		t.Fatalf("exit %d, out %q, err %q", code, out.String(), errb.String())
	}
	if b, _ := os.ReadFile(cur); string(b) != reloadBase {
		t.Fatal("the current inventory file was changed")
	}
}

// 7. a reload dry run with a malformed file: non-zero exit, a problem line, and it says the current inventory would be kept.
func TestReloadDryRunMalformed(t *testing.T) {
	cur, next := reloadFiles(t, reloadBase, "format = 1\n[[machine]\nid = \n")
	var out, errb bytes.Buffer
	code := dispatch([]string{"reload", "--dry-run", "--inventory", cur, next}, &out, &errb)
	if code != exitBadInventory || !strings.Contains(errb.String(), "would be kept") || out.Len() != 0 {
		t.Fatalf("exit %d, out %q, err %q", code, out.String(), errb.String())
	}
}

// Extra: without --dry-run the command refuses, because there is no live reload.
func TestReloadWithoutDryRunRefuses(t *testing.T) {
	cur, next := reloadFiles(t, reloadBase, reloadBase)
	var out, errb bytes.Buffer
	code := dispatch([]string{"reload", "--inventory", cur, next}, &out, &errb)
	if code != exitFailure || !strings.Contains(errb.String(), "not built") {
		t.Fatalf("exit %d, err %q", code, errb.String())
	}
}
