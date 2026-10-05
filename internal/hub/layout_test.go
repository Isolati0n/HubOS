package hub

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func newStore(t *testing.T) *layoutStore {
	t.Helper()
	return &layoutStore{dir: filepath.Join(t.TempDir(), "layouts"), now: time.Now}
}

func sampleLayout(name string) Layout {
	return Layout{Format: 1, Name: name, Saved: "2026-10-05T15:40:00Z",
		View:    &LayoutView{Zoom: 0.8, Cam: [2]float64{120.5, -80}},
		Windows: []LayoutWindow{{"ai-1", -2000, 0, 1280, 720}, {"nas-1", 2000, 0, 1280, 720}}}
}

func TestLayoutNameRule(t *testing.T) {
	good := []string{"work", "home-2", "a", "0", "x-y-z", strings.Repeat("a", MaxLayoutName)}
	bad := []string{"", "Work", "-lead", "has space", "under_score", "dot.dot", "ä", "a/b", "..", strings.Repeat("a", MaxLayoutName+1), "_active"}
	for _, n := range good {
		if err := ValidLayoutName(n); err != nil {
			t.Errorf("%q refused: %v", n, err)
		}
	}
	for _, n := range bad {
		if ValidLayoutName(n) == nil {
			t.Errorf("%q accepted", n)
		}
	}
}

func TestLayoutSaveAndLoadRoundTrip(t *testing.T) {
	s := newStore(t)
	l := sampleLayout("work")
	if err := s.save(l, false); err != nil {
		t.Fatal(err)
	}
	got, err := s.load("work")
	if err != nil {
		t.Fatal(err)
	}
	if got.View == nil || got.View.Zoom != 0.8 || got.View.Cam != [2]float64{120.5, -80} || len(got.Windows) != 2 || got.Windows[1] != l.Windows[1] {
		t.Errorf("read back %+v", got)
	}
	b, _ := os.ReadFile(s.path("work"))
	if !strings.Contains(string(b), `"sha256"`) {
		t.Errorf("no checksum in the file:\n%s", b)
	}
	if entries, _ := os.ReadDir(s.dir); len(entries) != 1 {
		t.Errorf("the folder holds more than the layout: %v", entries)
	}
}

func TestLayoutSaveRefusesAnExistingNameWithoutReplace(t *testing.T) {
	s := newStore(t)
	if err := s.save(sampleLayout("work"), false); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(s.path("work"))
	l2 := sampleLayout("work")
	l2.Windows = l2.Windows[:1]
	if err := s.save(l2, false); err == nil || !strings.Contains(err.Error(), "--replace") {
		t.Errorf("overwrite without --replace: %v", err)
	}
	if after, _ := os.ReadFile(s.path("work")); string(after) != string(before) {
		t.Error("the refused save changed the file")
	}
	if err := s.save(l2, true); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.load("work"); len(got.Windows) != 1 {
		t.Errorf("replace did not replace: %+v", got)
	}
}

func TestLayoutLimitIsOneHundred(t *testing.T) {
	s := newStore(t)
	for i := 0; i < MaxLayouts; i++ {
		l := sampleLayout("l" + strings.Repeat("x", i%5) + string(rune('a'+i/26)) + string(rune('a'+i%26)))
		if err := s.save(l, true); err != nil {
			t.Fatalf("layout %d: %v", i, err)
		}
	}
	if n := len(s.names()); n != MaxLayouts {
		t.Fatalf("%d layouts", n)
	}
	if err := s.save(sampleLayout("one-too-many"), false); err == nil || !strings.Contains(err.Error(), "100") {
		t.Errorf("101st layout: %v", err)
	}
	// replacing an existing one is still fine at the limit
	if err := s.save(sampleLayout(s.names()[0]), true); err != nil {
		t.Errorf("replace at the limit: %v", err)
	}
}

func TestLayoutFileTooBigIsRefused(t *testing.T) {
	s := newStore(t)
	l := sampleLayout("big")
	for i := 0; i < 2000; i++ {
		l.Windows = append(l.Windows, LayoutWindow{Machine: "m" + string(rune('a'+i%26)) + string(rune('a'+i/26%26)) + string(rune('a'+i/676)), W: 10, H: 10})
	}
	err := s.save(l, false)
	if err == nil || !strings.Contains(err.Error(), "limit") {
		t.Fatalf("got %v", err)
	}
	if _, statErr := os.Stat(s.path("big")); statErr == nil {
		t.Error("a file was written")
	}
}

