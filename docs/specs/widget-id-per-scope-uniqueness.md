# Spec: per-scope uniqueness (framework-computed effective IDs)

Status: **implemented** (phases A, B and C). Written 2026-08-09. Revised after
review (round 2: generation-time scope for widget state, datagrid exception,
migration-list gaps). Implemented 2026-08-09. Source: "Remaining work" in
[`widget-id-scoping.md`](widget-id-scoping.md). Target: major version (semantic
break, signatures unchanged).

## What shipped, and where it differs from this spec

Read this section first: three decisions below were changed during
implementation, and the code is the authority.

1. **The resolve pass runs from `layoutArrange`, not from `layoutPipeline`, and
   floats are _not_ a scope boundary.** `resolveShapeIDs` walks the whole main
   tree **before** `layoutRemoveFloatingLayouts` extracts the floats, then each
   injected overlay (toast, dialog, inspector) is resolved separately as its own
   root. Consequence: a float written inside an ID-bearing panel — a combobox
   dropdown, a popover — **keeps that panel's scope**, where the "each float is
   its own pipeline root with an empty scope" rule strips it. That rule was a
   consequence of _where_ the pass ran, not a goal, and keeping the scope is
   strictly better: it removes a class of cross-panel collision, and it lets the
   generation-time scope and the pass agree with no float special case (a widget
   cannot know its shape is a float before it builds it). Injected overlays
   still start from an empty scope, so the spec's statement about tooltips and
   menus injected from outside the tree holds.
