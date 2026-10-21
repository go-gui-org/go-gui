package tempdir

import (
	"errors"
	"io/fs"
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

// dataDir is a fresh app data directory holding files/, with no TMPDIR
// set. It is made before TMPDIR is cleared: t.TempDir itself reads
// TMPDIR.
func dataDir(t *testing.T) string {
	t.Helper()
	data := t.TempDir()
	if err := os.Mkdir(filepath.Join(data, "files"), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TMPDIR", "")
	return data
}

// The cache directory beside files/ becomes TMPDIR, made when missing,
// and temp files land in it (os.TempDir reads TMPDIR on Unix only). A
// trailing separator does not move it.
func TestFromFilesDirUsesCacheDir(t *testing.T) {
	for _, suffix := range []string{"", string(filepath.Separator)} {
		data := dataDir(t)
		if err := FromFilesDir(filepath.Join(data, "files") + suffix); err != nil {
			t.Fatal(err)
		}
		want := filepath.Join(data, "cache")
		if got := os.Getenv("TMPDIR"); got != want {
			t.Fatalf("suffix %q: TMPDIR = %q, want %q", suffix, got, want)
		}
		if runtime.GOOS == "windows" {
			continue
		}
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
func TestFromFilesDirKeepsExistingCacheDir(t *testing.T) {
	data := dataDir(t)
	cache := filepath.Join(data, "cache")
	keep := filepath.Join(cache, "host-data.txt")
	if err := os.Mkdir(cache, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keep, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := FromFilesDir(filepath.Join(data, "files")); err != nil {
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
func TestFromFilesDirKeepsExistingTMPDIR(t *testing.T) {
	data := t.TempDir()
	t.Setenv("TMPDIR", "/data/user/0/app/cache")
	if err := FromFilesDir(filepath.Join(data, "files")); err != nil {
		t.Fatal(err)
	}
	if got := os.Getenv("TMPDIR"); got != "/data/user/0/app/cache" {
		t.Fatalf("TMPDIR = %q", got)
	}
	if _, err := os.Stat(filepath.Join(data, "cache")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("cache directory made although TMPDIR was set: %v", err)
	}
}

// A directory not named files is not Context.getFilesDir(): it is
// refused, and nothing is made beside it.
func TestFromFilesDirRefusesOtherDir(t *testing.T) {
	data := dataDir(t)
	if err := FromFilesDir(filepath.Join(data, "settings")); err == nil {
		t.Fatal("FromFilesDir accepted a directory not named files")
	}
	if got := os.Getenv("TMPDIR"); got != "" {
		t.Fatalf("TMPDIR = %q", got)
	}
	if _, err := os.Stat(filepath.Join(data, "cache")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("cache directory made beside a non-files directory: %v", err)
	}
}

func TestFromFilesDirIgnoresEmptyValue(t *testing.T) {
	t.Setenv("TMPDIR", "")
	if err := FromFilesDir(""); err != nil || os.Getenv("TMPDIR") != "" {
		t.Fatalf("FromFilesDir(\"\") = %v, TMPDIR = %q", err, os.Getenv("TMPDIR"))
	}
}
