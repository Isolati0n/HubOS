// Package main is the source mirror tool described in docs/proposals/source-mirror.md:
// a folder (on the NAS) that holds every build input, each with a .meta.toml
// and a sha256 that was given by the caller, never taken from the download.
//
// Commands: fetch, verify, status, and path (the lookup a build uses; builds
// never fetch). Standard library plus the TOML library the repository already
// uses for the inventory (github.com/pelletier/go-toml/v2); no new dependency.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"syscall"
	"time"

	toml "github.com/pelletier/go-toml/v2"
)

// Meta is the content of FILE.meta.toml.
type Meta struct {
	Name           string `toml:"name"`
	Version        string `toml:"version"`
	UpstreamURL    string `toml:"upstream_url"`
	SHA256         string `toml:"sha256"`
	Signature      string `toml:"signature,omitempty"`        // file name of the upstream signature, stored beside the file
	SignatureKey   string `toml:"signature_key,omitempty"`    // key id; the keyring is <root>/keys/<key>.gpg
	SignatureState string `toml:"signature_state,omitempty"`  // "ok", "none upstream" or "unchecked" (no way to check here)
	FetchedAt      string `toml:"fetched_at"`
}

// Entry states, as printed by status and written to the status file.
const (
	StateOK        = "ok"
	StateCorrupt   = "corrupt"
	StateMissing   = "missing"
	StateUnchecked = "never verified"
)

// Entry is one input as status sees it.
type Entry struct {
	Name       string `json:"name"`
	Version    string `json:"version"`
	File       string `json:"file"`
	State      string `json:"state"`
	Detail     string `json:"detail,omitempty"`
	FetchedAt  string `json:"fetched_at,omitempty"`
	VerifiedAt string `json:"verified_at,omitempty"`
	SigState   string `json:"signature_state,omitempty"`
}

// Mirror is a mirror folder.
type Mirror struct {
	Root string
	HTTP *http.Client
	Now  func() time.Time
}

// New returns a mirror rooted at root.
func New(root string) *Mirror {
	return &Mirror{Root: root, HTTP: &http.Client{Timeout: 10 * time.Minute}, Now: time.Now}
}

var nameRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._+-]*$`)

func okComponent(s string) bool { return nameRe.MatchString(s) && !strings.Contains(s, "..") }

var shaRe = regexp.MustCompile(`^[0-9a-f]{64}$`)

// dir is the folder of one input: inputs/NAME/VERSION.
func (m *Mirror) dir(name, version string) string {
	return filepath.Join(m.Root, "inputs", name, version)
}

// atomicWrite writes data to path through a temporary file in the same folder, then renames it.
func atomicWrite(path string, data []byte) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// lock takes an exclusive lock on a file beside the input, so two fetches of the same file do not race.
func lock(path string) (func(), error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		f.Close()
		return nil, err
	}
	return func() { syscall.Flock(int(f.Fd()), syscall.LOCK_UN); f.Close() }, nil
}

// FetchOpts are the optional parts of fetch.
type FetchOpts struct {
	File         string // stored name; default: the last part of the URL
	SignatureURL string
	SignatureKey string
}

// ErrCorrupt is returned when an entry is marked corrupt: it is never downloaded again.
var ErrCorrupt = errors.New("entry is marked corrupt; not downloading again")

// Fetch downloads url into the mirror under name/version. sha256 is the expected hash, given by the caller.
// It returns "present" when the file was already there and correct (nothing is downloaded).
func (m *Mirror) Fetch(name, version, url, sha string, o FetchOpts) (string, error) {
	if !okComponent(name) || !okComponent(version) {
		return "", fmt.Errorf("name %q or version %q is not a plain name", name, version)
	}
	if !shaRe.MatchString(sha) {
		return "", fmt.Errorf("sha256 must be 64 lower-case hex characters")
	}
	file := o.File
	if file == "" {
		file = filepath.Base(strings.SplitN(url, "?", 2)[0])
	}
	if !okComponent(file) {
		return "", fmt.Errorf("file name %q is not a plain name", file)
	}
	d := m.dir(name, version)
	if err := os.MkdirAll(d, 0o755); err != nil {
		return "", err
	}
	unlock, err := lock(filepath.Join(d, "."+file+".lock"))
	if err != nil {
		return "", err
	}
	defer unlock()

	dst := filepath.Join(d, file)
	if _, err := os.Stat(dst + ".corrupt"); err == nil {
		return "", fmt.Errorf("%s %s %s: %w", name, version, file, ErrCorrupt)
	}
	if got, err := hashFile(dst); err == nil {
		if got == sha {
			if _, err := os.Stat(dst + ".meta.toml"); err == nil {
				return "present", nil
			}
		} else {
			// A file with another hash is already stored: never overwrite it silently.
			if err := os.WriteFile(dst+".corrupt", []byte(fmt.Sprintf("expected %s got %s at %s\n", sha, got, m.Now().UTC().Format(time.RFC3339))), 0o644); err != nil {
				return "", err
			}
			return "", fmt.Errorf("%s %s %s: stored file has sha256 %s, expected %s: marked corrupt", name, version, file, got, sha)
		}
	}

	tmp, err := os.CreateTemp(d, ".part-*")
	if err != nil {
		return "", err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	got, err := m.download(url, tmp)
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return "", fmt.Errorf("download %s: %w", url, err)
	}
	if got != sha {
		return "", fmt.Errorf("%s %s %s: downloaded sha256 %s, expected %s: nothing placed", name, version, file, got, sha)
	}

	meta := Meta{Name: name, Version: version, UpstreamURL: url, SHA256: sha, FetchedAt: m.Now().UTC().Format(time.RFC3339)}
	sigTmp := ""
	if o.SignatureURL != "" {
		st, err := os.CreateTemp(d, ".sig-*")
		if err != nil {
			return "", err
		}
		sigTmp = st.Name()
		defer os.Remove(sigTmp)
		_, derr := m.download(o.SignatureURL, st)
		st.Close()
		if derr != nil {
			return "", fmt.Errorf("download signature %s: %w", o.SignatureURL, derr)
		}
		state, serr := m.checkSignature(sigTmp, tmpName, o.SignatureKey)
		if serr != nil {
			return "", fmt.Errorf("%s %s %s: signature check failed: %w; nothing placed", name, version, file, serr)
		}
		meta.Signature = file + ".sig"
		meta.SignatureKey = o.SignatureKey
		meta.SignatureState = state
	} else {
		meta.SignatureState = "none upstream"
	}

	if sigTmp != "" {
		if err := os.Rename(sigTmp, filepath.Join(d, meta.Signature)); err != nil {
			return "", err
		}
	}
	if err := os.Chmod(tmpName, 0o644); err != nil {
		return "", err
	}
	if err := os.Rename(tmpName, dst); err != nil {
		return "", err
	}
	b, err := toml.Marshal(meta)
	if err != nil {
		return "", err
	}
	if err := atomicWrite(dst+".meta.toml", b); err != nil {
		return "", err
	}
	return "fetched", nil
}

func (m *Mirror) download(url string, w io.Writer) (string, error) {
	resp, err := m.HTTP.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("HTTP %s", resp.Status)
	}
	h := sha256.New()
	if _, err := io.Copy(io.MultiWriter(w, h), resp.Body); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// checkSignature checks a detached upstream signature with gpgv and the keyring <root>/keys/<key>.gpg.
// A signature that was checked and is wrong is an error (the entry is refused). If it cannot be checked
// here (no gpgv, no keyring, no key id) the state is "unchecked": that is UNKNOWN, recorded, not hidden.
func (m *Mirror) checkSignature(sig, file, key string) (string, error) {
	gpgv, err := exec.LookPath("gpgv")
	if err != nil || key == "" {
		return "unchecked", nil
	}
	ring := filepath.Join(m.Root, "keys", key+".gpg")
	if _, err := os.Stat(ring); err != nil {
		return "unchecked", nil
	}
	out, err := exec.Command(gpgv, "--keyring", ring, sig, file).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("gpgv: %v: %s", err, strings.TrimSpace(string(out)))
	}
	return "ok", nil
}

// readMeta reads FILE.meta.toml for a stored file.
func readMeta(file string) (*Meta, error) {
	b, err := os.ReadFile(file + ".meta.toml")
	if err != nil {
		return nil, err
	}
	var mt Meta
	if err := toml.Unmarshal(b, &mt); err != nil {
		return nil, err
	}
	if mt.Name == "" || mt.Version == "" || !shaRe.MatchString(mt.SHA256) {
		return nil, errors.New("meta file is incomplete")
	}
	return &mt, nil
}

// files lists the stored input files (the paths that have a .meta.toml), sorted.
func (m *Mirror) files() ([]string, error) {
	var out []string
	err := filepath.WalkDir(filepath.Join(m.Root, "inputs"), func(p string, d os.DirEntry, err error) error {
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return nil
			}
			return err
		}
		if !d.IsDir() && strings.HasSuffix(p, ".meta.toml") {
			out = append(out, strings.TrimSuffix(p, ".meta.toml"))
		}
		return nil
	})
	sort.Strings(out)
	return out, err
}

// Verify re-hashes the stored files (all, or only the named inputs), marks mismatches corrupt and writes the status file.
// The returned bool is true when every entry is ok.
func (m *Mirror) Verify(names ...string) ([]Entry, bool, error) {
	files, err := m.files()
	if err != nil {
		return nil, false, err
	}
	want := map[string]bool{}
	for _, n := range names {
		want[n] = true
	}
	var es []Entry
	allOK := true
	for _, f := range files {
		mt, merr := readMeta(f)
		if merr != nil {
			es = append(es, Entry{File: f, State: StateMissing, Detail: "meta file unreadable: " + merr.Error()})
			allOK = false
			continue
		}
		if len(want) > 0 && !want[mt.Name] {
			continue
		}
		e := Entry{Name: mt.Name, Version: mt.Version, File: f, FetchedAt: mt.FetchedAt, SigState: mt.SignatureState}
		got, herr := hashFile(f)
		switch {
		case errors.Is(herr, os.ErrNotExist):
			e.State, e.Detail = StateMissing, "file is missing"
		case herr != nil:
			e.State, e.Detail = StateMissing, herr.Error()
		case got != mt.SHA256:
			e.State, e.Detail = StateCorrupt, fmt.Sprintf("sha256 %s, expected %s", got, mt.SHA256)
			os.WriteFile(f+".corrupt", []byte(fmt.Sprintf("expected %s got %s at %s\n", mt.SHA256, got, m.Now().UTC().Format(time.RFC3339))), 0o644)
		default:
			e.State = StateOK
			e.VerifiedAt = m.Now().UTC().Format(time.RFC3339)
			atomicWrite(f+".verified", []byte(e.VerifiedAt+"\n"))
		}
		if e.State != StateOK {
			allOK = false
		}
		es = append(es, e)
	}
	if err := m.writeStatus(es, allOK); err != nil {
		return es, allOK, err
	}
	return es, allOK, nil
}

// Status reads the state of every entry without re-hashing: corrupt marker, missing file, never verified, ok.
func (m *Mirror) Status() ([]Entry, error) {
	files, err := m.files()
	if err != nil {
		return nil, err
	}
	var es []Entry
	for _, f := range files {
		mt, merr := readMeta(f)
		if merr != nil {
			es = append(es, Entry{File: f, State: StateMissing, Detail: "meta file unreadable: " + merr.Error()})
			continue
		}
		e := Entry{Name: mt.Name, Version: mt.Version, File: f, FetchedAt: mt.FetchedAt, SigState: mt.SignatureState}
		if b, err := os.ReadFile(f + ".verified"); err == nil {
			e.VerifiedAt = strings.TrimSpace(string(b))
		}
		switch _, serr := os.Stat(f); {
		case func() bool { _, e := os.Stat(f + ".corrupt"); return e == nil }():
			e.State = StateCorrupt
			if b, err := os.ReadFile(f + ".corrupt"); err == nil {
				e.Detail = strings.TrimSpace(string(b))
			}
		case serr != nil:
			e.State, e.Detail = StateMissing, "file is missing"
		case e.VerifiedAt == "":
			e.State = StateUnchecked
		default:
			e.State = StateOK
		}
		es = append(es, e)
	}
	return es, nil
}

type statusFile struct {
	WrittenAt string  `json:"written_at"`
	AllOK     bool    `json:"all_ok"`
	Entries   []Entry `json:"entries"`
}

// writeStatus writes <root>/status.json (the owner's decision: until a toast and a workshop view exist, a status file and a non-zero exit).
func (m *Mirror) writeStatus(es []Entry, allOK bool) error {
	if err := os.MkdirAll(m.Root, 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(statusFile{WrittenAt: m.Now().UTC().Format(time.RFC3339), AllOK: allOK, Entries: es}, "", "  ")
	if err != nil {
		return err
	}
	return atomicWrite(filepath.Join(m.Root, "status.json"), append(b, '\n'))
}

// Path is the lookup a build uses. It never downloads: a missing or corrupt input is an error that names the input.
// With file == "" the input must have exactly one stored file.
func (m *Mirror) Path(name, version, file string) (string, error) {
	if !okComponent(name) || !okComponent(version) {
		return "", fmt.Errorf("input %s %s: not a plain name", name, version)
	}
	d := m.dir(name, version)
	if file == "" {
		var found []string
		ms, _ := filepath.Glob(filepath.Join(d, "*.meta.toml"))
		for _, x := range ms {
			found = append(found, strings.TrimSuffix(x, ".meta.toml"))
		}
		switch len(found) {
		case 0:
			return "", fmt.Errorf("missing input %s %s: not in the mirror (builds never fetch)", name, version)
		case 1:
			file = filepath.Base(found[0])
		default:
			return "", fmt.Errorf("input %s %s has %d files; name one", name, version, len(found))
		}
	}
	p := filepath.Join(d, file)
	if _, err := os.Stat(p + ".corrupt"); err == nil {
		return "", fmt.Errorf("corrupt input %s %s %s: marked corrupt (builds never re-download)", name, version, file)
	}
	mt, err := readMeta(p)
	if err != nil {
		return "", fmt.Errorf("missing input %s %s %s: not in the mirror (builds never fetch)", name, version, file)
	}
	if _, err := os.Stat(p); err != nil {
		return "", fmt.Errorf("missing input %s %s %s: file is gone", name, version, file)
	}
	_ = mt
	return p, nil
}