2. **`EventCtx.EffID` was added.** Decision 7 assumed a stateful widget always
   has a `*Window` while it composes. Several do not: `Input`, `Slider`,
   `Splitter`, `ProgressBar`, `Skeleton` and `Scrollbar` build their trees in
   plain factories and capture `cfg.ID` as a leaf. Their handlers resolve at
   dispatch time instead, walking the ancestor chain for the shape that carries
   the leaf. Same `resolveLeaf`, so the two paths cannot drift. `Splitter` is
   now converted (issue #264): it is a struct view whose `GenerateLayout`
   resolves the effective ID once (`id := w.EffID(cfg.ID)`) and composes every
   inner ID (panes, handle, collapse buttons) from that path — the root shape
   stays on the plain leaf and the framework joins it to the same string, while
   the composed children are absolute.
3. **The "still globally competing" warning is a debug category, not an
   ergonomics-audit mode.** "Has no ID-bearing ancestor" is a property of the
   composed tree, which the AST does not have. It is `gui.DebugUnscopedIDs`,
   deliberately **outside** `DebugAll` — it reports a design property, not a
   defect, and fires on most widgets in a small app. Enable it with
   `gui.DebugCategories(gui.DebugUnscopedIDs)`.

Phase A.4 (producer simplification) was applied where a composite's own nesting
already mirrors `ScopeID(cfg.ID, part)` — combobox and select now set a plain
`"dropdown"` leaf. Composites whose inner IDs are reverse-parsed or whose owner
is not an ancestor keep absolute IDs, and the remaining stateful widgets resolve
`cfg.ID` once (`cfg.ID = w.EffID(cfg.ID)`) so their composed children are
absolute strings equal to their own effective paths. Datagrid is untouched, as
decided below.

**Phase C is done, and the benchmark decided it.** Without a cache the join cost
one allocation per ID-bearing widget per frame, measured on `BenchmarkViewFrame`
as 202 → 252 allocs/op (rows_50) and 802 → 1002 (rows_200) — exactly +1 per row.
`(*Window).joinLeaf` memoizes `(scope, leaf) → joined` in a bounded map shared
by both paths, which puts every one of those numbers back on its baseline.
`TestJoinLeafCachedIsAllocationFree` gates it. A hit is always correct and an
eviction only recomputes, because the key is identity, not position — the
objection to a positional cache never applied here.

One cost remains and cannot be cached away: `effID` moved `Shape` from 280 to
296 bytes, across a size-class boundary (288 → 320), so every shape allocates 32
bytes more. On an 11k-shape frame that is ~355 KB of extra transient garbage.
`focusOwner` is resolved **in place** rather than in a second field to avoid
paying that again — a second string leaves only 8 bytes of headroom before the
next size class.

One behavioral detail worth recording: `a11yLabel` now announces only the last
segment when it falls back to an ID. The fallback is a widget ID, IDs are now
paths, and a screen reader must hear `name`, not `settings:name`. An explicit
`A11YLabel` is never touched.

This document is the normative spec for the change. Implementation phases follow
the decisions below. Parent doc [`widget-id-scoping.md`](widget-id-scoping.md)
keeps the `:` grammar, `ScopeID` / `ScopeIDN`, and no-escaping rules.

## Problem

Nothing crashes. The remaining limit is **composability**.

Today every `Shape.ID` is a **window-global** key for focus, scroll, hover,
hero, and per-widget state. Two shapes with the same ID in one frame share one
slot. `TestDuplicateIDs` and the debug audit report that. They do not make reuse
safe.

This is illegal (or fragile) even when the widgets sit in different panels:

```go
Panel{ID: "settings", Content: Input{ID: "name", ...}}
Panel{ID: "profile",  Content: Input{ID: "name", ...}}
```

Both claim `"name"`. Authors must hand-namespace every nested control
(`ScopeID("settings", "name")`, …). In a large app, every leaf ID is a global
name.

The showcase reuses `"input-text"` across demos only because those demos do not
appear in the same frame. That is mutual exclusion, not scoping. Reuse under two
ID-bearing panels becomes safe only after this change, and only if those panels
actually carry IDs.

[`widget-id-scoping.md`](widget-id-scoping.md) already fixed the other half: one
separator, `ScopeID`, and framework self-collisions (`focusOwner`, markdown
ownership). **Per-scope uniqueness** is the leftover: can two panels each use
`"name"` if the framework joins ancestor IDs into an effective key?

## Why raw IDs and tree position both fail

Per-container uniqueness is only meaningful if the **framework** computes
identity:

| Approach                             | Result                                                                                                     |
| ------------------------------------ | ---------------------------------------------------------------------------------------------------------- |
| Key stores on raw `Shape.ID`         | No-op. Both panels still store under `"name"`.                                                             |
| Scope by tree position / child index | Silent identity migration on reorder — the failure Decision 1 in the parent spec rejects for implicit IDs. |

## Invariant

**Effective identity** (`effID`) is window-unique. **Leaf** `Shape.ID` can
repeat across scopes.

```
effID(shape) =
  shape.ID                           if shape.ID contains ":"   // absolute
  ScopeID(effID(nearest ID-bearing ancestor), shape.ID)
                                     if shape.ID != "" and an ID-bearing ancestor exists
  shape.ID                           if shape.ID != "" and no ID-bearing ancestor
  ""                                 if shape.ID == ""
```

Rules:

- **ID-bearing** means non-empty `Shape.ID`. Join only on those ancestors. Never
  position, never count.
- ID-less ancestors add no scope. Children under them stay flat. Collisions stay
  loud, as today.
- An absolute ancestor still contributes its full `effID` as the join prefix:
  leaf `"name"` under `"app:settings"` → `"app:settings:name"`.
- A leaf that already contains `:` is an **absolute** identity. The resolution
  pass does not join further. That is the compatibility path for today's
  `ScopeID` results.
- Absolute (`:`) is not an opt-out that keeps a bare global leaf under an ID'd
  ancestor. Under an ID'd ancestor, a plain leaf always becomes `ancestor:leaf`.
  There is no "stay bare `ok` under panel `p`" mode.
- **Joined vs absolute can collide:** leaf `"b"` under ID-bearing ancestor `"a"`
  yields `"a:b"`, which an absolute root leaf `"a:b"` also yields. Same stance
  as the parent spec's escaping note: it is a duplicate ID, reported loudly by
  `TestDuplicateIDs` and the debug audit, never silent.

`TestDuplicateIDs` and the debug audit key on `effID`, not on the leaf.

## Decisions

These supersede parent Decisions 1–2 for identity resolution. The `:` grammar
and no-escaping rules stay.

This is **not** a widget-side ID stack inside `GenerateLayout`. Widget factories
still set leaf `Shape.ID` (plain or absolute). Scope is the framework's job. A
post-build resolve pass stamps `effID` before any ID-keyed store or match that
runs after layout generation. Widget-internal state read **during**
`GenerateLayout` cannot wait for that pass — Decision 7 gives widgets their
scope at generation time instead.

1. **Framework join on ID-bearing ancestors.** Containers with an ID push a
   namespace for descendants that use plain leaf IDs. Moving a widget under a
   differently-named ID'd container **changes** its `effID` (and its state
   slot). That is intentional: the app changed an explicit ancestor ID. Reorder
   under the same ancestor keeps identity stable.
