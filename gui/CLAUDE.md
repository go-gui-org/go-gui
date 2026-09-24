# gui/CLAUDE.md

Rules for code under `gui/`: widgets, identity, theme, events and debug
categories. The root `CLAUDE.md` has a short form of the API-facing rules. This
file has the full rules and the internals behind them.

## Widgets

All widgets take a `*Cfg` struct (zero-initializable). Event callbacks share the
signature `func(EventCtx)`. Worked examples and failure modes for every rule
below: `docs/dx-cheat-sheet.md`.

### Focus

Focus requires **both** `Focusable` **and** a non-empty `ID` (`isFocusedTarget`,
`gui/event_traversal.go`); tab order additionally needs
`!FocusSkip && !Disabled` (`layout_query.go`). `Focusable` without an `ID` is a
silent no-op — the `requiredid` analyzer flags it, `gui.Debug` reports it at
runtime.

**Input controls are focusable by default; opt out with `FocusDisabled`, never
with `Focusable: false`.** Twenty Cfgs default on (Button, Input, Select,
Slider, Tree, Combobox, DatePicker, ListBox, …); everything else is opt-in. Full
list in `docs/architecture.md`, current inventory from
`ergonomics-audit -mode focus`. See `docs/specs/focusable-default-input.md`.

### Identity

**`Shape.ID` is a leaf; identity is the effective ID.** Layout _generation_
stamps `Shape.effID` = the leaf joined to its **ID-bearing ancestors**:
`generateViewLayout` stamps what a view returned, `appendChildViews` stamps a
parent before pushing that parent's scope (`gui/view.go`, `stampEffID` in
`gui/id_resolve.go`). Nothing re-derives identity afterwards, so there is
nothing to drift against; an unstamped shape is reported by `DebugStampDrift`.
Read `shape.idKey()`, never `shape.ID`, at a keying site. Only explicit IDs join
— never position, never child index; an ID-less container adds no scope. A leaf
already containing `:` is **absolute**. Effective IDs are unique per window,
strictly, including within one widget.

