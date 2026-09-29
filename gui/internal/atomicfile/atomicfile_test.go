package atomicfile

import (
	"errors"
	"math"
	"os"
	"path/filepath"
	"testing"
)

func TestWriteFileCreatesAndOverwrites(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	if err := WriteFile(path, []byte("one"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := WriteFile(path, []byte("two"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "two" {
		t.Fatalf("got %q, want %q", got, "two")
	}
}

func TestWriteFileLeavesNoStagingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	if err := WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "settings.json" {
		t.Fatalf("dir holds %v, want only settings.json", entries)
	}
}

// TestWriteFileRenameErrorRemovesStagingFile makes the rename fail after
// staging succeeded and checks the staging file is cleaned up. It does
// not check that the target keeps its old contents: on a failed rename
// that is the OS's rename guarantee, which a test cannot break without
// a fault seam.
func TestWriteFileRenameErrorRemovesStagingFile(t *testing.T) {
	dir := t.TempDir()
	// A non-empty directory at the target makes the rename fail on every
	// OS, while staging in dir still succeeds.
	blocked := filepath.Join(dir, "blocked")
	if err := os.Mkdir(blocked, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(blocked, "child"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := WriteFile(blocked, []byte("new"), 0o644); err == nil {
		t.Fatal("WriteFile over a non-empty directory: want error")
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "blocked" {
		t.Fatalf("staging file left behind: %v", entries)
	}
}

func TestWriteFileMissingDirIsError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "absent", "settings.json")
	if err := WriteFile(path, []byte("x"), 0o644); err == nil {
		t.Fatal("want error when the directory does not exist")
	}
}

func TestReadFileMissingIsNil(t *testing.T) {
	data, err := ReadFile(filepath.Join(t.TempDir(), "absent"), 16)
	if err != nil || data != nil {
		t.Fatalf("got (%q, %v), want (nil, nil)", data, err)
	}
}

func TestReadFileWithinLimit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "f")
	if err := os.WriteFile(path, []byte("12345678"), 0o644); err != nil {
		t.Fatal(err)
	}
	data, err := ReadFile(path, 8)
	if err != nil || string(data) != "12345678" {
		t.Fatalf("got (%q, %v)", data, err)
	}
}

func TestReadFileOverLimit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "f")
	if err := os.WriteFile(path, []byte("123456789"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := ReadFile(path, 8)
	if !errors.Is(err, ErrTooLarge) {
		t.Fatalf("got %v, want ErrTooLarge", err)
	}
}

func TestReadFileMaxLimitReadsAll(t *testing.T) {
	path := filepath.Join(t.TempDir(), "f")
	if err := os.WriteFile(path, []byte("abc"), 0o644); err != nil {
		t.Fatal(err)
	}
	// limit+1 would overflow to a negative LimitReader N and read
	// nothing.
	data, err := ReadFile(path, math.MaxInt64)
	if err != nil || string(data) != "abc" {
		t.Fatalf("got (%q, %v), want (abc, nil)", data, err)
	}
}

func TestReadFileNegativeLimitIsError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "f")
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := ReadFile(path, -1)
	if err == nil || errors.Is(err, ErrTooLarge) {
		t.Fatalf("got %v, want a bad-limit error", err)
	}
}
