package tempfont

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteCreatesUniqueFile(t *testing.T) {
	path1, err := Write("go-gui-test", []byte("one"))
	if err != nil {
		t.Fatalf("Write(path1): %v", err)
	}
	defer func() { _ = os.Remove(path1) }()

	path2, err := Write("go-gui-test", []byte("two"))
	if err != nil {
		t.Fatalf("Write(path2): %v", err)
	}
	defer func() { _ = os.Remove(path2) }()

	if path1 == path2 {
		t.Fatal("expected unique temp file names")
	}

	got1, err := os.ReadFile(path1)
	if err != nil {
		t.Fatalf("ReadFile(path1): %v", err)
	}
	if string(got1) != "one" {
		t.Fatalf("path1 contents = %q", got1)
	}

	got2, err := os.ReadFile(path2)
	if err != nil {
		t.Fatalf("ReadFile(path2): %v", err)
	}
	if string(got2) != "two" {
		t.Fatalf("path2 contents = %q", got2)
	}
}

func TestWriteFixedReusesPath(t *testing.T) {
	t.Setenv("TMPDIR", t.TempDir())
	path1, err := WriteFixed("go-gui-fixed.ttf", []byte("one"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Remove(path1) }()
	path2, err := WriteFixed("go-gui-fixed.ttf", []byte("two"))
	if err != nil {
		t.Fatal(err)
	}
	if path1 != path2 {
		t.Fatalf("paths differ: %q and %q", path1, path2)
	}
	got, err := os.ReadFile(path2)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "two" {
		t.Fatalf("contents = %q, want %q", got, "two")
	}
	// The unique file each write goes through is renamed away, not
	// left beside the fixed one.
	left, err := filepath.Glob(filepath.Join(filepath.Dir(path2), ".go-gui-fixed.ttf-*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(left) != 0 {
		t.Fatalf("temporary files left behind: %q", left)
	}
}
