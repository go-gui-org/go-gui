Fade a whole window, content included. `SetWindowOpacity` asks the compositor to
draw the window at a fraction of full strength, above the GL or Metal surface,
so the frame, the title bar and every widget fade together and no widget has to
know it is happening.

## Setting the Fade

```go
w.SetWindowOpacity(0.7) // 1 is opaque, 0 is invisible
```

The value is clamped to `[0, 1]`. A NaN is ignored, since it names no fade.

## Reading It Back

```go
current := w.WindowOpacity() // 1 until something sets it
```

The getter answers from a cached field rather than the window server, so a fade
animation can step from its own current position without a round trip:

```go
w.SetWindowOpacity(w.WindowOpacity() - 0.05)
```

## Before the Window Exists

Calling it in `OnInit`, or any time before the backend attaches, is safe. The
value is stored and applied at window creation, so the fade is on the first
frame the compositor ever shows — there is no opaque flash first.

```go
gui.NewWindow(gui.WindowCfg{
    OnInit: func(w *gui.Window) {
        w.SetWindowOpacity(0.85)
        w.UpdateView(rootView)
    },
})
```

## Not the Same as `Transparent`

`WindowCfg.Transparent` (see the Transparency example) is **per-pixel** alpha:
the window's own alpha channel reaches the compositor, so the desktop shows
through wherever the rendered content is not opaque. It is fixed when the window
is made, because it decides the X11 visual and the Win32 pixel format.

`SetWindowOpacity` is a **whole-window** fade applied on top of whatever the
window drew, and it can change at any time. The two compose everywhere except
Windows, where the layered-window path and per-pixel alpha do not combine — a
`Transparent` window there keeps its transparency and the fade is refused and
reported through `gui.Debug`.

## Platforms

| Platform | Fade                                    |
| -------- | --------------------------------------- |
| macOS    | Yes, composes with `Transparent`        |
| X11      | Yes, needs a compositing manager        |
| Windows  | Yes, unless the window is `Transparent` |
| Web      | Ignored                                 |
| Mobile   | Ignored                                 |

Best-effort throughout: a platform that cannot deliver the fade reports through
`gui.Debug` (category `DebugWindowDegraded`) rather than failing the call.
