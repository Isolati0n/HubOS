package hub

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// Saved window layouts (docs/proposals/driftwm-layouts.md, owner decisions in
// docs/hubd-slice2.md section 18).
//
// A layout is a named list of window places keyed by machine id, plus the
// view (zoom and camera). It lives in one small file under the config
// partition (default /config/hubos/layouts), written only when the owner
// asks. Every file carries a SHA-256 checksum of its own content. A file that
// does not read back correctly (half-written, damaged, edited by hand) is
// moved aside as <file>.corrupt.<time> and ignored with a warning; it never
// stops hubd.

const (
	layoutFormat   = 1
	MaxLayouts     = 100      // most layouts kept
	MaxLayoutBytes = 64 << 10 // largest layout file
	MaxLayoutName  = 64       // longest name (a guess; not decided by the owner)
	maxQuarantine  = 5        // most *.corrupt.* files kept
	layoutSuffix   = ".layout.json"
	activeFileName = "_active.json" // "_" cannot start a layout name
)

// layoutNameRe is the machine-id rule of the inventory: lower-case letters,
// digits and dashes, not starting with a dash.
var layoutNameRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

// ValidLayoutName says whether a name is allowed, with the reason if not.
func ValidLayoutName(name string) error {
	switch {
	case name == "":
		return errors.New("a layout needs a name")
	case len(name) > MaxLayoutName:
		return fmt.Errorf("the layout name %q is %d characters long; the limit is %d", name, len(name), MaxLayoutName)
	case !layoutNameRe.MatchString(name):
		return fmt.Errorf("the layout name %q is not allowed: use lower-case letters, digits and dashes, not starting with a dash (the same rule as a machine id)", name)
	}
	return nil
}

// LayoutWindow is one machine's window place: the centre and the size of the
// visible frame, Y up (the same numbers as `driftwm msg state` and the
// inventory's home).
type LayoutWindow struct {
	Machine string `json:"machine"`
	X       int    `json:"x"`
	Y       int    `json:"y"`
	W       int    `json:"w"`
	H       int    `json:"h"`
}

// LayoutView is the view: the canvas point at the centre of the screen (Y up,
// what `driftwm msg camera` takes) and the zoom.
type LayoutView struct {
	Zoom float64    `json:"zoom"`
	Cam  [2]float64 `json:"camera"`
}

// Layout is a saved layout.
type Layout struct {
	Format  int            `json:"format"`
	Name    string         `json:"name"`
	Saved   string         `json:"saved"` // for people only
	View    *LayoutView    `json:"view,omitempty"`
	Windows []LayoutWindow `json:"windows"`
}

// diskLayout is the file: the layout and the checksum of its compact JSON.
type diskLayout struct {
	Layout
	Sum string `json:"sha256"`
}

