package main

import (
	"bytes"
	"flag"
	"os"
	"testing"
)

// updateMirrors rewrites each showcase mirror from its canonical
// file. `//go:embed` cannot reach outside the package directory, so
// a guide kept in the repo's docs/ folder must be copied into
// examples/showcase/docs/ before the showcase can embed it.
var updateMirrors = flag.Bool("update", false, "rewrite showcase doc mirrors from docs/")

// docMirrors pairs a canonical guide with its embedded copy. The
// copy is byte-identical: docs/ stays the file people edit and link
// to, and the copy exists only for the embed.
var docMirrors = []struct {
	canonical string
	mirror    string
}{
	{"../../docs/dx-cheat-sheet.md", "docs/dx_cheat_sheet.md"},
}

// TestDocMirrors fails when a mirror and its canonical file differ.
// An edit to docs/ alone would otherwise ship a stale page in the
// showcase, and nothing at runtime would show it.
func TestDocMirrors(t *testing.T) {
	for _, m := range docMirrors {
		want, err := os.ReadFile(m.canonical)
		if err != nil {
			t.Fatalf("read %s: %v", m.canonical, err)
		}
		if *updateMirrors {
			if err = os.WriteFile(m.mirror, want, 0o644); err != nil {
				t.Fatalf("write %s: %v", m.mirror, err)
			}
			continue
		}
		got, err := os.ReadFile(m.mirror)
		if err != nil {
			t.Errorf("read %s: %v; run: go test ./examples/showcase/ "+
				"-run TestDocMirrors -update", m.mirror, err)
			continue
		}
		if !bytes.Equal(got, want) {
			t.Errorf("%s differs from %s; edit the canonical file, then run: "+
				"go test ./examples/showcase/ -run TestDocMirrors -update",
				m.mirror, m.canonical)
		}
	}
}
