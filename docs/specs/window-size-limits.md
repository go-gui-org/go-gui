# Window size limits

Status: implemented. Issue #494.

## Problem

`gui.WindowCfg` could set a starting size but not a floor. A user could drag a
window below the size its content needs, and the app had no way to stop it.

An app-level fix is not possible. The resize drag is serviced by the OS window
server (macOS), the window procedure (Windows) or the window manager (X11)
before the app is told anything. By the time an `EventResized` arrives, the new
size is already applied. An app that resized itself back would fight the drag
and flicker. The limit must be declared to the platform, which is what all three
platforms already offer.

## Solution

Four fields on `WindowCfg`, in the same logical pixels as `Width`/`Height`:

```go
gui.NewWindow(gui.WindowCfg{
    Width: 800, Height: 600,
    MinWidth: 400, MinHeight: 300,
    MaxWidth: 1600, MaxHeight: 1200,
})
```

Zero on a field means that bound is unset. The limits are applied once, at
window creation. There is no runtime setter — a window whose usable size changes
while it runs is a different feature, and no caller has asked for it.

## Normalization

`gui/window_size_limits.go` holds the whole policy, so the three native backends
cannot disagree about the edge cases. `WindowSizeLimits(cfg)` returns a
`SizeLimits`, applying these rules in order:

1. `FixedSize` collapses both bounds onto `Width`/`Height`, so a fixed window is
   expressed as minimum == maximum. A `FixedSize` window with no usable
   `Width`/`Height` falls through to the ordinary rules rather than pinning the
   window to zero.
2. A negative value reads as unset. It is a caller mistake, not a smaller
   window, and must not reach a platform call.
3. A ceiling below its floor is contradictory. The floor is the stronger promise
   — content stops fitting below it — so the ceiling is raised to meet it rather
   than the floor being dropped.
4. Every bound is capped at 32767 pixels, the largest extent an X11 drawable
   coordinate can carry and a comfortable guard for the `int32` Win32 track
   sizes. No display comes near it, so a caller asking for more gets the ceiling
   instead of a value that wraps negative inside a backend.

`SizeLimits.None()` lets a backend skip the platform call entirely, leaving the
window's platform defaults untouched rather than writing a no-op limit.

## Units

`WindowCfg` speaks logical pixels. What each platform wants differs, so the
conversion is the backend's, not the caller's:

| Platform | Unit wanted   | Conversion                          |
| -------- | ------------- | ----------------------------------- |
| macOS    | points        | none — Cocoa content size is points |
| Windows  | device pixels | `SizeLimits.Scaled(dpi/96)`         |
| X11      | device pixels | `SizeLimits.Scaled(dpiScale)`       |

`Scaled` is the only place that conversion lives, so Windows and X11 cannot
resolve the same `MinWidth` to different client sizes. It keeps an unset field
at zero, so "unset" survives scaling instead of becoming a zero-pixel bound;
raises a sub-pixel result to 1 so a small floor is not scaled away entirely; and
re-applies the 32767 cap so a high scale cannot carry a bound out of range.

It truncates rather than rounds, matching how both backends scale `Width` and
`Height`. Rounding would let a floor equal to the created size land a pixel
above it, and the window would grow by that pixel as it opened — most visibly
under `FixedSize`, which is expressed as a floor at exactly the created size.

## Per-platform mechanism

**macOS** (`gui/backend/metal/metal_window_darwin.m`). A new
`metalWindowSetSizeLimits` calls `setContentMinSize:` and `setContentMaxSize:`,
called from `createWindowState` just after `metalWindowCreate`.
`metalWindowCreate`'s signature is unchanged — the limits are a separate setter,
so the create call does not grow four more parameters. AppKit has no "no
maximum" sentinel; `CGFLOAT_MAX` is the idiom and is what an unconstrained
window already carries.

**Windows** (`gui/backend/gl/decoration_win32.go`). `WM_GETMINMAXINFO` was
already handled, but only for a frameless window's maximize rect. The frameless
gate is gone: two independent concerns write different `MINMAXINFO` fields, and
both may apply to one window, so neither short-circuits the other.
`applySizeLimits` fills `ptMinTrackSize`/`ptMaxTrackSize`, writing only the
constrained axes so an unset axis keeps the default Windows supplied.

Track size is the outer frame while `WindowCfg` speaks client area, so
`trackSizeFor` adds the frame overhead via the same `AdjustWindowRectExForDpi`
call `New` applies to `Width`/`Height`. Without it a `MinWidth` would mean a
smaller client area on Windows than elsewhere. The result is computed at create
time and stored on `platformState`; the message handler does no conversion and
no allocation per message.

**X11** (`gui/backend/gl/platform_x11.go`). `setSizeHints` writes
`WM_NORMAL_HINTS` with the `PMinSize` and `PMaxSize` flags of `XSizeHints`. The
property is written whole (18 words) because window managers read it at fixed
offsets. A bound is only honored when both its axes carry a value, so a one-axis
limit fills the other axis with a permissive value rather than a zero the WM
would read as "0 pixels".

The X11 backend rescales on a monitor-to-monitor move, so `dpi_x11.go` rewrites
the hints at the new scale. `platformState.limits` keeps the logical values for
that, since the physical ones are only correct for one scale.

Web, iOS and Android ignore the limits. The web backend fills its viewport and
already ignores `Width`/`Height`; iOS and Android are OS-sized.

## FixedSize on X11

`WindowCfg.FixedSize` was a silent no-op on X11 before this change — only macOS
(a style mask bit) and Windows (a window style) honored it. X11 has no resizable
style bit; `WM_NORMAL_HINTS` is the only way to constrain a drag, which is
exactly the property this change introduces. Rule 1 of the normalization above
reports a fixed window as minimum == maximum, so the X11 backend gets the fix
without a second code path.

This is a behavior change for Linux apps that set `FixedSize`: their windows now
actually stop resizing.

## Testing

`windowSizeLimits`, `None` and `Scaled` carry the policy and need no window, so
they are unit tested directly in `gui/window_size_limits_test.go` — unset,
negative, max-below-min, the `FixedSize` collapse, and the fall-through when a
`FixedSize` window has no dimensions.

`trackSizeFor` and `applySizeLimits` are testable without a window too, driven
against a synthetic `MINMAXINFO` in `decoration_win32_test.go`, matching how
`windowStyleFor` is already tested.

No golden test applies. Nothing about the rendered widget tree changes; the
limits never reach the layout pipeline.

Enforcement itself is a platform behavior and is verified by hand: drag a
window's corner inward and confirm it stops, per platform.

## See also

- `docs/specs/frameless-windows.md` — owns `FixedSize` and the Win32
  `WM_GETMINMAXINFO` handler this change extends.
