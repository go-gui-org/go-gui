# CLAUDE.md

Guidance for Claude Code (claude.ai/code) in this repo.

## Commands

```
go run ./examples/get_started/  # run the example app
make prepush                    # full gate (race, cross-lint, cross-compile, coverage, export audit)
make check-all                  # test + lint + vet (what .githooks/pre-push runs)
make check                      # fast subset (vet, deps-doc, large-files, generate/tidy/fmt-md checks)
make test / lint / vet          # individually
make fmt-md                     # Prettier over tracked .md (.prettierrc holds the flags)
make ergonomics-audit           # focus/callbacks inventory + ID/a11y/theme/visual/literals gates
make export-audit               # exported surface (advisory in-repo)
./scripts/large-files.sh        # Go files >800 lines in gui/
git config core.hooksPath .githooks  # enable tracked hooks
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

**Keep flat: only leaf subsystems (svg/, datagrid/, markdown/, backend/, and
more) in subpackages.** `gui/` itself holds the core: widget factories, layout
engine, theme, animation, event dispatch, state mgmt. No test backend package;
tests run with nil injected interfaces.

### Core Types

`View` — interface (`Content() []View`, `GenerateLayout(*Window) Layout`).
Widget factories return `View`, not `*Layout`; `*Layout` does NOT implement it.
In tests, reach a widget's shape via `v.GenerateLayout(w).Shape`.

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
silent no-op. The `requiredid` analyzer flags it; `gui.Debug` reports it at
runtime.

**Input controls are focusable by default; opt out with `FocusDisabled`, never
with `Focusable: false`.** Sixteen input Cfgs default on (Button, Input, Select,
Slider, Tree, …); everything else is opt-in via `Focusable: true`. Current
inventory: `ergonomics-audit -mode focus`. See
`docs/specs/focusable-default-input.md`.

**`Shape.ID` is a leaf; identity is the effective ID.** `resolveShapeIDs`
(`gui/id_resolve.go`, run from `layoutArrange`) stamps `Shape.effID` = the leaf
joined to the IDs of its **ID-bearing ancestors**. Read `shape.idKey()`, never
`shape.ID`, at a keying site. Only explicit IDs join (never position, never
child index); an ID-less container adds no scope. A leaf already containing `:`
is **absolute** and is not joined again. Effective IDs must be unique per
window.

Public APIs (`SetFocus`, `FindByID`, `IsFocus`, `ScrollVerticalTo`, `Test*`)
take the **effective** ID. Two seams for widget code: `w.EffID(cfg.ID)` for
state read during `GenerateLayout`, and `ctx.EffID(leaf)` for handlers in
factories that build eagerly with no `Window`. `gui/datagrid` is the documented
exception — its child IDs are absolute by design and stay window-global. See
`docs/specs/widget-id-per-scope-uniqueness.md`.

**Compose inner IDs with `gui.ScopeID` / `gui.ScopeIDN`, never by hand.** An ID
is a `:`-joined path (`grid:header:name:resize`); composition is associative.
`ScopeIDN` appends a numeric segment without allocating for the number — use it
for loop-derived identity. A **part** (a row key, a heading slug — a leaf value
fed _into_ a composition) must not contain `:` and keeps its own spelling; never
rebuild an ID at a lookup site. `ergonomics-audit` mode `ids` fails on
hand-rolled composition; see `docs/specs/widget-id-scoping.md`.

Uniqueness is strict, including within one widget: a composite widget's inner
shape that needs the owning widget's focus or spell-check state sets
`Shape.focusOwner` (a reference) instead of repeating its `ID` (an identity) —
see `Input`'s text shape and `Shape.focusKey()`. `(*Window).TestDuplicateIDs`
asserts a rendered window is clean.

#### Accessibility fields

**`A11YLabel` and `A11YDescription` live on the embedded `A11YCfg`
(`gui/a11y_cfg.go`); never redeclare them on a Cfg.** Reads stay spelled
`cfg.A11YLabel`, but construction must name the embed:

```go
gui.ButtonCfg{ID: "save", A11YCfg: gui.A11YCfg{A11YLabel: "Save"}}
```

`cfg.a11yInfo(fallback)` builds the shape's `accessInfo`; pass `""` where the
widget has no content to derive a name from. `A11YRole` and `A11YState` stay
plain fields — only `ButtonCfg` and `ContainerCfg` carry them.

#### `Opt[T]` vs plain fields

**Rule: types the repo owns self-flag; only primitives get `Opt`.** `Opt[T]` is
for a primitive whose zero value is a legitimate user choice that must be
distinguishable from "unset" — `SizeBorder` is the canonical case. Where zero is
not meaningful (most widths, heights, counts, indices) `Opt` buys nothing.

`Color` (`gui/color.go`), `Padding` (`gui/padding.go`) and `Sizing`
(`gui/sizing.go`) carry a `set` field, so they are plain fields with
`IsSet()`/`Or()`. Build them with the constructors — predefined sizing vars
(`FitFit`…`FillFixed`), `NewPadding`/`PadAll`/`PaddingNone`, `RGBA`/`RGB`/`Hex`.
A raw `Sizing{...}` / `Padding{...}` / `Color{...}` literal reads as unset
(ergoaudit mode `literals` gates this; the empty `Color{}` sentinel is exempt).

Decide this when authoring the field, not by copying whichever neighbor was
nearest.

#### Colors on `Cfg` structs

Plain `Color`, never `Opt[Color]`: `Color{}` is unset and `ColorTransparent` is
an explicit fully-transparent choice. `ColorSet` (`gui/color_set.go`) groups the
per-state colors; `Flat(c)` is the "keep one appearance" case. **Precedence: an
assigned flat `Color*` field wins over the `ColorSet`.**

#### Visual roles and tiers

**Never spell a de-emphasis alpha, a label's size step, or a form control's text
inset at a call site.** Each has one named source. Which role each decision
takes is `docs/style-guide.md`, enforced by `ergonomics-audit -mode visual`;
background in `docs/specs/widget-visual-consistency-audit.md`.

- **Quiet text** takes one of four `Theme` roles — `TextStyleSecondary`,
  `TextStyleLabel`, `TextStyleDisabled`, `TextStylePlaceholder`
  (`gui/theme_text_roles.go`). Use the role style directly where the text color
  is the theme's; use `withRoleAlpha(base, role)` where it is caller-supplied.
  If none of the four fits, add a fifth role — not a local number.
- **Role values are per-theme and contrast-matched.** `textRolesFor` picks the
  ladder from the theme's own polarity; `ThemeCfg.ColorText*` overrides it.
- **`TextStyle.disabledRole`** marks a style that already expresses the disabled
  state so `renderText` skips `dimAlpha`. Set only by `themeTextRoles`; never at
  a call site — `layoutDisables` stamps `Disabled` onto _every descendant_ of a
  disabled shape and halves a color the theme already quieted.
- **A form control's text inset** is `Theme.PaddingField`, so controls in one
  row share a height. Not the Small/Medium/Large ladder: those size the gap
  between things, this sizes a control.
- **A field's label** goes through `labelledField` (`gui/field_label.go`); a
  boolean control's through `trailingLabel`.

A container reserves space for its border whether or not it paints one, so a
structural wrapper must set `SizeBorder: NoBorder` — an unset one inherits the
theme's container border and silently adds height.

### External Dependencies

`glyph` — text shaping/rendering lib. Consumed as a versioned module; a
`go.work` (`use (. ../go-glyph)`) points the local build at
`~/Documents/github/go-glyph`. No `replace` directive. For text work, check
glyph first; only add new text routines when glyph lacks them.

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

#### Theme

- **Theme is window-owned; `guiTheme` and the `default*Style` mirrors are a
  frame-scoped cache, not app state.** `FrameFn` calls `w.installTheme()` before
  anything reads them. `w.Theme()` reads, `w.SetTheme` pins that window, package
  `SetTheme` sets the app default. `Themed(t, build)` scopes a theme to one
  subtree; its builder must run at generation time. Key anything theme-dependent
  on `Theme.id`, never `Theme.Name` — names are not unique. See
  `docs/specs/per-window-theme.md`.
- **Theme reads split by phase, not by reachability. Code running _outside_
  generation with a `*Window` in hand calls `w.Theme()`; factories and
  `GenerateLayout` keep the bare `guiTheme` / `default*Style` read** — `Themed`
  scopes by push/pop of the _installed_ theme, so `w.Theme()` there ignores the
  scope. `ergonomics-audit` mode `theme` gates the post-generation paths
  (`gui/backend/**`, `gui/scroll*.go`, `gui/event*.go`, `gui/native_*.go`,
  `gui/window_*.go`); mark a deliberate exception
  `ergonomics-audit:theme-global`.
- **`ThemeMaker` is the only source of default styling. The `default*Style`
  package vars have no initializers — never add one.** They are mirrors filled
  by `init`/`installTheme`. Exceptions: `DefaultTextStyle` and
  `defaultInspectorStyle` are ThemeMaker _inputs_. `ThemeDark` is bordered; use
  `Theme.WithBorders(false)` for the borderless look.
  `TestDefaultStylesMirrorThemeDark` is the gate. See
  `docs/specs/theme-style-single-source.md`.

#### Layout hooks and the frame lock

- `AmendLayout` runs after sizing to reposition overlays (color picker circles,
  splitter handles) or manage hover. Layout uses absolute coords. Moving a
  parent in `AmendLayout` does NOT move children — use the float system
  (`FloatAnchor`/`FloatTieOff`/`FloatOffsetX`/`FloatOffsetY`) to position
  elements with children.
- **`AmendLayout` runs under the frame lock (`w.mu`), so no callback reached
  from it can call a window-mutating API.** `SetFocus`, `ClearFocus`,
  `UpdateView`, `ClearDrawCanvasCache` and `Window.Lock` all take `w.mu`, which
  is not reentrant; they panic naming themselves. The remedy is
  `ctx.Window.QueueCommand`. Library code reaching app code from the pass raises
  it with `deferCallback` (`gui/window_deferred.go`), which is how Input's blur
  commit works — so `OnTextCommit` with `InputCommitBlur`, `OnBlur` and a
  normalize-driven `OnTextChanged` get a **nil `ctx.Layout`**. The Enter commit
  path dispatches from `EventFn` with no lock held. See
  `docs/specs/frame-lock-callback-deferral.md`.

#### Event consumption

**Nothing is marked handled for you. A callback that acts on an event calls
`ctx.Consume()`; one that does not, lets the event travel on.** This holds for
every callback — `OnClick` and `OnChar` no differently from `OnKeyDown` and
`OnHover`. There is no `ctx.Bubble()`: declining is what silence already means.
An empty `OnClick` blocks nothing. `ctx.Event` is nil in `AmendLayout` and
`OnScroll`; both `EventCtx` methods are nil-safe.

### Dev-mode diagnostics

**Most identity mistakes in this repo are already detected at runtime — turn the
gate on before hand-auditing a layout.** `gui.Debug(true)`, or `GOGUI_DEBUG=1`,
walks the composed tree every frame and reports to stderr (`gui/debug.go`):

- duplicate **effective** ID — names the key and both claim sites
- focusable shape with no `ID` (never joins the tab order)
- scrollable shape with no `ID` (shares one offset with every other)
- `OnMouseLeave` with no `ID` (callback never fires)
- a scrollable listbox that resolved to height 0
- a container setting both `Wrap` and `Overflow` (wrap wins)
- a callback that acted without `ctx.Consume()` while an ancestor also handles

Findings are warn-once per window. The gate allocates and walks the whole tree —
dev only, never production. Categories gate independently via
`gui.DebugCategories(mask)`; `gui.DebugEnabled()` queries the gate.

**`DebugUnscopedIDs` is not in `DebugAll`.** It reports an `ID` with no
ID-bearing ancestor — a window-global name, so the widget cannot be dropped into
a second panel as it stands. A design property rather than a bug, so ask for it
explicitly when auditing a screen for reusability:

```go
gui.DebugCategories(gui.DebugAll | gui.DebugUnscopedIDs)
```

Assertable forms for tests, which return findings as data:
`(*Window).TestDuplicateIDs` and `(*Window).TestUnconsumedEvents`.

## Coding Conventions

- **No variable shadowing.** Never `:=` redeclare a var from an outer scope. Use
  `=`, or pick a distinct name.
- Committed code must pass `golangci-lint run ./...` and `gofmt`. A PostToolUse
  hook auto-runs lint-fix + tests on every .go edit.
- **Minimal scoped diffs.** Touch only what the request needs. No cosmetic
  comment/formatting churn, no drive-by edits. Rename/regex passes must not
  alter comment prose (for example, apostrophes in possessives).

## Verification

- Rebuild AND run the relevant tests before claiming a fix works. Never report
  success on an unverified change. State failures plainly with the output; if a
  step was skipped, say so.
- **Visual claims get recorded, not read.** `gui/golden_test.go` builds a
  widget, drives the real frame pipeline and diffs the emitted `[]RenderCmd`
  against `gui/testdata/`, in both `ThemeDark` and `ThemeLight`. Re-record with
  `go test ./gui/ -run TestGolden -update` **after reading the diff**. Reading
  source is not equivalent: `GenerateLayout` output is taken before
  `layoutDisables` and before arrange, so it shows neither inherited `Disabled`
  nor resolved geometry. Add a case for any widget whose appearance a change can
  move; set `focusID` on it to record a focus state.
- After touching the exported surface of `gui/`, run `make export-audit`: every
  export must be referenced from outside gui/ or carry a `// exportaudit:keep`
  marker. The consumer scan is authoritative; the in-repo run is advisory.
  `internal`-class exports are accepted by policy. See
  `docs/specs/exportaudit-surface-policy.md`.
- Native/CGo or focus/activation bugs: confirm root cause with instrumented
  logging (evidence) before editing. Reproduce before, verify symptom gone
  after. Never leave the app non-launching. See `gui/backend/CLAUDE.md` for the
  two-sided logging technique.
- Verify factual / root-cause claims against the code before asserting them.

## CI signals

Distinguish CI runner noise (CPU variance, ns/op jitter) from real regressions.
Keep alloc gates hard; treat timing gates as advisory.

## Git Workflow

Before reviewing or editing a branch, confirm it is rebased on the current base
branch. If stale, update first — do not build work on a stale base.

## context-mode

Routing rules injected by SessionStart hook. Use `ctx_batch_execute` /
`ctx_search` / `ctx_execute_file` for research. Bash only for short
git/mkdir/rm/ls output. `ctx_fetch_and_index` instead of curl/wget/WebFetch.

## Rejected Approaches

- **WebGPU backend** — explored and rejected. Don't re-propose it.
  `gui/backend/gl/` already has no cgo (X11 via xgb, EGL via purego, Win32 via
  syscall).
- **Current CGo state:** `CGO_ENABLED=0 go build ./...` is green on Linux and
  Windows for the whole module. macOS (5.9k lines ObjC) stays cgo **by
  decision** (2026-08-12); do not re-open without a trigger.

Full history and rationale in `docs/specs/cgo-free-backend-feasibility.md`.

## Specs

Specs go in `docs/specs/`.
