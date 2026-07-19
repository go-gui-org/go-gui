# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- **Smooth programmatic scrolling.** New `Window.ScrollVerticalToSmooth` and
  `Window.ScrollHorizontalToSmooth` ease a scrollable to an absolute offset
  using the same exponential smoothing as discrete mouse-wheel scrolling.
  No-op when the scroll id is not found or the target equals the current
  offset; instant `ScrollVerticalTo`/`ScrollHorizontalTo` still cancel any
  in-flight ease. The wheel smoother's arm logic is now shared
  (`scrollSmoothParams`/`scrollSmoothArm`) between relative wheel deltas and
  absolute targets.

## [v0.39.0] - 2026-07-18

### Added

- **`BoundedMap.GetOr`.** New method on `BoundedMap` that accepts a constructor
  function, returning an existing value or publishing a new one without a
  double lookup. Internal `BoundedMap.Get` callers with ignored-ok returns have
  been migrated to `GetOr`, hardening the map against lost writes.
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

- **Dependency bump.** Updated go-glyph to v1.17.2 (background cache warming
  for CJK fallback coverage; no API change).

## [v0.38.1] - 2026-07-17

### Changed

- **Dependency bump.** Updated go-glyph to v1.17.1 (struct field alignment,
  lower per-instance memory; no API change).
- **`examples/fontviewer` cleanup.** Named magic numbers and removed code
  smells in the font viewer example.

## [v0.38.0] - 2026-07-17

### Added

- **Font viewer example + font-enumeration API.** New `examples/fontviewer`
  browses installed system fonts in a virtualized card grid — name filter,
  editable sample text, 12–72 px size slider, click-to-copy. Backed by new
  public API: `gui.ListSystemFonts` with the optional `FontLister` backend
  capability, and `gui.ListVisibleRange(itemCount, rowHeight, listHeight,
  scrollY, overscan)` for grid/list virtualization. Requires go-glyph
  v1.17.0.

### Changed

- **`Sizing: FillFill` root now fills the window.** A root layout has no
  parent to fill against, so a `FillFill` root previously collapsed to
  content size; filling the window required boilerplate `WindowSize()` plus
  an explicit `Width`/`Height` and `FixedFixed`. Each Fill axis of the root
  is now pinned to the window dimension, so the intuitive spelling works and
  the examples/docs drop the boilerplate.

### Fixed

