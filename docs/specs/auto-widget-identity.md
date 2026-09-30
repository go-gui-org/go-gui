# Spec: auto widget identity (kind-scoped positional key)

Issue: [#881](https://github.com/go-gui-org/go-gui/issues/881)

Status: **implemented, not released.** Written 2026-09-30. Revises Decision 1 of
[`widget-id-scoping.md`](widget-id-scoping.md) and the "tree position" row of
[`widget-id-per-scope-uniqueness.md`](widget-id-per-scope-uniqueness.md).

## Problem

Every stateful widget needed an explicit `ID`. `RequireID` and its two helpers
(`gui/state_registry.go`) panicked when the ID was empty. 30 factories called
them, 33 Cfg fields carried the `gui:"required"` tag, and a scrolling or
overflowing container also needed an ID. Most views keep the same structure for
the life of the app. For those views, the ID is only ceremony.

## Why the earlier rejection is revised

The earlier specs rejected IDs from tree position. Their reason is correct: a
position key moves state to the wrong widget, and nothing reports it. If a row
goes in at the top of a list, each row below gets the cursor, selection, scroll
offset and focus of the row above it.

The earlier specs cite Dear ImGui and egui, which use explicit IDs. React,
Flutter and Compose use a position key plus an explicit key, and that model
works. The fault to correct is the silent migration, not the position key. This
spec keeps the position key, limits how far a shift can go, and reports the
shift that does the most damage.

## Design

### The auto leaf

A widget with an empty `ID` gets an auto leaf during layout generation
(`gui/id_auto.go`). The leaf has three parts:

1. The **scope**: the nearest ancestor with an explicit ID. The current join
   supplies this part.
2. The **kind**: a constant string for each factory (`"input"`, `"slider"`,
   `"scroll"`). A factory name separates more than `AccessRole` does: Toggle,
   Switch and Checkbox share a role, and a container has none.
3. The **count**: the number of earlier widgets of the same kind in the same
   scope, in pre-order.

The leaf text is `~<kind><count>`, for example `~input3`. The leaf has no
`IDSep`, so it joins like any other leaf. The `~` prefix is reserved for auto
leaves. A Cfg field that takes an auto leaf carries the tag `gui:"auto"`.

### Where the leaf is assigned

The widget reads its state in `GenerateLayout`, and some factories capture
`cfg.ID` in handlers and scrollbars when they build. Thus the leaf must exist
before the factory body runs. Each factory starts with this branch:

```go
if cfg.ID == "" && !cfg.FocusDisabled {
	return ViewFunc(func(vw *Window) View {
		cfg.ID = vw.autoLeaf("input")
		return Input(cfg)
	})
}
```

The condition is the old requirement: `&& !cfg.FocusDisabled` where the ID was
required only for focus. `InputDate` and `ExpandPanel` drop that part: they name
inner parts from their own ID, so two ID-less copies with `FocusDisabled` would
share the bare inner IDs (`"calendar"`, `"head"`). A call with an explicit ID
keeps the old path and has no new allocation. The container factory
(`container()`) uses the same branch for `Scrollable` and `Overflow`.
`buildContainerShape` keeps a fallback for a `containerView` built past the
factory.

After this step the leaf is in `Shape.ID`. `stampEffID`, `joinLeaf` and
`idKey()` work without change. The focus, scroll and StateMap stores stay
string-keyed.

### An auto leaf opens no scope

A shape with an explicit ID opens a scope for its children. A shape with an auto
leaf does not (`childScopeID`, `resolveFocusOwnersWalk`, `ctx.EffID`). If it
did, an explicit ID below an ID-less scroll container would resolve to
`~scroll0:name`. That key is position-dependent, and `SetFocus("name")` would
stop reaching it.

The result is a rule for composites: build inner IDs with
`ScopeID(w.EffID(cfg.ID), part)`, never a relative leaf. Two ID-less instances
in one scope would otherwise collide. `ThinkingOrb` and `ThinkingOrbLabel` broke
this rule and were corrected. For an explicit ID the new inner IDs are the same
strings as before.

### Counters

`viewState` holds `autoCounts`, a map from (scope, kind) to the next count.
`updateLocked` clears it once for each frame (`resetAutoIDs`), before the root
view is generated. Overlays that arrange injects later continue the same
counters, so a dialog and the main tree cannot both claim `~button0` at the top
level. `clear` keeps the map buckets, so a frame after the first allocates
nothing here.

`Form` keys its field registry and `FormSummary` on the bare ID, and builds the
absolute layout ID `form:<id>`. Two ID-less forms in two panels would both be
`~form0`. Thus `Form` uses `autoLeafWindow`, which counts across the whole
window. The price is a weaker firewall for forms: a form inserted anywhere
earlier shifts the key of each form after it.

### Allocation

A per-window cache (`autoLeaves`) holds the leaf strings, indexed by kind and
count, up to 4096 entries. `joinLeaf` already caches the join of scope and leaf.
After the first frame, `autoLeaf` makes no allocation
(`TestAutoLeafSteadyStateDoesNotAllocate`). The deferred `ViewFunc` is one
closure allocation for each auto widget for each frame. This cost applies only
when the caller omits the ID.

### Properties

- **Other kinds do not shift the key.** An error `Text` or a spinner that
  appears above a form does not change the keys of its Inputs.
- **An explicit ID on a container is a firewall.** Its children count from zero
  in a new scope, so a shift outside it cannot reach them.
- **An explicit ID on a widget always wins.** This is the escape hatch.
- **Auto IDs are not addresses.** `SetFocus`, `FindByID` and `Test*` accept an
  auto key, but the key moves when the layout does. Code that names a widget
  gives it an explicit ID.
- **The change only adds behavior.** Code that sets IDs today does not change.
  Sibling repos need no change.

### Debug checks

New debug category `DebugAutoIDs`, in `DebugAll` (`gui/debug_auto_id.go`).

The **shift check** runs in the frame audit, after the `DebugUnknownFocus`
check:

1. If no segment of the focused key is an auto leaf, the check stops.
2. The check makes a fingerprint of the shape that holds the key: its role and
   an FNV-1a hash of its a11y label.
3. If the key is the same as in the last frame and the fingerprint is different,
   the check reports "auto identity shifted".
4. The check keeps the key and the fingerprint for the next frame.

The fingerprint does not use the value text, because the value changes while the
user types (`TestAutoIDValueChangeIsNotAShift`). A widget without a label has
only its role in the fingerprint. For that widget, the check cannot see a shift
to another widget of the same kind.

The **reserved-prefix check** reports an app ID whose last segment starts with
`~` and is not a leaf that `autoLeaf` made.

## Decisions

1. Every `gui:"required"` ID in `gui/` becomes `gui:"auto"`, except the two
   below. This includes the composites: Table, Tree, VirtualList, Form,
   Combobox, DatePicker, ListBox, ColorPicker, Menubar, ContextMenu,
   CommandPalette and OverflowPanel. `ExpandPanelCfg.ID`, which was never
   required, also becomes `gui:"auto"`.
2. Kept `required`: `InputGroupCfg.ID`, because the group ID is the scope that
   names its segments (`login:email`) and an auto leaf opens no scope; and
   `datagrid.DataGridCfg.ID`, because a subpackage cannot reach the unexported
   `autoLeaf`, and adding an exported seam is new API surface outside this
   issue.
3. Scrolling and overflowing containers take auto IDs. `requireScrollID`,
   `requireOverflowID` and `requireFocusID` are deleted.
4. Auto-ID widgets join Tab traversal. A form without IDs gets focus from Tab
   and from a click.
5. The shift check reads only the focused key.
6. An opt-in `Focusable: true` container still needs an explicit ID. The code
   that sets `Focusable` usually also calls `SetFocus`, which needs a name.
7. `RequireID` stays exported. `datagrid` and sibling repos call it.
8. `tools/requiredid` loses the Scrollable-without-ID rule. It keeps the tag
   rule, which now reaches only `required` fields, and the Focusable-without-ID
   rule. `ergonomics-audit -mode focus` counts `gui:"auto"` as guarded.

## Rejected Approaches

| Approach                                        | Why rejected                                                                                                                                                               |
| ----------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Child index path                                | Each inserted sibling shifts every key after it, and nothing reports the shift. The earlier specs rejected this, and that rejection stands.                                |
| Call site (`runtime.Callers` PC, Compose-style) | About 200 ns for each widget, about 1 ms for each frame at 5000 widgets. Factories that one helper wraps all share one PC. Inlining moves PCs.                             |
| `AccessRole` as the kind                        | Toggle, Switch and Checkbox share a role, and a container has none. A factory-name constant separates more kinds at the same cost.                                         |
| Auto leaf that opens a scope                    | An explicit child of an ID-less container would get a position-dependent key, and code could no longer name it.                                                            |
| Per-scope counter stack pushed in generation    | Overlays start a new root generation. A stack reset there gives a dialog and the main tree the same top-level leaves. A (scope, kind) map cleared once per frame does not. |
| Detector on every stateful auto key             | Needs a bounded map and a walk each frame. A focus shift corrupts data, because keystrokes go into the wrong field. A scroll or selection shift is visual only.            |
| Detector on same-kind count for each scope      | Reports each legitimate dynamic list.                                                                                                                                      |
| Warning on a public lookup with an auto key     | A test or a view function can read a key back and use it in the same frame, which is correct. The shift check already reports the failure that matters.                    |
| A new numeric key type in the stores            | Changes every store and every public API that takes an ID. A string leaf reuses the current join, cache and stamp path.                                                    |
| Removing the tag instead of `gui:"auto"`        | `ergonomics-audit -mode focus` reads an untagged focus ID as unguarded and flags every ID-less call site. The tag also tells a reader that an empty ID is handled.         |

## Verification

- `gui/id_auto_test.go`, written before the code:
  - The keys stay the same across frames.
  - A `Text` inserted above Inputs does not move focus.
  - An explicit-ID container stops a shift.
  - An explicit child of an ID-less scroll container keeps its unscoped ID.
  - A same-kind insert above the focused widget makes `DebugAutoIDs` report,
    through `w.TestFindings`. A value change does not.
  - Tab goes through auto-ID inputs in order.
  - `autoLeaf` makes no allocation after the first frame.
  - An explicit ID replaces the auto leaf.
  - A user ID with the `~` prefix gets a warning.
  - Every converted widget, twice in one scope and once in each of two panels,
    has no duplicate ID (`TestAutoIDEveryWidgetTwiceHasNoDuplicates`). The same
    sweep runs with `FocusDisabled`
    (`TestAutoIDFocusDisabledTwiceHasNoDuplicates`).
- `TestFocusWidgetsWithoutIDTakeAutoLeaf` and
  `TestFormWithoutIDCountsWindowWide` replace the tests that asserted the old
  panics.
- The golden tests (`gui/golden_test.go`) do not change, because an auto ID has
  no effect on rendering.
- `go test ./tools/requiredid/` passes with the new testdata.
