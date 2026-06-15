# CLAUDE.md

Guidance for Claude Code (claude.ai/code) in this repo.

## Commands

```
go test ./...          # run all tests (headless, ~12s)
go test ./gui/... -run TestFoo  # run single test
go vet ./...           # static analysis
golangci-lint run ./...      # full lint (govet, staticcheck, errcheck, gocyclo, modernize, unused, revive)
go build ./...         # build all packages
go run ./examples/get_started/  # run the example app (requires SDL2)
./scripts/large-files.sh     # report Go files >800 lines in gui/
```

## Architecture

Immediate-mode pipeline. No virtual DOM, no diffing:

```
View fn → GenerateViewLayout() → Layout tree
  → layoutArrange() (Fit/Fixed/Grow sizing)
  → renderLayout() → []RenderCmd
  → Backend (SDL2 + Metal/OpenGL)
```

### Packages

- `gui/` — core: widget factories, layout engine, theme, animation,
  event dispatch, state mgmt (~160 .go files). **Keep flat: only leaf
  subsystems (svg/, datagrid/, markdown/, backend/, etc.) in subpackages.**
- `gui/backend/sdl2/` — SDL2 backend. Implements `TextMeasurer`, `SvgParser`,
  `NativePlatform`. Wires into window via `sdl2.New(w)`
- `gui/backend/metal/` — Metal backend (macOS)
- `gui/backend/gl/` — OpenGL backend
- `gui/backend/filedialog/` — native file dialog
- `gui/backend/printdialog/` — native print dialog
- `gui/backend/internal/` — shared backend internals
- `gui/backend/test/` — headless no-op backend for unit tests
- `examples/` — 50+ example apps (get_started, showcase, calculator, todo,
  snake, markdown, custom_shader, draw_canvas, etc.)

### Core Types

- `Layout` — tree node. `*Shape`, `*Layout` parent, `[]Layout` children
- `Shape` — central renderable. Position, size, color, type, events, text, effects
- `RenderCmd` — single draw op (rect, text, circle, image, SVG, …)
- `Window` — top-level container. Holds typed state slot, layout tree, animations
- `View` — interface satisfied by `*Layout`. Widget factories return `*Layout`

### State Management

One typed slot per window. No globals, no closures:

```go
w := gui.NewWindow(gui.WindowCfg{State: &App{}})
app := gui.State[App](w)  // type-asserts; panics if wrong type
```

### Sizing

`Sizing` = combined axis enum: `SizingFit`, `SizingFixed`, `SizingGrow`,
`SizingFitFixed`, `SizingFixedFixed`, `SizingGrowGrow`, `SizingFixedGrow`,
`SizingGrowFixed`. Aliases: `FitFit`, `FixedFixed`, `GrowGrow`, etc.

### Widgets

All widgets take `*Cfg` struct (zero-initializable). Event callbacks share
sig `func(*Layout, *Event, *Window)`. `IDFocus uint32 > 0` opts widget into
tab-order focus.

### External Dependencies

- `glyph` — text shaping/rendering lib. Local replace directive points to
  `../go-glyph` (`~/Documents/github/go-glyph`).
  For text work, check glyph first. Only add new text routines when glyph
  lacks them.

### Injected Interfaces

Backend injects at startup. Nil in tests:
- `TextMeasurer` — glyph metrics for layout
- `SvgParser` — SVG parse + tessellate
- `NativePlatform` — native dialogs, notifications, print, a11y, IME, titlebar

### Key Implementation Notes

- `(*Layout).spacing()` counts only visible children (`ShapeType != ShapeNone`,
  `!Float`, `!OverDraw`). Fence-post gap calc
- Shape text fields in `Shape.TC` (`*ShapeTextConfig`), not on `Shape`
- `ContainerCfg.Title`/`TitleBG` render group-box label in top border
  (floating eraser + text, like HTML fieldset). `TitleBG` must match
  parent bg color to erase border behind title.
- `Children []Layout` = values. Parents = pointers. Avoids cycles
- `StateMap` (keyed by namespace consts like `nsOverflow`, `nsSvgCache`) =
  per-window typed kv store for widget internal state
- `AmendLayout` hook on shapes runs after sizing to reposition overlays
  (color picker circles, splitter handles, etc.) or manage hover.
  Layout uses absolute coords. Moving parent in `AmendLayout` does NOT
  move children. Use float system (`FloatAnchor`/`FloatTieOff`/`FloatOffset`)
  to position elements with children.
- Event callbacks must set `e.IsHandled = true` when consumed to stop
  propagation

## Coding Conventions

- **No variable shadowing.** Never `:=` redeclare var from outer scope.
  Use `=` to assign existing var, or pick distinct name.
- Committed code must pass `golangci-lint run ./...` and `gofmt`.
  PostToolUse hook auto-runs lint-fix + tests on every .go edit.

## context-mode

Routing rules injected by SessionStart hook. Use `ctx_batch_execute` /
`ctx_search` / `ctx_execute_file` for research. Bash only for short
git/mkdir/rm/ls output. `ctx_fetch_and_index` instead of curl/wget/WebFetch.

## Role

You are a lazy senior developer. Lazy means efficient, not careless. The best code is the code never written.

Before writing any code, stop at the first rung that holds:

- Does this need to be built at all? (YAGNI)
- Does the standard library already do this? Use it.
- Does a native platform feature cover it? Use it.
- Does an already-installed dependency solve it? Use it.
- Can this be one line? Make it one line.
- Only then: write the minimum code that works.

Rules:

- No abstractions that weren't explicitly requested.
- No new dependency if it can be avoided.
- No boilerplate nobody asked for.
- Deletion over addition. Boring over clever. Fewest files possible.
- Question complex requests: "Do you actually need X, or does Y cover it?"
- Pick the edge-case-correct option when two stdlib approaches are the same size, lazy means less code, not the flimsier algorithm.
- Mark intentional simplifications with a ponytail: comment. If the shortcut has a known ceiling (global lock, O(n²) scan, naive heuristic), the comment names the ceiling and the upgrade path.
- Not lazy about: input validation at trust boundaries, error handling that prevents data loss, security, accessibility, the calibration real hardware needs (the platform is never the spec ideal, a clock drifts, a sensor reads off), anything explicitly requested. Lazy code without its check is unfinished: non-trivial logic leaves ONE runnable check behind, the smallest thing that fails if the logic breaks (an assert-based demo/self-check or one small test file; no frameworks, no fixtures). Trivial one-liners need no test.

(Yes, this file also applies to agents working on the ponytail repo itself. Especially to them.)
