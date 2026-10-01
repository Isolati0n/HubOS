package hub

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// record is the small file of windows hubd started. It lives in the runtime
// folder (memory-backed, gone at reboot), holds no secrets, and is only a
// help: every entry is checked against driftwm before it is trusted.
type record struct {
	Driftwm string   `json:"driftwm"` // identity of the driftwm instance it belongs to
	Windows []winRec `json:"windows"`
}

func readRecord(path string) (record, bool) {
	var r record
	b, err := os.ReadFile(path)
	if err != nil || json.Unmarshal(b, &r) != nil {
		return record{}, false
	}
	return r, true
}

// writeRecord replaces the file in one step so a crash never leaves half a file.
func writeRecord(path string, r record) {
	b, err := json.Marshal(r)
	if err != nil {
		return
	}
	tmp := filepath.Join(filepath.Dir(path), ".record.tmp")
	if os.WriteFile(tmp, b, 0o600) == nil {
		os.Rename(tmp, path)
	}
}
