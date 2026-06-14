# Go-Gui

![Go version](https://img.shields.io/badge/go-1.26%2B-blue)
![License](https://img.shields.io/badge/license-MIT-blue)
![CI](https://github.com/go-gui-org/go-gui/actions/workflows/ci.yml/badge.svg)
[![Ask DeepWiki](https://deepwiki.com/badge.svg)](https://deepwiki.com/mike-ward/go-gui)
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
| Linux          | [Go-Gui-Showcase-\<version\>-linux-amd64.tar.gz](https://github.com/go-gui-org/go-gui/releases) |
| Windows        | [Go-Gui-Showcase-\<version\>-windows-amd64.zip](https://github.com/go-gui-org/go-gui/releases)  |

Sibling projects:

- **go-charts**\
  Interactive chart widgets. https://github.com/mike-ward/go-charts

- **go-edit**\
  Code editor widget. https://github.com/mike-ward/go-edit

- **go-map**\
  SMIL map widgets. https://github.com/mike-ward/go-map

- **go-term**\
  Embeddable terminal emulator. https://github.com/go-gui-org/go-term

- **go-glyph**\
  Text rendering engine on steroids. https://github.com/go-gui-org/go-glyph

## How it works

Each frame, a plain Go view function returns a layout tree. The framework sizes,
positions, and renders it in a single pass — no virtual DOM, no diffing.

```
View fn → GenerateViewLayout() → Layout tree
  → layoutArrange() (Fit/Fixed/Grow sizing)
  → renderLayout() → []RenderCmd
  → Backend (Metal / OpenGL / Web/WASM)
```

State lives in a typed slot per window (`gui.State[T](w)`). Backend interfaces
(TextMeasurer, SvgParser, NativePlatform) are injected at startup — swap backends
without changing widget code. Headless test backend included.

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
- **Headless test backend** — all layout and widget logic runs without a display
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
    gui.SetTheme(gui.ThemeDarkBordered)

    w := gui.NewWindow(gui.WindowCfg{
        State:  &App{},
        Title:  "get_started",
        Width:  300,
        Height: 300,
        OnInit: func(w *gui.Window) { w.UpdateView(mainView) },
    })

    backend.Run(w)
}

func mainView(w *gui.Window) gui.View {
    ww, wh := w.WindowSize()
    app := gui.State[App](w)

    return gui.Column(gui.ContainerCfg{
        Width:  float32(ww),
        Height: float32(wh),
        Sizing: gui.FixedFixed,
        HAlign: gui.HAlignCenter,
        VAlign: gui.VAlignMiddle,
        Content: []gui.View{
            gui.Text(gui.TextCfg{
                Text:      "Hello GUI!",
                TextStyle: gui.CurrentTheme().B1,
            }),
            gui.Button(gui.ButtonCfg{
                IDFocus: 1,
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

Requires **Go 1.26+** and SDL2 development libraries. See the
[Installation Guide](https://github.com/go-gui-org/go-gui/wiki/Installation)
for platform-specific instructions.

```bash
# macOS:   brew install go pkg-config sdl2 sdl2_mixer freetype harfbuzz pango fontconfig
# Ubuntu:  sudo apt-get install golang libsdl2-dev libsdl2-mixer-dev libfreetype6-dev libharfbuzz-dev libpango1.0-dev libfontconfig1-dev
# Fedora:  sudo dnf install golang SDL2-devel SDL2_mixer-devel freetype-devel harfbuzz-devel pango-devel fontconfig-devel
# Arch:    sudo pacman -S go sdl2 sdl2_mixer freetype2 harfbuzz pango fontconfig
# Windows: Use MSYS2 MinGW x64 — pacman -S mingw-w64-x86_64-go mingw-w64-x86_64-SDL2 mingw-w64-x86_64-SDL2_mixer mingw-w64-x86_64-freetype mingw-w64-x86_64-harfbuzz mingw-w64-x86_64-pango mingw-w64-x86_64-fontconfig

go get github.com/go-gui-org/go-gui
```

![todo example](assets/todo.png)

---

## Configuration

### Backend Selection

`backend.Run(w)` auto-selects Metal on macOS and OpenGL elsewhere:

```go
import "github.com/go-gui-org/go-gui/gui/backend"

backend.Run(w) // Metal on macOS, GL on Linux/Windows
```

To force a specific backend, import it directly:

```go
import metal "github.com/go-gui-org/go-gui/gui/backend/metal" // macOS only
import gl    "github.com/go-gui-org/go-gui/gui/backend/gl"    // cross-platform
import sdl2  "github.com/go-gui-org/go-gui/gui/backend/sdl2"  // software fallback
import web   "github.com/go-gui-org/go-gui/gui/backend/web"   // WASM/browser
import ios   "github.com/go-gui-org/go-gui/gui/backend/ios"   // iOS
```

### Themes

```go
gui.SetTheme(gui.ThemeDark)          // set globally before NewWindow
gui.SetTheme(gui.ThemeDarkBordered)  // dark with visible borders
t := gui.CurrentTheme()              // read anywhere
```

Custom themes are built with `gui.ThemeMaker` and registered via
`gui.RegisterTheme`.

---

## Usage Examples

### State Management

Per-window state via a single typed slot — no globals, no closures:

```go
type Counter struct{ N int }

w := gui.NewWindow(gui.WindowCfg{State: &Counter{}})

// Inside any callback or view:
s := gui.State[Counter](w)
s.N++
```

### Event Handling

Events are wired through `Cfg` structs:

```go
gui.Button(gui.ButtonCfg{
    IDFocus: 1,
    OnClick: func(l *gui.Layout, e *gui.Event, w *gui.Window) {
        // handle click
        e.IsHandled = true
    },
})

gui.Input(gui.InputCfg{
    OnKeyDown:    func(l *gui.Layout, e *gui.Event, w *gui.Window) { … },
    OnCharInput:  func(l *gui.Layout, e *gui.Event, w *gui.Window) { … },
    OnTextCommit: func(l *gui.Layout, e *gui.Event, w *gui.Window) { … },
})
```

### Stdlib Data Binding

Data widgets accept Go stdlib types via convenience fields. This is the
zero-configuration path:

| Widget           | Field       | Type                  | Replaces           |
| ---------------- | ----------- | --------------------- | ------------------ |
| Select, Combobox | `Options`   | `[]string`            | _(already stdlib)_ |
| ListBox          | `Items`     | `[]string`            | `Data`             |
| RadioButtonGroup | `Items`     | `[]string`            | `Options`          |
| Table            | `RawData`   | `[][]string`          | `Data`             |
| Tree             | `ItemPaths` | `[]string`            | `Nodes`            |
| DataGrid         | `RowsData`  | `[]map[string]string` | `Rows`             |

When both the stdlib field and the typed struct field are set, the stdlib
field takes precedence. Use the typed field when you need IDs different
from display text, per-row styling, lazy loading, or custom cell editors.

**ListBox — simple string list**

```go
gui.ListBox(gui.ListBoxCfg{
    ID:    "langs",
    Items: []string{"Go", "Rust", "Zig"},
    OnSelect: func(ids []string, _ *gui.Event, w *gui.Window) {
        gui.State[App](w).Selected = ids
    },
})
```

**Table — CSV-style data**

```go
w.Table(gui.TableCfg{
    ID: "simple-table",
    RawData: [][]string{
        {"Name", "Age"},
        {"Alice", "30"},
        {"Bob",   "25"},
    },
})
```

**Tree — flat path strings**

```go
gui.Tree(gui.TreeCfg{
    ID:        "project",
    ItemPaths: []string{"src/main.go", "src/lib.go", "docs/readme.md"},
    OnSelect: func(id string, _ *gui.Event, w *gui.Window) { ... },
})
```

**DataGrid — key-value maps**

```go
w.DataGrid(gui.DataGridCfg{
    ID: "simple-grid",
    RowsData: []map[string]string{
        {"name": "Alice", "age": "30"},
        {"name": "Bob",   "age": "25"},
    },
})
```

### Example Apps

45+ example apps in [`examples/`](examples/) — from `get_started` to
`showcase`, `calculator`, `snake`, `dock_layout`, `digital_rain`, and more.

```bash
go run ./examples/get_started/
go run ./examples/showcase/
```

![Digital Rain Screenshot](assets/digital-rain.png)

---

Full widget reference: [Widget Catalogue](https://github.com/go-gui-org/go-gui/wiki/Widget-Catalogue)

## Architecture

```
┌─────────────────────────────────────────────────────────┐
│                    Application Layer                    │
│      examples/  ──  View fn(w *Window) *Layout          │
│                 gui.State[T](w) typed state slot        │
└────────────────────────┬────────────────────────────────┘
                         │
┌────────────────────────▼────────────────────────────────┐
│                  gui/ (core package)                    │
│                                                         │
│  ┌──────────────┐  ┌──────────────┐  ┌───────────────┐  │
│  │   Widgets    │  │  State Mgmt  │  │   Animation   │  │
│  │  Button,Text │  │  StateMap    │  │   Subsystem   │  │
│  │  Container…  │  │  per-window  │  │               │  │
│  └──────┬───────┘  └──────┬───────┘  └───────┬───────┘  │
│         │                 │                  │          │
│  ┌──────▼─────────────────▼──────────────────▼───────┐  │
│  │              Layout Engine                        │  │
│  │  GenerateViewLayout() → Layout tree               │  │
│  │  layoutArrange() — Fit/Fixed/Grow sizing          │  │
│  │  renderLayout() → []RenderCmd                     │  │
│  └──────────────────────┬────────────────────────────┘  │
│                         │                               │
│  ┌──────────────────────▼────────────────────────────┐  │
│  │            Event Dispatch                         │  │
│  │  Mouse · Keyboard · Focus · Scroll                │  │
│  └───────────────────────────────────────────────────┘  │
└────────────────────────┬────────────────────────────────┘
                         │
        ┌────────────────┼─────────────────┐
        │                │                 │
┌───────▼──────┐ ┌───────▼──────┐ ┌────────▼───────┐
│  TextMeasurer│ │  SvgParser   │ │ NativePlatform │
│  (interface) │ │  (interface) │ │  (interface)   │
└───────┬──────┘ └───────┬──────┘ └────────┬───────┘
        │                │                 │
┌───────▼────────────────▼─────────────────▼───────────┐
│               backend/sdl2/                          │
│  Injects interfaces at startup · Window management   │
├──────────────┬───────────────┬───────────────────────┤
│  backend/    │  backend/gl/  │  backend/filedialog/  │
│  metal/      │  OpenGL       │  backend/printdialog/ │
│  Metal(macOS)│               │                       │
├──────────────┼───────────────┼───────────────────────┤
│  backend/    │  backend/ios/ │  backend/spellcheck/  │
│  web/        │  Metal+UIKit  │  backend/atspi/       │
│  WASM+Canvas │  (iOS)        │  (Linux a11y)         │
└──────────────┴───────────────┴───────────────────────┘
        │
┌───────▼───────┐
│   go-glyph    │
│  Text shaping │
│  rendering    │
│  wrapping     │
└───────────────┘
```

**Key types:** `Layout` (tree node), `Shape` (renderable), `RenderCmd`
(draw op), `Window` (top-level + state slot)

![calculator example](assets/calculator.png)

---

## Running Tests

Tests run headlessly via the `gui/backend/test` no-op backend — no display
server required.

```bash
# Run all tests
go test ./...

# Run a specific test
go test ./gui/... -run TestFoo

# Static analysis
go vet ./...

# Full lint suite (govet, staticcheck, errcheck, gosimple, unused,
# gofmt, goimports, revive)
golangci-lint run ./...

# Build all packages
go build ./...
```

---

## Building the Showcase

The root `Makefile` builds standalone showcase binaries for each platform.

| Target               | Output                       | Command                                 |
| -------------------- | ---------------------------- | --------------------------------------- |
| `make build-linux`   | `build/showcase-linux`       | `go build -tags static`                 |
| `make build-macos`   | `build/showcase-macos`       | `go build`                              |
| `make build-windows` | `build/showcase-windows.exe` | `go build -tags static` (cross-compile) |
| `make build-wasm`    | `build/showcase.wasm`        | `GOOS=js GOARCH=wasm go build`          |
| `make release`       | `.tar.gz`, `.dmg`, `.zip`    | All of the above + packaging            |

**Linux and Windows** use `-tags static` which activates go-sdl2's bundled
pre-compiled static libraries. No SDL2 installation required — a single
`go build -tags static ./examples/showcase/` produces a self-contained
binary.

**Windows cross-compilation** requires `mingw-w64`. On Windows (MSYS2),
use `make build-windows CC_WINDOWS=gcc`.

Version and commit are injected from git tags via `-ldflags`.

---

## Contributing

1. Install **Go 1.26+** and SDL2 development libraries (see
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

## License

[MIT](LICENSE)