// A damaged file is detected, moved aside, ignored with a warning, and never
// stops anything. Every way a file can be wrong is a row.
func TestLayoutCorruptFileIsQuarantinedAndIgnored(t *testing.T) {
	good := func(t *testing.T, s *layoutStore) []byte {
		if err := s.save(sampleLayout("work"), false); err != nil {
			t.Fatal(err)
		}
		b, _ := os.ReadFile(s.path("work"))
		return b
	}
	cases := []struct {
		name   string
		damage func(b []byte) []byte
	}{
		{"half-written (cut in the middle)", func(b []byte) []byte { return b[:len(b)/2] }},
		{"empty", func(b []byte) []byte { return nil }},
		{"garbage", func(b []byte) []byte { return []byte("\x00\x01 not json at all") }},
		{"one digit changed (checksum)", func(b []byte) []byte {
			return []byte(strings.Replace(string(b), `"x": -2000`, `"x": -2001`, 1))
		}},
		{"checksum removed", func(b []byte) []byte {
			i := strings.Index(string(b), `"sha256"`)
			return []byte(string(b[:i]) + "\"sha256\": \"\"\n}\n")
		}},
		{"unknown field", func(b []byte) []byte {
			return []byte(strings.Replace(string(b), `"format": 1,`, `"format": 1, "extra": 1,`, 1))
		}},
		{"two documents", func(b []byte) []byte { return append(append([]byte(nil), b...), b...) }},
		{"holds another name", func(b []byte) []byte {
			// a valid file of the layout "other" under the name work
			o := newStore(t)
			o.save(sampleLayout("other"), false)
			ob, _ := os.ReadFile(o.path("other"))
			return ob
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := newStore(t)
			b := good(t, s)
			if err := os.WriteFile(s.path("work"), c.damage(b), 0o644); err != nil {
				t.Fatal(err)
			}
			_, err := s.load("work")
			ce, ok := err.(*CorruptError)
			if !ok {
				t.Fatalf("got %T %v", err, err)
			}
			if ce.MovedTo == "" || !strings.Contains(ce.MovedTo, ".corrupt.") {
				t.Errorf("not moved aside: %+v", ce)
			}
			if _, err := os.Stat(s.path("work")); err == nil {
				t.Error("the damaged file is still in place")
			}
			if _, err := os.Stat(ce.MovedTo); err != nil {
				t.Errorf("the quarantined file is missing: %v", err)
			}
			// ignored: not listed, and a second load says there is no such layout
			if infos, _ := s.list(); len(infos) != 0 {
				t.Errorf("listed: %v", infos)
			}
			if _, err := s.load("work"); err != ErrNoLayout {
				t.Errorf("second load: %v", err)
			}
			// and the name can be used again
			if err := s.save(sampleLayout("work"), false); err != nil {
				t.Errorf("save after quarantine: %v", err)
			}
		})
	}
}

func TestLayoutOversizeFileIsQuarantined(t *testing.T) {
	s := newStore(t)
	os.MkdirAll(s.dir, 0o755)
	os.WriteFile(s.path("big"), []byte(strings.Repeat(" ", MaxLayoutBytes+1)), 0o644)
	if _, err := s.load("big"); err == nil {
		t.Fatal("accepted")
	} else if _, ok := err.(*CorruptError); !ok {
		t.Errorf("%T %v", err, err)
	}
}

// A file written by a newer hubd (after a rollback of the image) is not damaged: it is left alone.
func TestLayoutNewerFormatIsLeftInPlace(t *testing.T) {
	s := newStore(t)
	os.MkdirAll(s.dir, 0o755)
	content := `{"format": 2, "name": "future", "somethingnew": true, "sha256": "x"}`
	os.WriteFile(s.path("future"), []byte(content), 0o644)
	_, err := s.load("future")
	if _, ok := err.(*errNewerFormat); !ok {
		t.Fatalf("got %T %v", err, err)
	}
	if b, _ := os.ReadFile(s.path("future")); string(b) != content {
		t.Error("the file was changed")
	}
	if m, _ := filepath.Glob(filepath.Join(s.dir, "*.corrupt.*")); len(m) != 0 {
		t.Errorf("quarantined: %v", m)
	}
}

func TestLayoutQuarantineKeepsOnlyTheNewestFive(t *testing.T) {
	s := newStore(t)
	os.MkdirAll(s.dir, 0o755)
	tick := time.Now()
	s.now = func() time.Time { tick = tick.Add(time.Second); return tick }
	for i := 0; i < 9; i++ {
		os.WriteFile(s.path("work"), []byte("broken"), 0o644)
		s.load("work")
	}
	m, _ := filepath.Glob(filepath.Join(s.dir, "*.corrupt.*"))
	if len(m) != maxQuarantine {
		t.Errorf("%d quarantined files kept: %v", len(m), m)
	}
}

