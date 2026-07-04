# Go-Gui Architecture

## High-Level Pipeline

Immediate-mode GUI — no virtual DOM, no diffing. Each frame rebuilds the
entire UI from the view function.

```
┌─────────────────────────────────────────────────────────────────────┐
│                          APPLICATION                                │
│                                                                     │
│  w := gui.NewWindow(WindowCfg{State: &App{}})                       │
│  app := gui.State[App](w)    ← typed state slot per window          │
│                                                                     │
│  View func(w) → returns *Layout tree                                │
└──────────────────────────────┬──────────────────────────────────────┘
                               │
                               ▼
┌─────────────────────────────────────────────────────────────────────┐
│                     FRAME PIPELINE (per frame)                      │
│                                                                     │
│  ┌──────────────┐    ┌──────────────┐    ┌───────────────────────┐  │
│  │ View func    │───▶│ generateView │───▶│ Layout tree           │  │
│  │ (user code)  │    │ Layout()     │    │ (Layout + Shape nodes)│  │
│  └──────────────┘    └──────────────┘    └───────────┬───────────┘  │
│                                                      │              │
│                                                      ▼              │
│  ┌───────────────────────────────────────────────────────────────┐  │
│  │ layoutArrange()                                               │  │
│  │  ├─ resolve Sizing (Fit/Fixed/Grow per axis)                  │  │
│  │  ├─ layoutFillWidths / layoutFillHeights                      │  │
│  │  ├─ spacing() — visible-children-only gap calc                │  │
│  │  └─ AmendLayout hooks (overlay repositioning)                 │  │
│  └───────────────────────────────────┬───────────────────────────┘  │
│                                      │                              │
│                                      ▼                              │
│  ┌───────────────────────────────────────────────────────────────┐  │
│  │ renderLayout(bgColor, clip, w) → emits into w.renderers       │  │
│  │  ├─ walk arranged tree                                        │  │
│  │  ├─ emit RenderCmd per Shape (rect, text, circle, image, SVG) │  │
│  │  ├─ apply ColorFilter / effects                               │  │
│  │  └─ clip regions, overflow handling                           │  │
│  └───────────────────────────────────┬───────────────────────────┘  │
│                                      │                              │
│                                      ▼                              │
│                              w.renderers                            │
└──────────────────────────────┬──────────────────────────────────────┘
                               │
                               ▼
┌─────────────────────────────────────────────────────────────────────┐
│                         BACKEND LAYER                               │
│                                                                     │
│  Build-tag dispatch (gui/backend/run_*.go):                         │
│                                                                     │
│  darwin && !ios → Metal (CGo) + SDL2 windowing                      │
│  ios            → Metal (CGo) + UIKit windowing                     │
│  android        → OpenGL ES 3.0 (CGo) + Android Activity/View       │
│  js && wasm     → Canvas2D + WebGL2 (custom shaders)                │
│  !darwin && !js && !android && gl → OpenGL 3.3 + SDL2 windowing     │
│  !darwin && !js && !android && !gl → SDL2 software renderer         │
│                                                                     │
│  ┌──────────┬──────────┬──────────┬──────────┬──────────┐           │
│  │ macOS    │ iOS      │ Linux    │ Windows  │ Web      │           │
│  │ Metal    │ Metal    │ GL/SDL2  │ GL/SDL2  │ Canvas2D │           │
│  │ + SDL2   │ + UIKit  │ + SDL2   │ + SDL2   │ + WebGL2 │           │
│  └──────────┴──────────┴──────────┴──────────┴──────────┘           │
│  ┌──────────────────────────────────────────────────────┐           │
│  │ Android: GLES3 (CGo) + Android Activity/View         │           │
│  └──────────────────────────────────────────────────────┘           │
│                                                                     │
│  Shared services (all backends):                                    │
│  ├─ TextMeasurer (via go-glyph)                                     │
│  ├─ SvgParser (SVG parse + tessellate)                              │
│  ├─ NativeDialogs (filedialog / printdialog)                        │
│  └─ NativePlatform (a11y, IME, tray, menubar, spellcheck,           │
│       notifications, bookmarks, URI opening)                        │
│                                                                     │
│  ┌──────────────────────────────────────────────────────┐           │
│  │ Tests: nil injected interfaces — no backend needed   │           │
│  └──────────────────────────────────────────────────────┘           │
└─────────────────────────────────────────────────────────────────────┘
```

## Core Types

```
┌──────────────────────────────────────────────────────────────────┐
│ Window                                                           │
│  ├─ state     any           ← typed slot: State[T](w)            │
│  ├─ stateMap  map[ns]any    ← per-widget internal state          │
│  ├─ layout    Layout        ← root of current frame's tree       │
│  ├─ renderers []RenderCmd   ← draw list for backend              │
│  ├─ animations map[string]Animation                              │
│  └─ commands  []Command     ← keyboard shortcuts                 │
├──────────────────────────────────────────────────────────────────┤
│ Layout                                                           │
│  ├─ Shape    *Shape         ← renderable properties              │
│  ├─ Parent   *Layout        ← pointer up                         │
│  ├─ Children []Layout       ← values down (no pointer cycles)    │
│  ├─ Axis     AxisType       ← Row / Column / None                │
│  └─ Sizing   SizingType     ← Fit/Fixed/Grow per axis            │
├──────────────────────────────────────────────────────────────────┤
│ Shape                                                            │
│  ├─ Pos, Size              ← absolute coordinates                │
│  ├─ Color, ColorBorder     ← appearance                          │
│  ├─ ShapeType              ← Rect, Circle, Text, Image, SVG...   │
│  ├─ TC *ShapeTextConfig    ← text fields (not on Shape directly) │
│  ├─ Events callbacks       ← OnClick, OnHover, OnKey...          │
│  ├─ Effects []Effect       ← shadows, blur, filters              │
│  └─ AmendLayout func       ← post-sizing hook                    │
├──────────────────────────────────────────────────────────────────┤
│ RenderCmd                                                        │
│  ├─ Kind     RenderCmdKind ← what to draw                        │
│  ├─ Pos, Size              ← screen coordinates                  │
│  ├─ Color, Radius          ← visual properties                   │
│  └─ ...per-kind fields     ← text, image, SVG data, clip, etc.   │
└──────────────────────────────────────────────────────────────────┘
```

