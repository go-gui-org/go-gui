# Go-Gui

![Go version](https://img.shields.io/badge/go-1.26%2B-blue)
![License](https://img.shields.io/badge/license-MIT-blue)
![CI](https://github.com/go-gui-org/go-gui/actions/workflows/ci.yml/badge.svg)
[![Ask DeepWiki](https://deepwiki.com/badge.svg)](https://deepwiki.com/go-gui-org/go-gui)
[![Wiki](https://img.shields.io/badge/docs-wiki-blue)](https://github.com/go-gui-org/go-gui/wiki)

**Cross-platform, hybrid immediate-mode GUI framework for Go — no virtual DOM,
no diffing, just fast, composable UI.**

```go
package main

import (
    "fmt"

    "github.com/go-gui-org/go-gui/gui"
    "github.com/go-gui-org/go-gui/gui/backend"
)

type App struct{ Clicks int }

func main() {
    w := gui.NewWindow(gui.WindowCfg{
        State:  &App{},
        Title:  "Counter",
        Width:  300,
        Height: 150,
        OnInit: func(w *gui.Window) { w.UpdateView(mainView) },
    })

    backend.Run(w)
}

func mainView(w *gui.Window) gui.View {
    app := gui.State[App](w)

    return gui.Column(gui.ContainerCfg{
        Content: []gui.View{
            gui.Text(gui.TextCfg{Text: fmt.Sprintf("%d Clicks", app.Clicks)}),
            gui.Button(gui.ButtonCfg{
                ID: "counter",
                Content: []gui.View{
                    gui.Text(gui.TextCfg{Text: "Click Me"}),
                },
                OnClick: func(ctx gui.EventCtx) {
                    gui.State[App](ctx.Window).Clicks++
                },
            }),
        },
    })
}
```

See [`examples/get_started/`](examples/get_started/) for the full runnable
version and [`examples/web_demo/`](examples/web_demo/) for the browser build.

---

https://go-gui.com

📜 [Documentation](https://github.com/go-gui-org/go-gui/wiki)

> **Upgrading?** Event callbacks now take a single `gui.EventCtx`, and
> click-like events are marked handled by dispatch. See
> [docs/migration-eventctx.md](docs/migration-eventctx.md).

---

## Try It

| Platform       | Download                                                                                        |
| -------------- | ----------------------------------------------------------------------------------------------- |
| Browser (WASM) | [**Open Showcase**](https://go-gui-org.github.io/showcase/) — zero install, instant evaluation  |
| macOS          | [Go-Gui-Showcase-\<version\>.dmg](https://github.com/go-gui-org/go-gui/releases)                |
| Linux          | [go-gui-showcase-\<version\>-linux-amd64.tar.gz](https://github.com/go-gui-org/go-gui/releases) |
| Windows        | [go-gui-showcase-\<version\>-windows-amd64.zip](https://github.com/go-gui-org/go-gui/releases)  |

![showcase](assets/showcase.png)

_Showcase contains the framework documentation. Every widget demo has a button
in the upper-right corner that displays documentation about the widget._

Sibling projects:

- **go-charts**\
  Interactive chart widgets. https://github.com/go-gui-org/go-charts

- **go-edit**\
  Code editor widget. https://github.com/go-gui-org/go-edit

- **go-kite**\
  Desktop Bluesky client. https://github.com/go-gui-org/go-kite

- **go-map**\
  SMIL map widgets. https://github.com/go-gui-org/go-map

- **go-term**\
  Embeddable terminal emulator. https://github.com/go-gui-org/go-term

- **go-glyph**\
  Text rendering engine on steroids. https://github.com/go-gui-org/go-glyph

## Why

GUI frameworks in Go target the browser and tie you to HTML/CSS and JavaScript.
go-gui takes the opposite approach: write your UI in pure Go, render it with
native GPU acceleration — no browser runtime, no JavaScript bridge, no DOM. Your
data stays in Go structs; your UI stays in Go code.

The second thesis: a GUI toolkit should be an **ecosystem of composable
libraries**, not a monolith. go-glyph handles text. go-charts handles data.
go-edit handles code. Each library is usable on its own or together — all
sharing the same rendering pipeline and event system.

## Features

- **50+ widgets** — buttons, inputs, sliders, tables, trees, tabs, menus,
  dialogs, toasts, DataGrid with virtualization (CSV/XLSX/PDF export), Markdown
  and RTF views, SVG rendering, and more
- **GPU-accelerated** — Metal (macOS), OpenGL (Linux/Windows), WebGL/WASM
  (browser), Metal/UIKit (iOS)
- **Animation subsystem** — keyframe, spring, tween, hero transitions, color
  filters, box shadows, blur effects
- **Touch gesture recognition** — tap, double-tap, long-press, pan, swipe,
  pinch, rotate with automatic mouse-event synthesis
- **Time-travel debugging** — opt-in scrubber rewinds/replays app state
  frame-by-frame; implement `Snapshotter` on your state type and set
  `DebugTimeTravel: true`
- **Headless testing** — all layout and widget logic runs without a display
- **Cross-platform integration** — native file dialogs, menus, notifications,
  print/PDF, system tray, IME, a11y, spell check
- **go-glyph powered** — professional text shaping, rendering, bidirectional
  layout

![gallery](assets/gallery.png)

---

## Installation

Requires **Go 1.26+**. A **C toolchain** (CGo) is needed only on **macOS** — the
Metal backend is Objective-C. Linux and Windows build fully cgo-free
(`CGO_ENABLED=0 go build ./...`). The desktop backends are native: Metal on
macOS, X11 + EGL on Linux, Win32 + WGL on Windows. Text shaping and
rasterization are pure Go via go-glyph.

```bash
go get github.com/go-gui-org/go-gui
```

See the
[Installation Guide](https://github.com/go-gui-org/go-gui/wiki/Installation) for
platform-specific instructions.

![todo example](assets/todo.png)

---

## Debugging

A few widget mistakes are silent by construction, because they produce no error
and no visual difference:

- Two widgets sharing an `ID`. `ID` is the identity key for focus, scroll
  offsets, and per-widget state, so the two collapse onto one identity.
- A focusable widget with no `ID`. It renders and it clicks, but focus traversal
  is keyed by `ID`, so it never joins the tab order.
- A scrollable widget with no `ID`. Every ID-less scrollable in a window shares
  the key `""`, so they scroll in lockstep.
- An `OnMouseLeave` on a widget with no `ID`. Leave tracking is keyed by `ID`,
  so the callback never fires. This one survives `FocusDisabled: true` — opting
  out of focus does not opt out of needing an identity.

`gui.Debug(true)` — or `GOGUI_DEBUG=1` in the environment — audits every frame
for these and writes findings to stderr, once per finding per window:

```go
gui.Debug(true)
// gui: focusable shape at 0/2/1 has no ID; focus traversal is keyed by
// ID, so it renders and clicks but never joins the tab order
```

Leave it off in production: the checks walk the whole layout tree each frame and
allocate while doing it.

For the mistakes that are visible in the source, `requiredid` reports them at
build time instead, naming the `Cfg` type:

```fish
go run github.com/go-gui-org/go-gui/tools/requiredid/cmd/requiredid ./...
```

`go vet -vettool=` and a golangci-lint custom plugin work equally well. The tool
is offered, not required — it is an internal tool whose rules may tighten
between releases, so nothing breaks if you never run it. Without it, a widget
that needs an `ID` and has none panics on its first render rather than failing
your build.

### Widgets that require an `ID`

Every input control panics when constructed without a non-empty `Cfg.ID`:
`Button`, `Input`, `InputDate`, `NumericInput`, `RadioButtonGroup`, `Radio`,
`Select`, `Switch`, `Toggle`, plus the stateful widgets that already did
(`ColorPicker`, `Combobox`, `DatePicker`, `ListBox`, `Slider`, `Tree`, `Table`,
`Form`, `Menu`, `Menubar`, `ContextMenu`, `CommandPalette`, `ProgressBar`,
`DataGrid`).

The `ID` is what focus traversal, per-widget input state, scroll offsets, and
`OnMouseLeave` dispatch are all keyed by, so a control without one is not merely
anonymous — it is unreachable by keyboard and shares state with every other
ID-less control. IDs must be unique within a window.

A decorative control that should never take focus opts out instead of inventing
an ID:

```go
gui.Button(gui.ButtonCfg{FocusDisabled: true, Disabled: true})
```

`FocusDisabled: true` satisfies the requirement for the widgets above. It does
not exempt a widget whose `ID` is tagged `gui:"required"` without the `focus`
option — those key state by `ID` regardless of focus.

---

## Styling widgets

A widget that should keep one appearance through hover, press and focus used to
mean assigning six `Color*` fields. `ColorSet` groups them, and `Flat` covers
that case in a line:

```go
gui.Button(gui.ButtonCfg{
    ID:     "add-todo",
    Colors: gui.Flat(colorAccent),
})
```

`ColorSet` has six fields — `Base`, `Hover`, `Click`, `Focus`, `Border`,
`BorderFocus`. `Base` backs the three interactive states when they are unset, so
`ColorSet{Base: c}` gives a widget that does not react to the pointer while
keeping its themed border. `Flat(c)` additionally pins both borders, which is
what makes it visually inert rather than merely uniform.

Anything left unset falls through to the theme, and an unassigned `ColorSet`
changes nothing.

Two rules worth knowing:

- **An assigned flat `Color*` field wins over the `ColorSet`.** The direction is
  deliberate: code that sets flat fields today must keep its current appearance
  when a `ColorSet` arrives from a preset or a half-finished edit.
- **Colors are plain `Color`, not `Opt[Color]`.** `Color` already tracks whether
  it was set, so `gui.ColorTransparent` is a real choice and `Color{}` means
  "unspecified".

`ColorSet` is on `ButtonCfg` today; other widgets still take the flat fields.

---

## Testing your app

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

---

## Contributing

1. Install **Go 1.26+** (a C toolchain too if developing on macOS, see
   [Installation](#installation)).
2. Clone the repo.
3. Run tests and lint:

```bash
go test ./...
go vet ./...
golangci-lint run ./...
```

4. Open a pull request with a clear description of the change.

---

![Digital Rain Screenshot](assets/digital-rain.png)

## Roadmap

Planning lives in [GitHub Issues](../../issues) and the go-gui-org project
board, not a checked-in roadmap file. Browse open issues for current and planned
work.

## License

[MIT](LICENSE)
