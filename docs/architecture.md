# Go-Gui Architecture

## High-Level Pipeline

This is an immediate-mode GUI. There is no virtual DOM and no diffing. Each
frame rebuilds the entire UI from the view function.

```
┌─────────────────────────────────────────────────────────────────────┐
│                          APPLICATION                                │
│                                                                     │
│  w := gui.NewWindow(WindowCfg{State: &App{}})                       │
│  app := gui.State[App](w)    ← typed state slot per window          │
│                                                                     │
│  viewGenerator → View, GenerateLayout → Layout value                │
└───────────────────────────────┬─────────────────────────────────────┘
                                │
                                ▼
┌─────────────────────────────────────────────────────────────────────┐
│                     FRAME PIPELINE (per frame)                      │
│                                                                     │
│  ┌──────────────┐   ┌────────────────────┐   ┌───────────────────┐  │
│  │ View         │──▶│ generateViewLayout │──▶│ Layout tree       │  │
│  │ (user code)  │   │ (per View node)    │   │ (Layout+Shape)    │  │
│  └──────────────┘   └────────────────────┘   └─────────┬─────────┘  │
│                                                        │            │
│                                                        ▼            │
│  ┌───────────────────────────────────────────────────────────────┐  │
│  │ layoutArrange()                                               │  │
│  │  ├─ resolve Sizing (Fit/Fixed/Fill per axis)                  │  │
│  │  ├─ layoutFillWidths / layoutFillHeights                      │  │
│  │  ├─ layoutSizeChildren (Splitter/DockLayout size their panes) │  │
│  │  ├─ spacing() — visible-children-only gap calc                │  │
│  │  └─ layoutAmend fires AmendLayout (overlays, hover, caret)    │  │
│  └───────────────────────────────────┬───────────────────────────┘  │
│                                      │                              │
│                                      ▼                              │
│  ┌───────────────────────────────────────────────────────────────┐  │
│  │ renderLayout(layout, bgColor, clip, w) → builds w.renderers   │  │
│  │  ├─ walk arranged tree                                        │  │
│  │  ├─ emit RenderCmd per Shape (rect, text, circle, image, SVG) │  │
│  │  ├─ apply ColorFilter / effects                               │  │
│  │  └─ clip regions, overflow handling                           │  │
│  └───────────────────────────────────┬───────────────────────────┘  │
│                                      │                              │
│                                      ▼                              │
│              w.renderers → Renderers() → backend renderersDraw      │
└───────────────────────────────┬─────────────────────────────────────┘
                                │
                                ▼
┌─────────────────────────────────────────────────────────────────────┐
│                         BACKEND LAYER                               │
│                                                                     │
│  Build-tag dispatch (gui/backend/run_*.go):                         │
│                                                                     │
│  darwin && arm64 && !ios → Metal + AppKit (purego, no CGo)          │
│  darwin && !arm64 && !ios → link error, Intel unsupported           │
│  ios → Metal (purego, shared) + UIKit (CGo)                         │
│  android → Run panics, use android.SetWindow/Start/Render           │
│  js && wasm → Canvas2D + WebGL2 (custom shaders)                    │
│  linux && !js && !android && !gl → native GL (X11 + EGL)            │
│  windows && !js && !android && !gl → native GL (Win32 + WGL)        │
│  !darwin && !js && !android && gl → force GL backend                │
│  (any other combination → panics: unsupported platform)             │
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
│       notifications, bookmarks, URI opening, window opacity,        │
│       frameless drag/resize)                                        │
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
│  ├─ state      any              ← typed slot: State[T](w)        │
│  ├─ viewState  viewState        ← registry + focus/hover/press   │
│  ├─ layout     Layout           ← root of current frame's tree   │
│  ├─ renderers []RenderCmd   ← draw list, embedded windowRender   │
│  ├─ animations              ← map[string]Animation + animMu      │
│  └─ commands []queuedCommand← frame queue (+ shortcut registry)  │
├──────────────────────────────────────────────────────────────────┤
│ Layout                                                           │
│  ├─ Shape *Shape            ← renderable properties              │
│  ├─ Parent *Layout          ← pointer up                         │
│  └─ Children []Layout       ← values down (no pointer cycles)    │
│  (Axis and Sizing live on Shape, not here)                       │
├──────────────────────────────────────────────────────────────────┤
│ Shape                                                            │
│  ├─ X/Y/Width/Height        ← absolute geometry (+Min/Max)       │
│  ├─ Axis Axis, Sizing Sizing ← direction and per-axis size       │
│  ├─ Color, ColorBorder      ← appearance                         │
│  ├─ shapeType (unexported)  ← Rect/Circle/Text/Image/SVG...      │
│  ├─ TC *ShapeTextConfig     ← text fields, not on Shape          │
│  ├─ events *eventHandlers   ← OnClick/OnKey... + AmendLayout     │
│  ├─ fx *shapeEffects        ← shadow/gradient/blur/filter        │
│  └─ a11Y/bc/svgOpts/xf      ← other optionals (unexported)       │
├──────────────────────────────────────────────────────────────────┤
│ RenderCmd                                                        │
│  ├─ Kind         RenderKind    ← what to draw                    │
│  ├─ X, Y, W, H              ← coords (own frame if transformed)  │
│  ├─ Color, Radius           ← visual properties                  │
│  └─ ...per-kind fields        ← text, image, SVG data, clip, more│
└──────────────────────────────────────────────────────────────────┘
```

