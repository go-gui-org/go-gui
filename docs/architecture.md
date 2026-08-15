# Go-Gui Architecture

## High-Level Pipeline

Immediate-mode GUI — no virtual DOM, no diffing. Each frame rebuilds the entire
UI from the view function.

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
│  darwin && !ios → Metal (CGo) + AppKit windowing                    │
│  ios            → Metal (CGo) + UIKit windowing                     │
│  android        → OpenGL ES 3.0 (CGo) + Android Activity/View       │
│  js && wasm     → Canvas2D + WebGL2 (custom shaders)                │
│  linux   && !js && !android && !gl → native GL backend              │
│  windows && !js && !android && !gl → native GL backend              │
│  !darwin && !js && !android && gl  → force GL backend               │
│  (any other combination            → panics: unsupported platform)  │
│                                                                     │
│  ┌──────────┬──────────┬──────────┬──────────┬──────────┐           │
│  │ macOS    │ iOS      │ Linux    │ Windows  │ Web      │           │
│  │ Metal    │ Metal    │ GL       │ GL       │ Canvas2D │           │
│  │ + AppKit │ + UIKit  │ + X11    │ + Win32  │ + WebGL2 │           │
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
│  ├─ Kind     RenderKind    ← what to draw                        │
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
│ OS event → Event struct           │  │  │ Animation interface:      │
│  ├─ hit-test Layout tree          │  │  ├─ Tween (value lerp)       │
│  ├─ travel up to ancestors        │  │  ├─ Spring (physics-based)   │
│  ├─ ctx.Consume() stops it;       │  │  ├─ Keyframe (waypoints)     │
│  │   silence lets it travel       │  │  ├─ Layout (FLIP-style)      │
│  └─ callbacks: func(EventCtx)     │  │  ├─ Hero (cross-view)        │
│                                   │  │  └─ BlinkCursor              │
│ Key dispatch also feeds Commands  │  │                              │
│ (keyboard shortcuts / Shortcut)   │  │ Easing: bezier LUT cache     │
└───────────────────────────────────┘  └──────────────────────────────┘

┌───────────────────────────────────┐  ┌──────────────────────────────┐
│ STATE MANAGEMENT                  │  │ THEME SYSTEM                 │
│                                   │  │                              │
│ Per-window typed slot:            │  │ Use Opt[T] where zero is a   │
│   gui.State[App](w)               │  │ a real user choice. Plain    │
│                                   │  │ fields elsewhere. Unset      │
│ Per-widget internal state:        │  │ falls through to theme.      │
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
└── examples/                     ← 59 example apps
    ├── get_started/
    ├── showcase/
    ├── calculator/
    ├── todo/
    ├── snake/
    └── ...