Public APIs (`SetFocus`, `FindByID`, `IsFocus`, `ScrollVerticalTo`, `Test*`)
take the **effective** ID. Two seams for widget code: `w.EffID(cfg.ID)` for
state read during `GenerateLayout`, `ctx.EffID(leaf)` for handlers in factories
that build eagerly with no `Window`. `gui/datagrid` is the documented exception
— it spells child IDs absolutely from the grid's own resolved ID, because
`dataGridHeaderColIDFromLayoutID` recovers a column by trimming that prefix
(#519). See `docs/specs/widget-id-per-scope-uniqueness.md`.

**Read an effective ID back rather than spelling it.** `w.ResolveID("nav")`
returns what the last frame stamped (`["detail:nav"]`); `w.EffectiveIDs()` lists
the frame. Both read the arranged tree, so they answer what the addressing APIs
take (#521).

**Compose with `gui.ScopeID` / `gui.ScopeIDN`, never by hand.** An ID is a
`:`-joined path (`grid:header:name:resize`); composition is associative.
`ScopeIDN` appends a numeric segment without allocating — use it for
loop-derived identity. A **part** (a row key, a heading slug — a leaf fed _into_
a composition) must not contain `:` and keeps its own spelling; never rebuild an
ID at a lookup site. `ergonomics-audit -mode ids` fails on hand-rolled
composition; see `docs/specs/widget-id-scoping.md`.

A composite widget's inner shape that needs the owning widget's focus or
spell-check state sets `Shape.focusOwner` (a reference) instead of repeating its
`ID` (an identity) — see `Input`'s text shape and `Shape.focusKey()`. That
reference is the one identity generation cannot stamp, because it names an
ancestor by leaf: `resolveFocusOwners` (from `layoutArrange`) rewrites it in
place. `(*Window).TestDuplicateIDs` asserts a rendered window is clean.

### Accessibility fields

**`A11YLabel` and `A11YDescription` live on the embedded `A11YCfg`
(`gui/a11y_cfg.go`); never redeclare them on a Cfg.** Reads stay
`cfg.A11YLabel`, but construction names the embed:

```go
gui.ButtonCfg{ID: "save", A11YCfg: gui.A11YCfg{A11YLabel: "Save"}}
```

`cfg.a11yInfo(fallback)` builds the shape's `accessInfo`; pass `""` where there
is no content to derive a name from. `A11YRole` and `A11YState` stay plain
fields — only `ButtonCfg` and `ContainerCfg` carry them.

### `Opt[T]`, colors, and literals

**Types the repo owns self-flag; only primitives get `Opt`.** `Opt[T]` is for a
primitive whose zero value is a legitimate choice that must be distinguishable
from "unset" — `SizeBorder` is the canonical case. Where zero is not meaningful
(most widths, heights, counts, indices) `Opt` buys nothing. Decide when
authoring the field, not by copying the nearest neighbor.

`Color` (`gui/color.go`), `Padding` (`gui/padding.go`) and `Sizing`
(`gui/sizing.go`) carry a `set` field, so they are plain fields with
`IsSet()`/`Or()`. Build them with the constructors — `FitFit`…`FillFixed`,
`NewPadding`/`PadAll`/`PaddingNone`, `RGBA`/`RGB`/`Hex`. A raw `Sizing{...}` /
`Padding{...}` / `Color{...}` literal reads as unset
(`ergonomics-audit -mode literals`; the empty `Color{}` sentinel is exempt).

On Cfg structs use plain `Color`, never `Opt[Color]`: `Color{}` is unset and
`ColorTransparent` is an explicit fully-transparent choice. `ColorSet`
(`gui/color_set.go`) groups per-state colors; `Flat(c)` is the "one appearance"
case. **`Colors` is the only spelling for per-state colors; no Cfg has
`ColorHover`/`ColorFocus`/`ColorBorder`/`ColorBorderFocus` fields.** The flat
`Color` is the shorthand for `Colors.Base` and wins over it. Setting
`Colors.Border` alone also pins `BorderFocus` to it (`ColorSet.resolve`); spell
`BorderFocus` too when the theme's focus border should stay.

### Visual roles and tiers

**Never spell a de-emphasis alpha, a label's size step, or a form control's text
inset at a call site.** Each has one named source. Which role each decision
takes is `docs/style-guide.md`, enforced by `ergonomics-audit -mode visual`;
background in `docs/specs/widget-visual-consistency-audit.md`.

- **Quiet text** takes one of four `Theme` roles — `TextStyleSecondary`,
  `TextStyleLabel`, `TextStyleDisabled`, `TextStylePlaceholder`
  (`gui/theme_text_roles.go`). Use the role style directly where the text color
  is the theme's; `withRoleAlpha(base, role)` where it is caller-supplied. If
  none of the four fits, add a fifth role — not a local number.
- **Role values are per-theme and contrast-matched.** `textRolesFor` picks the
  ladder from the theme's polarity; `ThemeCfg.ColorText*` overrides it.
- **`TextStyle.disabledRole`** marks a style that already expresses the disabled
  state so `renderText` skips `dimAlpha`. Set only by `themeTextRoles` — never
  at a call site, because `layoutDisables` stamps `Disabled` onto _every_
  descendant and would halve a color the theme already quieted.
- **A form control's text inset** is `Theme.PaddingField`, so controls in one
  row share a height. Not the Small/Medium/Large ladder: those size the gap
  between things, this sizes a control.
- **A field's label** goes through `labelledField` (`gui/field_label.go`); a
  boolean control's through `trailingLabel`.

A container reserves space for its border whether or not it paints one, so a
structural wrapper must set `SizeBorder: NoBorder` — an unset one inherits the
theme's container border and silently adds height.

## Theme

- **Theme is window-owned; `guiTheme` and the `default*Style` mirrors are a
  frame-scoped cache, not app state.** `FrameFn` calls `w.installTheme()` before
  anything reads them. `w.Theme()` reads, `w.SetTheme` pins that window, package
  `SetTheme` sets the app default. `Themed(t, build)` scopes a theme to one
  subtree; its builder must run at generation time. Key anything theme-dependent
  on `Theme.id`, never `Theme.Name` — names are not unique.
  (`docs/specs/per-window-theme.md`)
- **Theme reads split by phase, not by reachability.** Code running _outside_
  generation with a `*Window` calls `w.Theme()`; factories and `GenerateLayout`
  keep the bare `guiTheme` / `default*Style` read — `Themed` scopes by push/pop
  of the _installed_ theme, so `w.Theme()` there ignores the scope.
  `ergonomics-audit -mode theme` gates the post-generation paths
  (`gui/backend/**`, `gui/scroll*.go`, `gui/event*.go`, `gui/native_*.go`,
  `gui/window_*.go`); mark a deliberate exception
  `ergonomics-audit:theme-global`.
- **`ThemeMaker` is the only source of default styling. The `default*Style`
  package vars have no initializers — never add one.** They are mirrors filled
  by `init`/`installTheme`. Exceptions: `DefaultTextStyle` and
  `defaultInspectorStyle` are ThemeMaker _inputs_. `ThemeDark` is bordered; use
  `Theme.WithBorders(false)` for the borderless look.
  `TestDefaultStylesMirrorThemeDark` is the gate.
  (`docs/specs/theme-style-single-source.md`)

## Layout hooks and the frame lock

`AmendLayout` runs after sizing to reposition overlays (color picker circles,
splitter handles) or manage hover, in absolute coords. Moving a parent there
does **not** move children — use the float system
(`FloatAnchor`/`FloatTieOff`/`FloatOffsetX`/`FloatOffsetY`) to position elements
that have children.

**`AmendLayout` runs under the frame lock (`w.mu`), so no callback reached from
it can call a window-mutating API.** `SetFocus`, `ClearFocus`, `SetView`,
`ClearDrawCanvasCache` and `Window.Lock` all take `w.mu`, which is not
reentrant; they panic naming themselves. The remedy is
`ctx.Window.QueueCommand`. Library code reaching app code from the pass raises
it with `deferCallback` (`gui/window_deferred.go`) — which is how Input's blur
commit works, so `OnTextCommit` with `InputCommitBlur`, `OnBlur` and a
normalize-driven `OnTextChanged` get a **nil `ctx.Layout`**. The Enter commit
path dispatches from `EventFn` with no lock held.
(`docs/specs/frame-lock-callback-deferral.md`)

## Event consumption

**Nothing is marked handled for you. A callback that acts on an event calls
`ctx.Consume()`; one that does not lets the event travel on.** This holds for
every callback — `OnClick` and `OnChar` no differently from `OnKeyDown`. There
is no `ctx.Bubble()`: declining is what silence already means. An empty
`OnClick` blocks nothing. `ctx.Event` is nil in `AmendLayout` and `OnScroll`;
both `EventCtx` methods are nil-safe.

## Other implementation notes

- `(*Layout).spacing()` counts only visible children (`ShapeType != ShapeNone`,
  `!Float`, `!OverDraw`) — fence-post gap calc.
- Shape text fields live in `Shape.TC` (`*shapeTextConfig`), not on `Shape`.
- `ContainerCfg.Title`/`TitleBG` render a group-box label in the top border
  (floating eraser + text, like an HTML fieldset). `TitleBG` must match the
  parent bg color to erase the border behind the title.
- `Children []Layout` = values, parents = pointers. Avoids cycles.
- `StateMap` (keyed by namespace consts like `nsOverflow`, `nsSvgCache`) is the
  per-window typed kv store for widget internal state.

## Dev-mode diagnostics

**Most identity mistakes here are already detected at runtime — turn the gate on
before hand-auditing a layout.** `gui.Debug(true)`, or `GOGUI_DEBUG=1`, walks
the composed tree every frame and reports to stderr (`gui/debug.go`). Findings
are warn-once; the walk allocates, so dev only. Categories gate independently
via `gui.DebugCategories(mask)`; `gui.DebugEnabled()` queries the gate.

Reported: duplicate effective ID (names both claim sites); `Focusable`,
scrollable, or `OnMouseLeave` shape with no `ID`; a scrollable listbox that
resolved to height 0; a container setting both `Wrap` and `Overflow`; a gradient
over the shader's stop limit; a window feature the platform refused; a stamp
that disagrees with its scope (`DebugStampDrift` — a hand-built `Layout` in a
generated tree); a callback that acted without `ctx.Consume()` while an ancestor
also handles; a link that opened nothing.

Four categories worth knowing by name:

- **`DebugUnscopedIDs` — the only one _not_ in `DebugAll`.** Reports an `ID`
  with no ID-bearing ancestor: a window-global name, so the widget cannot be
  dropped into a second panel as it stands. A design property rather than a bug,
  so ask for it explicitly when auditing a screen for reusability.
- **`DebugUnresolvedKeys`.** A resolve that answered with the bare leaf while
  that shape landed under a scope, or a `StateMap` key left a bare leaf while an
  ancestor join rewrote the shape — the widget's state key and its identity are
  different strings. Latent: it works until a second instance of the same
  `cfg.ID` appears under another scope, then both share one state slot. Remedy
  is `w.EffID(cfg.ID)` in `GenerateLayout` or `ctx.EffID(leaf)` in a handler
  (#518, #519, #520).
- **`DebugUnknownFocus`.** A frame ended holding a focus ID no focusable shape
  claims — `SetFocus` on a misspelled or unscoped ID. Names the spelling the
  frame did stamp (#521).
- **`DebugUnknownLookup`.** `FindByID`/`ScrollVerticalTo`/`ScrollVerticalToPct`
  found nothing while the frame stamped that leaf under a scope. Only a **near
  miss** reports, so a probe stays silent; library code that probes on purpose
  calls the unexported `findByID`. Asserted through `captureDebugMask`, not
  `TestFindings` (#536).

**A widget factory that reads window state must defer its build.** A factory
body runs while the _parent's_ `Content` slice is being built, before the
framework descends into the container the widget will sit in, so `w.EffID` there
joins the enclosing scope rather than the widget's own. Return a view struct or
a `viewFunc` and resolve inside `GenerateLayout`.

Assertable forms for tests, returning findings as data:
`(*Window).TestDuplicateIDs`, `(*Window).TestUnconsumedEvents`, and the general
`(*Window).TestFindings(mask)` — which takes the categories to run, so an opt-in
category is assertable instead of stderr-only:

```go
found := w.TestFindings(gui.DebugAll | gui.DebugUnscopedIDs)
```
