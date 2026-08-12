# Spec: `Focusable` defaults true for input controls (`FocusDisabled` opt-out)

Status: **implemented** — Phase 1 shipped v0.36.0 (Input, Select, Slider,
Toggle, Switch); Phase 2 shipped v0.37.0, dropping `Focusable bool` for
`FocusDisabled bool` on the remaining nine controls. Both phases in CHANGELOG.

Base: `main` @ `8522098`
Target release: go-gui `v0.36.0` (breaking)

## Motivation

Nearly every interactive `InputCfg` site in-repo sets `Focusable: true`;
the exceptions are not designed non-focusable inputs but the two wrapper
factories (which pass `cfg.Focusable` through) and a couple of tests.
Requiring the field on every input is boilerplate that adds nothing — an
input the user can't tab to is a bug, not a design choice. Flip the
default: input controls are focusable unless the caller opts out with
`FocusDisabled`.

**Phase 2 is an accessibility change, not just deboilerplating.** `Select`,
`Slider`, `Toggle`, `Switch` are mostly *not* focusable in-repo today
(Slider: 0% set `Focusable: true`), so flipping them enrolls those controls
in the Tab order — the intended, correct behavior (a slider should be
keyboard-adjustable), a visible behavior change, not silent cleanup. Zero
`Focusable: false` anywhere (repo + all five siblings) confirms nobody
relies on the opt-out, so the flip is safe.

Scope caveat: focus still requires `s.ID != ""` (`layout_query.go:94`), so
this benefit only reaches **ID-bearing** call sites. `Slider`'s ID is
`gui:"required"` (always present); `Select`/`Toggle`/`Switch` IDs are
optional, so an ID-less one becomes a focusable-but-inert shape (renders,
never a tab stop) — consistent with the whole "no ID → inert" design, not
a regression. The a11y win lands where callers supply an ID; encourage
that in the field docs (Phase 3), do not force it.

This is **only a default flip**, not auto-generated identity. Focus still
requires a non-empty `ID` (`isFocusedTarget`: `Focusable && ID != ""`). A
defaulted-focusable input with no `ID` is simply **inert** — it renders,
but holds no focus, cursor, or state. That graceful degradation is
accepted: no ID → no state → the control doesn't respond. No ID is ever
fabricated, so the identity==state-key invariant (see
[idfocus-to-focusable.md](idfocus-to-focusable.md)) is untouched and no
state can be silently corrupted.

## Why `FocusDisabled bool`, not `Opt[bool]`

`Focusable bool` cannot default true (zero value is false) and can't
distinguish "unset" from "explicitly false." Inverting to `FocusDisabled
bool` gives a zero-value default of _focusable_, a single obvious opt-out,
and — because the `Focusable` field is removed from in-scope Cfgs — turns
every existing `Focusable: true` on those Cfgs into a **compile error**,
which is the migration guide. Preferred over `Opt[bool]`: no wrapper type,
no `.Get()` at every read, loud break at the call site.

## Decisions (locked)

1. In-scope Cfgs drop `Focusable bool`, add `FocusDisabled bool`. Factory
   sets `Shape.Focusable = !cfg.FocusDisabled`.
2. `Shape.Focusable bool` (runtime field) is **unchanged**. Only the
   Cfg-level field flips. Tab traversal, `isFocusedTarget`, and the
   `requiredid` analyzer are all keyed on `Shape`/literals and need no
   model change.
3. No auto-ID. No ID → inert. ID stays **optional** where it is optional
   today (Input) and stays **required** where `gui:"required"` already
   demands it (Slider, Combobox, DatePicker, Listbox — unchanged).
4. `Disabled` still excludes from tab order (`collectFocusCandidates`
   already gates `!Disabled`); `ReadOnly` still keeps the control
   focusable. `FocusDisabled` is orthogonal to both.
5. Out-of-scope widgets keep `Focusable bool` opt-in (Button, Container,
   Text, RTF, Markdown, DrawCanvas, TabControl, Splitter, Breadcrumb,
   OverflowPanel, TermGrid, Inspector, and standalone `Radio` — the
   `RadioButtonGroup` composite is deferred separately).