2. **Public APIs take effective IDs.** `SetFocus`, `ScrollVerticalTo`,
   `FindByID`, and test helpers keep the same signatures (flat `string`). The
   string they match is `effID`, not the leaf. Call sites that pass a bare leaf
   after an ancestor gains an ID must pass the full path (or keep composing with
   `ScopeID` and rely on the absolute escape). This is a **semantic** API break
   on a major version.
3. **`Shape.ID` is the leaf. `Shape.effID` is the resolved path.** New
   unexported field. Do not rewrite `ID` in the pass — that breaks debug paths
   and mid-frame reads that still mean the leaf.
4. **No bare global leaf under an ID'd ancestor.** Answer to the opt-out
   question: the `:` absolute form is sufficient. A separate opt-out is not.
5. **Hero stays identity-keyed.** Same leaf under different ancestor IDs does
   **not** hero-match across a transition. Apps that need a shared hero key must
   use the same absolute (`:`-bearing) ID on both sides, or share an ID-bearing
   ancestor path.
6. **`focusOwner` / `focusKey` resolve to `effID`.** Today `focusOwner` is a
   string copy of the owner's leaf ID. `focusKey()` returns that string or
   `s.ID`. After stores key on `effID`, that leaf string is wrong under a scoped
   ancestor (`"name"` vs `"settings:name"`). The resolve pass (or `focusKey`)
   must return the **owner's `effID`**. Preferred: stamp an unexported owner
   effective key during resolve (or turn `focusOwner` into a `*Shape` and read
   `owner.effID`). Do not leave `focusKey` returning a bare leaf while focus /
   input-state / spell-check maps hold `effID`.
7. **Generation-time scope for widget-state keys.** Widgets that read `StateMap`
   inside `GenerateLayout` (combobox open/query/highlight/items, theme-picker
   select, tree / sidebar / listbox state, …) cannot wait for the resolve pass —
   their tree shape depends on the read. `generateViewLayout` maintains a
   framework-side scope: before recursing into a child it pushes the child's
   `effID`. `w.EffID(leaf)` joins the current scope with the leaf (absolute `:`
   leaves pass through unchanged). Stateful widgets key every `ns*` map on
   `w.EffID(cfg.ID)`, reads and writes alike (event handlers close the key over
   at generation. It stays valid while ancestor IDs do). The float boundary
   resets scope to `""` at each float root during generation, matching the
   resolve pass. One `resolveLeaf(scope, leaf)` helper implements both paths so
   they cannot drift. Cost: one `:`-join per stateful widget per frame — Phase
   C's memo is shared with this path.

## Worked examples

