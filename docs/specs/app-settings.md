# Spec: app settings store

Issue: #848. Status: **implemented** (unreleased). Part 1 of 3: window geometry
(issue step 5) and the sibling migrations (step 6) follow.

## Problem

go-gui had no API to store app settings. go-edit, go-term and go-kite each chose
a different location rule, and none of them worked on web or Android, where a
user config directory is missing or private.

## Decision

A typed struct, JSON-encoded, one blob per app:

```go
func LoadSettings[T any](w *Window, dst *T) error
func SaveSettings[T any](w *Window, v T) error
var ErrNoAppID error
```

- **App ID.** From `w.AppInfo().ID` (the #849 manifest). It is checked as one
  path element: letters, digits, `.` and `-`, and not `.` or `..`.
- **Defaults.** `LoadSettings` decodes into a struct the caller has filled with
  defaults. JSON sets only the fields the saved data has, so a new field keeps
  its default. Nothing saved yet returns `nil` and leaves `dst` as it is.
- **Errors keep `dst`.** Decoding goes into a copy, assigned only on success.
- **Size cap.** 1 MiB on load and on save, so Save never writes what Load
  refuses.

### Store selection

1. A window from `NewTestWindow`: in-memory, no disk.
2. A platform with the optional hook `SettingsLoad(appID) ([]byte, error)` /
   `SettingsSave(appID, []byte) error`: the hook. Web uses `localStorage`
   (`gogui.settings.<app ID>`), Android a file under the directory given to
   `android.SetFilesDir`.
3. Otherwise: `os.UserConfigDir()/<app ID>/settings.json`, written through
   `gui/internal/atomicfile` (temp file, sync, rename). On Linux that is XDG. On
   iOS `HOME` is the sandbox container, so the same path is correct there.

The in-memory store is keyed on `NewTestWindow`, not on a nil `NativePlatform`.
A real window has no platform until `backend.Run` attaches it, and a save in
that time must reach disk. On web and Android the file store fails there with an
error, which is loud, not silent.

The hook is an optional interface found by type assertion, like the system sound
hook (`gui/sound_system.go`). Each backend that implements it asserts the method
set at compile time, because a drifted signature would otherwise fall back to
the file store without an error.

### Scope

- App state only. A file the user edits by hand (go-term's `config`) stays a
  file the app owns. A running app overwrites a hand edit of `settings.json`.
- No secrets. The data is plain JSON; go-kite's `0o600` token file stays in
  go-kite.

## Rejected Approaches

- **Key-value store** (`String(key, def)`, `SetString`, …). About 10 exported
  methods, flat string keys, no schema: a typo in a key fails silently.
- **Location only** (`AppConfigDir(appID)`). Leaves web, mobile, atomic writes
  and the test store to every app.
- **`LoadSettings[T](w) (T, error)`.** Returns zero for a field the saved data
  lacks, so the app cannot tell "unset" from a real zero `bool` or `int`.
- **An explicit `appID string` argument.** Spells the ID again that #849 put in
  one place, and leaves no window to hold the test store.
- **Nil `NativePlatform` selects the memory store.** Loses a real app's early
  saves with no error (see Store selection).
- **New methods on `NativePlatform`.** Breaks every implementer outside the
  repo; the optional interface does not.
- **NSUserDefaults on macOS and iOS.** Needs cgo for no gain: the file default
  is already inside the sandbox container on both.
- **gob, XML or TOML.** gob is binary and Go-only, and `localStorage` needs
  strings. XML has no plain mapping for maps. Full TOML needs a new module.
- **A public `atomicfile` package.** go-edit's document save also follows
  symlinks and keeps the file mode; a settings helper must not carry that.
  Internal can become public later without a break.