6. Clean break — no back-compat shim. All consumers are sibling repos
   owned by the same author (matches idfocus-to-focusable decision #5).
7. **Phase 1 and Phase 2 ship together** as a single `v0.36.0` migration
   (one break for consumers, not two).
8. **Composites and Input wrappers deferred** (Combobox, DatePicker,
   Listbox, RadioButtonGroup, **NumericInput, InputDate**) — each either
   governs focus over internal children or has a focus-model defect the
   flip would expose (see [Deferred — why](#deferred--why)). Revisit in a
   follow-up.
9. **Borderline widgets stay opt-in** (ColorPicker, ThemePicker, Tree) —
   Tree is navigation, the pickers are composite.
10. **Analyzer stays silent** on an in-scope input with no `ID`. Inert is
    a valid choice; no positive lint.
11. **In-scope invariant** — a widget qualifies for the flip only if it
    (a) never fabricates a non-empty `ID` from an empty `cfg.ID`, and
    (b) exposes exactly **one** focus candidate (tab stop). Both are
    required so that "no ID → inert" holds and the flip adds no duplicate
    tab stops. Audited per widget below.

## In-scope widget set

Only widgets that satisfy the Decision-11 invariant are in scope. Audit
against the two failure modes (fabricated ID from empty `cfg.ID`; more
than one focus candidate):

| Cfg               | Focus candidates                                   | Fabricates ID? | Verdict |
| ----------------- | -------------------------------------------------- | -------------- | ------- |
| `InputCfg`        | 1 (outer `Column`; inner `Text` is `FocusSkip`, same `ID`) | no             | ✅ in — Phase 1 |
| `SelectCfg`       | 1 (`cfg.ID`; dropdown never focusable)             | no¹            | ✅ in — Phase 2 |
| `SliderCfg`       | 1 (`cfg.ID`)                                        | no             | ✅ in — Phase 2 |
| `ToggleCfg`       | 1 (`cfg.ID`)                                        | no             | ✅ in — Phase 2 |
| `SwitchCfg`       | 1 (`cfg.ID`)                                        | no             | ✅ in — Phase 2 |
| `NumericInputCfg` | **2** (outer Row `cfg.ID` + inner Input `cfg.ID+"_field"`) | no (gated)     | ❌ deferred |
| `InputDateCfg`    | 1, but inner Input `ID = cfgID+".input"` **ungated** | **yes**        | ❌ deferred |
| `ComboboxCfg`, `DatePickerCfg`, `ListBoxCfg`, `RadioButtonGroupCfg` | focus over internal children | — | ❌ deferred (composite) |

¹ `SelectCfg.ID` is optional; empty `ID` yields one *inert* focus target
but its open-state (`nsSelect`) is keyed on `""` and thus shared across
ID-less Selects — a pre-existing quirk, not focus corruption. Passing an
`ID` is recommended in practice; not required for the flip.

**Phase 1:** `InputCfg` (single-line + multiline; the 96% case).
**Phase 2:** `SelectCfg`, `SliderCfg` (`gui:"required"` ID), `ToggleCfg`,
`SwitchCfg`.

### Deferred — why

The two Input **wrappers** each break the Decision-11 invariant and need a
dedicated focus-model fix before they can flip:

- **`InputDate`** — `inputDateTextField` sets the inner Input's `ID` to
  `cfgID + ".input"` unconditionally (`view_input_date.go:233`); with an
  empty `cfg.ID` that is `".input"`, a fabricated non-empty ID. After the
  flip the inner Input becomes a live tab stop under `".input"`, two
  ID-less InputDates collide, and `nsInputDateText`/`nsInputDate` state
  keyed on `""` is shared — the exact corruption the invariant forbids.
  Same for `cfgID + ".picker"` (line 158). It also embeds the deferred
  `DatePicker` composite. Fix later: make `InputDateCfg.ID` required
  (it is stateful), or gate every derived ID on `cfg.ID != ""` the way
  `NumericInput` gates `_field` (`view_input_numeric.go:156`).
- **`NumericInput`** — with step buttons, the outer Row (`cfg.ID`,
  focusable) and the inner Input (`cfg.ID+"_field"`, focusable) are two
  distinct tab stops for one control. Not caused by the flip, but the flip
  makes it the default. A correct fix is non-trivial: the Row also takes
  clicks and calls `SetFocus(cfg.ID)`, while the editable state lives on
  `cfg.ID+"_field"`, so "just `FocusSkip` the Row" focuses the wrong
  shape. Rework so the inner Input is the sole tab stop and clicks focus
  it, then re-scope.

## Core changes

### Per in-scope Cfg

- Remove `Focusable bool`; add `FocusDisabled bool`.
- Factory: every `Focusable: cfg.Focusable` → `!cfg.FocusDisabled`.
  `InputCfg` sets this on both the outer `Column` (tab stop) and the inner
  `Text` (which stays `FocusSkip` under the same `ID` → still one tab
  stop).
- Phase 2 controls each set it on their single focusable shape.

### Call-site migration (mandatory, same commit as the field removal)

Enumerate `InputCfg` construction sites with a command rather than a fixed
count (the count drifts — do not hardcode it):

```
rg -n '\bInputCfg\{' -g '*.go'      # Go literals (compile-relevant)
rg -n 'InputCfg\{' -g '*.md' docs README.md .claude   # docs/snippets
```

**The compiler is not a complete checklist.** Removing the field only
breaks literals that *write* `Focusable:`; a literal that omits it compiles
fine but silently flips to focusable-by-default — a behavior change the
build won't flag. And Markdown snippets never compile at all. So review
**every** `InputCfg` literal the commands list, not just the ones that
error. Each must be rewritten in the same phase so the repo compiles and
the gate passes:

- Literal `Focusable: true` → **delete the line** (now the default; these
  widgets keep an `ID`, so behavior is unchanged).
- `Focusable: <expr>` → `FocusDisabled: !<expr>`.

**Critically, the deferred wrappers and other internal factories that
build an `InputCfg` must translate, not drop, their focus intent** — else
the deferred bug (Decision 11 / [Deferred — why](#deferred--why)) ships
early:

```go
// view_input_date.go, view_input_numeric.go — stays opt-in:
Input(InputCfg{ ..., FocusDisabled: !cfg.Focusable })
```

Internal `gui/` factories to migrate in Phase 1 (all currently pass
`Focusable: true` or `cfg.Focusable`): `view_input_numeric.go`,
`view_input_date.go`, `view_color_picker.go` (×2),
`view_command_palette.go`, `view_dialog.go`, and `datagrid/`
(`data_source_grid.go`, `view_data_grid_edit.go`, `_events.go`,
`_header.go`, `_pager.go`). Internal Inputs that set `Focusable: true`
just drop the line; the two wrappers use `FocusDisabled: !cfg.Focusable`.

### `view_input.go` a11y read-only clause

Currently:

```go
if cfg.ReadOnly || !cfg.Focusable {
    a11yState = AccessStateReadOnly
}
```

Drop the second clause → `if cfg.ReadOnly`. With focusable-by-default,
non-focusable is now an explicit `FocusDisabled` opt-out, not the
historical proxy for read-only; a missing ID must not announce read-only
on a field nobody marked read-only.

### `requiredid` analyzer (`tools/requiredid/`)

No code change expected: its "Focusable: true without ID" rule keys on the
literal field, which no longer exists on in-scope Cfgs (writing it is a
compile error). Verify the rule is generic (not Cfg-type-named) and that
`gui:"required"` tags on Slider/Combobox/DatePicker/Listbox IDs are
retained. An analyzer test can assert no diagnostic on a fake Cfg with
neither field, but note it is low value — `tools/requiredid/testdata`
defines its own fake widgets, so it cannot catch regressions against the
real `gui` types.

## Migration

Each phase is a single compile break for the field it removes and **must
migrate every construction site of that Cfg in the same commit** (internal
factories, examples, and tests) so `make check && go test ./...` stays
green. A phase is not "widget + its own tests" — it is the widget plus its
whole in-repo call graph.

- `Focusable: true` on an in-scope Cfg → delete the line (now default).
- `Focusable: <expr>` (wrappers/internal factories) → `FocusDisabled:
  !<expr>`.
- `Focusable: false` on an in-scope Cfg → `FocusDisabled: true`
  (**zero** occurrences in-repo and across all five consumers).
- Out-of-scope widgets: untouched.

### Consumers (breaking; dependency order)

Re-verify per repo before shipping (grep `Focusable: true` on in-scope
Cfg literals):

1. **go-charts** — 1 `InputCfg` site.
2. **go-kite** — `Input` literals.
3. **go-map** — verify no in-scope literals.
4. **go-edit** — builds `EditorCfg`, not `Input`; expected clean.
5. **go-term** — no `gui.Input(` usage; expected clean.

## Tests / examples / docs

- Update in-scope example literals: drop `Focusable: true`.
- Unit assertions, split by ID-requiredness (an ID-less required-ID widget
  cannot be constructed — `Slider` calls `RequireID`/panics at
  `view_slider.go:45`):
  - **All in-scope widgets** (with an `ID`, no `FocusDisabled`): **exactly
    one** focus candidate (assert the count via
    `collectFocusCandidates`/`NextFocusable`, to pin against duplicate tab
    stops). With `FocusDisabled: true`: zero.
  - **Optional-ID widgets only** (`Input`, `Select`, `Toggle`, `Switch`):
    with no `ID`, zero candidates (inert, still renders). Not applicable to
    `Slider` (would panic).
- `view_input_test.go`: existing `Focusable: true` literals → remove.
- **Rewrite `TestInputReadOnlyWithoutFocus`** — it currently asserts
  non-focusable ⇒ `AccessStateReadOnly`, which relied on the dropped
  `!cfg.Focusable` clause. New assertions: `ReadOnly` announces
  `AccessStateReadOnly`; `FocusDisabled` alone does **not**.
- Docs: `shape.go` doc comment already documents `Shape.Focusable`
  (unchanged). Update per-Cfg field docs, the widget skill scaffold,
  README/CHANGELOG, and the per-example READMEs of any example app whose
  literals changed in Phases 1–2. Add a note that input controls are
  focusable by default and inert without an `ID`. Include a
  **disambiguation table** for the four now-coexisting flags:

  | Flag | Meaning |
  |------|---------|
  | `Shape.Focusable` | widget participates in the focus system |
  | `FocusSkip` | focusable + click/selection, but excluded from Tab order |
  | `FocusDisabled` (Cfg) | opt out of the default-on focus (in-scope Cfgs) |
  | `Disabled` | non-interactive; also excluded from Tab order |

## Release

Breaking (field removed) → minor bump `v0.36.0`, CHANGELOG entry. Merge to
`main`, tag, then bump siblings per dependency order.

## Execution protocol

Branch `feat/focusable-default-input` off `main`. One commit per phase,
**pause for review after each**. Verification gate before every commit:

```
make check && go test ./... && golangci-lint run ./...
```

Each code phase migrates the **whole in-repo call graph** of the Cfg it
changes (internal factories, examples, tests), or the gate fails to
compile — see [Call-site migration](#call-site-migration-mandatory-same-commit-as-the-field-removal).

- **Phase 0** — create the branch, commit this spec, pause.
- **Phase 1** — `InputCfg` (single-line + multiline): drop `Focusable`,
  add `FocusDisabled`, drop the `!cfg.Focusable` a11y clause. Migrate
  **all `InputCfg` sites** (enumerate via the commands in [Call-site
  migration](#call-site-migration-mandatory-same-commit-as-the-field-removal)):
  internal factories (wrappers via
  `FocusDisabled: !cfg.Focusable`; color-picker / command-palette / dialog
  / datagrid drop `Focusable: true`), the 8 example files, and tests
  (incl. the one-candidate assertion and the `TestInputReadOnlyWithoutFocus`
  rewrite). Gate, commit, pause.
- **Phase 2** — `Select`, `Slider`, `Toggle`, `Switch`: same swap, each
  with its full call graph (factories, examples, tests). Gate, commit,
  pause.
- **Phase 3** — docs only (per-Cfg field docs, disambiguation table,
  widget skill scaffold, README, per-example READMEs for the example apps
  touched in Phases 1–2), `requiredid` analyzer test, CHANGELOG. Gate,
  commit, pause.
- **Release** — merge to `main`, tag `v0.36.0`. Then siblings in
  dependency order (go-charts → go-kite → go-map → go-edit → go-term):
  one commit per repo, gated and shipped, pause after each.

No commits without explicit permission at each pause.

## Resolved

- Phase 1 reduced to `Input`; the two Input wrappers (`NumericInput`,
  `InputDate`) join the deferred bucket per the Decision-11 invariant and
  the audit (Findings 1–2 from review).
- Composites deferred; borderline widgets stay opt-in; Phase 1+2 ship
  together as `v0.36.0`; analyzer stays silent (Decisions 7–11).
- Review round 3 folded in: wrappers/internal factories map to
  `FocusDisabled: !cfg.Focusable` (keeps deferred bug out); each phase
  migrates its full call graph (not "widget + own tests"); Motivation
  stats corrected and Phase 2 reframed as an a11y change;
  `TestInputReadOnlyWithoutFocus` rewrite named; standalone `Radio`
  explicitly opt-in.
- Review round 4 folded in: Phase 2 a11y win scoped to ID-bearing sites
  (focus still needs `ID != ""`); the "no ID → zero candidates" test split
  by ID-requiredness (`Slider` panics without an ID, so it's excluded);
  hardcoded `InputCfg` counts replaced with `rg` commands, with the caveat
  that the compiler catches only `Focusable`-bearing literals (docs and
  omitted-field sites need manual review).

## Open questions

None blocking. Confirm the exact consumer call-sites at ship time by
grepping `Focusable: true` on in-scope Cfg literals in each repo.
