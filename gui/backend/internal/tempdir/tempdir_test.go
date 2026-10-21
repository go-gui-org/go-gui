package tempdir

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestAdoptSetsTMPDIRWhenGoSeesNone(t *testing.T) {
	t.Setenv("TMPDIR", "")
	if !Adopt("/data/user/0/app/cache") {
		t.Fatal("Adopt did not set TMPDIR")
	}
	if got := os.Getenv("TMPDIR"); got != "/data/user/0/app/cache" {
		t.Fatalf("TMPDIR = %q", got)
	}
}

func TestAdoptKeepsExistingTMPDIR(t *testing.T) {
	t.Setenv("TMPDIR", "/host/chose/this")
	if Adopt("/data/user/0/app/cache") {
		t.Fatal("Adopt replaced a TMPDIR Go already saw")
	}
	if got := os.Getenv("TMPDIR"); got != "/host/chose/this" {
		t.Fatalf("TMPDIR = %q", got)
	}
}

func TestAdoptIgnoresEmptyValue(t *testing.T) {
	t.Setenv("TMPDIR", "")
	if Adopt("") || os.Getenv("TMPDIR") != "" {
		t.Fatal("Adopt set TMPDIR from an empty value")
	}
}

// The explicit cache directory becomes TMPDIR, is made when missing,
// and temp files land in it (os.TempDir reads TMPDIR on Unix only).
func TestSetUsesCacheDir(t *testing.T) {
	data := t.TempDir()
	t.Setenv("TMPDIR", "")
	want := filepath.Join(data, "host-cache")
	if err := Set(want); err != nil {
		t.Fatal(err)
	}
	if got := os.Getenv("TMPDIR"); got != want {
		t.Fatalf("TMPDIR = %q, want %q", got, want)
	}
	if runtime.GOOS != "windows" {
		f, err := os.CreateTemp("", "probe-*")
		if err != nil {
			t.Fatal(err)
		}
		_ = f.Close()
		if filepath.Dir(f.Name()) != want {
			t.Fatalf("CreateTemp wrote to %q, want %q", filepath.Dir(f.Name()), want)
		}
	}
}

// A cache directory Android already made is used as it is.
func TestSetKeepsExistingCacheDir(t *testing.T) {
	data := t.TempDir()
	cache := filepath.Join(data, "host-cache")
	keep := filepath.Join(cache, "host-data.txt")
	if err := os.Mkdir(cache, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keep, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TMPDIR", "")
	if err := Set(cache); err != nil {
		t.Fatal(err)
	}
	if got := os.Getenv("TMPDIR"); got != cache {
		t.Fatalf("TMPDIR = %q, want %q", got, cache)
	}
	if _, err := os.Stat(keep); err != nil {
		t.Fatal(err)
	}
}

// Where TMPDIR is already set, it is kept and no cache directory is
// made.
func TestSetKeepsExistingTMPDIR(t *testing.T) {
	data := t.TempDir()
	t.Setenv("TMPDIR", "/data/user/0/app/cache")
	if err := Set(filepath.Join(data, "host-cache")); err != nil {
		t.Fatal(err)
	}
	if got := os.Getenv("TMPDIR"); got != "/data/user/0/app/cache" {
		t.Fatalf("TMPDIR = %q", got)
	}
}

func TestSetIgnoresEmptyValue(t *testing.T) {
	t.Setenv("TMPDIR", "")
	if err := Set(""); err != nil || os.Getenv("TMPDIR") != "" {
		t.Fatalf("Set(\"\") = %v, TMPDIR = %q", err, os.Getenv("TMPDIR"))
	}
}
