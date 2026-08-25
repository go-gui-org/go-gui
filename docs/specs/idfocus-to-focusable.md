# Spec: Replace `IDFocus uint32` with `Focusable bool` + string focus identity

Status: in progress (branch `focusable-migration`) Base: `main` @ `7f80679`
Target release: go-gui `v0.34.0` (breaking)

## Motivation

`IDFocus uint32` pulls double duty: it is both the **tab-order value**
(ascending) and the **StateMap key** for per-widget input state (cursor,
selection, undo/redo, IME, spell-check, markdown selection). Changing a widget's
tab order therefore silently changes its state identity — an acknowledged
accidental coupling.

Every stateful widget already requires a non-empty string `ID` (via `RequireID`)
and every widget `Cfg` already carries an `ID string` field. Using that string
as the sole focus identity removes the coupling:

- **Focus identity / StateMap key** = existing `Cfg.ID` (string).
- **Tab participation** = new `Focusable bool` opt-in.
- **Tab order** = layout-tree depth-first (DFS) traversal order.

`IDScroll` (scroll-state key, `uint32`) has no double-duty problem and is
explicitly **out of scope**. It stays `uint32` and keeps its `FnvSum32` string
derivation.

## Decisions (locked)

1. Tab order: layout-tree DFS order. Numeric ordering retired.
2. Window API: `SetFocus(id string)`, `FocusID() string`,
   `IsFocus(id string) bool`, `ClearFocus()`. No public `SetFocus("")`.
   `ClearFocus()` is the documented way to defocus.
3. Opt-in: explicit `Focusable bool`. `Focusable: true` requires a non-empty
   `ID`.

   **Amended 2026-08-07 (developer-ergonomics §4.2).** The runtime guard named
   here was never wired up: `RequireFocusID` shipped with zero call sites in
   go-gui or any sibling repo, so `Focusable: true` without an `ID` rendered and
   clicked but never joined the tab order, silently. The guard is now deleted.
   Enforcement is the `requiredid` analyzer at build time (static case, with the
   `Cfg` type named) plus the `gui.Debug` gate at render time (dynamic
   `Focusable`, or an `ID` expression that evaluates empty).

4. `IDScroll` untouched.
5. Clean break — no back-compat shim (single owner: all consumers are sibling
   repos owned by the same author).
6. Duplicate focusable IDs in one frame: `collectFocusCandidates` dedups to a
   single tab stop and emits a **dev-mode warning** (once per colliding ID per
   frame), guarded by the debug/inspector gate — silent in release.

   **Amended 2026-08-07 (developer-ergonomics §4.1).** The warning moved to the
   `gui.Debug` gate, which walks the composed tree once per frame and so sees
   **every** duplicate ID, not only focusable ones — `ID` also keys scroll
   offsets and per-widget state. It warns once per ID per window rather than
   once per frame.

7. go-term: add optional `term.Cfg.ID string`. When set, it is the focus
   identity. When empty, fall back to the existing generated `"term-"+seq`
   scheme.

## Core model changes

### `Shape` (`gui/shape.go`)

- Remove `IDFocus uint32`.
- Add `Focusable bool`.
- Reuse existing `Shape.ID string` as focus identity.
- `FocusSkip bool` unchanged: click-focusable and holds selection state, but
  excluded from Tab traversal. Text / RTF / Markdown selection widgets set
  `Focusable: true, FocusSkip: true, ID: ...`.

### `Window` / `ViewState`

- `ViewState.idFocus uint32` → `focusID string`.
- `gui/window_focus.go`: `IDFocus()→FocusID() string`,
  `SetIDFocus(uint32)→SetFocus(string)`, add `ClearFocus()`,
  `IsFocus(uint32)→IsFocus(string)`. Blink-cursor / IME gates change `id > 0` →
  `id != ""` (`setFocusLocked`).

### Tab traversal (`gui/layout_query.go`, `gui/window_event.go`)

- `focusCandidate{shape *Shape; id string}`.
- `collectFocusCandidates`: gate `Focusable && !FocusSkip && !Disabled`. Dedup
  by `ID` (dev-mode warn on collision). DFS order preserved.
- Replace numeric `focusFindNext`/`focusFindPrevious` with **positional**
  next/previous: find current `FocusID()` index in the ordered slice, return
  `(i±1)` with wrap. Current not found → first / last.
- `FindLayoutByIDFocus` → `FindLayoutByFocusID(layout, id string)` matching
  `Shape.ID`.
- `handleKeyDownEvent` dialog-layer scoping is unchanged, so Tab stays trapped
  in the dialog subtree (preserves `RetainDialogFocus`).

### Click / scroll focus (`gui/event_handlers.go`)

- Click-to-focus: `Focusable && ID != "" && button != right → SetFocus(ID)`.
- `focusedScrollTarget`: guard `FocusID() == ""`. Use `FindLayoutByFocusID`.

## StateMap key retype (`gui/state_registry.go` + consumers)

Six namespaces flip `uint32` (IDFocus) → `string` key. `nsScrollX` / `nsScrollY`
(IDScroll) stay `uint32`.

