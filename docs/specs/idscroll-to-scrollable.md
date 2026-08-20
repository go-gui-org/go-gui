# Spec: Replace `IDScroll uint32` with `Scrollable bool` + string scroll identity

Status: implemented (go-gui #78, released v0.35.0; all five siblings bumped)
Base: `main` @ `3c5dac7` Target release: go-gui `v0.35.0` (breaking) Precedent:
[idfocus-to-focusable.md](idfocus-to-focusable.md) (`v0.34.0`)

## Motivation

`IDScroll uint32` carries the same overload `IDFocus` did: the value is both the
**opt-in flag** (`IDScroll > 0` gates scroll dispatch, scrollbar injection,
sizing, and hit-testing) and the **key** into the window's scroll-offset maps
(`w.scrollXMap` / `w.scrollYMap`, reached via `Window.ScrollX()` / `ScrollY()`).
It is a weaker case than `IDFocus`, which was overloaded three ways — the
numeric value also encoded tab order. `IDScroll` has no ordering semantics, so
this is consistency work, not a coupling bug.

`v0.34.0` explicitly deferred it ("`IDScroll` untouched"). Post-#76 the API is
split-brained: focus opts in with a bool and identifies by `ID`, scroll opts in
with a magic number and identifies by that number. Consumers hash strings into
it by hand — `FnvSum32(cfg.ID + ":scroll")` (datagrid),
`FnvSum32(cfg.ID + ".dropdown")` (`view_select`), `idScrollHash(id)` (go-charts)
— the same derivation #76 deleted from the focus path. Examples pick numbers out
of the air: `9110`, `101`, `9182`.

## Key insight: the derived-ID convention already exists

**Every composite widget that hashes a scroll id is hashing a string it already
stores in `Shape.ID`.** This is the single most important fact for the
implementer, and it de-risks most of PR C:

| site                                 | today                                                                                     |
| ------------------------------------ | ----------------------------------------------------------------------------------------- |
| `datagrid/view_data_grid.go:584-585` | `ID: resolvedCfg.ID + ":scroll"` **and** `IDScroll: FnvSum32(resolvedCfg.ID + ":scroll")` |
| `view_select.go:68,128`              | `ID: cfg.ID + ".dropdown"` **and** `IDScroll: FnvSum32(cfg.ID + ".dropdown")`             |
| `view_combobox.go:207,219`           | `ID: cfg.ID + ".dropdown"` **and** `IDScroll: cfg.IDScroll`                               |

For these, the migration is: **delete the hash, set `Scrollable: true`, and the
identity is already correct in `ID`.** No new naming scheme is needed —
`cfg.ID + ":scroll"` is the established convention (mirroring
`cfg.ID + ":focus"` from #76).

## Measured evidence

Benchmarked on `main` @ `3c5dac7` (M5, go1.26.5, `benchstat`, n=8–10). All
spikes reverted; nothing committed.

**Shape size — the migration makes `Shape` smaller:**

| field change                                                                   | bytes         |
| ------------------------------------------------------------------------------ | ------------- |
| remove `IDScroll uint32`                                                       | −4            |
| remove `IDScrollContainer uint32` (dead)                                       | −4            |
| add `Scrollable bool` (absorbed into existing `Focusable`/`FocusSkip` padding) | +0            |
| identity reuses existing `Shape.ID string`                                     | +0            |
| **net**                                                                        | **272 → 264** |

Measured with a mirror struct, not predicted. An earlier estimate of _+24 bytes_
assumed both fields must become strings; wrong on both counts.

**Key type:**

| operation                           | uint32   | string   |
| ----------------------------------- | -------- | -------- |
| `BoundedMap.Get` (per frame)        | 10.70 ns | 5.23 ns  |
| raw `map[K]float32` Get             | 2.37 ns  | 5.33 ns  |
| `BoundedMap.Set` (per scroll event) | 10.25 ns | 14.15 ns |
| tree walk, worst case               | 241 ns   | 310 ns   |

Net cost is the tree walk: +69 ns per scrollbar per frame via `AmendLayout`,
against a 4–5 ms frame budget. Noise. Read the `Get` improvement with the caveat
in "Out of scope" — it is borrowed, not earned.

## Decisions (locked)

1. **Opt-in**: `Scrollable bool`. Identity = existing `Cfg.ID` (string).
2. **`ScrollMode` stays** as the axis restriction. It cannot serve as the
   opt-in: its zero value is `ScrollBoth`, so making it the gate would silently
   turn every container into a scroll container.
3. **`IDScrollContainer` deleted outright**, not migrated (PR A).
4. **`FindLayoutByIDScroll` → `FindLayoutByScrollID`**, parallel to #76's
   `FindLayoutByIDFocus` → `FindLayoutByFocusID`. Keep it as a separate function
   from `FindLayoutByFocusID` — near-identical, differing only in which bool
   they gate on (`Scrollable` vs `Focusable`). Predicate:
   `id != "" && layout.Shape.Scrollable && layout.Shape.ID == id` (empty id →
   miss, same as focus). Do not leave the old name: `IDScroll` is being deleted.

   **This is a behavior change, not only a rename.** Today's predicate
   (`layout_query.go:61-63`) is a bare `layout.Shape.IDScroll == idScroll` — no
   opt-in gate and no zero guard, so `findScrollLayout(w, 0)` currently matches
   the root (the first shape with `IDScroll == 0`). The new predicate is
   strictly better; it is called out here so the tightened diff does not read as
   an unexplained mismatch during review.

5. **Meaningful string IDs** in examples (`"catalog"`, `"detail"`), not
   mechanical translations of the old numbers.
6. Clean break — no back-compat shim.
7. `Scrollable: true` requires non-empty `ID`. Add `RequireScrollID` next to
   `RequireFocusID` (`gui/state_registry.go:134`) as the sibling helper:

   ```go
   // RequireScrollID panics if a Scrollable widget has an empty ID.
   func RequireScrollID(widget string, scrollable bool, id string) {
       if scrollable && id == "" {
           panic("gui: " + widget + " with Scrollable:true requires a non-empty Cfg.ID")
       }
   }
   ```

   **Do not frame this as "mirroring wired focus enforcement."**
   `RequireFocusID` exists but has **zero call sites** — #76 enforced non-empty
   IDs via `RequireID` on stateful widget factories, not via a central Focusable
   check. Containers are different: `ContainerCfg.ID` is optional for non-scroll
   containers, so there is no per-factory `RequireID` to lean on. **Wire
   `RequireScrollID("container", cfg.Scrollable, cfg.ID)` inside
   `buildContainerShape`** (`gui/view_container.go:316`) — every layout
   container compiles down to it, including widgets that build a `containerView`
   directly. That is intentional strengthening relative to focus, not a copy of
   an existing pattern.

   **Coverage caveat — it catches one class of site, not "every missed site".**
   Do not treat this as a general safety net:

   - **Suffix-derived ids defeat it.** `cfg.ID + ":scroll"` and
     `cfg.ID + ".dropdown"` are never empty, so DataGrid, CommandPalette,
     Combobox, Select and Table's freeze path can never trip it — even with an
     empty `cfg.ID`, which silently keys scroll on the bare suffix.
     (Pre-existing: `FnvSum32(".dropdown")` has the same hole today. Not a
     regression, not fixed here.)
   - **Direct-`cfg.ID` containers already have a guard.** ListBox, Tree, Table,
     Combobox, CommandPalette and DataGrid each call `RequireID` at their
     factory (`view_listbox.go:75`, `view_tree.go:149`, `view_table.go:130`,
     `view_combobox.go:64`, `view_command_palette.go:63`,
     `datagrid/view_data_grid.go:455`), so `RequireScrollID` is redundant there.
   - **Its only live catch** is a bare `Column(ContainerCfg{Scrollable: true})`
     with no `ID` — which is exactly what the 14 examples using `IDScroll: 9110`
     become if migrated carelessly. That is why it is still worth wiring.
   - A hand-rolled `&Shape{Scrollable: true}` bypasses it entirely. None exist
     today; grep `rg -n 'Shape\{' --type go gui/` at land time to confirm none
     arrived in the meantime.

8. **No duplicate-ID detection. Do not build it.** #76 warns on duplicate
   focusable IDs, and the obvious move is to mirror that here. Don't — the two
   failure modes are not comparable:

   |           | duplicate focus ID                   | duplicate scroll ID                                                |
   | --------- | ------------------------------------ | ------------------------------------------------------------------ |
   | symptom   | widget silently skipped in tab order | two containers scroll in lockstep                                  |
   | noticed   | easy to miss                         | usually obvious when scrolling; weaker signal under virtualization |
   | diagnosis | needs a warning to find              | usually self-evident; not worth a per-frame seen-set               |

   A warning would cost a `scrollDebug` env gate, a `scrollDupWarn` helper, and
   a per-frame seen-set in the view-phase runtime — to detect a bug that usually
   announces itself the first time anyone scrolls. Not worth it.

   Note decision 12 forces a deliberate near-miss here: Table's freeze path puts
   a scrollable `bodyCfg` under an outer that already holds `ID: cfg.ID`.
   Suffixing the body `:scroll` keeps ids unique; the rejected alternative would
   have made two shapes share one.

   `RequireScrollID` (decision 7) still catches the empty-`ID` case, which is
   the one that matters during this migration: it is a compile-clean mistake
   with a panic as the backstop.

9. **`DataGridCfg.IDScroll` override is deleted**, not migrated.
   `dataGridScrollID` collapses to `return cfg.ID + ":scroll"` with no branch,
   matching its sibling `dataGridFocusID` (`:119-121`), which has no override.
   Breaking for any consumer setting it explicitly; grep shows none across the
   five siblings, so blast radius is zero.

10. **`ScrollbarCfg.IDScroll` is renamed `ScrollID string`** — it points at
    another container's state and never becomes a bool. Name locked now, before
    the sibling migration, since go-charts touches it. `dragReorderStartCfg`
    takes the same `ScrollID string` treatment (unexported, no consumer impact).
    Also retype the runtime `dragReorder` state field and
    `dragReorderAutoScroll` parameter (`drag_reorder.go:135`, `:527`) — not just
    the start cfg.

    **Static assist (not a guarantee):** tag `ScrollbarCfg.ScrollID` with
    `` `gui:"required"` `` so `tools/requiredid` flags bare
    `Scrollbar(ScrollbarCfg{…})` literals missing a target. The analyzer only
    inspects factory-arg composite literals — it does **not** see
    `ScrollbarCfgX/Y: &ScrollbarCfg{…}` overrides. Those are fine:
    `appendScrollbar` always overwrites `ScrollID` from the container. Real
    safety nets: that overwrite + `RequireScrollID` (decision 7).

11. **Every public `IDScroll uint32` is an identity handle, not just an opt-in —
    decision 9 generalizes to all of them.** The caller picks the number _and_
    can pass it to `Window.ScrollVerticalTo` / `ScrollX()` / `ScrollY()`.
    go-kite does exactly this (`IDScroll: timelineScrollID` **and**
    `ScrollVerticalTo(timelineScrollID, 0)`), which is why the "3 refs, trivial"
    note in the sibling section understates it.

    Deleting the field takes that handle away from `ContainerCfg`, `ListBoxCfg`,
    `TreeCfg`, `TableCfg`, `ComboboxCfg`, `CommandPaletteCfg`, `InputCfg` and
    `DataGridCfg` alike. Do **not** reintroduce an optional `ScrollID string`
    override to preserve it — that is the branch decision 9 just deleted.

    Instead, **the derivation becomes the public contract and must be documented
    on each `Scrollable` field's doc comment.** An undocumented derivation is
    not a handle, it is a private convention consumers have to read go-gui's
    source to discover. Today only datagrid's (`cfg.ID + ":scroll"`) is written
    down anywhere.

    | Cfg                    | scroll key after migration                                          |
    | ---------------------- | ------------------------------------------------------------------- |
    | `ContainerCfg`         | `cfg.ID`                                                            |
    | `ListBoxCfg`           | `cfg.ID`                                                            |
    | `TreeCfg`              | `cfg.ID`                                                            |
    | `InputCfg` (multiline) | `cfg.ID`                                                            |
    | `TableCfg`             | `cfg.ID`, or `cfg.ID + ":scroll"` when `FreezeHeader` (decision 12) |
    | `ComboboxCfg`          | `cfg.ID + ".dropdown"`                                              |
    | `CommandPaletteCfg`    | `cfg.ID + ":scroll"`                                                |
    | `DataGridCfg`          | `cfg.ID + ":scroll"`                                                |

    ```go
    // ListBoxCfg
    //
    // Scrollable opts the list into the scroll system. Scroll state is
    // keyed by Cfg.ID — pass that same id to Window.ScrollVerticalTo.
    Scrollable bool

    // ComboboxCfg
    //
    // Scrollable opts the dropdown into the scroll system. Scroll state
    // is keyed by Cfg.ID + ".dropdown".
    Scrollable bool
    ```

    CHANGELOG must list the lost handle as breaking for all eight, not just
    `DataGridCfg` — **and must list `Window.ScrollX()` / `ScrollY()` as a third
    breaking change**: they return `*BoundedMap[uint32, float32]` today and
    `*BoundedMap[string, float32]` after. Both are exported and their doc
    comments state they exist for external packages doing virtualization, so the
    retype breaks any consumer reading offsets directly, not just those setting
    a Cfg field.

12. **`TableCfg` has two layout paths with different scroll structure; identity
    branches on `freeze`.** See "Table's two paths" under PR C. Non-freeze keeps
    identity `cfg.ID` (the outer `Column` already carries it); the freeze path's
    `bodyCfg` gets `ID: cfg.ID + ":scroll"`. Add
    `tableScrollID(cfg *TableCfg, freeze bool) string` so the container and the
    virtualization read cannot drift apart:

    ```go
    func tableScrollID(cfg *TableCfg, freeze bool) string {
        if freeze {
            return cfg.ID + ":scroll" // inner bodyCfg carries it
        }
        return cfg.ID // outerCfg carries it
    }
    ```

    **Call it once per view, into a local — do not call it per use.** On the
    freeze path each call allocates a concatenation, and there are two uses (the
    `:202` virtualization read and the `bodyCfg` literal at `:471`). `freeze` is
    computed at `:175`; derive `scrollID` immediately below it and thread the
    local to both. Same rule for every other suffix-derived site (see "Per-frame
    allocation" below).

    Rejected: giving the freeze `bodyCfg` `ID: cfg.ID` to avoid the branch.
    Scroll would still resolve (`FindLayoutByScrollID` gates on `Scrollable`,
    and the freeze outer is not scrollable), but two shapes in the same tree
    would share an id and `FindByID(cfg.ID)` becomes ambiguous. Also rejected:
    restructuring the non-freeze path to add a dedicated inner scroll container
    so both derive `cfg.ID + ":scroll"` — correct, but it adds a layout node and
    is outside this spec's blast radius.

## Phase 0 — branch + execution protocol (do before any edit)

Work happens on one feature branch, not on `main`. Confirm the base is current
first (`git fetch && git status` — rebase if stale), then:

```fish
git switch -c idscroll-to-scrollable main
```

The work spans six repos. Phases 1–4 are go-gui on the branch above; phases 5–9
are the release and the sibling migration, each sibling on its own branch in its
own repo.

| phase | repo                     | scope                                                                      | commit subject                                                          |
| ----- | ------------------------ | -------------------------------------------------------------------------- | ----------------------------------------------------------------------- |
| 0     | go-gui                   | branch created, no code                                                    | _(no commit)_                                                           |
| 1     | go-gui                   | PR A — delete `IDScrollContainer`                                          | `gui: delete dead IDScrollContainer field and its tree walk`            |
| 2     | go-gui                   | PR B — `ViewFrame` benchmark                                               | `gui: add BenchmarkViewFrame to catch Shape-size regressions`           |
| 3     | go-gui                   | PR C — `Scrollable` + string identity                                      | `gui: replace IDScroll uint32 with Scrollable + string scroll identity` |
| 4     | go-gui                   | tests / examples / docs / CHANGELOG                                        | `docs: migrate IDScroll references to Scrollable`                       |
| 5     | go-kite, go-charts       | **dry run** — migrate against unreleased go-gui, prove clean, do not merge | _(no commit; branches held)_                                            |
| 6     | go-gui                   | release `v0.35.0` (PR A + PR C batched), await proxy                       | _(release skill)_                                                       |
| 7     | go-kite                  | migration + `go.mod` bump, one PR                                          | `deps: migrate to go-gui v0.35.0 Scrollable API`                        |
| 8     | go-charts                | migration + `go.mod` bump + CI `ref:` pins, one PR                         | `deps: migrate to go-gui v0.35.0 Scrollable API`                        |
| 9     | go-edit, go-map, go-term | bump only, via `/sync-siblings`                                            | _(sync-siblings drives)_                                                |

Rules for every phase:

1. **Run the verification gate before committing**
   (`go build ./... && go vet ./... && golangci-lint run ./... && go test ./...`).
   A phase with a red gate is not done; do not commit past it.
2. **Commit at the end of the phase**, that phase only — no squashing phases
   together, no work from the next phase riding along.
3. **Pause after the commit and wait for review.** Do not start the next phase
   until the reviewer says so. Report what was committed, the gate result, and
   anything the phase surfaced that the spec did not predict.
4. Do not push or open PRs without explicit permission.

Phase 3 is the breaking one and is large; if it cannot reach a green gate as a
single commit, stop and ask rather than committing a broken tree or inventing a
sub-split the spec does not describe.

### Why phase 5 (dry run) precedes phase 6 (release)

**Module tags are immutable.** Once `v0.35.0` is pushed it cannot be corrected,
only superseded — and a defect found while migrating go-kite or go-charts would
burn a `v0.35.1` across all five siblings. go-kite is the canonical decision-11
case and go-charts owns its own `idScrollHash` helper, so those two repos are
precisely where a spec defect surfaces.

Phase 5 migrates both **against the unreleased go-gui**, using a local
resolution rather than a published version:

```fish
go mod edit -replace github.com/go-gui-org/go-gui=../go-gui
go build ./... && go test ./...   # NOT GOWORK=off — local resolution is the point
```

Do **not** commit the `replace`, do not merge, do not tag. The branches are held
and reused by phases 7–8. If phase 5 fails, the fix goes into go-gui **before**
the tag exists — that is the entire point.

### Why migration and bump are one commit (phases 7–8)

For go-kite and go-charts the two cannot be split, in either order:

| order                     | result                                                                              |
| ------------------------- | ----------------------------------------------------------------------------------- |
| bump first, migrate after | `go.mod` points at `v0.35.0`; `IDScroll` is gone; repo does not compile             |
| migrate first, bump after | migrated code needs `Scrollable`; `v0.34.x` does not have it; repo does not compile |

So the bump rides in the migration PR. **`/sync-siblings` cannot drive these
two** — its phase 3 is `go get` → `go mod tidy` → `go build`, with no step that
edits call sites, and its red-CI triage has no category for "the upstream API
changed." It would `go get` the breaking version and stall on compile errors. It
drives phase 9 only, where the three zero-ref repos are pure bumps and that flow
fits exactly.

Phases 7–8 drop the phase-5 `replace`, add the real require, and gate with
`GOWORK=off` (go-kite has a gitignored `go.work` that would otherwise mask the
published-module resolution CI uses).

### go-charts also pins go-gui in CI (easy to miss)

`go-charts/.github/workflows/ci.yml:27` and `gallery.yml` check out go-gui at a
**hardcoded `ref: v0.34.0`** and `go mod edit -replace` onto it. That pin
overrides `go.mod`, so bumping the require alone leaves CI building the migrated
source against a go-gui that still has `IDScroll` — red, with a confusing error.
**Phase 8 must bump both `ref:` pins to `v0.35.0` in the same PR.**

Consequence: go-charts CI cannot verify phase 5 (the workflow needs a real tag).
Its dry run is local-only, which is why phase 5 exists rather than trusting CI
to catch it. Note also that this pin means go-gui's `main` advancing does
**not** red go-charts — do not expect that signal.

## PR A (phase 1) — delete `IDScrollContainer` (independent, do first)

Pure dead-code removal, no design dependency on the rest of this spec.

- `gui/shape.go:70-72` — remove field + doc comment.
- `gui/layout_position.go:193-205` — remove `layoutScrollContainers` entirely.
  It walks the **whole tree every frame** and its only output is the unread
  field.
- `gui/layout_pipeline.go:31` — remove the call.
- `gui/layout_test.go:334-359` (`TestLayoutScrollContainersNearestScrollParent`)
  and `gui/layout_position_test.go:190-201`
  (`TestLayoutScrollContainersNoScroll`) — delete. Both only assert the value
  the writer just wrote.

Verified: zero readers in go-gui outside its own writer, zero across all five
sibling repos. History — field dates from Phase 1 (`3edcac9`), last functional
touch `985a7b4` (Slider). Orphaned legacy, not scaffolding.

Effect: `Shape` 272 → 268, one full-tree pass removed from every frame.

## PR B (phase 2) — `ViewFrame` benchmark (independent, non-breaking, do first)

**Why this exists:** every current view-phase benchmark pre-builds its `Shape`s
in the fixture outside the loop (`benchViewFlat` / `benchViewNested`,
`gui/view_bench_test.go:28-70`) and only allocates `Layout{Shape: &v.shape}`
pointers in the hot loop. The suite therefore **cannot see a `Shape`-size
regression at all** — a flat `B/op` from those benches is an artifact, not
evidence. Without this bench, PR C's "−8 bytes" claim is unverifiable in CI.

**Do not "fix" the existing benches instead.** They measure something real
(layout generation over a stable tree); this is an additional bench, not a
replacement.

Add as `gui/view_frame_bench_test.go` and add `ViewFrame` to the `bench-gate`
regex in the `Makefile` (target `bench-gate`, ~line 86):

```go
package gui

import (
	"strconv"
	"testing"
)

// benchFrameIDs is hoisted so the bench measures view construction, not
// strconv.
var benchFrameIDs = func() []string {
	ids := make([]string, 256)
	for i := range ids {
		ids[i] = "row-" + strconv.Itoa(i)
	}
	return ids
}()

// buildFrameView constructs a widget tree through the public factories,
// exactly as a per-frame view function would. Unlike benchViewFlat, the
// Shapes are allocated inside the loop, which is what makes this bench
// sensitive to sizeof(Shape).
func buildFrameView(rows int) View {
	content := make([]View, rows)
	for i := range rows {
		content[i] = Row(ContainerCfg{
			ID:      benchFrameIDs[i],
			Sizing:  FillFit,
			Padding: NoPadding,
			Content: []View{
				Column(ContainerCfg{Sizing: FillFit, Color: ColorTransparent}),
				Column(ContainerCfg{Sizing: FillFit, Color: ColorTransparent}),
			},
		})
	}
	return Column(ContainerCfg{
		ID:      "frame-root",
		Sizing:  FillFill,
		Content: content,
	})
}

// BenchmarkViewFrame builds the view tree AND generates the layout each
// iteration, which is what happens on every real frame.
func BenchmarkViewFrame(b *testing.B) {
	for _, rows := range []int{50, 200} {
		b.Run("rows_"+strconv.Itoa(rows), func(b *testing.B) {
			w := &Window{scratch: newScratchPools()}
			w.windowWidth = 1200
			w.windowHeight = 900
			b.ReportAllocs()
			for b.Loop() {
				// Mirrors window_update.go: the frame-scoped arena is
				// recycled once per frame.
				w.scratch.resetViewPools()
				view := buildFrameView(rows)
				_ = generateViewLayout(view, w)
			}
		})
	}
}
```

Reference numbers on `main` @ `3c5dac7` (M5): `rows_50` ≈ 16.3 µs, 54.34 KiB,
**353 allocs**; `rows_200` ≈ 66.5 µs, 216.2 KiB, 1403 allocs. The nonzero alloc
count is the point — if a future edit drives `B/op` to a constant, the bench has
regressed into the same blind spot.

Sanity check: with a `[24]byte` pad spiked into `Shape`, this bench reports
`B/op` +8.7% and `allocs/op` unchanged. That is the signal it exists to catch.

**Known blind spot — do not mistake this bench for full alloc coverage.**
`buildFrameView` constructs no scrollable containers, so it cannot see the
per-frame concatenation allocs PR C introduces on the suffix-derived scroll
paths (see "Per-frame allocation" under PR C). It gates `sizeof(Shape)`, which
is what it was built for. The scroll-path allocs are prevented by hoisting, by
review — not by this gate. The fixture is left alone deliberately: the reference
numbers above are measured, and editing the fixture to chase a second signal
invalidates them.

## PR C (phase 3) — `Scrollable` + string identity

### `Shape` (`gui/shape.go`)

- Remove `IDScroll uint32`; add `Scrollable bool` next to
  `Focusable`/`FocusSkip` (**adjacent placement matters** — that is what lets it
  land in existing padding for free).
- Reuse existing `Shape.ID string` as scroll identity.
- Update doc comments referencing `IDScroll`: `shape.go:65-66`, `shape.go:153`.

### Cfg classification — DO NOT swap mechanically

The `IDScroll` fields are **not all the same thing**. A blind `IDScroll` →
`Scrollable: true` pass will corrupt the reference and override cases into
self-scrolling widgets. Each is classified below; the "action" column is the
whole job.

| Cfg                   | file:line                        | kind                            | action                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                   |
| --------------------- | -------------------------------- | ------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `ContainerCfg`        | `view_container.go:98`           | **container**                   | `Scrollable bool`; identity = `cfg.ID`                                                                                                                                                                                                                                                                                                                                                                                                                                                                   |
| `ListBoxCfg`          | `view_listbox.go:48`             | **container**                   | `Scrollable bool`; container already sets `ID: cfg.ID` (`:118`, `:217`) alongside `Focusable` — same shape, both identities, different namespaces, no collision                                                                                                                                                                                                                                                                                                                                          |
| `TreeCfg`             | `view_tree.go:36`                | **container**                   | same as ListBox (`:238`)                                                                                                                                                                                                                                                                                                                                                                                                                                                                                 |
| `TableCfg`            | `view_table.go:70`               | **container ×2**                | ⚠️ two paths. Non-freeze: `outerCfg` already sets `ID: cfg.ID` (`:271`), gate at `:289` becomes `if cfg.Scrollable`. Freeze: see "Table's two paths" below — the spec's earlier draft missed it entirely                                                                                                                                                                                                                                                                                                 |
| `ComboboxCfg`         | `view_combobox.go:46`            | **container + identity handle** | dropdown already sets `ID: cfg.ID + ".dropdown"` (`:207`); `:219` becomes `Scrollable: cfg.Scrollable` — **not `true`**, see below. ⚠️ `cfg.IDScroll` is also the caller's scroll handle (decision 11) — key becomes `cfg.ID + ".dropdown"`, document it. Read at `:121` needs that string, not a bool                                                                                                                                                                                                   |
| `CommandPaletteCfg`   | `view_command_palette.go:48`     | **container, NO ID**            | ⚠️ `IDScroll uint32` → `Scrollable bool`. The scroll `Column` at `:218` has **no `ID` field at all** — must add `ID: cfg.ID + ":scroll"`. `CommandPaletteShow` (`:234`) and `CommandPaletteToggle` (`:261`) also take `idScroll uint32`; remove the parameter, derive `id + ":scroll"` internally for the scroll-reset on show (`:243`). **Behavior change:** today Show resets scroll only when `idScroll > 0`; after the param is gone it **always** resets `id+":scroll"` to 0 on show — intentional. |
| `InputCfg`            | `view_input.go:78`               | **container** (multiline only)  | gate at `:152` is `cfg.Mode == InputMultiline && cfg.IDScroll > 0`; becomes `&& cfg.Scrollable`. Threads to `inputHandlerCfg` (`:134`, `:222`)                                                                                                                                                                                                                                                                                                                                                           |
| `inputHandlerCfg`     | `view_input.go:284`              | **internal plumb**              | unexported; thread `string` through `:546`, `:644` (`inputScrollCursorIntoView`)                                                                                                                                                                                                                                                                                                                                                                                                                         |
| `DataGridCfg`         | `datagrid/view_data_grid.go:238` | **identity override**           | ⚠️ **not** a bool — **delete the field** (decision 9). See below.                                                                                                                                                                                                                                                                                                                                                                                                                                        |
| `ScrollbarCfg`        | `view_scrollbar.go:23`           | **reference**                   | ⚠️ rename to `ScrollID string` and tag with `` `gui:"required"` `` (decision 10) — points at _another_ container's state. Never becomes a bool.                                                                                                                                                                                                                                                                                                                                                          |
| `dragReorderStartCfg` | `drag_reorder.go:210`            | **reference**                   | ⚠️ unexported; `ScrollID string` (decision 10). Never a bool.                                                                                                                                                                                                                                                                                                                                                                                                                                            |
| `inspectorNodeProps`  | `inspector.go:64`                | **display copy**                | ⚠️ unexported; holds the _inspected_ shape's id for rendering (`:494`), not an opt-in                                                                                                                                                                                                                                                                                                                                                                                                                    |

**`appendScrollbar`** (`view_container.go:436`) is the bridge that wires the
container's ID into the scrollbar's `IDScroll` field. Post-migration, its
`idScroll uint32` parameter becomes `id string`, and struct-literal lines `:443`
and `:448` change `IDScroll:` → `ScrollID:` (decision 10). Called from
`container()` at `:387-390`, which changes `cfg.IDScroll` → `cfg.ID`.

**`DataGridCfg.IDScroll` is a third category.** Today
(`view_data_grid_helpers.go:124-127`):

```go
func dataGridScrollID(cfg *DataGridCfg) uint32 {
    if cfg.IDScroll > 0 { return cfg.IDScroll }   // user override
    return gg.FnvSum32(cfg.ID + ":scroll")        // derived default
}
```

It is an optional _identity override_, not an opt-in flag. Per decision 9 the
field is **deleted** and the id always derived:

```go
func dataGridScrollID(cfg *DataGridCfg) string {
    return cfg.ID + ":scroll"
}
```

That is already the exact string the container's `ID` carries at
`view_data_grid.go:584`, so the function may be inlinable at its call sites —
check whether it still earns its keep. `Scrollable: true` goes on that same
container (`:585`, replacing `IDScroll: scrollID`).

### Per-frame allocation — the migration is not alloc-neutral

An earlier draft claimed this migration "only removes per-frame work."
**Wrong.** Every `cfg.ID + ":scroll"` / `cfg.ID + ".dropdown"` is a string
concatenation, i.e. a heap allocation, evaluated in the view phase — the one
phase that is not already zero-alloc. The ledger per scrollable widget per
frame:

| site                               | today                                            | after                                  | delta      |
| ---------------------------------- | ------------------------------------------------ | -------------------------------------- | ---------- |
| DataGrid                           | concat (`ID:`) + concat-and-hash (`IDScroll:`)   | one concat, reused                     | **−1**     |
| Select / Combobox dropdown         | concat (`ID:`) + concat-and-hash / passthrough   | one concat, reused                     | **−1 / 0** |
| CommandPalette                     | none (the scroll `Column` has **no `ID` today**) | one concat                             | **+1**     |
| Table, freeze                      | none                                             | one concat, if hoisted; **two if not** | **+1**     |
| Table, non-freeze                  | none                                             | none (`cfg.ID` used directly)          | 0          |
| Container / ListBox / Tree / Input | none                                             | none (`cfg.ID` used directly)          | 0          |

Net is close to zero and none of it is on a hot path, but the `+1` rows are real
and the `+2` row is avoidable. **Rule: derive each scroll id once per view call
into a local, then thread the local.** Never call a derivation helper
(`tableScrollID`, `dataGridScrollID`) at two sites in the same view — that
doubles the alloc for nothing. The two-call trap is specifically Table's freeze
path (decision 12) and CommandPalette, where `Show`/`Toggle` derive
`id + ":scroll"` independently of the container.

`BenchmarkViewFrame` (PR B) does **not** cover this — it builds no scrollable
containers. This is a review obligation, not a gated one.

### Table's two paths (`gui/view_table.go`) — easy to miss

`Table` builds two different trees, and **the scroll container is a different
node in each**:

|                 | non-freeze (`:271-300`)                           | freeze (`tableFreezeLayout`, `:440-500`)                  |
| --------------- | ------------------------------------------------- | --------------------------------------------------------- |
| outer `Column`  | `ID: cfg.ID`, **carries `IDScroll`** (`:289-290`) | `ID: cfg.ID` (`:481`), **not scrollable**                 |
| inner `bodyCfg` | —                                                 | **carries `IDScroll`** (`:471`), **no `ID` field at all** |

The freeze path is therefore the same ⚠️ case the spec flags for
`CommandPaletteCfg`: the scrollable container has no identity to reuse and one
must be invented. Per decision 12 it becomes `ID: cfg.ID + ":scroll"`.

**The trap is `:202`, not the containers.** Virtualization reads the key once,
for both paths:

```go
scrollY, _ := w.scrollY().Get(cfg.IDScroll)  // :202, path-independent today
```

That works today because both paths pass the same `uint32`. Once identity is
derived from where the container sits, it is no longer path-independent — `:202`
must read `tableScrollID(&cfg, freeze)` (decision 12) or the freeze path
virtualizes against an empty key and renders the wrong rows. `freeze` is
computed at `:175`, above the read, so it is in scope.

Also update `:175`, `:196`, `:290` (gates) and `:471` (`bodyCfg` literal).

### Compile break vs silent break — read this before the lists

Deleting `IDScroll` makes **every** reference to it a compile error. The gate
and plumbing lists below are therefore a _convenience for estimating the diff_,
not a safety net, and they are known to be incomplete (e.g.
`view_container.go:354`, `:387`; `view_table.go:175,196,202,471`;
`view_listbox.go:187,300`; `view_tree.go:181,189`; `view_input.go:185`). A
missed site does not ship — it fails to build. Regenerate at land time rather
than trusting the lists.

**The sites that matter are the ones that still compile after a mechanical
swap.** Two shapes:

1. **Bool flip next to a key read.**
   `if cfg.IDScroll > 0 { x, _ = w.scrollY().Get(cfg.IDScroll) }` → the gate
   becomes `cfg.Scrollable` _and_ the `Get` needs the derived string. Swapping
   only the gate leaves a compile error, but swapping both without checking
   **which** string that container carries compiles and reads the wrong key.
   Sites: `view_combobox.go:120-121` (`cfg.ID + ".dropdown"`),
   `view_command_palette.go:120-121` (`cfg.ID + ":scroll"`), `view_table.go:202`
   (branches, decision 12), `view_listbox.go:300` (`cfg.ID`).
2. **Reference/override Cfgs** (`ScrollbarCfg`, `dragReorderStartCfg`,
   `inspectorNodeProps`) — already covered in the classification table.
3. **Passthrough opt-in mistaken for an unconditional one.** Two dropdown sites
   look identical and are not:

   | site                   | today                                                                                            | after                           |
   | ---------------------- | ------------------------------------------------------------------------------------------------ | ------------------------------- |
   | `view_select.go:143`   | `IDScroll: idScroll`, hashed unconditionally at `:68` — Select's dropdown is _always_ scrollable | `Scrollable: true` ✅           |
   | `view_combobox.go:219` | `IDScroll: cfg.IDScroll` — **passthrough**, scroll is the caller's opt-in                        | `Scrollable: cfg.Scrollable` ✅ |

   Writing `Scrollable: true` at `view_combobox.go:219` compiles, and makes
   every dropdown scrollable regardless of what the caller asked for — while the
   read at `:121` is gated on `cfg.Scrollable`. Gate and container then
   disagree: a combobox that did not opt in gets a scrollbar injected and scroll
   dispatch, but virtualizes against a key nothing reads. Copy the gate
   expression; do not assume a literal.

### Gates: `IDScroll > 0` → `Scrollable` (bool gate sites)

`event_handlers.go:103,343`; `gesture.go:528`; `layout_overflow.go:10`;
`layout_sizing.go:123,433`; `layout_position.go:11,68,236`;
`scroll.go:112,190,214,243`; `scroll_smooth.go:140-141`;
`view_container.go:308,384`; `inspector.go:432`; `view_combobox.go:121`;
`view_command_palette.go:120`; `view_listbox.go:144,293`; `view_tree.go:175`;
`view_input.go:152`; `view_table.go:175,196,289`.

Also in `view_container.go`, not gates but same file:

- `:354` — `IDScroll: cfg.IDScroll` inside `buildContainerShape`, the literal
  that puts the value on the `Shape`. Becomes `Scrollable: cfg.Scrollable`.
- `:387` — `deriveContainerA11YRole` maps `IDScroll > 0` →
  `AccessRoleScrollArea`. Becomes `c.Scrollable`.

**Easy to miss** (parent-walk lookups that read `Shape.IDScroll` off an
ancestor, not a Cfg):

- `view_rtf_select.go:84-85`
- `view_text_select.go:101-102`
- `scroll.go:112,120` (`scrollParent.Shape.IDScroll`)
- `scroll.go:243-244`

### Typed plumbing: `uint32` key → `string` (easy to miss)

Bool gates alone are not enough. These hold or thread the scroll key type and
will not compile until retargeted — regenerate with
`rg -n 'IDScroll|idScroll' --type go gui/` at land time:

| location                                              | what changes                                                    |
| ----------------------------------------------------- | --------------------------------------------------------------- |
| `window.go:183-184`                                   | `scrollXMap` / `scrollYMap` → `*BoundedMap[string, float32]`    |
| `state_registry.go:77-110`                            | `ScrollX`/`ScrollY`/`scrollXRead`/`scrollYRead` signatures      |
| `scroll_smooth.go:32,42`                              | `scrollSmoothEntry.idScroll`, `scrollApply.idScroll` → `string` |
| `scroll_smooth.go:115-207`                            | `entryFor` / `findEntry` / `scrollSmoothCancel` params          |
| `view_scrollbar.go:288,299`                           | `offsetMouseChangeX/Y` map + `idScroll` params                  |
| `drag_reorder.go:135,210,527`                         | cfg + runtime state + `dragReorderAutoScroll`                   |
| `view_listbox.go:118,217,537`                         | `IDScroll:` literals / locals → `Scrollable` + `ID`             |
| `view_tree.go:238`; `view_tree_rows.go:254`           | same                                                            |
| `view_combobox.go:219`; `view_command_palette.go:218` | same                                                            |
| `datagrid/view_data_grid.go:585` + helpers            | override deleted; string identity                               |

### Window API (`gui/scroll.go`, `gui/state_registry.go`, `gui/layout_query.go`)

All `idScroll uint32` → `id string`: `ScrollHorizontalBy/To/ToPct/Pct`
(`:260,277,292,309`), `ScrollVerticalBy/To/ToPct/Pct` (`:324,341,356,373`),
`ScrollX()`/`ScrollY()` → `*BoundedMap[string, float32]`
(`state_registry.go:77,84`), `FindLayoutByIDScroll` →
`FindLayoutByScrollID(layout, id string)` (decision 4; predicate gates on
`Scrollable`), `findScrollLayout` (`scroll.go:7`), `inputScrollCursorIntoView`
(`scroll.go:45`, guard `idScroll == 0` → `id == ""`).

`ScrollToView(id string)` (`scroll.go:235`) already takes a string — unaffected.

**`nsScrollX` / `nsScrollY` need no change — do not touch them.** They are
namespace _name_ strings (`state_registry.go:163-164`), not keys, and the scroll
maps are not StateMap namespaces at all: `ScrollX()`/`ScrollY()` return
dedicated `w.scrollXMap`/`scrollYMap` fields via `lazyBoundedMap`
(`state_registry.go:63-90`), which never registers into `registry.maps`. There
is nothing to rekey.

Consequence worth knowing but **out of scope**: `nsScrollX`/`nsScrollY` are in
time-travel's snapshotable whitelist (`time_travel.go:58-59`), but
`snapshotWhitelistedNamespaces` walks `registry.maps` — which the scroll maps
are absent from. **Scroll offsets are therefore not captured by time-travel
today.** Pre-existing bug, unrelated to this migration and unchanged by it (the
whitelist stays just as inert after the retype). File it separately; do not fix
it here.

### Hash derivations to delete

- `datagrid/view_data_grid_helpers.go:127` — `FnvSum32(cfg.ID + ":scroll")`
- `view_select.go:68` — `FnvSum32(cfg.ID + ".dropdown")` (feeds `:143`)
- `view_theme_picker.go:64` — `FnvSum32(lbID)`

Then check `FnvSum32` (`gui/fnv.go`) for remaining callers — #76 removed the
focus ones. If none remain it is deletable, **but it is exported API**: grep all
five siblings first.

### Inspector specifics (`gui/inspector.go`)

- `:12` — `inspectorIDScrollPanel = uint32(0xFFF00001)` magic sentinel → string
  const, e.g. `inspectorScrollPanel = "gui:inspector:panel"`. Consumed at
  `:156`, `:676`.
- `:435` — `strconv.Itoa(int(p.IDScroll))` displays the id; with a string it is
  printed directly. Compile break, trivial fix.
- `:494` — `IDScroll: shape.IDScroll` copies the inspected shape's id for
  display. Becomes the string.

## Tests / examples / docs (phase 4, go-gui)

- 21 test files reference `IDScroll` → `Scrollable: true, ID: "x"`. Regenerate
  the list with: `grep -rl IDScroll --include='*_test.go' gui/`
- 14 example files: `benchmark`, `rtf`, `markdown`, `gradient_demo`, `listbox`,
  `particles`, `command_demo`, `scroll_demo`, `multiline_input`,
  `showcase/{catalog,detail,demo_layout, demo_selection,demo_data}`. Meaningful
  IDs per decision 5.
- **Doc comments on every `Scrollable` field must state that Cfg's derived
  scroll key** (decision 11 table). This is a deliverable, not a nicety — it is
  the only place the post-migration scripting contract is written down.
- Docs: `CHANGELOG.md`, `docs/commands.md`, 10
  `examples/showcase/docs/widget_*.md` (`listbox`, `table`, `command_palette`,
  `scrollbar`, `row`, `combobox`, `gesture`, `column`, `tree`, `data_grid`),
  plus `examples/showcase/docs/commands.md` — which is **not**
  `widget_`-prefixed and is a different file from `docs/commands.md`. Both need
  updating. 13 files total; regenerate with `rg -l IDScroll --glob '*.md'`.
- `.claude/skills/{widget,new-example}` scaffolds if they mention `IDScroll`.

## Release (phase 6)

Breaking → minor bump `v0.35.0`, CHANGELOG entry. **Batch PR A and PR C into the
same release** so siblings migrate once. PR B is non-breaking and lands
independently.

**Gated on phase 5.** Do not tag until the go-kite and go-charts dry runs build
clean against the unreleased go-gui — the tag is immutable, and those two repos
are where a spec defect surfaces. See Phase 0.

CHANGELOG must call out **two** breaking changes, not one: the opt-in retype
(`IDScroll uint32` → `Scrollable bool`) _and_ the loss of the caller-supplied
scroll handle across all eight Cfgs (decision 11), with the derived-key table so
consumers can fix their `ScrollVerticalTo` calls without reading go-gui's
source.

## Sibling repos (dependency order)

Zero-ref repos: bump → gate → ship, via `/sync-siblings` (phase 9). go-kite and
go-charts: migrate **+** bump in one PR (phases 7–8) — see "Why migration and
bump are one commit" under Phase 0. `/sync-siblings` does not migrate API call
sites and cannot drive those two.

1. **go-edit** — 0 refs. Bump only.
2. **go-map** — 0 refs. Bump only.
3. **go-term** — 0 refs. Bump only.
4. **go-kite** — 2 refs, `views.go:116` (`IDScroll: timelineScrollID`) and
   `:124` (`ScrollVerticalTo(timelineScrollID, 0)`). Mechanically small, but it
   is the **canonical decision-11 case**, not a rename: the same const is both
   the opt-in and the scroll handle. The container must acquire
   `ID: timelineScrollID` (a string) with `Scrollable: true`, and the
   `ScrollVerticalTo` call must pass that same string. `RequireScrollID` catches
   the container if its `ID` is left empty; it does **not** catch an `ID` that
   simply disagrees with what `ScrollVerticalTo` passes. That case compiles and
   scrolls nothing.
5. **go-charts** — regenerate counts at land
   (`rg -n 'IDScroll|idScrollHash|ScrollVertical|ScrollHorizontal|scrollCatalog|scrollDetail'`).
   `chart/data_table.go:14,67` has its own `idScrollHash(id)` helper feeding
   `gui.TableCfg.IDScroll`; **delete the helper**, pass the string.
   `examples/showcase` uses `scrollCatalog`/`scrollDetail` consts across
   `catalog.go:29,94-96,237-238`, `detail.go:11,32`, `main.go:9-10,51-56`.

   ⚠️ **Also a CI change, not just source.** `.github/workflows/ci.yml:27` and
   `gallery.yml` hardcode `ref: v0.34.0` for the go-gui checkout they `replace`
   onto. Both pins → `v0.35.0` in the same PR, or CI builds migrated source
   against the old API. The pin overrides `go.mod`, so the require bump alone
   does not fix it.

## Verification gate (every repo)

```
go build ./... && go vet ./... && golangci-lint run ./... && go test ./...
```

Plus `make bench-gate` on go-gui, which after PR B can see the `Shape` delta.
Expect `B/op` on `BenchmarkViewFrame` to _drop_ ~3% from PR A+C (272 → 264). If
it does not move at all, the bench is not wired in.

## Out of scope

- **`BoundedMap[uint32, V]` generic penalty** —
  [#77](https://github.com/go-gui-org/go-gui/issues/77). `Get` is ~4.5× slower
  than a raw `map[uint32]V` (10.70 ns vs 2.37 ns) while the string instantiation
  pays nothing. Suspected GC-shape stenciling dropping the lookup off the
  runtime's `fast32` path — **effect measured, mechanism unverified**. Affects
  every uint32-keyed namespace.

  **Not a prerequisite. #77 does not block this spec, and this spec does not
  block #77.** Do not chase it inside this work.

  Read the `Get` improvement correctly: PR C does not _fix_ this bug, it
  _sidesteps_ it by keying with a string. If #77 is fixed first, uint32 reads
  drop to ~2.4 ns and string keys become the slower choice by ~2.8 ns per read —
  a handful of containers × 2.8 ns per frame against a 4–5 ms budget, i.e.
  irrelevant. The size (272 → 264) and consistency arguments carry this spec on
  their own; **PR C must not be justified on the `Get` number.**

- Enum reorder of `ScrollMode` to add a `ScrollNone` zero value.

## Unresolved

None. Decisions 4, 7, 8, 9, and 10 incorporate the first 2026-07-15 review pass
(rename, RequireScrollID wiring clarifications, typed-plumbing checklist,
requiredid caveat). Decisions 11 and 12, the "Table's two paths" and "Compile
break vs silent break" subsections, and the go-kite note incorporate the second
2026-07-15 pass, which verified the spec's claims against `main` @ `3c5dac7`:

- `IDScrollContainer` has zero readers (sole writer `layout_position.go:200`,
  plus the two named tests). PR A confirmed.
- `RequireFocusID` has zero call sites. Decision 7's framing confirmed.
- `Shape` is **272** bytes today, and adding `Scrollable bool` next to
  `FocusSkip` is **free** (measured 272 → 272 on a spike). The
  padding-absorption claim holds; the net 264 follows from removing 8 bytes of
  `uint32`.
- `FnvSum32` retains only test callers once the three production sites go —
  deletable pending the sibling grep, as written.

A third 2026-07-15 pass re-verified against the same base and corrected four
things the second pass got wrong or oversold:

- **`IDScroll` is not a StateMap key.** The motivation said it was and the
  typed-plumbing list carried a "rekey `nsScrollX`/`nsScrollY`" item. Offsets
  live in dedicated `w.scrollXMap`/`scrollYMap`; the consts are inert. Item
  deleted. Surfaced a real pre-existing bug — time-travel does not snapshot
  scroll offsets — explicitly left out of scope.
- **Combobox.** The classification table said "just `Scrollable: true`";
  `view_combobox.go:219` is a passthrough (`IDScroll: cfg.IDScroll`), so that
  literal would have silently forced scroll on every dropdown. Now
  `Scrollable: cfg.Scrollable`, with the Select contrast written out as a third
  silent-break shape.
- **Alloc neutrality.** "This migration only removes per-frame work" was false:
  CommandPalette and Table-freeze each gain a concat. New "Per-frame allocation"
  subsection; decision 12 now mandates hoisting; PR B's blind spot documented
  rather than papered over.
- **`RequireScrollID`** was framed as catching "every missed site". It catches
  one: a bare `ContainerCfg` with `Scrollable: true` and no `ID`. Every other
  scroll container either derives a never-empty suffix id or already calls
  `RequireID`. Still worth wiring; no longer oversold.
- Also: `Window.ScrollX()`/`ScrollY()` added to the CHANGELOG's breaking list as
  a third item; decision 4 flagged as a behavior change (today's predicate has
  no opt-in gate and no zero guard); go-kite ref count corrected 3 → 2; doc file
  count corrected 11 → 13 with `commands.md` miscategorization fixed.

Measured this pass: `sizeof(Shape) == 272` on `main` @ `3c5dac7`, confirming the
272 → 264 claim's starting point.

Nothing in this spec requires a judgement call the implementer has to make alone
— if something here looks underspecified, that is a defect in the spec, not an
invitation to improvise. Decision 12 exists because the first draft _was_
underspecified on Table.

## Open risks

- Scroll identity must be unique per frame among scroll containers; duplicates
  share offsets. **Accepted, undetected** (decision 8) — usually obvious when
  scrolling; weaker under virtualization, still not worth a per-frame seen-set.
- Containers opting in with `IDScroll: 1` and no `ID` must acquire one. Wire
  `RequireScrollID` in **early** — it is the safety net that turns every missed
  site in this migration into a panic at first render rather than a silent
  no-scroll.
- `ScrollbarCfg` / `dragReorderStartCfg` / `inspectorNodeProps` are references,
  not opt-ins. A mechanical pass corrupts them.
- Typed key holders (`scroll_smooth` entries, `window` maps, `view_scrollbar`
  helpers, drag-reorder runtime state) are easy to miss if the implementer only
  greps `IDScroll > 0` — use the typed-plumbing table in PR C. Missing one is a
  **build failure**, not a silent bug; see "Compile break vs silent break".
- **The silent failures are gate-flips next to a key read** — the four sites
  listed in "Compile break vs silent break". A swap that satisfies the compiler
  while reading a key nothing writes produces a container that renders but never
  scrolls, or virtualizes against offset 0.
- **Table's freeze path** (`tableFreezeLayout`) puts the scroll container
  somewhere different than the non-freeze path does, and `:202` reads the key
  for both. Decision 12 branches; forgetting the branch mis-virtualizes
  frozen-header tables specifically, which is the configuration least likely to
  be in a smoke test.
- **Consumers lose the caller-supplied scroll handle** (decision 11). The
  derived key is recoverable from the docs, but a sibling that migrates the Cfg
  field and forgets its `ScrollVerticalTo` call site compiles and silently stops
  scrolling. go-kite is the known instance; regrep the others for
  `ScrollVertical|ScrollHorizontal` at land time, not just for `IDScroll`.
- `CommandPaletteCfg`'s scroll container has no `ID` today and needs one
  invented (`cfg.ID + ":scroll"`); Show always resets that key after the API
  shrink.
- `allocs/op` on `BenchmarkViewFrame` (PR B) must not rise after PR C. Nothing
  in this migration touches the paths that bench covers, so any movement there
  is a mistake regardless of where it came from. Note this is a **weaker** gate
  than it looks: the bench builds no scrollable containers, and the migration is
  **not** alloc-neutral on the scroll paths it cannot see (+1/frame for
  CommandPalette and Table-freeze, +2 if the derivation is called twice). See
  "Per-frame allocation" under PR C — those are caught by hoisting and review,
  not by `bench-gate`.
