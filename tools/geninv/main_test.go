package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"hubos/internal/inventory"
)

func TestGeneratedInventoriesAreValid(t *testing.T) {
	for _, n := range []int{1, 7, 100, 5000} {
		dir := t.TempDir()
		inv, nodes := filepath.Join(dir, "i.toml"), filepath.Join(dir, "n.txt")
		if err := generate(n, 20, 21000, inv, nodes); err != nil {
			t.Fatal(err)
		}
		got, problems, err := inventory.Load(inv)
		if err != nil || len(problems) > 0 {
			t.Fatalf("n=%d: %v %q", n, err, problems)
		}
		if len(got.Machines) != n+1 {
			t.Errorf("n=%d: %d machines", n, len(got.Machines))
		}
		b, _ := os.ReadFile(nodes)
		lines := strings.Count(string(b), "\n")
		if n >= 20 && (lines >= n || lines < n*9/10) {
			t.Errorf("n=%d: %d up-nodes", n, lines)
		}
	}
}