- **Fixed-size containers with a 0 dimension no longer break clipping and
  hit-testing.** A container with `SizingFixed` and an explicit 0
  width/height rendered (its children self-draw) but kept zero-area bounds,
  collapsing the `shapeClip` — and therefore the clip region and the
  pointer/hit-test region — of every descendant, so a child with `Clip:
  true` vanished and interactive children went inert. Such a box now
  degrades to content sizing on the zero axis. (#94)

## [v0.37.0] - 2026-07-16

### Changed

- **BREAKING: remaining interactive controls are focusable by default.**
  `ButtonCfg`, `RadioCfg`, `RadioButtonGroupCfg`, `ComboboxCfg`,
  `ListBoxCfg`, `TreeCfg`, `DatePickerCfg`, `ColorPickerCfg`,
  `NumericInputCfg`, and `InputDateCfg` drop `Focusable bool` and gain
  `FocusDisabled bool`: the zero value is now _focusable_, and
  `FocusDisabled: true` is the explicit opt-out. This completes the
  focusable-by-default flip started in v0.36.0 (which covered Input,
  Select, Slider, Toggle, and Switch).

  Focus still requires a non-empty `ID` — an ID-less control is inert.
  `Disabled` still excludes from the tab order.

  Migration (compile error on the removed field is the guide):
  - `Focusable: true` → delete the line (now the default).
  - `Focusable: <expr>` → `FocusDisabled: !<expr>`.
  - `Focusable: false` → `FocusDisabled: true`.

- **`InputDateCfg` outer Column gains focusability.** Previously the
  outer container never set `Focusable`, so even with `Focusable: true`
  the date field was unreachable by keyboard. The outer Column now maps
  to `Focusable: !cfg.FocusDisabled`, matching the inner Input's focus
  state.

- **`DatePickerCfg` and `ColorPickerCfg` drop redundant `cfg.Focusable`
  gates** on focus-visual handlers. Focus visuals (border, color) now
  always apply when focused; `Disabled` remains the guard.

- **`NumericInputCfg` and `InputDateCfg` propagate `FocusDisabled`
  directly** to their inner `Input` instead of translating the inverse
  (`!cfg.Focusable`). The opt-out intent passes through transparently.

### Fixed

- `InputDateCfg` callers: the outer Column was never focusable even with
  `Focusable: true`. The inner `Input` was reachable but the container
  wasn't — focus now flows consistently through the composite widget.

## [v0.36.0] - 2026-07-16

### Added

- **`InputCfg.ReadOnly`** — blocks text edits while the field stays
  focusable and selectable. Navigation, selection, and copy keep
  working; typing, IME text, paste, cut, undo/redo, delete, multiline
  Enter, and `PostCommitNormalize` are all skipped. Single-line Enter
  still fires `OnEnter`/`OnTextCommit`, with the text uncommitted and
  unnormalized. The field is announced to assistive tech as
  `AccessStateReadOnly`. Mirrors HTML's `readonly`, and is distinct from
  `Disabled`, which removes the field from interaction entirely.

  This state was previously inexpressible: `AccessStateReadOnly` could
  only be produced by setting `Focusable: false`, which also dropped the
  field from the tab order — so an Input was either editable or
  unreachable, with nothing in between. (With the focusable-by-default
  flip below, `ReadOnly` is now the only trigger for the read-only
  announcement.)

- **`NumericInputCfg.ReadOnly` and `InputDateCfg.ReadOnly`** — extend
  `InputCfg.ReadOnly` to the two composite wrappers. Both forward the
  flag to their inner `Input` (blocking typing) and gate the secondary
  mutation paths that bypass the text field: `NumericInput` disables and
  gates its step buttons at the `numericInputApplyStep` choke point, and
  `InputDate` keeps the calendar popup closed so its picker can never
  emit a selection. Enter-commit on the read-only inner `Input` no longer
  surfaces a value/date change. Both wrappers announce
  `AccessStateReadOnly`. `NumericInputCfg` and `InputDateCfg` already had
  `Disabled`; `ReadOnly` is the focusable-but-uneditable counterpart.

### Fixed

- **Read-only Input no longer renders IME preedit.** A composition
  started on a read-only field displayed preedit text that could never
  commit (`makeInputOnChar` swallows the commit), leaving a stray
  artifact until focus change. Preedit rendering is now suppressed for
  read-only fields; selection and cursor still render, since the field
  stays `Focusable`. Editable fields are unaffected.

- **`CommandButton` now auto-fills `ID`** from the command ID, prefixed
  with `cmdbtn:`. Focus traversal is keyed by `Shape.ID`, so a
  `CommandButton` with `Focusable: true` but no explicit `ID` was
  silently unreachable by keyboard. The prefix keeps the button's focus
  ID distinct from the menu item driven by the same command, which
  carries the raw command ID. Widgets that were dead tab stops now join
  the tab order; pass an explicit `cfg.ID` for two buttons on one
  command in the same window.
- **36 examples** set `Focusable: true` without an `ID` and were not
  keyboard-reachable, including `get_started`. All now carry stable IDs.
  `snake` also dropped `controlsIDBase`/`startButtonID` numeric focus
  IDs left over from the removed `IDFocus uint32` API.

### Changed

- **BREAKING: input controls are focusable by default.** `InputCfg`,
  `SelectCfg`, `SliderCfg`, `ToggleCfg`, and `SwitchCfg` drop
  `Focusable bool` and gain `FocusDisabled bool`: the zero value is now
  _focusable_, and `FocusDisabled: true` is the explicit opt-out. An
  input the user can't tab to is a bug, not a design choice — and for
  `Select`/`Slider`/`Toggle`/`Switch` this is an accessibility fix, not
  just deboilerplating: ID-bearing call sites that never set `Focusable`
  now join the Tab order (a slider should be keyboard-adjustable).

  Focus still requires a non-empty `ID` (`Focusable && ID != ""`). An
  ID-less control is **inert**: it renders but never becomes a tab stop,
  and no ID is ever fabricated. `Disabled` still excludes a control from
  the tab order; `ReadOnly` still keeps it focusable.

  Migration (compile error on the removed field is the guide):
  - `Focusable: true` → delete the line (now the default).
  - `Focusable: <expr>` → `FocusDisabled: !<expr>`.
  - `Focusable: false` → `FocusDisabled: true`.

  Out-of-scope widgets keep opt-in `Focusable bool`: Button, Container,
  Text, and the composites/wrappers (`Combobox`, `DatePicker`,
  `ListBox`, `RadioButtonGroup`, `NumericInput`, `InputDate`, `Radio`,
  ColorPicker, ThemePicker, Tree) — the wrappers translate their
  `Focusable` into the inner Input's `FocusDisabled`.

  The four focus flags, disambiguated:

  | Flag                  | Meaning                                                 |
  | --------------------- | ------------------------------------------------------- |
  | `Shape.Focusable`     | widget participates in the focus system                 |
  | `FocusSkip`           | focusable + click/selection, but excluded from Tab order |
  | `FocusDisabled` (Cfg) | opt out of the default-on focus (in-scope Cfgs)         |
  | `Disabled`            | non-interactive; also excluded from Tab order           |

- **A non-focusable Input no longer announces `AccessStateReadOnly`.**
  Before the flip, `Focusable: false` was the only way to express an
  uneditable field, so it doubled as the read-only signal. Now that
  non-focusable means an explicit `FocusDisabled` opt-out, only
  `ReadOnly: true` announces read-only.

- **`requiredid` analyzer** now also flags Cfg literals that set
  `Focusable: true` without a non-empty `ID`, catching this class of
  silent no-op at `go vet` time.

## [v0.35.1] - 2026-07-15

Documentation-only release. No code or behavior changes; no migration needed.

### Changed

- **`GenerateViewLayout` is no longer deprecated**. It is the supported
  entry point for composite View widgets, which need to recurse a View
  tree into a Layout tree. The deprecation pointed at
  `View.GenerateLayout`, which builds a single node and does not recurse
  into `Content()` — it was never an equivalent replacement, and no other
  exported path existed. Callers that hand-rolled their own recursion to
  avoid the warning should call `GenerateViewLayout` again; it applies
  shape normalization, the child-count clamp, and frame-arena pre-sizing
  that a hand-rolled copy misses. (#52)

### Fixed

- **README**: removed SDL2-era install steps, corrected the text stack
  description to the current pure-Go path, and refreshed the code samples
  to the current API.

## [v0.35.0] - 2026-07-15

### Changed

- **BREAKING — Scroll API**: `Shape.IDScroll uint32` is replaced by
  `Scrollable bool` plus string scroll identity (the widget's `ID`). A
  container opts into the scroll system with `Scrollable: true` and a
  non-empty `ID`; scroll offset is keyed by that `ID`. Migration:
  `IDScroll: N` → `Scrollable: true` (with a non-empty `ID`). Scrollable
  containers now panic at build if `ID` is empty (`RequireScrollID`).
- **BREAKING — Lost scroll handle**: the caller-supplied `IDScroll uint32`
  is removed from `ContainerCfg`, `ListBoxCfg`, `TreeCfg`, `TableCfg`,
  `ComboboxCfg`, `CommandPaletteCfg`, `InputCfg` and `DataGridCfg`. The
  scroll key is now *derived* from the widget's `ID`; pass that same
  derived string to `Window.ScrollVerticalTo`/`ScrollHorizontalTo`/`…Pct`
  etc. Derived keys:

  | Cfg | scroll key |
  |-----|------------|
  | `ContainerCfg`, `ListBoxCfg`, `TreeCfg`, `InputCfg` | `cfg.ID` |
  | `TableCfg` | `cfg.ID`, or `cfg.ID + ":scroll"` when `FreezeHeader` |
  | `ComboboxCfg` | `cfg.ID + ".dropdown"` |
  | `CommandPaletteCfg` | `cfg.ID + ":scroll"` |
  | `DataGridCfg` | `cfg.ID + ":scroll"` |

  `DataGridCfg.IDScroll` (an identity override) is deleted, not migrated;
  the key always derives from `cfg.ID + ":scroll"`.
- **BREAKING — Window scroll offset maps**: `Window.ScrollX()` and
  `Window.ScrollY()` now return `*BoundedMap[string, float32]` (was
  `*BoundedMap[uint32, float32]`). All `Window.Scroll*` methods
  (`ScrollHorizontalBy/To/ToPct/Pct`, `ScrollVerticalBy/To/ToPct/Pct`)
  and `FindLayoutByScrollID` (renamed from `FindLayoutByIDScroll`) take a
  `string` id.
- **BREAKING — Scrollbar/command-palette cfgs**: `ScrollbarCfg.IDScroll
  uint32` → `ScrollID string` (points at the target container's scroll
  key). `CommandPaletteShow`/`CommandPaletteToggle` drop the `idScroll`
  parameter; Show always resets the results scroll (keyed
  `id + ":scroll"`) to the top.
- **Scroll internals**: the scroll-offset maps are rekeyed uint32→string,
  which sidesteps the `BoundedMap[uint32]` generic lookup penalty
  ([#77](https://github.com/go-gui-org/go-gui/issues/77)); FnvSum32
  scroll-hash derivation removed from Select, Combobox, DataGrid and the
  theme picker. `Shape` shrinks 272 → 264 bytes (this change −8 with the
  `IDScrollContainer` removal below).

### Removed

- Dead `Shape.IDScrollContainer uint32` field and its per-frame
  whole-tree `layoutScrollContainers` pass (zero readers).

### Added

- `BenchmarkViewFrame` gates `sizeof(Shape)` regressions by allocating
  Shapes inside the hot loop (added to the `bench-gate` target).

## [v0.34.0] - 2026-07-14

### Changed

- **BREAKING — Focus API**: `Shape.IDFocus uint32` is replaced by
  `Focusable bool` plus string focus identity (the widget's `ID`). Tab
  order now follows layout-tree (DFS) order instead of ascending numeric
  IDs. Window API: `SetFocus(id string)`, `FocusID() string`,
  `IsFocus(id string)`, and `ClearFocus()` replace the uint32 variants.
  Migration: `IDFocus: N` → `Focusable: true` (with a non-empty `ID`);
  `SetIDFocus(0)` → `ClearFocus()`.
- **BREAKING — Widget cfgs**: `MenubarCfg`, `ContextMenuCfg`,
  `CommandPaletteCfg`, and `DataGridCfg` lose `IDFocus`;
  `DialogCfg.IDFocus` → `FocusID string`; `RadioButtonGroupCfg` gains
  `ID`; menus and context menus now require an `ID`.
  `CommandPaletteShow`/`CommandPaletteToggle` drop the `idFocus`
  parameter (focus derives from the palette input's ID).
- **Focus internals**: six per-window state namespaces rekeyed
  uint32→string; FnvSum32 focus-hash derivation removed from menus and
  datagrid (header/editor focus ids now derive from cell ID and column
  index). Duplicate focusable IDs collapse to one tab stop with a
  dev-mode warning (`GOGUI_FOCUS_DEBUG=1`). `IDScroll` is unchanged.

## [v0.33.1] - 2026-07-14

### Fixed

- **Dependencies**: go-glyph bumped to v1.16.2 — text symbols across many
  Unicode blocks (heavy asterisk U+2731 ✱, mahjong tiles, playing cards,
  alchemical and chess symbols, Supplemental Arrows-C, ~760 codepoints) no
  longer render as the base font's `.notdef` tofu box, and default-text
  emoji such as U+2733 ✳, ❄, ❤, ☀ now render as monochrome text glyphs
  instead of being forced to color, matching Core Text and Ghostty. Also
  propagates InlineObject metadata through rich-text layout and applies
  script fallback in `LayoutRichText`.

## [v0.32.1] - 2026-07-12

### Fixed

- **Dependencies**: go-glyph bumped to v1.16.1 — text symbols such as
  U+23F5 ⏵ (the Misc Technical media triangles) no longer render as the
  base font's `.notdef` tofu box; they now fall back to a real glyph
  (STIX), matching Core Text.

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

- **Dependencies**: go-glyph bumped to v1.15.0 — pure-Go text backends on
  Linux, Android, macOS, and Windows (`go-text/typesetting` +
  `x/image/vector`, `CGO_ENABLED=0`), replacing the cgo FreeType+HarfBuzz
  stack. Pulls in `go-text/typesetting` and `golang.org/x/image` as indirect
  deps; regenerates `docs/dependencies.md`. Drops the obsolete Android
  native-deps build step from CI/release workflows.
- **Markdown defaults**: paragraph `LineSpacing` reduced to 3 and
  `BlockSpacing` raised to 12 so inter-block gaps stay larger than
  intra-line gaps (fixes cramped spacing between wrapped list items).

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
  substitution now falls back to Consolas; internal draw/renderer dedup
  between the FreeType and Darwin backends). Regenerates
  `docs/dependencies.md` to match.

## [v0.30.0] - 2026-07-08

### Changed

- **Dependencies**: go-glyph bumped to v1.13.0 (FreeType+HarfBuzz replaces
  Pango/SDL2, native GLX+WGL backends, ASCII monospace shaping fast-path on
  Darwin). The prior v2.0.0 tag was retracted — it lacks the `/v2` module
  path suffix required by Go module conventions.

### Added

- **Window vibrancy** (macOS): `Window.SetWindowVibrancy(VibrancyMaterial)`
  places a translucent, blurred native `NSVisualEffectView` backdrop behind
  the window content. Pair with a translucent `WindowCfg.BgColor` (alpha <
  255) to reveal the backdrop; `VibrancyNone` restores an opaque window.
  Implemented on the Metal backend (makes the window and its `CAMetalLayer`
  non-opaque so content composites over the blur); no-op on SDL2, OpenGL,
  web, iOS, and Android (Linux/Windows are out of scope, matching the
  `TermGrid` issue). Built for go-term. See `examples/vibrancy`. (#31)
- **`TermGrid` primitive**: a terminal character-grid widget
  (`TermGrid`/`TermGridCfg`) that draws a fixed-pitch cell buffer in a single
  `RenderTermGrid` command — no per-cell `Layout` node and no per-cell
  `RenderText`. Callers hand over a row-major `[]TermCell` with pre-resolved
  RGBA foreground/background, plus cursor and selection state; the backend
  batches same-background runs into fills and pins glyphs to exact cell
  columns via `DrawLayoutPlaced`. Honors the reverse and underline
  attributes; bold/italic are reserved in `TermAttr` for a follow-up.
  Rendered by the Metal and SDL2 backends (OpenGL out of scope). Built for
  go-term and reusable across siblings. See `examples/termgrid`. (#30)

### Fixed

- **GL backend**: narrow build tags from `!js` to `!js && !darwin` on the
  real implementation files so they don't compile on macOS where
  `platform_other.go` returns nil. Eliminates 50 unused-code lint warnings
  on the default dev platform. macOS uses Metal, not GL.
- **Native dialogs**: track native (OS) modal visibility so `DialogIsVisible`
  and the quit/close dedup see `NSAlert`-style dialogs too. Previously a
  native confirm-before-quit could stack a duplicate dialog because the
  second quit/close re-invoked `OnCloseRequest`. `DispatchCloseRequest` now
  guards its hook path while a dialog is showing (the no-hook path still
  closes). (#18)
- **Modal dialogs**: retain keyboard focus inside an in-app modal dialog when
  a focus-claiming widget (one that re-asserts `SetIDFocus` every view
  rebuild) tries to steal it, so Tab/Esc/Enter keep working. Apps no longer
  need to guard their own `SetIDFocus` with `DialogIsVisible`. (#18)

## [Unreleased]

## [v0.29.0] - 2026-06-28

### Added

- **`TextStyle.EmojiBoxWidth`**: optional target cell-box width (logical px),
  threaded through `ToGlyphStyle` and the GPU backend draw path
  (`GuiStyleToGlyphConfig`). When > 0, go-glyph scales color/emoji glyphs to
  fill the caller's cell box instead of the font's natural emoji advance. Used
  by grid callers such as go-term. Requires go-glyph v1.12.0.

## [v0.28.2] - 2026-06-26

### Fixed

- **macOS Metal backend**: complete the Launch Services launch handshake so
  a `.app` bundle launched from Finder is fully registered as a foreground
  app — fixes absence from Cmd+Tab, gray titlebar buttons, and the
  double-click-to-close behavior. Restore `activateIgnoringOtherApps:` for
  the CLI-launch case so bare-exec windows come up active.
- **macOS Metal backend**: fire `EventFocused` on
  `applicationDidBecomeActive:` so keyboard and left-click input are
  restored after a system dialog (e.g. TCC permissions) is dismissed, and
  re-key the frontmost window when `keyWindow` is nil on app switch.
- **DockLayout**: enlarge the tab close button (14×14 → 18×18) with a larger
  × glyph, and add a spacer between the tab label and close button.

### Changed

- **macOS Metal backend**: cache NSCursor selector C strings once at startup
  to drop per-frame `C.CString` alloc/free from the cursor-update hot path.
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
  management, event handling, cursor support, and menu integration via
  AppKit. Replaces the SDL2 backend on macOS for proper platform
  behavior.
- **DockLayout**: `HideSingleTab` option hides the tab bar when only one
  tab is present.

### Changed

- **Performance**: content dimensions and sibling sums cached during
  fill pass to avoid redundant recalculation during layout.

### Fixed

- **DockLayout**: close button rendered as × instead of an empty box.
- **WASM backend**: hardened against iPadOS Safari crashes.
- **SVG**: `arcToCubic` guarded against NaN/Inf radii and float32
  overflow panic.
- **Website**: `status.html` included in deploy output.
- **Windows CI**: MSYS2 pinned to stable release.

## [v0.27.1] - 2026-06-20

### Fixed

- **Windows CI**: pin MSYS2 to stable release to fix `__ms_vsscanf`
  undefined reference linker error with GCC 16.1.0.

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
- **Shared native-platform glue**: `App` native integration tests and
  extracted platform abstraction for backend consistency.

### Changed

- **Performance**: eliminated hot-path heap allocations across layout
  calculation, gesture hit-testing, render command generation, and event
  dispatch. Two-pass allocation scrub.
- **Lock splitting**: animation lock separated from layout lock; `w.mu`
  scope narrowed to reduce contention.
- **GPU backend consolidation**: shared `gpu` package for vertex types and
  draw code reused across Metal, OpenGL, and SDL2 backends.
- **Large-file refactoring**: 14 files over 800 lines split into 32 focused
  files; datagrid dot-imports removed; markdown fetcher uses dependency
  injection.
- **Dependencies**: go-glyph bumped to v1.10.0, golangci-lint to v2.12,
  GitHub Actions to latest major versions.
- **Test parallelization**: tests now run concurrently; per-package coverage
  floors enforced in CI.

### Fixed

- **macOS**: `NSApp` activated before window creation, fixing focus issues
  on launch.
- **DataGrid**: scroll position read from correct state map; `UpdateView`
  no longer clears `idFocus` on full rebuild.
- **SVG**: `arcToCubic` guarded against coincident endpoints (NaN from
  `Inf*0/1`).
- **GPU**: `gpu.Vertex` struct literals use keyed fields across all backends.
- **Web backend**: removed unused `syscall/js` import; restored `strconv`
  import.
- **Build tags**: drift corrected across source files and CHANGELOG.
- **Animation**: map data races fixed under concurrent access.
- **Windows**: `__ms_vsscanf` compat shim for MinGW GCC 15+; static builds
  and DLL alignment hardened; CI smoke test added.
- **Security**: 242 gosec issues resolved; G204 false positives suppressed
  on `exec.Command` calls; privacy audit and resource caps applied.
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
- Godoc improved on core types; subpackage analysis and widget cookbook
  added.

## [v0.26.0] - 2026-06-12

### Breaking

- **DataGrid moved to `gui/datagrid/`** — `DataGrid`, `DataGridCell`,
  `DataGridTheme`, and related symbols (~30) extracted from `gui/` into a
  separate package. Import `github.com/go-gui-org/go-gui/gui/datagrid`
  and use `datagrid.New()` instead of `gui.NewDataGrid()`.
- **SVG constant renames** — `StrokeCap` → `SvgStrokeCap`, `StrokeJoin` →
  `SvgStrokeJoin`, plus typed constants for stroke cap/join, spread method,
  and units. Callers using the old untyped string constants will need to
  update to the new typed values.
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
- Render validators and SVG element handlers extracted from large functions
  into focused helpers.
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
- Web backend keydown/keyup registered on `document` instead of `canvas`,
  fixing focus-edge-case missed keys.
- Showcase wasm build missing `cleanupEmbeddedAssets` stub.
- Showcase audio made opt-in behind `audio` build tag, fixing
  Windows FLAC DLL issue (#8).
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
- Linux release CI (added freetype6/harfbuzz/pango dev packages for go-glyph
  CGo compilation).
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
- `RadioButtonGroupCfg.Items []string` — convenience field; each string
  becomes a `RadioOption` with `Label==Value`.
- `TableCfg.RawData [][]string` — convenience field for CSV-style data.
  First row is treated as the header.
- `TreeCfg.ItemPaths []string` — convenience field for flat path strings
  (`"a/b/c"`), auto-expanded into nested `TreeNodeCfg` nodes with
  duplicate prefix merging.
- `DataGridCfg.RowsData []map[string]string` — convenience field for
  key-value row data. Map keys match column IDs. Columns are
  auto-generated from sorted keys of the first entry when `Columns`
  is empty.

### Changed

- When both the stdlib convenience field and the typed struct field are
  set, the stdlib field takes precedence.

## [v0.22.0] - 2026-06-05

### Added

- Single-binary deploy on Linux and Windows via `go-sdl2 -tags static`,
  eliminating the `libSDL2.so` / `SDL2.dll` runtime dependency.
- Root `Makefile` with `build-linux`, `build-windows`, `build-macos`,
  `build-wasm`, `release`, and `clean` targets.
- `gui.Version` and `gui.Commit` build-time variables injected via
  `-ldflags`.
- CI release workflow (`.github/workflows/release.yml`) triggered on `v*`
  tags and `workflow_dispatch`, building all desktop platforms.

## [v0.21.1] - 2026-05-30

### Fixed

- Hunspell spellcheck is now opt-in via `-tags hunspell` build tag on
  Linux, avoiding a hard runtime dependency on `libhunspell`.

### Changed

- Remove local `replace` directive for `go-glyph` — the module now
  consumes upstream `go-glyph` directly.
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
  backend's DPI scale and included in the `DrawCanvasCache` key, so a canvas
  is re-tessellated when the display scale changes (e.g. window moved between
  Retina and non-Retina monitors).
- All backends (gl, metal, sdl2) now refresh `dpiScale` on window resize, so
  display migration no longer leaves a stale scale for the lifetime of the
  window.
- Web backend now sets `w.BackingScale` each frame; previously `DrawContext.Scale`
  was always 1 on web regardless of `devicePixelRatio`.
- Scale sanitization guard relaxed from `< 1` to `<= 0`, allowing valid sub-1
  device pixel ratios (e.g. browser zoomed below 100%) to pass through.

## [v0.20.0] - 2026-05-18

### Added

- RTF widgets with `IDFocus` set now support interactive text selection:
  click to place cursor, drag to extend selection (with scroll-aware
  auto-scroll), double-click to select word, keyboard navigation
  (arrow keys, Home/End, Ctrl/Cmd+A), and Ctrl/Cmd+C to copy.
- Markdown widgets with `IDFocus` set gain the same selection and copy
  capability across all block types (paragraphs, headings, lists,
  blockquotes, definition terms/values). Selection uses a unified
  rune-offset model so Cmd+C copies the correct cross-block span.

## [v0.19.1] - 2026-05-17

### Added

- Metal backend: scroll phase bridge (`scroll_phase_darwin`) fires
  `EventScrollBegan` when a momentum scroll starts on macOS.

### Fixed

- Context menu: focus is now restored on dismiss; a second right-click no
  longer clobbers the saved focus state.

## [v0.19.0] - 2026-05-16

### Added

- Animation heartbeat is now view-bound: orphaned animations (whose owning
  layout is no longer present in the view tree) are automatically cancelled,
  preventing runaway tickers in long-lived windows.

### Fixed

- Metal backend: per-frame autorelease pool now spans the full frame
  (`metalBeginFrame` → `metalEndFrame`). Command buffers, render pass
  descriptors, encoders, and one-off `MTLBuffer` allocations were
  accumulating in the thread's ambient pool indefinitely (Go threads have
  no runloop). Uses `objc_autoreleasePoolPush`/`Pop` (ARC-compatible).

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

- `AppFontPaths` registry lets apps declare custom font search paths
  before window creation. SDL2/Metal/GL backends load the registry
  at backend init so glyph rasterization picks up app-bundled fonts
  without ad-hoc backend hooks.

### Changed

- Bumped `github.com/go-gui-org/go-glyph` to `v1.7.0`.

### Fixed

- Inline math (markdown RTF render) now uses the per-`InlineObject`
  Height/Offset when available, preserving true aspect ratio for
  tall constructs like fractions and integrals. Previously height
  was clamped to line-height (ascent+descent), squashing oversize
  glyphs. Legacy entries without `Object` keep the old line-height
  fallback.

## [v0.16.0] - 2026-04-28

### Added

- `<use>` referencing `<symbol viewBox=...>` now honors
  `preserveAspectRatio`. Default is `xMidYMid meet` (uniform scale +
  center) per SVG 1.1; `preserveAspectRatio="none"` opts back into
  legacy independent-axis stretch; `slice` uses uniform max-scale
  and now mints a synthesized `clipPath` covering the `<use>` box
  so overflow from max-scale is cropped per spec. Author
  `clip-path=` on the `<use>` itself wins over the synth clip.
  Earlier impl always stretched and never clipped slice overflow.
- `clip-path` and `filter` now participate in the cascade: CSS rules
  (`<style>.cls { clip-path: url(#cp) }`) and inline `style=""`
  declarations set them, not only the bare presentation attribute.
  `clip-path: none` / `filter: none` clear inherited values per
  cascade origin precedence.
- Distinct elements sharing one `filter="url(#X)"` now composite as
  separate offscreen groups in document order. Previously they were
  merged by FilterID and z-ordered against unfiltered siblings
  incorrectly. Each occurrence carries a per-element `FilterGroupKey`
  (parser counter assigned during cascade).
- Nested `<svg>` elements now establish a child viewport. `x`, `y`,
  `width`, `height` accept user-space units or percentages of the
  parent viewport; an inner `viewBox` (with `preserveAspectRatio`
  meet/slice/none) composes onto the cascaded transform from the
  element's own `transform=` attr. Descendants inherit paint and
  cascade through the wrapper. Previously the inner subtree was
  dropped silently.
- Nested `<svg>` viewports now synthesize a rectangle clip-path in
  outer-parent coordinates so descendants outside the authored
  viewport rect are masked at tessellation time (default
  `overflow:hidden` for `<svg>`). Sibling viewports mint distinct
  ids; doubly-nested viewports cascade the innermost clip onto
  descendants. `<clipPath>`, `<linearGradient>`, and `<filter>` defs
  inside a nested `<svg>` reach the global registry. Empty or
  zero-area viewports skip emission. When the inner `<svg>` carries
  an authored `clip-path` (presentation attr / CSS / inline style),
  the synth viewport clip is suppressed so the asset's explicit
  semantic survives — true intersection of viewport and author
  clip awaits a multi-clip renderer.
- `gui.PreserveAlignFractions` exported (was `preserveAlignFractions`)
  so `gui/svg` can resolve `preserveAspectRatio` align fractions
  without duplicating the switch.

### Hardened

- SMIL `from`/`to`/`by`/`values` reject malformed tokens (NaN, Inf,
  garbage) instead of coercing to 0. Bogus 0 endpoints would
  previously synthesize real animation timelines; now the timeline
  drops. Color keyframe stops with invalid paint also drop the
  whole color timeline.
- `<use>` `symbolViewportScale` rejects degenerate viewBox
  (`<= 0` width/height) and clamps combined translate (`tx+ax`,
  `ty+ay`) via `boundedScale` so alignment offsets cannot push the
  transform past `±maxCoordinate`.
- Nested-`<svg>` viewport math sanitizes NaN, ±Inf, and oversized
  inputs on `x`/`y`/`width`/`height`, `viewBox`, `parent.W`/`parent.H`,
  and the resulting scale/translate so a poisoned attribute cannot
  propagate non-finite values into the path transform. Percentages
  parse via float64 so `1e30%` no longer truncates to ±Inf before
  scaling.
- `mixOptsHash` clamps `HoveredElementID` / `FocusedElementID` via
  `clampElementID` (256-byte cap) before the FNV mix. Hostile callers
  passing megabyte-sized pseudo-state IDs can no longer burn CPU in
  the cache lookup hash phase; downstream `parseSvgWith` already
  clamped, so cache key and parsed state stay in sync.
- Inline-SVG cache `sourceKey` now hashes the source via SHA-256
  instead of retaining the raw string. With the 4 MB parse cap and
  512 cache slots, the prior format pinned up to 2 GB of source
  retention from cache keys alone. Hashing is incremental (no
  `[]byte(data)` copy) and produces a fixed 71-byte key.
- `mintUseSliceClipID` (slice `<use>` clip emitter) rejects
  non-finite or out-of-range `viewBox` numbers explicitly before
  falling back to viewBox dims for missing `<use>` width/height; a
  hostile `viewBox="0 0 NaN Inf"` can no longer survive `<= 0`
  coercion and propagate into the clip rect.
- Synthesized slice-clip ids (`__use_clip_N`) now skip any id
  already present in the document index, so an authored
  `id="__use_clip_1"` cannot silently shadow or be shadowed by the
  synthesized rect. Synth ids remain monotonic so two synth ids
  never collide with each other.
- `clampCycle` rejects NaN explicitly (NaN compares false against
  both `<= 0` and `> maxCycleSec`, so it would otherwise fall through
  unchanged into downstream cycle/floor math). `parseTimeValue`
  layers `finiteF32` so `dur="NaN"` / `dur="1e9999s"` cannot reach
  cycle math even if `parseF32` ever loosens.
- `parseKeySplinesIfSpline` switches to `parseFloatStrict` and
  rejects control points outside `[0, 1]`. `parseF32` would coerce
  `NaN`/`Inf` tokens to 0 and slip past the range check, silently
  producing wrong easing on hostile authoring.
- `parseAnimateDashArrayElement` defers `flat` allocation until
  stride is known from the first frame, sizing it to
  `len(frames) × stride` instead of `len(frames) × SvgAnimDashArrayCap`
  (4× less waste for the common stride=2 case).
- `Parser.animatedScratch` pool is now bounded by
  `maxAnimatedScratchCap` (4096) on both ends. `putAnimatedScratch`
  rejects oversized buffers so one pathological frame cannot pin a
  giant backing array; `getAnimatedScratch` clamps `minCap` so a
  hostile SVG with synthesized millions of animated paths cannot
  force a giant `make` per frame.
- `findAttr` reverses the five entity escapes emitted by
  `buildOpenTag` (`&amp;`, `&lt;`, `&gt;`, `&quot;`, `&#39;`) before
  returning attribute values. `encoding/xml` decodes attribute
  entities once, so without this reversal a legitimate `&` in an
  attribute round-tripped as the literal sequence `&amp;` and
  reached downstream parsers (color, url, id, transform) as
  garbage. Unknown entities pass through unchanged; allocation
  occurs only when at least one `&` is present.
- `decodeSvgTree` now accumulates per-node CharData into a parallel
  stack of `strings.Builder` frames instead of `top.Text += s` /
  `last.Tail += s`. A hostile `<text>` body fragmented into many
  small chunks (e.g. by sprinkling numeric entity refs) was O(N²)
  under the prior incremental concat; the builder rewrite is linear.
  Tail accumulation flushes to `Children[idx].Tail` before the next
  sibling append so a slice grow cannot strand pending tail data.

### Fixed

- `<text>` now routes through the CSS cascade like shapes, so author
  rules (`text { fill: ... }`), `:hover` / `:focus` matches, and
  `display:none` apply. Previously `<text>` only saw inherited
  computed style with no per-element rule matching.
- Invalid color syntax (e.g. `fill="#GGGGGG"`, `fill="rgb(abc,def,ghi)"`,
  `stroke=""`) is now ignored by the cascade per CSS
  "invalid → ignore", letting inherited paint survive instead of
  clobbering with transparent black. `parseHexColor` rejects
  non-hex digits; `parseRGBColor` rejects non-numeric channels.
- CSS-wide control keywords (`inherit`, `unset`, `revert`,
  `revert-layer`) on `fill` / `stroke` are no-ops so the cascade-
  copied parent paint survives. `<text stroke="inherit">` with no
  ancestor stroke now falls back to a visible default rather than
  being silently dropped.
- `<text>` now inherits `stroke` / `stroke-width` from the cascade,
  and `stroke="inherit"` resolves against the cascade rather than
  forcing black. `<text stroke="none">` clears any ancestor stroke.
- `<tspan>` honors its own `stroke`, `stroke-width`, and `opacity`
  attrs instead of silently copying parent values. `opacity="50%"`
  on `<tspan>` now equals 0.5 (matches CSS keyframe parity below).
- Mixed-content `<text>` runs preserve trailing and interleaved char
  data. `<text>A <tspan>B</tspan> C</text>` now renders all three
  runs; previously the trailing "C" was dropped because only
  pre-first-child `Leading` text was captured. New `xmlNode.Tail`
  field stashes post-child char data so `<use>`-cloned subtrees
  carry it through too.
- CSS `@keyframes { opacity: 50% }` now compiles to 0.5 (was 1.0).
  `compileOpacityTimeline` switched from `parseFloatTrimmed` to
  `parseOpacityNumber` so the static cascade and animated values
  agree on percentage notation.
- `Parser.InvalidateSvgSource` now correctly drops file-backed cache
  entries and every option-variant (FlatnessTolerance,
  HoveredElementID, FocusedElementID, PrefersReducedMotion). Prior
  impl reconstructed hashes from the path string alone, which never
  matched file entries (whose key mixes file contents) and only
  covered two of the option permutations. Walks the entry table by a
  stored `sourceKey` instead.
- `<use>` cloned subtrees no longer leak duplicate descendant ids.
  `stripID` is now recursive; previously only the clone root and (for
  `<symbol>` targets) its top-level children had their ids removed,
  so any nested id collided with the original and corrupted
  `url(#id)` resolution, CSS `#id` matching, and animation targeting.
- `<use width=W height=H>` of a `<symbol viewBox=...>` now scales the
  symbol's viewport to fill the requested box via a composed
  `translate · scale · translate(-vbX,-vbY)` transform. Width/height
  were previously dropped, so callers could not size symbol reuses.
- `clip-path` / `filter` declarations are now marked authored only
  after the value resolves to a usable `url(#id)` reference or the
  `none` keyword. Previously the cascade flipped the authored flag
  on property name alone, so `clip-path: bogus` could suppress the
  synthesized nested-`<svg>` viewport clip and `filter: bogus` could
  allocate a fresh per-occurrence offscreen group buffer for a
  declaration that contributed no actual filter.
- Markdown inline math (`$...$`) now renders after the async
  codecogs fetch completes. The cross-frame RTF layout cache key
  did not include diagram cache state, so the layout shaped on the
  first frame with the raw-LaTeX text fallback (cache=Loading) was
  reused after the fetch transitioned to Ready — the InlineObject
  placeholder was never emitted and `renderRtf` produced no
  `RenderImage`. New `rtfMathStateKey` mixes per-math-run
  State/Width/Height/DPI into the cache key so a Loading→Ready
  transition forces re-shape. Display math (`$$...$$`) was
  unaffected because it renders through the `Image` view, not RTF.
  FNV-1a constants in `view_rtf.go` extracted to package consts
  (`fnvOffset64`, `fnvPrime64`, `fnvFieldSep`).

### Security

- `<use x="…" y="…">` author values are parsed numerically instead of
  spliced into the synthesized transform attribute, closing an
  injection vector (`x="0)scale(99)"` previously emitted an extra
  `scale` into the transform list). Also rejects percentage `x`/`y`
  rather than treating "50%" as raw 50, and clamps `<use>`-vs-
  -viewBox scale to ±maxCoordinate to prevent pathological tiny
  viewBox dims from emitting absurd scale factors.
- `stroke-width` on `<text>` and `<tspan>` clamps NaN and negative
  values to 0 via new `sanitizeStrokeWidth`. Negative widths are
  invalid per SVG spec; NaN propagation broke tessellation
  (uint8/uint16 casts implementation-defined, Inf coords break
  bbox math).
- `writeAttrEscaped` (used to reconstruct each element's `OpenTag`
  for substring-scanning helpers like `findAttr` /
  `findStyleProperty`) now also escapes `'` (`&#39;`) and `>`
  (`&gt;`). A hostile attribute value containing a single quote
  could previously smuggle a fake attribute past the cascade
  (`<rect note=" x='99' " x="1"/>` parsed as `x=99`). Both quote
  styles plus `<`/`>`/`&` are now escaped so no value can terminate
  the embedded attr or open a markup token.
- `parseSvg(string)` and `parseSvgDimensions(string)` now enforce
  the existing 4 MB `maxSvgFileSize` cap. The cap was previously
  applied only to file-loaded content; callers passing arbitrarily
  large in-memory strings (e.g. network-fetched SVGs) bypassed it,
  letting unbounded `xml.CharData` accumulation and full-document
  scans run on hostile input. `parseSvg` returns an error;
  `parseSvgDimensions` truncates to the cap before probing.
- `clipPath` triangulation is now cached per `ClipPathID` for the
  duration of one `tessellatePaths` call. N paths sharing one
  complex `clipPath` previously triggered N full re-tessellations
  (`O(N · clipComplexity)` CPU DoS); the cache reduces this to one
  tessellation per unique id. Cache is `nil` when the graphic
  declares no `clipPath`s, so the common icon/spinner path takes
  no extra allocation.

## [v0.15.0] - 2026-04-27

### Added

- `<use href="#id">` (and `xlink:href`) resolution. The referenced
  subtree is cloned at parse time, wrapped in a synthesized `<g>`
  carrying a `translate(x,y)` transform plus the `<use>`
  presentation attrs (`fill`, `style`, `class`, ...). Cycles are
  guarded by a visited-set + depth-8 cap; the clone has its `id`
  stripped to avoid duplicate ids in the post-expansion tree.
- `<symbol>` is now honored as a `<use>` target — the symbol's
  children are inlined directly (the wrapper is dropped). Untargeted
  `<symbol>` elements continue to render no output. Symbol-level
  `viewBox` / `preserveAspectRatio` honoring is a future polish.
- `spreadMethod` on `<linearGradient>` and `<radialGradient>`:
  `pad` (default), `reflect` (triangle wave), `repeat` (sawtooth).
  `gui.SvgGradientDef.SpreadMethod` is the new field; the previous
  silent-pad behavior is the zero-value default so existing
  fingerprints stay stable.
- `gui.SvgCfg.FlatnessTolerance float32` — tessellation tolerance
  floor in viewBox units. Default 0 keeps the historic 0.15 floor.
  Plumbed via a new `SvgParseOpts.FlatnessTolerance` field and a
  `Window.LoadSvgWithOpts` method; the cache key tracks tolerance
  per quantized 1e-4 step.
- `gui.SvgCfg.HoveredElementID` / `FocusedElementID string` — drive
  CSS `:hover` / `:focus` matching for the SVG element with that id.
  Plumbed through `SvgParseOpts` into the cascade `MatchState`;
  cache invalidates per id transition.
- `examples/svg_use_symbol`, `examples/svg_gradient_spread`,
  `examples/svg_flatness`, `examples/svg_css_states`.

### Changed

- `gui.SvgGradientDef` gains a `SpreadMethod SvgGradientSpread`
  field. Keyed struct literals are unaffected; positional users in
  sibling repos must update.
- `gui.SvgParseOpts` gains `FlatnessTolerance float32`,
  `HoveredElementID string`, `FocusedElementID string`. Additive.
- `gui/svg.ParseOptions` mirrors the same additions.
- `gui/svg.VectorGraphic` gains `FlatnessTolerance float32`. Internal.
- `Window.LoadSvgWithOpts(src, w, h, opts SvgParseOpts)` is the new
  per-render-override entry point. `Window.LoadSvg` is unchanged.

### Deferred to v0.16.0

- Automatic mouse-driven hover detection on the `Svg` widget.
  v0.15.0 ships the parser/cascade/cache plumbing so apps can
  drive `HoveredElementID` themselves (e.g. by hit-testing
  `TessellatedPath.ContainsPoint`); built-in pointer tracking with
  internal hit-test on the widget will land in v0.16.0.
- `<symbol>` `viewBox` / `preserveAspectRatio` honoring.
- `spreadMethod`-aware stop-boundary subdivision (currently
  pad-clamped, so reflect/repeat AA at wrap points is slightly
  softer than at first/last stop).

## [v0.14.0] - 2026-04-26

### Added

- CSS sibling combinators: adjacent (`+`) and general sibling (`~`).
  Match engine (`gui/svg/css`) now takes a preceding-siblings slice
  alongside ancestors when resolving complex selectors.
- CSS attribute selectors: `[name]`, `[name=v]`, `[name~=v]`,
  `[name|=v]`, `[name^=v]`, `[name$=v]`, `[name*=v]`. Names are
  case-insensitive; values are case-sensitive (no `i`/`s` flag).
  `ElementInfo.Attrs map[string]string` carries the per-element
  attribute map; svg parser populates it from the raw open tag.
- CSS `:hover`, `:focus`, `:not(inner)` selectors — parser + matcher
  only. `Compound` gained `HoverPseudo`, `FocusPseudo`, `Not`
  fields; `ElementInfo` gained a `MatchState{Hover, Focus bool}`
  block. Build-time state can be set via `ElementInfo.State`;
  runtime mouse-event auto-toggle is deferred to v0.15.0.
- `:not()` is single-compound only — comma-list (`:not(.a, .b)`)
  and nested `:not(:not(...))` are deferred.
- `var(--name, fallback)` resolution. The fallback is itself
  resolved recursively (so `var(--a, var(--b, red))` works);
  recursion bounded at depth 32.
- `calc()` arithmetic: `+ - * /`, parens, units `px` and unitless.
  Mixed-unit operands and divide-by-zero invalidate the declaration
  per spec. Nested `calc()` and `calc()` inside `var()` fallback
  are resolved.
- `examples/svg_css_selectors`, `examples/svg_css_vars` — visual
  demos for the new selector and value-resolution machinery.

### Changed

- `css.Match()` and `css.ComplexSelector.Matches()` gained a
  `siblings []ElementInfo` parameter. The sole external caller in
  `gui/svg/style.go` is updated; sibling repos (go-glyph,
  go-charts, go-edit, go-kite) do not call into `gui/svg/css`
  directly. Internal test sites pass `nil` for the new param.
- `Compound`, `ElementInfo`, `MatchedDecl` gained additive fields.
  Keyed struct literals are unaffected.
- `gui/svg.makeElementInfo()` signature gained an `attrs
  map[string]string` parameter (the parsed open-tag attributes).
- The CSS package status table in `docs/svg-support.md` flips
  several rows from "No" to "Yes" (sibling combinators, attribute
  selectors, `:not()`, `var()` fallback, `calc()`).

### Deferred to v0.15.0

- `:hover` / `:focus` runtime mouse-event auto-toggle. The selector
  is recognized today; v0.15.0 will wire the dispatcher (sits at
  the `gui` ↔ `gui/svg` ↔ backend interface boundary, lands cleanly
  alongside `<use>`/`<symbol>` dynamic-cascade work).
- `examples/svg_css_states` — depends on the runtime auto-toggle.

## [v0.13.0] - unreleased

### Added

- SVG accessibility metadata. `<title>`, `<desc>`, `aria-label`,
  `aria-roledescription`, and `aria-hidden` on the root `<svg>` are
  now parsed and exposed via `SvgParsed.A11y` (new `SvgA11y` nested
  struct). Previously dropped silently.
- `<radialGradient>` is now parsed and rendered. Supports
  `cx`/`cy`/`r`/`fx`/`fy` in `objectBoundingBox` (default) or
  `userSpaceOnUse`. Stops use the same semantics as linear
  gradients. Focal interpolation uses a simplified
  distance-from-focal model; full SVG cone-focused projection is
  noted as future polish in `docs/svg-support.md`.
- `preserveAspectRatio` is now honored on the root `<svg>`. All 9
  alignment values (`xMin`/`Mid`/`Max` × `YMin`/`Mid`/`Max`) plus
  `meet`/`slice` are supported. The default (`xMidYMid meet`) is
  unchanged from prior behavior, so existing SVGs render
  identically. `none` (non-uniform stretch) currently falls back to
  default — adding non-uniform render support is tracked as polish.
- `(*TessellatedPath).ContainsPoint(px, py)` for hit-testing filled
  SVG paths. `TessellatedPath` now carries a precomputed bbox
  (`MinX`/`MinY`/`MaxX`/`MaxY`) for fast reject. Author base
  transforms are inverted before the barycentric triangle test.
  Stroke contributions are skipped — pass the fill `TessellatedPath`
  for hit-testing.
- `examples/svg_a11y`, `examples/svg_radial`, `examples/svg_aspect`,
  `examples/svg_hittest` — visual demos for each new feature.

### Changed

- `SvgParsed`, `TessellatedPath`, and `CachedSvg` gained additive
  fields. Keyed struct literals are unaffected; positional literals
  would need to be updated (none found in tree or sibling repos —
  go-glyph, go-charts, go-edit, go-kite).

## [v0.12.7] - 2026-04-26

### Fixed

- SVG fingerprint goldens (`TestPhase0SmilSpinnerFingerprint`,
  `TestPhaseGCssSpinnerFingerprint`) failed on Linux/WASM CI because
  amd64 ships an asm `math.Sin`/`math.Cos` while arm64 uses pure-Go
  — ULP-level drift in trig output flipped digest bits versus the
  darwin-generated goldens. `hashTessellated` / `hashAnimations` now
  quantize finite floats to a 1e-3 grid before `Float32bits`, so the
  fingerprints stay platform-stable while still catching real
  geometry regressions. Goldens regenerated.

## [v0.12.6] - 2026-04-25

### Added

- `SvgSpinner` widget for animated SVG loaders. Full SMIL pipeline:
  `animate`, `animateTransform` (rotate/translate/scale), `animateMotion`,
  per-shape animation keying, attribute overrides, spline easing,
  syncbase `begin` timing, dash animations, TRS-sandwich transforms,
  and per-role opacity. CSS pipeline added: cascade, `@keyframes`,
  `@media`, animation shorthand. Ships with 39 spinner assets across
  the SMIL and CSS sets. See `examples/showcase` for the live gallery.
- `TessellateAnimated` plus parse benchmarks for the SVG path/anim
  pipeline.
- Standalone XML tree parser with per-path animation routing.

### Changed

- SVG parser correctness and performance improvements: scanline
  fill-rule, `Z`-then-`M` path parse fix, dead `GroupID` stripped from
  `TessellatedPath`/`CachedSvgPath`, deduped float helpers, hardened
  animation pipeline.
- Ear-clip tessellator capped at 2048 verts to keep CI under timeout.
- README rewritten: accurate why-go-gui section, spinners video,
  formatting fixes, immediate-mode framing toned down.

## [v0.12.5] - 2026-04-18

### Changed

- `Animation.Update` now takes `*AnimationCommands` instead of
  `*[]queuedCommand`. `queuedCommand` was always unexported, which
  made the `Animation` interface effectively impossible for third-
  party packages to implement — they could not name the parameter
  type. `AnimationCommands` wraps the deferred command queue behind
  two public methods:
  - `AppendOnDone(fn func(*Window))` — queues a terminal callback.
  - `AppendOnValue(fn func(float32, *Window), v float32)` — queues
    a per-frame interpolated-value callback.
  All existing first-party animations (`Animate`, `SpringAnimation`,
  `TweenAnimation`, `KeyframeAnimation`, `LayoutTransition`,
  `HeroTransition`, `BlinkCursorAnimation`) updated; callers of the
  stable concrete factories (`NewSpringAnimation`, etc.) see no
  change. Breaking only for downstream code that implemented
  `Animation` directly — impossible to do before this release, so
  no real-world migration.

## [v0.12.4] - 2026-04-18

### Added

- Per-call image fetcher on `DrawContext`. New
  `DrawContext.ImageWithFetcher(..., fetcher ImageFetcher)` and
  matching `DrawCanvasImageEntry.Fetcher` field let each image draw
  override `WindowCfg.ImageFetcher` for its own download. Typical
  use: a map widget pairs each tile layer with its source-specific
  User-Agent (OSM-policy UA for one layer, a WMS-provider UA for
  another) without a shared composite fetcher. Existing
  `DrawContext.Image` is unchanged and still routes through the
  window-level fetcher.
- New exported `ImageFetcher` function type and
  `ResolveImageSrcWithFetcher(w, src, fetcher)` helper. Existing
  `ResolveImageSrc(w, src)` is a thin wrapper that passes `nil`, so
  no caller needs to migrate.

### Notes

- Scope cut: `ImageCfg` (the Image widget) keeps the single-fetcher
  path. Per-widget fetcher override will land when a consumer
  demands it; no speculative API.
- Known limit: downloads are URL-keyed process-wide, so the first
  entry observed for a URL binds the fetcher for that URL's in-
  flight download. Consumers wiring two fetchers to overlapping URL
  namespaces must route by URL prefix themselves.

## [v0.12.3] - 2026-04-17

### Fixed

- `renderDrawCanvas` now emits images before triangle batches and
  text, so `DrawCanvas` consumers that compose tile backgrounds with
  `DrawContext` overlays get the correct z-order. Previously images
  painted on top of every batch/text in the same canvas — invisible
  in unit tests that only inspect `Texts()`/`Batches()` but user-
  visible once a tile-map demo ran in a window

### Changed

- SDL2 / GL / Metal backends now forward high-resolution
  `MouseWheelEvent.PreciseX` / `PreciseY` for smooth-scroll devices
  (trackpad pixel-scroll, Magic Mouse, high-res wheels), falling
  back to integer `X`/`Y` when the precise field is zero or the SDL
  runtime predates 2.0.18. Enables sub-integer scroll deltas in
  consumers that accumulate fractional `ScrollY`

## [v0.12.2] - 2026-04-16

### Added

- Image download pipeline now handles remote URLs for
  `DrawContext.Image`. Shared `ResolveImageSrc(w, src)` resolves
  http/https URLs to local cache paths, schedules background
  downloads when uncached, and returns "" while in flight.
  `gui.Image` and `emitDrawCanvasImages` both route through it so
  DrawCanvas tiles render after the first fetch
- `WindowCfg.ImageFetcher` hook: apps can supply a custom HTTP
  client to set User-Agent, auth headers, or route through a
  shared pool. Default fetcher sends `User-Agent: go-gui/vX.Y.Z`
  so providers (e.g. OSM) can identify traffic
- `WindowCfg.MaxImageDownloads`: process-wide cap on concurrent
  image downloads. Defaults to 6; first-window-wins for sizing
- Exported `Version` const tracks the module tag

### Fixed

- HTTP status codes are now checked before the body is written to
  disk. Non-200 responses (4xx/5xx) no longer poison the cache
  with error-page payloads

### Changed

- `downloadImage` dropped the HEAD pre-flight and validates
  size/content-type on the GET response. Single round trip per
  fetch

### Performance

- `ResolveImageSrc` caches the URL→path mapping per window so
  already-resolved tiles skip the `MkdirAll` + `Stat` syscalls
  each frame. Critical for DrawCanvas-based tile maps that render
  dozens of images per frame at 60fps

## [v0.12.1] - 2026-04-16

### Added

- DrawCanvas: `DrawContext.Image(x, y, w, h, src, bgOpacity, bgColor)`
  draws images inside the canvas via the same deferred-emit pipeline
  as text. `src` accepts the same forms as `ImageCfg.Src` (local path,
  http/https URL, data URL)
- DrawCanvas: `DrawCanvasCfg.IDFocus` and `OnKeyDown` enable keyboard
  focus and key event handling. A11Y role flips to button when the
  canvas is focusable

## [v0.12.0] - 2026-04-15

### Added

- Time-travel debugging: opt-in via WindowCfg.DebugTimeTravel. User state
  implements Snapshotter (Snapshot/Restore; optional Size). Framework
  captures a snapshot after every dispatched event; scrubber window
  auto-spawns alongside the app window with a slider, step buttons
  (first/prev/next/last), cause label, counter, freeze toggle, and
  keyboard shortcuts (arrows, home/end, space, esc)
- Window.Now() virtual clock: returns pinned snapshot timestamp during
  scrub, live time otherwise; use in view fns that render clock-driven
  data so scrubbed frames match their snapshot
- Window.EnableHistory(maxBytes), HistoryLen(), OpenDebugWindow(),
  Freeze/Resume/IsFrozen, PostRestore(idx) public API
- RegisterNamespaceSnapshot(ns): widget authors opt additional StateMap
  namespaces into scrub restore; scroll (nsScrollX/nsScrollY) and
  widget-local focus (nsInputFocus, nsListBoxFocus, nsTreeFocus) are
  pre-registered
- BoundedMap.cloneAny/restoreAny: type-preserving snapshot through an
  interface so whitelisted namespaces rewind without reflection
- examples/time_travel: counter demo wiring Snapshotter + DebugTimeTravel

### Hardening

- Snapshotter.Size() capped at 1 GiB to prevent totalBytes overflow
- Slider NaN/Inf rejected before int conversion in the scrubber
- BoundedMap restore recovers from type-assertion panics so a single
  out-of-sync namespace doesn't break the rest of the scrub
- Parent-window title truncated before composing the scrubber title

### Notes

- Read-only scrub only: rewinding state does not un-do past side effects
  (HTTP requests, file writes, sounds)
- Requires multi-window mode (App + App.OpenWindow). Single-window apps
  log a notice and no-op
- Zero-cost when disabled: nil-history check short-circuits the hot
  path with no allocation

## [v0.11.0] - 2026-04-14

### Added

- WindowCfg.OnCloseRequest hook: intercept OS window-close and app-quit
  events for save/discard/cancel prompts. Callback owns calling
  Window.Close() to proceed or doing nothing to cancel. Dispatch
  extracted into DispatchCloseRequest / DispatchQuitRequest helpers
  shared by sdl2/gl/metal backends.

## [v0.10.0] - 2026-04-14

### Added

- DockNode/SplitterState JSON serialization: struct tags, text-marshaled
  enums (DockNodeKind, DockSplitDir, SplitterOrientation, SplitterCollapsed),
  DockNodeSanitize for post-unmarshal hardening
- Showcase docs: new dock_layout component entry, splitter serialization section

### Changed

- SplitterStateNormalize handles NaN/Inf ratios and invalid Collapsed values
- Modernize: sync.OnceFunc/OnceValue, slices.SortStableFunc, cmp.Compare

## [v0.9.9] - 2026-04-13

### Fixed

- Metal backend: native Cocoa file-drop bridge bypasses go-sdl2 crash
  (SDL_free on Cocoa-allocated string); per-window callback map for
  multi-window support

## [v0.9.8] - 2026-04-13

### Added

- File-drop event support: OnFileDrop callback on Container, DrawCanvas,
  and EventHandlers; SDL2 backend maps DropEvent to EventFileDropped

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
- Replace `fmt.Sprintf` with `strconv` in hot paths (data grid, inspector,
  a11y, data source)
- Eliminate per-frame heap allocations: gesture Event scratch pool, defer
  removal in render/image opacity, rotateCoordsInverse float path,
  stack-array cellContent, inspector cache map reuse
- Convert copy-paste spinner tests to table-driven
- Remove redundant state and unnecessary comments
- 42 files changed, −278 net lines

## [v0.9.5] - 2026-04-11

### Added

- `Window.FrameCount() uint64` accessor for the monotonic frame
  counter; lets widgets detect repeat callbacks within a render cycle

## [v0.9.4] - 2026-04-11

### Added

- `Window.SetTitle(string)` + `Window.SetTitleFn(func(string))` — dynamic
  OS window title updates. Wired in sdl2, metal, and gl backends via
  `sdl.Window.SetTitle`
- Input hardening on `SetTitle`: 4 KiB cap, UTF-8-safe truncation,
  embedded-NUL stripping; no-alloc fast path for clean short inputs

## [v0.9.3] - 2026-04-10

### Added

- `NativeSaveDiscardDialog` — Save / Don't Save / Cancel alert for
  unsaved-changes flows
- Native menubar: route macOS app-menu "About" through `OnAction`

### Changed

- License: PolyForm NC 1.0 → MIT

### Fixed

- Solitaire example: replace double-click auto-move with right-click
- CI: brew upgrade harfbuzz/pango text stack on macOS; checkout
  go-glyph for test job and use local replace directive

## [v0.9.2] - 2026-04-09

### Added

- `Window.TextMeasurer()` accessor for downstream widgets that need
  direct access to the backend measurer

### Fixed

- Drop `t.Parallel` on tests mutating `guiTheme.ScrollMultiplier`
  (race-avoidance)

## [v0.9.1] - 2026-04-08

### Changed

- Bump `github.com/go-gui-org/go-glyph` to v1.6.4
- Bump `golang.org/x/sys` to v0.43.0

## [v0.9.0] - 2026-04-07

### Added

- `gui/highlight` subpackage: chroma-backed syntax highlighter with curated lexer set (go, python, js/ts, rust, c/cpp, java, ruby, shell, html, css, json, yaml, toml, sql, markdown, diff, dockerfile, make) and DoS caps (256KB source, 100k tokens)
- `MarkdownStyle.CodeHighlighter` field: optional highlighter for fenced code blocks; nil preserves parser's built-in tokenizer
- `MarkdownStyle.CodeTypeColor`, `CodeFunctionColor`, `CodeBuiltinColor` palette fields
- Showcase: component docs, welcome, data grid features, markdown demo, and inspector overlay all use `highlight.Default()`

## [v0.8.0] - 2026-04-06

### Added

- `Spinner` widget: animated mathematical curve loading indicator with 21 named `CurveType` constants (rose, lissajous, hypotrochoid, butterfly, cardioid, lemniscate, epitrochoid, heart wave, spiral, fourier and variants)
- Spinner particle-trail rendering via `DrawCanvas` with faint ghost path outline
- Spinner optional slow rotation (`Rotate` field, 30s per revolution)
- Spinner `Opt[float32]` params (ParamA/B/D) for custom curve tuning
- DrawContext: `QuadBezier`, `CubicBezier` drawing primitives
- DrawCanvas: `OnMouseUp` event
- `ClearNamespace` and `ClearDrawCanvasCache` for targeted cache flush
- Mouse button state in motion events; `OnMouseMove` on `DrawCanvas`
- `OnMouseLeave` event and `RequestRedraw()` for tooltip support
- Showcase: Spinner demo with all 21 curves, varied colors, and rotation examples

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

- `GridPaginationCursor`/`GridPaginationOffset` iota values shifted; new `GridPaginationNone` (0) added
- `Color.Over` returns `ColorTransparent` (set=true) instead of zero `Color` when both inputs are fully transparent
- `executeFocusCallback`/`executeMouseCallback` removed unused debug string parameter

### Fixed

- Race: synchronize `guiTheme` and `Default*Style` globals with `sync.RWMutex`
- Race: `App.Broadcast` no longer holds lock during user callback (deadlock)
- Race: metal a11y buffers protected with mutex
- Race: SDL2 resize event watcher allocates per-callback instead of sharing pointer
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
- Correctness: `render_validate` checks NaN/Inf/nil for gradient, shadow, blur, shader, rotate
- Correctness: `WithColors` borderFocus falls back to theme-level `ColorSelect`
- Correctness: `WithColors` updates SkeletonStyle and InspectorStyle
- Correctness: `AdjustFontSize` clamps each sub-size individually
- Correctness: `SetTheme` syncs `DefaultInspectorStyle`
- Correctness: `ColorFilterCompose` nil-checks inputs
- Correctness: scroll handlers set `IsHandled` and use shape-relative coords
- Correctness: gesture emits rotate `Began` before first `Changed`
- Correctness: `InMemoryDataSource.Capabilities` acquires read lock
- Correctness: `effectivePaginationKind` returns `GridPaginationNone` when unsupported
- Correctness: dock tree nil Root guard
- Correctness: `bounded_map` eviction handles tombstone-only runs
- Fix: variable shadowing in gesture, data_source, data_source_orm, locale_bundle, view_listbox
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
- `contentWidth`/`contentHeight` skip Float and ShapeNone children, matching `spacing()`
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
- Showcase: updated draw canvas demo with line chart (joined polyline, dashed grid, text labels) and bar chart (rounded bars, dashed reference line)
