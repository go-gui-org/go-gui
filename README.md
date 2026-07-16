# Go-Gui

![Go version](https://img.shields.io/badge/go-1.26%2B-blue)
![License](https://img.shields.io/badge/license-MIT-blue)
![CI](https://github.com/go-gui-org/go-gui/actions/workflows/ci.yml/badge.svg)
[![Ask DeepWiki](https://deepwiki.com/badge.svg)](https://deepwiki.com/go-gui-org/go-gui)
[![Wiki](https://img.shields.io/badge/docs-wiki-blue)](https://github.com/go-gui-org/go-gui/wiki)

**Cross-platform, hybrid immediate-mode GUI framework for Go — no virtual DOM,
no diffing, just fast, composable UI.**

https://go-gui.com

![showcase](assets/showcase.png)

_Showcase contains the framework documentation. Every widget demo has a button
in the upper-right corner that displays documentation about the widget._

📜 [Documentation](https://github.com/go-gui-org/go-gui/wiki)

## Try It

| Platform       | Download                                                                                        |
| -------------- | ----------------------------------------------------------------------------------------------- |
| Browser (WASM) | [**Open Showcase**](https://go-gui-org.github.io/showcase/) — zero install, instant evaluation  |
| macOS          | [Go-Gui-Showcase-\<version\>.dmg](https://github.com/go-gui-org/go-gui/releases)                |
| Linux          | [go-gui-showcase-\<version\>-linux-amd64.tar.gz](https://github.com/go-gui-org/go-gui/releases) |
| Windows        | [go-gui-showcase-\<version\>-windows-amd64.zip](https://github.com/go-gui-org/go-gui/releases)  |

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

---

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

## Quick Start

```go
package main

import (
    "fmt"

    "github.com/go-gui-org/go-gui/gui"
    "github.com/go-gui-org/go-gui/gui/backend"
)

type App struct{ Clicks int }

func main() {
    gui.SetTheme(gui.ThemeDark.WithBorders(true))

    w := gui.NewWindow(gui.WindowCfg{
        State:  &App{},
        Title:  "Get Started",
        Width:  300,
        Height: 300,
        OnInit: func(w *gui.Window) { w.UpdateView(mainView) },
    })

    backend.Run(w)
}

func mainView(w *gui.Window) gui.View {
    app := gui.State[App](w)

    // FillFill fills the window and tracks resize automatically.
    return gui.Column(gui.ContainerCfg{
        Sizing: gui.FillFill,
        HAlign: gui.HAlignCenter,
        VAlign: gui.VAlignMiddle,
        Content: []gui.View{
            gui.Text(gui.TextCfg{
                Text:      "Hello GUI!",
                TextStyle: gui.CurrentTheme().B1,
            }),
            gui.Button(gui.ButtonCfg{
                ID: "gs_counter",
                Content: []gui.View{
                    gui.Text(gui.TextCfg{
                        Text: fmt.Sprintf("%d Clicks", app.Clicks),
                    }),
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

## Installation

Requires **Go 1.26+** and a C toolchain (CGo). The desktop backends are
native: Metal on macOS, X11 + EGL on Linux, Win32 + WGL on
Windows. Text shaping and rasterization are pure Go via go-glyph — no
FreeType, HarfBuzz, Pango, CoreText, or DirectWrite libraries required. See
the
[Installation Guide](https://github.com/go-gui-org/go-gui/wiki/Installation)
for platform-specific instructions.

```bash
# macOS:   xcode-select --install && brew install go   # Metal is a system framework
# Ubuntu:  sudo apt-get install golang gcc pkg-config libgl1-mesa-dev libx11-dev
# Fedora:  sudo dnf install golang gcc pkgconf-pkg-config mesa-libGL-devel libX11-devel
# Arch:    sudo pacman -S go gcc pkgconf mesa libx11
# Windows: mingw-w64 GCC required (e.g. MSYS2 MinGW x64) —
#          pacman -S mingw-w64-x86_64-go mingw-w64-x86_64-gcc

go get github.com/go-gui-org/go-gui
```

![todo example](assets/todo.png)

---

## Building the Showcase

The root `Makefile` builds standalone showcase binaries for each platform.

| Target               | Output                                 | Command                              |
| -------------------- | -------------------------------------- | ------------------------------------ |
| `make build-macos`   | `build/showcase-macos`                 | `go build`                           |
| `make build-linux`   | `build/showcase-linux`                 | `go build`                           |
| `make build-windows` | `build/showcase-windows.exe`           | `go build` (mingw-w64 cross-compile) |
| `make build-wasm`    | `build/showcase.wasm` + `wasm_exec.js` | `GOOS=js GOARCH=wasm go build`       |
| `make release`       | `.tar.gz`, `.zip`, `.dmg`              | All of the above + packaging         |

Related targets: `make build-ios` (c-archive for the iOS demo),
`make build-android` (gomobile `.aar`), `make build-examples` (compile
every example to `examples/bin/`).

**Linux and Windows** produce self-contained binaries with no external
library dependencies: text shaping and rasterization are pure Go
(go-glyph). A plain `go build ./examples/showcase/` is self-contained;
on Linux the showcase's audio demo needs ALSA headers at build time
(`libasound2-dev`).

**Windows cross-compilation** requires `mingw-w64`. On Windows (MSYS2),
use `make build-windows CC_WINDOWS=gcc`.

Version and commit are injected from git tags via `-ldflags`.

![Digital Rain Screenshot](assets/digital-rain.png)

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

## Roadmap

Planning lives in [GitHub Issues](../../issues) and the go-gui-org project
board, not a checked-in roadmap file. Browse open issues for current and
planned work.

## License

[MIT](LICENSE)