```

## Maintainer Invariants

Rules maintainers rely on during changes. Each entry states what must remain
true, where to change things, and which local or CI check catches a
regression. Existing specs are linked where they define policy in depth —
do not re-derive policy from code that drifts.

### Frame lifecycle

- Every frame runs on the one main OS thread, in order: `generateViewLayout`
  (builds the Layout tree) → `layoutArrange` (sizing, ID resolution, float
  extraction) → `renderLayout` (emits into `w.renderers`) → backend
  `renderersDraw` (drains read-only). All `*Window` access is main-thread-only;
  backends wake the loop from other threads via `SetWakeMainFn`.
- Frame-scoped objects come from the `w.scratch` pools (`allocShape`,
  `allocEventHandlers`, `allocEffects`, layer slices). Pointers are valid only
  until the next frame's pool reset — never store them in window/app state.
- **Where to change:** `gui/window_update.go` owns the frame pass; keep new
  pipeline work inside `FrameFn` on the main thread, allocating from scratch
  pools.
- **Catches:** `make test-race`; `make bench-gate` treats alloc counts as hard
  gates; `(*Window).FrameCount()` distinguishes one frame from re-entrancy.

### Event consumption (the one-event rule)

- Nothing is marked handled for you. A callback that acts on an event calls
  `ctx.Consume()`; one that does not lets the event travel to ancestors. There
  is no `ctx.Bubble()` — declining is silence. Applies to every callback,
  `OnClick`/`OnChar` no differently from `OnKeyDown`/`OnHover`.
- `ctx.Event` is nil in `AmendLayout` and `OnScroll`; both `EventCtx` methods
  are nil-safe.
- **Where to change:** any callback. Consuming for an effect, silence for a
  pass-through — never pre-mark events handled.
- **Catches:** `gui.Debug(true)` (category `DebugUnconsumed`) reports handlers
  that act without consuming while an ancestor also handles;
  `(*Window).TestUnconsumedEvents` sweeps a whole window; `make ergonomics-audit`
  mode `callbacks` inventories signatures. Policy:
  `docs/specs/eventctx-callback-refactor.md`.

### ID scoping

- `Shape.ID` is a leaf. `resolveShapeIDs` (run from `layoutArrange`) stamps
  `Shape.effID` = the leaf joined to its ID-bearing ancestors, and every
  ID-keyed store and lookup uses `shape.idKey()` — never bare `Shape.ID` at a
  keying site. Effective IDs must be unique per window; a leaf containing `:`
  is absolute and is not joined again.
- Compose inner IDs with `gui.ScopeID`/`gui.ScopeIDN`, never by hand (a part —
  row key, heading slug — must not contain `:`; rebuilding an ID at a lookup
  site is how producers and consumers drift). A composite widget's inner shape
  that needs the owner's focus state sets `Shape.focusOwner` (a reference)
  instead of repeating its ID.
- **Where to change:** factories building IDs, any new ID-keyed store.
- **Catches:** `(*Window).TestDuplicateIDs`; `gui.Debug` (category
  `DebugDuplicates`); `make ergonomics-audit` mode `ids` fails hand-rolled
  composition. Specs: `docs/specs/widget-id-scoping.md`,
  `docs/specs/widget-id-per-scope-uniqueness.md`.

### Render command ownership

- `renderLayout` rebuilds `w.renderers` from scratch every frame. Backends
  consume it read-only via `(*Window).Renderers()` during `renderersDraw` and
  must not retain or mutate commands across frames.
- Bracket kinds (`RenderClip`, `RenderStencilBegin/End`,
  `RenderFilterBegin/End`, `RenderRotateBegin/End`) must stay balanced; clip
  commands take effect immediately for subsequent commands in the stream.
- **Where to change:** `gui/render_layout.go` to emit; `gui/backend/*/draw.go`
  to consume. Adding a `RenderKind` touches the dispatch switch in every
  backend.
- **Catches:** `gui/render_layout_test.go`, `gui/backend/*/render*_test.go`
  (draw-order and bracket-balance coverage).

### Backend locking & threading

- Backends call into gui only on the main OS thread; they lock only their own
  state (e.g. the GL backend's own `w.Lock()` in `gui/backend/gl/backend.go`),
  never window state.
- Platform dispatch is by build tag per the Backend Layer diagram above.
  macOS stays CGo by decision; `CGO_ENABLED=0 go build ./...` is green on
  Linux and Windows — do not re-open the CGo question without a trigger.
- **Where to change:** new backends must follow the injected-interface pattern
  (below) and the build-tag table; keep `gui/` free of platform imports.
- **Catches:** `make cross-compile`, `make lint-cross`, `make test` (runs the
  GL backend with `CGO_ENABLED=0`). Direction:
  `docs/specs/cgo-free-backend-feasibility.md`, `docs/specs/macos-native-backend.md`.

### State ownership

- One typed slot per window: `gui.State[T](w)` type-asserts and panics if the
  window holds a different type. Widget internal state lives in the per-window
  `StateMap`, keyed by namespace consts (`nsOverflow`, `nsInput`, ...). No
  globals, no closures for state.
- Theme is window-owned; `guiTheme` and the `default*Style` package vars are a
  frame-scoped cache, never app state — `installTheme` refills them per theme
  change and they have no initializers (the mirrors would drift). The two
  exceptions are `DefaultTextStyle` and `defaultInspectorStyle`, which are
  `ThemeMaker` inputs. Key on `Theme.id`, never `Theme.Name`.
- Theme reads split by phase: outside generation call `w.Theme()`; factories
  and `GenerateLayout` keep the bare `guiTheme`/`default*Style` read (required
  for `Themed` subtree scoping). Mark deliberate exceptions
  `ergonomics-audit:theme-global`.
- **Where to change:** adding widget state → new namespace const + `StateMap`;
  adding a default style → `ThemeMaker` only.
- **Catches:** `TestDefaultStylesMirrorThemeDark`;
  `make ergonomics-audit` mode `theme` gates the post-generation paths. Specs:
  `docs/specs/per-window-theme.md`, `docs/specs/theme-style-single-source.md`.

### Native platform boundaries

- Backends inject `TextMeasurer` (glyph metrics), `SvgParser`, and
  `NativePlatform` (dialogs, notifications, print, a11y, IME, titlebar) at
  startup. All are nil in tests — test code must never require a backend.
- `gui/` never imports backend packages; `NativePlatform` is the only seam to
  platform services.
- **Where to change:** a new platform capability is a new `NativePlatform`
  method implemented per backend, not a direct call from `gui/`.
- **Catches:** `make test` runs with nil injected interfaces; `make vet`
  (incl. the `requiredid` analyzer) flags structural mistakes.

### Widget Cfg invariants

- **Focusable defaults:** the 15 input Cfgs (`Button`, `ColorPicker`,
  `Combobox`, `DatePicker`, `Input`, `InputDate`, `ListBox`, `NumericInput`,
  `RadioButtonGroup`, `Radio`, `Select`, `Slider`, `Switch`, `Toggle`, `Tree`)
  are focusable by default; opt out with `FocusDisabled`, never
  `Focusable: false`. `Focusable` without a non-empty `ID` is a silent no-op —
  the widget renders and clicks but never joins the tab order. Spec:
  `docs/specs/focusable-default-input.md`.
- **A11y fields:** `A11YLabel`/`A11YDescription` live on the embedded
  `A11YCfg`; never redeclare them on a Cfg. Construction names the embed:
  `gui.ButtonCfg{ID: "save", A11YCfg: gui.A11YCfg{A11YLabel: "Save"}}`.
- **`Opt[T]` vs plain fields:** only primitives get `Opt` (when zero is a
  legitimate user choice that must be distinguishable from unset, e.g.
  `SizeBorder`). Owned types self-flag instead: `Color`, `Padding`, and
  `Sizing` carry a `set` field, so they are plain fields with
  `IsSet()`/`Or()`. Build them with constructors — `RGBA`/`RGB`/`Hex`,
  `NewPadding`/`PadAll`, the predefined `Sizing` vars — never raw
  `Color{...}`/`Padding{...}`/`Sizing{...}` literals (they read as unset).
- **Where to change:** any widget factory or Cfg struct.
- **Catches:** `make vet` (`requiredid`), `make ergonomics-audit` modes
  `focus`, `a11y`, `opt`, and `literals`; `gui.Debug` reports focusable
  shapes with no ID at runtime.

### Generated-file expectations

- `gui/svg_spinner_kinds_gen.go` is produced by `go generate ./...`
  (`gui/internal/gen/spinnerkinds/`). Commit generated output; never hand-edit
  it. Any change to the generator must be committed with its regenerated
  output, or `make generate-check` fails the push.
- **Where to change:** `gui/internal/gen/`, then run `go generate ./...`.
- **Catches:** `make generate-check` (part of `make check` and `prepush`)
  fails if `go generate ./...` produces a diff in `*_gen.go`.

### Validation map

| Invariant class                    | Local / CI gate                                                                |
| ---------------------------------- | ------------------------------------------------------------------------------ |
| Frame lifecycle, concurrency       | `make test-race`, `make bench-gate` (alloc gates)                              |
| Event consumption                  | `TestUnconsumedEvents`, `gui.Debug` (`DebugUnconsumed`), ergoaudit `callbacks` |
| Duplicate / mis-scoped IDs         | `TestDuplicateIDs`, `gui.Debug` (`DebugDuplicates`), ergoaudit `ids`           |
| Render command stream              | `gui/render_layout_test.go`, backend render tests                              |
| Backend build matrix, threading    | `make cross-compile`, `make lint-cross`, `make test` (`CGO_ENABLED=0` GL)      |
| Theme single source                | `TestDefaultStylesMirrorThemeDark`, ergoaudit `theme`                          |
| Native platform seam               | `make test` (nil interfaces), `make vet` (`requiredid`)                        |
| Focusable + ID, a11y, Opt/literals | `make vet`, ergoaudit `focus`/`a11y`/`opt`/`literals`, `gui.Debug`             |
| Generated files                    | `make generate-check`                                                          |
| Exported surface                   | `make export-audit` — see `docs/specs/exportaudit-surface-policy.md`           |

## Future Directions

- **WebGPU**: Explored on the `webgpu-backend` branch (deleted) — 12 WGSL shader
  pipelines, device init, and render loop were working. Superseded: the native
  GL backend already runs cgo-free on Linux and Windows (X11 via xgb, EGL via
  purego). WebGPU would add 10–40 MB of runtime shared libraries and still leave
  macOS — the only real CGo backend — untouched. Full assessment:
  `docs/specs/cgo-free-backend-feasibility.md`.
- **Native GL on desktop**: The native GL backend is the default renderer on
  Linux and Windows. It provides direct platform windowing (X11 on Linux, Win32
  on Windows) with EGL/WGL contexts — no intermediate library dependency.
