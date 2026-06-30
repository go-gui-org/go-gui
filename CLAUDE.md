# CLAUDE.md

Guidance for Claude Code (claude.ai/code) in this repo.

## Commands

```
go test ./...          # run all tests (headless, ~12s)
go test ./gui/... -run TestFoo  # run single test
go vet ./...           # static analysis
golangci-lint run ./...      # full lint: govet, staticcheck, errcheck, gocyclo, cyclop, modernize, unused, revive, gocritic, perfsprint, gosmopolitan (+ gofmt/goimports formatters)
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
  event dispatch, state mgmt (~400 non-test .go files at top level).
  **Keep flat: only leaf subsystems (svg/, datagrid/, markdown/,
  backend/, etc.) in subpackages.**
- `gui/backend/sdl2/` — SDL2 backend. Implements `TextMeasurer`, `SvgParser`,
  `NativePlatform`. Wires into window via `sdl2.New(w)`
- `gui/backend/metal/` — Metal backend (macOS)
- `gui/backend/gl/` — OpenGL backend; `web/`, `android/`, `ios/` — other
  platform backends
- `gui/backend/filedialog/`, `printdialog/`, `nativemenu/` — native dialogs/menus
- `gui/backend/atspi/`, `sni/`, `spellcheck/` — Linux a11y / tray / spelling
- `gui/backend/internal/` — shared backend internals
- (no test backend package; tests run with nil injected interfaces)
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

- `glyph` — text shaping/rendering lib. Consumed as versioned module; a
  `go.work` (`use (. ../go-glyph)`) points the local build at the working
  copy in `~/Documents/github/go-glyph`. No `replace` directive.
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

## Debugging native backends

### CGo boundary blindness

The LLM cannot trace values across the C↔Go boundary. A variable set in
ObjC (e.g. `_evType`) and read in Go (`C.metalEventType()`) is opaque —
the model can read the code on both sides but cannot verify the value
survives the crossing. When a bug sits on this boundary:

- Add debug logging on **both** sides simultaneously in the first pass.
  stderr in C (`fprintf(stderr, ...); fflush(stderr)`), stderr in Go
  (`fmt.Fprintf(os.Stderr, ...)` or `log.Printf`).
- If the log on one side never appears, the value was lost or the code
  path was never reached. This cuts diagnosis from hours to minutes.

### Platform knowledge asymmetry

The LLM knows Cocoa APIs in isolation but not their interactions with
AppKit internals (event masks, tracking areas, responder chain, run-loop
modes). Standard desktop behaviors that "just work" in a normal Cocoa
app require correct participation in these systems. When platform-native
behavior is broken (cursors, menus, title bar, window management):

- Ask for a **minimal native test program** (10-30 lines of ObjC) before
  modifying any go-gui code. Compare against a known-working variant
  (e.g. `[NSApp run]` vs custom event loop, plain NSView vs
  CAMetalLayer, `NSEventMaskAny` vs explicit mask). The test programs
  in `gui/backend/metal/test_*.m` (untracked, build with `clang
  -fobjc-arc -framework AppKit -framework Metal -framework QuartzCore`)
  were essential to isolating the event-mask and menu bugs.
- Ask "what would a Cocoa developer check first?" — the answer is
  usually event masks, tracking areas, or run-loop configuration, not
  application-level cursor or menu logic.

## Coding Conventions

- **No variable shadowing.** Never `:=` redeclare var from outer scope.
  Use `=` to assign existing var, or pick distinct name.
- Committed code must pass `golangci-lint run ./...` and `gofmt`.
  PostToolUse hook auto-runs lint-fix + tests on every .go edit.
- **Minimal scoped diffs.** Touch only what the request needs. No
  cosmetic comment/formatting churn, no drive-by edits to unrelated
  code. Rename/regex passes must not alter comment prose (e.g.
  apostrophes in possessives).

## Verification

- Rebuild AND run the relevant tests before claiming a fix works.
  Never report success on an unverified change. State failures plainly
  with the output; if a step was skipped, say so.
- Native/CGo or focus/activation bugs: confirm root cause with
  instrumented logging (evidence) before editing. Reproduce before,
  verify symptom gone after. Never leave the app non-launching. See
  "Debugging native backends" for the logging technique.
- Verify factual / root-cause claims against the code before asserting
  them. Don't state "go-glyph has a pure-Go path", "X is a table-version
  difference", etc. as fact without checking.

## CI signals

- Distinguish CI runner noise (CPU variance, ns/op jitter) from real
  regressions. Keep alloc gates hard; treat timing gates as advisory.

## Git Workflow

- Before reviewing or editing a branch, confirm it is rebased on the
  current base branch. If stale, update first — do not build work on a
  stale base.

## Release / Multi-repo

- Direct go-gui consumers (require a go-gui bump on release): **go-charts,
  go-edit, go-kite, go-map, go-term**. Verified against each repo's
  `go.mod` `require` (all `github.com/go-gui-org/*`). go-glyph is
  *upstream* — go-gui depends on it, not the reverse; never a bump target.
- On release, re-verify the list from `go.mod` files; don't rely on
  memory. New consumers get added without updating this note.

## context-mode

Routing rules injected by SessionStart hook. Use `ctx_batch_execute` /
`ctx_search` / `ctx_execute_file` for research. Bash only for short
git/mkdir/rm/ls output. `ctx_fetch_and_index` instead of curl/wget/WebFetch.

## Rejected Approaches

- **WebGPU Backend** (2026-06): Explored in branch `webgpu-backend` (deleted).
  12 WGSL shader pipelines, device init, render loop all working. Rejected
  because WebGPU has no native text rendering — font measurement and glyph
  rasterization require Canvas2D. A hybrid backend defeats the purpose; a
  pure-Go TTF rasterizer would be needed in go-glyph first. The existing
  Canvas2D backend already handles every render command correctly. GPU
  acceleration doesn't address the actual bottleneck (heap allocations).

## Specs
- Specs should be written to docs/specs folder.