| Namespace            | Change                                                                |
| -------------------- | --------------------------------------------------------------------- |
| `nsInput`            | key `uint32`→`string` (widget `ID`)                                   |
| `nsInputFocus`       | key `uint32`→`string`                                                 |
| `nsSpellCheck`       | key `uint32`→`string`                                                 |
| `nsMdSel`            | key `uint32`→`string`                                                 |
| `nsMdBlocks`         | key `uint32`→`string`                                                 |
| `nsMenu`             | key `uint32`→`string`, drop `FnvSum32("menu_"+ID)`, use `ID` directly |
| `nsContextMenuFocus` | **value** `uint32`→`string` (saved prior focus)                       |

Consumers threading `idFocus uint32` → `focusID string`: `input_state.go`,
`view_input*.go`, `view_rtf_select.go`, `render_text.go`, `spell_check.go`,
`markdown_select.go`, `view_menu*.go`, `a11y_tree.go`, `inspector.go`.

## Widget factories (34 Cfgs)

All 34 focusable `Cfg` structs already have an `ID string` field — **no new ID
fields required**. Per Cfg:

- Drop `IDFocus uint32`. Add `Focusable bool`. Focus identity = `ID`.

Special cases:

- Menus (`view_menu.go`, `view_menubar.go`, `view_context_menu.go`): drop
  `FnvSum32` focus derivation. Use `cfg.ID`.
- `RadioButtonGroup`: `idFocus++` → per-child
  `ID = cfg.ID + "/" + strconv.Itoa(i)`.
- `DataGrid`: focus id → `cfg.ID + ":focus"` (string). The `:scroll` IDScroll
  derivation keeps `FnvSum32` (IDScroll untouched).
- Splitter / Slider / TabControl internal `idFocus` plumbing → string.

Cfg structs affected (files): `view_slider`, `view_radio`, `view_button`,
`view_rtf`, `view_radio_button_group`, `view_splitter`, `view_text`,
`view_command_palette`, `view_tab_control`, `view_input` (Input +
MultiLineInput), `view_select`, `view_switch`, `view_combobox`,
`view_breadcrumb`, `inspector`, `view_color_picker`, `termgrid`,
`view_date_picker_roller`, `view_listbox`, `view_overflow_panel`,
`view_container`, `view_markdown`, `view_theme_picker`, `view_input_date`,
`view_input_numeric`, `view_tree`, `view_toggle`, `view_context_menu`,
`view_dialog`, `view_draw_canvas`, `view_menubar`, `view_date_picker`,
`datagrid/view_data_grid`.

## Tests / examples / docs (go-gui)

- Tests: swap numeric IDs → `Focusable: true, ID: "x"` and
  `viewState.focusID = "x"` (`layout_query_test`, `view_widget_test`,
  `view_input_test`, `input_state_test`, `render_layout_test`, `a11y_tree_test`,
  `event_handlers_test`, `spell_check_test`, `termgrid_test`,
  `event_fuzz_test`).
- 37 example files: `IDFocus: N` → `Focusable: true, ID: "..."`.
- 41 `.md` files + `.claude/skills/{widget,new-example}` scaffolds + `shape.go`
  doc comment: update field name and guidance.

## Release

Breaking → minor bump `v0.34.0`, CHANGELOG entry. Merge branch to `main` and tag
before bumping siblings.

## Sibling repos (dependency order)

Each: migrate → bump `go.mod` require →
`go build && go vet && golangci-lint run && go test ./...` → ship.

1. **go-charts** — 1 site (`InputCfg`, iota `focusSearch`). Trivial.
2. **go-kite** — 4 literals (`Input`/`Button`/`Container`). Trivial.
3. **go-map** — `mapview` Cfg `IDFocus`/`IDFocusBase uint32` → string base.
   Legend/gallery derive `ID+index` (build order == old numeric order → tab
   order preserved). Its own string-based overlay-marker focus system is
   independent → unaffected.
4. **go-edit** — `EditorCfg.IDFocus uint32` → `EditorCfg.ID string`. Root
   container `Focusable: true`. `SetIDFocus(focusEditor)` →
   `SetFocus("editor")`. 3 prod + 114 test occurrences (mechanical).
5. **go-term** — `Term.focusID uint32` → `string` (optional `Cfg.ID`, else
   `"term-"+seq`). `FocusID() uint32→string`. 11 runtime
   `SetIDFocus`→`SetFocus`. Workspace pane compares strings.

## Verification gate (every repo)

```
go build ./... && go vet ./... && golangci-lint run ./... && go test ./...
```

## Execution protocol

- Branch `focusable-migration` off `main`.
- Phase 0: this spec (commit, pause).
- Phase 1+2 folded into one green commit (core + widgets), gate, pause.
- Phase 3: tests/examples/docs, full gate, pause.
- Release: merge to main, changelog, tag `v0.34.0`, pause.
- Phase 4: one commit per sibling, gated + shipped, pause after each.

## Open risks

- Tab order changes for any app that relied on numeric order differing from tree
  order. Inventory shows all siblings assign IDs in ascending tree order, so
  tree order matches. Low risk.
- Focus identity must be unique per frame among focusable widgets. Duplicates
  collapse to one tab stop (dev-mode warned).
- Auto-focus-on-launch (`SetIDFocus` in `OnInit`, go-edit/go-term) becomes
  `SetFocus(string)`.
