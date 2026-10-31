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
  pins the matching theme, and re-pins on every OS change. Each apply calls
  `TitlebarDark`. The Metal backend implements it on macOS (#1207, see below),
  and the GL backend on Windows (#1201, see below) and on Wayland for a frame
  drawn by libdecor's GTK plugin (#1192). So only there does the titlebar
  follow.

An explicit `SetTheme` ends following until `FollowSystemAppearance` is called
again. `StopFollowSystemAppearance` ends it without changing the theme.
Package-level `SetTheme` is untouched: a following window is pinned, so
`appUpdateWindows` already skips it.

The OS change event arrives on a watcher thread. Each backend invokes one
callback; the gui side marshals it through `QueueCommand`, so the pin and the
user hook run on the frame thread.

Subscription is demand-driven: a backend runs its watcher only while a window is
following or hooked. `SetNativePlatform` replays a pending subscription
(follow/hook are reachable before the backend attaches), and a stored
`TitlebarDark` (#1201). `WindowCleanup` unregisters.

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

### Titlebar on Windows

Windows draws every window's caption light, whatever the system setting, until
the app opts in. `TitlebarDark` sets `DWMWA_USE_IMMERSIVE_DARK_MODE` on the
window (#1201). Windows 10 before 20H1 (1809 to 1909) knows the attribute as 19
instead of 20, so a rejected 20 is retried as 19. Before 1809 neither exists and
the caption stays light. The setting is per window, unlike libdecor's
per-process variant.

After a change, `SetWindowPos` with `SWP_FRAMECHANGED` (no move, size, stacking
or activation) repaints the frame. Windows 11 repaints at once without it.
Windows 10 is reported to keep the old caption until a focus change or a resize.
The call sends `WM_NCCALCSIZE` and `WM_NCPAINT` but no `WM_SIZE`, so no frame
runs inside the caller's. The window remembers the last variant set, so the
repeat calls from `FollowSystemAppearance` cost nothing.

With "Show accent color on title bars" on, Windows paints the active caption in
the accent color whichever variant is set. Only the inactive caption shows it.

The backend shows the window in `New`, before `Run` calls `OnInit`. A
`FollowSystemAppearance` from `OnInit` on a dark desktop therefore lands on a
window already on screen, and Windows animates the caption from light to dark.
Sampled off the screen, the caption reads up to 157 and fades to 32 over about
130 ms. A `FollowSystemAppearance` before `Run` is applied by
`SetNativePlatform` inside `New`, before `ShowWindow`, and the caption never
reads above 32. The docs say so, and each app picks the order: only the app
knows whether its `OnInit` is slow.

`Window` stores the last `TitlebarDark` and `SetNativePlatform` replays it, like
`SetWindowOpacity`, so an app that pins a dark theme gets the same by calling
`TitlebarDark` before `Run`. Measured the same way, it never reads above 32; the
same calls from `OnInit` fade from 128. A following window that applies an OS
setting on attach takes its titlebar from that instead: one push, and the
setting wins. A window whose app never called `TitlebarDark` gets no push. On
Wayland the window's thread is not recorded yet during `SetNativePlatform`, so
`wlWindow.titlebarDark` posts the call to the loop. It runs once the window is
registered for frame repaints and before its first frame. The frame shows dark
from the start under weston in the Docker harness and on GNOME (Debian 13,
libdecor 0.2.2), where a log shows nothing resetting the GTK setting afterwards.
Under mutter in the harness, which takes no screenshots, the log shows the same
sequence.

### Titlebar on macOS

`TitlebarDark` sets `NSWindow.appearance` on that window (#1207): `DarkAqua` for
dark, `Aqua` for light. AppKit draws the frame, the title, the window buttons
and a vibrancy backdrop (`SetWindowVibrancy`) in that appearance. The HUD
material is dark either way. The pin is per window, as on Windows. App-modal
dialogs (`runModal`) and the menu bar take the app's appearance, so they keep
the system's. A window whose app never calls `TitlebarDark` has no appearance of
its own and follows the system, as before.

Light pins `Aqua` instead of clearing the window's appearance. A cleared one
follows the system, so a light theme on a dark desktop would keep a dark frame.
A following window is pinned to the system's value on each apply. After an OS
change, its frame waits for the watcher's queued apply and switches with the
theme.

On macOS the frame thread is the main thread, so the call reaches AppKit at
once, like `SetWindowOpacity`. A call that repeats the window's appearance
changes nothing. `metalWindowCreate` orders the window in before
`SetNativePlatform` replays a stored `TitlebarDark`. Both that replay and
`OnInit` run before the event loop's first pass. In between, the first
`metalAppFinishLaunch` runs `[NSApp run]` once inside `metalWindowCreate`, with
the window already on screen. An app launched as a bundle waits there for Launch
Services' open event. A probe pinned dark on a light desktop and held `OnInit`
for 2 s before the loop. Started from Terminal and as an `.app` launched with
`open`, it showed a dark frame throughout the hold, whether it called
`TitlebarDark` before `Run` or from `OnInit`. At normal speed no run flashed
light.

With vibrancy the window's background is clear, and the backdrop covers only the
content area. Over that clear background AppKit draws a dark frame's title bar
clear, so the desktop shows through it unblurred, and a light one as usual. A
dark desktop does the same without `TitlebarDark`: `examples/vibrancy` on
`main`, which never calls it, shows the clear title bar there. It belongs to
vibrancy, not to the pin.

### Titlebar on X11

On X11 the window manager draws the frame, and `TitlebarDark` does nothing: the
frame keeps the window manager's variant. GTK asks for a dark frame with the
`_GTK_THEME_VARIANT` property on the window, and mutter followed it up to
GNOME 43. Since GNOME 44 the X11 frames come from `mutter-x11-frames`, which
ignores it, in an X11 session and under Xwayland alike
([mutter#2792](https://gitlab.gnome.org/GNOME/mutter/-/issues/2792), open since
2023). Set by hand with
`xprop -f _GTK_THEME_VARIANT 8u -set _GTK_THEME_VARIANT dark` on a go-gui window
under GNOME 48 (Debian 13, Xwayland), the frame stays light. Mutter closed a
merge request adding color-scheme support to those frames unmerged
([mutter!2541](https://gitlab.gnome.org/GNOME/mutter/-/merge_requests/2541)) and
in 2026 merged "window: Stop exposing gtk-theme-variant"
([mutter!5062](https://gitlab.gnome.org/GNOME/mutter/-/merge_requests/5062)).

## Verification status

Built and tested on macOS (gui suite, metal build, ObjC syntax against the iOS
SDK, iOS backend type-check with the Makefile's `build-ios` flags). The Windows
(registry + `WndProc` hook), X11 (`gsettings`), web (`matchMedia`) and Android
(Kotlin push) paths are unverified on hardware, like `window-opacity.md` at
inception. The `gsettings` parser and the Windows flip-suppression have unit
tests that run everywhere.

The Windows titlebar (#1201) was checked on Windows 11 build 26300 by screen
capture: with the desktop light, pinning dark and light switches the inactive
caption between dark and light at once, and DWM reads the attribute back. A unit
test reads it back from a real window. Windows 10 is unverified.

The macOS titlebar (#1207) was checked on an M1 Mac mini. The real-window test
in the Metal backend's main-thread tests reads the appearance back before the
first call, after each call, and for a call made before `New`. No CI job sets
`GO_GUI_MAIN_THREAD_TESTS`, so it runs only on a Mac by hand.
`examples/system_appearance` follows the desktop both ways, and a pinned theme
keeps its frame when the desktop changes. Under a dark pin on a light desktop,
the vibrancy backdrop is dark, and a native message box and the app's menus stay
light.

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
- **Windows titlebar from `DWMWA_CAPTION_COLOR` and `DWMWA_TEXT_COLOR`.**
  Windows 11 only, and they take a color, not a variant, so `TitlebarDark` would
  need a new color API. Could return as its own feature (a caption in the
  theme's exact color).
- **`NSApp.appearance` for the macOS titlebar.** `SystemAppearance` and its
  watcher read `NSApp.effectiveAppearance`. An app-wide pin replaces that with
  the app's own choice, so a following window would never see the system change
  again. It would also switch every window and the app's dialogs at once.
- **Light as a cleared appearance (`nil`) on macOS.** A cleared appearance
  follows the system, so a light pin on a dark desktop would keep the frame
  dark.
- **An appearance on the macOS titlebar view alone.** That view is private to
  AppKit, reached through the superview of `standardWindowButton`, and its
  hierarchy changes between releases. The vibrancy backdrop would also stay out
  of step with the theme.
- **A macOS titlebar painted in the theme's color**
  (`titlebarAppearsTransparent` and a background color). It takes a color, not a
  variant: the same reason as `DWMWA_CAPTION_COLOR` on Windows.
- **Windows titlebar through undocumented `uxtheme.dll` ordinals**
  (`SetPreferredAppMode`, `AllowDarkModeForWindow`). Unsupported and changed
  between builds. Their extra effect, dark Win32 menus and scrollbars, is of no
  use to go-gui, which draws its own.
- **Dark caption at window creation from the system setting.** Changes every
  app's caption on upgrade and gives a light-themed app a dark caption.
  `FollowSystemAppearance` already covers the following app.
- **Other Windows 10 repaints.** A faked `WM_NCACTIVATE` toggles activation
  state, hiding and showing the window flickers, and no repaint leaves Windows
  10 showing the old caption.
- **Hiding the start-up caption animation in the backend.** Cloaking the window
  (`DWMWA_CLOAK`) from `ShowWindow` until `OnInit` has run, or calling
  `ShowWindow` after `OnInit`, would remove it for every app. Both keep a window
  with a slow `OnInit` off screen, and showing after `OnInit` also undoes an
  `OnInit` that hides the window (a tray app) unless tracked. Each app chooses
  instead, by where it calls `FollowSystemAppearance` or `TitlebarDark`.
- **A `WindowCfg` field for the start-up titlebar.** New API, read by every
  backend, for what the `TitlebarDark` replay gives with none: two ways to set
  the same thing.
- **Setting `_GTK_THEME_VARIANT` from `TitlebarDark` on X11.** It would help
  only GNOME 43 and older in an X11 session, and window managers that read the
  hint (which ones is unverified). Current GNOME ignores it, and neither of
  those setups was at hand to verify it on.
- **A titlebar go-gui draws itself on X11** (a frameless window plus a gui
  titlebar). It would follow the theme under every window manager, but it means
  move, resize, snapping, buttons and shadows of its own: a feature of its own,
  not a follow-up.
