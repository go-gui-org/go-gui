# Spec: window state and placement

Issue #1220. Status: **implemented** on every backend (unreleased). This also
covers #1171, fullscreen on web, iOS and Android; see Phasing.

## Problem

An app cannot minimize, maximize, restore, resize or move its own window, and it
cannot go fullscreen. A custom title bar (`DecorationNone`) has to offer
minimize and maximize buttons it cannot implement. A media player, a
presentation and a kiosk all need fullscreen (go-gui-org/go-video#1).

## Decision

Design A: one verb per operation on `Window`, one matching method per operation
on `NativePlatform`, in a new `nativeWindowState` sub-interface. A backend that
misses a method does not compile.

| Window method                     | Native method               |
| --------------------------------- | --------------------------- |
| `Minimize()`                      | `MinimizeWindow()`          |
| `Maximize()`                      | `MaximizeWindow()`          |
| `Restore()`                       | `RestoreWindow()`           |
| `SetFullscreen(bool)`             | `SetWindowFullscreen(bool)` |
| `SetWindowSize(width, height)`    | `ResizeWindow(w, h)`        |
| `SetPosition(x, y)`               | `MoveWindow(x, y)`          |
| `Position() (x, y int, ok bool)`  | `WindowPosition()`          |
| `IsMaximized()`, `IsFullscreen()` | none: gui's atomics         |

State comes back through one call,
`gui.DispatchWindowState(w, maximized, fullscreen)`, the shape of
`DispatchWindowOccluded`.

### Contracts

- **Thread.** As `Hide` and `Show`: UI thread or `Window.QueueCommand`.
- **A queued state change must check again.** gui refuses one on a hidden
  window, but `Maximize` followed by `Hide` in one handler reaches the pump
  after the hide. Windows and macOS drop it there, or it would show the window.
  X11 needs no check, because its requests reach the window manager in call
  order.
- **Native calls must not deliver events synchronously.** A native method posts
  the request and the backend applies it from its message pump. These calls are
  made from `OnClick`. On Win32 and macOS the OS answers a resize inline, and
  the backends' resize handlers run `EventFn` and `FrameFn` from that answer: a
  nested `EventFn` overwrites the event in flight (the #1127 hazard), and a
  nested `FrameFn` rebuilds the tree an event handler is still walking.
- **Getters report what the OS confirmed.** `IsFullscreen` stays false until the
  backend dispatches, which on macOS is after the animation. This is what lets
  them follow a change the user makes through the OS.
- **A state change redraws.** The dispatch marks a layout refresh and wakes the
  main thread, so a view reading a getter updates even when no `EventResized`
  arrives: on X11 the state and the size are separate events.
- **Flags, not a mode.** Maximized and fullscreen are independent, so a window
  maximized before fullscreen returns to maximized.
- **`Restore`** undoes minimize, then maximize. It leaves fullscreen alone:
  `SetFullscreen(false)` is the only exit.
- **Before the backend attaches**, calls are stored and replayed by
  `SetNativePlatform`, like `TitlebarDark`. The last request of each kind wins.
  That replaces a `WindowCfg` field for starting maximized or fullscreen.
- **`SetWindowSize`** takes logical pixels of the content area, the unit
  `WindowSize` reads. It clamps to `WindowSizeLimits`.
- **`Position`** is the top-left corner of the content area in OS desktop units:
  physical pixels on Windows and X11, points on macOS. `SetPosition(Position())`
  is a no-op. Logical units cannot round-trip across monitors of different DPI.
  `ok` is false where the OS does not say (Wayland, a minimized window).
- **No-ops, in gui so every backend agrees.** `Maximize` and `SetWindowSize` on
  a `FixedSize` window. `SetWindowSize` and `SetPosition` while maximized or
  fullscreen. `Minimize`, `Maximize`, `Restore` and `SetFullscreen` while
  hidden: on Windows they would show the window behind `Hide`.
- **Fullscreen ignores `MaxWidth` and `MaxHeight`.** They bound the user's drag
  and the maximize button, not a window filling its monitor.

## Backends

| Backend | Min / max / restore     | Fullscreen                 | Position  |
| ------- | ----------------------- | -------------------------- | --------- |
| Windows | `ShowWindow`, posted    | Style swap, monitor rect   | Yes       |
| macOS   | `miniaturize:`, `zoom:` | `toggleFullScreen:`        | Yes       |
| X11     | EWMH, `WM_CHANGE_STATE` | `_NET_WM_STATE_FULLSCREEN` | Yes       |
| Wayland | xdg or libdecor         | xdg or libdecor            | No: no-op |
| Web     | No-op                   | Fullscreen API on the page | No-op     |
| Android | No-op                   | Bars, through the host     | No-op     |
| iOS     | No-op                   | Status bar, home indicator | No-op     |

Wayland has no client request to un-minimize a window, so `Restore` does nothing
for a minimized window there.

`Show` on Windows used `SW_RESTORE` unconditionally, which un-maximizes a
maximized window. It now restores only a minimized window.

## Backend notes

These cost a wrong first attempt each. Keep them when changing the code.

- **Windows: `WS_MAXIMIZE` is cleared while fullscreen.** With the bit set,
  Windows treats the window as already maximized, so `SetWindowPlacement` on the
  way back keeps the monitor rect and never asks `WM_GETMINMAXINFO` for the work
  area: a frameless window stays over the taskbar and `MaxWidth` is ignored. The
  saved placement answers `IsMaximized` meanwhile.
- **X11: move with the default gravity and subtract `_NET_FRAME_EXTENTS`.**
  `StaticGravity` names the client's corner directly, but openbox stores those
  coordinates as the frame's, so the first restore lands the window off by the
  frame.
- **X11: `_NET_WM_STATE_HIDDEN` as well as `UnmapNotify`.** mutter keeps a
  minimized window mapped, so unmapping alone misses it. Bringing it back takes
  `_NET_ACTIVE_WINDOW`, because `MapWindow` does nothing to a mapped window.
- **Fullscreen and `IsMaximized`.** openbox drops the maximized state during
  fullscreen and mutter keeps it. X11 and Wayland hold the answer from outside
  fullscreen, so both read the same.
- **`Restore` must check fullscreen in each backend.** gui cannot do it: a
  minimized fullscreen window still has to come back. mutter un-maximizes behind
  a fullscreen window if asked.
- **Wayland: the ceiling comes off for fullscreen, as on X11.** The protocol
  lets a compositor "allow or disallow" fullscreen from the max size. sway,
  weston and mutter do not, but the request should not depend on that.
- **Wayland: a configure of 0×0 means the client picks.** Leaving maximized, the
  client's current size is the maximized one, so the floating size is tracked.
- **`EventResized` reaches an unfocused window.** `eventAllowed` dropped it with
  the input events. macOS drops key status during a fullscreen transition, so
  the final size of the animation was lost at random, and only the position gave
  it away.
- **macOS: `isZoomed` is not "maximized".** It compares the frame with the
  standard frame, which the content limits cap. A `FixedSize` window always
  reads as zoomed, and raising the ceiling for fullscreen flips the answer. Use
  `goguiIsZoomed`, and read it before lifting the limits.
- **macOS: `Restore` sets the saved frame, it does not call `zoom:`.** The
  system's Fill tiles a window to the screen without a zoom, and `isZoomed` then
  reads true, so `zoom:` finds no frame to return to and only nudges the window
  smaller. Logs from Fill show no `windowShouldZoom:`, and `isZoomed` going true
  about 10 points short of the final frame, mid-animation, while the frames
  before it read false. So `fsNormalFrame` keeps only a frame that stopped
  changing.
- **macOS: properties on `GUIWindow` carry an `fs` prefix.** A plain name on an
  `NSWindow` subclass can collide with a private AppKit one.
- **Web: a request made at touch-down waits for the `touchend`.** Outside a
  scrollable, gui fires `OnClick` at touch-down, and a `touchstart` grants no
  user activation, so Chrome refuses the request there ("Permissions check
  failed"). The backend holds a request made during a `touchstart` without
  activation and makes it at the next `touchend`. A `touchcancel` or a new call
  drops it. Only a `touchstart` holds a request: one held from a timer would
  fire at some later, unrelated tap.
- **Web: the page goes fullscreen, not the canvas.** The hidden IME input and
  the accessibility tree sit beside the canvas and must stay on screen.
  `IsFullscreen` follows `fullscreenchange`, where Esc and Back arrive, and the
  backend reports the state once at start, for a page that is fullscreen before
  the app runs. The browser's own fullscreen (F11) is not page fullscreen, so it
  does not count.
- **Web: a call during a transition waits for it.** The browser enters and
  leaves asynchronously, and `fullscreenElement` changes only at the end. A
  leave asked for while the entry was pending saw "not fullscreen" and did
  nothing, so the page stayed fullscreen. The backend keeps the app's last
  request and goes on toward it once the transition settles. An entry asked for
  during an exit is requested then, and the browser may refuse it for lack of a
  recent user action.
- **iOS: `IsFullscreen` is worked out, not reported.** Hiding the status bar
  never resizes the view, and a notched iPhone sends no callback, so after each
  layout pass the controller reports "asked for, and the window fills the
  screen". A windowed iPad does not fill it: `IsFullscreen` stays false there
  and turns true once the user makes the window full size again, with no new
  call.
- **iOS: `prefersStatusBarHidden` keeps UIKit's answer.** It ORs in `super`,
  which hides the status bar on an iPhone in landscape. Returning only the
  request would show the bar there.
- **iOS: an app toggles its own request, not `IsFullscreen`.** On a windowed
  iPad `IsFullscreen` stays false, so a button that toggles it asks again on
  every tap and can never take the request back; the app then goes fullscreen
  unasked once the window fills the screen. `examples/ios_demo` keeps the
  request in its state.
- **Android: the host does the work.** The backend cannot call Java. It leaves
  the request for the host, which polls `PendingFullscreen` each frame, applies
  it on the UI thread (`GoGuiFullscreen.kt` in `examples/android_demo`) and
  answers through `FullscreenChanged`. The view does not resize on Android 15
  and later, so the state has to come back that way.
- **Android: the fill settings apply only while fullscreen.** The view reaches
  into the cutout (Android 9 to 14) and under the bars (11 to 14) while
  fullscreen, and leaving undoes both. Measured on the API 30 emulator: 2063 to
  2340 pixels high and back to exactly 2063. An app that never asks for
  fullscreen keeps its layout on every version.
- **Android: leaving restores what showed before.** The host saves the bars'
  visibility on entering and shows only those again. The demo's theme hides the
  status bar, and on the Pixel 7 it stayed hidden after leaving.
- **Android below 11: apply the flags again.** The system clears the hiding
  flags when the app comes back to the front (0x1706 to 0x1700 on Android 8.1)
  and while the soft keyboard is up. The host applies them again in
  `onWindowFocusChanged` and `onSystemUiVisibilityChange`.
- **Android: a new activity starts out of fullscreen.** It reports so at once,
  so `IsFullscreen` turns false, and the app asks again if it wants to; the demo
  does so in `SurfaceCreated`. The Go window outlives the activity
  (`docs/specs/android-activity-lifecycle.md`, #1246), so the backend could
  carry the request over itself; it does not yet.
- **Android 9: a strip stays over the cutout** with a theme that keeps the
  status bar, because the content is still inset by the cutout (API 28 emulator;
  Android 10 is untested). The demo's `Theme.NoTitleBar.Fullscreen` fills it.
  Documented, not fixed: it would need `FLAG_FULLSCREEN` while fullscreen and
  the theme's own flag restored after.

## Testing

Each backend ran the same sequence: size, move, move round trip, maximize,
fullscreen, `Restore` in fullscreen, leave, restore, fullscreen from normal,
leave, minimize, restore.

| Backend | How                                                              |
| ------- | ---------------------------------------------------------------- |
| Windows | Real mouse messages into `OnClick`, on this desktop; unit tests  |
| X11     | Probe under openbox and `mutter --x11` in Xvfb; GNOME 48 by hand |
| Wayland | `scripts/wayland/test.sh`, three compositors; GNOME 48 by hand   |
| macOS   | Probe on a Mac, logs read line by line; Fill and tiling by hand  |
| Web     | Headless Chrome over CDP; Chrome on Windows and Pixel 7 by hand  |
| iOS     | `examples/ios_demo` and Safari on iPhone and iPad simulators     |
| Android | `examples/android_demo` on two phones; API 28 and 30 emulators   |

Each desktop backend ran decorated, frameless, with `MaxWidth` and with
`FixedSize`.

The web ran `examples/window_state`. Headless Chrome took trusted touch, mouse
and key input over the DevTools protocol: a tap entered at its `touchend`, while
a bare request at `touchstart` was refused. By hand, Chrome on Windows entered
by click and Enter and left by Esc, and F11 left `IsFullscreen` false. On the
Pixel 7 (Chrome, API 37) a tap entered more than 5 seconds after any other
input, so through the held request, and Back left. A page made fullscreen before
the app started read `IsFullscreen` true; the build without the start-up report
read false. Unit tests settle the browser's promises by hand to cover a call
made while entering or leaving.

iOS ran on the iOS 27 simulators. On the iPhone 18 Pro the button hid the status
bar and showed it again, and `IsFullscreen` stayed true through rotation. On the
iPad Pro, full screen gave `IsFullscreen` true with a top safe area of 0;
windowed gave false with 10 at the top and bottom, and the full-size window gave
true again without a new call. Safari on the iPad grew the page from 820×1094 to
820×1120 and left by the button; on the iPhone the button did nothing, as
expected.

Android ran `examples/android_demo`. On a Pixel 7 (API 37) entering hid the
gesture bar (safe area bottom 24 to 0) and leaving brought it back with the
theme's status bar still hidden; `IsFullscreen` stayed true through home and
back, the soft keyboard and rotation. On an Aquaris X Pro (API 27) the flags
read 0x1706 after entering, after home and back and after the keyboard, and
leaving restored 0x400. The API 28 and 30 emulators offer only GLES 2, so a
plain view measured entering and leaving there, with the demo's theme and with a
plain one.

GNOME does not cap a maximized window at `MaxWidth`, on X11 or on Wayland, which
is why `WindowCfg` promises that cap on Windows and macOS only.

## Phasing

| #   | Scope                                               | Status |
| --- | --------------------------------------------------- | ------ |
| 1   | gui core, Windows, `Show` fix                       | done   |
| 2   | X11 and Wayland                                     | done   |
| 3   | macOS                                               | done   |
| 4   | Docs and `examples/window_state`, frameless buttons | done   |
| 5   | Web fullscreen                                      | done   |
| 6   | iOS fullscreen                                      | done   |
| 7   | Android fullscreen                                  | done   |

Position lands with each desktop backend instead of in its own phase: the
interface needs all seven methods at once, so each backend is done in one pass.

Phases 5 to 7 finish #1171. The plan there came from device tests.

## Rejected Approaches

- **A 32-bit position on Windows.** A review proposed it, since desktop
  coordinates are 32-bit. Measured with the full value reaching `SetWindowPos`:
  x=40000 lands at 32767, so Windows clamps the rect to 16 bits itself.

- **One state enum** (`SetWindowMode`). It reverses the #1171 decision and
  cannot represent "fullscreen, was maximized".
- **One native setter taking an op.** Fewer stubs, but a backend's `switch` can
  miss an op and silently do nothing.
- **`IsMinimized`.** Wayland does not report minimize (`suspended` also covers
  another workspace and a locked screen), so the getter would be wrong there. A
  custom title bar does not need it: a minimized window shows no title bar.
  #1170's `IsOccluded` serves a media player.
- **A state-change event.** The redraw plus `EventResized` is enough, and an
  event can be added later without breaking anything.
- **A `WindowCfg` field for the initial state.** The replay does this.
- **Deferring in gui through `QueueCommand`.** Web's `requestFullscreen` needs
  the user gesture on the stack, so the call has to reach the backend
  synchronously. The deferral belongs to the backends that need it.
- **Monitor enumeration and always-on-top.** Nobody has asked for them.
- **The Android fill settings always on.** It would avoid the resize on Android
  9 to 14 and match 15, but it puts every app under the bars there, whether it
  pads by `SafeAreaPadding` or not, and the safe area stays opt-in.
- **`CanFullscreen`.** `IsFullscreen` already shows the outcome: it stays false
  where the OS cannot fill the screen, as on a windowed iPad or in iPhone
  Safari.
