package image

// Tests of the two small scripts that guard the hub compositor's settings file (owner decision 5, 2026-10-06), with a
// FAKE driftwm that rejects a file containing the word "bogus" (the real driftwm rejects an unknown field; that part is
// tested against the real binary in docs/proposals/driftwm-patches.md section 3.2). They need only /bin/sh.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// fakeDriftwm writes a program that answers --check-config like driftwm: "Config OK" (exit 0) for a good file,
// "Config OK, 1 warning(s)" for a file containing "warn", an error and exit 1 for a file containing "bogus".
func fakeDriftwm(t *testing.T, dir string) string {
	t.Helper()
	p := filepath.Join(dir, "driftwm")
	script := `#!/bin/sh
[ "$1" = "--check-config" ] || exit 2
f=$3
if grep -q bogus "$f"; then echo "driftwm: TOML parse error: unknown field bogus" >&2; exit 1; fi
if grep -q warn "$f"; then echo "Config OK, 1 warning(s)"; exit 0; fi
echo "Config OK"
`
	if err := os.WriteFile(p, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

const goodCfg = "[decorations]\nshadow = false\n\n[session]\nrestore_windows = false\n"

func TestBuildCheckRefusesBadHubConfigs(t *testing.T) {
	dir := t.TempDir()
	fake := fakeDriftwm(t, dir)
	root := filepath.Join(dir, "root")
	os.MkdirAll(filepath.Join(root, "etc/hubos"), 0o755)
	cfg := filepath.Join(root, "etc/hubos/driftwm.toml")
	run := func(content string) (string, error) {
		os.WriteFile(cfg, []byte(content), 0o644)
		cmd := exec.Command("bash", "check-driftwm-config.sh", root)
		cmd.Env = append(os.Environ(), "DRIFTWM_CHECK_CMD="+fake)
		out, err := cmd.CombinedOutput()
		return string(out), err
	}
	if out, err := run(goodCfg); err != nil {
		t.Fatalf("a good file was refused: %v\n%s", err, out)
	}
	for name, c := range map[string]string{
		"an unknown field":                   goodCfg + "[effects]\nbogus = 1\n",
		"a warning (strict)":                 goodCfg + "# warn\n",
		"no restore_windows line":            "[decorations]\nshadow = false\n",
		"restore_windows = true":             "[session]\nrestore_windows = true\n",
		"restore_windows in the wrong table": "[decorations]\nrestore_windows = false\n[session]\n",
	} {
		out, err := run(c)
		if err == nil {
			t.Errorf("%s: the check accepted it:\n%s", name, out)
		} else if !strings.Contains(out, "FAILED") {
			t.Errorf("%s: no clear message:\n%s", name, out)
		}
	}
	// the real file of the hub image has to pass the same rules (with the fake driftwm: the structure rules)
	real, err := os.ReadFile("../../image/machines/hub/rootfs/etc/hubos/driftwm.toml")
	if err != nil {
		t.Fatal(err)
	}
	if out, err := run(string(real)); err != nil {
		t.Errorf("the hub's real settings file is refused: %v\n%s", err, out)
	}
	// a missing file
	os.Remove(cfg)
	cmd := exec.Command("bash", "check-driftwm-config.sh", root)
	cmd.Env = append(os.Environ(), "DRIFTWM_CHECK_CMD="+fake)
	if out, err := cmd.CombinedOutput(); err == nil {
		t.Errorf("a missing file was accepted:\n%s", out)
	}
}

func TestRuntimeConfigFallsBackToTheLastGoodFileWithAnAlert(t *testing.T) {
	dir := t.TempDir()
	fake := fakeDriftwm(t, dir)
	img := filepath.Join(dir, "image.toml")
	good := filepath.Join(dir, "config/hubos/driftwm.last-good.toml")
	mark := filepath.Join(dir, "run/fallback")
	console := filepath.Join(dir, "console")
	run := func() (string, string) {
		cmd := exec.Command("sh", "../../image/machines/hub/rootfs/usr/lib/hubos/driftwm-config")
		cmd.Env = append(os.Environ(), "DW_IMAGE_CFG="+img, "DW_LAST_GOOD="+good, "DW_BIN="+fake, "DW_FALLBACK_MARK="+mark, "DW_CONSOLE="+console)
		var stderr strings.Builder
		cmd.Stderr = &stderr
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("driftwm-config: %v\n%s", err, stderr.String())
		}
		return strings.TrimSpace(string(out)), stderr.String()
	}
	exists := func(p string) bool { _, err := os.Stat(p); return err == nil }

	// 1. a bad file and no last good copy: defaults ("" = no --config), with the alert
	os.WriteFile(img, []byte(goodCfg+"bogus = 1\n"), 0o644)
	if out, _ := run(); out != "" {
		t.Errorf("bad file, no last good: printed %q, want nothing (built-in defaults)", out)
	}
	if b, _ := os.ReadFile(mark); !strings.Contains(string(b), "built-in defaults") {
		t.Errorf("alert marker: %q", b)
	}
	if b, _ := os.ReadFile(console); !strings.Contains(string(b), "HUB OS ALERT") {
		t.Errorf("alert on the console: %q", b)
	}
	os.Remove(console)
	// 2. a good file: used, copied as the last good one, the alert is gone
	os.WriteFile(img, []byte(goodCfg), 0o644)
	if out, _ := run(); out != img {
		t.Errorf("good file: printed %q, want %q", out, img)
	}
	if b, _ := os.ReadFile(good); string(b) != goodCfg {
		t.Errorf("last good copy: %q", b)
	}
	if exists(mark) {
		t.Error("the alert marker was not removed after a good start")
	}
	// 3. a bad file again: the last good copy is used, with the alert
	os.WriteFile(img, []byte(goodCfg+"bogus = 1\n"), 0o644)
	out, errText := run()
	if out != good {
		t.Errorf("bad file with a last good copy: printed %q, want %q", out, good)
	}
	if b, _ := os.ReadFile(mark); !strings.Contains(string(b), "last good settings file") || !strings.Contains(errText, "HUB OS ALERT") {
		t.Errorf("alert marker %q, stderr %q", b, errText)
	}
	if b, _ := os.ReadFile(good); string(b) != goodCfg {
		t.Errorf("the last good copy was overwritten by the bad file: %q", b)
	}
	// 4. a file with only a warning is still used (driftwm clamps the value)
	os.WriteFile(img, []byte(goodCfg+"# warn\n"), 0o644)
	if out, _ := run(); out != img {
		t.Errorf("file with a warning: printed %q, want %q", out, img)
	}
}