func TestAtomicWriteFailureChangesNothingAndLeavesNoTemp(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "x.layout.json")
	if err := atomicWrite(path, []byte("one")); err != nil {
		t.Fatal(err)
	}
	// the target is a folder: the rename fails, the old content stays, no temp file is left
	bad := filepath.Join(dir, "adir")
	os.Mkdir(bad, 0o755)
	os.WriteFile(filepath.Join(bad, "keep"), nil, 0o644)
	if err := atomicWrite(bad, []byte("two")); err == nil {
		t.Error("writing over a folder worked")
	}
	if b, _ := os.ReadFile(path); string(b) != "one" {
		t.Errorf("content %q", b)
	}
	ents, _ := os.ReadDir(dir)
	for _, e := range ents {
		if strings.Contains(e.Name(), ".tmp") {
			t.Errorf("temporary file left: %s", e.Name())
		}
	}
	// a folder that does not exist
	if err := atomicWrite(filepath.Join(dir, "nope", "f"), []byte("x")); err == nil {
		t.Error("write into a missing folder worked")
	}
}

func TestLayoutTempFilesFromACrashAreRemovedAtStart(t *testing.T) {
	r := newRig(t, machineDoc("a", "A", "ai", "moonlight", 1, 1, 1, ""))
	dir := filepath.Join(t.TempDir(), "layouts")
	os.MkdirAll(dir, 0o755)
	os.WriteFile(filepath.Join(dir, ".work.layout.json.tmp123"), []byte("half"), 0o644)
	os.WriteFile(filepath.Join(dir, "keep.layout.json.corrupt.1"), []byte("old"), 0o644)
	set := r.set
	set.LayoutDir = dir
	New(r.inv, r.vt, r.f, r.l.launch, set, "")
	ents, _ := os.ReadDir(dir)
	for _, e := range ents {
		if strings.Contains(e.Name(), ".tmp") {
			t.Errorf("temp file kept: %s", e.Name())
		}
	}
	if len(ents) != 1 || ents[0].Name() != "keep.layout.json.corrupt.1" {
		t.Errorf("folder: %v", ents)
	}
}

func TestActiveFileRoundTripAndCorruption(t *testing.T) {
	s := newStore(t)
	if n, err := s.readActive(); n != "" || err != nil {
		t.Errorf("no file: %q %v", n, err)
	}
	if err := s.writeActive("work"); err != nil {
		t.Fatal(err)
	}
	if n, err := s.readActive(); n != "work" || err != nil {
		t.Errorf("read back: %q %v", n, err)
	}
	if err := s.writeActive(""); err != nil {
		t.Fatal(err)
	}
	if n, err := s.readActive(); n != "" || err != nil {
		t.Errorf("cleared: %q %v", n, err)
	}
	for _, damage := range []string{"", "{", `{"format":1,"layout":"work","sha256":"00"}`, `{"format":1,"layout":"../etc","sha256":"x"}`} {
		os.WriteFile(s.activePath(), []byte(damage), 0o644)
		n, err := s.readActive()
		if _, ok := err.(*CorruptError); !ok || n != "" {
			t.Errorf("%q: got %q %v", damage, n, err)
		}
		if _, statErr := os.Stat(s.activePath()); statErr == nil {
			t.Errorf("%q: damaged active file still in place", damage)
		}
	}
}

func TestOverlapWarning(t *testing.T) {
	apart := []LayoutWindow{{"a", 0, 0, 100, 100}, {"b", 200, 0, 100, 100}}
	if o := overlapPairs(apart); len(o) != 0 {
		t.Errorf("apart: %v", o)
	}
	touching := []LayoutWindow{{"a", 0, 0, 100, 100}, {"b", 100, 0, 100, 100}}
	if o := overlapPairs(touching); len(o) != 0 {
		t.Errorf("touching: %v", o)
	}
	slight := []LayoutWindow{{"a", 0, 0, 100, 100}, {"b", 80, 0, 100, 100}} // 20 percent
	if o := overlapPairs(slight); len(o) != 0 {
		t.Errorf("slight: %v", o)
	}
	heavy := []LayoutWindow{{"a", 0, 0, 100, 100}, {"b", 30, 0, 100, 100}, {"c", 1000, 0, 100, 100}}
	if o := overlapPairs(heavy); len(o) != 1 || !strings.HasPrefix(o[0], "a and b (70%") {
		t.Errorf("heavy: %v", o)
	}
	small := []LayoutWindow{{"big", 0, 0, 1000, 1000}, {"small", 0, 0, 100, 100}} // a small one inside a big one: all of the small one
	if o := overlapPairs(small); len(o) != 1 {
		t.Errorf("inside: %v", o)
	}
}
