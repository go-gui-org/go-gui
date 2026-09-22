# Follow OS light/dark appearance

Status: implemented. Issue #752.

## Problem

A window did not follow the OS light/dark setting. No backend read the OS
appearance. `TitlebarDark` (`gui/titlebar.go`) only pushes app state to the OS,
and every backend implements it as a no-op. Nothing flowed from OS to app: no
query and no change event. Every app had to write its own platform code to get
this.

## Design

Two layers, shipped together (recommendation B on primitive A from #752):

- **A, platform layer:** `SystemAppearance() (Appearance, bool)` and
  `OnSystemAppearance(func(Appearance))` on `*Window`, backed by a
  `nativeAppearance` sub-interface of `NativePlatform`. The `bool` is false
  where the OS reports no setting (nil platform, missing Linux schema, unknown
  desktop), and the app keeps its own theme.
- **B, policy layer:** `FollowSystemAppearance(light, dark Theme)` queries once,
  pins the matching theme, and re-pins on every OS change. The titlebar syncs to
  the applied theme (`TitlebarDark`).

An explicit `SetTheme` ends following until `FollowSystemAppearance` is called
again. `StopFollowSystemAppearance` ends it without changing the theme.
Package-level `SetTheme` is untouched: a following window is pinned, so
`appUpdateWindows` already skips it.

The OS change event arrives on a watcher thread. Each backend invokes one
callback; the gui side marshals it through `QueueCommand`, so the pin and the
user hook run on the frame thread.

Subscription is demand-driven: a backend runs its watcher only while a window is
following or hooked. `SetNativePlatform` replays a pending subscription
(follow/hook are reachable before the backend attaches), and `WindowCleanup`
unregisters.

| Platform | Query                                 | Change event                         |
| -------- | ------------------------------------- | ------------------------------------ |
| macOS    | `NSApp.effectiveAppearance`           | KVO observer, fanned out per window  |
| Windows  | registry `AppsUseLightTheme`          | `WM_SETTINGCHANGE`, re-query on flip |
| Linux    | `gsettings get … color-scheme`        | shared `gsettings monitor` child     |
| web      | `prefers-color-scheme` media query    | `change` listener                    |
| iOS      | VC `traitCollection` (C-held current) | `traitCollectionDidChange:`          |
| Android  | last uiMode pushed from Kotlin        | Kotlin `onConfigurationChanged` push |

Android is push-based because the Kotlin host owns the configuration: the host
pushes at startup and on change (wired in the `android_demo` host; `uiMode`
added to its `configChanges` so the activity is not restarted on toggle).
Queries before the first push report no setting.

A theme change applied by following fades when the window has a transition set
(`SetThemeTransition`, issue #753, `docs/specs/theme-fade.md`).

## Verification status

Built and tested on macOS (gui suite, metal build, ObjC syntax against the iOS
SDK, iOS backend type-check with the Makefile's `build-ios` flags). The Windows
(registry + `WndProc` hook), X11 (`gsettings`), web (`matchMedia`) and Android
(Kotlin push) paths are unverified on hardware, like `window-opacity.md` at
inception. The `gsettings` parser and the Windows flip-suppression have unit
tests that run everywhere.

## Rejected Approaches

- **C, sentinel theme.** The installed theme id is the hot-path key
  (`needsInstall`). A sentinel adds a branch to every frame, and any code that
  reads `Theme.id` sees a value that never matches a real theme.
- **Auto-follow by default.** Changes behavior for every existing app on
  upgrade. Breaks apps that pin `ThemeDark`.
- **Poll the OS each frame.** Costs a syscall or D-Bus call per frame on every
  platform, with no gain over an event.
- **Three-value `Appearance` enum (Light/Dark/Unknown).** An `Unknown` value
  leaks into every switch over the type. `(Appearance, bool)` keeps light/dark
  total and puts "no setting" in the second result, decided at API review.
- **Package-level follow default.** Window-only keeps the policy next to the pin
  it manages; the app default stays manual. Decided at API review; revisit if
  siblings ask.
- **D-Bus for Linux.** `gsettings` covers GNOME-family with no new dependency.
  KDE and bare window managers report no setting (accepted gap).
