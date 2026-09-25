# SegmentedControl

Issue #600.

Status: **implemented** — `SegmentedControl(SegmentedControlCfg)`, unreleased.

## The problem

An app that wants a compact single choice of two to six short options (a view
mode, a time range) had no control with that look. `RadioButtonGroupRow` has
the same semantics but draws radio dots. `TabControl` has arrow-key selection
but owns a content area. `Toggle` and `Switch` hold one boolean only.

## The design

`SegmentedControl` is a new factory with its own Cfg. The API follows
`RadioButtonGroupCfg`: `Value`, `Items`, `Options`, and
`OnSelect(string, EventCtx)`. An app can replace one control with the other and
change only the type names. `SegmentOption` adds `Icon` and a per-segment
`Disabled`.

The control is a track with an inset of 2px. The selected segment is its own
rounded fill (the pill) inside the track. The pill radius is the track radius
less the inset, so the corners are concentric. A divider stands between two
segments. A divider that touches the pill is transparent, and it keeps its
width, so the segments do not move when the selection moves.

The pill is the accent (`ColorSelect`), and its label is `ColorTextOnSelect`.
This is the same fill and the same label color as a selected tab (#741, #373).
The track takes the field fill (`ColorInterior`). Each segment is a `Button`
with the unexported `selected` flag, so hover and press come from the same
`ColorSet.pick` path as a tab.

The track is the one tab stop. The segments set `FocusDisabled`. A click on a
segment calls `OnSelect`, then moves the focus to the track: the segment
consumes the press, so the track never sees the press that focuses it. The keys
use `tabControlOnKeydown` without a change: Left/Right and Up/Down move and
wrap, Home/End jump, Space/Enter send the current value again, and disabled
segments are skipped.

A disabled control still shows its selection. The pill takes the dimmed accent.

The height of a segment is `Theme.PaddingField` less the inset, above and below.
The control is then as high as an `Input` (31.6px at body 14), and the two share
a row.

`Sizing: FillFit` makes each segment fill. The fill pass grows the smallest
segment first, so the segments become equal when the space is sufficient. A
segment does not become smaller than its content.

The style is `segmentedControlStyle`. It is unexported, like `buttonStyle`
(#735). `segmentedControlStyleFor` in `gui/theme_maker_segmented.go` builds it,
and `WithColors` moves its colors.

## Rejected Approaches

- **A style variant of `RadioButtonGroup`**
  (`RadioButtonGroupCfg.Style: RadioGroupSegmented`). It adds fields to
  `RadioButtonGroupCfg` that do nothing in the radio style. A field that silently
  does nothing is the failure that the ergonomics audits exist to stop.
- **A per-cell event helper with a default skin** (go-shirei
  `ProcessSegmentEvents`). It fits an immediate-mode builder, not the
  zero-initializable Cfg factories of go-gui. The caller owns the chrome and can
  easily forget arrow keys, focus, and a11y.
- **Flush fill with rounded outer corners only** (Material 3, shirei, GTK). The
  renderer has one radius per rect (`RenderCmd.Radius`) and rectangular clips
  only. This look needs a radius per corner in `RenderCmd` and in the Metal and
  GL shaders.
- **Flush fill with square ends.** It needs no renderer change, but it looks
  dated beside the rounded controls of every theme.
- **A neutral raised pill** (Apple: grey track, white pill). The goldens showed
  that it fails: in `ThemeLight`, `ColorPanel` and `ColorInterior` are both
  white, so the pill cannot be seen. In `ThemeDark`, the panel is darker than the
  interior, so the pill looks sunken. The two ladders go in opposite
  directions, and each platform preset has its own ladder.
- **The name `ButtonGroup`.** In Bootstrap and MUI, a button group is a set of
  action buttons with no selection. `SegmentedControl` is the name that Apple
  and go-shirei use for a single-select control.
- **Multi-select and a vertical orientation.** These are out of scope. A
  multi-select row is a set of toggles.
