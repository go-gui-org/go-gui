# Testing your app

`NewTestWindow` builds a window with no backend, no platform, and no run loop,
and the `Test*` methods drive it the way a user would. Widgets are addressed by
`ID`.

```go
func TestIncrement(t *testing.T) {
    w := gui.NewTestWindow(gui.WindowCfg{State: &App{}})
    w.TestRender(view)

    if err := w.TestClick("increment"); err != nil {
        t.Fatal(err)
    }
    if got := gui.State[App](w).Count; got != 1 {
        t.Fatalf("Count = %d, want 1", got)
    }
}
```

| Method                   | Does                                                |
| ------------------------ | --------------------------------------------------- |
| `TestRender(view)`       | Runs one frame; returns the root `*Layout`          |
| `TestClick(id)`          | Left press + release at the widget's visible center |
| `TestFocus(id)`          | Moves keyboard focus                                |
| `TestKey(id, key, mods)` | Focuses, then presses and releases a key            |
| `TestType(id, text)`     | Focuses, then types the text one rune at a time     |
| `TestTab(dir)`           | Tab / Shift-Tab; returns the newly focused `ID`     |
| `TestScroll(id, dx, dy)` | Scrolls over the widget                             |
| `TestScrollOffset(id)`   | Reads a scroll container's current offset           |

Each method synthesizes a real `Event` and pushes it through the same dispatch
the backend uses, then settles a frame. That is deliberate: it means a test sees
the widget never joining the tab order, an overlay swallowing the click, or a
container clamping the scroll — the failures that only exist between the widget
and the event system. It also means the tree is rebuilt after every action, so a
`*Layout` captured before one must not be read after it; call `TestRender(nil)`
to get the current tree.

Failures are returned as errors, not panics — an unknown `ID`, a disabled
widget, a widget with no matching handler, or an event nothing handled. Match
them with `errors.Is` against `gui.ErrTestNoSuchID` and friends.

Two behaviors worth knowing before writing assertions:

- **A nil return from `TestClick` does not prove the target fired.** Dispatch
  does not report which shape it delivered to, so a click absorbed by an overlay
  is indistinguishable from one the target handled. Assert on the state the
  click was supposed to change.
- **`TestScroll` sends a precise/trackpad scroll, not a wheel notch.** The
  discrete-wheel path eases the offset over later frames via the animation
  goroutine, which a headless test does not run. The precise path writes the
  offset synchronously, which is the only thing a test can assert on.

Injected interfaces (`TextMeasurer`, `SvgParser`, `NativePlatform`) stay nil, so
text extents are approximations. Assert on structure, state and focus — not on
pixel widths.