| Tree                                                      | Leaf IDs           | `effID`s                                         |
| --------------------------------------------------------- | ------------------ | ------------------------------------------------ |
| `Panel{ID:"settings"}` → `Input{ID:"name"}`               | `settings`, `name` | `settings`, `settings:name`                      |
| Two such panels (`settings` / `profile`)                  | same leaves        | `settings:name` vs `profile:name` — no collision |
| ID-less `Column` → two `Input{ID:"name"}`                 | `name`, `name`     | both `name` — loud collision, as today           |
| `Input{ID: ScopeID("grid","row","1")}` under any ancestor | `grid:row:1`       | `grid:row:1` (absolute, no further join)         |

### Producer simplification is load-bearing

Absolute children skip the join. Nested `ScopeID(cfg.ID, part)` therefore stays
window-global even when the owner shape sits under an ID'd panel:

```go
// Two pickers, same cfg.ID, under different panels — owners are fine,
// absolute scroll children collide:
Panel{ID: "settings", Content: ColorPicker{ID: "palette"}} // owner → settings:palette
Panel{ID: "profile",  Content: ColorPicker{ID: "palette"}} // owner → profile:palette
// each still emits ScopeID("palette", "scroll") → "palette:scroll" (absolute)
```

A composite whose own container nesting already mirrors `ScopeID(owner, part)`
**must** drop that `ScopeID` and set a plain leaf (no `:`) once
`resolveShapeIDs` exists. Until those producers simplify, reuse under ID'd
ancestors is **not** safe for that widget — `TestDuplicateIDs` will still report
the absolute children. Leave absolute only when the leaf needs `:` (`ScopeIDN`)
or the owner is not an ancestor ID (synthetic prefixes, toast, command-button
scopes).

```go
// App content under ID'd panels — the composability win
Panel{ID: "settings", Content: Input{ID: "name"}}  // effID → settings:name
Panel{ID: "profile",  Content: Input{ID: "name"}}  // effID → profile:name

// Framework composite — scroll child under a shape that already has cfg.ID
scrollID := ScopeID(cfg.ID, "scroll") // today: absolute "palette:scroll"
ID: "scroll"                          // after: leaf; effID → palette:scroll
                                      // under panel "settings" → settings:palette:scroll

// Keep absolute when the leaf needs ":" (ScopeIDN) or the owner is not an
// ancestor ID (synthetic prefixes, toast, command-button scopes):
ID: ScopeIDN(cfg.ID, "opt", i)        // "group:opt:2" — do not strip
```

`ScopeIDN("", "opt", i)` is **not** a valid simplification: it yields `"opt:2"`,
which contains `:` and therefore skips the join.

**Datagrid is the canonical cannot-simplify composite.** Its children are
absolute multi-part IDs by design: `dataGridHeaderColIDFromLayoutID` reverse-
parses a header cell ID by trimming `dataGridHeaderPrefix(gridID)` off a
`ScopeID(gridID, "header", col)` leaf (`gui/datagrid/view_data_grid_header.go`),
and row keys (`ScopeID(cfg.ID, "row", rowID)`) and the scroll child
(`dataGridScrollID`) are likewise multi-part. Absolute leaves skip the join, so
two grids with the same `cfg.ID` under different ID'd panels **still collide on
every child ID** — the composability win does not extend to datagrid, and the
children must stay absolute because the reverse parse requires it. The grid
root's own leaf becomes `panel:grid`. Audit its leaf-ID consumers
(`FindByID(cfg.ID)`, `IsFocus(cfg.ID)`, reverse-parse callers) as migration
work. A future per-grid prefix fix parses against the grid's `effID`.

