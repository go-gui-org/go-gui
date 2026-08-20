# Widget ID scoping

Status: implemented. Written 2026-08-09 after a `GOGUI_DEBUG=1` run of the
showcase reported 39 duplicate IDs; decided and implemented the same day.

## Problem

`Shape.ID` is the identity key for focus, scroll offsets, per-widget state,
`FindByID`, and hero animations. It must be unique per window. Nothing enforces
that at compile time, and the failure is silent: two widgets sharing an ID
collapse onto one tab stop and one state slot, with no error and no visual
difference.

The showcase run produced 39 reports. Only two were application mistakes. The
rest came from the framework:

| Count | Cause                                                         |
| ----- | ------------------------------------------------------------- |
| 32    | `Input` put `cfg.ID` on both its container and its inner text |
| 5     | every markdown paragraph claimed the markdown widget's ID     |
| 2     | showcase: a constant ID in a loop; a nested widget copy-paste |

Both framework causes were fixed first (`Shape.focusOwner` for the first,
container-owned ID for the second), and the check is available as
`(*Window).TestDuplicateIDs`. This document is about the ergonomics question
those bugs exposed.

## Evidence that scoping was the missing primitive

Composite widgets built scoped IDs by hand at roughly 70 sites in `gui/`, with
**five** different separators:

| Separator | Example                                                               |
| --------- | --------------------------------------------------------------------- |
| `:`       | `cfg.ID + ":handle"` (`view_splitter_handle.go`)                      |
| `.`       | `cfg.ID + ".day." + strconv.Itoa(d)` (`view_date_picker_calendar.go`) |
| `_`       | `"tc_" + controlID + "_" + tabID` (`view_tab_control.go`)             |
| `-`       | `"spell-check-" + focusID` (`spell_check.go`)                         |
| `/`       | `cfg.ID + "/" + strconv.Itoa(i)` (`view_radio_button_group.go`)       |

Every composite widget reimplemented an ID stack through string concatenation,
and `mdRenderParagraph` was the one that forgot. A primitive the framework
itself needs 70 times is a primitive, not a convenience.

Two further consequences of having no scope:

- Uniqueness is window-global. The showcase reuses `"input-text"` in two
  different panels and gets away with it only because the panels never appear at
  once. Every ID in a large application is a global name.
- Heading blocks keyed off `block.AnchorSlug`, so two markdown views showing the
  same heading collided.

## Why implicit IDs were the wrong fix

The obvious reaction — derive IDs from tree position and let the author skip
them — trades a loud failure for a silent one. In an immediate-mode tree,
identity must survive reordering, insertion, and conditional rendering. A
position-derived key migrates state when a list changes: insert a row at the top
and every row below it inherits the row above's cursor, selection, scroll offset
and focus. There is no warning for that, because from the framework's view
nothing is wrong.

Prior art agrees. Dear ImGui uses an explicit ID stack (`PushID`/`PopID`)
precisely so loop iterations disambiguate; egui derives an `Id` from the parent
scope plus a caller-supplied `id_salt`. Both are explicit identity plus a scope,
not implicit identity.

## Decisions

1. **An explicit helper, not an implicit scope push.** `gui.ScopeID` and
   `gui.ScopeIDN` compose an inner ID from an owner and one or more parts.
   Containers do not push a namespace, so moving a widget in the tree never
   changes its identity.
2. **IDs stay flat, window-global strings.** No public API changed: `SetFocus`,
   `ScrollVerticalTo`, `FindByID` and the test helpers still take the whole
   composed string. Per-scope uniqueness was considered and rejected for now —
   it would touch `FindByID`, focus traversal, scroll keys, `StateMap` keys and
   the debug audit, all of which assume a flat namespace.
3. **One separator: `:`.** Already dominant, already parsed by the datagrid, and
   rare in application IDs, which favour `-` and `_`.
4. **No escaping.** See below.

## The grammar

An ID is a `:`-joined path. The _owner_ may itself already be composed — that is
how nesting works — and composition is associative:

```go
base := gui.ScopeID(cfg.ID, "header", col.ID) // "grid:header:name"
gui.ScopeID(base, "resize")                   // "grid:header:name:resize"
```

`ScopeIDN` appends a numeric last segment without materialising the number as
its own string, for loop-derived identity:

```go
gui.ScopeIDN(cfg.ID, "opt", i) // "group:opt:2"
```

Empty segments are dropped, so an ID-less composite still produces workable
inner IDs — and two of them in one window collide loudly rather than silently.