func sumOf(v any) string {
	b, _ := json.Marshal(v)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

// validate checks the values themselves (the names of the machines are
// checked against the inventory when the layout is used, not here).
func (l *Layout) validate() error {
	if l.Format != layoutFormat {
		return fmt.Errorf("format %d is not understood (this hubd reads format %d)", l.Format, layoutFormat)
	}
	if err := ValidLayoutName(l.Name); err != nil {
		return err
	}
	seen := map[string]bool{}
	for _, w := range l.Windows {
		if !layoutNameRe.MatchString(w.Machine) {
			return fmt.Errorf("window for %q: not a machine id", w.Machine)
		}
		if seen[w.Machine] {
			return fmt.Errorf("machine %s appears twice", w.Machine)
		}
		seen[w.Machine] = true
		if w.W <= 0 || w.H <= 0 || w.W > 1_000_000 || w.H > 1_000_000 || abs(w.X) > 100_000_000 || abs(w.Y) > 100_000_000 {
			return fmt.Errorf("window for %s: the numbers are out of range", w.Machine)
		}
	}
	if v := l.View; v != nil {
		ok := func(f float64) bool { return !math.IsNaN(f) && !math.IsInf(f, 0) && math.Abs(f) < 1e9 }
		if !(v.Zoom > 0 && v.Zoom <= 1) || !ok(v.Cam[0]) || !ok(v.Cam[1]) {
			return errors.New("the view (zoom and camera) is out of range")
		}
	}
	return nil
}

func abs(i int) int {
	if i < 0 {
		return -i
	}
	return i
}

// byMachine finds a machine's window in the layout.
func (l *Layout) byMachine(id string) (LayoutWindow, bool) {
	for _, w := range l.Windows {
		if w.Machine == id {
			return w, true
		}
	}
	return LayoutWindow{}, false
}

// ErrNoLayout means there is no layout of that name.
var ErrNoLayout = errors.New("no such layout")

// errNewerFormat is a file of a format this hubd does not read. It is not
// damaged, so it is neither moved nor deleted.
type errNewerFormat struct {
	name   string
	format int
}

func (e *errNewerFormat) Error() string {
	return fmt.Sprintf("layout %s is in format %d, which this hubd does not read; left in place and ignored", e.name, e.format)
}

// CorruptError says a file did not read back correctly; it has been moved aside.
type CorruptError struct {
	Name, Reason, MovedTo string
}

func (e *CorruptError) Error() string {
	if e.MovedTo == "" {
		return fmt.Sprintf("%s is damaged (%s) and could not be moved aside; ignored", e.Name, e.Reason)
	}
	return fmt.Sprintf("%s was damaged (%s); moved aside to %s and ignored", e.Name, e.Reason, filepath.Base(e.MovedTo))
}

// layoutStore reads and writes the layout files in one folder. The caller
// serialises calls.
type layoutStore struct {
	dir string
	now func() time.Time
}

func (s *layoutStore) path(name string) string { return filepath.Join(s.dir, name+layoutSuffix) }

// atomicWrite replaces path in one step that survives a power cut: the data
// goes to a temporary file in the same folder, which is flushed, renamed over
// the target, and then the folder is flushed. On any failure the target is
// left as it was and the temporary file is removed.
func atomicWrite(path string, data []byte) (err error) {
	dir := filepath.Dir(path)
	f, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer func() {
		if err != nil {
			f.Close()
			os.Remove(tmp)
		}
	}()
	if _, err = f.Write(data); err != nil {
		return err
	}
	if err = f.Chmod(0o644); err != nil {
		return err
	}
	if err = f.Sync(); err != nil {
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	if err = os.Rename(tmp, path); err != nil {
		return err
	}
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer d.Close()
	return d.Sync()
}

// cleanTemps removes the temporary files a crash left behind.
func (s *layoutStore) cleanTemps() {
	ents, _ := os.ReadDir(s.dir)
	for _, e := range ents {
		if strings.HasPrefix(e.Name(), ".") && strings.Contains(e.Name(), ".tmp") {
			os.Remove(filepath.Join(s.dir, e.Name()))
		}
	}
}

// quarantine moves a damaged file aside and trims old quarantined files.
func (s *layoutStore) quarantine(path, reason string) *CorruptError {
	ce := &CorruptError{Name: filepath.Base(path), Reason: reason}
	dst := fmt.Sprintf("%s.corrupt.%d", path, s.now().Unix())
	if err := os.Rename(path, dst); err == nil {
		ce.MovedTo = dst
		if d, err := os.Open(s.dir); err == nil {
			d.Sync()
			d.Close()
		}
	}
	// keep only the newest few
	old, _ := filepath.Glob(filepath.Join(s.dir, "*.corrupt.*"))
	if len(old) > maxQuarantine {
		sort.Slice(old, func(i, j int) bool {
			a, _ := os.Stat(old[i])
			b, _ := os.Stat(old[j])
			if a == nil || b == nil {
				return old[i] < old[j]
			}
			return a.ModTime().Before(b.ModTime())
		})
		for _, p := range old[:len(old)-maxQuarantine] {
			os.Remove(p)
		}
	}
	return ce
}

// readChecked reads one file, checks size, JSON, checksum and values. A
// damaged file is quarantined and a *CorruptError returned.
func (s *layoutStore) readChecked(path string, into func(b []byte) error) error {
	fi, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return ErrNoLayout
		}
		return err
	}
	if fi.Size() > MaxLayoutBytes {
		return s.quarantine(path, fmt.Sprintf("%d bytes is more than the limit of %d", fi.Size(), MaxLayoutBytes))
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return into(b)
}

// load reads and checks one layout.
func (s *layoutStore) load(name string) (Layout, error) {
	if err := ValidLayoutName(name); err != nil {
		return Layout{}, err
	}
	path := s.path(name)
	var out Layout
	err := s.readChecked(path, func(b []byte) error {
		if f, ok := s.newerFormat(b); ok {
			return &errNewerFormat{filepath.Base(path), f}
		}
		var d diskLayout
		dec := json.NewDecoder(bytes.NewReader(b))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&d); err != nil {
			return s.quarantine(path, "not readable: "+err.Error())
		}
		if dec.More() {
			return s.quarantine(path, "extra data after the layout")
		}
		if sumOf(d.Layout) != d.Sum {
			return s.quarantine(path, "the checksum does not match")
		}
		if d.Name != name {
			return s.quarantine(path, fmt.Sprintf("the file holds the layout %q, not %q", d.Name, name))
		}
		if err := d.Layout.validate(); err != nil {
			return s.quarantine(path, err.Error())
		}
		out = d.Layout
		return nil
	})
	return out, err
}

