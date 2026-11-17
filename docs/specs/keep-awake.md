# Spec: keep the display awake

Issue #1172. Status: **implemented** (unreleased).

## Problem

A video player must keep the display from going dark while it plays
(go-gui-org/go-video#1). go-gui had no way to ask the OS for that. The request
also has to be safe when two players in one app start and stop at different
times, and it must not keep the display on for a window nobody sees.

## Decision

A token per caller:

```go
release := w.KeepAwake("Playing a video")
// ... playback ...
release()
```

- Each call is its own request. `release` ends only that one, and a second call
  of it does nothing. The display may sleep once every request is released.
- The tokens belong to their window. `WindowCleanup` ends a closed window's
  requests.
- The OS gets one request for the whole app. It is held while some window with a
  token is on screen: `IsVisible() && !IsOccluded()` (issue #1170). A minimized
  or hidden window, or an app in the background, lets the display sleep, and
  showing the window asks again.
- `KeepAwake` and `release` are safe from any goroutine, also from `AmendLayout`
  under the frame lock.
- The reason is a label for people. The OS shows it where it lists what keeps
  the computer awake. It gets one reason: that of the oldest request at the
  moment the display starts being kept on. It stays while the display stays on,
  also after that request ends. Updating it would mean ending and making the OS
  request again on every release, for a label.

### How gui decides

gui keeps the tokens (`gui/keep_awake.go`) and calls one backend method on the
changes of the app-wide state:

```go
SetKeepAwake(on bool, app, reason string) error
```

- `KeepAwake` and `release` change the token list under a mutex and queue a
  check with `QueueCommand`. The check runs on the UI thread at the start of the
  next frame and reads the live state, so a request taken and released before
  that frame never reaches the OS.
- A visibility change checks at once, in `reportVisibility`, before the
  time-travel freeze check. On Android, iOS and the web no frame runs while the
  app is hidden, so a queued check would come too late.
- `WindowCleanup` always checks. A `release` just before a window closes queued
  its check on that window, and a destroyed window's commands never run.
- `SetNativePlatform` checks too, for a request made before the backend
  attached.
- A returned error is reported once under `DebugWindowDegraded`. After a failed
  on, nothing is held, and the next change asks again. The web is the exception:
  the browser refuses later, through a promise, so the refusal goes to the
  browser console and gui takes the request as held until the next off (the page
  hidden and shown, or every request released). A lock the browser grants and
  later drops while the page is still visible (a power saver turning on) is
  asked for again by the web backend itself, unless it was held for less than a
  second, which would be a loop.
- A `QueueCommand` from another goroutine can reach a window that is being
  destroyed. That is safe here as for any other caller: every backend's wake
  call survives its window (Win32 posts to a dead window and fails, Wayland
  checks a closed fd under its mutex, xgb returns after `Close`, macOS posts an
  app-level event), and the queued command of a destroyed window never runs,
  which `WindowCleanup` makes up for by checking itself.

The two strings are never empty, because gnome-session refuses an inhibit
without a reason. The app name is `AppInfo.ID` (the Linux `.desktop` name GNOME
looks the app up by), then `AppInfo.Name`, then the window title, then
`"go-gui"`. The reason is the caller's, trimmed, or
`"Keeping the display awake"`. Both are valid UTF-8 (D-Bus requires it), have no
NUL, and are cut at 128 characters, the limit macOS has for an assertion name.

### Backends

The OS request belongs to the process, so each backend keeps it at package
level: gui may end it through another window's platform than the one that began
it.

| Platform | Request                                                                     | Notes                                                                                                                                  |
| -------- | --------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------- |
| Windows  | `PowerCreateRequest`, then `PowerSetRequest` for the display and the system | A display request alone lets the system sleep. On Modern Standby on battery, Windows ends it five minutes after the sleep timeout      |
| macOS    | `NSProcessInfo beginActivityWithOptions:NSActivityIdleDisplaySleepDisabled` | Foundation only                                                                                                                        |
| Linux    | `org.freedesktop.ScreenSaver.Inhibit` on a private session-bus connection   | X11 and Wayland. The service ends an inhibit whose caller left the bus, so a crash releases it. sway and Hyprland have no such service |
| Web      | `navigator.wakeLock.request("screen")`                                      | HTTPS or localhost. A refusal comes from the promise and goes to the console                                                           |
| iOS      | `UIApplication.idleTimerDisabled`, on the main queue                        | Works under a Swift host too                                                                                                           |
| Android  | the host polls `PendingKeepAwake()` and sets `FLAG_KEEP_SCREEN_ON`          | The backend cannot call Java; see `examples/android_demo`                                                                              |

## Testing

- gui (`gui/keep_awake_test.go`), with a spy platform: two tokens and one
  release keep it on; a double release does nothing; a token taken and released
  in one frame makes no call; closing a window ends its requests, also right
  after a release; hide, show, minimize and restore end and take it again, also
  while frozen; a request made before the platform attaches is asked for on
  attach; the request follows windows on screen across windows; the first reason
  is shown; the strings are never empty; a platform error is reported once and
  asked again; many goroutines at once, under `-race`.
- Windows: the system's execution state, read with `CallNtPowerInformation` (no
  admin rights needed), has the display and system bits while held; a real
  window's minimize ends the request.
- Linux: a private `dbus-daemon` with a fake ScreenSaver service.
- macOS: `pmset -g assertions` lists the reason while held.
- Web: a fake `navigator.wakeLock` whose promises the test settles.
- Android: the state getter.

Checked by hand: Windows (powercfg, a minimized window lets the screen turn off,
a visible one keeps it on past the timeout).

## Rejected Approaches

- **`SetKeepAwake(bool)` on the window.** A boolean lets two players clear each
  other's request; a counter behind it leaks on two `true` calls and ends
  someone else's request on a stray `false`.
- **The count in the platform layer.** gui's tests run with nil or spy
  platforms, so it would be untestable, and six backends would each keep a copy.
- **Holding the request while the window is hidden or minimized.** On desktop
  nothing else would release it, so a minimized player that forgot to pause
  would keep the display on for ever. Keeping the system awake for background
  audio is a different request on every OS, and not part of this.
- **A main-thread-only API.** gui has no main-thread check, so a call from a
  playback goroutine would be a silent data race.
- **One OS request per token**, so each reason shows. Android, iOS and the web
  have one switch, so gui would need two models.
- **A random string when the reason is empty.** It tells the user nothing where
  the OS lists what keeps the computer awake; a constant cannot be empty either.
- **`SetThreadExecutionState`** on Windows. It binds the state to the calling
  thread, so every call would have to run on the UI thread's locked OS thread.
  **The display flag alone** lets the system sleep with the display on.
- **`IOPMAssertionCreateWithName`** on macOS. Same effect as the activity, but
  it links IOKit.
- **`SetKeepAwake(on bool)` without strings**, as first planned. The app name
  and reason must reach the OS, so the method carries them, and returns an error
  that gui reports, so no backend needs a debug method of its own.

## Follow-ups

- Wayland's `zwp_idle_inhibit_manager_v1`, for compositors without the
  ScreenSaver service (sway, Hyprland).