Both functions cost **exactly one allocation**: the returned string. `ScopeID`
uses single-expression concatenation for the common arities and an exactly sized
`strings.Builder` beyond them; the variadic backing array does not escape.
`gui/id_scope_test.go` asserts this with `testing.AllocsPerRun`, and that test
is the guard — a future edit that lets `parts` escape shows up there and nowhere
else.

### Scopes versus parts

`ScopeID` composes **identities**. A **part** is a leaf value fed _into_ a
composition — a row key, a column key, a heading slug. Because there is no
escaping, a part must not contain `:`, so parts keep their own spelling:

- `__src_o_`, `__src_c_`, `__src_cx_` synthetic row keys
  (`gui/datagrid/data_source_grid.go`)
- `__auto_` (`view_data_grid_helpers.go`) and `__draft_`
  (`view_data_grid_crud.go`) row keys

All three flow through `dataGridRowID` into `ScopeID(cfg.ID, "row", rowID)`.
Rewriting them as `__src:o:5` would make a part contain the separator, producing
`grid:row:__src:o:5` — ambiguous with a real nested scope. Underscore form is
correct here, not a leftover.

### Why no escaping

Escaping would break the datagrid's reverse parse.
`dataGridHeaderColIDFromLayoutID` recovers a column ID by trimming
`dataGridHeaderPrefix(gridID)` and comparing the remainder verbatim on a
per-frame hit-test path; an unescape step would add an allocation there.
Composed IDs are also user-facing — applications pass them to
`ScrollVerticalTo`, and they appear in `GOGUI_DEBUG` output and the inspector —
and `a\:b` is worse to read and worse to type.

The hazard escaping would remove is ambiguity: two different `(owner, parts)`
tuples composing to the same string. That hazard is exactly a duplicate ID,
which `debugCheckShape` already reports and `(*Window).TestDuplicateIDs` already
asserts.

## Enforcement

`go run ./tools/ergonomics-audit/ -mode ids .` (part of `make ergonomics-audit`)
fails on any hand-rolled composition in `gui/`. It flags a separator-bearing
concatenation or `fmt.Sprintf` that either lands in an ID position — a Cfg
`...ID` field, an assignment to an `...ID` variable or field, the result of an
`...ID` function — or is built off something already named like an ID.

That second rule is the one that earns its keep. The producers are easy to
migrate; the **consumers** drift. `w.IsFocus(cfg.ID+"_popup")` rebuilds an ID
the producer has already moved, and it sits in a call argument rather than any
position the first rule can name. Four such consumers existed, and each broke
its widget when only the producer was migrated.

Two markers exempt a line, each naming its reason:

- `ergonomics-audit:id-part` — a leaf part, per the rule above.
- `ergonomics-audit:not-an-id` — not a widget ID at all: an ibus socket name, a
  spreadsheet column name, a math cache key.

The check deliberately lives in `ergonomics-audit` and not in
`tools/requiredid`, which ships as a vet pass over _application_ code where
hand-rolled composition is legitimate.

## Collisions this fixed

Three, of which the spec had predicted two:

1. `mdRenderHeading` assigned a bare `block.AnchorSlug`, so two markdown views
   sharing a heading collided. Now `ScopeID(cfg.ID, "h", slug)`.
2. `renderMdCode` keyed its copy button by block index alone. Now
   `ScopeIDN(cfg.ID, "code", blockIdx)`.
3. **Found by the new regression test, not by inspection:** the document-level
   copy button used the constant `"md_cp_doc"`, so every `Markdown` widget in a
   window shared one. Now `ScopeID(cfg.ID, "copy-doc")`.

Two identical headings in _one_ document still collide. That is now reported by
the duplicate audit rather than passing silently.

## Remaining work

**Done.** Specified and implemented in
[`widget-id-per-scope-uniqueness.md`](widget-id-per-scope-uniqueness.md), which
supersedes Decisions 1–2 here for identity resolution:

- **Per-scope uniqueness** (shipped) — framework-computed `effID` from
  ID-bearing ancestors; leaf `Shape.ID` may repeat across scopes; uniqueness
  stays on the effective key.
- **Hero animations** (shipped) — keyed on `effID`; same leaf under different
  ancestors does not match (constraint of ID-only join).
- **Caching composed IDs across frames** (shipped) — the benchmark showed +1
  alloc per widget per frame, so the identity-keyed memo landed
  (`(*Window).joinLeaf`); a positional cache is still rejected.