// newerFormat says whether the file is valid JSON that declares a format
// newer than this hubd reads (for example after a rollback of the hub image).
// Such a file is not damaged, so it must not be moved aside.
func (s *layoutStore) newerFormat(b []byte) (int, bool) {
	var head struct {
		Format int `json:"format"`
	}
	if json.Unmarshal(b, &head) != nil {
		return 0, false
	}
	return head.Format, head.Format > layoutFormat
}

// LayoutInfo is one line of the list.
type LayoutInfo struct {
	Name    string
	Windows int
	Saved   string
	HasView bool
}

// names lists the layout file names (without the suffix), sorted.
func (s *layoutStore) names() []string {
	ents, _ := os.ReadDir(s.dir)
	var out []string
	for _, e := range ents {
		if e.Type().IsRegular() && strings.HasSuffix(e.Name(), layoutSuffix) && !strings.HasPrefix(e.Name(), ".") {
			out = append(out, strings.TrimSuffix(e.Name(), layoutSuffix))
		}
	}
	sort.Strings(out)
	return out
}

// list reads every layout, checks it, and returns the good ones and a
// warning for each one that was damaged or unreadable.
func (s *layoutStore) list() (infos []LayoutInfo, warns []string) {
	for _, n := range s.names() {
		l, err := s.load(n)
		if err != nil {
			warns = append(warns, "layout "+n+": "+err.Error())
			continue
		}
		infos = append(infos, LayoutInfo{Name: n, Windows: len(l.Windows), Saved: l.Saved, HasView: l.View != nil})
	}
	return
}

// save writes a layout. Without replace an existing name is refused.
func (s *layoutStore) save(l Layout, replace bool) error {
	if err := l.validate(); err != nil {
		return err
	}
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return fmt.Errorf("cannot use the layout folder %s: %w", s.dir, err)
	}
	_, statErr := os.Stat(s.path(l.Name))
	exists := statErr == nil
	if exists && !replace {
		return fmt.Errorf("a layout named %s already exists; use --replace to overwrite it", l.Name)
	}
	if !exists && len(s.names()) >= MaxLayouts {
		return fmt.Errorf("there are already %d layouts, which is the most kept; delete one first", MaxLayouts)
	}
	b, err := json.MarshalIndent(diskLayout{Layout: l, Sum: sumOf(l)}, "", " ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	if len(b) > MaxLayoutBytes {
		return fmt.Errorf("the layout would be %d bytes, more than the limit of %d; nothing was saved", len(b), MaxLayoutBytes)
	}
	if err := atomicWrite(s.path(l.Name), b); err != nil {
		return fmt.Errorf("could not write the layout (nothing was changed): %w", err)
	}
	return nil
}

