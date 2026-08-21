# CLAUDE.md

Guidance for Claude Code (claude.ai/code) in this repo.

## Commands

```
go run ./examples/get_started/  # run the example app
make prepush                    # full pre-push gate (race, cross-lint, cross-compile, coverage, export audit)
make check-all                  # test + lint + vet gates (what .githooks/pre-push runs)
make check                      # fast subset (vet, deps-doc, large-files, generate-check, tidy-check, fmt-md-check)
make fmt-md                     # format every tracked .md with Prettier (.prettierrc holds the flags)
git config core.hooksPath .githooks  # enable tracked pre-commit/pre-push hooks
make test                       # tests only
make lint                       # golangci-lint (pinned version)
make vet                        # go vet + requiredid analyzer
make ergonomics-audit                 # focus/callbacks inventory + ID/a11y/theme/visual gates
make export-audit               # exported surface (advisory in-repo)
./scripts/large-files.sh        # report Go files >800 lines in gui/
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
`func(EventCtx)`. Worked examples and failure modes for every rule below:
`docs/dx-cheat-sheet.md`.

Focus requires **both** `Focusable` **and** a non-empty `ID` (`isFocusedTarget`,
`gui/event_traversal.go`); tab order additionally needs
`!FocusSkip && !Disabled` (`layout_query.go`). `Focusable` without an `ID` is a
silent no-op — the widget renders and clicks but never joins the tab order. The
`requiredid` analyzer flags this, and `gui.Debug` reports it at runtime.

**Input controls are focusable by default (v0.36.0); opt out with
`FocusDisabled`, never with `Focusable: false`.** Sixteen Cfgs default on:
`Button`, `ColorPicker`, `Combobox`, `DatePicker`, `Input`, `InputDate`,
`ListBox`, `NumericInput`, `RadioButtonGroup`, `Radio`, `Select`, `Slider`,
`Switch`, `Toggle`, `Tree`, `VirtualList`. Everything else is opt-in via
`Focusable: true`. See `docs/specs/focusable-default-input.md`; run
`ergonomics-audit -mode focus` for the current inventory.

**`Shape.ID` is a leaf; identity is the effective ID.** `resolveShapeIDs`
(`gui/id_resolve.go`, run from `layoutArrange`) stamps `Shape.effID` = the leaf
joined to the IDs of its **ID-bearing ancestors**, and every ID-keyed store and
lookup uses that — read `shape.idKey()`, never `shape.ID`, at a keying site.
Only explicit IDs join (never position, never child index); an ID-less container
adds no scope, and its children collide as loudly as before. A leaf already
containing `:` is **absolute** and is not joined again. Effective IDs must be
unique per window.

Public APIs (`SetFocus`, `FindByID`, `IsFocus`, `ScrollVerticalTo`, `Test*`)
take the **effective** ID. Two seams exist for widget code: `w.EffID(cfg.ID)`
for state read during `GenerateLayout` (the read decides the subtree, so it
cannot wait for the pass), and `ctx.EffID(leaf)` for handlers in factories that
build eagerly with no `Window`. Both go through `resolveLeaf`, so they cannot
drift from the pass. `gui/datagrid` is the documented exception — its child IDs
are absolute by design (reverse-parsed) and stay window-global. See
`docs/specs/widget-id-per-scope-uniqueness.md`.

**Compose inner IDs with `gui.ScopeID` / `gui.ScopeIDN`, never by hand.** An ID
is a `:`-joined path (`grid:header:name:resize`); the owner may itself be
composed, and composition is associative. `ScopeIDN` appends a numeric segment
without allocating for the number — use it for loop-derived identity. Both cost
exactly one allocation; `gui/id_scope_test.go` asserts that. A **part** (a row
key, a heading slug — a leaf value fed _into_ a composition) must not contain
`:` and keeps its own spelling; rebuilding an ID at a lookup site is how
producers and consumers drift. `make ergonomics-audit` (mode `ids`) fails on any
hand-rolled composition; see `docs/specs/widget-id-scoping.md`.

Uniqueness is strict, including within one widget: a composite widget's inner
shape that needs the owning widget's focus or spell-check state sets
`Shape.focusOwner` (a reference) instead of repeating its `ID` (an identity) —
see `Input`'s text shape and `Shape.focusKey()`. `(*Window).TestDuplicateIDs`
asserts a rendered window is clean.

#### Accessibility fields

**`A11YLabel` and `A11YDescription` live on the embedded `A11YCfg`
(`gui/a11y_cfg.go`); never redeclare them on a Cfg.** They are a perfect
co-occurrence group — every Cfg that carries one carries the other — so they are
declared once and embedded in all 35. Promotion keeps reads spelled unchanged
(`cfg.A11YLabel`), but Go has no promoted-field key in a composite literal, so
**construction names the embed**:

```go
gui.ButtonCfg{ID: "save", A11YCfg: gui.A11YCfg{A11YLabel: "Save"}}
```

`A11YCfg.a11yInfo(fallback)` builds the shape's `accessInfo` — it is the
`makeA11YInfo(a11yLabel(cfg.A11YLabel, X), cfg.A11YDescription)` pairing, and
staticcheck elides the embedded selector, so the call reads `cfg.a11yInfo(X)`.
Pass `""` where the widget has no content to derive a name from. `A11YRole` and
`A11YState` stay plain fields — only `ButtonCfg` and `ContainerCfg` carry them.

#### `Opt[T]` vs plain fields

**Rule: types the repo owns self-flag; only primitives get `Opt`.** `Opt[T]` is
for when the zero value is a legitimate user choice that must be distinguishable
from "unset" — and only when the type cannot carry that distinction itself.
`SizeBorder` is the canonical case: `0` is a border width a caller means, so a
plain `float32` cannot tell "no border" from "not specified". Where zero is not
meaningful — most widths, heights, counts, and indices — `Opt` costs a wrapper
call and buys nothing.

Owned struct types self-flag instead of wrapping: `Color` (`gui/color.go`),
`Padding` (`gui/padding.go`) and `Sizing` (`gui/sizing.go`) carry a `set` field,
so they are plain fields with `IsSet()`/`Or()` rather than `Opt[T]`. `Sizing`'s
zero value (FitFit) is a real combination, which is exactly why it flags: build
sizings with the predefined vars (`FitFit`…`FillFixed`), never a raw
`Sizing{...}` literal — that reads as unset (ergoaudit mode `literals` gates
this). Build `Padding` values with `NewPadding`/`PadAll`/`PaddingNone`; a raw
`Padding{...}` literal reads as unset (ergoaudit mode `literals` gates this).

Decide this when authoring the field, not by copying whichever neighbor was
nearest.

#### Colors on `Cfg` structs

Use plain `Color`, never `Opt[Color]`. `Color` carries its own `set` flag
(`gui/color.go`), so `Color{}` is unset and `ColorTransparent` is an explicit
fully-transparent choice — wrapping would give the field two notions of unset.
Build colors with `RGBA`/`RGB`/`Hex`; a raw `Color{...}` literal reads as unset
(ergoaudit mode `literals` gates this; the empty `Color{}` sentinel is exempt).

`ColorSet` (`gui/color_set.go`) groups the per-state colors; `Flat(c)` is the
"keep one appearance" case. **Precedence: an assigned flat `Color*` field wins
over the `ColorSet`**, so existing code keeps its appearance when a set arrives.

#### Visual roles and tiers

**Never spell a de-emphasis alpha, a label's size step, or a form control's text
inset at a call site.** Each has one named source; a literal there is the
divergence issue #335 measured and removed
(`docs/specs/widget-visual-consistency-audit.md`). The _when_ — which role each
decision takes, and when a deviation may carry the marker — is
`docs/style-guide.md`, enforced by `ergonomics-audit -mode visual`.

- **Quiet text** takes one of four `Theme` roles — `TextStyleSecondary`,
  `TextStyleLabel`, `TextStyleDisabled`, `TextStylePlaceholder`
  (`gui/theme_text_roles.go`). Use the role style directly where the text color
  is the theme's; use `withRoleAlpha(base, role)` where it is caller-supplied,
  which shares the _amount_ of de-emphasis without taking the hue. If none of
  the four fits, add a fifth role — not a local number.
- **Role values are per-theme and contrast-matched.** Alpha blends toward the
  background, so one multiplier cannot mean the same thing on a dark and a light
  ground. `textRolesFor` picks the ladder from the theme's own polarity;
  `ThemeCfg.ColorText*` overrides it.
- **`TextStyle.disabledRole`** marks a style that already expresses the disabled
  state so `renderText` skips `dimAlpha`. It is set only by `themeTextRoles` and
  must not be spelled at a call site. Without it, `layoutDisables`
  (`gui/layout.go`) — which stamps `Disabled` onto _every descendant_ of a
  disabled shape — halves a color the theme already quieted.
- **A form control's text inset** is `Theme.PaddingField`, so controls in one
  row share a height. Not the Small/Medium/Large ladder: those size the gap
  between things, this sizes a control.
- **A field's label** goes through `labelledField` (`gui/field_label.go`); a
  boolean control's through `trailingLabel`. Both exist so the placement is
  decided once.

A container reserves space for its border whether or not it paints one, so a
structural wrapper must set `SizeBorder: NoBorder` — an unset one inherits the
theme's container border and silently adds height.

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
- Shape text fields in `Shape.TC` (`*shapeTextConfig`), not on `Shape`
- `ContainerCfg.Title`/`TitleBG` render group-box label in top border (floating
  eraser + text, like HTML fieldset). `TitleBG` must match parent bg color to
  erase border behind title.
- `Children []Layout` = values. Parents = pointers. Avoids cycles
- `StateMap` (keyed by namespace consts like `nsOverflow`, `nsSvgCache`) =
  per-window typed kv store for widget internal state
- **Theme is window-owned; `guiTheme` and the `default*Style` mirrors are a
  frame-scoped cache, not app state.** `FrameFn` calls `w.installTheme()` before
  anything reads them, so a factory-time read resolves to the window being
  generated — that works only because every window's frame pass runs on the one
  main OS thread. `w.Theme()` reads, `w.SetTheme` pins that window, package
  `SetTheme` sets the app default for windows that never pinned one.
  `Themed(t, build)` scopes a theme to one subtree; its builder must run at
  generation time because factories resolve defaults when called. Anything keyed
  on a theme keys on `Theme.id`, never `Theme.Name` — names are not unique. See
  `docs/specs/per-window-theme.md`.
- **Theme reads split by phase, not by reachability. Code running _outside_
  generation with a `*Window` in hand calls `w.Theme()`; factories and
  `GenerateLayout` keep the bare `guiTheme` / `default*Style` read.** Outside
  generation the frame cache holds whichever window generated last — wrong as
  soon as there are two windows (issue #301 migrated 13 such sites). Inside
  generation the bare read is required, not tolerated: `Themed` scopes a theme
  by push/pop of the _installed_ theme, so `w.Theme()` there would ignore the
  scope. `make ergonomics-audit` mode `theme` gates the post-generation paths
  (`gui/backend/**`, `gui/scroll*.go`, `gui/event*.go`, `gui/native_*.go`,
  `gui/window_*.go`); mark a deliberate exception
  `ergonomics-audit:theme-global`.
- **`ThemeMaker` is the only source of default styling. The `default*Style`
  package vars have no initializers — never add one.** They are mirrors: `init`
  fills them with `applyTheme(ThemeDark)` and `installTheme` refills them per
  theme change. A literal there is a second source of truth that silently drifts
  (issue #300 removed ~30 of them; `ThemeDark` is bordered since 2026-08 — 90 of
  104 examples call `WithBorders(true)` explicitly — use
  `Theme.WithBorders(false)` for the old borderless look). The two exceptions
  are `DefaultTextStyle` and `defaultInspectorStyle`, which are ThemeMaker
  _inputs_. `TestDefaultStylesMirrorThemeDark` is the gate. See
  `docs/specs/theme-style-single-source.md`.
- `AmendLayout` hook on shapes runs after sizing to reposition overlays (color
  picker circles, splitter handles, etc.) or manage hover. Layout uses absolute
  coords. Moving parent in `AmendLayout` does NOT move children. Use float
  system (`FloatAnchor`/`FloatTieOff`/`FloatOffsetX`/`FloatOffsetY`) to position
  elements with children.
- **`AmendLayout` runs under the frame lock (`w.mu`), so no callback reached
  from it may call a window-mutating API.** `SetFocus`, `ClearFocus`,
  `UpdateView`, `ClearDrawCanvasCache` and `Window.Lock` all take `w.mu`, which
  is not reentrant, and the frame thread is the platform event loop — the app
  froze permanently (issue #394). Since the fix those APIs panic naming
  themselves instead of hanging; the remedy is `ctx.Window.QueueCommand`.
  Library code that reaches app code from the pass raises it with
  `deferCallback` and the pass runs it after unlocking
  (`gui/window_deferred.go`), which is how the Input's blur commit works:
  `OnTextCommit` with `InputCommitBlur`, `OnBlur` and a normalize-driven
  `OnTextChanged` therefore get a **nil `ctx.Layout`** — the tree is rebuilt
  from pooled arenas before they run. The Enter commit path dispatches from
  `EventFn` with no lock held and is unchanged. See
  `docs/specs/frame-lock-callback-deferral.md`.
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
  `(*Window).TestUnconsumedEvents` sweeps a whole window for them (see Dev-mode
  diagnostics below)

### Dev-mode diagnostics

**Most identity mistakes in this repo are already detected at runtime — turn the
gate on before hand-auditing a layout.** `gui.Debug(true)`, or `GOGUI_DEBUG=1`
in the environment, walks the composed tree every frame from
`updateLayoutLocked` and reports to stderr (`gui/debug.go`):

- duplicate **effective** ID — names the key and both claim sites
- focusable shape with no `ID` (never joins the tab order)
- scrollable shape with no `ID` (shares one offset with every other)
- `OnMouseLeave` with no `ID` (callback never fires)
- a scrollable listbox that resolved to height 0
- a container setting both `Wrap` and `Overflow` (wrap wins; issue #380)
- a callback that acted without `ctx.Consume()` while an ancestor also handles

Findings are warn-once per window, so a real defect does not scroll past at
frame rate. The gate allocates and walks the whole tree — dev only, never
production.

Categories gate independently via `gui.DebugCategories(mask)`;
`gui.DebugEnabled()` queries the gate. `Debug(true)` is
`DebugCategories(gui.DebugAll)`.

**`DebugUnscopedIDs` is not in `DebugAll` and `Debug(true)` does not turn it
on.** It reports an `ID` with no ID-bearing ancestor — still a window-global
name, so the widget cannot be dropped into a second panel as it stands. That is
a design property rather than a bug and fires on most widgets in a small app, so
ask for it explicitly when auditing a screen for reusability:

```go
gui.DebugCategories(gui.DebugAll | gui.DebugUnscopedIDs)
```

Assertable forms for tests, which return findings as data instead of printing:
`(*Window).TestDuplicateIDs` and `(*Window).TestUnconsumedEvents`.

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
- **Visual claims get recorded, not read.** `gui/golden_test.go` builds a
  widget, drives the real frame pipeline and diffs the emitted `[]RenderCmd`
  against `gui/testdata/`, in both `ThemeDark` and `ThemeLight`. Re-record with
  `go test ./gui/ -run TestGolden -update` **after reading the diff**. Reading
  source is not equivalent: a widget's `GenerateLayout` output is taken before
  `layoutDisables` and before arrange, so it shows neither inherited `Disabled`
  nor resolved geometry. Both mistakes were made and caught this way (issue
  #335). Add a case for any widget whose appearance a change can move; set
  `focusID` on it to record a focus state.
- After touching the exported surface of `gui/`, run `make export-audit`
  (tools/exportaudit): every export must be referenced from outside gui/ or
  carry a `// exportaudit:keep` marker. The consumer scan is authoritative; the
  in-repo run is advisory. `internal`-class exports (shared between gui/
  packages) are accepted by policy, not hard failures; the deferred list is in
  the gate advisory. See `docs/specs/exportaudit-surface-policy.md`.
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

- **WebGPU backend** — explored and rejected; superseded by issue #137. Don't
  re-propose it. `gui/backend/gl/` already has no cgo (X11 via xgb, EGL via
  purego, Win32 via syscall), so purego bindings got CGo-free Linux+Windows for
  ~600 lines, where wgpu would have added 10–40 MB of runtime libs, supplied
  none of `NativePlatform`, and left macOS untouched.
- **Current CGo state:** `CGO_ENABLED=0 go build ./...` is green on Linux and
  Windows for the whole module. macOS (5.9k lines ObjC) stays cgo **by
  decision** (2026-08-12) — the value argument inverts there; do not re-open
  without a trigger. See `docs/specs/cgo-free-backend-feasibility.md` § Phase 2.

Full history and rationale — WebGPU, the go-gl→glbind swap, the `gui/audio`
de-cgo — in `docs/specs/cgo-free-backend-feasibility.md`.

## Specs

- Specs should be written to docs/specs folder.
