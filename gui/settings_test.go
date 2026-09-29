package gui

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-gui-org/go-gui/gui/appinfo"
)

type testSettings struct {
	FontSize int
	Dark     bool
	Recent   []string
}

// settingsTestInfo is the manifest the settings tests run under.
var settingsTestInfo = appinfo.Info{ID: "org.go-gui.settings-test", Name: "Settings Test"}

// useTempConfigDir points os.UserConfigDir at a fresh directory on every
// OS and returns it, so a file-store test never touches the real one.
func useTempConfigDir(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("AppData", filepath.Join(home, "AppData"))
	dir, err := os.UserConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

// hookPlatform stores the settings blob in memory through the optional
// settingsPlatform hook, as the web and Android backends do.
type hookPlatform struct {
	noopNativePlatform
	blobs   map[string][]byte
	loadErr error
}

func (p *hookPlatform) SettingsLoad(appID string) ([]byte, error) {
	if p.loadErr != nil {
		return nil, p.loadErr
	}
	return p.blobs[appID], nil
}

func (p *hookPlatform) SettingsSave(appID string, data []byte) error {
	p.blobs[appID] = data
	return nil
}

func TestSettingsTestWindowRoundTripInMemory(t *testing.T) {
	cfgDir := useTempConfigDir(t)
	w := NewTestWindow(t, WindowCfg{AppInfo: settingsTestInfo})
	in := testSettings{FontSize: 16, Dark: true, Recent: []string{"a.txt"}}
	if err := SaveSettings(w, in); err != nil {
		t.Fatal(err)
	}
	var out testSettings
	if err := LoadSettings(w, &out); err != nil {
		t.Fatal(err)
	}
	if out.FontSize != 16 || !out.Dark || len(out.Recent) != 1 || out.Recent[0] != "a.txt" {
		t.Fatalf("round trip = %+v, want %+v", out, in)
	}
	// The test store must not touch disk.
	if _, err := os.Stat(filepath.Join(cfgDir, settingsTestInfo.ID)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("test window wrote to the config dir (stat err %v)", err)
	}
}

func TestSettingsLoadEmptyStoreKeepsDefaults(t *testing.T) {
	w := NewTestWindow(t, WindowCfg{AppInfo: settingsTestInfo})
	s := testSettings{FontSize: 14}
	if err := LoadSettings(w, &s); err != nil {
		t.Fatal(err)
	}
	if s.FontSize != 14 {
		t.Fatalf("FontSize = %d, want default 14 kept", s.FontSize)
	}
}

func TestSettingsLoadKeepsDefaultsForAbsentFields(t *testing.T) {
	p := &hookPlatform{blobs: map[string][]byte{
		settingsTestInfo.ID: []byte(`{"Dark": true}`),
	}}
	w := NewWindow(WindowCfg{AppInfo: settingsTestInfo})
	w.nativePlatform = p
	s := testSettings{FontSize: 14}
	if err := LoadSettings(w, &s); err != nil {
		t.Fatal(err)
	}
	if !s.Dark || s.FontSize != 14 {
		t.Fatalf("got %+v, want Dark from the file and FontSize default 14", s)
	}
}

func TestSettingsNoAppID(t *testing.T) {
	w := NewTestWindow(t, WindowCfg{})
	var s testSettings
	if err := LoadSettings(w, &s); !errors.Is(err, ErrNoAppID) {
		t.Fatalf("Load: got %v, want ErrNoAppID", err)
	}
	if err := SaveSettings(w, s); !errors.Is(err, ErrNoAppID) {
		t.Fatalf("Save: got %v, want ErrNoAppID", err)
	}
}

func TestSettingsRejectsPathLikeAppID(t *testing.T) {
	useTempConfigDir(t)
	for _, id := range []string{".", "..", "a/b", `a\b`, "a:b", "a b"} {
		w := NewWindow(WindowCfg{AppInfo: appinfo.Info{ID: id}})
		if err := SaveSettings(w, testSettings{}); err == nil {
			t.Errorf("SaveSettings with app ID %q: want error", id)
		}
	}
}

func TestSettingsPlatformHookIsUsed(t *testing.T) {
	cfgDir := useTempConfigDir(t)
	p := &hookPlatform{blobs: map[string][]byte{}}
	w := NewWindow(WindowCfg{AppInfo: settingsTestInfo})
	w.nativePlatform = p
	if err := SaveSettings(w, testSettings{FontSize: 20}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(p.blobs[settingsTestInfo.ID]), `"FontSize": 20`) {
		t.Fatalf("hook got %q, want the JSON blob", p.blobs[settingsTestInfo.ID])
	}
	if _, err := os.Stat(filepath.Join(cfgDir, settingsTestInfo.ID)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("hook platform also wrote the file (stat err %v)", err)
	}
}

func TestSettingsPlatformHookLoadError(t *testing.T) {
	p := &hookPlatform{loadErr: errors.New("quota")}
	w := NewWindow(WindowCfg{AppInfo: settingsTestInfo})
	w.nativePlatform = p
	s := testSettings{FontSize: 14}
	if err := LoadSettings(w, &s); err == nil {
		t.Fatal("want the hook's error")
	}
	if s.FontSize != 14 {
		t.Fatalf("dst changed on error: %+v", s)
	}
}

// TestSettingsFileStoreWithoutPlatform covers a real app that calls
// before backend.RunApp attaches the platform: it must reach disk, not
// a memory store that the process exit throws away.
func TestSettingsFileStoreWithoutPlatform(t *testing.T) {
	cfgDir := useTempConfigDir(t)
	w := NewWindow(WindowCfg{AppInfo: settingsTestInfo})
	if err := SaveSettings(w, testSettings{FontSize: 18}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(cfgDir, settingsTestInfo.ID, "settings.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("settings file: %v", err)
	}
	if !strings.Contains(string(data), `"FontSize": 18`) {
		t.Fatalf("file holds %q", data)
	}
	// A second window of the same app reads it back.
	w2 := NewWindow(WindowCfg{AppInfo: settingsTestInfo})
	var s testSettings
	if err := LoadSettings(w2, &s); err != nil {
		t.Fatal(err)
	}
	if s.FontSize != 18 {
		t.Fatalf("FontSize = %d, want 18", s.FontSize)
	}
}

func TestSettingsFileStoreWithHooklessPlatform(t *testing.T) {
	cfgDir := useTempConfigDir(t)
	w := NewWindow(WindowCfg{AppInfo: settingsTestInfo})
	w.nativePlatform = noopNativePlatform{}
	if err := SaveSettings(w, testSettings{Dark: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(cfgDir, settingsTestInfo.ID, "settings.json")); err != nil {
		t.Fatalf("want the file store: %v", err)
	}
}

func TestSettingsCorruptFileKeepsDst(t *testing.T) {
	cfgDir := useTempConfigDir(t)
	dir := filepath.Join(cfgDir, settingsTestInfo.ID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "settings.json"), []byte(`{"FontSize": 9, "Dark": `), 0o644); err != nil {
		t.Fatal(err)
	}
	w := NewWindow(WindowCfg{AppInfo: settingsTestInfo})
	s := testSettings{FontSize: 14}
	if err := LoadSettings(w, &s); err == nil {
		t.Fatal("want a decode error")
	}
	if s.FontSize != 14 {
		t.Fatalf("dst changed on error: %+v", s)
	}
}

func TestSettingsOversizeFileIsError(t *testing.T) {
	cfgDir := useTempConfigDir(t)
	dir := filepath.Join(cfgDir, settingsTestInfo.ID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	big := make([]byte, maxSettingsBytes+1)
	if err := os.WriteFile(filepath.Join(dir, "settings.json"), big, 0o644); err != nil {
		t.Fatal(err)
	}
	w := NewWindow(WindowCfg{AppInfo: settingsTestInfo})
	var s testSettings
	if err := LoadSettings(w, &s); err == nil {
		t.Fatal("want an error for an oversize file")
	}
}

func TestSettingsOversizeSaveIsError(t *testing.T) {
	w := NewTestWindow(t, WindowCfg{AppInfo: settingsTestInfo})
	big := testSettings{Recent: []string{strings.Repeat("x", maxSettingsBytes)}}
	if err := SaveSettings(w, big); err == nil {
		t.Fatal("want an error: a blob Load would refuse must not be saved")
	}
}

func TestSettingsNilDst(t *testing.T) {
	w := NewTestWindow(t, WindowCfg{AppInfo: settingsTestInfo})
	if err := LoadSettings[testSettings](w, nil); err == nil {
		t.Fatal("want an error for a nil dst")
	}
}