// delete removes a layout file.
func (s *layoutStore) delete(name string) error {
	if err := ValidLayoutName(name); err != nil {
		return err
	}
	err := os.Remove(s.path(name))
	if errors.Is(err, fs.ErrNotExist) {
		return ErrNoLayout
	}
	if err != nil {
		return err
	}
	if d, err := os.Open(s.dir); err == nil {
		d.Sync()
		d.Close()
	}
	return nil
}

// ---- the active layout ----

type activeDoc struct {
	Format int    `json:"format"`
	Layout string `json:"layout"` // "" = none
}
type activeDisk struct {
	activeDoc
	Sum string `json:"sha256"`
}

func (s *layoutStore) activePath() string { return filepath.Join(s.dir, activeFileName) }

// writeActive remembers which layout is active ("" = none).
func (s *layoutStore) writeActive(name string) error {
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return err
	}
	d := activeDoc{Format: layoutFormat, Layout: name}
	b, _ := json.MarshalIndent(activeDisk{activeDoc: d, Sum: sumOf(d)}, "", " ")
	return atomicWrite(s.activePath(), append(b, '\n'))
}

// readActive returns the remembered name ("" = none). A damaged file is moved
// aside and reported as an error together with the name "".
func (s *layoutStore) readActive() (string, error) {
	path := s.activePath()
	var name string
	err := s.readChecked(path, func(b []byte) error {
		if f, ok := s.newerFormat(b); ok {
			return &errNewerFormat{filepath.Base(path), f}
		}
		var d activeDisk
		dec := json.NewDecoder(bytes.NewReader(b))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&d); err != nil {
			return s.quarantine(path, "not readable: "+err.Error())
		}
		if dec.More() || sumOf(d.activeDoc) != d.Sum {
			return s.quarantine(path, "the checksum does not match")
		}
		if d.Format != layoutFormat {
			return s.quarantine(path, fmt.Sprintf("format %d", d.Format))
		}
		if d.Layout != "" {
			if err := ValidLayoutName(d.Layout); err != nil {
				return s.quarantine(path, err.Error())
			}
		}
		name = d.Layout
		return nil
	})
	if errors.Is(err, ErrNoLayout) {
		return "", nil // no file: none active
	}
	return name, err
}

// ---- overlap ----

// overlapPairs names the pairs of windows in a layout that overlap by more
// than half of the smaller one (the threshold proposed in
// docs/proposals/driftwm-layouts.md section 6). It is a warning, never an error.
func overlapPairs(ws []LayoutWindow) []string {
	var out []string
	for i := 0; i < len(ws); i++ {
		for j := i + 1; j < len(ws); j++ {
			a, b := ws[i], ws[j]
			ix := min(a.X+a.W/2, b.X+b.W/2) - max(a.X-a.W/2, b.X-b.W/2)
			iy := min(a.Y+a.H/2, b.Y+b.H/2) - max(a.Y-a.H/2, b.Y-b.H/2)
			if ix <= 0 || iy <= 0 {
				continue
			}
			small := min(a.W*a.H, b.W*b.H)
			if small > 0 && float64(ix)*float64(iy)/float64(small) > 0.5 {
				out = append(out, fmt.Sprintf("%s and %s (%d%% of the smaller one)", a.Machine, b.Machine, int(math.Round(100*float64(ix)*float64(iy)/float64(small)))))
			}
		}
	}
	return out
}
