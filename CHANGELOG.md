# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to
[Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Fixed

- **Tab out of a text input no longer freezes the app** (#394) — the Input's
  blur commit fired from its `AmendLayout` hook, which `layoutArrange` runs
  while `Update` holds the window mutex. An `OnTextCommit`/`OnBlur` handler that
  called back into `SetFocus` — the natural thing to do from a commit handler,
  and what `examples/todo` did — deadlocked the main thread against itself: a
  frozen window, no panic, no CPU burn. App callbacks reached from the frame
  pass are now raised and run after the lock is released
  (`gui/window_deferred.go`).
- **macOS Option+printable shortcuts reach the app** (#393) — `keyDown:` routed
  every key through `interpretKeyEvents:` and treated any resulting preedit as
  an input-method claim, so on the US layout Option+I (the circumflex dead key)
  never arrived as `ModAlt|KeyI`, and the pending accent then composed itself
  into the next keystroke. A preedit started with no editable text widget
  focused is now discarded and the key-down delivered. CJK composition and
  Option+e → é inside a focused input are unchanged.
- **A key the input method declines mid-composition reaches the widget** (#393)
  — `keyDown:` claimed every key while a composition was live, so Tab could not
  leave a field with a dead key pending: the accent committed, `insertTab:` came
  straight back through `doCommandBySelector:`, and the key was dropped anyway.
  A declined key is now delivered, held back just long enough for the
  composition it committed to arrive first, so the text lands in the field being
  left rather than the one taking focus.

### Added

- **`InputCommitEnter` and `InputCommitBlur`** are exported. `OnTextCommit`'s
  second parameter is an `InputCommitReason`, but its values were unexported, so
  an app could receive the reason and not branch on it — the whole point of
  distinguishing "the user pressed Enter" from "focus moved on".
- **`DebugCallbacks`** joins `DebugAll`. It reports app callbacks the frame pass
  dropped because they kept re-queueing themselves round after round.

### Changed

- **A window API called while the frame lock is held now panics instead of
  hanging** (#394). `SetFocus`, `ClearFocus`, `UpdateView`,
  `ClearDrawCanvasCache` and `Window.Lock` probe with `TryLock` and, when a
  frame pass owns the mutex, panic naming the API and the remedy
  (`QueueCommand`). This is the case an app's own `ContainerCfg.AmendLayout` /
  `ButtonCfg.AmendLayout` hook can still reach — the hook exists to mutate the
  tree in place, so it cannot be deferred. A crash that says what to fix beats a
  freeze that says nothing.
- **`ctx.Layout` is nil in a blur-triggered `OnTextCommit`, `OnBlur` and
  `OnTextChanged`.** These now run after the arrange pass, and the tree they
  were raised from is rebuilt from pooled arenas before then, so the pointer
  would dangle. `ctx.Window` and the text argument are unchanged. The Enter
  commit path is untouched and still carries a live `ctx.Layout`.
- **`examples/todo` adds a task on Enter only**, not on blur. Tabbing out of a
  half-typed task left the draft as a silently created todo.
- **The platform input method is activated only for editable text widgets**
  (#393). `IMEStart`/`IMEStop` used to fire on any focus change to any focusable
  widget; they now follow the focused widget's edit context, decided each frame
  from the arranged tree (`gui/ime_context.go`,
  `docs/specs/ime-text-context.md`). This is the contract the backends already
  documented: Windows keeps the IMM context detached, X11 no longer sends ibus a
  `FocusIn` for a button, and the web backend no longer moves DOM focus into its
  hidden `<input>`. A consumer text widget not built on `Input` — one that
  handles `EventChar` itself and sets no `focusOwner` — no longer composes dead
  keys; plain characters still arrive.

## [v0.64.0] - 2026-08-21

Visual refresh (`docs/specs/visual-refresh.md`, phases 1–8): type ladder and
density, accent ramp, radius ladder and elevation, focus rings in every preset,
button variants, retuned progress/boolean sizing, registry down to dark, light
and the platform pairs.

### Added

- **`ExpandPanelCfg.FocusDisabled`** — opts the header row out of the tab order
  and its focus ring, matching the `FocusDisabled` opt-out the sixteen
  focus-by-default input Cfgs already carry. For decorative or demo panels where
  keyboard toggling is not wanted.

- **`ThemeCfg.SizeFieldMinWidth` / `Theme.SizeFieldMinWidth`** — the MinWidth
  floor a text-bearing form control takes when its Cfg states none, seeded to
  160 in `baseCfg()` so every preset carries it. Zero means no floor, not
  "derive the default": a hand-built `ThemeCfg` that leaves it unset asks for
  Fit-to-content. `Input`, `NumericInput`, `InputDate`, `Select` and `Combobox`
  consume it; a Cfg stating its own `MinWidth` or `Width` opts out. This retires
  four literals in `ThemeMaker` (`MinWidth: 75` / `MaxWidth: 200`, spelled once
  on `selectStyle` and again on `comboboxStyle`), which is why a `Select` and an
  `Input` in one row disagreed on width for a reason neither the theme nor the
  caller stated.

### Changed

- **`Form` and `Table` default their `Sizing` to `FillFit`,** for the same
  reason as `ExpandPanel` below. `Table`'s header and body zones were already
  `FillFit`/`FillFill` inside a Fit outer, so a table hugged its columns and its
  focus ring stopped short of the row it sat in; a form shrink-wrapped to its
  widest label row. No example set `TableCfg.Sizing` at all, and the two that
  set `FormCfg.Sizing` were spelling the new default by hand.

- **`ExpandPanel`'s header puts the disclosure arrow at the trailing edge.** A
  flexible spacer between the head view and the arrow absorbs the surplus width.
  Previously the arrow hugged the head view, which only looked right while the
  panel shrink-wrapped.

- **`ExpandPanel` defaults its `Sizing` to `FillFit`.** Left at the zero
  `Sizing` (FitFit) the panel widened to its longest unwrapped line, and since a
  container does not clip, it painted through its own border and off the window;
  a `TextModeWrap` body inside it never wrapped, because a Fill child in a Fit
  parent has no width to wrap against. A disclosure panel is a full-width block
  whose height follows its body, so it now picks that default the way
  `Breadcrumb`, `MenuBar`, `Sidebar`, `Splitter`, `TabControl` and `DockLayout`
  already do. Callers wanting the old shrink-to-fit set `Sizing: gui.FitFit`
  explicitly.

- **`selectStyle` and `comboboxStyle` are no longer width-capped.** The old
  `MaxWidth: 200` stopped a `Select` given `FillFit` in a 900px row at 200px
  with no way for the caller to see why, and it would have silently defeated the
  labelled-field fix below. A caller wanting a ceiling sets `MaxWidth` on the
  Cfg. `maxDropdownHeight` is unrelated and unchanged.
- **An unsized labelled field is now 160px wide, not the width of its content.**
  Behaviour change for any app relying on a labelled `Input`, `Select`,
  `Combobox`, `NumericInput` or `InputDate` shrink-wrapping its text.
- **A `Row`'s stated `MinWidth` is now border-box** (issue #385). `layoutWidths`
  added the row's padding and inter-child gap sum on top of the caller's
  `MinWidth` on a row but not on a column, so `Select` and `Combobox` (rows of
  three children) arranged at 193px under the 160 field floor while an `Input`
  beside them arranged at exactly 160. A row now takes a stated `MinWidth` as
  the whole width budget — matching the column branch and `MaxWidth` — and
  arranges narrower by its padding and spacing than before. Behaviour change for
  any app calling `Row(ContainerCfg{MinWidth: X})`.

### Removed

- **Six theme presets** (`docs/specs/visual-refresh.md` §7, phase 8):
  `dark-no-padding`, `dark-bordered`, `light-no-padding`, `light-bordered`,
  `blue-dark` and `blue-dark-bordered` are gone from the registry. The registry
  now holds eight: `dark` (the default), `light`, and the platform pairs
  `macos`/`macos-dark`, `gnome`/`gnome-dark`, `windows`/`windows-dark`.
  Migration: `dark-bordered` was identical to `ThemeDark` (bordered is the
  default), `light-bordered` is `ThemeLight.WithBorders(true)`, the no-padding
  pair is `Theme.WithBorders(false)` plus a `ThemeMaker` padding override, and a
  blue variant is a `ThemeMaker(ThemeCfg{...})` call. Name-based `ThemeGet` for
  any removed name now misses; `ThemePicker` lists eight.

### Fixed

- **A `ListBox` painted its keyboard-focus row and its mouse-hover row the same
  grey, and the focus row did not follow a click.** Two separate defects with
  one symptom: a list could show a focus highlight on the first row, a hover
  highlight on a third, and the selection wash on a second, with nothing to say
  which was which. Clicking a row called `OnSelect` but never touched
  `nsListBoxFocus`, so the focus index stayed at its default 0 and the next
  arrow key resumed from the stale row; `Tree` already did this correctly in
  `treeRowClick`. A click now moves the focus index onto the clicked row and
  gives the list window focus (unless `FocusDisabled`). The focus row takes an
  accent ring instead of `ColorHover` (visual-refresh section 4.3), so it can no
  longer be confused with hover. Reorderable rows get the ring too — they
  carried no focus indication at all. The ring is stroked from `AmendLayout`
  rather than set as a Cfg border: a border there insets content, which would
  have added its width to every row in the list. Its width is named once
  (`listBoxRingWidth`) rather than taken from the list's own `SizeBorder`, which
  is 0 in a borderless theme and would have left `ThemeLight` with no visible
  keyboard cursor.

- **A labelled field could not fill its row** (`docs/specs/visual-refresh.md`
  section 1). `labelledField` hardcoded `Sizing: FitFit` on the wrapper
  `Column`, so the caller's sizing was unreachable: an
  `InputCfg{Sizing: FillFit}` inside a `FillFit` `Row` still arranged at the
  width of its text. Nine widgets route through that function, so no labelled
  field in any app built on this toolkit could fill its row. The wrapper now
  takes the field's width mode and pins its own height to `Fit`, so the
  label/field stack still hugs its content. A Cfg that never set `Sizing` gets
  the wrapper it got before. `ColorFields` and `DatePicker` pass `FitFit`
  explicitly — neither Cfg has a `Sizing` field, and both are fixed-shape by
  design.

- **Dock panel state was orphaned by any rearrangement of the dock tree** (issue
  #389). A dock group's container carried its bare `DockNode` ID, so its
  effective ID was the splitter path that happened to lead to it — and the group
  scopes its panel content, so a drop anywhere in the dock re-keyed every widget
  inside every panel: scroll offset, focus, input state. The group container now
  takes `ScopeID(dockID, node.ID)` and the splitter
  `ScopeID(dockID, "split", node.ID)`, both absolute and independent of tree
  position; tab and close buttons scope under the dock ID rather than
  window-global `dock_tab:` / `dock_close:` prefixes. Node IDs minted on a drop
  join with `-` instead of `:`, so they stay parts rather than reading as
  absolute IDs. A panel group with an empty ID is no longer a drop target: it
  used to resolve to the dock container itself and offer the whole dock as its
  zone. `DockLayout` gets its first golden recording.

## [v0.63.0] - 2026-08-19

### Added

- **`gnome`/`gnome-dark` and `windows`/`windows-dark` theme presets** (issue
  #374 pattern). Adwaita-derived and WinUI-derived platform themes, registered
  like the `macos` presets and reached by name through `ThemeGet`: hairline
  borders, platform corner rounding (GNOME rounder, Windows squarer), platform
  accent colors, and subtle popover/dialog elevation. Both keep the platform's
  hard accent outline for focus (`ColorBorderFocus`) instead of the macOS glow —
  `FocusRing` stays nil. Fonts stay unpinned so each resolves to the machine's
  system face (Cantarell on GNOME, Segoe UI on Windows).

### Fixed

- **`Wrap` + `Overflow` on one container hid children that fit** (issue #380).
  The two flags express contradictory strategies for the same condition — `Wrap`
  breaks content onto a new row, `Overflow` hides the tail behind a trigger —
  and nothing arbitrated between them. When a wrap container's content fit a
  single row, `layoutWrapContainers` kept its left-to-right axis and
  `layoutOverflow` then hid a child anyway, because it reserves room for a
  trigger button that a wrap never has. `Wrap` now wins: `layoutOverflow` skips
  any container with `Wrap` set, and the new `DebugWrapOverflow` category (part
  of `DebugAll`) reports the combination once per window. Scrollable containers
  were already unaffected.

- **`Wrap` stopped wrapping once the child count grew past the window width**
  (issue #378). `layoutWidths` seeded a container's min-width floor with the
  full single-row inter-child gap sum, which is right for a plain `Row` but
  wrong for `Wrap` and `Overflow` — both exist so the container can be narrower
  than one row of content. The floor therefore grew linearly with the child
  count (`(n-1) * Spacing`), and once it exceeded the available width the fill
  pass clamped the container back up to it, so wrapping measured against a width
  wider than the window and rows spilled past the right edge. Only visible in a
  non-maximized window, and only past the item count where the floor overtook
  the real width — hence "wraps at 40 items, overflows at 50". A wrapping or
  overflowing container now floors at its widest single child plus padding. Not
  verified against the reporter's repo, which pins released v0.51; the fix lands
  on the unreleased line.

- **Restored caller-facing `*Cfg` fields unexported by the #230 surface sweep**
  (issue #372). The sweep unexported every symbol with no in-repo external
  reference, including configuration fields users set directly in struct
  literals. Restored: `SvgCfg.FileName`/`NoAnimate`,
  `SelectCfg.SubheadingStyle`/`NoWrap`, `ListBoxCfg.SubheadingStyle`,
  `InputCfg.PreTextChange`/`PostCommitNormalize`,
  `NumericStepCfg.ShiftMultiplier`/`AltMultiplier`/`MouseWheel`/`Keyboard`,
  `ContainerCfg.OnScroll`, `OnIMECommit`, `ClickOnSpace`, `ClickOnEnter`,
  `ClipContents` and `FloatAutoFlip`, the
  TabControl/Breadcrumb/Menubar/ContextMenu/Splitter/Slider/Scrollbar/DockLayout
  style ladders, `ThemeCfg` size/font/elevation knobs, `WindowCfg`
  `AllowedSvgRoots`/`MaxImageDownloads`/`HistoryBytes`, datagrid's `DataGridCfg`
  and `GridColumnCfg` surface, and ~80 more fields across every widget Cfg, plus
  their enum types (`FormValidateOn`, `SpringCfg`, `ToastSeverity`,
  `DatePickerRollerDisplayMode`, `GridColumnPin`, `GridCellEditorKind`,
  `GridPaginationKind`, `GridAggregateCfg`, `GridCellFormat`, `MathFetcher`,
  `MermaidFetcher`, `PrintHeaderFooterCfg`).

## [v0.62.0] - 2026-08-18

### Added

- **Headless frame rendering: a software rasterizer and `RenderToPNG`** (issue
  #333). go-gui could not produce a pixel image of a frame without a GPU and a
  window, which ruled out CI screenshots and pixel-level regression tests.
  - New package `gui/backend/soft`: `RenderToImage(w, scale)`,
    `RenderToPNG(w, scale, path)` and `Release(w)`. It replays the same flat
    `[]RenderCmd` stream the GPU backends and the PDF printer consume, so no
    existing backend changes and none consults it. `scale` is the device pixel
    ratio; a window that has not rendered yet has its `OnInit` run first, so the
    same window value passed to `backend.Run` can be passed here instead.
  - Text is real, not approximated. The package implements `glyph.DrawBackend`
    over a CPU framebuffer and installs the resulting `glyph.TextSystem` as the
    window's `gui.TextMeasurer`, so shaping, metrics and glyph rasterization are
    the code the GPU backends already run — go-glyph is pure Go.
  - `(*Window).SetHeadlessRender` / `HeadlessRender` mark a window as rendering
    headlessly, suppressing wall-clock-driven visuals (today the blinking caret)
    so repeated captures of one window state produce the same pixels.
  - Phase 1 covers clip, rect, stroke rect, circle, line, gradient, gradient
    border, image and every text kind. See
    `docs/specs/headless-software-rendering.md` and `examples/headless_png/`.

- **Software rasterizer phase 2: SVG, shadow and blur, filters, stencil,
  rotation, terminal grid** (issue #360). `gui/backend/soft` now draws every
  render kind the pipeline emits, so a headless capture is the frame rather than
  the subset of it phase 1 could rasterize.
  - `RenderSvg` rasterizes the tessellated triangle list, honouring `HasXform`,
    `RotAngle` and `VertexAlphaScale` in the same order the GL backend applies
    them. Triangles are filled in runs of one colour so a shape's interior edges
    cancel instead of showing as antialiasing seams; a triangle with per-vertex
    colours is interpolated barycentrically.
  - `RenderShadow` and `RenderBlur` rasterize their rounded rect into a coverage
    mask and blur it with three box passes, the shadow additionally erasing the
    part that would sit under its caster.
  - Filter, stencil and rotation brackets share one mechanism: the bracketed
    commands draw into a pooled offscreen layer, and the matching end composites
    it back with the effect applied — a coverage mask, an inverse-mapped
    resample, or blur plus colour matrix plus repeated composite. Because the
    layer is composited rather than the geometry transformed, the effects hold
    for text and images, not only for shapes.
  - `RenderTermGrid` renders the terminal character grid — background runs,
    selection, cursor styles, column-pinned glyph runs and underlines — so
    go-term can capture a terminal headlessly.
  - Hostile input is bounded rather than trusted: blur radii and filter layer
    counts are clamped, NaN extents and corner radii are rejected before they
    reach the rasterizer, and compositing saturates instead of wrapping a
    colour-matrixed pixel to near-black.
  - `RenderCustomShader` remains unsupported by design: it is GLSL, and there is
    no CPU equivalent to compile.

- **`VirtualList` — virtualized lists whose rows may differ in height, and
  index-addressed scrolling** (issue #332). Virtualization was arithmetic over
  one scalar row height, which is exact for a widget that owns its row shape and
  useless for rows the caller builds; and a row virtualization had not built
  could not be scrolled to at all, because `scrollToView` resolves through
  `FindByID`.
  - `VirtualList`/`VirtualListCfg` build only the rows near the viewport and
    hold the rest of the space in two spacers, sized from a per-list prefix-sum
    (Fenwick) height model that seeds from an estimate and converges on measured
    heights. `ItemHeight` is the cheap path — exact from frame 1, no
    measurement. `ItemKey` stores measured heights under a stable identity, so
    an insert, delete or reorder keeps each height on its own item. `OverscanPx`
    tunes how far past the viewport rows are built, in pixels rather than rows.
  - `Window.ScrollToIndex`, `ScrollToIndexAt(frac)`, `ScrollIndexIntoView` and
    `ScrollToEnd` address rows by index through the height model, so they reach
    rows that do not exist this frame. `ScrollToEnd` is the correct
    pin-to-bottom: `ScrollVerticalToPct(id, 1)` takes a percentage of a content
    height that virtualized rows only estimate, so it drifts.
    `Window.InvalidateListHeights` re-measures a list whose content changed
    under a stable key.
  - `ListBox`, `Table`, `Tree`, `Combobox` and the command palette register the
    uniform (O(1), no per-item storage) form of the same model, so the index API
    works on them too. Index spaces differ per widget — a frozen table header is
    data index 0 but sits outside the scrollable, and the combobox indexes its
    _filtered_ items.
  - `Window.VirtualListFocusedIndex` / `SetVirtualListFocusedIndex`: an unbuilt
    row has no shape to hold focus, so a virtual list's keyboard focus is an
    index on the list.
  - A row must not turn the width `ItemView` hands it into a width it demands:
    `MinWidth` taken from that argument ratchets the list wider every frame, and
    each width change re-wraps every row, so the list never settles. `Debug`
    reports it (the `DebugListBoxNoHeight` category) after four consecutive
    frames of widening with the window unchanged.
  - See `docs/specs/virtualized-variable-height-lists.md` and
    `examples/virtual_list`.

- **Keyboard navigation for `Table`, `ColorSwatch` and the `ExpandPanel`
  header** — the three widgets the #335 audit recorded as "not focusable" (issue
  #345).
  - `TableCfg.Focusable` opts a table into the tab order. Focus lands on the
    table; Up/Down/Home/End move an active row — tinted with the hover color,
    scrolled into view under virtualization, and synced to mouse clicks — and
    Enter/Space activate the active row the way a click would. Selection follows
    movement when `OnSelect` is set; Shift extends a range under `MultiSelect`.
    `TableCfg.ColorBorderFocus` (theme-backed) paints the focus ring.
  - The `ExpandPanel` header now joins the tab order ahead of the panel body's
    own focusables; Space/Enter toggle it. `ExpandPanelCfg.ColorBorderFocus`
    (theme-backed) paints the ring.
  - `ColorSwatchCfg` gains opt-in `Focusable` and `OnClick`; Space/Enter
    activate a focused swatch like a click. The ring replaces the resting
    outline on the color layer, the same border convention as the other color
    controls.
- **`Colors ColorSet` on eleven more widgets** — `Input`, `NumericInput`,
  `Select`, `Combobox`, `ListBox`, `Tree`, `Slider`, `ContextMenu`, `Menubar`,
  `Table` and `ExpandPanel` now accept the per-state `ColorSet` the six widgets
  of v0.54.0 already used. Additive and zero visual change: the surviving flat
  `Color*` fields still win over the set when both are set (the `applyTo`
  precedence), so existing callers keep their appearance. `ColorSelect` and
  `ColorHighlight`, which have no set slot, stay flat. Inline `ColorSet{...}`
  constructions inside gui/ and gui/datagrid/ resolve at the construction site
  instead of relying on the receiving widget to resolve (issue #342).
- **Named text roles on `Theme`** — `TextStyleSecondary`, `TextStyleLabel`,
  `TextStyleDisabled`, `TextStylePlaceholder`, with matching
  `ThemeCfg.ColorText*` overrides. A widget that wants quiet text now names the
  reason it is quiet and takes the theme's answer, instead of spelling an alpha.
  The values are per-theme and contrast-matched, not one multiplier: alpha
  blends toward the background, and the same alpha that reads as "present but
  quiet" on a dark ground reads as nearly gone on a light one. Polarity is
  derived from the theme's own text and background luminance, so a theme an app
  builds itself gets the right ladder without knowing the roles exist. See
  `docs/specs/widget-visual-consistency-audit.md` (issue #335).
- **`Label` on eight field Cfgs** — `Input`, `Select`, `Combobox`, `Slider`,
  `NumericInput`, `DatePicker`, `InputDate` and `ColorPicker`. One convention,
  decided once: above the field, left-aligned, in the theme's label role. Fills
  `A11YLabel` when that is unset. Additive — an empty `Label` renders exactly as
  before, with no wrapper and no extra shape (issue #335).
- **`Theme.PaddingField` / `ThemeCfg.PaddingField`** — the inset a text-bearing
  form control puts around its text, so controls in one row share a height
  (issue #335).
- **`ListBoxCfg.ColorBorderFocus` and `ListBoxStyle.ColorBorderFocus`** — a
  `ListBox` is focusable and key-navigable and previously drew no focus ring at
  all (issue #335).
- **Golden render tests** (`gui/golden_test.go`, `gui/testdata/`). Builds a
  widget, drives the real frame pipeline, and diffs the emitted `[]RenderCmd`
  against a recording, in both `ThemeDark` and `ThemeLight`. Re-record with
  `go test ./gui/ -run TestGolden -update` after reading the diff.

- **Composable color components.** `ColorPlane` (saturation × lightness),
  `ColorWheel` (hue × saturation), `ColorChannelSlider` (one HSLA channel, with
  a track showing the colors it can pick, horizontal or `Vertical`),
  `ColorSwatch` (a color over a transparency checkerboard) and `ColorFields`
  (hex and channel inputs) are now public widgets. They are stateless: each
  takes a `gui.HSLA` and reports changes, so an app holding one value can drive
  any arrangement of them and they stay in sync. See
  `docs/specs/color-picker-components.md`.
- **`gui.HSLA`**, the color model those components speak, plus `ColorToHSLA`,
  `HSLA.Color`, `HSLA.String` (CSS `hsla()` notation), `Color.Hex` and
  `ColorFromHex`. An app holds an `HSLA` rather than a `Color` because RGBA
  cannot carry hue through a gray or through black.
- **In-memory image registry** — `gui.UseImage`, `HasImage`, `DropImage`,
  `SetMemImageBudget`. Register an NRGBA8 pixel buffer under a content key and
  get a `Src` string usable anywhere `ImageCfg.Src` or `DrawContext.Image` is
  accepted; every backend uploads it as a texture. This is what makes imagery no
  gradient can express — a hue wheel, an HSL plane, an alpha checkerboard —
  drawable at all.

### Changed

- **Spacing tiers now mean relatedness** (issue #344). The four rungs —
  `SpacingTight 2`, `SpacingSmall 5`, `SpacingMedium 10`, `SpacingLarge 15` —
  state what each gaps, documented at the const block in `gui/styles.go` and in
  `docs/style-guide.md`. The tab strip, submenu items and calendar cells fold
  their private `2`/`1`/`2` into `SpacingTight`; the toast stack and
  `RadioButtonGroup`'s default move from `Small` to `Medium` (toasts spread
  8→10, radio options 5→10); markdown block spacing moves 12→15 as the first
  internal caller of `SpacingLarge`. `nestIndent` stays 16 and is documented as
  a structural indent, not a sibling gap.

- **`gui.ColorPicker` is now a composition of the components above.** Its
  `ColorPickerCfg` is unchanged and existing callers compile untouched. Five
  visible changes: the square is the HSL saturation × lightness plane rather
  than the HSV square, the hue and alpha sliders stand vertically to the right
  of the plane instead of stacking beneath it (so the picker is squarer), the
  preview swatch moves next to the hex field it previews, the alpha slider gains
  a real transparency checkerboard, and the hue strip is exact instead of being
  silently resampled to five gradient stops. `ShowHSV` still works and is
  deprecated in favour of `ShowHSL`.

- **Visual: de-emphasized text is unified across widgets** (issue #335). The
  same "disabled text" state used to render at three different alphas — 65 on a
  tab, 127 in an `Input`, 130 on a breadcrumb — because whether the themed value
  got halved again depended on how each widget happened to be built. Disabled
  tab text lightens (65 → 128) and breadcrumb text moves 130 → 128. On the light
  theme every quiet role gains contrast, since the alphas had been copied from
  the dark theme unchanged.
- **Visual: form controls share a height.** `Input`, `Select` and `Combobox`
  each picked their own text inset, so a `Select` rendered six pixels taller
  than the `Input` beside it. All text-bearing controls now take
  `Theme.PaddingField` (issue #335).
- **Visual: `ListBox`, `ColorPlane`, `ColorWheel` and `ColorChannelSlider` show
  focus.** All four take keyboard focus and previously gave no indication of it
  (issue #335).
- **Visual: `Switch` and `Toggle` labels gain the gap `Radio` already had.** The
  three spelled their trailing label three ways; all three now sit the same
  distance from their control (issue #335).
- **Visual: `ColorFields` channel labels** take the theme's label role instead
  of an invented two-step size drop and 0.7 alpha, and the shared spacing tier
  instead of a bespoke 2px gap (issue #335).
- **Visual: vertically-centred text is centred on its ink, not its line box**
  (issue #346). A text shape is sized to the font's full height, so the descent
  it reserves goes unused by text that cannot descend — digits above all — and
  the text reads high. Every vertically-centred control that owns its text now
  corrects for this: `Badge` counts, the progress-bar readout, `Button` and
  everything built on it (tabs, command buttons), menu items, `Select`'s label,
  `ColorFields` channels, date masks and numeric fields. Roughly 1px at 16pt,
  3px at 48pt.

  The correction is deliberately not uniform, because a single rule was tried
  and measured worse: it made badges read right while dropping single-line
  inputs too low. What the text **is** decides the band it is centred on — a
  value on its own ink, a label on the face's cap band, a glyph (icon, step
  triangle, `×`) always on its own ink, and editable text on a band that ignores
  content so the baseline cannot move as the user types. Text the user types
  into an unconstrained control is not corrected at all. See
  `docs/specs/text-optical-centring.md` and the "Vertical centring" section of
  `docs/style-guide.md`.

  No new public API: the opt-ins are internal, so a widget outside `gui/`
  inherits the correction through `Button` and the field widgets but cannot
  request a band of its own. Run `examples/optical_centring/` to see the bands
  side by side at 16/24/48pt.

### Fixed

- **Word motion and double-click select by rune class, not whitespace** (issue
  #329). `Ctrl+Left`/`Ctrl+Right` and double-click treated only space, tab and
  newline as separators, so `foo.bar.baz` was one word, `a+b` was one word, and
  Japanese text had no interior boundaries at all — a word motion jumped the
  whole line and a double-click selected it. A word is now a maximal run of one
  rune class (whitespace, punctuation, word, Han, Hiragana, Katakana), so a
  punctuation run is a word of its own and a script change ends a word.
  Underscore stays a word rune, keeping `snake_case_name` whole, and a combining
  mark stays attached to its base.
  - The rules are go-glyph's, not a second copy: the no-layout path now calls
    `glyph.WordStartLeft`, `glyph.WordStartRight` and
    `glyph.WordBoundsInString`, which apply the same class runs as the `Layout`
    methods the measured path already used. A window with a text measurer and
    one without now segment identically; previously they disagreed even about
    `\n`.
  - `Input` double-click and double-click drag-extend never had a layout path,
    so this is a visible fix there, not only in headless tests.
  - `moveCursorWordRight` now lands on the next word's start rather than past
    the current word's trailing whitespace. Identical for space-separated text.
  - Each `Ctrl+arrow` keypress and each double-click drops one `[]rune`
    conversion; the helpers take the string and convert at the go-glyph
    boundary.

- **Multi-line text sat high in its box, so `ListBox` rows read top-biased.**
  go-glyph sizes a line box as the baseline-to-baseline advance — ascent +
  descent + leading, floored at 1.15em — while rendering puts the baseline at
  `FontAscent` below the shape top. The layout pass took that height verbatim,
  so the leading below the _last_ baseline was reserved at the bottom of every
  multiline or wrapped text shape and nothing ever painted into it: 2.4px at
  size 16, all of it under the text. A text shape is now sized to the
  ascent..descent box the single-line path already uses
  (`(lines-1)*lineHeight + FontHeight`), which is also the box
  `docs/specs/text-optical-centring.md` assumes. Inter-line spacing is
  unchanged; only the trailing leading goes.
- **A virtualized `ListBox` built too few rows to cover its viewport.** The row
  height it virtualizes with was estimated as `1.4 x font size + padding`, which
  over-counted a row by ~30% once a real text measurer was present — a row is
  one text shape (`FontHeight`) plus its padding. Spacers, the visible range and
  the height model `ScrollToIndex` reads all divide by that number, so a tall
  scrolled list left a blank strip along its bottom edge that the two-row
  overscan could not cover. The estimate now asks the measurer, falling back to
  the same approximation the text path itself uses when there is none, and the
  spacers take the caller's number rather than recomputing it. `Combobox`, the
  command palette and `VirtualList` share the estimate and are fixed with it.
- **`Input` undo recorded every keystroke as its own step, evicting the whole
  history after 50 characters** (issue #328). Consecutive edits of the same kind
  — a typing run, a backspace chain — now coalesce into one undo step whose
  anchor is the state before the run began, so Ctrl+Z after typing a sentence
  undoes the sentence, and a long run no longer pushes older history out of the
  50-entry cap. A run breaks on caret motion or click, on a drag selection, on
  paste or any multi-rune insert (IME commits included), on a programmatic text
  set, and on a change of edit kind (typing after backspace and backspace after
  typing are separate steps).
- **`Input` ignored the theme's own padding** (issue #335). `InputStyle.Padding`
  was dead: the widget fell back to a hardcoded inset, so a theme author editing
  it saw `Container` and `ListBox` move while `Input` stayed put. `Input`'s
  inner row also left `SizeBorder` unset and so inherited the theme's container
  border, silently reserving 3px of height for a border it never painted.
- **A live menu item's shortcut hint wore the disabled dim** (issue #335), so an
  enabled item and a dead one rendered identically.
- **`ColorFields`' private optical correction overshot by ~0.04em** — about
  0.7px at 16pt, 2px at 48pt (issue #346). It shifted padding from bottom to
  top, which moves the text by twice the offset, and compensated for a
  half-leading term the line box does not carry: `glyph` reports line gap
  separately from the height a text shape is sized to. Measured ink replaces
  both of its local ratios, and the shared helper is now what every widget
  reaches for.
- **The date-picker calendar read the wall clock directly** to ring today, via
  `time.Now()` rather than the window's clock, so it could not be pinned by a
  test and the golden recording broke on every date rollover (issue #346).
- **`DockLayout`'s close glyph bypassed the theme**, reading a package size
  constant directly, so a theme that shifted its type scale could not move it
  (issue #335).
- **`buildapp` ad-hoc signing silently revoked every TCC grant on each
  rebuild.** The tool hard-coded `codesign -s -`, and an ad-hoc signature
  carries no certificate and no team identifier, so TCC keys its grant on the
  cdhash — which changes every build. Screen recording, microphone, camera,
  accessibility, input monitoring and full disk access were all dropped on every
  `make app`, while the System Settings row survived (that list is keyed by
  bundle id, the authorization check by cdhash), so the permission looked
  granted and the API returned denied. New `-sign <identity>` flag, defaulting
  to `$BUILDAPP_SIGN_IDENTITY` and then to `-`, so existing callers are
  unchanged and anyone with a self-signed code-signing certificate keeps grants
  across rebuilds. The `-bundle-deps` re-sign now reports `codesign`'s own
  output instead of a bare exit status. See `cmd/buildapp/README.md` § Signing
  (issue #303).

## [v0.61.0] - 2026-08-15

### Changed

- **go-glyph bumped v1.21.0 → v1.22.0.** Brings `backend/ebitengine` out of the
  root module into its own module (same import path), so ebiten and the `oto/v3`
  audio stack drop out of the dependency graph for every go-gui consumer. Also
  adds a `make prepush` gate on the glyph side.

### Fixed

- **Layout and hero transitions left ID-less descendants behind.**
  `AnimateLayout` only moved shapes that had a snapshot of their own, and layout
  coordinates are absolute, so a sliding container's `Text` (no `ID`, so no
  snapshot) jumped straight to its final position while the container eased
  under it. The walk now carries the nearest interpolated ancestor's position
  delta and applies it to descendants that have no snapshot — the standard FLIP
  subtree shift. A shape with its own snapshot replaces the carried delta rather
  than adding to it, since the snapshot is absolute and already accounts for its
  ancestors. `AnimSnapPos` zeroes the delta for the subtree, so a snapped
  container still holds everything under it still. `HeroTransition` had the
  identical defect — a morphing card travelled without its label — and
  `applyHeroRecursive` now carries the shift by the same rule.

- **`examples/animations`: the seven toolbar buttons all shared one `ID`.** A
  duplicate effective ID is a single identity, and the layout transition
  snapshots by effective ID, so pressing **Layout** made every button lerp from
  whichever button wrote the snapshot last — the toolbar borders visibly slid.
  Each button now keys on its label under a scoping `animations_toolbar` row,
  and `TestMainViewNoDuplicateIDs` asserts the window is clean.

### Added

- **Per-shape snap mask for layout transitions (issue #310).** `AnimateLayout`
  used to be all-or-nothing over the whole window: every ID-bearing shape got X,
  Y, Width and Height lerped, with no way to exclude a channel, a shape or a
  subtree. `Shape.AnimSnap` (and its `ContainerCfg.AnimSnap` write-through, the
  same seam `Hero` uses) now marks channels that snap instead of easing —
  `AnimSnapPos`, `AnimSnapSize`, `AnimSnapAll`. This covers "slide, don't
  stretch" card and grid reflow, and holding a scroll viewport, datagrid body or
  virtualised list still while the surrounding chrome animates. The mask is an
  **opt-out**: the zero value animates every channel the active transition
  covers, so no existing caller changes behaviour. It is OR-inherited down the
  layout walk, so a snapped container snaps its whole subtree and a child cannot
  escape it. The hero transition is unaffected — `Shape.Hero` is already an
  explicit per-shape opt-in. Cost is a `uint8` that lands in existing struct
  padding (`Shape` stays 304 bytes) plus two branches inside a walk that already
  runs.

### Changed

- **BREAKING: `A11YLabel` and `A11YDescription` moved to an embedded `A11YCfg`
  (issue #311).** The two fields were declared independently in 35 `Cfg`
  structs; an AST scan across every `Cfg` shows they are a perfect co-occurrence
  group — 35 carry both, **zero** carry exactly one — so they are now declared
  once and embedded. Behaviour is unchanged: `accessInfo`, `makeA11YInfo`,
  `a11yLabel` and the accessibility tree are untouched, and no test assertion
  moved. Field promotion keeps every **read** and **assignment** spelled exactly
  as before (`cfg.A11YLabel`), but Go has no promoted-field key in a composite
  literal, so code that **sets** either field in a literal must name the embed:

  ```go
  // before
  gui.ButtonCfg{ID: "save", A11YLabel: "Save"}
  // after
  gui.ButtonCfg{ID: "save", A11YCfg: gui.A11YCfg{A11YLabel: "Save"}}
  ```

  `A11YRole` and `A11YState` are unaffected — only `ButtonCfg` and
  `ContainerCfg` carry those, which is far short of a group worth embedding. The
  new `A11YCfg.a11yInfo(fallback)` method gives the recurring
  `makeA11YInfo(a11yLabel(cfg.A11YLabel, X), cfg.A11YDescription)` pairing a
  home; it is a wrapper, and both helpers stay as they were.

### Fixed

- **The check mark now sits in the middle of the checkbox.** `Toggle` (and its
  `Checkbox` alias) centred the check on the font's advance box, which spans the
  descent the glyph never paints into and each side bearing, so the check
  floated above centre — barely at the default size, glaringly on a large one.
  It is now centred on its ink: the backend measures the glyph's painted box
  (new go-glyph `InkBounds`) and `AmendLayout` moves the arranged glyph, so
  nothing about sizing, draw order, or the widget tree changes. Backends without
  the capability, and tests with no measurer, keep the previous advance-box
  centring. The toggle theme style's padding is symmetric again, its left-heavy
  value having been a partial nudge for the same defect. The same correction now
  applies wherever a single glyph is centred inside a drawn frame: the markdown
  task-list checkbox and the splitter's collapse buttons.

## [v0.60.0] - 2026-08-14

### Added

- **Per-window themes (issue #296).** A `Window` now owns its theme.
  `w.SetTheme(t)` themes that window alone and `w.Theme()` reads it back;
  windows that never pin one follow the app default, which package-level
  `gui.SetTheme` still sets. The frame pass installs the active window's theme
  before the view function runs, so widget factories keep resolving their
  defaults exactly as before — no factory signature changed, no allocation was
  added, and existing single-window code behaves identically. See
  `docs/specs/per-window-theme.md`.

- **`gui.Themed(t, build)` scopes a theme to one subtree.** Views the builder
  returns resolve their defaults from `t`; everything outside keeps the window's
  theme, and `Themed` nests. The builder runs at layout-generation time, which
  is what makes the scope work: factories resolve defaults when they are called,
  so ready-made child views would already carry the enclosing theme.

### Changed

- **`View` is now a single method — `GenerateLayout(*Window) Layout`
  (breaking).** `View.Content() []View` is gone. The 24 container and composite
  widgets that built child trees in `Content()` now build them in
  `GenerateLayout` and hand them to `appendChildViews`, which owns the child
  walk, the event-children cap, the scratch-arena reservation and the ID-scope
  push/restore — one mechanism instead of two, with identical behavior and
  allocations (flat_100 100, nested_3x10 100, deep_12x1 1). `*Layout` does not
  implement `View`; `ContainerCfg.Content` remains a plain field. Sibling
  consumers migrate in lockstep: go-charts records its gallery charts at build
  time (go-charts#41), go-map and go-term drop their `Content()`
  implementations. See `docs/specs/view-single-method.md`.

- **The default appearance changed: `ThemeMaker` is now the only source of
  widget styling (issue #300).** Go-Gui carried two sets of defaults — about 30
  `default*Style` literals in `styles*.go`, and `ThemeDark` — and they had
  drifted apart. Because `init` never installed a theme, an app that never
  called `SetTheme` ran on a mixture of the two, and switching themes once
  restyled it. The literals are gone; `init` installs `ThemeDark`.

  Every widget now looks the way it already did after any theme switch. The
  visible differences from the old fresh-app look:

  - Buttons, inputs, containers, dialogs, toasts, tooltips, expand panels,
    selects, trees, switches, toggles, tab controls, date pickers, color
    pickers, menubars, splitters, the command palette, comboboxes and listboxes
    lose their 1.5px border. Sliders lose 1px, radios 2px. `ThemeDark` is
    borderless by design; bordered is the separate `dark-bordered` preset.
  - The **data grid gains its styling**. Its literal was an unstyled placeholder
    with zero colors, so an unthemed datagrid rendered flat.
  - Inputs and listboxes take the theme's medium padding (10 rather than 5/6),
    so they are slightly taller.
  - Badges turn grey with bold white text instead of blue with unstyled text.
  - Dialogs gain 200/300 width bounds; toast titles are no longer bold; the
    button click color, input/switch/toggle focus colors, toggle radius, submenu
    spacing, command-palette detail color and input spell-error color all move
    to their `ThemeDark` values.
  - `NumericInput` and `RadioButtonGroup` follow the active theme. Both had a
    private hard-coded 1.5px border that no theme could reach, light included.

  To keep the previous look, install the bordered variant:

  ```go
  gui.SetTheme(gui.ThemeDark.WithBorders(true))
  ```

  See `docs/specs/theme-style-single-source.md`.

### Fixed

- **Post-generation code now reads the theme of the window it is acting on
  (issue #301).** Event handlers, post-arrange work and the backends resolved
  the theme from the frame-scoped cache, which holds whichever window generated
  last — correct in a one-window app, wrong with two. Thirteen such sites now
  call `w.Theme()`: the scroll and key-scroll paths, print export, the theme
  picker's highlight sync (visibly wrong before), toast max-visible, the select
  dropdown scroll, the inspector wireframe, and every backend's clear color.
  Widget factories and `GenerateLayout` are unchanged and keep the bare read,
  which is what makes `gui.Themed` subtree scoping work. The window and app
  theme stores now hold a pointer to an immutable value, so the per-event reads
  do not copy a struct holding ~40 style structs (the scroll benchmark stays at
  ~39 ns/op, 0 allocs); `w.Theme()` still returns a value and is unchanged for
  callers. A new `make ergonomics-audit` mode (`theme`) gates the
  post-generation paths. See `docs/specs/per-window-theme.md`.

- **Theme-keyed text caches no longer serve stale layouts.** The per-window
  markdown and rich-text layout caches invalidated on `Theme.Name`; two themes
  can share a name and differ in text styles, as a derived or scoped theme does.
  Both now key on a stamped theme identity.

- **The datagrid column-resize handle now renders its active color during a
  resize drag (issue #284).** A drag holds a mouse lock and `layoutHover` bails
  under a lock, so the handle's pressed color — painted only from `OnHover` —
  could not fire mid-drag and the handle showed its resting color while
  resizing. The handle's color now comes from the resize state read at
  generation time (`dataGridActiveResizeColID`), live for the whole drag and
  reverting on release or cancel.

- **Pressed-while-hovered colors now render.** Seven `OnHover` handlers paint a
  click color while the left button is held (button `Colors.Click`, breadcrumb
  crumb, expand-panel header, switch pill, toggle, radio, datagrid resize
  handle), but no frame could reach those branches: `layoutHover` synthesized
  its event with `MouseButton: MouseInvalid` no matter what the user was
  holding. The window now reports the real held button in hover events (see
  Changed below), so a press-and-hold renders the click color and release
  restores the hover color.

- **Interrupted (capture-loss) selection drags no longer leave a stuck selection
  (issue #281).** Input, Text and RTF selection drags hold a mouse lock whose
  `Cancel` hook fired on capture loss — a mouse-up that will never arrive
  (window-resize steal, alt-tab, the #237 class) — but only removed the
  edge-scroll animation; the partial selection survived as if committed. The
  three Cancel hooks now zero the dragged widget's own `selectBeg`/`selectEnd`
  (scoped to its `nsInput` key, never other widgets'), so an interrupted drag
  leaves nothing. A normal release still commits even outside the widget — the
  lock delivers the mouse-up anywhere; only capture loss goes through the Cancel
  hook.

- **Re-asserting focus no longer clears input selections (issue #277).**
  `setFocusLocked` called `clearInputSelections()` on every `SetFocus`,
  unconditionally, while the IME clear next to it was already guarded by
  `id != prev` (#156). Because consumers legitimately re-assert focus from
  inside their View function — which runs on every layout rebuild — an
  unconditional clear wiped every `nsInput` selection window-wide on each
  re-assert, silently dropping selections in the focused input and every
  unrelated field. The clear is now guarded exactly like the IME clear: only a
  real focus change drops selections (window-wide, as before); a same-widget
  re-assert leaves them alone. Clicking an already-focused input still collapses
  its selection to a caret at the click — that path sets the selection itself
  and never depended on the clear.

- **Markdown/RTF in-document anchor links (`#slug`) now scroll (issue #278).**
  `rtfOnClick` scrolled the bare slug, but a heading's ID is scoped to its
  document (`ScopeID(docID, "h", slug)` — e.g. `md:h:slug`, or `panel:md:h:slug`
  nested), so `FindByID(slug)` missed and the scroll silently no-opped — even at
  root. The `#` branch now resolves through `rtfResolveAnchor`: for a markdown
  block (every block carries the document's effective ID in `TC.markdownID`, now
  stamped on non-focusable documents too), the document-scoped heading spelling
  is tried first, then the bare slug — so arbitrary absolute targets
  (`#view:bottom`) and standalone RTF links keep working. The identity stamping
  and the selection machinery are separate flags now: a non-focusable markdown
  document gains anchor resolution but keeps its exact prior click behavior (no
  focus, no mouse lock, no per-frame selection walk).

- **Markdown: cross-block selection works under ID-bearing ancestors (issue
  #273).** The document's blocks were stamped with the raw `cfg.ID` at
  generation time, while the container's amend and key handlers keyed on the
  _effective_ ID the resolve pass stamps — so a markdown nested in a panel with
  an ID collected an empty block list (`nsMdBlocks` was keyed `panel:md`, the
  blocks matched `md`), and cross-block drag, Ctrl+A, Ctrl+C and click focus
  were dead, with the click writing selection state under a key nothing read.
  `Markdown` is now a struct view whose `GenerateLayout` resolves the effective
  ID once (`w.EffID(cfg.ID)`, the same mechanism the splitter uses) and stamps
  it into every block and inner ID, so the container, the blocks, the state
  slots and `SetFocus` all agree on one identity. Flat documents are unchanged;
  two documents sharing a leaf under different panels now keep separate
  selections.

- **Markdown: a stale selection no longer survives a document content change
  (issue #275).** `markdownContainerAmendLayout` now hashes the block list
  (offsets, rune counts, flat text) each frame and resets the per-widget
  selection when the hash changes: `SelBeg`/`SelEnd` are rune offsets into the
  _previous_ source, so a stale range would highlight — and Ctrl+C would copy —
  the wrong runes of the new text. A same-length rewrite of the document is
  caught too; a pure layout change (resize, font) leaves the selection alone.

### Changed

- **`OnHover` receives the held mouse button.** The window tracks the held
  button from its own event stream — set on `EventMouseDown`, cleared on
  `EventMouseUp` and `MouseCancel` — and `layoutHover` reports it in the
  synthesized hover event: `MouseLeft`/`MouseRight`/`MouseMiddle` while held,
  `MouseInvalid` when none is held. A hover handler can now distinguish a
  press-and-hold from a plain hover; it could not before, because the event
  always arrived with `MouseInvalid`. The event is still `Type: EventMouseMove`
  and is rebuilt field-by-field every frame.

- **`Theme.WithInspectorStyle` is unexported (breaking; no known consumers).**
  It was the only exported member of the `with*Style` family and was carried on
  an `exportaudit:keep` claim of sibling-repo use that does not exist. It is now
  `withInspectorStyle`, matching `withDataGridStyle` and the rest of the family.
  This completes the issue #288 API-surface review: with all sibling repos
  scanned, the authoritative audit reports a clean surface — one `none` export
  (`FillBorder`, intentionally public, showcase-documented) and every
  self/selftest-only export carries a justified keep marker.

### Added

- **Markdown interaction test suite expands to the nested-document and
  source-change cases** (`gui/markdown_select_interaction_test.go`):
  effective-ID block collection, click focus, Ctrl+A/C, cross-block drag and
  per-panel isolation under ID-bearing ancestors; offset-block click accuracy
  (the click handlers receive shape-relative coordinates via `callRelative`, so
  a click maps to the rune under the cursor wherever the block sits); and
  selection reset on content change with an idle-frame positive control. Plus
  diagram-cache async tests (`gui/markdown_diagram_fetch_test.go`) and
  block-renderer tests (`gui/view_markdown_blocks_test.go`). Package `gui/`
  moves from 81.0% to 82.4%.

- **FillFill roots on axis-less containers resolve to the window size (issue
  #262).** `updateLayoutLocked` pins a Fill root to the window dimensions via
  `Min = Max`, but the width/height sizing passes only consulted the pin on
  axis-bearing containers — so a `Splitter` (built on a `Canvas`) returned
  directly as the root view resolved to 0x0 and was invisible, despite `Sizing`
  defaulting to `FillFill`. The `axisNone` branch of both passes now honors
  explicit min/max pins, so `FillFill` is honest on axis-less roots and tracks
  window resize. It also makes an explicit `MinWidth`/`MaxWidth` on a
  `Fit`-sized `Canvas` take effect, which is the documented intent of those
  fields (Fill roots keep the window pin; the fill distribution clamps nested
  Fill children). Children of axis-less containers keep their sizing semantics
  unchanged.

- **Splitter: a collapsed pane with content now reaches its collapsed size
  (issue #263).** `splitterLayoutChild` pinned the pane to the size
  `splitterCompute` decided, then re-ran the fit passes, which recomputed the
  pane from its content's minimum and overwrote the pin — so a pane holding real
  content kept a sliver that drew underneath the handle. The pin is now
  re-applied after the fit passes, so collapse is exact (zero-width with the
  default `collapsedSize`, or exactly `collapsedSize` when set). Contract note:
  pane sizes come from ratio/min/max only; content larger than the computed pane
  size is clipped (panes have `Clip: true`), never stretches the pane.
- **Splitter: the spacebar toggles collapse again.** `splitterOnKeydown` tested
  `Event.CharCode`, which backends populate only on `EventChar` — a space
  keydown arrives with `KeyCode == KeySpace` and `CharCode == 0`, so the branch
  never fired. Now reads `KeyCode`, matching `DatePicker`, `Tree`, `Menu` and
  `Select`.
- **Breadcrumb and TabControl: the spacebar activates again.** Both had the same
  defect — `bcOnKeydown` and `tabControlOnKeydown` tested `Event.CharCode` from
  an `OnKeyDown` handler, so space never selected the focused crumb or tab. Both
  now read `KeyCode == KeySpace`, grouped with the identical `KeyEnter` branch.
  `Splitter`, `Breadcrumb` and `TabControl` were the only three sites with this
  defect; every other `CharCode` reader in `gui/` is an `OnChar` handler, where
  the field is populated.
- **Splitter: `SplitterStyle`'s border and radius reach the handle.**
  `applySplitterDefaults` seeded every color from the style but skipped
  `SizeBorder`, `Radius` and `radiusBorder`, so the handle and the collapse
  buttons fell through to the generic container and button defaults. Visible
  change under the stock theme: corner radius on both goes from `radiusMedium`
  (5.5) to the splitter's `radiusSmall` (3.5) — the border width was already
  `sizeBorderDef` either way. For a custom theme the splitter's own three values
  were dead and now apply. An explicit `SomeF(0)` still means "no border".
- **Splitter: part IDs now scope with the root (issue #264).** `Splitter`
  composed its pane, handle and collapse-button IDs in the factory, where no
  `Window` exists, so under an ID-bearing ancestor the root resolved to
  `outer:sp` while every part stayed window-global (`sp:handle`, …) — two
  splitters reusing one leaf ID collided on every part. The splitter is now a
  struct view whose `GenerateLayout` resolves the effective ID
  (`w.EffID(cfg.ID)`) and composes every part under that path. With no
  ID-bearing ancestor the spellings are unchanged. API consequence: parts of a
  nested splitter are addressed by scoped effective IDs (`outer:sp:handle`,
  `outer:sp:pane:first`).
- **Splitter: the handle's active (button-held) color now renders during a drag
  (issue #265).** `splitterOnHandleHover` painted `colorHandleActive` only while
  `Event.MouseButton == MouseLeft` — a branch no real frame could reach, because
  `layoutHover` bails while the mouse is locked (and a splitter drag runs under
  `MouseLock`) and always synthesizes hover events with
  `MouseButton: MouseInvalid`. The active color is now driven from the
  splitter's own drag state instead: the press paints the handle immediately and
  records a pressed flag in window state, and `splitterAmendLayout` re-paints
  the active color every frame while the lock is held, so the pressed feedback
  survives the per-frame regeneration of the handle. The flag is cleared on
  release and by the mouse lock's `Cancel` hook, and the amend paint is
  additionally guarded by the window's actual lock state, so capture lost
  without a release (issue #237) can never leave the handle permanently active.

### Added

- **Splitter interaction test suite** (`gui/view_splitter_interaction_test.go`):
  AmendLayout geometry, drag resize through the mouse lock, hover, the keyboard
  map, and the collapse buttons. Both splitter files reach 100% statement
  coverage; package `gui/` moves from 79.8% to 81.0%.

## [v0.59.2] - 2026-08-13

### Changed

- **go-glyph bumped to v1.20.2** — the face LRU now idle-evicts and the fallback
  cache evicts FIFO, so long sessions release font memory without new face
  churn; single-codepoint emoji fallback is decided from cmap coverage alone (PR
  #266).

## [v0.59.1] - 2026-08-12

### Fixed

- **GL backend tests now run cgo-free** to dodge a runtime exit crash on macOS
  (issue #162, PR #257).
- **`make` gates match golangci-lint's version string** without assuming a `v`
  prefix (PR #258), fixing local runs against newer linter releases.

### Changed

- **go-glyph bumped to v1.20.1** — the face LRU is byte-budgeted, capping
  resident memory from a single typeface (PR #260).

## [v0.59.0] - 2026-08-11

### Added

- **`Sizing` self-flags like `Padding`/`Color` (#243, #254).** `Sizing` carries
  a `set` field: the zero value (`FitFit`) is a real combination, so the flag is
  what distinguishes "unset" from an explicit `Sizing{FitFit}`. Predefined vars
  (`FitFit`…`FillFixed`) set it; read with `IsSet()`/`Or()`. The raw
  `Sizing{...}` literal now reads as unset (ergoaudit literals mode flags it).
  `DataGridCfg.Sizing` drops `Opt[gg.Sizing]` for plain `gg.Sizing` (breaking).
  An explicit `FitFit` + wrap mode in text is no longer clobbered to `FillFit`.

### Changed

- **Async present outside live resize on macOS Metal (#255).** During live
  window resize the Metal backend now presents asynchronously, decoupling
  presentation from the resize hot path instead of blocking on the drawable.
  Reduced jank and dropped frames while resizing.

## [v0.58.0] - 2026-08-11

### Added

- **`scripts/dev-loop.sh` rebuild-and-relaunch wrapper (#218).** Watches the
  module for `*.go`/`go.mod`/`go.sum` changes, rebuilds into the gitignored
  `build/dev-loop/`, kills the running app and relaunches it with preserved
  args. Build failures print loudly and keep the watcher and the last good
  binary running; INT/TERM/EXIT traps clean up the app and artifacts. Documented
  in `docs/dev-loop.md`, linked from `CONTRIBUTING.md`.

- **`TextStyle.CellWidth`, `TextStyle.CellHeight` and
  `TextStyle.NoBuiltinBoxGlyphs` (#251).** go-glyph draws box-drawing
  (U+2500–257F) and block-element (U+2580–259F) characters procedurally at cell
  size instead of through the font. Without a cell it derives one from the glyph
  advance and the run's ascent+descent, which fixes the stroke weight and the
  sub-pixel phase but leaves a sub-pixel overlap wherever the advance is
  fractional. Grid callers (terminals) build a `gui.TextStyle`, never a
  `glyph.TextStyle`, so they had no way to pass their real cell: setting
  `CellWidth`/`CellHeight` is what makes neighbouring cells abut exactly.
  `NoBuiltinBoxGlyphs` keeps the font's own glyphs instead. All three map
  through both converters — the rich-text path (`toGlyphStyle`) and the
  plain-text measure/render path (`glyphconv.GuiStyleToGlyphConfig`), which is
  what `TermGrid` renders through. Zero/false keep today's behaviour.

## [v0.57.0] - 2026-08-10

### Changed

- **`Padding` self-flags; `Opt[Padding]` and `SomeP` removed (breaking, #243).**
  `Padding` carries a `set` flag like `Color`: the zero value is "unset" (theme
  default applies), `PaddingNone` is an explicitly-set zero, and values are
  built with `NewPadding` / `PadAll` / `PaddingNone`. `IsSet()` and `Or(def)`
  replace `Opt[Padding]`'s `IsSet()`/`Get(def)` on the 50 affected Cfg fields;
  read sites moved from `.Get(Padding{})` to `.Or(PaddingNone)`. Raw
  `Padding{...}` or `Color{...}` literals read as unset, so
  `make ergonomics-audit` now also runs the new `-mode literals` gate over the
  whole tree (defining files and the empty `Color{}` sentinel exempt);
  `-mode opt` exempts Padding-typed fields like it exempts Color. `ThemeMaker`
  stamps the flag on its `cfg.Padding*` copies so a resolved theme value never
  reads as unset. Breaking for consumers of `SomeP` and `Opt[Padding]`:
  go-charts, go-edit, go-kite, go-map and go-term bump together.

- **Exported API surface reduced (~1,400 symbols) as the v1.0 freeze
  (breaking).** `tools/exportaudit` classifies every export in `gui/` by who
  references it — sibling repos (consumers), examples, tests, other gui
  packages, or nothing — and its sweep mode unexported everything referenced
  only inside its own package, its tests, or nowhere at all. Internal helpers,
  dead constants, and swept types are now lowercase. `make export-audit` gates
  the surface: any export whose references stay inside `gui/` fails unless
  marked `// exportaudit:keep`, and the marker explains why (json reflection,
  stdlib interface conformance, same-name collisions, signature reachability).
  Cross-package plumbing (gui/backend ↔ gui) is deferred and reported as
  advisory. 449 files changed; consumers at v0.56.0 are unaffected unless they
  referenced internal-only names.

### Fixed

- **Windows: window and tray icons now render (#235).** `hicon.FromPNG` passed a
  NULL `hbmMask` to `CreateIconIndirect`, which Win32 requires — the call always
  failed, so the title bar, taskbar, alt-tab, and SNI tray icons fell back to
  the shell's generic icon whether or not the app set `WindowCfg.IconPNG`. The
  function now creates a zeroed 1bpp mask (fully opaque), letting the 32bpp
  color bitmap's alpha shape the icon, so Windows honors `IconPNG` like the
  other platforms.

- **Windows: a revoked mouse capture no longer leaves a drag stuck (#110).**
  Win32 can take capture away without ever sending the matching button-up — a
  system modal or UAC prompt, Ctrl+Alt+Del, Win+L, another process calling
  `SetCapture`, a debugger breaking in. `WM_CAPTURECHANGED` was unhandled, so
  the `MouseLock` never cleared and every later mouse move kept driving the drag
  with no button held (text selection tracking the cursor, a splitter or slider
  still following it). The message now cancels the drag. Our own
  `ReleaseCapture` raises the same message, so the backend tracks capture
  ownership and only cancels on an involuntary loss.

### Added

- **`MouseLockCfg.Cancel`** — a `func(*Window)` hook that unwinds a drag ending
  without a release. `MouseUp` cannot serve: it _commits_, so synthesising one
  on capture loss would dock a panel or reorder a list the user never dropped.
  `(*Window).MouseCancel` unlocks and runs the hook; widgets whose state is just
  the lock need no hook. Wired for text/RTF/input selection (stops the
  edge-scroll animation), sliders, the color picker, datagrid column resize,
  dock drags, and drag-reorder.

- **`make export-audit` now actually fails the build when the gate trips.** The
  authoritative consumer scan previously ran under `|| true`, so a real gate
  failure (offenders found with siblings present) exited zero. It now only
  tolerates missing sibling repos; CI shallow-clones the five consumers into
  `../` and runs the authoritative scan, so the freeze is enforced on every PR.
  See `docs/specs/exportaudit-surface-policy.md` for the accepted
  `internal`-class policy.

## [v0.56.0] - 2026-08-09

### Changed

- **Widget IDs are per-scope, not window-global (breaking).** `Shape.ID` is now
  a **leaf**; the identity every store and lookup keys on is the **effective
  ID** — the leaf joined to the IDs of its ID-bearing ancestors. Two panels may
  each hold `Input{ID: "name"}`; they resolve to `settings:name` and
  `profile:name` and keep separate focus, scroll, hover and per-widget state.
  Only explicit ancestor IDs join — never tree position, never a child index —
  so identity survives a reorder and changes only when an app changes an
  ancestor's ID. A leaf that already contains `:` is **absolute** and is left
  alone, which is why existing `ScopeID` composites keep working.

  **Semantic break:** `SetFocus`, `ScrollVerticalTo`, `FindByID`, `IsFocus`, and
  the `Test*` helpers take the effective ID. A call that passes a bare leaf
  stops matching as soon as an ancestor of that widget carries an ID — pass the
  full path (`"settings:name"`). Signatures are unchanged, so the compiler will
  not find these; `(*Window).TestDuplicateIDs` and `GOGUI_DEBUG=1` report the
  identities a frame actually has. `gui/datagrid` is unchanged: its children are
  absolute multi-part IDs by design (a header cell's column is recovered by
  reverse-parsing its ID), so two grids sharing a `cfg.ID` still collide. See
  `docs/specs/widget-id-per-scope-uniqueness.md`.

  Identity resolution is allocation-neutral: the joins are memoized per window,
  so `BenchmarkViewFrame` holds its previous allocs/op. `Shape` itself grew by
  one string (280 → 296 bytes), which crosses a size class and costs 32 bytes
  per shape.

- **Framework-internal identities are absolute.** The inspector tree
  (`gui:inspector:tree`), the RTF link menu (`gui:rtf:link_menu`) and the
  dialog's focus ID are written from event handlers, outside layout generation,
  so they cannot be resolved against the tree they are mounted in. They now
  carry `:` and are therefore their own identity wherever they are mounted.
  Without this the inspector's wireframe followed a pick while its tree never
  selected the row, and dialog focus landed on nothing.

- **Accessibility labels announce a name, not a path.** Where a widget falls
  back to its ID for `A11YLabel`, only the last segment is announced —
  `settings:name` is read as "name". An explicit `A11YLabel` is untouched,
  including one containing a colon.

### Added

- **`(*Window).EffID` and `EventCtx.EffID`.** Resolve a leaf ID to the identity
  the framework's stores use. `w.EffID(cfg.ID)` is for a widget that reads its
  own state _during_ `GenerateLayout` — a combobox's open flag decides the
  subtree it returns, so it cannot wait for the resolve pass. `ctx.EffID(leaf)`
  is for a factory that builds its tree eagerly, with no `Window` in hand
  (`Input`), and resolves against the shape the event is on.

- **`gui.DebugUnscopedIDs`.** Opt-in dev-mode category reporting a focusable or
  scrollable widget with no ID-bearing ancestor — its leaf is still a
  window-global name, so it cannot be reused elsewhere in the window.
  Deliberately outside `DebugAll`: it reports a design property, not a defect.
  Enable with `gui.DebugCategories(gui.DebugUnscopedIDs)`.

- **`gui.ScopeID` and `gui.ScopeIDN`.** The one way to compose an inner widget's
  ID. An ID is now a `:`-joined path (`grid:header:name:resize`); the owner may
  itself be composed, and composition is associative. `ScopeIDN` appends a
  numeric last segment without materialising the number as its own string, for
  loop-derived identity (a list row, a calendar day, a radio option). Both cost
  exactly one allocation, asserted with `testing.AllocsPerRun`. They replace ~70
  hand-rolled concatenations that used five different separators. There is no
  escaping: a **part** — a row key, a heading slug, any leaf value fed into a
  composition — must not contain `:` and keeps its own spelling. See
  `docs/specs/widget-id-scoping.md`.

- **`ergoaudit -mode ids`.** Part of `make ergo-audit`; fails on any hand-rolled
  ID composition in `gui/`. It flags both producers and, importantly, consumers
  — `w.IsFocus(cfg.ID+"_popup")` rebuilds an ID the producer may have moved, and
  that is how the two drift. Exempt a line with `ergoaudit:id-part` (a leaf
  part) or `ergoaudit:not-an-id` (not a widget ID at all).

- **`(*Window).TestDuplicateIDs`.** The assertable form of what `GOGUI_DEBUG=1`
  prints: renders the window and returns every identity defect in the frame —
  duplicate IDs, and ID-less shapes whose focus, scroll, or `OnMouseLeave`
  silently does nothing. Unlike `TestUnconsumedEvents` it dispatches nothing and
  fires no callbacks, so it is safe on a window an assertion still depends on.
  It sees the window as rendered, so drive the app into each state and check
  again.

- **`ErrTestNoScrollRoom`.** `TestScroll` on a scroll container whose content
  already fits now says so, instead of returning the generic `ErrTestUnhandled`.
  The two conditions call for opposite fixes — one means the fixture never had
  anything to scroll, the other means the container is real but already pinned
  at its limit — and the message carries the content and viewport sizes that
  decided it.

### Changed

- **Composed widget IDs are respelled with a single `:` separator.** Every
  framework-composed inner ID changed. Nothing in the public API changed —
  `SetFocus`, `ScrollVerticalTo`, `FindByID` and the test helpers still take the
  whole composed string — but an application that hardcoded one of these
  spellings (most likely in its own tests) must update it. There is no compile
  error for that; the symptom is a test that no longer finds its widget.

  | Widget                 | Was                           | Now                           |
  | ---------------------- | ----------------------------- | ----------------------------- |
  | Tab control button     | `tc_main_settings`            | `main:tab:settings`           |
  | ListBox item           | `lb_list_apple`               | `list:item:apple`             |
  | Tree row               | `tr_tree_node1`               | `tree:row:node1`              |
  | Radio group option     | `rbg/0`                       | `rbg:opt:0`                   |
  | Markdown table         | `doc.table.0`                 | `doc:table:0`                 |
  | Markdown heading       | `some-heading`                | `doc:h:some-heading`          |
  | Calendar day           | `dp.day.14`                   | `dp:day:14`                   |
  | Color picker channel   | `cp.rgb.0`, `cp.hex`          | `cp:rgb:0`, `cp:hex`          |
  | Dialog buttons         | `__gui_dialog__/1`            | `__gui_dialog__:1`            |
  | Context menu / tooltip | `menu_popup`                  | `menu:popup`                  |
  | Numeric input parts    | `n_field`, `n_step_up`        | `n:field`, `n:step_up`        |
  | Select / combobox      | `sel.dropdown`                | `sel:dropdown`                |
  | Date input parts       | `d.input`, `d.picker`         | `d:input`, `d:picker`         |
  | Toast buttons          | `toast_1:action`              | `toast:1:action`              |
  | Animation keys         | `skeleton_x`, `spell-check-x` | `skeleton:x`, `spell-check:x` |
  | Dock node IDs          | `main_new_editor`             | `main:new:editor`             |
  | Inspector tree paths   | `0.3.1`                       | `0:3:1`                       |

  Dock node IDs are generated during drag-drop and stored in `DockNode`, which
  an application may persist. Nothing parses them, so an old saved layout keeps
  working — it simply carries old-format IDs alongside newly minted ones.

  Unchanged: everything already spelled with `:` (`cmdbtn:`, `dock_close:`,
  datagrid cells and headers, splitter handles, scroll IDs), and data-derived
  row keys (`__auto_`, `__draft_`, `__src_o_`), which are parts rather than
  scopes.

### Fixed

- **Two `Markdown` widgets in one window no longer share IDs.** A heading's ID
  was its bare anchor slug, a code block's copy button was keyed by block index
  alone, and the document-level copy button was the constant `md_cp_doc` — so
  two documents in one window collided on all three. All are now scoped to the
  document. Two identical headings in a _single_ document still collide, but the
  duplicate audit now reports that instead of letting it pass silently.

- **Composite widgets no longer duplicate their own ID.** `Input` put `cfg.ID`
  on both its outer container and its inner text shape, and every `Markdown`
  paragraph claimed the markdown widget's ID — so a `GOGUI_DEBUG=1` run of a
  normal application drowned in duplicate-ID reports it could do nothing about,
  and the check could not be trusted to mean anything. An inner shape that needs
  the owning widget's focus and spell-check state now references it instead of
  claiming its identity, and a markdown document's container owns the ID while
  its blocks are tied to it through the existing markdown selection key.
  `FindByID` on a markdown ID now resolves to the container rather than to the
  first paragraph. `ID` is once again an identity that is unique per window.
  Rationale and the remaining ergonomics question:
  `docs/specs/widget-id-scoping.md`.

- **`Markdown` document selection works.** Giving the markdown container the
  widget's ID also fixed a latent defect: the container carried no ID at all,
  and both its selection hook and its key handler bail out on an empty one — so
  drag-selection across blocks, `Ctrl+A` and `Ctrl+C` were dead on every
  `Focusable` markdown document.

- **Wrapped text has a plausible height in headless tests.** Text is not
  zero-sized without a `TextMeasurer` — the width falls back to `0.6em` per rune
  — but the post-sizing pass that turns a paragraph into N lines bailed out, so
  wrapped text kept its one-line seed however much text it held. A scroll
  container around a wall of text therefore never overflowed, and `TestScroll`
  failed for a reason that had nothing to do with the widget under test. Heights
  are now estimated from the same per-rune approximation. Still an estimate:
  assert on structure and overflow, not on pixels.

- `examples/scroll_demo` now asserts that its panel scrolls and that the
  percentage buttons move it, the conversion §4.8 of
  `docs/specs/developer-ergonomics.md` attempted and abandoned.

## [v0.55.0] - 2026-08-08

### Changed

- **BREAKING: one event rule. Nothing is marked handled for you.** A callback
  that acts on an event calls `ctx.Consume()`; a callback that does not, lets
  the event travel on. This is now true of every callback, closing §4.3b of
  `docs/specs/developer-ergonomics.md` — the last open item in that spec.

  Until now `OnClick`, `OnChar`, `OnMouseUp`, `OnGesture` and `OnFileDrop` were
  "consume-class": dispatch marked their events handled _before_ the callback
  ran, and the callback opted out with `ctx.Bubble()`. Everything else consumed
  explicitly. Which of the two you had written was invisible in the signature.

  ```go
  // Before — the pre-mark absorbed the click
  OnClick: func(ctx gui.EventCtx) {
      dismiss(ctx.Window)
  },

  // After
  OnClick: func(ctx gui.EventCtx) {
      dismiss(ctx.Window)
      ctx.Consume()
  },
  ```

- **BREAKING: `EventCtx.Bubble()` is deleted.** Declining is what saying nothing
  already means. Every call site is a compile error; deleting the call is the
  fix.

- **BREAKING: `(*Window).TestEventCollapse` is now `TestUnconsumedEvents`.** It
  measured the cost of a collapse that has since happened. It now reports
  handlers that act without consuming while an ancestor would also receive the
  event, and `gui.Debug(true)` reports the same as they dispatch.

**Upgrading.** The compile-time half is loud — `Bubble()` disappears. The other
half is silent, so search for **empty** consume-class handlers: an `OnClick`
with an empty body used to be a working click-blocker, and now blocks nothing.
Overlays, backdrops, popups and cards that stop clicks reaching what they cover
all need a real `ctx.Consume()`. Four of the five handlers this change broke
inside go-gui were exactly that. `docs/migration-eventctx.md` has the full
guide; `TestUnconsumedEvents` sweeps a window for the rest.

Also in this release, as precursors: 14 sites across 12 files that wrote
`ctx.Event.IsHandled = true` now call `ctx.Consume()`, the unconsumed-event
check counts focus-stealing ancestors (a focusable ancestor takes focus on an
unconsumed click, with no `OnClick` involved anywhere), and
`examples/scroll_demo` no longer gives five buttons the same ID.

## [v0.54.0] - 2026-08-08

Developer-ergonomics phases 2–5 (`docs/specs/developer-ergonomics.md`), shipped
as PRs #195–#202. This completes the spec: every phase is now on `main`.

**This release is breaking**, in three ways, all mechanical:

1. **17 event callbacks take an `EventCtx`** instead of a trailing `*Window`,
   and four no longer leak a raw `*Event`.
2. **`RtfCfg` is now `RTFCfg`**, along with the `Rtf*` fields on `Shape.TC`.
3. **The five flat state-color fields are deleted from six `Cfg`s** and replaced
   by one `Colors ColorSet`.

**Upgrading:** each item under _Changed_ carries its own before/after. The
`EventCtx` conversion has a tool — `docs/migration-eventctx.md` documents the
`-fields` mode that drove it. The color change is the one to read carefully:
`Color` still means "resting fill only" and does **not** pin hover, click and
focus; `Flat(c)` is how you ask for a widget that does not react.

Everything else — the app-testing API, `ColorSet` itself, the event-collapse
check and the example audit — is additive.

### Changed

- **BREAKING: 17 event callbacks now take an `EventCtx`** instead of a trailing
  `*Window`, and the four that leaked a raw `*Event` no longer do. The affected
  fields are `OnAction`, `OnChange`, `OnColumnPinChange`, `OnCopyRows`,
  `OnItemClick`, `OnLayoutChange`, `OnPanelClose`, `OnPanelSelect`, `OnReorder`,
  `OnReset`, `OnSelect`, `OnSubmit`, `OnTextCommit`, `OnToggle` and
  `OnValueCommit`.

  The rule is uniform: `EventCtx` replaces the `*Layout`, `*Event` and `*Window`
  parameters and lands last, while payloads keep their order and results are
  untouched. So `TableCfg.OnSelect func(map[int]bool, int, *Event, *Window)`
  becomes `func(map[int]bool, int, EventCtx)`, and `GridCfg.OnCopyRows` becomes
  `func([]GridRow, EventCtx) (string, bool)` — returning a value was never a
  reason to keep the old shape.

  This finishes what the v0.52.0 `EventCtx` refactor started. Of the 30 distinct
  signatures still carrying a `*Window` or a raw `*Event`, 17 converted; the
  remaining 13 are deliberate. Twelve fire from a timer tick, a dialog
  completion or a lifecycle transition (`OnInit`, `OnCloseRequest`, `OnOkYes`,
  `OnCancelNo`, `OnDismiss`, `OnReply`, `OnLazyLoad`, `OnValue`, four `OnDone`
  variants) — **there is no event** at those points, and handing them an
  `EventCtx` would promise a permanently-nil `ctx.Event`. The thirteenth,
  `Window.OnEvent`, is a raw escape hatch by design.

  Where a widget dispatches one of these from inside an already-converted
  callback, the full context is now passed through rather than a synthesized
  one, so `OnPanelClose` and friends receive the layout they fire from —
  strictly more than the old `*Window` carried.

  Migration: `docs/migration-eventctx.md`, including the new `-fields` mode that
  drove this round. No sibling repo is affected — none of the five uses any of
  the 17.

- **BREAKING: `RtfCfg` is now `RTFCfg`**, and the exported `Rtf*` fields on
  `Shape.TC` are now `RTF*` — `RTFRuns`, `RTFLayout`, `RTFFlatText`,
  `RTFBaseStyle`, `RTFLineSpacing`. `RTF(cfg RtfCfg)` was the only factory whose
  name disagreed with its `Cfg` in casing. The `Shape` fields are renamed
  alongside it because leaving them as `Rtf*` next to a new `RTFCfg` would trade
  one inconsistency for another inside the same widget. Unexported identifiers
  (`renderRtf`, `hasRtfLayout`) are unchanged.

  The casing split was not only cosmetic. `requiredid` derives a factory name by
  trimming `Cfg`, so `RtfCfg` implied a factory called `Rtf`, never matched the
  real `RTF`, and every `gui.RTF(gui.RtfCfg{…})` literal was skipped as
  possibly-wrapped. The rename re-enabled the check and surfaced four focusable
  RTF blocks in the showcase with no `ID` — keyboard-unreachable since they were
  written. All four now have one.

- **BREAKING: the five flat state-color fields are deleted from the six `Cfg`s
  that carried the full set** — `ButtonCfg`, `SwitchCfg`, `ToggleCfg`,
  `RadioCfg`, `InputDateCfg`, `DatePickerCfg`. `ColorHover`, `ColorFocus`,
  `ColorClick`, `ColorBorder` and `ColorBorderFocus` are replaced by
  `Colors ColorSet` on each. `Color` stays as the shorthand for `Colors.Base`.

  Migration is mechanical:

  ```go
  // before
  gui.ButtonCfg{Color: c, ColorHover: c, ColorClick: c,
      ColorFocus: c, ColorBorder: c, ColorBorderFocus: c}
  // after
  gui.ButtonCfg{Colors: gui.Flat(c)}

  // before
  gui.ButtonCfg{Color: bg, ColorBorder: b, ColorBorderFocus: a}
  // after
  gui.ButtonCfg{Colors: gui.ColorSet{Base: bg, Border: b, BorderFocus: a}}
  ```

  The 24 `Cfg`s holding a partial set (four fields down to one) keep the fields
  they had. "Carries the full five" is the line, because that is where
  `ColorSet` fits exactly.

  **`Color` does not pin the interactive states.** It sets the resting color and
  leaves hover, click and focus to the theme, exactly as before. `Flat(c)` is
  the deliberate opt-in for a widget that should not react.

### Fixed

- **Four focusable RTF blocks in `examples/showcase` had no `ID`**, so they
  rendered and clicked but never joined the tab order. Found by `requiredid`
  once the `RTFCfg` rename let it see them.

- **BREAKING: `DataGridCfg.OnCellFormat` and `OnDetailRowView` are now
  `CellFormat` and `DetailRowView`.** Neither is an event callback — both are
  view builders that return a value (`GridCellFormat` and `gg.View`), so the
  `On*` prefix misdescribed them. No reference to either name exists in any
  sibling repo, so the rename costs nothing outside this module.

### Added

- **Examples now teach `FillFill` instead of manual viewport math** (spec §4.8).
  `WindowSize()` in `examples/` drops from 45 files / 50 call sites to 8 files /
  9 call sites. The dominant pattern — fetch the window size, set it as the
  root's `Width`/`Height`, mark the root `FixedFixed` — is now just
  `Sizing: gui.FillFill`. `todo`'s hand-computed `cardView(ww-24, wh-24)` and
  `listbox`'s `Height: wh - 70` are gone; `minesweeper` no longer threads the
  viewport through both screens.

  The 8 remaining calls are allowlisted and each carries a one-line reason at
  the call site: canvas, particle, game and virtualized-list demos that compute
  positions in pixel space. `calculator` is one of them for a reason worth
  knowing — its root is a `Canvas`, which does not arrange its content, so a
  `FillFill` child of it measures 0×0.

  Three example tests moved from "did not panic" to real state assertions using
  the phase-2 testing API: `key_up_demo`, `todo` and `dialogs`.

- **An event-collapse check under `gui.Debug(true)`**, plus
  `(*Window).TestEventCollapse` to sweep a window with it armed. It reports
  consume-class callbacks that rely on automatic handling while an ancestor
  would also have received the event — the sites that would silently start
  firing twice if the consume/notify split were ever collapsed into one rule
  (spec §4.3b).

  This class of risk cannot be counted from source, because "would an ancestor
  have received it" is a property of the layout tree at dispatch time. The check
  therefore runs after each consume-class callback and replays that event's real
  dispatch condition — hit test plus `ClickButton` filter for `OnClick`, the
  centroid for `OnGesture`, focus for `OnChar` — against each ancestor.

  Sweeping 37 examples finds **18** such sites, against the 138 consume-class
  sites a source count had suggested, and nearly every one is a go-gui widget
  nested in another go-gui widget rather than application code. go-charts and
  go-map sweep clean. Findings and method: spec §4.3.3.

  Diagnostics only; nothing about dispatch changes. Off by default, and the
  event benchmarks stay at zero allocations.

- **`ColorSet` and `Flat`** (`gui/color_set.go`), as the `Colors` field. Landed
  on `ButtonCfg` in phase 3 and widened to six widgets in phase 4 (see
  "Changed"). Groups `Base`, `Hover`, `Click`, `Focus`, `Border` and
  `BorderFocus` into one value; `Flat(c)` pins all six for a widget that should
  not react to the pointer. `Base` backs the three interactive states,
  `BorderFocus` falls back to `Border`, and anything still unset falls through
  to the theme. An assigned flat `Color*` field takes precedence over the set,
  so existing code keeps its appearance. `examples/todo`'s accent button drops
  from six color lines to one. See [Styling widgets](README.md#styling-widgets).

  Fields are plain `Color`, reversing the spec's Q5: `Color` already carries its
  own set flag, so `Opt[Color]` would have added a second, competing notion of
  unset.

- **App-testing API in package `gui`.** `NewTestWindow` builds a backendless
  window; `TestRender`, `TestClick`, `TestFocus`, `TestKey`, `TestType`,
  `TestTab`, `TestScroll` and `TestScrollOffset` drive it by widget `ID`. Each
  synthesizes a real `Event` and pushes it through `Window.EventFn`, then
  settles a frame — so a test observes hit-testing, focus traversal and scroll
  clamping, not just the callback. Failures are returned as errors
  (`ErrTestNoSuchID`, `ErrTestDisabled`, `ErrTestNotFocusable`,
  `ErrTestNoHandler`, `ErrTestNotVisible`, `ErrTestUnhandled`), because in a
  test they are assertion failures, not programmer errors. See
  [Testing your app](README.md#testing-your-app).

  It lives in `gui` rather than a `guitest` subpackage because driving a click
  requires `Shape.events`, and a sibling package could only reach it by
  exporting that field permanently.

- **Nested-scroll regression gate** (`gui/scroll_nested_test.go`). Pins the
  current contract: an inner scrollable pinned at its limit declines the scroll
  so it cascades to the enclosing container. This is the one place the planned
  one-event-class collapse changes behavior with no compile error, and it was
  untestable before `TestScroll`/`TestScrollOffset` existed.

## [v0.53.0] - 2026-08-07

Developer-ergonomics phase 1 (`docs/specs/developer-ergonomics.md`), shipped as
PRs #189–#193.

A missing widget `ID` used to be invisible: the widget rendered, it clicked, and
it silently never joined the tab order. This release makes that a panic at
construction, a build-time diagnostic, and a runtime gate — and fixes the twelve
places the library itself got it wrong.

**Upgrading:** the nine input factories listed under _Changed_ now panic on an
empty `Cfg.ID`. Give each control a window-unique `ID`, or set
`FocusDisabled: true` on a control that is genuinely decorative. Measured
against the five sibling repos, this forces three edits in total (go-charts 1,
go-map 2), all in example code.

### Fixed

- **Twelve first-party widgets shipped buttons no keyboard user could reach**
  (developer-ergonomics §4.2, phase 1). Focus traversal is keyed by `ID`, so a
  focusable widget without one renders, clicks, and silently never joins the tab
  order. Each of these carried an `OnClick` and had no `ID`: the toast action
  and dismiss buttons, the date picker's month toggle and prev/next arrows, the
  date input's calendar button, the markdown code-block copy button, the
  overflow panel's trigger, the time-travel freeze button, and the DataGrid's
  header, CRUD, pager, filter-clear, column-chooser, and source-paging controls.

  IDs are **namespaced by the owning widget's own ID** rather than fixed
  strings, because these widgets can appear more than once in a window — two
  toasts, two date pickers, two grids. A constant `ID` would have replaced an
  unreachable button with a duplicate one, collapsing both onto a single tab
  stop and a single state slot. New `toastBtnID` helper;
  `dataGridIndicatorButton`, `dataGridOrderButton`, and `ttButton` take an `id`
  parameter.

  Those three take `id` as a parameter rather than deriving it from their
  `label` because the labels are not stable identity: DataGrid's come from
  `gg.ActiveLocale`, and the freeze button's alternates between "Pause" and
  "Resume". An ID tracking the label would move the widget's focus and state on
  a locale switch or a state change.

  One case went the other way: the date picker's out-of-month calendar cell is
  blank, disabled, and handler-less, so it now sets `FocusDisabled: true` rather
  than being given an ID for empty space. That is the decorative case the
  opt-out exists for.

### Added

- **Two preventive identity checks** (developer-ergonomics §4.9 and §4.1, phase
  1). Both found **zero** live defects in this repo, which is the point of
  landing them: the twelve a11y defects above all reached release because
  nothing looked.

  `tools/requiredid` gained `checkScrollableID`, reporting a literal that sets
  `Scrollable: true` with no `ID`. Scroll offsets are keyed by `ID`, so every
  ID-less scrollable in a window shares the key `""` and they scroll in lockstep
  — visible only once a second one exists, which is how it survives review. It
  mirrors `checkFocusableID` and keys on the field in the literal rather than a
  tag: a scrollable container is the exception, and a `gui:"required"` tag on
  `ContainerCfg.ID` would invert the default and flag the common case. The
  runtime half, `RequireScrollID`, was already wired.

  The `gui.Debug` gate gained an `OnMouseLeave`-without-`ID` check. This is a
  fourth ID-keyed behaviour the spec did not enumerate: `layoutMouseLeave`
  tracks the cursor through a map keyed by `ID` and, unlike the focus path, has
  **no `Focusable` precondition**. A `FocusDisabled` control carrying an
  `OnMouseLeave` is therefore still silently broken — the decorative opt-out
  covers focus, not identity — and neither the focus check nor `requireFocusID`
  catches it.

- **`ergoaudit -fix` codemod** (developer-ergonomics §8, phase 1). Tools-only;
  no shipped library code changes. Mode `focus` can now rewrite the literals it
  reports, inserting a generated `ID` into each. Phase 1 tags nine `Cfg` types
  with `gui:"required"`, which turns 111 literals in this repo into build
  failures at once — and hand-editing 111 literals is how a typo'd ID reaches
  `main` looking like intent. `-fix-dry-run` reports the proposed IDs without
  writing; `-fix-only` and `-fix-exclude` are regexps over repo-relative paths.
  `make ergo-fix-dry` and `make ergo-fix` wrap the phase-1 invocation, scoped to
  `_test.go` and `examples/` — go-gui's own widget defects get hand-chosen IDs,
  because a shipped widget's `ID` is public identity, not scaffolding.

  IDs come from a nested `TextCfg` label where there is one (`ButtonCfg`, the
  dominant broken type, has no `Text` field — its label lives in `Content`),
  otherwise the enclosing function, otherwise the `Cfg` type; each is made
  unique within its file and checked against IDs already written by hand. The
  rewrite is a byte splice applied in reverse offset order and then `gofmt`ed,
  rather than a reprint of the whole AST, so the diff is the insertions and
  nothing else. Single-line literals stay on one line.

  This does not reopen §5.1. That section rejects IDs _computed at runtime from
  tree position_, which have no source-level existence and silently reassign
  scroll offsets and dropdown state when a sibling is inserted. An ID generated
  into the source is an ordinary ID a human reads, reviews, and edits.

- **`gui:"required,focus"` tag option** (developer-ergonomics §4.2, phase 1).
  `tools/requiredid` now reads options off the `gui` tag: the `focus` option
  scopes the requirement to controls that join focus traversal, so a literal
  setting `FocusDisabled: true` satisfies it. A plain `gui:"required"` field is
  unaffected and stays required regardless — its state is keyed by `ID` whether
  or not the control takes focus.

  `ergoaudit` parses the tag rather than matching the bare `gui:"required"`
  string, which read any tag carrying options as absent. It also no longer
  counts a literal handed to a _different_ factory as broken:
  `CommandButton(cmdID, ButtonCfg{})` fills the `ID` in itself, so the empty
  `ID` at that call site proves nothing. That was one false positive in the 111,
  and neither `requiredid` (which matches on the factory name) nor the runtime
  guard (which runs after the wrapper has filled the field) ever agreed with it.

- **`gui.Debug(bool)` dev-mode diagnostics gate** (developer-ergonomics §4.1,
  phase 1). Generalizes the focus-only `GOGUI_FOCUS_DEBUG` check into one gate
  that audits each composed frame for identity defects the library otherwise
  reports in no way at all: two shapes sharing an `ID`, a focusable shape with
  no `ID` (renders and clicks, never joins the tab order), and a scrollable
  shape with no `ID` (shares one scroll offset with every other ID-less
  scrollable in the window). Findings go to stderr **once per finding per
  window** — these run per frame, so an undeduplicated warning would emit at the
  frame rate and be a reason to turn the gate back off. Cycling the gate off and
  on clears that memory, so a re-enabled gate reports current state. Set at
  startup by `GOGUI_DEBUG=1`; `GOGUI_FOCUS_DEBUG=1` still works. The flag is an
  `atomic.Bool` because `Debug` makes it mutable at runtime and the checks read
  it from the frame goroutine. Measured cost when off: no change in
  `BenchmarkViewFrame` allocs (202 / 802 allocs/op unchanged) or ns/op.

### Changed

- **Nine focusable-by-default widgets now require an `ID`**
  (developer-ergonomics §4.2, phase 1). `Button`, `Input`, `InputDate`,
  `NumericInput`, `RadioButtonGroup`, `Radio`, `Select`, `Switch`, and `Toggle`
  panic when handed an empty `Cfg.ID`. **This is a breaking change**: a config
  that used to render a control which quietly never took focus now fails on the
  first frame instead. That is the point — the old behaviour had no signal at
  all, so the defect survived to release twelve times in this repo alone.

  The requirement is scoped, not absolute. A control marked
  `FocusDisabled: true` never joins the tab order, so it has no identity to name
  and is exempt — the decorative case, such as the date picker's blank
  out-of-month cell. Both halves are expressed by a new tag option,
  `gui:"required,focus"`, which `tools/requiredid` reads statically and the
  unexported `requireFocusID` enforces at runtime.

  111 literals across this repo's tests and examples were updated by
  `make ergo-fix` rather than by hand.

- **`State[T]` names both types when it panics.** A window holding the wrong
  state type still panics — that is a programmer error discoverable on the first
  frame, not a runtime condition worth threading through every view function —
  but the message now reports the type held alongside the type requested instead
  of Go's bare interface-conversion text.

### Removed

- **`RequireFocusID`** (developer-ergonomics §4.2). Dead exported guard: zero
  call sites in go-gui, tests included, and zero across all five sibling repos,
  despite fourteen files exposing a `Focusable bool` field. Retaining it implied
  a check that never ran, which reads like coverage. Its static case belongs to
  the `requiredid` analyzer, which reports at build time with the `Cfg` type
  named; its dynamic case belongs to `RequireID` and the new debug gate. No
  consumer action required.

## [v0.52.1] - 2026-08-07

### Fixed

- **`tools/eventctx` converts callbacks declared as methods.** The rewriter
  skipped every `func` with a receiver, so a repo that wires its callbacks as
  method values (`fv.internalClick`) got a converted call site and an
  unconverted declaration — go-charts hit 109 unresolvable build errors. The
  scanner now classifies methods alongside plain functions, the value-use scan
  ignores the method name in a call's selector, and the call folder accepts a
  selector callee. Tools-only: no shipped library code changed, so v0.52.0 and
  v0.52.1 are identical for anyone who is not running the migration.

## [v0.52.0] - 2026-08-07

### Changed

- **BREAKING: event callbacks take a single `EventCtx`.** Every callback that
  took `(*Layout, *Event, *Window)` now takes `func(EventCtx)`, where `EventCtx`
  bundles `Layout`, `Event` and `Window`. Payload carriers keep their payload as
  a leading argument: `func(*Layout, string, *Window)` becomes
  `func(string, EventCtx)` and `func(T, *Event, *Window)` becomes
  `func(T, EventCtx)`. `func(*Layout, *Window)` callbacks (`OnScroll`,
  `AmendLayout`, `InputCfg.OnBlur`) become `func(EventCtx)` with a nil
  `ctx.Event`. `func(*Window)` lifecycle callbacks, `OnDraw` and
  `Window.OnEvent` are unchanged. `EventCtx` is passed by value — three pointers
  in registers — so the zero-allocation layout, render and event phases stay at
  zero allocations.
- **BREAKING: consuming events are handled by default.** `OnClick`, `OnChar`,
  `OnMouseUp`, `OnGesture` and `OnFileDrop` are marked handled by dispatch
  before the callback runs; drop the trailing `e.IsHandled = true` and call
  `ctx.Bubble()` on paths that mean "not mine". Every other callback is
  unchanged and still calls `ctx.Consume()` to stop propagation — notably
  `OnKeyDown` (which must leave Tab and accelerators alone), the
  hover/move/leave notifications, and `OnMouseScroll` (whose cascade to the
  enclosing scroll container depends on the event staying unhandled). One
  visible consequence: `Window.OnEvent` no longer sees clicks, characters,
  mouse-ups, file drops or gestures that a widget handled.
- **`EventCtx` methods.** `Consume()` marks handled, `Bubble()` unmarks it,
  `Handled()` reports the flag. All three are nil-`Event` safe, so they are
  callable from `AmendLayout` and `OnScroll` without a guard. `Bubble()` opts
  out of the current callback's auto-consume only; it does not un-handle an
  event an earlier handler consumed.
- **Migration guide:** `docs/migration-eventctx.md`. The rewrite tool that
  produced this change ships as `tools/eventctx`.

### Removed

- **`spacebarToClick`, `enterToClick` and `leftClickOnly`.** The three
  deprecated wrapper helpers had no production call sites; the `ClickOnSpace`,
  `ClickOnEnter` and `ClickButton` dispatch fields superseded them and avoid the
  per-frame closure allocation.

## [v0.51.1] - 2026-08-07

### Changed

- **go-glyph v1.18.2 → v1.19.0.** Brings cap-height-matched fallback glyphs
  (fallback letters as tall as the primary's), upright-face preference on weight
  ties, and scratch `CachedGlyph` slice reuse (no per-frame allocation in the
  terminal steady-state path).

## [v0.51.0] - 2026-08-06

### Changed

- **`Event.ScrollX`/`ScrollY` now have a defined unit.** For a discrete mouse
  wheel (`ScrollPrecise` false) they carry **lines of text**, and every backend
  converts its native representation to the platform's lines-per- notch —
  roughly three on all of them. Precise/trackpad deltas are unchanged and still
  carry pre-scaled points of finger travel; consumers that care must branch on
  `ScrollPrecise`. The field previously had no documented unit and each backend
  picked its own: Metal pre-scaled a notch to 2.5 while Win32 and X11 emitted a
  bare 1.0, so the same gesture scrolled 2.5x further on macOS than on Windows
  or Linux. Embedders that scaled `ScrollY` by their own constant (rather than
  by `Theme.ScrollMultiplier`) should re-check their factor.

### Fixed

- **Mouse wheel ignored the Windows scroll-speed setting.** The Win32 backend
  reported a bare notch count and never read `SPI_GETWHEELSCROLLLINES` (Control
  Panel → Mouse → Wheel, three lines by default), so an embedder scaling by its
  own per-unit constant scrolled a fraction of what the user asked for. The
  setting is now honoured, including the "one screen at a time"
  (`WHEEL_PAGESCROLL`) sentinel, and re-read per event so a change applies
  without a restart. `WM_MOUSEHWHEEL` gained the matching
  `SPI_GETWHEELSCROLLCHARS` handling.
- **X11 wheel buttons carried no magnitude.** Buttons 4–7 now report three lines
  per notch, matching GTK, Qt, and the Windows default.

## [v0.50.0] - 2026-08-05

### Added

- **Window icons on Linux and Windows.** The GL backend now publishes
  `WindowCfg.IconPNG` (or the go-gui default) as `_NET_WM_ICON` on X11 and as
  the big/small window icons via `WM_SETICON` on Windows, so the taskbar and
  alt-tab show the app's icon instead of the window manager's default. New
  `WindowCfg.WMClass` sets the X11 `WM_CLASS` property (both slots) for window
  grouping and `.desktop`-file matching. The Windows tray icon path moves into
  `gui/backend/internal/hicon`, shared with the GL backend.

## [v0.49.1] - 2026-08-05

### Changed

- **Dependency: `go-glyph` v1.18.1 → v1.18.2.** Picks up the atlas fix where
  glyph uploads now precede textured draws, eliminating the one-frame blank lag
  on first glyph appearance.

## [v0.49.0] - 2026-08-05

### Added

- **X11 IME support via IBus over D-Bus (#150).** Linux windows can now compose
  text with an input method: IBus connects over D-Bus, receives pre-edit
  clauses, and delivers committed text to the focused widget. Pre-edit display
  is hooked into the standard composition path, the selected clause is
  highlighted from `IBusAttrList` (#163), and the connection closes race-free
  with a bounded queue that matches the release keysym column (#167).

### Changed

- **Linux audio is now cgo-free by default (#141).** `gui/audio` decoded and
  mixed with pure-Go `beep`, but its output sink went through `oto`, whose Linux
  driver is cgo (ALSA). That left one package blocking
  `CGO_ENABLED=0 GOOS=linux go build ./...` even after the go-gl removal in
  #137. The output driver is now a small three-function seam
  (`outputInit`/`outputPlay`/`outputClose`) with two implementations: the
  default Linux build uses a pure-Go PulseAudio sink
  (`github.com/jfreymuth/pulse`, native protocol over a socket), so the whole
  module cross-compiles with no C toolchain. It requires a running PulseAudio or
  PipeWire server — present on any desktop; when absent, `audio.Init` returns an
  error and audio is disabled rather than crashing. Building with
  `-tags otoaudio` selects the previous oto/ALSA backend for direct-ALSA or
  maximum hardware compatibility. The public `gui/audio` API is unchanged, and
  Windows/macOS still use oto. The CGo-free CI cross-compile widened from
  `./gui/backend/...` to `./...`.
- **Dependencies: go-glyph bumped to v1.18.1.**
- **Test: GL backend smoke test skips when cgo is linked (#168).** The smoke
  test needs the cgo-free dispatch path; when the binary is built with cgo the
  load fails, so the test now skips instead of failing.
- **Docs: C toolchain requirement scoped to macOS only.**

## [v0.48.1] - 2026-08-04

### Fixed

- **Key repeat delivered no characters on macOS (#159).** macOS press-and-hold
  is on by default. After the first `insertText:`, AppKit hands the held key to
  the accent-palette machinery and stops calling `insertText:` for the
  auto-repeat `keyDown:` events that follow, so every repeat reached the toolkit
  as a bare `EventKeyDown` with no `EventChar` behind it. Holding a letter typed
  one character, holding Backspace deleted one character, and a terminal pane
  echoed nothing for the whole hold. `metalAppInit` now registers
  `ApplePressAndHoldEnabled=NO` before `sharedApplication`. Registered rather
  than written: the registration domain sits at the bottom of the defaults
  search order, so nothing is persisted to disk and
  `defaults write -g ApplePressAndHoldEnabled -bool true` — which lives in
  NSGlobalDomain and outranks it — still restores the accent palette for anyone
  who wants it over key repeat.

## [v0.48.0] - 2026-08-04

### Fixed

- **Re-focusing an already-focused widget destroyed the IME composition
  (#156).** `SetFocus` was not idempotent: it cleared the preedit and
  re-activated the platform input context even when the requested ID was already
  the focused one. Consumers legitimately re-assert focus from inside their
  `View` function, which runs on every layout rebuild — i.e. after every
  keystroke — so a CJK composition was torn down between each update. The
  preedit flashed and never accumulated, leaving CJK input unusable end to end.
  `imeClear` and the `IMEStart`/`IMEStop` pair now run only on a real focus
  change. Text-selection clearing and the cursor-blink reset are unchanged, so
  callers that re-focus to reset a selection still work.
- **App fonts were ignored on iOS and Android (#132).** Both backends loaded the
  bundled icon font but never called `gui.LoadAppFonts`, so `RegisterAppFont` /
  `RegisterAppFontBytes` were a silent no-op there. That also made
  `ThemeCfg.IconFontFamily` unusable on those platforms — retargeting the themed
  icon styles at a font that never loaded rendered tofu. Both now register the
  app-font lists right after the icon font, as the GL and Metal backends already
  did.
- **Metal backend dropped IME commit events (#149).** Text-input callbacks wrote
  a single global event slot, so when one `[NSApp sendEvent:]` fired several
  `NSTextInputClient` callbacks — the normal CJK sequence of `insertText:`
  (commit) immediately followed by `setMarkedText:` (residual composition) —
  each overwrote the previous and only the last reached Go. Committing Japanese
  produced zero characters; preedit rendering was unaffected. Text events now go
  through a FIFO drained one per `metalPollEvent`, so every callback is
  delivered, in order. The `_evIMEGeneration`/`_evIMEConsumedGen` pair is gone —
  an empty queue encodes the same signal.
- **Japanese converted text could never be committed on macOS.** After the IME
  converted a composition (Space, or picking from the candidate window), Enter
  did nothing at all: no `insertText:`, no callback of any kind, and the preedit
  stayed on screen forever. Two `NSTextInputClient` methods were stubbed in ways
  the input method treats as "this client has no document": `selectedRange`
  returned `{NSNotFound, 0}` — which IMK then used as the base for its range
  arithmetic, producing garbage queries like `{NSNotFound - 30, 30}` — and
  `attributedSubstringForProposedRange:` returned `nil`, though a converted
  commit asks for the text it is replacing. `selectedRange` now reports the
  selection the IME last set inside the composition, and
  `attributedSubstringForProposedRange:` serves the composition text (clamped,
  `nil` only when genuinely out of range). `validAttributesForMarkedText` now
  advertises the standard marked-text attributes instead of an empty list.
  Reproduced and fixed against a plain AppKit program with no go-gui code in it,
  so this was protocol conformance, not an event-loop problem. Unconverted kana
  commits were unaffected, which is why plain `nihongo` + Enter always worked.
- **Keys the input method owns no longer also reach the widget on macOS.** While
  a composition is live the IME owns the keyboard — arrows move between
  conversion clauses, Enter commits, Escape reverts — but the raw `EventKeyDown`
  was delivered as well, so the field's caret (and the preedit drawn at it) slid
  sideways under the arrow keys, and Enter could fire a widget action
  mid-composition. `keyDown:` now marks a keystroke as claimed when a
  composition was in progress or the key started one, and `metalPollEvent` drops
  it; the visible effect arrives as the queued composition or commit instead.
- **Placeholder text no longer stays visible during IME composition.** The
  preedit was inserted into the placeholder rather than replacing it, so the
  hint was pushed out to the right of the composing text ("かんEnter a name"). A
  placeholder is a hint, not content, so a composition now replaces it outright;
  a field with real text still gets the preedit inserted at the cursor. Applies
  to every backend, not just macOS.
- **Metal backend now reports the end of an IME composition.** An empty
  `setMarkedText:` (composition cancelled) previously wrote no event at all,
  leaving the preedit on screen. It is delivered as an `EventIMEComposition`
  with empty `IMEText`, which `Window.imeUpdate` already treats as "clear".
- **IME composition offsets are clamped to the preedit.** `IMEStart` and
  `IMELength` cross a platform boundary — Cocoa's `NSRange`, the Android
  bridge's `int64`, the browser's composition events — and fed slice arithmetic
  in the render path unchecked. `NSNotFound` in particular truncates to `-1` in
  the event's `int32`. `Window.imeUpdate` now clamps both to the composition's
  rune range, so a confused or hostile input method can mis-place an underline
  but cannot panic the renderer.
- **Web backend: keys the input method owns no longer reach the widget.** The
  browser fires `keydown` for arrows, Enter and Escape during a composition
  (`key == "Process"`, `isComposing == true`); the handler mapped the physical
  `code` and delivered an `EventKeyDown` anyway, so the caret slid out from
  under the preedit and Enter could fire a widget action mid-composition. The
  same class of bug as the macOS fix above. Composing keydowns are now ignored.
- **Web backend: a cancelled composition now reports its end.** `compositionend`
  with empty data (Escape) returned without emitting anything, relying on a
  preceding `compositionupdate` to clear the state — a browser/IME-dependent
  assumption. It now emits an `EventIMEComposition` with empty `IMEText`
  directly.

### Changed

- **GL backend no longer needs cgo (#155).** The `github.com/go-gl/gl`
  dependency is replaced by `gui/backend/internal/glbind`, which loads the GL
  entry points through purego, so the Linux and Windows backends build with
  `CGO_ENABLED=0`. Loading is all-or-nothing — the full binding table resolves
  or init returns an error — so there is no partial, silently broken GL state.
  Proc loading is per-platform (`egl_linux.go`, `wgl_windows.go`). No API change
  for embedders; the macOS/Metal and web backends are untouched.
- **macOS now emits `EventKeyDown` for printable keys**, followed by
  `EventChar`, matching X11, win32 and web. The KeyDown was previously swallowed
  by the same single-slot overwrite. Text input is unaffected (insertion runs on
  the Char path only), but two consequences are worth noting for macOS apps: a
  `Command` registered with a modifier-less `Shortcut` (e.g.
  `Shortcut{Key: KeyA}`) now fires on plain typing, as it already did on
  Linux/Windows; and `InputCfg.OnKeyDown` now fires for printable keys.
  Space-activated `OnKeyDown` handlers (select, combobox, listbox, tree, date
  picker, menu) become live on macOS — previously dead.

### Documentation

- `Event.CharCode` documents that it carries only the first rune of an IME
  commit; the full string is in `IMEText`.

## [v0.47.0] - 2026-08-01

### Added

- **Exported setters for the four remaining mouse cursors.**
  `Window.SetMouseCursorIBeam`, `SetMouseCursorNotAllowed`,
  `SetMouseCursorResizeNESW` and `SetMouseCursorResizeNWSE` join the six that
  were already public. All ten `MouseCursor` values were implemented and loaded
  by every backend (metal `NSCursor`, x11 `XC_xterm`/`XC_X_cursor`, win32
  `IDC_IBEAM`, web `"text"`); these four had stayed unexported only because
  go-gui's own widgets were the sole callers. An embedder driving the cursor
  from an external protocol — a terminal widget honoring OSC 22, a canvas app
  with its own hit regions — could not reach them and had to fall back to Arrow.
  Additive; no behavior change.

## [v0.46.0] - 2026-08-01

### Added

- **X11 PRIMARY selection.** `Window.SetPrimary`/`GetPrimary` (with the matching
  `SetPrimaryFn`/`SetPrimaryGetFn` backend hooks) expose the select-to-copy
  buffer that middle-click pastes on Unix — independent of CLIPBOARD, so an app
  can hold two different values at once. The X11 backend already owned
  selections for CLIPBOARD; PRIMARY reuses that machinery via a shared
  `selectionState` mapping, and `SelectionClear` now releases only the selection
  actually lost instead of both. Other backends leave the hooks nil, so
  `GetPrimary` returns `""` and `SetPrimary` is a no-op on macOS, Windows, web,
  and mobile.

## [v0.45.1] - 2026-07-31

### Fixed

- **Metal backend: AppKit calls could land on a non-main thread.** The Go
  runtime starts the main goroutine on thread 0 but does not keep it there — any
  blocking syscall or cgo call before the backend starts (config load, font
  registration, reading a file named on the command line) can let the scheduler
  resume `main` on another M. `backend.New` / `backend.RunApp` then called
  `runtime.LockOSThread` on the _wrong_ thread and the first AppKit call aborted
  with `API misuse: setting the main menu on a non-main thread`. The metal
  package now calls `runtime.LockOSThread` from `init`, which runs while the
  main goroutine is still on thread 0. Importing the backend is sufficient;
  embedders need no init boilerplate.
- **Same main-thread pin applied to the `gl` (X11/Win32) and `ios` backends.**
  Latent rather than fatal there, but the same migration applies: an OpenGL
  context is current on one thread at a time, Win32 delivers a window's messages
  only to the thread that created it, and UIKit has AppKit's main-thread rule.
  All three now lock from `init` instead of relying on the late `LockOSThread`
  in `New` / `RunAppE` / `Run`.

## [v0.45.0] - 2026-07-31

### Added

- **`DrawContext.ImageClipped`.** Draws an image restricted to a sub-rectangle:
  the texture still maps to the full destination rect, a scissor decides what is
  visible. `DrawCanvasImageEntry` gained `ClipX`/`ClipY`/`ClipW`/`ClipH` plus a
  `Clipped` flag to carry it, and the emit path intersects that rect with the
  canvas clip (`RenderClip` replaces the scissor rather than nesting) and
  restores the canvas clip afterwards. Consumers that must paint a fragment of
  an image without cropping the source file — a terminal emulator showing the
  visible cells of an image whose remaining cells sit behind another pane —
  could not express that before. Recorders (SVG/PDF export) receive the
  unclipped image, unchanged.

## [v0.44.0] - 2026-07-30

### Added

- **`NativeMenubarCfg.OmitAboutItem`.** Drops "About <AppName>" and its
  separator from the app menu, leaving Quit alone, for apps that expose About
  under their own Help menu. Takes precedence over `AboutActionID` — no About
  item is created at all.
- **`NativeMenubarCfg.IncludeWindowMenu`.** Auto-wires the standard Window menu
  (Close, Minimize, Zoom, Bring All to Front) and registers it with the OS
  window list. Installing a menubar replaces the backend's default one, so
  without this an app silently loses Cmd+W / Cmd+M.
- **`ThemeCfg.IconFontFamily`.** Sets the font family used by every theme-driven
  icon style (`Theme.Icon1`…`Icon6` and `TreeStyle.TextStyleIcon`), so an app
  that ships its own curated icon font can retarget them all with one field.
  Mirrors the existing `MonoFontFamily` handling: defaulted to `IconFontName` in
  `baseCfg()`, and an empty value falls back to `IconFontName`. No behavior
  change for existing apps — the bundled Feather font stays embedded,
  registered, and the default.
- **`gui.RegisterAppFontBytes([]byte)`.** Registers an in-memory font (e.g. one
  loaded with `go:embed`) alongside the existing path-based `RegisterAppFont`.
  The text system persists the bytes to its own temp file and removes it on
  teardown, so callers no longer need to manage one. Consumed by the GL and
  Metal backends, matching where `AppFontPaths` is consumed today.
- **`gui.LoadAppFonts(FontRegistrar, string)`.** Registers everything in
  `AppFontPaths` and `AppFontData` with a text system, logging and skipping
  fonts that fail to load so one bad font cannot stop the window from coming up.
  Replaces the per-backend loops the GL and Metal backends each carried, and
  takes the narrow `gui.FontRegistrar` interface (`AddFontFile`/`AddFontBytes`)
  so backends yet to adopt it — iOS, Android — can wire it up with one call.

### Fixed

- **Programmatic `Window.Dialog` / `DialogDismiss` now force a layout rebuild.**
  The dialog overlay is built during a full layout pass, so a caller outside the
  event path — a native menu action, a `QueueCommand` from a worker — left the
  window otherwise idle and the render-only frames a blinking cursor produces
  reused the existing layout tree. The dialog stayed invisible (or, on dismiss,
  stayed on screen) until an unrelated event forced a rebuild.

## [v0.43.0] - 2026-07-26

### Added

- **`Window.PumpFrame`.** Drives a single frame — flush, rebuild, present — from
  outside the normal event loop. This is what lets the Metal backend keep
  rendering while a nested AppKit run loop (a modal sheet, a native menu
  tracking session) owns the main thread.

### Changed

- **BREAKING: `CommandButton` no longer takes a `*Window`.** The signature is
  now `CommandButton(cmdID string, cfg ButtonCfg) View`. Command lookup,
  auto-label, auto-disable, and `OnClick` wiring are deferred to
  `GenerateLayout` via `ViewFunc`, so the widget no longer needs a window at
  construction time. Callers drop the first argument:
  `gui.CommandButton(w, "save", cfg)` → `gui.CommandButton("save", cfg)`.
- **Dependencies: `go-glyph` bumped to v1.18.0**, picking up `TextSystem.Purge`
  / `GlyphAtlas.Reset` / `Renderer.PurgeGlyphCache` for mid-session glyph and
  atlas memory reclamation.

### Fixed

- **Metal: frames no longer freeze under a nested AppKit run loop.** A modal
  sheet or native menu tracking session blocked the event loop; frames are now
  pumped for the duration.
- **Metal: removed an MSL lambda that broke every app on macOS Sonoma.** The
  shader failed to compile on Sonoma, taking down app startup.

## [v0.42.0] - 2026-07-20

### Added

- **System alert sound.** `Window.Beep` plays the platform's alert sound
  (`NSBeep` on macOS, `MessageBeep` on Windows, the freedesktop `bell` event via
  `canberra-gtk-play` on Linux), honoring the user's system-wide alert sound
  choice, volume, and mute settings. `Window.BeepAvailable` reports whether the
  platform can actually produce one, so callers can fall back to a visual cue on
  mobile, wasm, and Linux without canberra installed. Backed by the new
  `gui/backend/sysbeep` package and a `NativeSound` sub-interface on
  `NativePlatform`. This is for incidental out-of-band alerts such as a terminal
  BEL — it loads no assets and holds no output device open, unlike `gui/audio`.
- **ViewFunc.** A function adapter for `gui.View` that defers construction of
  window-dependent subtrees to layout time, keeping the content tree free of a
  pre-created `*Window` reference.

## [v0.41.1] - 2026-07-20

### Fixed

- **Command-queue buffer race.** `flushCommands` recycled its buffer into the
  scratch pool before finishing iteration; a concurrent `QueueCommand` from the
  animation goroutine could reclaim that same array and append into it
  mid-iteration, losing or duplicating queued commands. The buffer is now handed
  back only after execution completes.
- **Smooth scrolling could end fractionally short of its target.** A settled
  ease retires one animation tick after computing its final value; a main thread
  that missed that 16ms window dropped the snap-to-target. Entries now carry a
  dirty flag so the final value survives until the next flush, and a cancel
  clears it so direct offset writes are never overwritten by a stale eased
  value.

## [v0.41.0] - 2026-07-20

### Added

- **One-shot scroll anchoring.** New `Window.ScrollAnchor(scrollID, anchorID)`
  corrects the scroll offset on the next layout pass so the anchor view keeps
  the viewport-relative position it has now — content inserted or removed above
  the reader no longer causes a visual jump. The correction runs inside the
  layout pipeline (new `layoutApplyScrollAnchors` pass after `layoutPositions`),
  before the frame renders, so no intermediate position is ever shown. Requests
  are one-shot, last-write-wins per scrollable, vertical only, and bail to a
  plain jump when the anchor left the view, the content fits the viewport, or
  the correction would leave the scrollable range.
- **`Window.ScrollAnchorReveal`** anchors, then eases the scrollable to the top
  with the same smoothing as `ScrollVerticalToSmooth`, so prepended content
  glides into view. The ease arms inside the pipeline after the correction lands
  (arming beforehand would no-op when the reader is already at the top). An
  in-flight ease stays continuous across an anchoring correction: its displayed
  position shifts with the content, its absolute target is preserved.
- **`Window.ScrollVerticalOffset(id)`** returns the current vertical scroll
  offset (<= 0, 0 = top), complementing the existing percentage-based
  `ScrollVerticalPct`.

## [v0.40.0] - 2026-07-19

### Added

- **Smooth programmatic scrolling.** New `Window.ScrollVerticalToSmooth` and
  `Window.ScrollHorizontalToSmooth` ease a scrollable to an absolute offset
  using the same exponential smoothing as discrete mouse-wheel scrolling. No-op
  when the scroll id is not found or the target equals the current offset;
  instant `ScrollVerticalTo`/`ScrollHorizontalTo` still cancel any in-flight
  ease. The wheel smoother's arm logic is now shared
  (`scrollSmoothParams`/`scrollSmoothArm`) between relative wheel deltas and
  absolute targets.

### Changed

- **Deps:** go-glyph upgraded to v1.17.3 (perf/mem optimizations across the
  shaping pipeline, lazy font parsing, raster scratch-buffer pooling, sampled
  glyph-cache eviction).

### Fixed

- **Switch focus highlight.** Spacer no longer participates in focus ring
  drawing.
- **Toggle check-box sizing.** Off-by-one in hit-target calculation fixed.
- **Metal mouse-down consumed by window resize drag.** Drag resize no longer
  leaks mouse-down to the app's mouse handler.
- **Input batched-event echo and vertical centering.** Batched updates no longer
  echo raw text; single-line fields center vertically after font-size changes.
- **Data-grid quick-filter debounce.** Draft rendering during quick-filter is
  properly debounced.
- **Sidebar close layout:** Sidebar drops children when fully closed to prevent
  a fixed-width-0 content-resize bug.
- **BoundedMap small-map fast path.** Pre-sizing the BoundedMap data map is
  removed to keep the fast path for small maps.
- **buttonView folded into containerView,** and shapeEffects are pooled,
  reducing per-button allocations.

## [v0.39.0] - 2026-07-18

### Added

- **`BoundedMap.GetOr`.** New method on `BoundedMap` that accepts a constructor
  function, returning an existing value or publishing a new one without a double
  lookup. Internal `BoundedMap.Get` callers with ignored-ok returns have been
  migrated to `GetOr`, hardening the map against lost writes.
- **Process monitor example.** New `examples/process_monitor` — a live task
  manager: filterable process list with flat/tree views, sortable columns, and
  per-process CPU/RAM history charts built from plain containers. Data is
  collected dependency-free (`ps` on macOS/Linux, `tasklist` on Windows;
  `/proc/meminfo` or `sysctl`+`vm_stat` for system memory), sampled on a
  background goroutine that publishes under the window lock and refreshes via
  `Window.UpdateWindow`. Styled entirely from the standard theme tokens.
  Includes a headless `-once` terminal report. Functional port of the go-shirei
  example of the same name.

### Changed

- **Dependency bump.** Updated go-glyph to v1.17.2 (background cache warming for
  CJK fallback coverage; no API change).

## [v0.38.1] - 2026-07-17

### Changed

- **Dependency bump.** Updated go-glyph to v1.17.1 (struct field alignment,
  lower per-instance memory; no API change).
- **`examples/fontviewer` cleanup.** Named magic numbers and removed code smells
  in the font viewer example.

## [v0.38.0] - 2026-07-17

### Added

- **Font viewer example + font-enumeration API.** New `examples/fontviewer`
  browses installed system fonts in a virtualized card grid — name filter,
  editable sample text, 12–72 px size slider, click-to-copy. Backed by new
  public API: `gui.ListSystemFonts` with the optional `FontLister` backend
  capability, and
  `gui.ListVisibleRange(itemCount, rowHeight, listHeight, scrollY, overscan)`
  for grid/list virtualization. Requires go-glyph v1.17.0.

### Changed

- **`Sizing: FillFill` root now fills the window.** A root layout has no parent
  to fill against, so a `FillFill` root previously collapsed to content size;
  filling the window required boilerplate `WindowSize()` plus an explicit
  `Width`/`Height` and `FixedFixed`. Each Fill axis of the root is now pinned to
  the window dimension, so the intuitive spelling works and the examples/docs
  drop the boilerplate.

### Fixed

- **Fixed-size containers with a 0 dimension no longer break clipping and
  hit-testing.** A container with `SizingFixed` and an explicit 0 width/height
  rendered (its children self-draw) but kept zero-area bounds, collapsing the
  `shapeClip` — and therefore the clip region and the pointer/hit-test region —
  of every descendant, so a child with `Clip: true` vanished and interactive
  children went inert. Such a box now degrades to content sizing on the zero
  axis. (#94)

## [v0.37.0] - 2026-07-16

### Changed

- **BREAKING: remaining interactive controls are focusable by default.**
  `ButtonCfg`, `RadioCfg`, `RadioButtonGroupCfg`, `ComboboxCfg`, `ListBoxCfg`,
  `TreeCfg`, `DatePickerCfg`, `ColorPickerCfg`, `NumericInputCfg`, and
  `InputDateCfg` drop `Focusable bool` and gain `FocusDisabled bool`: the zero
  value is now _focusable_, and `FocusDisabled: true` is the explicit opt-out.
  This completes the focusable-by-default flip started in v0.36.0 (which covered
  Input, Select, Slider, Toggle, and Switch).

  Focus still requires a non-empty `ID` — an ID-less control is inert.
  `Disabled` still excludes from the tab order.

  Migration (compile error on the removed field is the guide):
  - `Focusable: true` → delete the line (now the default).
  - `Focusable: <expr>` → `FocusDisabled: !<expr>`.
  - `Focusable: false` → `FocusDisabled: true`.

- **`InputDateCfg` outer Column gains focusability.** Previously the outer
  container never set `Focusable`, so even with `Focusable: true` the date field
  was unreachable by keyboard. The outer Column now maps to
  `Focusable: !cfg.FocusDisabled`, matching the inner Input's focus state.

- **`DatePickerCfg` and `ColorPickerCfg` drop redundant `cfg.Focusable` gates**
  on focus-visual handlers. Focus visuals (border, color) now always apply when
  focused; `Disabled` remains the guard.

- **`NumericInputCfg` and `InputDateCfg` propagate `FocusDisabled` directly** to
  their inner `Input` instead of translating the inverse (`!cfg.Focusable`). The
  opt-out intent passes through transparently.

### Fixed

- `InputDateCfg` callers: the outer Column was never focusable even with
  `Focusable: true`. The inner `Input` was reachable but the container wasn't —
  focus now flows consistently through the composite widget.

## [v0.36.0] - 2026-07-16

### Added

- **`InputCfg.ReadOnly`** — blocks text edits while the field stays focusable
  and selectable. Navigation, selection, and copy keep working; typing, IME
  text, paste, cut, undo/redo, delete, multiline Enter, and
  `PostCommitNormalize` are all skipped. Single-line Enter still fires
  `OnEnter`/`OnTextCommit`, with the text uncommitted and unnormalized. The
  field is announced to assistive tech as `AccessStateReadOnly`. Mirrors HTML's
  `readonly`, and is distinct from `Disabled`, which removes the field from
  interaction entirely.

  This state was previously inexpressible: `AccessStateReadOnly` could only be
  produced by setting `Focusable: false`, which also dropped the field from the
  tab order — so an Input was either editable or unreachable, with nothing in
  between. (With the focusable-by-default flip below, `ReadOnly` is now the only
  trigger for the read-only announcement.)

- **`NumericInputCfg.ReadOnly` and `InputDateCfg.ReadOnly`** — extend
  `InputCfg.ReadOnly` to the two composite wrappers. Both forward the flag to
  their inner `Input` (blocking typing) and gate the secondary mutation paths
  that bypass the text field: `NumericInput` disables and gates its step buttons
  at the `numericInputApplyStep` choke point, and `InputDate` keeps the calendar
  popup closed so its picker can never emit a selection. Enter-commit on the
  read-only inner `Input` no longer surfaces a value/date change. Both wrappers
  announce `AccessStateReadOnly`. `NumericInputCfg` and `InputDateCfg` already
  had `Disabled`; `ReadOnly` is the focusable-but-uneditable counterpart.

### Fixed

- **Read-only Input no longer renders IME preedit.** A composition started on a
  read-only field displayed preedit text that could never commit
  (`makeInputOnChar` swallows the commit), leaving a stray artifact until focus
  change. Preedit rendering is now suppressed for read-only fields; selection
  and cursor still render, since the field stays `Focusable`. Editable fields
  are unaffected.

- **`CommandButton` now auto-fills `ID`** from the command ID, prefixed with
  `cmdbtn:`. Focus traversal is keyed by `Shape.ID`, so a `CommandButton` with
  `Focusable: true` but no explicit `ID` was silently unreachable by keyboard.
  The prefix keeps the button's focus ID distinct from the menu item driven by
  the same command, which carries the raw command ID. Widgets that were dead tab
  stops now join the tab order; pass an explicit `cfg.ID` for two buttons on one
  command in the same window.
- **36 examples** set `Focusable: true` without an `ID` and were not
  keyboard-reachable, including `get_started`. All now carry stable IDs. `snake`
  also dropped `controlsIDBase`/`startButtonID` numeric focus IDs left over from
  the removed `IDFocus uint32` API.

### Changed

- **BREAKING: input controls are focusable by default.** `InputCfg`,
  `SelectCfg`, `SliderCfg`, `ToggleCfg`, and `SwitchCfg` drop `Focusable bool`
  and gain `FocusDisabled bool`: the zero value is now _focusable_, and
  `FocusDisabled: true` is the explicit opt-out. An input the user can't tab to
  is a bug, not a design choice — and for `Select`/`Slider`/`Toggle`/`Switch`
  this is an accessibility fix, not just deboilerplating: ID-bearing call sites
  that never set `Focusable` now join the Tab order (a slider should be
  keyboard-adjustable).

  Focus still requires a non-empty `ID` (`Focusable && ID != ""`). An ID-less
  control is **inert**: it renders but never becomes a tab stop, and no ID is
  ever fabricated. `Disabled` still excludes a control from the tab order;
  `ReadOnly` still keeps it focusable.

  Migration (compile error on the removed field is the guide):
  - `Focusable: true` → delete the line (now the default).
  - `Focusable: <expr>` → `FocusDisabled: !<expr>`.
  - `Focusable: false` → `FocusDisabled: true`.

  Out-of-scope widgets keep opt-in `Focusable bool`: Button, Container, Text,
  and the composites/wrappers (`Combobox`, `DatePicker`, `ListBox`,
  `RadioButtonGroup`, `NumericInput`, `InputDate`, `Radio`, ColorPicker,
  ThemePicker, Tree) — the wrappers translate their `Focusable` into the inner
  Input's `FocusDisabled`.

  The four focus flags, disambiguated:

  | Flag                  | Meaning                                                  |
  | --------------------- | -------------------------------------------------------- |
  | `Shape.Focusable`     | widget participates in the focus system                  |
  | `FocusSkip`           | focusable + click/selection, but excluded from Tab order |
  | `FocusDisabled` (Cfg) | opt out of the default-on focus (in-scope Cfgs)          |
  | `Disabled`            | non-interactive; also excluded from Tab order            |

- **A non-focusable Input no longer announces `AccessStateReadOnly`.** Before
  the flip, `Focusable: false` was the only way to express an uneditable field,
  so it doubled as the read-only signal. Now that non-focusable means an
  explicit `FocusDisabled` opt-out, only `ReadOnly: true` announces read-only.

- **`requiredid` analyzer** now also flags Cfg literals that set
  `Focusable: true` without a non-empty `ID`, catching this class of silent
  no-op at `go vet` time.

## [v0.35.1] - 2026-07-15

Documentation-only release. No code or behavior changes; no migration needed.

### Changed

- **`GenerateViewLayout` is no longer deprecated**. It is the supported entry
  point for composite View widgets, which need to recurse a View tree into a
  Layout tree. The deprecation pointed at `View.GenerateLayout`, which builds a
  single node and does not recurse into `Content()` — it was never an equivalent
  replacement, and no other exported path existed. Callers that hand-rolled
  their own recursion to avoid the warning should call `GenerateViewLayout`
  again; it applies shape normalization, the child-count clamp, and frame-arena
  pre-sizing that a hand-rolled copy misses. (#52)

### Fixed

- **README**: removed SDL2-era install steps, corrected the text stack
  description to the current pure-Go path, and refreshed the code samples to the
  current API.

## [v0.35.0] - 2026-07-15

### Changed

- **BREAKING — Scroll API**: `Shape.IDScroll uint32` is replaced by
  `Scrollable bool` plus string scroll identity (the widget's `ID`). A container
  opts into the scroll system with `Scrollable: true` and a non-empty `ID`;
  scroll offset is keyed by that `ID`. Migration: `IDScroll: N` →
  `Scrollable: true` (with a non-empty `ID`). Scrollable containers now panic at
  build if `ID` is empty (`RequireScrollID`).
- **BREAKING — Lost scroll handle**: the caller-supplied `IDScroll uint32` is
  removed from `ContainerCfg`, `ListBoxCfg`, `TreeCfg`, `TableCfg`,
  `ComboboxCfg`, `CommandPaletteCfg`, `InputCfg` and `DataGridCfg`. The scroll
  key is now _derived_ from the widget's `ID`; pass that same derived string to
  `Window.ScrollVerticalTo`/`ScrollHorizontalTo`/`…Pct` etc. Derived keys:

  | Cfg                                                 | scroll key                                            |
  | --------------------------------------------------- | ----------------------------------------------------- |
  | `ContainerCfg`, `ListBoxCfg`, `TreeCfg`, `InputCfg` | `cfg.ID`                                              |
  | `TableCfg`                                          | `cfg.ID`, or `cfg.ID + ":scroll"` when `FreezeHeader` |
  | `ComboboxCfg`                                       | `cfg.ID + ".dropdown"`                                |
  | `CommandPaletteCfg`                                 | `cfg.ID + ":scroll"`                                  |
  | `DataGridCfg`                                       | `cfg.ID + ":scroll"`                                  |

  `DataGridCfg.IDScroll` (an identity override) is deleted, not migrated; the
  key always derives from `cfg.ID + ":scroll"`.

- **BREAKING — Window scroll offset maps**: `Window.ScrollX()` and
  `Window.ScrollY()` now return `*BoundedMap[string, float32]` (was
  `*BoundedMap[uint32, float32]`). All `Window.Scroll*` methods
  (`ScrollHorizontalBy/To/ToPct/Pct`, `ScrollVerticalBy/To/ToPct/Pct`) and
  `FindLayoutByScrollID` (renamed from `FindLayoutByIDScroll`) take a `string`
  id.
- **BREAKING — Scrollbar/command-palette cfgs**: `ScrollbarCfg.IDScroll uint32`
  → `ScrollID string` (points at the target container's scroll key).
  `CommandPaletteShow`/`CommandPaletteToggle` drop the `idScroll` parameter;
  Show always resets the results scroll (keyed `id + ":scroll"`) to the top.
- **Scroll internals**: the scroll-offset maps are rekeyed uint32→string, which
  sidesteps the `BoundedMap[uint32]` generic lookup penalty
  ([#77](https://github.com/go-gui-org/go-gui/issues/77)); FnvSum32 scroll-hash
  derivation removed from Select, Combobox, DataGrid and the theme picker.
  `Shape` shrinks 272 → 264 bytes (this change −8 with the `IDScrollContainer`
  removal below).

### Removed

- Dead `Shape.IDScrollContainer uint32` field and its per-frame whole-tree
  `layoutScrollContainers` pass (zero readers).

### Added

- `BenchmarkViewFrame` gates `sizeof(Shape)` regressions by allocating Shapes
  inside the hot loop (added to the `bench-gate` target).

## [v0.34.0] - 2026-07-14

### Changed

- **BREAKING — Focus API**: `Shape.IDFocus uint32` is replaced by
  `Focusable bool` plus string focus identity (the widget's `ID`). Tab order now
  follows layout-tree (DFS) order instead of ascending numeric IDs. Window API:
  `SetFocus(id string)`, `FocusID() string`, `IsFocus(id string)`, and
  `ClearFocus()` replace the uint32 variants. Migration: `IDFocus: N` →
  `Focusable: true` (with a non-empty `ID`); `SetIDFocus(0)` → `ClearFocus()`.
- **BREAKING — Widget cfgs**: `MenubarCfg`, `ContextMenuCfg`,
  `CommandPaletteCfg`, and `DataGridCfg` lose `IDFocus`; `DialogCfg.IDFocus` →
  `FocusID string`; `RadioButtonGroupCfg` gains `ID`; menus and context menus
  now require an `ID`. `CommandPaletteShow`/`CommandPaletteToggle` drop the
  `idFocus` parameter (focus derives from the palette input's ID).
- **Focus internals**: six per-window state namespaces rekeyed uint32→string;
  FnvSum32 focus-hash derivation removed from menus and datagrid (header/editor
  focus ids now derive from cell ID and column index). Duplicate focusable IDs
  collapse to one tab stop with a dev-mode warning (`GOGUI_FOCUS_DEBUG=1`).
  `IDScroll` is unchanged.

## [v0.33.1] - 2026-07-14

### Fixed

- **Dependencies**: go-glyph bumped to v1.16.2 — text symbols across many
  Unicode blocks (heavy asterisk U+2731 ✱, mahjong tiles, playing cards,
  alchemical and chess symbols, Supplemental Arrows-C, ~760 codepoints) no
  longer render as the base font's `.notdef` tofu box, and default-text emoji
  such as U+2733 ✳, ❄, ❤, ☀ now render as monochrome text glyphs instead of
  being forced to color, matching Core Text and Ghostty. Also propagates
  InlineObject metadata through rich-text layout and applies script fallback in
  `LayoutRichText`.

## [v0.32.1] - 2026-07-12

### Fixed

- **Dependencies**: go-glyph bumped to v1.16.1 — text symbols such as U+23F5 ⏵
  (the Misc Technical media triangles) no longer render as the base font's
  `.notdef` tofu box; they now fall back to a real glyph (STIX), matching Core
  Text.

## [v0.32.0] - 2026-07-12

### Changed

- **Dependencies**: go-glyph bumped to v1.16.0 — supplies a recommended line
  height (`TextMetrics.LineHeight`: font leading floored to 1.15×em) and no
  longer discards leading in multi-line layout, so wrapped text stacks with
  correct spacing. Regenerates `docs/dependencies.md`.
- **Markdown defaults**: removed the manual paragraph `LineSpacing = 3`
  workaround in `DefaultMarkdownStyle`; line spacing now comes from the font's
  recommended line height provided by go-glyph.

### Fixed

- **gl backend**: populate `MouseDX`/`MouseDY` on motion events so scrollbar
  thumb drags track the pointer. (#66)
- **svg**: reject non-finite `tx`/`ty` in `decomposeTRS`. (#67)

## [v0.31.0] - 2026-07-11

### Changed

- **Dependencies**: go-glyph bumped to v1.15.0 — pure-Go text backends on Linux,
  Android, macOS, and Windows (`go-text/typesetting` + `x/image/vector`,
  `CGO_ENABLED=0`), replacing the cgo FreeType+HarfBuzz stack. Pulls in
  `go-text/typesetting` and `golang.org/x/image` as indirect deps; regenerates
  `docs/dependencies.md`. Drops the obsolete Android native-deps build step from
  CI/release workflows.
- **Markdown defaults**: paragraph `LineSpacing` reduced to 3 and `BlockSpacing`
  raised to 12 so inter-block gaps stay larger than intra-line gaps (fixes
  cramped spacing between wrapped list items).

### Fixed

- **Markdown line spacing**: `TextStyle.LineSpacing` now applies to wrapped
  rich-text (RTF) rendering. Line spacing lives on glyph's `BlockStyle`, so
  `ToGlyphStyle` dropped it and markdown paragraph/list line spacing was a
  no-op; the value is now carried through both RTF layout paths.

## [v0.30.2] - 2026-07-10

### Changed

- **Dependencies**: go-glyph bumped to v1.14.0. Regenerates
  `docs/dependencies.md` to match.

### Removed

- Remove all SDL2 vestiges from the codebase.
- Trim vestigial MSYS2 packages from Windows CI.

## [v0.30.1] - 2026-07-10

### Changed

- **Dependencies**: go-glyph bumped to v1.13.1 (Windows proportional-font
  substitution now falls back to Consolas; internal draw/renderer dedup between
  the FreeType and Darwin backends). Regenerates `docs/dependencies.md` to
  match.

## [v0.30.0] - 2026-07-08

### Changed

- **Dependencies**: go-glyph bumped to v1.13.0 (FreeType+HarfBuzz replaces
  Pango/SDL2, native GLX+WGL backends, ASCII monospace shaping fast-path on
  Darwin). The prior v2.0.0 tag was retracted — it lacks the `/v2` module path
  suffix required by Go module conventions.

### Added

- **Window vibrancy** (macOS): `Window.SetWindowVibrancy(VibrancyMaterial)`
  places a translucent, blurred native `NSVisualEffectView` backdrop behind the
  window content. Pair with a translucent `WindowCfg.BgColor` (alpha < 255) to
  reveal the backdrop; `VibrancyNone` restores an opaque window. Implemented on
  the Metal backend (makes the window and its `CAMetalLayer` non-opaque so
  content composites over the blur); no-op on SDL2, OpenGL, web, iOS, and
  Android (Linux/Windows are out of scope, matching the `TermGrid` issue). Built
  for go-term. See `examples/vibrancy`. (#31)
- **`TermGrid` primitive**: a terminal character-grid widget
  (`TermGrid`/`TermGridCfg`) that draws a fixed-pitch cell buffer in a single
  `RenderTermGrid` command — no per-cell `Layout` node and no per-cell
  `RenderText`. Callers hand over a row-major `[]TermCell` with pre-resolved
  RGBA foreground/background, plus cursor and selection state; the backend
  batches same-background runs into fills and pins glyphs to exact cell columns
  via `DrawLayoutPlaced`. Honors the reverse and underline attributes;
  bold/italic are reserved in `TermAttr` for a follow-up. Rendered by the Metal
  and SDL2 backends (OpenGL out of scope). Built for go-term and reusable across
  siblings. See `examples/termgrid`. (#30)

### Fixed

- **GL backend**: narrow build tags from `!js` to `!js && !darwin` on the real
  implementation files so they don't compile on macOS where `platform_other.go`
  returns nil. Eliminates 50 unused-code lint warnings on the default dev
  platform. macOS uses Metal, not GL.
- **Native dialogs**: track native (OS) modal visibility so `DialogIsVisible`
  and the quit/close dedup see `NSAlert`-style dialogs too. Previously a native
  confirm-before-quit could stack a duplicate dialog because the second
  quit/close re-invoked `OnCloseRequest`. `DispatchCloseRequest` now guards its
  hook path while a dialog is showing (the no-hook path still closes). (#18)
- **Modal dialogs**: retain keyboard focus inside an in-app modal dialog when a
  focus-claiming widget (one that re-asserts `SetIDFocus` every view rebuild)
  tries to steal it, so Tab/Esc/Enter keep working. Apps no longer need to guard
  their own `SetIDFocus` with `DialogIsVisible`. (#18)

## [v0.29.0] - 2026-06-28

### Added

- **`TextStyle.EmojiBoxWidth`**: optional target cell-box width (logical px),
  threaded through `ToGlyphStyle` and the GPU backend draw path
  (`GuiStyleToGlyphConfig`). When > 0, go-glyph scales color/emoji glyphs to
  fill the caller's cell box instead of the font's natural emoji advance. Used
  by grid callers such as go-term. Requires go-glyph v1.12.0.

## [v0.28.2] - 2026-06-26

### Fixed

- **macOS Metal backend**: complete the Launch Services launch handshake so a
  `.app` bundle launched from Finder is fully registered as a foreground app —
  fixes absence from Cmd+Tab, gray titlebar buttons, and the
  double-click-to-close behavior. Restore `activateIgnoringOtherApps:` for the
  CLI-launch case so bare-exec windows come up active.
- **macOS Metal backend**: fire `EventFocused` on `applicationDidBecomeActive:`
  so keyboard and left-click input are restored after a system dialog (e.g. TCC
  permissions) is dismissed, and re-key the frontmost window when `keyWindow` is
  nil on app switch.
- **DockLayout**: enlarge the tab close button (14×14 → 18×18) with a larger ×
  glyph, and add a spacer between the tab label and close button.

### Changed

- **macOS Metal backend**: cache NSCursor selector C strings once at startup to
  drop per-frame `C.CString` alloc/free from the cursor-update hot path.
- **macOS Metal backend**: rename app-launch entry points to reflect their
  lifecycle stage (`metalAppInit` / `metalAppFinishLaunch`) and dedupe the
  wake-event construction and activation-focus paths.

## [v0.28.1] - 2026-06-22

### Fixed

- **macOS Metal backend**: `finishLaunching` deferred until after first window
  is created, fixing off-screen windows when launched as a `.app` bundle via
  Finder/Launch Services.

## [v0.28.0] - 2026-06-22

### Added

- **Native macOS backend**: new Metal-based backend with native window
  management, event handling, cursor support, and menu integration via AppKit.
  Replaces the SDL2 backend on macOS for proper platform behavior.
- **DockLayout**: `HideSingleTab` option hides the tab bar when only one tab is
  present.

### Changed

- **Performance**: content dimensions and sibling sums cached during fill pass
  to avoid redundant recalculation during layout.

### Fixed

- **DockLayout**: close button rendered as × instead of an empty box.
- **WASM backend**: hardened against iPadOS Safari crashes.
- **SVG**: `arcToCubic` guarded against NaN/Inf radii and float32 overflow
  panic.
- **Website**: `status.html` included in deploy output.
- **Windows CI**: MSYS2 pinned to stable release.

## [v0.27.1] - 2026-06-20

### Fixed

- **Windows CI**: pin MSYS2 to stable release to fix `__ms_vsscanf` undefined
  reference linker error with GCC 16.1.0.

## [v0.27.0] - 2026-06-17

### Added

- **Platform parity**: Save/Discard/Cancel close confirmations for unsaved
  changes, plus Windows system tray icon support.
- **Dialog quit guard**: quit requests are trapped when a modal dialog is
  visible, preventing accidental app termination.
- **Tooling spec**: automated linting, license checking, Renovate dependency
  management, and coverage reporting integrated into CI.
- **CI hardening**: race detector enabled, caching and cache-key rotation,
  deduplication, 800-line file-size gate, deadcode detection, `go mod tidy`
  check, fuzz-crash detection, and security scans (gosec).
- **Shared native-platform glue**: `App` native integration tests and extracted
  platform abstraction for backend consistency.

### Changed

- **Performance**: eliminated hot-path heap allocations across layout
  calculation, gesture hit-testing, render command generation, and event
  dispatch. Two-pass allocation scrub.
- **Lock splitting**: animation lock separated from layout lock; `w.mu` scope
  narrowed to reduce contention.
- **GPU backend consolidation**: shared `gpu` package for vertex types and draw
  code reused across Metal, OpenGL, and SDL2 backends.
- **Large-file refactoring**: 14 files over 800 lines split into 32 focused
  files; datagrid dot-imports removed; markdown fetcher uses dependency
  injection.
- **Dependencies**: go-glyph bumped to v1.10.0, golangci-lint to v2.12, GitHub
  Actions to latest major versions.
- **Test parallelization**: tests now run concurrently; per-package coverage
  floors enforced in CI.

### Fixed

- **macOS**: `NSApp` activated before window creation, fixing focus issues on
  launch.
- **DataGrid**: scroll position read from correct state map; `UpdateView` no
  longer clears `idFocus` on full rebuild.
- **SVG**: `arcToCubic` guarded against coincident endpoints (NaN from
  `Inf*0/1`).
- **GPU**: `gpu.Vertex` struct literals use keyed fields across all backends.
- **Web backend**: removed unused `syscall/js` import; restored `strconv`
  import.
- **Build tags**: drift corrected across source files and CHANGELOG.
- **Animation**: map data races fixed under concurrent access.
- **Windows**: `__ms_vsscanf` compat shim for MinGW GCC 15+; static builds and
  DLL alignment hardened; CI smoke test added.
- **Security**: 242 gosec issues resolved; G204 false positives suppressed on
  `exec.Command` calls; privacy audit and resource caps applied.
- **CI**: various workflow fixes — golangci-lint install path, tidy-check
  ordering, generate-check path/scope, coverage subshell, fuzz timeout.

### Docs

- WebGPU backend documented as explored and rejected (2026-06).
- Roadmap updated with new features and Phase 2 progress.
- README, CONTRIBUTING, and docs trimmed of fluff.
- Dependencies.md regenerated; depsdoc generator added.
- Async DataGrid and custom shader cookbooks added.
- Platform matrix, form validation patterns, time-travel example test
  documented.
- Godoc improved on core types; subpackage analysis and widget cookbook added.

## [v0.26.0] - 2026-06-12

### Breaking

- **DataGrid moved to `gui/datagrid/`** — `DataGrid`, `DataGridCell`,
  `DataGridTheme`, and related symbols (~30) extracted from `gui/` into a
  separate package. Import `github.com/go-gui-org/go-gui/gui/datagrid` and use
  `datagrid.New()` instead of `gui.NewDataGrid()`.
- **SVG constant renames** — `StrokeCap` → `SvgStrokeCap`, `StrokeJoin` →
  `SvgStrokeJoin`, plus typed constants for stroke cap/join, spread method, and
  units. Callers using the old untyped string constants will need to update to
  the new typed values.
- **Spinner renamed to MathSpinner** — `gui.NewSpinner()` →
  `gui.NewMathSpinner()`. Disambiguates from future loading-spinner widget.

### Added

- `SvgAlignNone` now does non-uniform stretch with independent scaleX/scaleY
  instead of treating the SVG as xMidYMid.
- Backend `RunE`/`RunAppE` variants (gl, sdl2, metal, web) that return errors
  instead of panicking on init failure.

### Changed

- SVG path parser refactored into `pathParser` struct with per-command methods
  for lower allocation and better readability.
- Render validators and SVG element handlers extracted from large functions into
  focused helpers.
- `keyName` and `EventFn` complexity reduced via helper extraction.
- Showcase temp-file handling hardened, lazy-load abort wired, allocations
  simplified.

### Fixed

- Web backend build broken by `newBackend` return value mismatch and
  non-constant `fmt.Errorf` format strings.
- macOS linker duplicate `-lobjc` warnings suppressed via
  `-Wl,-no_warn_duplicate_libraries`.
- Web backend keyboard modifiers guarded against `KeyboardEvent` lacking
  `.buttons`.
- Web backend keydown/keyup registered on `document` instead of `canvas`, fixing
  focus-edge-case missed keys.
- Showcase wasm build missing `cleanupEmbeddedAssets` stub.
- Showcase audio made opt-in behind `audio` build tag, fixing Windows FLAC DLL
  issue (#8).
- CI showcase deploy race condition on GitHub Pages fixed.

## [v0.25.0] - 2026-06-08

### Changed

- Reduce exported API surface (~138 symbols removed) in preparation for v1.0.
  Numerous internal types, functions, constants, and methods made unexported.
  **Breaking:** callers importing removed symbols must update to use the
  remaining public API.

## [v0.24.8] - 2026-06-07

### Fixed

- iOS simulator .app artifact missing from GitHub Release (upload step was
  skipped in the release workflow).
- Android gomobile bind failing due to missing native dependencies (FreeType,
  HarfBuzz now built before bind step).
- Android gomobile bind failing due to missing golang.org/x/mobile tool
  dependency in release workflow.

### Added

- iOS simulator README bundled in the release artifact zip.

## [v0.24.6] - 2026-06-06

### Changed

- Update roadmap platform table to reflect final build approaches.

## [v0.24.5] - 2026-06-06

### Added

- Windows showcase binary (.zip with bundled SDL2 DLLs) published to GitHub
  Releases.

### Fixed

- Release workflow upload race (first-completed job published release before
  other jobs could attach assets). Restructured to build→artifact→publish
  pipeline.
- macOS release CI (`brew install sdl2 sdl2_mixer sdl2_ttf sdl2_image`,
  `-bundle-deps` for self-contained .app).
- Linux release CI (added freetype6/harfbuzz/pango dev packages for go-glyph CGo
  compilation).
- Windows release CI (switched from go-sdl2 static libs to MSYS2 SDL2 packages
  to resolve MinGW `__ms_vsscanf` linker incompatibility).

## [v0.24.0] - 2026-06-06

### Added

- WASM showcase deployed to GitHub Pages at
  `https://go-gui-org.github.io/showcase/` with loading spinner and download
  progress indicator.
- Desktop binaries (macOS `.dmg`, Linux `.tar.gz`, Windows `.zip`) built and
  published as GitHub Release artifacts on `v*` tags.

### Changed

- Release workflow now builds all four platforms (macOS, Linux, Windows, WASM)
  and uploads artifacts to the GitHub Release.

## [v0.23.0] - 2026-06-06

### Added

- `ListBoxCfg.Items []string` — convenience field; each string becomes a
  `ListBoxOption` with `ID==Name==Value`.
- `RadioButtonGroupCfg.Items []string` — convenience field; each string becomes
  a `RadioOption` with `Label==Value`.
- `TableCfg.RawData [][]string` — convenience field for CSV-style data. First
  row is treated as the header.
- `TreeCfg.ItemPaths []string` — convenience field for flat path strings
  (`"a/b/c"`), auto-expanded into nested `TreeNodeCfg` nodes with duplicate
  prefix merging.
- `DataGridCfg.RowsData []map[string]string` — convenience field for key-value
  row data. Map keys match column IDs. Columns are auto-generated from sorted
  keys of the first entry when `Columns` is empty.

### Changed

- When both the stdlib convenience field and the typed struct field are set, the
  stdlib field takes precedence.

## [v0.22.0] - 2026-06-05

### Added

- Single-binary deploy on Linux and Windows via `go-sdl2 -tags static`,
  eliminating the `libSDL2.so` / `SDL2.dll` runtime dependency.
- Root `Makefile` with `build-linux`, `build-windows`, `build-macos`,
  `build-wasm`, `release`, and `clean` targets.
- `gui.Version` and `gui.Commit` build-time variables injected via `-ldflags`.
- CI release workflow (`.github/workflows/release.yml`) triggered on `v*` tags
  and `workflow_dispatch`, building all desktop platforms.

## [v0.21.1] - 2026-05-30

### Fixed

- Hunspell spellcheck is now opt-in via `-tags hunspell` build tag on Linux,
  avoiding a hard runtime dependency on `libhunspell`.

### Changed

- Remove local `replace` directive for `go-glyph` — the module now consumes
  upstream `go-glyph` directly.
- Add Dependabot config for `go-glyph` dependency updates.

## [v0.21.0] - 2026-05-28

### Changed

- Module path renamed from `github.com/mike-ward/go-gui` to
  `github.com/go-gui-org/go-gui`.
- `go-glyph` dependency bumped to v1.9.0.
- Repository moved to `go-gui-org` GitHub organization.

### Added

- Benchmark and inspector screenshots in README.

## [v0.20.2] - 2026-05-24

### Changed

- Bump `go-glyph` dependency to v1.8.0.

## [v0.20.1] - 2026-05-22

### Fixed

- `DrawContext.Scale` (device pixel ratio) is now correctly populated from the
  backend's DPI scale and included in the `DrawCanvasCache` key, so a canvas is
  re-tessellated when the display scale changes (e.g. window moved between
  Retina and non-Retina monitors).
- All backends (gl, metal, sdl2) now refresh `dpiScale` on window resize, so
  display migration no longer leaves a stale scale for the lifetime of the
  window.
- Web backend now sets `w.BackingScale` each frame; previously
  `DrawContext.Scale` was always 1 on web regardless of `devicePixelRatio`.
- Scale sanitization guard relaxed from `< 1` to `<= 0`, allowing valid sub-1
  device pixel ratios (e.g. browser zoomed below 100%) to pass through.

## [v0.20.0] - 2026-05-18

### Added

- RTF widgets with `IDFocus` set now support interactive text selection: click
  to place cursor, drag to extend selection (with scroll-aware auto-scroll),
  double-click to select word, keyboard navigation (arrow keys, Home/End,
  Ctrl/Cmd+A), and Ctrl/Cmd+C to copy.
- Markdown widgets with `IDFocus` set gain the same selection and copy
  capability across all block types (paragraphs, headings, lists, blockquotes,
  definition terms/values). Selection uses a unified rune-offset model so Cmd+C
  copies the correct cross-block span.

## [v0.19.1] - 2026-05-17

### Added

- Metal backend: scroll phase bridge (`scroll_phase_darwin`) fires
  `EventScrollBegan` when a momentum scroll starts on macOS.

### Fixed

- Context menu: focus is now restored on dismiss; a second right-click no longer
  clobbers the saved focus state.

## [v0.19.0] - 2026-05-16

### Added

- Animation heartbeat is now view-bound: orphaned animations (whose owning
  layout is no longer present in the view tree) are automatically cancelled,
  preventing runaway tickers in long-lived windows.

### Fixed

- Metal backend: per-frame autorelease pool now spans the full frame
  (`metalBeginFrame` → `metalEndFrame`). Command buffers, render pass
  descriptors, encoders, and one-off `MTLBuffer` allocations were accumulating
  in the thread's ambient pool indefinitely (Go threads have no runloop). Uses
  `objc_autoreleasePoolPush`/`Pop` (ARC-compatible).

## [v0.18.0] - 2026-05-09

### Added

- `OnKeyUp` callback on Input widgets; `KeyUp` event dispatched through
  `Window.EventFn` mirrors the existing `OnKeyDown` pipeline.
- `key_up_demo` example demonstrating `KeyUp` event flow.

### Changed

- Extracted `ScrollDeltaHome` constant (replaces magic number `10_000_000`).
- Added `hasAnyModifiers()` helper and `inputTextChange()` function to reduce
  complexity in `makeInputOnChar()`; modernized loops with `slices.Backward`.

### Fixed

- Context menu no longer hijacks `IDFocus` on right-click. Pre-open focus is
  saved in `nsContextMenuFocus` (survives `dismissPopups`) and restored on
  action selection; `menuItemClick` only resets focus to zero when no action
  callback changed it.

## [v0.17.0] - 2026-04-30

### Added

- `AppFontPaths` registry lets apps declare custom font search paths before
  window creation. SDL2/Metal/GL backends load the registry at backend init so
  glyph rasterization picks up app-bundled fonts without ad-hoc backend hooks.

### Changed

- Bumped `github.com/go-gui-org/go-glyph` to `v1.7.0`.

### Fixed

- Inline math (markdown RTF render) now uses the per-`InlineObject`
  Height/Offset when available, preserving true aspect ratio for tall constructs
  like fractions and integrals. Previously height was clamped to line-height
  (ascent+descent), squashing oversize glyphs. Legacy entries without `Object`
  keep the old line-height fallback.

## [v0.16.0] - 2026-04-28

### Added

- `<use>` referencing `<symbol viewBox=...>` now honors `preserveAspectRatio`.
  Default is `xMidYMid meet` (uniform scale + center) per SVG 1.1;
  `preserveAspectRatio="none"` opts back into legacy independent-axis stretch;
  `slice` uses uniform max-scale and now mints a synthesized `clipPath` covering
  the `<use>` box so overflow from max-scale is cropped per spec. Author
  `clip-path=` on the `<use>` itself wins over the synth clip. Earlier impl
  always stretched and never clipped slice overflow.
- `clip-path` and `filter` now participate in the cascade: CSS rules
  (`<style>.cls { clip-path: url(#cp) }`) and inline `style=""` declarations set
  them, not only the bare presentation attribute. `clip-path: none` /
  `filter: none` clear inherited values per cascade origin precedence.
- Distinct elements sharing one `filter="url(#X)"` now composite as separate
  offscreen groups in document order. Previously they were merged by FilterID
  and z-ordered against unfiltered siblings incorrectly. Each occurrence carries
  a per-element `FilterGroupKey` (parser counter assigned during cascade).
- Nested `<svg>` elements now establish a child viewport. `x`, `y`, `width`,
  `height` accept user-space units or percentages of the parent viewport; an
  inner `viewBox` (with `preserveAspectRatio` meet/slice/none) composes onto the
  cascaded transform from the element's own `transform=` attr. Descendants
  inherit paint and cascade through the wrapper. Previously the inner subtree
  was dropped silently.
- Nested `<svg>` viewports now synthesize a rectangle clip-path in outer-parent
  coordinates so descendants outside the authored viewport rect are masked at
  tessellation time (default `overflow:hidden` for `<svg>`). Sibling viewports
  mint distinct ids; doubly-nested viewports cascade the innermost clip onto
  descendants. `<clipPath>`, `<linearGradient>`, and `<filter>` defs inside a
  nested `<svg>` reach the global registry. Empty or zero-area viewports skip
  emission. When the inner `<svg>` carries an authored `clip-path` (presentation
  attr / CSS / inline style), the synth viewport clip is suppressed so the
  asset's explicit semantic survives — true intersection of viewport and author
  clip awaits a multi-clip renderer.
- `gui.PreserveAlignFractions` exported (was `preserveAlignFractions`) so
  `gui/svg` can resolve `preserveAspectRatio` align fractions without
  duplicating the switch.

### Hardened

- SMIL `from`/`to`/`by`/`values` reject malformed tokens (NaN, Inf, garbage)
  instead of coercing to 0. Bogus 0 endpoints would previously synthesize real
  animation timelines; now the timeline drops. Color keyframe stops with invalid
  paint also drop the whole color timeline.
- `<use>` `symbolViewportScale` rejects degenerate viewBox (`<= 0` width/height)
  and clamps combined translate (`tx+ax`, `ty+ay`) via `boundedScale` so
  alignment offsets cannot push the transform past `±maxCoordinate`.
- Nested-`<svg>` viewport math sanitizes NaN, ±Inf, and oversized inputs on
  `x`/`y`/`width`/`height`, `viewBox`, `parent.W`/`parent.H`, and the resulting
  scale/translate so a poisoned attribute cannot propagate non-finite values
  into the path transform. Percentages parse via float64 so `1e30%` no longer
  truncates to ±Inf before scaling.
- `mixOptsHash` clamps `HoveredElementID` / `FocusedElementID` via
  `clampElementID` (256-byte cap) before the FNV mix. Hostile callers passing
  megabyte-sized pseudo-state IDs can no longer burn CPU in the cache lookup
  hash phase; downstream `parseSvgWith` already clamped, so cache key and parsed
  state stay in sync.
- Inline-SVG cache `sourceKey` now hashes the source via SHA-256 instead of
  retaining the raw string. With the 4 MB parse cap and 512 cache slots, the
  prior format pinned up to 2 GB of source retention from cache keys alone.
  Hashing is incremental (no `[]byte(data)` copy) and produces a fixed 71-byte
  key.
- `mintUseSliceClipID` (slice `<use>` clip emitter) rejects non-finite or
  out-of-range `viewBox` numbers explicitly before falling back to viewBox dims
  for missing `<use>` width/height; a hostile `viewBox="0 0 NaN Inf"` can no
  longer survive `<= 0` coercion and propagate into the clip rect.
- Synthesized slice-clip ids (`__use_clip_N`) now skip any id already present in
  the document index, so an authored `id="__use_clip_1"` cannot silently shadow
  or be shadowed by the synthesized rect. Synth ids remain monotonic so two
  synth ids never collide with each other.
- `clampCycle` rejects NaN explicitly (NaN compares false against both `<= 0`
  and `> maxCycleSec`, so it would otherwise fall through unchanged into
  downstream cycle/floor math). `parseTimeValue` layers `finiteF32` so
  `dur="NaN"` / `dur="1e9999s"` cannot reach cycle math even if `parseF32` ever
  loosens.
- `parseKeySplinesIfSpline` switches to `parseFloatStrict` and rejects control
  points outside `[0, 1]`. `parseF32` would coerce `NaN`/`Inf` tokens to 0 and
  slip past the range check, silently producing wrong easing on hostile
  authoring.
- `parseAnimateDashArrayElement` defers `flat` allocation until stride is known
  from the first frame, sizing it to `len(frames) × stride` instead of
  `len(frames) × SvgAnimDashArrayCap` (4× less waste for the common stride=2
  case).
- `Parser.animatedScratch` pool is now bounded by `maxAnimatedScratchCap` (4096)
  on both ends. `putAnimatedScratch` rejects oversized buffers so one
  pathological frame cannot pin a giant backing array; `getAnimatedScratch`
  clamps `minCap` so a hostile SVG with synthesized millions of animated paths
  cannot force a giant `make` per frame.
- `findAttr` reverses the five entity escapes emitted by `buildOpenTag`
  (`&amp;`, `&lt;`, `&gt;`, `&quot;`, `&#39;`) before returning attribute
  values. `encoding/xml` decodes attribute entities once, so without this
  reversal a legitimate `&` in an attribute round-tripped as the literal
  sequence `&amp;` and reached downstream parsers (color, url, id, transform) as
  garbage. Unknown entities pass through unchanged; allocation occurs only when
  at least one `&` is present.
- `decodeSvgTree` now accumulates per-node CharData into a parallel stack of
  `strings.Builder` frames instead of `top.Text += s` / `last.Tail += s`. A
  hostile `<text>` body fragmented into many small chunks (e.g. by sprinkling
  numeric entity refs) was O(N²) under the prior incremental concat; the builder
  rewrite is linear. Tail accumulation flushes to `Children[idx].Tail` before
  the next sibling append so a slice grow cannot strand pending tail data.

### Fixed

- `<text>` now routes through the CSS cascade like shapes, so author rules
  (`text { fill: ... }`), `:hover` / `:focus` matches, and `display:none` apply.
  Previously `<text>` only saw inherited computed style with no per-element rule
  matching.
- Invalid color syntax (e.g. `fill="#GGGGGG"`, `fill="rgb(abc,def,ghi)"`,
  `stroke=""`) is now ignored by the cascade per CSS "invalid → ignore", letting
  inherited paint survive instead of clobbering with transparent black.
  `parseHexColor` rejects non-hex digits; `parseRGBColor` rejects non-numeric
  channels.
- CSS-wide control keywords (`inherit`, `unset`, `revert`, `revert-layer`) on
  `fill` / `stroke` are no-ops so the cascade- copied parent paint survives.
  `<text stroke="inherit">` with no ancestor stroke now falls back to a visible
  default rather than being silently dropped.
- `<text>` now inherits `stroke` / `stroke-width` from the cascade, and
  `stroke="inherit"` resolves against the cascade rather than forcing black.
  `<text stroke="none">` clears any ancestor stroke.
- `<tspan>` honors its own `stroke`, `stroke-width`, and `opacity` attrs instead
  of silently copying parent values. `opacity="50%"` on `<tspan>` now equals 0.5
  (matches CSS keyframe parity below).
- Mixed-content `<text>` runs preserve trailing and interleaved char data.
  `<text>A <tspan>B</tspan> C</text>` now renders all three runs; previously the
  trailing "C" was dropped because only pre-first-child `Leading` text was
  captured. New `xmlNode.Tail` field stashes post-child char data so
  `<use>`-cloned subtrees carry it through too.
- CSS `@keyframes { opacity: 50% }` now compiles to 0.5 (was 1.0).
  `compileOpacityTimeline` switched from `parseFloatTrimmed` to
  `parseOpacityNumber` so the static cascade and animated values agree on
  percentage notation.
- `Parser.InvalidateSvgSource` now correctly drops file-backed cache entries and
  every option-variant (FlatnessTolerance, HoveredElementID, FocusedElementID,
  PrefersReducedMotion). Prior impl reconstructed hashes from the path string
  alone, which never matched file entries (whose key mixes file contents) and
  only covered two of the option permutations. Walks the entry table by a stored
  `sourceKey` instead.
- `<use>` cloned subtrees no longer leak duplicate descendant ids. `stripID` is
  now recursive; previously only the clone root and (for `<symbol>` targets) its
  top-level children had their ids removed, so any nested id collided with the
  original and corrupted `url(#id)` resolution, CSS `#id` matching, and
  animation targeting.
- `<use width=W height=H>` of a `<symbol viewBox=...>` now scales the symbol's
  viewport to fill the requested box via a composed
  `translate · scale · translate(-vbX,-vbY)` transform. Width/height were
  previously dropped, so callers could not size symbol reuses.
- `clip-path` / `filter` declarations are now marked authored only after the
  value resolves to a usable `url(#id)` reference or the `none` keyword.
  Previously the cascade flipped the authored flag on property name alone, so
  `clip-path: bogus` could suppress the synthesized nested-`<svg>` viewport clip
  and `filter: bogus` could allocate a fresh per-occurrence offscreen group
  buffer for a declaration that contributed no actual filter.
- Markdown inline math (`$...$`) now renders after the async codecogs fetch
  completes. The cross-frame RTF layout cache key did not include diagram cache
  state, so the layout shaped on the first frame with the raw-LaTeX text
  fallback (cache=Loading) was reused after the fetch transitioned to Ready —
  the InlineObject placeholder was never emitted and `renderRtf` produced no
  `RenderImage`. New `rtfMathStateKey` mixes per-math-run State/Width/Height/DPI
  into the cache key so a Loading→Ready transition forces re-shape. Display math
  (`$$...$$`) was unaffected because it renders through the `Image` view, not
  RTF. FNV-1a constants in `view_rtf.go` extracted to package consts
  (`fnvOffset64`, `fnvPrime64`, `fnvFieldSep`).

### Security

- `<use x="…" y="…">` author values are parsed numerically instead of spliced
  into the synthesized transform attribute, closing an injection vector
  (`x="0)scale(99)"` previously emitted an extra `scale` into the transform
  list). Also rejects percentage `x`/`y` rather than treating "50%" as raw 50,
  and clamps `<use>`-vs- -viewBox scale to ±maxCoordinate to prevent
  pathological tiny viewBox dims from emitting absurd scale factors.
- `stroke-width` on `<text>` and `<tspan>` clamps NaN and negative values to 0
  via new `sanitizeStrokeWidth`. Negative widths are invalid per SVG spec; NaN
  propagation broke tessellation (uint8/uint16 casts implementation-defined, Inf
  coords break bbox math).
- `writeAttrEscaped` (used to reconstruct each element's `OpenTag` for
  substring-scanning helpers like `findAttr` / `findStyleProperty`) now also
  escapes `'` (`&#39;`) and `>` (`&gt;`). A hostile attribute value containing a
  single quote could previously smuggle a fake attribute past the cascade
  (`<rect note=" x='99' " x="1"/>` parsed as `x=99`). Both quote styles plus
  `<`/`>`/`&` are now escaped so no value can terminate the embedded attr or
  open a markup token.
- `parseSvg(string)` and `parseSvgDimensions(string)` now enforce the existing 4
  MB `maxSvgFileSize` cap. The cap was previously applied only to file-loaded
  content; callers passing arbitrarily large in-memory strings (e.g.
  network-fetched SVGs) bypassed it, letting unbounded `xml.CharData`
  accumulation and full-document scans run on hostile input. `parseSvg` returns
  an error; `parseSvgDimensions` truncates to the cap before probing.
- `clipPath` triangulation is now cached per `ClipPathID` for the duration of
  one `tessellatePaths` call. N paths sharing one complex `clipPath` previously
  triggered N full re-tessellations (`O(N · clipComplexity)` CPU DoS); the cache
  reduces this to one tessellation per unique id. Cache is `nil` when the
  graphic declares no `clipPath`s, so the common icon/spinner path takes no
  extra allocation.

## [v0.15.0] - 2026-04-27

### Added

- `<use href="#id">` (and `xlink:href`) resolution. The referenced subtree is
  cloned at parse time, wrapped in a synthesized `<g>` carrying a
  `translate(x,y)` transform plus the `<use>` presentation attrs (`fill`,
  `style`, `class`, ...). Cycles are guarded by a visited-set + depth-8 cap; the
  clone has its `id` stripped to avoid duplicate ids in the post-expansion tree.
- `<symbol>` is now honored as a `<use>` target — the symbol's children are
  inlined directly (the wrapper is dropped). Untargeted `<symbol>` elements
  continue to render no output. Symbol-level `viewBox` / `preserveAspectRatio`
  honoring is a future polish.
- `spreadMethod` on `<linearGradient>` and `<radialGradient>`: `pad` (default),
  `reflect` (triangle wave), `repeat` (sawtooth).
  `gui.SvgGradientDef.SpreadMethod` is the new field; the previous silent-pad
  behavior is the zero-value default so existing fingerprints stay stable.
- `gui.SvgCfg.FlatnessTolerance float32` — tessellation tolerance floor in
  viewBox units. Default 0 keeps the historic 0.15 floor. Plumbed via a new
  `SvgParseOpts.FlatnessTolerance` field and a `Window.LoadSvgWithOpts` method;
  the cache key tracks tolerance per quantized 1e-4 step.
- `gui.SvgCfg.HoveredElementID` / `FocusedElementID string` — drive CSS `:hover`
  / `:focus` matching for the SVG element with that id. Plumbed through
  `SvgParseOpts` into the cascade `MatchState`; cache invalidates per id
  transition.
- `examples/svg_use_symbol`, `examples/svg_gradient_spread`,
  `examples/svg_flatness`, `examples/svg_css_states`.

### Changed

- `gui.SvgGradientDef` gains a `SpreadMethod SvgGradientSpread` field. Keyed
  struct literals are unaffected; positional users in sibling repos must update.
- `gui.SvgParseOpts` gains `FlatnessTolerance float32`,
  `HoveredElementID string`, `FocusedElementID string`. Additive.
- `gui/svg.ParseOptions` mirrors the same additions.
- `gui/svg.VectorGraphic` gains `FlatnessTolerance float32`. Internal.
- `Window.LoadSvgWithOpts(src, w, h, opts SvgParseOpts)` is the new
  per-render-override entry point. `Window.LoadSvg` is unchanged.

### Deferred to v0.16.0

- Automatic mouse-driven hover detection on the `Svg` widget. v0.15.0 ships the
  parser/cascade/cache plumbing so apps can drive `HoveredElementID` themselves
  (e.g. by hit-testing `TessellatedPath.ContainsPoint`); built-in pointer
  tracking with internal hit-test on the widget will land in v0.16.0.
- `<symbol>` `viewBox` / `preserveAspectRatio` honoring.
- `spreadMethod`-aware stop-boundary subdivision (currently pad-clamped, so
  reflect/repeat AA at wrap points is slightly softer than at first/last stop).

## [v0.14.0] - 2026-04-26

### Added

- CSS sibling combinators: adjacent (`+`) and general sibling (`~`). Match
  engine (`gui/svg/css`) now takes a preceding-siblings slice alongside
  ancestors when resolving complex selectors.
- CSS attribute selectors: `[name]`, `[name=v]`, `[name~=v]`, `[name|=v]`,
  `[name^=v]`, `[name$=v]`, `[name*=v]`. Names are case-insensitive; values are
  case-sensitive (no `i`/`s` flag). `ElementInfo.Attrs map[string]string`
  carries the per-element attribute map; svg parser populates it from the raw
  open tag.
- CSS `:hover`, `:focus`, `:not(inner)` selectors — parser + matcher only.
  `Compound` gained `HoverPseudo`, `FocusPseudo`, `Not` fields; `ElementInfo`
  gained a `MatchState{Hover, Focus bool}` block. Build-time state can be set
  via `ElementInfo.State`; runtime mouse-event auto-toggle is deferred to
  v0.15.0.
- `:not()` is single-compound only — comma-list (`:not(.a, .b)`) and nested
  `:not(:not(...))` are deferred.
- `var(--name, fallback)` resolution. The fallback is itself resolved
  recursively (so `var(--a, var(--b, red))` works); recursion bounded at
  depth 32.
- `calc()` arithmetic: `+ - * /`, parens, units `px` and unitless. Mixed-unit
  operands and divide-by-zero invalidate the declaration per spec. Nested
  `calc()` and `calc()` inside `var()` fallback are resolved.
- `examples/svg_css_selectors`, `examples/svg_css_vars` — visual demos for the
  new selector and value-resolution machinery.

### Changed

- `css.Match()` and `css.ComplexSelector.Matches()` gained a
  `siblings []ElementInfo` parameter. The sole external caller in
  `gui/svg/style.go` is updated; sibling repos (go-glyph, go-charts, go-edit,
  go-kite) do not call into `gui/svg/css` directly. Internal test sites pass
  `nil` for the new param.
- `Compound`, `ElementInfo`, `MatchedDecl` gained additive fields. Keyed struct
  literals are unaffected.
- `gui/svg.makeElementInfo()` signature gained an `attrs map[string]string`
  parameter (the parsed open-tag attributes).
- The CSS package status table in `docs/svg-support.md` flips several rows from
  "No" to "Yes" (sibling combinators, attribute selectors, `:not()`, `var()`
  fallback, `calc()`).

### Deferred to v0.15.0

- `:hover` / `:focus` runtime mouse-event auto-toggle. The selector is
  recognized today; v0.15.0 will wire the dispatcher (sits at the `gui` ↔
  `gui/svg` ↔ backend interface boundary, lands cleanly alongside
  `<use>`/`<symbol>` dynamic-cascade work).
- `examples/svg_css_states` — depends on the runtime auto-toggle.

## [v0.13.0] - unreleased

### Added

- SVG accessibility metadata. `<title>`, `<desc>`, `aria-label`,
  `aria-roledescription`, and `aria-hidden` on the root `<svg>` are now parsed
  and exposed via `SvgParsed.A11y` (new `SvgA11y` nested struct). Previously
  dropped silently.
- `<radialGradient>` is now parsed and rendered. Supports
  `cx`/`cy`/`r`/`fx`/`fy` in `objectBoundingBox` (default) or `userSpaceOnUse`.
  Stops use the same semantics as linear gradients. Focal interpolation uses a
  simplified distance-from-focal model; full SVG cone-focused projection is
  noted as future polish in `docs/svg-support.md`.
- `preserveAspectRatio` is now honored on the root `<svg>`. All 9 alignment
  values (`xMin`/`Mid`/`Max` × `YMin`/`Mid`/`Max`) plus `meet`/`slice` are
  supported. The default (`xMidYMid meet`) is unchanged from prior behavior, so
  existing SVGs render identically. `none` (non-uniform stretch) currently falls
  back to default — adding non-uniform render support is tracked as polish.
- `(*TessellatedPath).ContainsPoint(px, py)` for hit-testing filled SVG paths.
  `TessellatedPath` now carries a precomputed bbox (`MinX`/`MinY`/`MaxX`/`MaxY`)
  for fast reject. Author base transforms are inverted before the barycentric
  triangle test. Stroke contributions are skipped — pass the fill
  `TessellatedPath` for hit-testing.
- `examples/svg_a11y`, `examples/svg_radial`, `examples/svg_aspect`,
  `examples/svg_hittest` — visual demos for each new feature.

### Changed

- `SvgParsed`, `TessellatedPath`, and `CachedSvg` gained additive fields. Keyed
  struct literals are unaffected; positional literals would need to be updated
  (none found in tree or sibling repos — go-glyph, go-charts, go-edit, go-kite).

## [v0.12.7] - 2026-04-26

### Fixed

- SVG fingerprint goldens (`TestPhase0SmilSpinnerFingerprint`,
  `TestPhaseGCssSpinnerFingerprint`) failed on Linux/WASM CI because amd64 ships
  an asm `math.Sin`/`math.Cos` while arm64 uses pure-Go — ULP-level drift in
  trig output flipped digest bits versus the darwin-generated goldens.
  `hashTessellated` / `hashAnimations` now quantize finite floats to a 1e-3 grid
  before `Float32bits`, so the fingerprints stay platform-stable while still
  catching real geometry regressions. Goldens regenerated.

## [v0.12.6] - 2026-04-25

### Added

- `SvgSpinner` widget for animated SVG loaders. Full SMIL pipeline: `animate`,
  `animateTransform` (rotate/translate/scale), `animateMotion`, per-shape
  animation keying, attribute overrides, spline easing, syncbase `begin` timing,
  dash animations, TRS-sandwich transforms, and per-role opacity. CSS pipeline
  added: cascade, `@keyframes`, `@media`, animation shorthand. Ships with 39
  spinner assets across the SMIL and CSS sets. See `examples/showcase` for the
  live gallery.
- `TessellateAnimated` plus parse benchmarks for the SVG path/anim pipeline.
- Standalone XML tree parser with per-path animation routing.

### Changed

- SVG parser correctness and performance improvements: scanline fill-rule,
  `Z`-then-`M` path parse fix, dead `GroupID` stripped from
  `TessellatedPath`/`CachedSvgPath`, deduped float helpers, hardened animation
  pipeline.
- Ear-clip tessellator capped at 2048 verts to keep CI under timeout.
- README rewritten: accurate why-go-gui section, spinners video, formatting
  fixes, immediate-mode framing toned down.

## [v0.12.5] - 2026-04-18

### Changed

- `Animation.Update` now takes `*AnimationCommands` instead of
  `*[]queuedCommand`. `queuedCommand` was always unexported, which made the
  `Animation` interface effectively impossible for third- party packages to
  implement — they could not name the parameter type. `AnimationCommands` wraps
  the deferred command queue behind two public methods:
  - `AppendOnDone(fn func(*Window))` — queues a terminal callback.
  - `AppendOnValue(fn func(float32, *Window), v float32)` — queues a per-frame
    interpolated-value callback. All existing first-party animations (`Animate`,
    `SpringAnimation`, `TweenAnimation`, `KeyframeAnimation`,
    `LayoutTransition`, `HeroTransition`, `BlinkCursorAnimation`) updated;
    callers of the stable concrete factories (`NewSpringAnimation`, etc.) see no
    change. Breaking only for downstream code that implemented `Animation`
    directly — impossible to do before this release, so no real-world migration.

## [v0.12.4] - 2026-04-18

### Added

- Per-call image fetcher on `DrawContext`. New
  `DrawContext.ImageWithFetcher(..., fetcher ImageFetcher)` and matching
  `DrawCanvasImageEntry.Fetcher` field let each image draw override
  `WindowCfg.ImageFetcher` for its own download. Typical use: a map widget pairs
  each tile layer with its source-specific User-Agent (OSM-policy UA for one
  layer, a WMS-provider UA for another) without a shared composite fetcher.
  Existing `DrawContext.Image` is unchanged and still routes through the
  window-level fetcher.
- New exported `ImageFetcher` function type and
  `ResolveImageSrcWithFetcher(w, src, fetcher)` helper. Existing
  `ResolveImageSrc(w, src)` is a thin wrapper that passes `nil`, so no caller
  needs to migrate.

### Notes

- Scope cut: `ImageCfg` (the Image widget) keeps the single-fetcher path.
  Per-widget fetcher override will land when a consumer demands it; no
  speculative API.
- Known limit: downloads are URL-keyed process-wide, so the first entry observed
  for a URL binds the fetcher for that URL's in- flight download. Consumers
  wiring two fetchers to overlapping URL namespaces must route by URL prefix
  themselves.

## [v0.12.3] - 2026-04-17

### Fixed

- `renderDrawCanvas` now emits images before triangle batches and text, so
  `DrawCanvas` consumers that compose tile backgrounds with `DrawContext`
  overlays get the correct z-order. Previously images painted on top of every
  batch/text in the same canvas — invisible in unit tests that only inspect
  `Texts()`/`Batches()` but user- visible once a tile-map demo ran in a window

### Changed

- SDL2 / GL / Metal backends now forward high-resolution
  `MouseWheelEvent.PreciseX` / `PreciseY` for smooth-scroll devices (trackpad
  pixel-scroll, Magic Mouse, high-res wheels), falling back to integer `X`/`Y`
  when the precise field is zero or the SDL runtime predates 2.0.18. Enables
  sub-integer scroll deltas in consumers that accumulate fractional `ScrollY`

## [v0.12.2] - 2026-04-16

### Added

- Image download pipeline now handles remote URLs for `DrawContext.Image`.
  Shared `ResolveImageSrc(w, src)` resolves http/https URLs to local cache
  paths, schedules background downloads when uncached, and returns "" while in
  flight. `gui.Image` and `emitDrawCanvasImages` both route through it so
  DrawCanvas tiles render after the first fetch
- `WindowCfg.ImageFetcher` hook: apps can supply a custom HTTP client to set
  User-Agent, auth headers, or route through a shared pool. Default fetcher
  sends `User-Agent: go-gui/vX.Y.Z` so providers (e.g. OSM) can identify traffic
- `WindowCfg.MaxImageDownloads`: process-wide cap on concurrent image downloads.
  Defaults to 6; first-window-wins for sizing
- Exported `Version` const tracks the module tag

### Fixed

- HTTP status codes are now checked before the body is written to disk. Non-200
  responses (4xx/5xx) no longer poison the cache with error-page payloads

### Changed

- `downloadImage` dropped the HEAD pre-flight and validates size/content-type on
  the GET response. Single round trip per fetch

### Performance

- `ResolveImageSrc` caches the URL→path mapping per window so already-resolved
  tiles skip the `MkdirAll` + `Stat` syscalls each frame. Critical for
  DrawCanvas-based tile maps that render dozens of images per frame at 60fps

## [v0.12.1] - 2026-04-16

### Added

- DrawCanvas: `DrawContext.Image(x, y, w, h, src, bgOpacity, bgColor)` draws
  images inside the canvas via the same deferred-emit pipeline as text. `src`
  accepts the same forms as `ImageCfg.Src` (local path, http/https URL, data
  URL)
- DrawCanvas: `DrawCanvasCfg.IDFocus` and `OnKeyDown` enable keyboard focus and
  key event handling. A11Y role flips to button when the canvas is focusable

## [v0.12.0] - 2026-04-15

### Added

- Time-travel debugging: opt-in via WindowCfg.DebugTimeTravel. User state
  implements Snapshotter (Snapshot/Restore; optional Size). Framework captures a
  snapshot after every dispatched event; scrubber window auto-spawns alongside
  the app window with a slider, step buttons (first/prev/next/last), cause
  label, counter, freeze toggle, and keyboard shortcuts (arrows, home/end,
  space, esc)
- Window.Now() virtual clock: returns pinned snapshot timestamp during scrub,
  live time otherwise; use in view fns that render clock-driven data so scrubbed
  frames match their snapshot
- Window.EnableHistory(maxBytes), HistoryLen(), OpenDebugWindow(),
  Freeze/Resume/IsFrozen, PostRestore(idx) public API
- RegisterNamespaceSnapshot(ns): widget authors opt additional StateMap
  namespaces into scrub restore; scroll (nsScrollX/nsScrollY) and widget-local
  focus (nsInputFocus, nsListBoxFocus, nsTreeFocus) are pre-registered
- BoundedMap.cloneAny/restoreAny: type-preserving snapshot through an interface
  so whitelisted namespaces rewind without reflection
- examples/time_travel: counter demo wiring Snapshotter + DebugTimeTravel

### Hardening

- Snapshotter.Size() capped at 1 GiB to prevent totalBytes overflow
- Slider NaN/Inf rejected before int conversion in the scrubber
- BoundedMap restore recovers from type-assertion panics so a single out-of-sync
  namespace doesn't break the rest of the scrub
- Parent-window title truncated before composing the scrubber title

### Notes

- Read-only scrub only: rewinding state does not un-do past side effects (HTTP
  requests, file writes, sounds)
- Requires multi-window mode (App + App.OpenWindow). Single-window apps log a
  notice and no-op
- Zero-cost when disabled: nil-history check short-circuits the hot path with no
  allocation

## [v0.11.0] - 2026-04-14

### Added

- WindowCfg.OnCloseRequest hook: intercept OS window-close and app-quit events
  for save/discard/cancel prompts. Callback owns calling Window.Close() to
  proceed or doing nothing to cancel. Dispatch extracted into
  DispatchCloseRequest / DispatchQuitRequest helpers shared by sdl2/gl/metal
  backends.

## [v0.10.0] - 2026-04-14

### Added

- DockNode/SplitterState JSON serialization: struct tags, text-marshaled enums
  (DockNodeKind, DockSplitDir, SplitterOrientation, SplitterCollapsed),
  DockNodeSanitize for post-unmarshal hardening
- Showcase docs: new dock_layout component entry, splitter serialization section

### Changed

- SplitterStateNormalize handles NaN/Inf ratios and invalid Collapsed values
- Modernize: sync.OnceFunc/OnceValue, slices.SortStableFunc, cmp.Compare

## [v0.9.9] - 2026-04-13

### Fixed

- Metal backend: native Cocoa file-drop bridge bypasses go-sdl2 crash (SDL_free
  on Cocoa-allocated string); per-window callback map for multi-window support

## [v0.9.8] - 2026-04-13

### Added

- File-drop event support: OnFileDrop callback on Container, DrawCanvas, and
  EventHandlers; SDL2 backend maps DropEvent to EventFileDropped

### Changed

- Rename EventFilesDropped → EventFileDropped (singular)

## [v0.9.7] - 2026-04-13

### Changed

- Bump go-glyph dependency from v1.6.4 to v1.6.5

## [v0.9.6] - 2026-04-12

### Changed

- Deduplicate helpers across gui/ (asciiLower, f64Clamp, FNV-1a hash,
  skipLayoutChild, shapeBounds, emitClipCmd, cpInputColumn,
  progressBarCenterLabel, finishDiagramFetch, baseCfg)
- Replace `fmt.Sprintf` with `strconv` in hot paths (data grid, inspector, a11y,
  data source)
- Eliminate per-frame heap allocations: gesture Event scratch pool, defer
  removal in render/image opacity, rotateCoordsInverse float path, stack-array
  cellContent, inspector cache map reuse
- Convert copy-paste spinner tests to table-driven
- Remove redundant state and unnecessary comments
- 42 files changed, −278 net lines

## [v0.9.5] - 2026-04-11

### Added

- `Window.FrameCount() uint64` accessor for the monotonic frame counter; lets
  widgets detect repeat callbacks within a render cycle

## [v0.9.4] - 2026-04-11

### Added

- `Window.SetTitle(string)` + `Window.SetTitleFn(func(string))` — dynamic OS
  window title updates. Wired in sdl2, metal, and gl backends via
  `sdl.Window.SetTitle`
- Input hardening on `SetTitle`: 4 KiB cap, UTF-8-safe truncation, embedded-NUL
  stripping; no-alloc fast path for clean short inputs

## [v0.9.3] - 2026-04-10

### Added

- `NativeSaveDiscardDialog` — Save / Don't Save / Cancel alert for
  unsaved-changes flows
- Native menubar: route macOS app-menu "About" through `OnAction`

### Changed

- License: PolyForm NC 1.0 → MIT

### Fixed

- Solitaire example: replace double-click auto-move with right-click
- CI: brew upgrade harfbuzz/pango text stack on macOS; checkout go-glyph for
  test job and use local replace directive

## [v0.9.2] - 2026-04-09

### Added

- `Window.TextMeasurer()` accessor for downstream widgets that need direct
  access to the backend measurer

### Fixed

- Drop `t.Parallel` on tests mutating `guiTheme.ScrollMultiplier`
  (race-avoidance)

## [v0.9.1] - 2026-04-08

### Changed

- Bump `github.com/go-gui-org/go-glyph` to v1.6.4
- Bump `golang.org/x/sys` to v0.43.0

## [v0.9.0] - 2026-04-07

### Added

- `gui/highlight` subpackage: chroma-backed syntax highlighter with curated
  lexer set (go, python, js/ts, rust, c/cpp, java, ruby, shell, html, css, json,
  yaml, toml, sql, markdown, diff, dockerfile, make) and DoS caps (256KB source,
  100k tokens)
- `MarkdownStyle.CodeHighlighter` field: optional highlighter for fenced code
  blocks; nil preserves parser's built-in tokenizer
- `MarkdownStyle.CodeTypeColor`, `CodeFunctionColor`, `CodeBuiltinColor` palette
  fields
- Showcase: component docs, welcome, data grid features, markdown demo, and
  inspector overlay all use `highlight.Default()`

## [v0.8.0] - 2026-04-06

### Added

- `Spinner` widget: animated mathematical curve loading indicator with 21 named
  `CurveType` constants (rose, lissajous, hypotrochoid, butterfly, cardioid,
  lemniscate, epitrochoid, heart wave, spiral, fourier and variants)
- Spinner particle-trail rendering via `DrawCanvas` with faint ghost path
  outline
- Spinner optional slow rotation (`Rotate` field, 30s per revolution)
- Spinner `Opt[float32]` params (ParamA/B/D) for custom curve tuning
- DrawContext: `QuadBezier`, `CubicBezier` drawing primitives
- DrawCanvas: `OnMouseUp` event
- `ClearNamespace` and `ClearDrawCanvasCache` for targeted cache flush
- Mouse button state in motion events; `OnMouseMove` on `DrawCanvas`
- `OnMouseLeave` event and `RequestRedraw()` for tooltip support
- Showcase: Spinner demo with all 21 curves, varied colors, and rotation
  examples

### Fixed

- Table column auto-sizing; DrawRecorder `Text()` fall-through
- Live resize redraw on Windows (SDL event watcher)
- Mutex safety: defer Unlock, add missing lock in `ClearViewState`
- gofmt alignment in theme_defaults const blocks

### Changed

- Bump go-gl/gl to 2025-03-31 snapshot
- Bump go-glyph v1.6.1 → v1.6.2
- Set default font to Segoe UI on Windows

## [v0.7.0] - 2026-04-02

### Breaking

- `GridPaginationCursor`/`GridPaginationOffset` iota values shifted; new
  `GridPaginationNone` (0) added
- `Color.Over` returns `ColorTransparent` (set=true) instead of zero `Color`
  when both inputs are fully transparent
- `executeFocusCallback`/`executeMouseCallback` removed unused debug string
  parameter

### Fixed

- Race: synchronize `guiTheme` and `Default*Style` globals with `sync.RWMutex`
- Race: `App.Broadcast` no longer holds lock during user callback (deadlock)
- Race: metal a11y buffers protected with mutex
- Race: SDL2 resize event watcher allocates per-callback instead of sharing
  pointer
- Bug: layout overflow hides visible children when Float/OverDraw interleaved
- Bug: Fill distribution subtracts OverDraw widths never added to parent
- Bug: stencil depth decrement without matching increment at depth 255
- Bug: masked input edits skip undo/redo stack
- Bug: `InputDate.OnSelect` passes nil `*Event` to callback
- Bug: `queueOnValue` missing nil function guard
- Bug: `ColorFromHSVA` produces wrong colors for negative hue
- Bug: data grid OnHover closure captures stale window pointer
- Correctness: `renderImage`/`renderShape` use defer for shape color restore
- Correctness: SVG render checks `rectIntersection` ok before drawing
- Correctness: `render_validate` checks NaN/Inf/nil for gradient, shadow, blur,
  shader, rotate
- Correctness: `WithColors` borderFocus falls back to theme-level `ColorSelect`
- Correctness: `WithColors` updates SkeletonStyle and InspectorStyle
- Correctness: `AdjustFontSize` clamps each sub-size individually
- Correctness: `SetTheme` syncs `DefaultInspectorStyle`
- Correctness: `ColorFilterCompose` nil-checks inputs
- Correctness: scroll handlers set `IsHandled` and use shape-relative coords
- Correctness: gesture emits rotate `Began` before first `Changed`
- Correctness: `InMemoryDataSource.Capabilities` acquires read lock
- Correctness: `effectivePaginationKind` returns `GridPaginationNone` when
  unsupported
- Correctness: dock tree nil Root guard
- Correctness: `bounded_map` eviction handles tombstone-only runs
- Fix: variable shadowing in gesture, data_source, data_source_orm,
  locale_bundle, view_listbox
- Fix: date-dependent nil panic in TestDatePickerSubElementClickFocus
- Fix: wrap bench missing pool reset; raise CI alert threshold to 200%

### Added

- `GridPaginationNone` constant for unsupported pagination
- `WithInspectorStyle` theme builder
- `StrSourceChanged` locale field
- Data grid CRUD source-change detection and toolbar indicator

### Changed

- Replace `intMin`/`intMax` with Go builtin `min`/`max` (33 call sites)
- Replace `fmt.Sprintf` with `strconv` in per-frame data grid/source paths
- `f32IsFinite` uses bit-pattern check instead of float64 round-trip
- `ColorFilter` factories return pointers to package-level singletons
- `Shortcut.String()` pre-allocates buffer
- `contentWidth`/`contentHeight` skip Float and ShapeNone children, matching
  `spacing()`
- Move test-only helpers from production files to `_test.go` files
- `native_print` uses defer for lock/unlock
- Document animation spring divergence threshold and zero-delay repeat behavior

## [v0.6.0] - 2026-04-01

### Added

- DrawContext: `Text`, `TextWidth`, `FontHeight` for canvas text rendering
- DrawContext: `FilledRoundedRect`, `RoundedRect` for rounded-corner rectangles
- DrawContext: `DashedLine`, `DashedPolyline` for dashed stroke patterns
- DrawContext: `PolylineJoined` for polylines with miter joins at vertices
- DrawContext: `Texts()`, `Batches()` accessors for testing canvas output
- Render pipeline emits `RenderText` commands from `DrawCanvas`
- Showcase: updated draw canvas demo with line chart (joined polyline, dashed
  grid, text labels) and bar chart (rounded bars, dashed reference line)
