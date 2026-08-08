# CLAUDE.md

Guidance for Claude Code (claude.ai/code) in this repo.

## Commands

```
go run ./examples/get_started/  # run the example app
./scripts/large-files.sh     # report Go files >800 lines in gui/
```

## Architecture

Immediate-mode pipeline. No virtual DOM, no diffing:

```
View fn → generateViewLayout() → Layout tree
  → layoutArrange() (Fit/Fixed/Fill sizing)
  → renderLayout() (emits into w.renderers)
  → Backend (Metal on macOS; native GL on Linux/Windows)
```

### Packages

- **Keep flat: only leaf subsystems (svg/, datagrid/, markdown/, backend/, etc.)
  in subpackages.** `gui/` itself holds the core: widget factories, layout
  engine, theme, animation, event dispatch, state mgmt.
- (no test backend package; tests run with nil injected interfaces)

### Core Types

- `View` — interface (`Content() []View`, `GenerateLayout(*Window) Layout`).
  Widget factories return `View`, not `*Layout`; `*Layout` does NOT implement
  it. In tests, reach a widget's shape via `v.GenerateLayout(w).Shape`

### State Management

One typed slot per window. No globals, no closures. `gui.State[T](w)`
type-asserts and **panics** if the window holds a different type.

### Sizing

A `FillFill` root fills the window (min=max seed in `updateLayout`).

### Widgets

All widgets take `*Cfg` struct (zero-initializable). Event callbacks share sig
`func(EventCtx)`.

Focus requires **both** `Focusable: true` **and** a non-empty `ID`
(`isFocusedTarget`, `gui/event_traversal.go`); tab order additionally needs
`!FocusSkip && !Disabled` (`layout_query.go`). `Focusable: true` without an `ID`
is a silent no-op — the widget renders and clicks but never joins the tab order.
The `requiredid` analyzer flags this. IDs must be unique per window: menu items
are keyed by raw command ID, so `CommandButton` namespaces its auto-filled ID
with `cmdbtn:`.

#### `Opt[T]` vs plain fields

**Rule: `Opt[T]` when the zero value is a legitimate user choice that must be
distinguishable from "unset". A plain field everywhere else.**

`SizeBorder` is the worked example: a border width of `0` is something a caller
means, so a plain `float32` cannot tell "no border" from "not specified" and
silently applies the theme default. The same holds for `Padding`, `Radius`,
`Spacing`, `Opacity`, and enum fields whose zero is a real member
(`HAlignLeft == 0`). Where zero is not meaningful — most widths, heights,
counts, and indices — `Opt` costs a wrapper call and buys nothing.

Decide this when authoring the field, not by copying whichever neighbor was
nearest.

#### Colors on `Cfg` structs

Use plain `Color`, never `Opt[Color]`. `Color` carries its own `set` flag
(`gui/color.go`), so `Color{}` is unset and `ColorTransparent` is an explicit
fully-transparent choice — the distinction `Opt` would add already exists, and
wrapping gives the field two independent notions of unset.

`ColorSet` (`gui/color_set.go`) groups the per-state colors; `Flat(c)` is the
"keep one appearance" case. **Precedence: an assigned flat `Color*` field wins
over the `ColorSet`**, so existing code keeps its appearance when a set arrives.

### External Dependencies

- `glyph` — text shaping/rendering lib. Consumed as versioned module; a
  `go.work` (`use (. ../go-glyph)`) points the local build at the working copy
  in `~/Documents/github/go-glyph`. No `replace` directive. For text work, check
  glyph first. Only add new text routines when glyph lacks them.

### Injected Interfaces

Backend injects at startup. Nil in tests:

- `TextMeasurer` — glyph metrics for layout
- `SvgParser` — SVG parse + tessellate
- `NativePlatform` — native dialogs, notifications, print, a11y, IME, titlebar

### Key Implementation Notes

- `(*Layout).spacing()` counts only visible children (`ShapeType != ShapeNone`,
  `!Float`, `!OverDraw`). Fence-post gap calc
- Shape text fields in `Shape.TC` (`*ShapeTextConfig`), not on `Shape`
- `ContainerCfg.Title`/`TitleBG` render group-box label in top border (floating
  eraser + text, like HTML fieldset). `TitleBG` must match parent bg color to
  erase border behind title.
- `Children []Layout` = values. Parents = pointers. Avoids cycles
- `StateMap` (keyed by namespace consts like `nsOverflow`, `nsSvgCache`) =
  per-window typed kv store for widget internal state
- `AmendLayout` hook on shapes runs after sizing to reposition overlays (color
  picker circles, splitter handles, etc.) or manage hover. Layout uses absolute
  coords. Moving parent in `AmendLayout` does NOT move children. Use float
  system (`FloatAnchor`/`FloatTieOff`/`FloatOffset`) to position elements with
  children.
