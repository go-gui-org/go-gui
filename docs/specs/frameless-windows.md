# Frameless windows

Status: implemented. Issue #473.

## Problem

`gui.WindowCfg` could not drop the OS window decorations. Custom title bars,
overlay tools and borderless desktop apps were impossible. The window style was
hardcoded in each backend: the macOS style mask in `metalWindowCreate`, the
Win32 style in `platform_win32.go`, and X11 set no decoration hint at all.

A second gap made a flag on its own useless. No backend had window drag-to-move
or client-driven resize. An undecorated window has nothing the user can grab, so
a `Borderless: true` flag would ship a window that cannot be moved.

## Design

One config field selects the frame, and two `Window` methods hand a gesture to
the OS.

```go
w := gui.NewWindow(gui.WindowCfg{
    Title:       "App",
    Width:       800,
    Height:      600,
    Decorations: gui.DecorationNone,
})
```

`WindowDecoration` has three values. The zero value is `DecorationDefault`, so
every existing caller keeps the standard frame.

| Value                      | macOS                                         | Windows      | X11         |
| -------------------------- | --------------------------------------------- | ------------ | ----------- |
| `DecorationDefault`        | standard frame                                | standard     | standard    |
| `DecorationHiddenTitlebar` | full-size content view, window controls float | degrades     | degrades    |
| `DecorationNone`           | borderless, edge resize kept                  | popup + grip | Motif hints |

`DecorationHiddenTitlebar` has no equivalent outside macOS, so it degrades to
`DecorationDefault` there. This follows the vibrancy precedent: a macOS-only
option is documented as a no-op elsewhere, not an error.

### Gestures

```go
func (w *Window) StartWindowDrag()
func (w *Window) StartWindowResize(edge WindowEdge)
```

Call either from an `OnMouseDown` handler. Both give the gesture to the OS,
which then owns the pointer until the button comes up. The app never sees the
matching release, so it holds no drag state of its own.

Giving the gesture to the OS is what keeps window snapping and edge tiling
working. A client-side `setFrameOrigin` loop would fight the window manager.

`WindowEdge` values are the `_NET_WM_MOVERESIZE` direction codes, so X11 passes
them through unmapped.

## Platform implementation

**macOS.** `metalWindowCreate` takes a `decorations` parameter. `DecorationNone`
uses `NSWindowStyleMaskBorderless`, and keeps `NSWindowStyleMaskResizable` so
AppKit still resizes the window from its edges — which is why
`StartWindowResize` is a no-op on macOS and the app needs no grip.
`DecorationHiddenTitlebar` adds `NSWindowStyleMaskFullSizeContentView`, sets
`titlebarAppearsTransparent` and hides the title.

`storeEvent` retains the last mouse-down `NSEvent`, because
`performWindowDragWithEvent:` wants the press that started the gesture and Go
only receives the flattened coordinates.

**Windows.** A frameless window is
`WS_POPUP | WS_THICKFRAME | WS_MINIMIZEBOX | WS_MAXIMIZEBOX`. `WS_THICKFRAME`
stays because it is what gives the window OS resize, Aero snap and the drop
shadow; the caption strip it would reserve is removed by returning 0 from
`WM_NCCALCSIZE`.

Zeroing the non-client area also removes the inset Windows applies when
maximizing, so `WM_GETMINMAXINFO` clamps the maximized rect to the monitor work
area. Without the clamp a maximized frameless window overhangs every screen edge
and covers the taskbar.

`StartWindowDrag` calls `ReleaseCapture` then
`SendMessage(WM_SYSCOMMAND, SC_MOVE|HTCAPTION)`; `StartWindowResize` sends
`SC_SIZE` with the matching `WMSZ_*` direction.

**X11.** X11 has no decoration property of its own. Every mainstream window
manager still reads the Motif hints CDE left behind, so `_MOTIF_WM_HINTS` with
flags 2 and decorations 0 is how a client asks for a bare frame. The property
has to be set before `MapWindow`, because the window manager reads it when it
reparents.

Gestures send `_NET_WM_MOVERESIZE` to the root window. The button press left an
implicit pointer grab on the app, so the grab is released first or the window
manager never sees the drag. The backend records the press position in root
coordinates, which the Go-side event no longer carries.

## Not included

- `FixedSize` is still ignored on X11 (no `WM_NORMAL_HINTS`). A pre-existing
  gap, left alone to keep this change scoped.
- No `gui.TitleBar` widget. `examples/frameless` builds its header from a plain
  `Row` and a `Button`.
- Web, software, iOS and Android backends are no-ops.

## Files

- `gui/window_decoration.go` — `WindowDecoration`, `WindowEdge`, the two
  `Window` methods.
- `gui/window_cfg.go` — the `Decorations` field.
- `gui/native_platform.go` — `StartWindowDrag`, `StartWindowResize`.
- `gui/backend/metal/metal_window_darwin.m`, `metal_window.h`, `backend.go`.
- `gui/backend/gl/decoration_win32.go`, `decoration_x11.go`, and the create and
  event paths in `platform_win32.go`, `platform_x11.go`, `events_win32.go`,
  `events_x11.go`.
- `examples/frameless/`.
