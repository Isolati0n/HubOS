// Package checkpush tests tools/check-push.sh with fake files in throwaway
// repositories (outside the working tree). Every "secret" here is invented.
package checkpush

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func script(t *testing.T) string {
	t.Helper()
	p, err := filepath.Abs("../check-push.sh")
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.invalid",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.invalid")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// newRepo makes a repo with one base commit, marks it as origin/main, and
// returns its directory.
func newRepo(t *testing.T) string {
	dir := t.TempDir()
	git(t, dir, "init", "-q")
	if err := os.WriteFile(filepath.Join(dir, "README"), []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, dir, "add", ".")
	git(t, dir, "commit", "-q", "-m", "base")
	git(t, dir, "update-ref", "refs/remotes/origin/main", "HEAD")
	return dir
}

func commitFile(t *testing.T, dir, name string, data []byte) {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, data, 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, dir, "add", "-f", name)
	git(t, dir, "commit", "-q", "-m", "add "+name)
}

func run(t *testing.T, dir string, extraEnv ...string) (string, int) {
	t.Helper()
	cmd := exec.Command("sh", script(t))
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), extraEnv...)
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	err := cmd.Run()
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatal(err)
	}
	return out.String(), code
}

func TestCleanDiffPasses(t *testing.T) {
	dir := newRepo(t)
	commitFile(t, dir, "notes.md", []byte("KEY=$HOME\nTOKEN=\nAPI_KEY=<your key here>\nplain words\n"))
	if out, code := run(t, dir); code != 0 {
		t.Fatalf("expected ok, got %d:\n%s", code, out)
	}
}

func TestCoreFileFails(t *testing.T) {
	dir := newRepo(t)
	commitFile(t, dir, "tools/x/core", []byte("fake core\n"))
	out, code := run(t, dir)
	if code != 1 || !strings.Contains(out, "core file") || !strings.Contains(out, "tools/x/core") {
		t.Fatalf("want core-file failure, got %d:\n%s", code, out)
	}
}

func TestCoreDotPidFails(t *testing.T) {
	dir := newRepo(t)
	commitFile(t, dir, "core.12345", []byte("x"))
	if out, code := run(t, dir); code != 1 || !strings.Contains(out, "core file") {
		t.Fatalf("got %d:\n%s", code, out)
	}
}

func TestCoreAddedThenDeletedStillFails(t *testing.T) {
	dir := newRepo(t)
	commitFile(t, dir, "core", []byte("fake core\n"))
	git(t, dir, "rm", "-q", "core")
	git(t, dir, "commit", "-q", "-m", "remove core")
	if out, code := run(t, dir); code != 1 || !strings.Contains(out, "core file") {
		t.Fatalf("a core added in an earlier commit must still fail, got %d:\n%s", code, out)
	}
}

func TestBigFileFailsUnlessAllowed(t *testing.T) {
	dir := newRepo(t)
	commitFile(t, dir, "data/big.bin", bytes.Repeat([]byte("a"), 1<<20+10))
	if out, code := run(t, dir); code != 1 || !strings.Contains(out, "over 1 MiB") {
		t.Fatalf("got %d:\n%s", code, out)
	}
	allow := filepath.Join(t.TempDir(), "allow")
	if err := os.WriteFile(allow, []byte("# fixture\ndata/big.bin\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, code := run(t, dir, "CHECK_PUSH_ALLOW="+allow); code != 0 {
		t.Fatalf("allowlisted file should pass, got %d:\n%s", code, out)
	}
}

func TestCredentialLineFailsAndValueNotPrinted(t *testing.T) {
	for _, name := range []string{"GITHUB_TOKEN", "GH_TOKEN", "AWS_SECRET_ACCESS_KEY", "AWS_ACCESS_KEY_ID", "ANTHROPIC_API_KEY", "DB_PASSWORD", "MY_SERVICE_SECRET", "SOME_TOKEN", "OTHER_KEY"} {
		dir := newRepo(t)
		val := "fakevalue" + "Xq93kd7"
		commitFile(t, dir, "conf.sh", []byte("export "+name+"="+val+"\n"))
		out, code := run(t, dir)
		if code != 1 || !strings.Contains(out, "credential-looking variable") || !strings.Contains(out, "conf.sh") {
			t.Fatalf("%s: got %d:\n%s", name, code, out)
		}
		if strings.Contains(out, val) {
			t.Fatalf("%s: the value was printed:\n%s", name, out)
		}
	}
}

func TestShellExpansionIsNotAValue(t *testing.T) {
	dir := newRepo(t)
	name := "XDG_ACTIVATION_" + "TOKEN" // built at run time so this file passes its own scan
	commitFile(t, dir, "open.sh", []byte("echo token_set=${"+name+":+yes} other=${"+name+":-none}\n"))
	if out, code := run(t, dir); code != 0 {
		t.Fatalf("a shell expansion is not a secret, got %d:\n%s", code, out)
	}
}

func TestPrivateKeyHeaderFails(t *testing.T) {
	dir := newRepo(t)
	commitFile(t, dir, "k.pem", []byte("-----BEGIN OPENSSH "+"PRIVATE KEY-----\nAAAA\n"))
	if out, code := run(t, dir); code != 1 || !strings.Contains(out, "private key") {
		t.Fatalf("got %d:\n%s", code, out)
	}
}

func TestEnvironmentValueByValue(t *testing.T) {
	dir := newRepo(t)
	val := "zzUnique-secret-value-8841"
	// The value appears with no variable name next to it, as in an env dump.
	commitFile(t, dir, "log.txt", []byte("something "+val+" something\n"))
	name := "FAKE_API_" + "TOKEN" // built at run time so this file itself passes the scan
	out, code := run(t, dir, name+"="+val)
	if code != 1 || !strings.Contains(out, name) {
		t.Fatalf("got %d:\n%s", code, out)
	}
	if strings.Contains(out, val) {
		t.Fatalf("the value was printed:\n%s", out)
	}
	if out, code := run(t, dir); code != 0 {
		t.Fatalf("without the variable it must pass, got %d:\n%s", code, out)
	}
}

func TestMissingBaseIsAnError(t *testing.T) {
	dir := newRepo(t)
	if out, code := run(t, dir, "CHECK_PUSH_BASE=origin/nope"); code != 2 {
		t.Fatalf("got %d:\n%s", code, out)
	}
}
