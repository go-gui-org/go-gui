# Spec: app manifest `appinfo.toml`

Issue: #849. Status: **implemented** (unreleased).

## Problem

An app's name, bundle ID, version and icon had no single source. Each Makefile
target, CI workflow and Go file typed them again. The runtime had its own
identity inputs (`SetFileAccessAppID`, `NativeMenubarCfg.AppName`), and nothing
tied them to the bundle. When the bundle ID and the runtime app ID differ, macOS
keeps NSUserDefaults and TCC grants under two identities, and reports no error.

## Decision

One file, `appinfo.toml`, beside the app's `main.go`. Two programs read it:

- `cmd/buildapp` reads it from disk. It uses `-manifest path`, or else
  `./appinfo.toml` when that file is there. A flag given on the command line
  wins over the file. A flag left at its default does not win (`flag.Visit`
  tells the two apart).
- The app embeds the same file with `//go:embed`, parses it with
  `appinfo.MustParse`, and gives it to `gui.WindowCfg.AppInfo`.

`NewWindow` fills only what the caller leaves empty: `Title` from `name`,
`WMClass` and the file-access app ID from `id`. `App.SetNativeMenubar` takes an
empty `AppName` from the main window's manifest. `App.OpenWindow` gives a window
with a zero `AppInfo` the main window's, because all windows of one app have one
identity. `(*Window).AppInfo()` reads it back; #848's settings store takes its
app ID from there.

The parser and the `Info` type are in the leaf package `gui/appinfo`. buildapp
imports that package only, not all of `gui`.

### Format

A strict subset of TOML. Every file that the parser accepts is also valid TOML.

1. `key = "value"`. Values are always double-quoted strings. Only `\"` and `\\`
   escapes.
2. `[section]` headers, one level. Sections are platform blocks: `[darwin]`
   (`category`) and `[linux]` (`categories`).
3. `#` comments, on their own line or after a value.
4. No arrays, numbers, nesting or multi-line values.
5. An unknown key or section, or a duplicate, is an error with its line number.

Top-level keys: `id`, `name`, `version`, `build`, `icon`. `id` may hold only
letters, digits, `.` and `-`, because it is a `CFBundleIdentifier` and a Linux
file name. A relative `icon` is relative to the manifest file. The input is
capped at 64 KiB.

### Version

`version` in the file is the source. buildapp's `-version` flag overrides it for
the package, so tag-based releases keep working. The running app reads the
embedded file, so it shows the file's version, not the flag. An app that shows
its version must bump the file or set it with `-ldflags -X`.

### Mobile

The app reads the embedded file on iOS and Android too. The package ID comes
from the host Xcode or Gradle project. The Go tools build a library and do not
write those files, so the manifest `id` is copied there by hand.

## Rejected Approaches

- **Packaging-only manifest** (buildapp reads it, the runtime does not). It
  removes the repetition, but the runtime still spells the ID by hand, so the
  mismatch stays.
- **Go code as the source** (`gui.AppInfo{…}` in Go, extracted by
  `go run . -print-app-info` or `-ldflags -X`). buildapp packages a compiled
  binary, often cross-compiled, so it has no reliable way to run it or read the
  source.
- **JSON.** No comments, and people edit the file by hand.
- **YAML, full TOML.** Each needs a new module dependency.
- **Loose INI** (unquoted values, `;` comments). With no spec, the parser's
  quirks become the format, and a later move to TOML would break files.
- **Read the version from the git tag in buildapp.** buildapp may run outside
  the source tree.
- **Package-global `gui.SetAppInfo`.** Process-global state; parallel tests
  collide, and forgetting the call is silent.
- **Per-window setter in `OnInit`.** It runs after the window is created, so the
  title needs a second set, and it races `RestoreFileAccess` for order.
- **Field name `WindowCfg.App`.** `(*Window).App()` already returns the parent
  `*App`; one name for two things misleads.
- **File name `gogui.toml`** (the issue's first choice), `app.toml` (Cosmos SDK
  uses it; too generic), `manifest.toml` (same file as Julia's `Manifest.toml`
  on the case-insensitive macOS file system), `app-manifest.toml` (long, and
  differs from the package name).
