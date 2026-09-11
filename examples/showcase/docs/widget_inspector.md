Built-in dev-mode overlay that shows the live layout tree and the properties of
any node in it. Not a widget: every window has it, and no code turns it on.
Press **F12** in a running app.

The inspector reads the arranged tree, so what it shows is post-sizing geometry
— the numbers the renderer used, not the ones the Cfg asked for.

## Usage

Nothing to add to a view. The key hook lives on the window (`inspectorKeyHook`,
`gui/window_event.go:75`), so an ordinary app already has it:

```go
func main() {
    w := gui.SimpleWindow("My App", 800, 600, &App{}, func(w *gui.Window) {
        w.SetView(mainView)
    })
    backend.Run(w) // press F12 in the running window
}
```

## Keyboard Shortcuts

| Key       | Action                           |
| --------- | -------------------------------- |
| F12       | Toggle the inspector             |
| Alt+Left  | Widen the panel by 50 px         |
| Alt+Right | Narrow the panel by 50 px        |
| Alt+Up    | Move the panel to the other side |

The panel never goes below 300 px wide, and never past 80% of the window
(`inspectorResize`, `gui/inspector.go:82`).

## What the Panel Shows

| Part                | Contents                                                                                                                     |
| ------------------- | ---------------------------------------------------------------------------------------------------------------------------- |
| Layout tree         | Every node, with its shape type, size, and ID                                                                                |
| Property detail     | Position, size, sizing, padding, spacing, color, radius, focus, scroll, alignment, float, clip, opacity, events, child count |
| Wireframe highlight | The picked node outlined in cyan, its padding area in green                                                                  |

## Production Builds

The whole inspector compiles out under `-tags prod`: `inspectorSupported` is a
build-tagged constant (`gui/inspector_support_prod.go`), and every call site
guards on it, so the dead code goes with it.

```sh
go build -tags prod ./...
```

## Styling

The panel's colors come from the window's theme, not from a Cfg. `ThemeMaker`
fills each theme's inspector palette from `defaultInspectorStyle`
(`gui/theme_maker.go:666`), so a custom theme gets a matching inspector with no
extra work.

## Related

The inspector answers "what does this frame look like". For "what is wrong with
this frame", turn on `gui.Debug(true)` or `GOGUI_DEBUG=1`, which reports
duplicate IDs, unresolved state keys, missing IDs on focusable shapes, and
unconsumed events to stderr.
