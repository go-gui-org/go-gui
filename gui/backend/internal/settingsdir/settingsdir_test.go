package settingsdir

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestDirSettingsStoreNoDir(t *testing.T) {
	var s Store
	if _, err := s.Load("org.example.app"); !errors.Is(err, ErrNoDir) {
		t.Fatalf("load: got %v, want ErrNoDir", err)
	}
	if err := s.Save("org.example.app", []byte("{}")); !errors.Is(err, ErrNoDir) {
		t.Fatalf("save: got %v, want ErrNoDir", err)
	}
}

func TestDirSettingsStoreRoundTrip(t *testing.T) {
	dir := t.TempDir()
	var s Store
	s.SetDir(dir)
	data, err := s.Load("org.example.app")
	if err != nil || data != nil {
		t.Fatalf("empty load = (%q, %v), want (nil, nil)", data, err)
	}
	if err = s.Save("org.example.app", []byte(`{"A":1}`)); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(filepath.Join(dir, "org.example.app", "settings.json")); err != nil {
		t.Fatalf("file not under the files dir: %v", err)
	}
	data, err = s.Load("org.example.app")
	if err != nil || string(data) != `{"A":1}` {
		t.Fatalf("load = (%q, %v)", data, err)
	}
}

func TestDirSettingsStoreOversizeSaveIsError(t *testing.T) {
	var s Store
	s.SetDir(t.TempDir())
	big := make([]byte, maxSettingsBytes+1)
	if err := s.Save("org.example.app", big); err == nil {
		t.Fatal("want an error: a blob Load would refuse must not be saved")
	}
}

func TestDirSettingsStoreOversizeFileIsError(t *testing.T) {
	dir := t.TempDir()
	var s Store
	s.SetDir(dir)
	appDir := filepath.Join(dir, "org.example.app")
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		t.Fatal(err)
	}
	big := make([]byte, maxSettingsBytes+1)
	if err := os.WriteFile(filepath.Join(appDir, "settings.json"), big, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Load("org.example.app"); err == nil {
		t.Fatal("want an error for an oversize file")
	}
}
