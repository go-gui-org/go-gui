# Safe area

Issue #1171. Status: **implemented** for iOS. Android and web report zero until
their backends supply it (steps 2 and 3 of the #1171 plan). Desktop is always
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
- **Backends report through `gui.DispatchSafeArea(w, top, right, bottom, left)`**,
  without the keyboard. Edges are clamped (negative, NaN and infinite become 0).
  A change marks a layout refresh (`refreshSafeArea`) and wakes the loop, so a
  view reading it redraws; a repeat does nothing. The value sits in an
  `atomic.Pointer`, because backends report from their own thread.

## Platforms

| Platform | Source                                                                       |
| -------- | ---------------------------------------------------------------------------- |
| iOS      | `safeAreaInsets` after every layout and on `viewSafeAreaInsetsDidChange`     |
| Android  | Zero until the host reports window insets (step 2)                           |
| Web      | Zero until CSS `env(safe-area-inset-*)` is measured (step 3)                 |
| Desktop  | Zero                                                                         |

Measured on the iOS spike for #1171: a home-button iPhone reports T20, notched
iPhones T59 to T62 and B34 in portrait, L62 R62 B20 in landscape. Hiding the
status bar changes nothing on a notched iPhone and fires no callback. A
foldable's hinge is not part of the safe area; iOS reports it separately.

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
- **Hinge or fold areas.** A different shape (a strip across the middle), not an
  edge inset. Add it when an app needs it.
