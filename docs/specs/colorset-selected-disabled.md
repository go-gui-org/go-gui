# Spec: `ColorSet` selected and disabled slots, exported `Pick`

Issue: #741

Status: **implemented**. Part 1 (the `Selected` and `Disabled` slots, tabs and
breadcrumbs) merged in #742. Part 2 (exported `Pick` and `Resolved`, data grid
rows, list rows) is on branch `colorset-pick-export-741`, not yet released.

## Motivation

`ColorSet` had slots for hover, press and focus, but none for selected or
disabled. Widgets that have a selected or disabled element kept flat fields for
them (`ColorTabSelected`, `ColorCrumbDisabled`, `ColorRowSelected`,
`listCoreCfg.ColorSelected`). Each of these widgets put the states in order by
hand. The tab helper `tabFlatFill` put the selected color in every fill slot, so
a selected tab did not react to hover or press. #720 recorded this gap.

The data grid is in a subpackage (`gui/datagrid`). It could not call the
unexported `pick`, so it had its own copy of `resolved` (`dataGridResolveSet`)
and its own hover rule: a selected row did not react to hover.

## Decision

1. **`ColorSet.Selected`** is the resting fill of a selected element. `pick`
   derives the selected hover and press fills from it with the OKLCH lightness
   step of the accent ramp (#732). A selected element with focus keeps
   `Selected` as its fill and gets the focus border. Selection does not change
   the border rule.
2. **`ColorSet.Disabled`** is the fill of a disabled element. When it is set, it
   **replaces** the renderer's half-alpha dim for the fill. The border and the
   text still dim. `layoutDisables` applies it, so it also works when the
   disabled flag comes from an ancestor. When it is unset, the old rule stays:
   `Base`, dimmed.
3. **`ColorSet.Pick(PickState)`** and **`ColorSet.Resolved(theme)`** are the
   exported form of `pick` and `resolved`. A widget outside `gui/` uses them to
   get the same state order as a widget inside it. `PickState` has five flat
   bools (`Disabled`, `Selected`, `Pressed`, `Focused`, `Hovered`).
4. Data grid rows: `DataGridCfg.ColorRowSelected` and `ColorRowSelectedSubtle`
   are removed. The selected wash is `ColorsRow.Selected`. The row fill goes
   through `Pick`, so a selected row now reacts to hover. `dataGridResolveSet`
   is replaced by `Resolved`.
5. List rows (`listCoreCfg`, used by combobox and command palette): the three
   flat colors are replaced by one `ColorSet`. The keyboard highlight is the
   `Focus` slot. The pointer wins over the highlight, and a selected row keeps
   `Selected` under the highlight.

`ColorSelect` as an accent (the checked indicator of radio, toggle, switch) and
`ColorTextOnSelect` stay flat. They are not interaction state, and `ColorSet`
holds fill and border only.

## Rejected Approaches

- **Separate slots for each combination (`Selected`, `SelectedHover`,
  `SelectedClick`, `Disabled`).** A new slot for each combination. An unset
  `SelectedHover` falls back to `Selected`, so the hover change disappears
  without a warning.
- **A nested `Selected *ColorSet`.** It makes a value type recursive, a nil
  pointer means "not themed", and a deep nesting is hard to read.
- **`Disabled` used with the dim, not in place of it.** The tab and crumb themes
  need an exact disabled color. A dim on top of it halves a color the theme
  already chose.
- **`Colors` and `Selected` on `ContainerCfg`, with `pick` run by the
  framework.** It makes every Row and Column an interactive widget to fix one
  row fill. It does nothing without an ID and gives no warning.
- **A data-grid-only rename that keeps the order written by hand.** It keeps a
  second copy of the order rule, which is the defect this issue exists to
  remove.
- **`PickState` with an embedded `InteractionState`.** A literal with embedded
  fields cannot name them directly (`PickState{Hovered: true}` does not
  compile). An `OnHover` callback knows only that the pointer is over the
  element and has no `InteractionState` to embed.
