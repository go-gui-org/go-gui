package gui

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/go-gui-org/go-gui/gui/internal/atomicfile"
)

// ErrNoAppID is returned by LoadSettings and SaveSettings when the
// window has no app ID. Set WindowCfg.AppInfo, usually from an embedded
// appinfo.toml, to give it one.
// exportaudit:keep — caller-facing errors.Is target (issue #848)
var ErrNoAppID = errors.New("gui: settings need an app ID: set WindowCfg.AppInfo")

// maxSettingsBytes caps one app's settings blob. Settings are small; the
// cap stops a huge or hostile file from making Load allocate without
// bound. Save refuses a blob over the cap, so Save never writes a file
// that Load would then refuse.
const maxSettingsBytes = 1 << 20

// settingsFileName is the file name inside the per-app config directory.
const settingsFileName = "settings.json"

// settingsPlatform is the optional NativePlatform hook for a platform
// with no usable config directory (web localStorage, Android filesDir).
// It is not part of NativePlatform, so no existing implementer breaks.
// A platform without it gets the file store. Load returns (nil, nil)
// when nothing is stored yet.
type settingsPlatform interface {
	SettingsLoad(appID string) ([]byte, error)
	SettingsSave(appID string, data []byte) error
}

// settingsState is the per-window part of the settings store.
type settingsState struct {
	mu sync.Mutex
	// memory is set by NewTestWindow. A nil NativePlatform is not the
	// test signal: a real app has one until backend.RunApp attaches the
	// platform, and a call in that time must reach disk, not a memory
	// store that is lost when the process exits (issue #848).
	memory bool
	// blob is the memory store's contents; nil means nothing saved.
	blob []byte
}

// LoadSettings reads the app's saved settings into dst. dst holds the
// defaults: JSON decoding only sets the fields the saved data has, so a
// field added in a later app version keeps its default when an older
// file is read. When nothing is saved yet, dst is left as it is and the
// error is nil.
//
// On an error (unreadable store, corrupt or oversize data), dst's
// top-level fields are left as they were, so the app can go on with its
// defaults. A map or slice that dst already holds can still be changed
// in place by the partial decode.
//
//	s := Settings{FontSize: 14} // defaults
//	if err := gui.LoadSettings(w, &s); err != nil {
//		log.Printf("settings: %v", err)
//	}
//
// The settings belong to the app, not the window: every window with the
// same WindowCfg.AppInfo.ID reads the same data. The store is:
//
//   - a window from NewTestWindow: memory, no disk;
//   - web and Android: the platform's own storage;
//   - every other platform: os.UserConfigDir()/<app ID>/settings.json.
//
// The file is app state, not a user config file: a running app
// overwrites hand edits on its next SaveSettings.
func LoadSettings[T any](w *Window, dst *T) error {
	if dst == nil {
		return errors.New("gui: LoadSettings: nil dst")
	}
	data, err := w.settingsLoad()
	if err != nil || data == nil {
		return err
	}
	// Decode into a copy and assign only on success, so a corrupt file
	// cannot leave dst half-filled.
	tmp := *dst
	if err = json.Unmarshal(data, &tmp); err != nil {
		return fmt.Errorf("gui: decode settings: %w", err)
	}
	*dst = tmp
	return nil
}

// SaveSettings stores v as the app's settings, JSON-encoded, replacing
// what was saved before. The desktop file is replaced atomically, so a
// crash during the save leaves the old settings or the new ones, never
// a partial file. See LoadSettings for where the data goes.
func SaveSettings[T any](w *Window, v T) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("gui: encode settings: %w", err)
	}
	if len(data) > maxSettingsBytes {
		return fmt.Errorf("gui: settings are %d bytes, over the %d-byte limit",
			len(data), maxSettingsBytes)
	}
	return w.settingsSave(data)
}

// settingsLoad returns the stored blob, or nil when nothing is stored.
func (w *Window) settingsLoad() ([]byte, error) {
	appID, err := w.settingsAppID()
	if err != nil {
		return nil, err
	}
	w.settings.mu.Lock()
	defer w.settings.mu.Unlock()
	if w.settings.memory {
		return w.settings.blob, nil
	}
	var data []byte
	if hook := w.settingsHook(); hook != nil {
		if data, err = hook.SettingsLoad(appID); err != nil {
			return nil, fmt.Errorf("gui: load settings: %w", err)
		}
		if len(data) > maxSettingsBytes {
			return nil, fmt.Errorf("gui: stored settings are %d bytes, over the %d-byte limit",
				len(data), maxSettingsBytes)
		}
		return data, nil
	}
	path, err := settingsFilePath(appID)
	if err != nil {
		return nil, err
	}
	if data, err = atomicfile.ReadFile(path, maxSettingsBytes); err != nil {
		return nil, fmt.Errorf("gui: load settings: %w", err)
	}
	return data, nil
}

// settingsSave stores data, replacing the previous blob.
func (w *Window) settingsSave(data []byte) error {
	appID, err := w.settingsAppID()
	if err != nil {
		return err
	}
	w.settings.mu.Lock()
	defer w.settings.mu.Unlock()
	if w.settings.memory {
		w.settings.blob = data
		return nil
	}
	if hook := w.settingsHook(); hook != nil {
		if err = hook.SettingsSave(appID, data); err != nil {
			return fmt.Errorf("gui: save settings: %w", err)
		}
		return nil
	}
	path, err := settingsFilePath(appID)
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return fmt.Errorf("gui: save settings: %w", err)
	}
	if err = atomicfile.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("gui: save settings: %w", err)
	}
	return nil
}

// settingsHook returns the platform's settings hook, or nil when there
// is no platform or it has no hook.
func (w *Window) settingsHook() settingsPlatform {
	sp, _ := w.nativePlatform.(settingsPlatform)
	return sp
}

// settingsAppID returns the window's app ID, checked for use as one
// path element. appinfo.Parse already allows only letters, digits, '.'
// and '-', but a hand-built AppInfo skips that check, and "." or ".."
// pass it; either would put the file outside the per-app directory.
func (w *Window) settingsAppID() (string, error) {
	id := w.AppInfo().ID
	if id == "" {
		return "", ErrNoAppID
	}
	if id == "." || id == ".." {
		return "", fmt.Errorf("gui: app ID %q is not usable as a directory name", id)
	}
	for i := 0; i < len(id); i++ {
		c := id[i]
		ok := c == '.' || c == '-' ||
			(c >= '0' && c <= '9') || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
		if !ok {
			return "", fmt.Errorf("gui: app ID %q may hold only letters, digits, '.' and '-'", id)
		}
	}
	return id, nil
}

// settingsFilePath returns os.UserConfigDir()/<appID>/settings.json. On
// Linux that follows XDG ($XDG_CONFIG_HOME, else ~/.config). On iOS it
// is inside the app sandbox, because HOME is the app's container.
func settingsFilePath(appID string) (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("gui: settings location: %w", err)
	}
	if dir == "" {
		return "", errors.New("gui: settings location: empty config dir")
	}
	return filepath.Join(dir, appID, settingsFileName), nil
}