**Dock is the second deliberate absolute composite, for the opposite reason.** A
dock group container takes `ScopeID(dockID, node.ID)` and a dock splitter takes
`ScopeID(dockID, "split", node.ID)` (`gui/view_dock_layout.go`), so both are
absolute and skip the join. That is the point: a group's position in the dock
tree changes on every drop, and the group scopes the panel content inside it, so
a position-derived group ID re-keys every widget in the panel — its scroll
offset, its focus, its input state — whenever anything else in the dock moves
(issue #389). The dock's own ID is resolved (`w.EffID(cfg.ID)`), so the composed
ID still carries the surrounding scope. What it does not carry is the splitter
path. Node IDs minted by `dockTreeSplitAt` / `dockTreeWrapRoot` join with `-`,
not `IDSep`: a node ID is tree data fed into `ScopeID` as a **part**, and an
`IDSep` in a part makes the composed leaf absolute in the wrong way —
window-global, outside the dock scope.

## Resolution pass

`resolveShapeIDs(layout, scope string)` runs **first** in every `layoutPipeline`
call (`gui/layout_pipeline.go`) — before width/height passes and before
`applyLayoutTransition` / `applyHeroTransition`. It stamps `effID` on every
shape before any ID-keyed store or match that runs after layout generation.

Wire it for **every root** `layoutArrange` feeds the pipeline
(`gui/layout_arrange.go`): main layout and each floating layout. A miss on any
root is a silent break.

**Float scope boundary:** each float/dialog is its own pipeline root with an
empty initial scope. It does **not** inherit ID-bearing ancestors from the main
tree. Injected tooltips/menus only pick up scope from ID'd ancestors **inside
their own tree**. Authors who need the triggering panel's prefix must set an
absolute ID (or put an ID'd ancestor in the float tree).

The Decision 7 generation-time scope resets identically: a `Float` layout's
subtree generates with empty scope, so a widget inside a float reads the same
keys the resolve pass produces. Both rules go through the shared
`resolveLeaf(scope, leaf)` helper.

## Migrate keying and match sites

Switch `shape.ID` → `effID` at stores and matches that run after layout, and
`cfg.ID` → `w.EffID(cfg.ID)` for widget-state keys read during `GenerateLayout`,
one commit each with a test:

- **Stores:** hover map (`gui/layout_pipeline.go`), focus `seen` map
  (`gui/layout_query.go`), focus store `w.viewState.focusID`
  (`gui/window_focus.go`), overflow map (`nsOverflow`, `gui/state_registry.go`),
  debug audit `ids` map (`gui/debug.go`), layout-transition snapshots
  (`gui/animation_layout.go`), hero snapshots/apply (`gui/animation_hero.go`),
  scroll matching / `findScrollLayout`, **StateMap** entries keyed by widget
  identity:
  - read **after** layout (input-state, spell-check, focus paint) → `effID`
  - read **during** `GenerateLayout` (combobox, theme picker, tree, sidebar,
    listbox, date picker, …) → `w.EffID(cfg.ID)` (Decision 7)
- **Matches:** `FindByID`, `FindLayoutByFocusID`, `FindLayoutByScrollID`,
  `gui/event_handlers.go`, `gui/view_slider.go`, `gui/view_tooltip.go`.
- **`reservedDialogID`:** keep comparing the **leaf** sentinel
  (`"___dialog_reserved_do_not_use___"`). Dialogs are separate float roots, so
  leaf and `effID` stay equal under empty scope. Do not fold this into the
  FindByID migration. Optionally make the constant absolute later if dialogs
  ever nest under ID'd ancestors in the same tree.
- **Also migrate** any widget code that compares `cfg.ID` or `Shape.ID` to the
  focus / scroll / hover / StateMap store (`IsFocus`, focus paint, input-state,
  spell-check). After the change those stores hold `effID`. "Already a full path
  string" is not enough once producers emit plain leaves. Fix `focusKey()` per
  Decision 6 so Input's inner text keeps working. Note the `AmendLayout`
  `IsFocus` call sites that pass the leaf today — for example the combobox's
  `AmendLayout` uses `ctx.Layout.Shape.ID` (`gui/view_combobox.go`) — they
  become `effID` (or `w.EffID`).

## Implementation phases

### Phase A — per-scope uniqueness

1. Land this spec (done as draft).
2. Add `effID` + `resolveShapeIDs`. Call it first in every `layoutPipeline`
   (main + floats). Implement Decision 6 (`focusKey` → owner `effID`) and
   Decision 7 (generation-time scope, `w.EffID`, shared `resolveLeaf`).
3. Migrate stores/matches as above (including StateMap: post-layout keys to
   `effID`, `GenerateLayout`-time keys to `w.EffID`. Exclude `reservedDialogID`
   leaf sentinel).
4. **Required before the composability claim:** simplify every nesting-mirroring
   `ScopeID(cfg.ID, part)` producer to a plain leaf. Leave absolute leaves that
   cannot simplify. Until this lands, two instances of the same composite under
   different ID'd panels still collide on absolute children.
5. ergonomics-audit `ids` mode: warn on state-keyed shapes (focusable /
   scrollable / stateful) with a plain leaf and no ID-bearing ancestor (still
   globally competing).
6. Docs: CLAUDE.md Focus section, parent Remaining work pointer, showcase
   regression. Showcase win requires ID-bearing demo/panel ancestors.

### Phase B — hero

Constraint, not a feature: join depends only on explicit IDs, so paths stay
stable across view transitions when ancestors stay the same. Key hero and layout
snapshots on `effID`. Add a test with an ID-bearing ancestor. Document that
different ancestor IDs do not match (Decision 5).

### Phase C — ID caching

The positional-cache objection dissolves: a cross-frame memo keyed by
**(ownerEffectiveID, leafID) → string** is identity-keyed. Eviction recomputes.
It never hands the wrong ID to the wrong row. The memo lives on `Window` and is
shared with Decision 7's generation-time joins — same key, same pure function,
so a miss recomputes identically.

1. Benchmark two frames (allocs/op + ns/op): (a) today's absolute datagrid path
   — Phase A adds nothing for `:` leaves. (b) A scoped-panel tree after Phase
   A.4, where joins run inside `resolveShapeIDs`. (c) A stateful-widget tree
   (combobox-style), where Decision 7 joins run during `GenerateLayout`. Closing
   "no cache" on (a) alone is wrong once (b) and (c) exist.
2. If (b) or (c) shows: bounded (LRU) memo in the resolve pass **and** the
   generation-time join path, alloc-gate test, invalidation note here.
3. If not: record "no cache" here and close the item.

## Relation to `widget-id-scoping.md`

| Parent                                                      | This spec                                                                                                                                                         |
| ----------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Decision 1: helper only, containers do not push a namespace | Superseded for plain leaves under ID-bearing ancestors — via post-build resolve plus a framework-side generation-time scope (Decision 7), not a widget-side stack |
| Decision 2: flat window-global strings, per-scope deferred  | Effective IDs stay flat strings. Uniqueness is on `effID`. Leaf reuse across scopes is allowed                                                                    |
| Grammar / no escaping / `ScopeID`                           | Unchanged. Absolute leaves are today's composed strings                                                                                                           |

## Closed questions

1. **Version:** major. Semantic break for callers that pass leaf IDs into public
   APIs once ancestors are ID'd.
2. **`effID` field:** new unexported field. Do not rewrite `ID`.
3. **Opt-out:** no separate opt-out. `:` means absolute. A plain leaf under an
   ID'd ancestor always joins.
4. **`focusOwner`:** resolve to owner's `effID` (Decision 6). Preferred stamp
   during resolve or `*Shape` reference. Bare leaf `focusKey` is invalid after
   stores migrate.
5. **Widget-state keys read during `GenerateLayout`:** migrate to
   `w.EffID(cfg.ID)` (Decision 7) — the resolve pass alone cannot serve reads
   that happen before it.
6. **Datagrid:** stays a permanent absolute-leaf exception. The composability
   claim explicitly excludes it until a per-grid `effID`-based parse exists.
7. **Joined vs absolute collision** (`"a:b"` from two spellings): allowed,
   reported loudly by the duplicate-ID check. No escaping, per parent spec.
