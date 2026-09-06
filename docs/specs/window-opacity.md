# Whole-window opacity

Status: implemented. Issue #516. Verified on macOS only; the Windows and X11
paths are unverified on hardware.

## Problem

`WindowCfg.Transparent` (#515, see `transparent-windows.md`) controls
**per-pixel** alpha: the window's alpha channel reaches the compositor, so the
desktop shows through wherever the rendered content is not opaque. How
see-through a pixel is comes from `BgColor` and from each widget's own colors.

That is not the "make this overlay 70% visible" knob. A whole-window fade
happens in the compositor, _above_ the GL or Metal surface, and it fades the
content too. Every desktop platform has one, and it composes on top of per-pixel
transparency rather than replacing it.

## Design

A runtime setter, not a `WindowCfg` field:

```go
func (w *Window) SetWindowOpacity(opacity float32)
func (w *Window) WindowOpacity() float32
```

`Transparent` has to be a creation-time field because it fixes the X11 visual
and the Win32 pixel format. Opacity fixes nothing, so it can change after
creation — which is the point: fade-in on show, dim on focus loss.

`opacity` is clamped to [0, 1]. A NaN is ignored rather than clamped: it names
no fade, so the current value stands.

Best-effort, the `Transparent` rule: a platform that cannot deliver the fade
reports through `gui.Debug` and leaves the window alone. Nothing errors.

| Platform          | Mechanism                                                 |
| ----------------- | --------------------------------------------------------- |
| macOS             | `NSWindow.alphaValue`                                     |
| Windows           | `WS_EX_LAYERED` + `SetLayeredWindowAttributes(LWA_ALPHA)` |
| X11               | `_NET_WM_WINDOW_OPACITY`; needs a compositing manager     |
| web, iOS, Android | Ignored                                                   |

### Why a getter

`SetWindowVibrancy` has no getter, and opacity does. A fade is a value an app
moves in steps — an animation reads its own current position back each frame —
where a vibrancy material is set once. `WindowOpacity` answers from a cached
field rather than the platform, so reading it costs no round trip to the window
server.

### The cached value, and replay at creation

`Window.windowOpacity` is seeded to 1 by `NewWindow`; the zero value would read
as an invisible window.

The setter is reachable before a backend attaches — in `OnInit`, or before
`backend.Run`, where `nativePlatform` is still nil. Rather than making such a
call a silent no-op, each backend re-applies `w.WindowOpacity()` at window
creation when it is below 1, in the same place #515 applies `Transparent`:
before the first frame on macOS, after `CreateWindowExW` on Windows, after
`MapWindow` on X11. So the call takes effect and no opaque frame is composited
first.

### Windows: the guard

#515 deliberately avoids `WS_EX_LAYERED`, because it composites through a
redirection surface that fights the GL swap chain. `SetLayeredWindowAttributes`
requires that same style bit, so the two features may not coexist.

Whether they actually conflict could not be tested — no Windows hardware was
available. The honest shape is therefore a guard rather than a guess:

```go
func layeredOpacityAllowed(transparent bool) bool { return !transparent }
```

A `Transparent` window that calls `SetWindowOpacity` is refused and told why
through `gui.Debug`. An ordinary window takes the fade. The alternatives were
worse: applying the style bit anyway risks a window that presents nothing, and
an offscreen composite path is a large amount of machinery for one platform.

The extended style bit is set on every call, which is idempotent, but it is put
back if `SetLayeredWindowAttributes` then fails: a layered window whose alpha
was never set is not painted at all, so leaving the bit on would turn a refused
fade into an invisible window.

**Still open:** whether `WS_EX_LAYERED` presents GL content at all on a WGL
window, and whether it survives `DwmEnableBlurBehindWindow` being active. If the
first is false, the guard is not enough and Windows drops to a reported no-op.

### X11

`_NET_WM_WINDOW_OPACITY` is a single 32-bit CARDINAL, 0 invisible and
`0xFFFFFFFF` opaque. At full opacity the property is **deleted** rather than
written as `0xFFFFFFFF`, which puts the window back on the compositor's
untouched path instead of asking for a 100% fade.

It is a window-manager hint, not a visual, so it is independent of `Transparent`
and needs no guard. Without a compositing manager nothing reads it, which is the
same silent failure `Transparent` has and is reported the same way.

The setter is on the path of a fade animation, so it is kept to one blocking
round trip per call. The atom is interned once and cached on `platformState`,
and the compositor check — two more round trips, feeding a dev-mode message only
— runs behind `gui.DebugEnabled()`.

### Degrading

Through the existing `gui.DebugWindowDegraded` category (in `DebugAll`), via a
new `(*Window).DebugWindowOpacity(reason)`. `reason` is the warn-once
discriminator, so each distinct cause reports once. Causes:

- Windows: the window is `Transparent`; a missing `user32` export; a failed
  `SetLayeredWindowAttributes`.
- X11: the server would not intern the property; no compositing manager.

## Not done

- **Wayland**, for the same reason as #515: the GL backend is X11 only.
- **Click-through.** A faded window still takes mouse input over every pixel.
  That is a separate feature (an input region / `WS_EX_TRANSPARENT`).
- **Animation helpers.** Fading over time is the app's loop to write;
  `WindowOpacity` gives it the value to step from.
