//go:build !js

package gui

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestStoreDiagramPNGWritesFile(t *testing.T) {
	data := []byte{0x89, 'P', 'N', 'G', 1, 2, 3}
	path, err := storeDiagramPNG(data, 42, "test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(path)

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, data) {
		t.Errorf("file holds %v, want %v", got, data)
	}
}

func TestRemoveDiagramPNGDeletesFile(t *testing.T) {
	f, err := os.CreateTemp("", "diagram_test_*.png")
	if err != nil {
		t.Fatal(err)
	}
	path := f.Name()
	_ = f.Close()

	removeDiagramPNG(path)

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("file should be deleted: %v", err)
	}
}

func TestRemoveDiagramPNGNonexistent(t *testing.T) {
	// Should not panic on missing file. Build the path from
	// the temp dir so the test stays portable.
	missing := filepath.Join(t.TempDir(), "missing.png")
	removeDiagramPNG(missing)
}
