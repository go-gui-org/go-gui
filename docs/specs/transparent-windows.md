# Transparent windows

Status: implemented. Issue #515.

## Problem

Discussion #514 asked how to make a transparent window on Windows and Linux. The
only translucency go-gui had was `Window.SetWindowVibrancy`, which is an
`NSVisualEffectView` and macOS only, so the answer looked like "not possible".

That conflated two features:

- **Vibrancy** is a _blurred_ backdrop. macOS only. Windows has acrylic, but
  only through an undocumented API; on Linux blur belongs to the window manager,
  not to X11.
- **Per-pixel window transparency** lets the window's alpha channel reach the
  compositor, so the desktop behind shows through unblurred. Every desktop
  platform supports it.

The GL backend was already most of the way to the second one. EGL asked for
`EGL_ALPHA_SIZE 8`, the X11 path already built a per-visual colormap and set
`CwBorderPixel`, WGL already asked for `cAlphaBits = 8`, and the frame was
already cleared with `BgColor.A`. Only the window-creation step was missing.

## Design

One creation-time field:

```go
w := gui.NewWindow(gui.WindowCfg{
    Title:       "overlay",
    Transparent: true,
})
```

Plain `bool`, zero value opaque, the same shape and the same "degrades, never
errors" rule as `Decorations` (see `frameless-windows.md`).

Creation-time only, with no runtime setter: the X11 visual and the Win32 pixel
format are fixed when the window is made. `SetWindowVibrancy` stays a separate
macOS-only blur API, and the two compose — a vibrant window is already
non-opaque.

| Platform          | Mechanism                                              |
| ----------------- | ------------------------------------------------------ |
| macOS             | `NSWindow.opaque = NO` and a non-opaque `CAMetalLayer` |
| Windows           | `DwmEnableBlurBehindWindow` with an empty blur region  |
| X11               | Depth-32 ARGB visual; needs a compositing manager      |
| web, iOS, Android | Ignored                                                |

### The background fallback

An unset `BgColor` took the theme background, which is opaque, so
`Transparent: true` on its own would have opened a window that looked broken.
The fallback now reads the flag: a transparent window with no `BgColor` clears
to `ColorTransparent`.

The five backends each held their own copy of that fallback. They now call one
`(*Window).FrameBackground`, so the rule cannot drift between them.

### Why the flag is not inferred from the alpha

How see-through a window is comes from `BgColor`'s alpha, so a flag that only
says "yes, really" looks redundant. It is not, for two reasons.

`Transparent` asks the platform for a capability and can be refused — no
compositing manager, no depth-32 visual, no `dwmapi` export. `BgColor` is a
color and cannot fail. The refusal has to be reported (see Degrading), and a
diagnostic can name a flag the app set; it cannot name "an alpha of 254
somewhere".

Their lifetimes also differ. `BgColor` is read every frame, so an app may assign
it at runtime. `Transparent` fixes the X11 visual and the Win32 pixel format at
window creation and can never change. Inferring one from the other would give a
field that works when set before `NewWindow` and silently does nothing after —
one value, one field, two behaviours depending on when it was touched.

A third, smaller point: `BgColor.A < 255` on an ordinary window is a silent
no-op today, because the compositor discards the alpha. Making it load-bearing
would change what existing code means.

### X11 visual selection

`eglChooseConfig` returns configs in the driver's preference order, and drivers
put the depth-24 ones first. Taking the first config, which is what the backend
did, can never produce a transparent window.

`eglInitDisplayN` now returns every matching config paired with its
`EGL_NATIVE_VISUAL_ID`, and `pickVisual` chooses among them against the X
screen's own visual list: the first depth-32 visual when the window is
transparent, the driver's first choice otherwise. `pickVisual` is pure, so it is
tested with no X server.

The window creation code below it did not change. A per-visual colormap and
`CwBorderPixel` were already there, and both are required for a depth-32 window
(a depth-32 window inheriting the root's depth-24 border pixmap is a
`BadMatch`).

### Windows

`DwmEnableBlurBehindWindow` with `DWM_BB_ENABLE | DWM_BB_BLURREGION` and an
_empty_ region. The empty region blurs nothing, but enabling blur-behind at all
is what takes the window off the opaque composition path, so the result is plain
per-pixel alpha with no blur.

`WS_EX_LAYERED` is deliberately not used: it composites through a redirection
surface that fights the GL swap chain.

The call runs after `CreateWindowExW` and before the pixel format is set, so DWM
is already off the opaque path when the GL surface is bound.

### Degrading

Two things can silently defeat a transparent X11 window: a driver that offers no
depth-32 visual, and no compositing manager running. The second renders the
window black, which is worse than opaque and gives the app author no clue.

Both are reported through a new `gui.DebugWindowDegraded` category, in
`DebugAll`, via `(*Window).DebugWindowTransparency`. Windows reports the same
way when the DWM call is unavailable or fails. Nothing fails window creation.

## Not done

- **Wayland.** The GL backend is X11 only; Wayland users reach it through
  XWayland, where this works the same way. A native Wayland backend would use an
  ARGB8888 surface format instead.
- **Blur on Windows and Linux.** Windows acrylic needs the undocumented
  `SetWindowCompositionAttribute`; Linux blur is per-window-manager (KDE and
  some wlroots compositors). Neither is a portable API, so vibrancy stays
  macOS-only.
- **Click-through.** A fully transparent region still takes mouse input. That is
  a separate feature (an input region / `WS_EX_TRANSPARENT`).
- **A whole-window fade.** Per-pixel alpha cannot dim the content itself. That
  is `Window.SetWindowOpacity` (#516), a compositor-level knob that composes on
  top of this one; see `window-opacity.md`.