## Subsystems

```
┌───────────────────────────────────┐  ┌──────────────────────────────┐
│ EVENT DISPATCH                    │  │ ANIMATION                    │
│                                   │  │                              │
│ OS event → Event struct           │  │  Animation implementations:  │
│  ├─ hit-test Layout tree          │  │  ├─ TweenAnimation (lerp)    │
│  ├─ travel up to ancestors        │  │  ├─ SpringAnimation (physics)│
│  ├─ ctx.Consume() stops it;       │  │  ├─ KeyframeAnimation        │
│  │   silence lets it travel       │  │  ├─ layoutTransition (FLIP)  │
│  └─ callbacks: func(EventCtx)     │  │  ├─ HeroTransition           │
│                                   │  │  └─ blinkCursorAnimation     │
│ Key dispatch also feeds Commands  │  │  lowercase = unexported      │
│ (keyboard shortcuts / Shortcut)   │  │ Easing: bezier LUT cache     │
└───────────────────────────────────┘  └──────────────────────────────┘

┌───────────────────────────────────┐  ┌──────────────────────────────┐
│ STATE MANAGEMENT                  │  │ THEME SYSTEM                 │
│                                   │  │                              │
│ Per-window typed slot:            │  │ Use Opt[T] where zero is a   │
│   gui.State[App](w)               │  │ a real user choice. Plain    │
│                                   │  │ fields elsewhere. Unset      │
│ Per-widget internal state:        │  │ falls through to theme.      │
│   StateMap[K, V](w, ns, maxSize)  │  │                              │
│   (*BoundedMap, per window)       │  │ defaultContainerStyle sets   │
│                                   │  │ baseline (SizeBorder=1)      │
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
├── gui/                          ← core (~370 non-test .go files at top level)
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
│   ├── datagrid/                 ← DataGrid (data sources, ORM, export)
│   ├── markdown/                 ← Markdown parse and render (goldmark)
│   ├── svg/                      ← SVG animation support
│   ├── shader/                   ← GLSL sources for the GL backend
│   ├── highlight/                ← Syntax highlighting (chroma)
│   ├── audio/                    ← Opt-in sound effects and music
│   ├── locales/                  ← Locale data (10 JSON files)
│   ├── appinfo/                  ← App manifest (ID, name, version, icon)
│   ├── instance/                 ← One running copy per user
│   └── backend/
│       ├── metal/                ← Metal renderer (macOS)
│       ├── gl/                   ← OpenGL renderer (Linux/Windows)
│       ├── soft/                 ← CPU renderer (headless, no-GPU fallback)
│       ├── filedialog/           ← Native file dialogs
│       ├── printdialog/          ← Native print dialogs
│       ├── android/              ← Android backend (GLES3 + JNI)
│       ├── ios/                  ← iOS backend (Metal + UIKit)
│       ├── web/                  ← Web/WASM backend (Canvas2D + WebGL2)
│       ├── nativemenu/           ← Native menu support
│       ├── atspi/                ← AT-SPI accessibility (Linux)
│       ├── ibus/                 ← IBus input method (Linux IME)
│       ├── sni/                  ← StatusNotifierItem / system tray
│       ├── sysbeep/              ← OS alert sound
│       ├── spellcheck/           ← Spell checking
│       └── internal/             ← Shared backend internals
└── examples/                     ← 89 example apps (plus internal/)
    ├── get_started/
    ├── showcase/
    ├── calculator/
    ├── todo/
    ├── snake/
    └── ...
```

