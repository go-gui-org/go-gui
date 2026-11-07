# Safe area

Issue #1171. Status: **implemented** on iOS, Android and web. Desktop is always
zero.

## Decision

`Window.SafeAreaPadding() Padding` returns the space the OS keeps along each
edge of the window, in logical pixels: a notch or camera cutout, a status bar, a
home indicator, a gesture bar.

- **Opt-in.** Nothing in the framework lays itself out by it. An app that never
  reads it draws exactly as before.
- **A set `Padding`.** With no report it returns `PaddingNone`, not the unset
  zero value, so `Padding: w.SafeAreaPadding()` pads nothing on desktop instead
  of falling back to the container's default padding.
- **All four edges.** A phone in landscape reserves the left and right edges for
  its notch. The iPhone Duo reserves the right edge when open.
- **The bottom edge gives way to the soft keyboard:**
  `max(0, safe bottom - SoftKeyboardInset)`, like Flutter's `padding`. The
  keyboard already covers the home bar, so an app that pads by both counts it
  once. The rule holds whether the app pads for the keyboard itself or a later
  change (#1109) shrinks the layout area by the keyboard.
- **Backends report through
  `gui.DispatchSafeArea(w, top, right, bottom, left)`**, without the keyboard.
  Edges are clamped (negative, NaN and infinite become 0). A change marks a
  layout refresh (`refreshSafeArea`) and wakes the loop, so a view reading it
  redraws; a repeat does nothing. The value sits in an `atomic.Pointer`, because
  backends report from their own thread.

## Platforms

| Platform | Source                                                                   |
| -------- | ------------------------------------------------------------------------ |
| iOS      | `safeAreaInsets` after every layout and on `viewSafeAreaInsetsDidChange` |
| Android  | The host's window insets, through the `SafeAreaInsets` bridge call       |
| Web      | CSS `env(safe-area-inset-*)`, with `viewport-fit=cover` in the page      |
| Desktop  | Zero                                                                     |

### Android

The backend cannot call Java, so the host reports. `GoGuiGLSurfaceView` in
`examples/android_demo` reads `rootWindowInsets` on every insets dispatch and
every layout change: system bars plus the display cutout on API 30+,
`systemWindowInset*` plus the cutout's `safeInset*` below. Each edge counts only
where it overlaps the view, measured from the view's place in the window. Below
Android 15 the view may sit between the bars, from Android 15 (targetSdk 35) it
reaches under them, and fullscreen moves its edges. The same measurement fixes
the keyboard height, which used to subtract the navigation bar by guess and came
out 24 dp short on a Pixel 7 with Android 15+.

Each app re-exports bridge calls from its own Go package, so an app needs
`SafeAreaInsets` in its Go bridge and the listener in its host. A host without
them keeps working, with a zero safe area. A report that arrives before the
window exists is kept and applied by the next `Start`, on the GL thread like the
report itself.

Measured on a Pixel 7 (API 37, gesture navigation): T52 B24 in portrait (the
theme hides the status bar, the camera cutout stays), L52 or R52 in landscape.
Aquaris X Pro (API 27, no cutout, no on-screen bar): all zero.

### Web

JavaScript cannot read `env()` directly. A hidden probe element takes the four
values as its padding, and `getComputedStyle` reads them back in CSS pixels,
which are gui's logical pixels. A browser reports non-zero values only when the
page opts in with `viewport-fit=cover`; without it the browser keeps the page
clear of the notch itself, so the page decides. The probe is read at start, on
each resize and on `fullscreenchange`, and again 300 ms after the last of a
burst of them: Chrome on Android updates the values after the resize event when
the page enters fullscreen (measured: T0 at the event, T52 after).

Measured in Chrome on a Pixel 7 with `viewport-fit=cover`: B24 normally (the
gesture bar), T52 in fullscreen portrait, L52 in fullscreen landscape, B0 with
the keyboard up.

### iOS

Measured on the iPhone 18 Pro simulator: T62 B34 in portrait, L62 R62 B20 in
landscape, B0 with the keyboard up. The #1171 spike found T20 on a home-button
iPhone and T59 on the iPhone 16 Plus. Hiding the status bar changes nothing on a
notched iPhone and fires no callback. A foldable's hinge is not part of the safe
area; iOS reports it separately.

## Rejected Approaches

- **Applying the safe area automatically** (to the root, to dialogs). It would
  change every existing layout on a phone. The user decided the safe area must
  not be forced on apps (2026-11-06).
- **Adding the keyboard to the bottom edge.** `SoftKeyboardInset` already
  reports it, and #1109 decides what the layout does with it. Folding it in here
  would make the safe area change on every keypress that resizes the keyboard,
  and an app could no longer tell the two apart.
- **A `SafeAreaInsets` type of its own.** `Padding` already fits every `Cfg`
  that takes padding, and has the four edges.
- **Polling the web safe area every frame.** `getComputedStyle` forces a style
  pass. The resize, `fullscreenchange` and one delayed re-read cover every
  change measured.
- **Hinge or fold areas.** A different shape (a strip across the middle), not an
  edge inset. Add it when an app needs it.