- **One event rule (since v0.55.0): nothing is marked handled for you. A
  callback that acts on an event calls `ctx.Consume()`; one that does not, lets
  the event travel on.** This holds for every callback — `OnClick` and `OnChar`
  no differently from `OnKeyDown` and `OnHover`. There is no `ctx.Bubble()`:
  declining is what silence already means. Before v0.55.0 the five hit-tested
  callbacks were "consume-class" and dispatch pre-marked their events, so an
  empty `OnClick` was a working click-blocker; such a handler now blocks nothing
  unless it consumes. `ctx.Event` is nil in `AmendLayout` and `OnScroll`; both
  `EventCtx` methods are nil-safe. `gui.Debug(true)` reports handlers that act
  without consuming while an ancestor also handles;
  `(*Window).TestUnconsumedEvents` sweeps a whole window for them

## Coding Conventions

- **No variable shadowing.** Never `:=` redeclare var from outer scope. Use `=`
  to assign existing var, or pick distinct name.
- Committed code must pass `golangci-lint run ./...` and `gofmt`. PostToolUse
  hook auto-runs lint-fix + tests on every .go edit.
- **Minimal scoped diffs.** Touch only what the request needs. No cosmetic
  comment/formatting churn, no drive-by edits to unrelated code. Rename/regex
  passes must not alter comment prose (e.g. apostrophes in possessives).

## Verification

- Rebuild AND run the relevant tests before claiming a fix works. Never report
  success on an unverified change. State failures plainly with the output; if a
  step was skipped, say so.
- Native/CGo or focus/activation bugs: confirm root cause with instrumented
  logging (evidence) before editing. Reproduce before, verify symptom gone
  after. Never leave the app non-launching. See `gui/backend/CLAUDE.md` for the
  two-sided logging technique.
- Verify factual / root-cause claims against the code before asserting them.
  Don't state "go-glyph has a pure-Go path", "X is a table-version difference",
  etc. as fact without checking.

## CI signals

- Distinguish CI runner noise (CPU variance, ns/op jitter) from real
  regressions. Keep alloc gates hard; treat timing gates as advisory.

## Git Workflow

- Before reviewing or editing a branch, confirm it is rebased on the current
  base branch. If stale, update first — do not build work on a stale base.

## context-mode

Routing rules injected by SessionStart hook. Use `ctx_batch_execute` /
`ctx_search` / `ctx_execute_file` for research. Bash only for short
git/mkdir/rm/ls output. `ctx_fetch_and_index` instead of curl/wget/WebFetch.

## Rejected Approaches

- **WebGPU Backend** (2026-06): Explored in branch `webgpu-backend` (deleted).
  12 WGSL shader pipelines, device init, render loop all working. Rejected at
  the time because WebGPU has no native text rendering — font measurement and
  glyph rasterization require Canvas2D. A hybrid backend defeats the purpose; a
  pure-Go TTF rasterizer in go-glyph was the missing piece. The existing
  Canvas2D backend already handles every render command correctly. GPU
  acceleration doesn't address the actual bottleneck (heap allocations).

  **Update (2026-07):** go-glyph now has a pure-Go text pipeline
  (`bitmap_puregoft.go` — go-text/typesetting harfbuzz shaping +
  golang.org/x/image/vector rasterization, no CGo). Combined with
  [goffi](https://github.com/go-webgpu/goffi) (zero-CGo FFI for calling
  wgpu-native), a CGo-free WebGPU desktop backend is now technically viable —
  blocked only by the upfront engineering cost of a full backend rewrite, not by
  any missing dependencies.

  **Superseded (2026-07-31, issue #137):** viable but the wrong instrument.
  `gui/backend/gl/` has no cgo at all — X11 via xgb, EGL via purego, Win32 via
  syscall. The sole CGo dependency on Linux/Windows is `github.com/go-gl/gl` (55
  functions, 45 constants), and its proc-address loader is already pure Go.
  Swapping that one dispatch layer for purego bindings gets CGo-free
  Linux+Windows for ~600 lines; wgpu is a superset that also ships 10–40 MB of
  runtime shared libs, supplies none of `NativePlatform` (10 sub-interfaces),
  and leaves macOS (the only real CGo backend, 5.9k lines ObjC) untouched. Full
  assessment: `docs/specs/cgo-free-backend-feasibility.md`.

  **Phase 1 done (2026-07-31):** go-gl replaced by `gui/backend/internal/glbind`
  (purego). `CGO_ENABLED=0` now builds the whole module for Windows, and
  `./gui/backend/...` for Linux. The remaining Linux CGo dependency was
  `gui/audio` → `ebitengine/oto` (ALSA), not the backend. macOS (Phase 2) is
  untouched.

  **Audio de-cgo'd (2026-08-04, issue #141):** `gui/audio`'s output driver is
  now a 3-function seam (`outputInit`/`outputPlay`/`outputClose`,
  `output_oto.go` vs `output_pulse.go`). The default Linux sink is a pure-Go
  PulseAudio client (`github.com/jfreymuth/pulse`), so
  `CGO_ENABLED=0 go build ./...` is green on Linux; oto/ALSA is opt-in via
  `-tags otoaudio`. Windows/macOS still use oto. beep decode/mix was already
  pure Go.

## Specs

- Specs should be written to docs/specs folder.
