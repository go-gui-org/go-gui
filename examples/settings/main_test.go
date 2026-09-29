package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

// NewTestWindow gives the window the in-memory settings store, so these
// tests never touch the real settings file.

// OnInit loads the defaults on a first run, counts the launch and saves.
func TestFirstLaunchKeepsDefaults(t *testing.T) {
	w := gui.NewTestWindow(t, windowCfg())
	app := gui.State[App](w)
	if app.Err != "" {
		t.Fatalf("Err = %q", app.Err)
	}
	if app.Settings.Launches != 1 || !app.Settings.Dark {
		t.Fatalf("Settings = %+v, want 1 launch and the Dark default", app.Settings)
	}
	var saved Settings
	if err := gui.LoadSettings(w, &saved); err != nil {
		t.Fatal(err)
	}
	if saved != app.Settings {
		t.Fatalf("saved %+v, want %+v", saved, app.Settings)
	}
}

func TestSwitchSavesDark(t *testing.T) {
	w := gui.NewTestWindow(t, windowCfg())
	// A click hit-tests the last frame, so render one first.
	w.TestRender(nil)
	if err := w.TestClick("dark"); err != nil {
		t.Fatal(err)
	}
	var saved Settings
	if err := gui.LoadSettings(w, &saved); err != nil {
		t.Fatal(err)
	}
	if saved.Dark {
		t.Fatal("Dark still saved as true after the switch was clicked")
	}
}

// A -screenshot run has no backend, so the store is the real file. It
// must neither read nor write it: the image must not depend on the
// machine, and a screenshot must not change the user's settings.
func TestScreenshotCfgSkipsStore(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("AppData", filepath.Join(home, "AppData"))
	cfgDir, err := os.UserConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	cfg := screenshotCfg()
	w := gui.NewWindow(cfg)
	cfg.OnInit(w)
	if _, err = os.Stat(filepath.Join(cfgDir, info.ID)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("screenshot run touched the settings dir (stat err %v)", err)
	}
	if s := gui.State[App](w).Settings; s.Launches != 1 || !s.Dark {
		t.Fatalf("Settings = %+v, want the defaults and one launch", s)
	}
}
