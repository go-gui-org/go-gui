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
                OnClick: func(_ *gui.Layout, e *gui.Event, w *gui.Window) {
                    gui.State[App](w).Clicks++
                    e.IsHandled = true
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

GUI frameworks in Go target the browser and tie you to HTML/CSS and
JavaScript. go-gui takes the opposite approach: write your UI in pure Go,
render it with native GPU acceleration — no browser runtime, no JavaScript
bridge, no DOM. Your data stays in Go structs; your UI stays in Go code.

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

Requires **Go 1.26+** and a **C toolchain** (CGo). The desktop backends are
native: Metal on macOS, X11 + EGL on Linux, Win32 + WGL on Windows. Text
shaping and rasterization are pure Go via go-glyph.

```bash
go get github.com/go-gui-org/go-gui
```

See the [Installation Guide](https://github.com/go-gui-org/go-gui/wiki/Installation)
for platform-specific instructions.

![todo example](assets/todo.png)

---

## Contributing

1. Install **Go 1.26+** and a C toolchain (see
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
board, not a checked-in roadmap file. Browse open issues for current and
planned work.

## License

[MIT](LICENSE)