## Subsystems

```
┌───────────────────────────────────┐  ┌──────────────────────────────┐
│ EVENT DISPATCH                    │  │ ANIMATION                    │
│                                   │  │                              │
│ OS event → SDL2 → Event struct    │  │ Animation interface:         │
│  ├─ hit-test Layout tree          │  │  ├─ Tween (value lerp)       │
│  ├─ bubble up to ancestors        │  │  ├─ Spring (physics-based)   │
│  ├─ e.IsHandled stops propagation │  │  ├─ Keyframe (waypoints)     │
│  └─ callbacks: func(*Layout,      │  │  ├─ Layout (FLIP-style)      │
│       *Event, *Window)            │  │  ├─ Hero (cross-view)        │
│                                   │  │  └─ BlinkCursor              │
│ Key dispatch also feeds Commands  │  │                              │
│ (keyboard shortcuts / Shortcut)   │  │ Easing: bezier LUT cache     │
└───────────────────────────────────┘  └──────────────────────────────┘

┌───────────────────────────────────┐  ┌──────────────────────────────┐
│ STATE MANAGEMENT                  │  │ THEME SYSTEM                 │
│                                   │  │                              │
│ Per-window typed slot:            │  │ Widget Cfg structs use       │
│   gui.State[App](w)               │  │ Opt[float32] for all numeric │
│                                   │  │ fields. Zero = use theme     │
│ Per-widget internal state:        │  │ default; Some(v) = override. │
│   StateMap[K,V](w, namespace,     │  │                              │
│     capacity)                     │  │ DefaultContainerStyle sets   │
│                                   │  │ baseline (SizeBorder=1.5)    │
│ No globals, no closures for state │  │                              │
└───────────────────────────────────┘  └──────────────────────────────┘

┌───────────────────────────────────┐  ┌──────────────────────────────┐
│ ACCESSIBILITY                     │  │ TEXT (via glyph)             │
│                                   │  │                              │
│ A11yNode tree built from Layout   │  │ go-glyph (versioned module): │
│ Exposes to platform via           │  │  ├─ text shaping             │
│   NativePlatform (AT-SPI on       │  │  ├─ rendering                │
│   Linux, NSAccessibility on mac)  │  │  ├─ line wrapping            │
│                                   │  │  ├─ bidi / RTL               │
└───────────────────────────────────┘  │  ├─ emoji / grapheme         │
                                       │  └─ measurement              │
                                       └──────────────────────────────┘
```

## Package Map

```
go-gui/
├── gui/                          ← core (~200 non-test .go files at top level)
│   ├── view*.go                  ← View interface, generateViewLayout
│   ├── layout*.go                ← Layout tree, arrange, query
│   ├── shape*.go                 ← Shape type + ShapeTextConfig
│   ├── render*.go                ← renderLayout, RenderCmd, filters
│   ├── window*.go                ← Window, lifecycle, state
│   ├── event*.go                 ← Event, dispatch, handlers
│   ├── animation*.go             ← Animation subsystem
│   ├── command*.go               ← Keyboard shortcuts
│   ├── a11y*.go                  ← Accessibility tree
│   ├── opt.go                    ← Opt[T] generic optional
│   ├── view_<widget>.go          ← Widget factories (button, input, grid...)
│   └── datagrid/                 ← DataGrid subpackage (data sources, ORM, export)
│   └── backend/
│       ├── sdl2/                 ← SDL2 backend (TextMeasurer, SvgParser, NativePlatform)
│       ├── metal/                ← Metal renderer (macOS)
│       ├── gl/                   ← OpenGL renderer (Linux/Windows)
│       ├── filedialog/           ← Native file dialogs
│       ├── printdialog/          ← Native print dialogs
│       ├── android/              ← Android backend (GLES3 + JNI)
│       ├── ios/                  ← iOS backend (Metal + UIKit)
│       ├── web/                  ← Web/WASM backend (Canvas2D + WebGL2)
│       ├── nativemenu/           ← Native menu support
│       ├── atspi/                ← AT-SPI accessibility (Linux)
│       ├── sni/                  ← StatusNotifierItem / system tray
│       ├── spellcheck/           ← Spell checking
│       └── internal/             ← Shared backend internals
└── examples/                     ← 53 example apps
    ├── get_started/
    ├── showcase/
    ├── calculator/
    ├── todo/
    ├── snake/
    └── ...
```

## Future Directions

- **WebGPU**: Explored on the `webgpu-backend` branch (deleted) — 12
  WGSL shader pipelines, device init, and render loop were working.
  Rejected because WebGPU has no native text rendering path; a pure-Go
  TTF rasterizer in go-glyph would be needed first.
- **SDL2 on desktop**: SDL2 is the default renderer on Linux and
  Windows. OpenGL is opt-in via the `gl` build tag. On macOS, SDL2
  provides windowing and input while Metal handles rendering.
  SDL2's own renderer skips `RenderCustomShader` (no GPU pipeline).
