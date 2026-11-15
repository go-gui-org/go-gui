# Spec: window visibility event

Issue #1170. Status: **implemented** (unreleased).

## Problem

A media player must stop decoding and pause its audio when nobody sees its
window: the window is minimized, the app hid it, or the app went to the
background on a phone (go-gui-org/go-video#1). An app could not learn any of
this.

- `Window.IsVisible` reported only the app's own `Hide`.
- gui already tracked whether the OS shows the window, for its animation loop
  (`w.occluded`, #943), but no app could read it.
- Android, iOS and web did not report it at all.

## Decision

Two getters and one event.

- `Window.IsVisible()` is false after the app's `Hide`. New: where no window can
  be hidden (web, iOS, Android, native Wayland), `Hide` does nothing and
  `IsVisible` stays true; see below.
- `Window.IsOccluded()` is new: true while the OS shows the window nowhere.
- `EventVisibilityChanged` is new. It fires when `IsVisible() && !IsOccluded()`
  changes, and reaches `Window.OnEvent`. The event carries no state; the app
  reads the two getters.

```go
w.OnEvent = func(e *gui.Event, w *gui.Window) {
    if e.Type == gui.EventVisibilityChanged {
        shown := w.IsVisible() && !w.IsOccluded()
        player.SetPaused(!shown)
    }
}
```

`Hide`, `Show` and `gui.DispatchWindowOccluded` all end in one function
(`reportVisibility`) that compares the combined state with the one last
reported. So every platform sends one event per real change:

| Platform | `Hide()`              | Occlusion flag on `Hide`              |
| -------- | --------------------- | ------------------------------------- |
| X11      | `XUnmapWindow`        | flips: `UnmapNotify` reports occluded |
| macOS    | `orderOut:`           | flips: AppKit reports it              |
| Win32    | `ShowWindow(SW_HIDE)` | does not flip                         |
| Wayland  | nothing: cannot hide  | `Hide` does nothing; see below        |

### `Hide` where no window can be hidden

On web, iOS and Android the backend's `HideWindow` does nothing: there is no
window manager to ask. On native Wayland it does nothing yet
(`docs/specs/wayland-backend.md`); found on GNOME, 2026-11-15. `Window.Hide`
still set the app-side flag, so `IsVisible` turned false while the user kept
seeing the app. With the event, a player would have paused in front of the user
(found in Chrome, 2026-11-14).

Those platforms now have a method `CanHideWindow() bool` that returns false. The
Linux platform answers false on Wayland and true on X11. gui asks for it through
an unnamed interface, as it does for `PrefersReducedMotion`, so nothing is
exported and no other backend changes. `Hide` then does nothing, `IsVisible`
stays true, no event is sent, and `DebugWindowDegraded` reports the call. A
platform without the method can hide.

A `Hide` made before the backend attached finds no platform to ask and marks the
window hidden. When a platform that cannot hide then attaches, the mark is
cleared and the app gets the event, so `IsVisible` does not stay false for a
window the user sees. Only the mark is put right: a `Minimize`, `Maximize` or
`SetFullscreen` made while the window was marked hidden was a no-op, as on any
hidden window, and is not made up for. On a platform that can hide, the mark
stands: the backend reads it when it creates the window and does not show it, so
the app starts with no window on screen (#1270).

### Delivery

- The event is sent inside the call that changed the state, on the UI thread. It
  never goes through the command queue: on Android, iOS and web the frame loop
  stops in the background, so a queued command would run when the app comes
  back, too late to pause audio.
- It is a new `Event`, not a backend's shared one. `Hide` inside a click handler
  can make the OS report the occlusion before `Hide` returns, while the click is
  still being handled (the #1127 hazard).
- It passes the unfocused gate (`eventAllowed`). A minimized window has lost
  focus before it is hidden, and on web `blur` comes before `visibilitychange`.
- It takes no time-travel snapshot, like the focus events.
- It asks for no frame by itself. A window going out of sight draws none. The
  path that shows it again (`Show`, or the occlusion report) marks one full
  refresh and wakes the main thread once, which also draws what the handlers
  changed. This keeps the #943 rule that a hide wakes nothing.
- While the time-travel debugger has the window frozen, the event is held back
  and sent on resume, if the state then differs from the one last reported.
- Wayland reports nothing before the window is ready, like focus: a window that
  opens on another workspace gets its first configure before `OnInit` has run.
- `DispatchWindowOccluded` is now UI-thread only, because it runs the app's
  handler.

### Where the report comes from

| Platform | Source                                                | Reported                                                 |
| -------- | ----------------------------------------------------- | -------------------------------------------------------- |
| Windows  | `WM_SIZE` with `SIZE_MINIMIZED` (#943)                | minimized                                                |
| macOS    | `NSWindowDidChangeOcclusionStateNotification` (#943)  | minimized, fully covered, other space                    |
| X11      | `UnmapNotify` / `MapNotify` (#954), `_NET_WM_STATE`   | minimized                                                |
| Wayland  | `suspended`, or a stalled frame callback (#953, #959) | minimized, other workspace, locked screen, fully covered |
| Android  | new: the host calls `android.VisibilityChanged`       | activity stopped                                         |
| iOS      | new: scene entered background / will enter foreground | app in the background                                    |
| Web      | new: `visibilitychange` on `document`                 | page hidden (other tab, minimized browser)               |

X11: a window manager minimizes in one of two ways. Some unmap the window.
mutter (GNOME) keeps it mapped and adds `_NET_WM_STATE_HIDDEN` to
`_NET_WM_STATE`. Until this change only the unmap was reported (#954), so a
minimized window under GNOME counted as shown. Both now count.

Android: the backend cannot see the activity, so the host reports it from
`onStart` / `onStop` through `queueEvent`. `GLSurfaceView` runs a queued event
while it is paused, so the report from `onStop` arrives at once. The host skips
the report when `isChangingConfigurations` is true. Android then replaces the
activity within milliseconds (seen on leaving split screen, Android 17), so the
app never leaves the screen, and the old and the new activity would report from
two GL threads in no fixed order.

iOS: the display link used to pause when the scene deactivated. That also
happens under Control Center, the notification pull-down and a system alert,
while the app is still on screen. Pause and report now share one notification
pair, `UISceneDidEnterBackgroundNotification` /
`UISceneWillEnterForegroundNotification`, so the window reports as shown exactly
while frames are drawn.

### Known gaps

- Windows: a covered window, the lock screen and another virtual desktop send no
  message. The lock screen could come later from
  `WTSRegisterSessionNotification`.
- X11: minimize only. A covered window is not reported.
- Wayland: a fully covered window is reported about one second late.
- Wayland under GNOME: opening the Activities overview within a few seconds of
  minimizing sends a short "shown, then hidden again" pair. GNOME paints the
  window's thumbnail there, which ends the stalled-frame state (#959), and marks
  the window suspended only after it has been out of sight for some seconds.
  With a longer wait the suspended state holds and there is one pair per cycle
  (seen on Debian 13, 2026-11-15). The window is on screen as a thumbnail in
  that moment, so the report is left as it is.
- Web: whether a covered browser window counts depends on browser and OS.

### The animation loop while hidden

On the desktop, a hidden window's animation loop keeps ticking and queues its
callbacks, and the backend wakes the main thread to run them (#943). App
callbacks keep their timing.

Android, iOS and web cannot do that: their frame loop stops while the app is
hidden, and nothing runs the queue. Each `KeyframeAnimation` with `OnValue`
(ThinkingOrb, Skeleton, the indeterminate ProgressBar, MathSpinner) and each
`Animate` with `Repeat` would add one command per tick, about 9 MB per hour and
animation, and all of them would run in the first frame after the return.

Before this change the problem ended by itself: no frame touched the view-bound
heartbeats, so those animations were cancelled after two seconds, and spinners
restarted at phase zero on return. Once the mobile backends report occlusion,
the hidden tick would renew the heartbeats instead and keep the animations
queuing. So the fix is part of this change.

A window with no wake function is one gui cannot wake. Android, iOS and web set
none; every desktop backend does. For such a window the hidden tick does nothing
and parks the loop, with its animations still registered. On show,
`DispatchWindowOccluded` renews the heartbeats once, resumes the loop and marks
the shown refresh. Phases come out right: every `Update` except spring and
smooth scroll reads the clock, not `dt`, and spring samples its own clock.

Divergence: on Android, iOS and web, animation callbacks do not run while the
app is hidden. They go on with the first frame after it returns.

## Testing

- `gui/window_visibility_event_test.go`: one event per change for each order of
  `Hide`, `Show` and the occlusion report; a report nested inside `Hide`; the
  unfocused gate; no snapshot; a window without a wake function queues nothing
  while hidden, keeps its animation, and continues at the right phase.
- `gui/animation_occluded_test.go`: the desktop contract. These tests set a wake
  function.
- Windows, by hand with `examples/visibility`: minimize and restore, from the
  title bar and with the Hide button.
- Android, by hand with `examples/android_demo` on Android 8.1 and Android 17:
  Home and back, the overview, split screen. Entering split screen stops the
  activity while the second app is picked, which counts. Leaving it relaunches
  the activity, which does not.
- Web, by hand with `examples/visibility`: tab switch, minimize, a long pause, a
  page opened in a background tab, and `Hide`, which logs nothing there. Chrome
  and Firefox on Windows, Brave and Firefox on Debian 13 (Wayland), Safari on
  macOS and iOS, Chrome on Android. `gui/backend/web/visibility_test.go` drives
  the report with a fake document under Node.
- macOS, by hand with `examples/visibility`: minimize, `Hide` (one event,
  although AppKit also reports the occlusion), a covered window, another Space.
- iOS, by hand with `examples/ios_demo`, in the simulator (iOS 27.2) and on an
  iPhone: Home and back counts once, Control Center and the notification
  pull-down do not count and the app keeps drawing under them, lock and unlock
  counts once, and fullscreen still works.
- Native Wayland, by hand with `examples/visibility` on Debian 13 (GNOME):
  minimize and restore through Alt-Tab and through the Activities overview; see
  Known gaps for the overview. This run found that `Hide` cannot hide a window
  there.
- X11, by hand with `examples/visibility` on Debian 13 (GNOME, XWayland):
  minimize and restore through Alt-Tab and through the Activities overview each
  give one pair, `Hide` gives one pair, maximize logs nothing. The first run
  gave no event for minimize, which led to the `_NET_WM_STATE` report above.

## Rejected Approaches

- **Keeping `Hide` as it was on web, iOS and Android, and documenting it.** It
  leaves a getter that is wrong on three platforms and an event that repeats the
  error.
- **`HideWindow() bool` on `NativePlatform`.** Same result as the optional
  method, but it breaks every backend and test stub at compile time.

- **`WM_ACTIVATEAPP` on Windows.** It reports activation, which `EventFocused` /
  `EventUnfocused` cover.
- **iOS deactivate / activate.** Also fires for Control Center and the
  notification pull-down while the app is still visible.
- **Android `onPause` / `onResume`.** A paused activity "is still visible if the
  user is in multi-window mode", and the lifecycle guide says to use `onStop`
  for work that depends on visibility.
- **An event per flag, plus one on `Hide` / `Show`.** X11 and macOS flip the
  occlusion flag on `Hide` too, Win32 does not, so an app would get one or two
  events per hide depending on the OS.
- **A visibility enum (hidden / occluded / background).** No two platforms fill
  it the same way.
- **A `WindowCfg.OnVisibilityChanged` hook, like `OnCloseRequest`.** It needs no
  entry in the unfocused gate, but it differs from how focus is reported and
  grows `Cfg`.
- **Reusing or deleting `EventIconified`, `EventRestored`, `EventSuspended`,
  `EventResumed`.** Nothing sends them. Deleting them renumbers `EventType` for
  no gain. They are untouched.
- **A second entry `gui.DispatchWindowSuspended(w, bool)`** for backends whose
  frame loop stops. It is an export on top of what the wake function already
  says: a window with none is one gui cannot wake.
- **Parking the loop on the desktop too.** It breaks the #943 contract that
  animation callbacks keep their timing while a window is minimized.
- **Dropping the queued callbacks on hide.** An `OnDone` may carry a state
  change, and the backlog between the frame loop stopping and the report is
  small.
- **Letting the event mark a refresh and wake the main thread, like input.** A
  hide would wake a loop that has nothing to draw, and a show would wake it
  twice. The backends' occlusion tests pin "hide wakes nothing, show wakes once"
  (#943).