## Maintainer Invariants

Rules maintainers rely on during changes. Each entry states what must remain
true, where to change things, and which local or CI check catches a regression.
Existing specs are linked where they define policy in depth — do not re-derive
policy from code that drifts.

### Frame lifecycle

- Every full frame runs on the one main OS thread, in order:
  `generateViewLayout` (builds the Layout tree and stamps each shape's `effID`)
  → `layoutArrange` (sizing, `focusOwner` resolution, float extraction) →
  `renderLayout` (called from `buildRenderers`, rebuilds `w.renderers`) →
  per-backend `renderersDraw` (drains read-only). A render-only refresh skips
  generation and arrangement: `renderOnlyLocked` rebuilds `w.renderers` from the
  existing tree. All `*Window` access is main-thread-only. Backends wake the
  loop from other threads via `SetWakeMainFn`.
- Frame-scoped objects come from the `w.scratch` arenas. The `w.allocShape`,
  `w.allocEventHandlers` and `w.allocEffects` methods draw from the
  `viewShapes`, `viewEvents` and `viewEffects` arenas. Layer slices come from
  the same pools. Pointers stay valid only until the next frame pool reset. Code
  must not store them in window or app state.
- **Where to change:** `gui/window_update.go` owns the frame pass. Keep new
  pipeline work inside `FrameFn` on the main thread. Allocate from the scratch
  pools.
- **Catches:** `make test-race` runs. `make bench-gate` treats alloc counts as
  hard gates. `(*Window).FrameCount()` distinguishes one frame from re-entrancy.

### Event consumption (the one-event rule)

- Nothing is marked handled for you. A callback that acts on an event calls
  `ctx.Consume()`. One that does not lets the event travel to ancestors. There
  is no `ctx.Bubble()` — declining is silence. It applies to every callback,
  `OnClick`/`OnChar` no differently from `OnKeyDown`/`OnHover`.
- `ctx.Event` is nil in `AmendLayout` and `OnScroll`. Both `EventCtx` methods
  are nil-safe.
- **Where to change:** any callback. Consume for an effect. Keep silence for a
  pass-through. Never pre-mark events handled.
- **Catches:** `gui.Debug(true)` (category `DebugUnconsumed`) reports handlers
  that act without consuming while an ancestor also handles.
  `(*Window).TestUnconsumedEvents` sweeps a whole window.
  `make ergonomics-audit` mode `callbacks` inventories signatures. Policy:
  `docs/specs/eventctx-callback-refactor.md`.

### ID scoping

- `Shape.ID` is a leaf. Layout generation stamps `Shape.effID` = the leaf joined
  to its ID-bearing ancestors. Every ID-keyed store and lookup uses
  `shape.idKey()`, never bare `Shape.ID` at a keying site. Effective IDs must be
  unique per window. A leaf containing `:` is absolute and is not joined again.
- Compose inner IDs with `gui.ScopeID`/`gui.ScopeIDN`, never by hand. A part
  (row key, heading slug) must not contain `:`. Rebuilding an ID at a lookup
  site is how producers and consumers drift. A composite widget's inner shape
  that needs the owner's focus state sets `Shape.focusOwner` (a reference)
  instead of repeating its ID.
- **Where to change:** factories that build IDs, any new ID-keyed store.
- **Catches:** `(*Window).TestDuplicateIDs` asserts a clean window. `gui.Debug`
  (category `DebugDuplicates`) reports duplicates. `make ergonomics-audit` mode
  `ids` fails hand-rolled composition. Specs: `docs/specs/widget-id-scoping.md`,
  `docs/specs/widget-id-per-scope-uniqueness.md`.

### Render command ownership

- `renderLayout` rebuilds `w.renderers` from scratch every frame. Backends
  consume it read-only via `(*Window).Renderers()` during `renderersDraw` and
  must not retain or mutate commands across frames.
- Bracket kinds (`RenderClip`, `RenderStencilBegin/End`,
  `RenderFilterBegin/End`, `RenderRotateBegin/End`, `RenderTransformBegin/End`)
  must stay balanced. Clip commands take effect immediately for subsequent
  commands in the stream.
- **Where to change:** `gui/render_layout.go` emits. `gui/backend/*/draw.go`
  consumes. Adding a `RenderKind` touches the dispatch switch in every backend.
- **Catches:** `gui/render_layout_test.go`, `gui/backend/*/render*_test.go`
  (draw-order and bracket-balance coverage).

### Backend locking & threading

- Backends call into gui only on the main OS thread. They lock only their own
  state (for example, the GL backend's own `w.Lock()` in
  `gui/backend/gl/backend.go`), never window state.
- Platform dispatch is by build tag per the Backend Layer diagram above. The
  macOS backend calls AppKit and Metal through purego (#1304, #1323), so
  `CGO_ENABLED=0 go build ./...` is green on Linux, Windows and darwin/arm64.
  darwin/amd64 is not supported. iOS and Android keep cgo. On iOS only the UIKit
  shell (`ios_app.m`) is cgo: the Metal renderer is the macOS one
  (`gui/backend/internal/mtldraw`) through purego, and purego itself needs cgo
  on iOS (#1342).
- **Where to change:** new backends must follow the injected-interface pattern
  (below) and the build-tag table. Keep `gui/` free of platform imports.
- **Catches:** `make cross-compile`, `make lint-cross`, `make test` (runs the GL
  backend with `CGO_ENABLED=0`). Direction:
  `docs/specs/cgo-free-backend-feasibility.md`,
  `docs/specs/macos-native-backend.md`.

### State ownership

- One typed slot per window: `gui.State[T](w)` type-asserts and panics if the
  window holds a different type. Widget internal state lives in the per-window
  `StateMap`, keyed by namespace consts (`nsOverflow`, `nsInput`, ...). No
  globals, no closures for state.
- Theme is window-owned. `guiTheme` and the `default*Style` package vars are a
  frame-scoped cache, never app state. `installTheme` refills them per theme
  change, and they have no initializers. An initializer makes the mirrors drift.
  The two exceptions are `DefaultTextStyle` and `defaultInspectorStyle`, which
  are `ThemeMaker` inputs. Key on `Theme.id`, never `Theme.Name`.
- Theme reads split by phase. Outside generation, call `w.Theme()`. Factories
  and `GenerateLayout` keep the bare `guiTheme`/`default*Style` read (required
  for `Themed` subtree scoping). Mark deliberate exceptions
  `ergonomics-audit:theme-global`.
- **Where to change:** to add widget state, use a new namespace const and a
  `StateMap`. To add a default style, change `ThemeMaker` only.
- **Catches:** `TestDefaultStylesMirrorThemeDark` runs. `make ergonomics-audit`
  mode `theme` gates the post-generation paths. Specs:
  `docs/specs/per-window-theme.md`, `docs/specs/theme-style-single-source.md`.

### Native platform boundaries

- Backends inject `TextMeasurer` (glyph metrics), `SvgParser`, and
  `NativePlatform` (dialogs, notifications, print, a11y, IME, titlebar, window
  opacity, frameless drag/resize) at startup. All are nil in tests — test code
  must never require a backend.
- `gui/` never imports backend packages. `NativePlatform` is the only seam to
  platform services.
- **Where to change:** a new platform capability is a new `NativePlatform`
  method implemented per backend, not a direct call from `gui/`.
- **Catches:** `make test` runs with nil injected interfaces. `make vet` (incl.
  the `requiredid` analyzer) flags structural mistakes.

### Window furniture

- `WindowCfg` carries the frame: `Transparent` (creation-time only, needs a
  compositor on X11), `BgColor` alpha (read every frame), `Min/MaxWidth/Height`
  (`FixedSize` pins both), and `Decorations` (frameless needs an app drag handle
  via `StartWindowDrag`/`StartWindowResize`). Opacity is the runtime setter
  `SetWindowOpacity` / `WindowOpacity`, refused on a transparent Windows window.
  State and placement are runtime calls too: `Minimize`, `Maximize`, `Restore`,
  `SetFullscreen`, `SetWindowSize`, `SetPosition`. Their native methods must not
  deliver events before they return, because gui calls them from event handlers.
  `Hide` / `Show` take the window off the screen and bring it back. `IsVisible`
  reports the app's own `Hide`, `IsOccluded` what the OS does (minimized, app in
  the background), and `EventVisibilityChanged` each change of the two together.
  Backends report the OS side through `gui.DispatchWindowOccluded`, on the UI
  thread. A platform that cannot hide a window has `CanHideWindow() bool`
  returning false, and `Hide` does nothing there. `KeepAwake` keeps the display
  on while a window with a request is on screen; gui counts the requests and
  calls `SetKeepAwake` only when the whole app's state changes.
- **Where to change:** `gui/window_cfg.go` for config, `gui/window_opacity.go`,
  `gui/window_decoration.go`, `gui/window_placement.go` and
  `gui/window_visibility.go` and `gui/keep_awake.go` for runtime, one
  `NativePlatform` method per capability, implemented per backend.
- **Catches:** `gui.Debug` (category `DebugWindowDegraded`) reports a refused
  feature. Specs: `docs/specs/transparent-windows.md`,
  `docs/specs/window-opacity.md`, `docs/specs/window-size-limits.md`,
  `docs/specs/frameless-windows.md`, `docs/specs/window-state.md`,
  `docs/specs/window-visibility-event.md`, `docs/specs/keep-awake.md`.

### Widget Cfg invariants

- **Focusable defaults:** 23 Cfgs are focusable by default. They are `Button`,
  `ColorChannelSlider`, `ColorPicker`, `ColorPlane`, `ColorWheel`, `Combobox`,
  `DataGrid`, `DatePicker`, `ExpandPanel`, `FlipPanel`, `Input`, `InputDate`,
  `ListBox`, `NumericInput`, `RadioButtonGroup`, `Radio`, `SegmentedControl`,
  `Select`, `Slider`, `Switch`, `Toggle`, `Tree`, `VirtualList`. Opt out with
  `FocusDisabled`, never `Focusable: false`. A focusable-by-default control with
  an empty `ID` takes a generated one and still joins the tab order; an opt-in
  `Focusable` without an `ID` is a silent no-op — the widget renders and clicks
  but never joins the tab order. Spec: `docs/specs/focusable-default-input.md`,
  `docs/specs/auto-widget-identity.md`.
- **A11y fields:** `A11YLabel`/`A11YDescription` live on the embedded `A11YCfg`.
  Never redeclare them on a Cfg. Construction names the embed:
  `gui.ButtonCfg{ID: "save", A11YCfg: gui.A11YCfg{A11YLabel: "Save"}}`.
- **`Opt[T]` vs plain fields:** only primitives get `Opt` (when zero is a
  legitimate user choice that must be distinguishable from unset, for example
  `SizeBorder`). Owned types self-flag instead: `Color`, `Padding`, `Sizing`,
  `Spacing`, `Radius` and `Border` carry a `set` field, so they are plain fields
  with `IsSet()`/`Or()`. Build them with constructors or roles —
  `RGBA`/`RGB`/`Hex`, `NewPadding`/`PadAll`, the predefined `Sizing` vars,
  `SpacingMedium`/`SpacingPx`, `RadiusMedium`/`RadiusPx`,
  `BorderThin`/`BorderPx` — never raw `Color{...}`/`Padding{...}`/`Sizing{...}`
  literals (they read as unset). A `Spacing`/`Radius`/`Border` role resolves
  against the active theme at build time, so a `ThemeCfg` step moves every call
  site that names it.
- **Where to change:** any widget factory or Cfg struct.
- **Catches:** `make vet` (`requiredid`) and `make ergonomics-audit` modes
  `focus`, `a11y`, `opt`, and `literals`. `gui.Debug` reports focusable shapes
  with no ID at runtime.

### Generated-file expectations

- `go generate ./...` produces `gui/svg_spinner_kinds_gen.go` (from
  `gui/internal/gen/spinnerkinds/`). Commit generated output. Never hand-edit
  it. Any change to the generator must be committed with its regenerated output,
  or `make generate-check` fails the push.
- **Where to change:** `gui/internal/gen/`, then run `go generate ./...`.
- **Catches:** `make generate-check` (part of `make check` and `prepush`) fails
  if `go generate ./...` produces a diff in `*_gen.go`.

### Validation map

| Invariant class                    | Local / CI gate                                                                                                                     |
| ---------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------- |
| Frame lifecycle, concurrency       | `make test-race`, `make bench-gate` (alloc gates)                                                                                   |
| Event consumption                  | `TestUnconsumedEvents`, `gui.Debug` (`DebugUnconsumed`), ergoaudit `callbacks`                                                      |
| Duplicate / mis-scoped IDs         | `TestDuplicateIDs`, `gui.Debug` (`DebugDuplicates`, `DebugUnresolvedKeys`, `DebugUnknownFocus`, `DebugStampDrift`), ergoaudit `ids` |
| Render command stream              | `gui/render_layout_test.go`, backend render tests                                                                                   |
| Backend build matrix, threading    | `make cross-compile`, `make lint-cross`, `make test` (`CGO_ENABLED=0` GL)                                                           |
| Theme single source                | `TestDefaultStylesMirrorThemeDark`, ergoaudit `theme`                                                                               |
| Native platform seam               | `make test` (nil interfaces), `make vet` (`requiredid`)                                                                             |
| Focusable + ID, a11y, Opt/literals | `make vet`, ergoaudit `focus`/`a11y`/`opt`/`literals`, `gui.Debug`                                                                  |
| Generated files                    | `make generate-check`                                                                                                               |
| Exported surface                   | `make export-audit` — see `docs/specs/exportaudit-surface-policy.md`                                                                |

## Future Directions

- **WebGPU**: Explored on the `webgpu-backend` branch (deleted) — 12 WGSL shader
  pipelines, device init, and render loop were working. Superseded: the native
  GL backend already runs cgo-free on Linux and Windows (X11 via xgb, EGL via
  purego). If adopted, WebGPU adds 10–40 MB of runtime shared libraries. The
  macOS backend is cgo-free too since #1323. Full assessment:
  `docs/specs/cgo-free-backend-feasibility.md`.
- **Native GL on desktop**: The native GL backend is the default renderer on
  Linux and Windows. It provides direct platform windowing (X11 on Linux, Win32
  on Windows) with EGL/WGL contexts — no intermediate library dependency.
