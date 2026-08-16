# Widget visual consistency — audit

Issue #335, step 1. This document is **measurements, not proposals**: the
evidence the theme-role work is justified from. Every row cites a file and line
so a later reader can check whether the finding still holds.

## Problem

Nothing in the repo tells an author how a control should look. There is no named
role for "secondary text", no tier for "the padding a field puts around its
text", and no rule for where a field's label goes. So every visual decision was
made independently, at each widget, by whoever wrote it — and the widgets
diverged without anyone being careless.

This surfaced while building `ColorFields`, whose channel label shrinks two type
steps and dims to 70% alpha. Both numbers were invented on the spot, and both
are defensible in isolation. There was nothing to copy.

The fix is not a prose style guide. That is the pattern this repo already
rejected: issue #300 deleted ~30 literal style mirrors because a second source
of truth drifts silently, and a document telling authors to write `160` is
exactly such a second source. The fix is **named roles and tiers in
`ThemeMaker`** that widgets read, so the decision cannot be re-litigated per
widget.

The audit below covers seven axes. Notably, **dimming — the axis the issue was
filed about — is the smallest of them.** The three with the largest effect on
how the toolkit reads are missing interaction states (§6), form density (§7),
and the absent field-label affordance (§3).

## 1. Secondary-text dimming — three mechanisms, seven values

Three unrelated notations express the same idea: a raw `RGBA(c.R, c.G, c.B, N)`,
`Color.WithOpacity(f)` (`gui/color.go:70`), and `dimAlpha`
(`gui/render_helpers.go:10`, which halves alpha).

| Effective  | Role                                     | Site                                        |
| ---------- | ---------------------------------------- | ------------------------------------------- |
| 0.31 (80)  | date cell, adjacent month                | `gui/view_date_picker_calendar.go:220`      |
| 0.31 (80)  | roller item, distance > 1                | `gui/view_date_picker_roller.go:207`        |
| 0.39 (100) | placeholder (Input/Select/Combobox)      | `gui/theme_maker.go:47`                     |
| 0.39 (100) | InputDate placeholder — re-derived       | `gui/view_input_date.go:333-336`            |
| 0.39 (100) | date cell, disabled                      | `gui/view_date_picker_calendar.go:141`      |
| 0.50       | any shape carrying `Disabled` (10 sites) | `gui/render_helpers.go:10`                  |
| 0.50       | **live** menu shortcut hint              | `gui/view_menu_item.go:106`                 |
| 0.51 (130) | tab, disabled                            | `gui/theme_maker.go:337`                    |
| 0.51 (130) | breadcrumb, disabled                     | `gui/theme_maker.go:366`                    |
| 0.51 (130) | inspector help text                      | `gui/styles.go:248`                         |
| 0.55 (140) | command-palette detail                   | `gui/theme_maker.go:431`                    |
| 0.55 (140) | datagrid indicator                       | `gui/datagrid/view_data_grid_helpers.go:19` |
| 0.59 (150) | roller item, adjacent                    | `gui/view_date_picker_roller.go:205`        |
| 0.63 (160) | breadcrumb separator                     | `gui/theme_maker.go:370`                    |
| 0.63 (160) | calendar weekday header                  | `gui/view_date_picker_calendar.go:45`       |
| 0.70 (178) | color-field label                        | `gui/view_color_fields.go:116`              |

Seven distinct values covering four semantic roles. Two of them
(`view_input_date.go:333`, `theme_maker.go:47`) are the _same_ role expressed
twice, one a hand-rolled copy of the other.

### 1.1 `dimAlpha` is a renderer-level path, not a theme value

`dimAlpha` is reached from `render_layout.go` (fills, borders),
`render_text.go`, `render_svg.go` and `view_container.go`. It is per-shape and
does **not** propagate to children.

**Verified, not assumed:** a `Text` inside a `Disabled` `Button` is _not_
double-dimmed. `Button` stamps `Disabled` on its own container shape only
(`gui/view_button.go:165`); the `Content` children carry `false`; and
`render_text.go:51` tests the text shape's **own** `Disabled`. Probed directly
on a disabled `Button` wrapping a themed-disabled `Text`: container
`disabled=true`, text child `disabled=false`.

So `dimAlpha` and the themed `textStyleDisabled` alphas apply to **disjoint**
shapes. They are complements, not duplicates — which means a `TextStyleDisabled`
role sits _beside_ `dimAlpha` rather than replacing it, and unifying the themed
values does not make tab or breadcrumb text lighter.

The one genuine misuse is `gui/view_menu_item.go:106`, which applies the
**disabled** dim to a **live** shortcut hint. A live control and a dead one
render identically there.

### 1.2 Out of scope

These are ramps and non-text alphas, not de-emphasis roles, and a later gate
must exempt them: the roller distance ramp
(`gui/view_date_picker_roller.go:204-211`), the math spinner ghost
(`gui/view_math_spinner.go:283,304`), and every non-text alpha — command-palette
scrim (`gui/theme_maker.go:434`), markdown block backgrounds, drag ghost,
selection highlight, swatch edge (`gui/view_color_swatch.go:64`).

## 2. Type-size steps — three parallel systems

The ladder (`gui/styles.go:27-35`) is tiny 10, xsmall 12, small 14, **medium
16**, large 20, xlarge 24 — non-uniform: −2, −2, −4 going down, +4, +4 going up.
`ThemeMaker` builds `N/B/i/bI/M/Icon 1..6` handles from it
(`gui/theme_maker.go:545-599`).

Three ways to reach a size coexist:

1. **Named handles** — markdown uses the full `B1..B6` ladder; `N3`/`N4`
   elsewhere. The intended path.
2. **Raw theme constants** — `guiTheme.sizeTextXSmall` (`gui/inspector.go:161`,
   `gui/inspector_props.go:57,109`).
3. **Inline arithmetic** — `Size-4` floored at **10**
   (`gui/view_color_fields.go:90-93`) and `Size-4` floored at **8**
   (`gui/view_input_numeric.go:230`). One conceptual step, implemented twice,
   with two different floors.

Two structural findings:

- **`gui/view_dock_layout.go:343` reads the package const `sizeTextSmall`
  directly**, bypassing the theme entirely. A theme that shifts its size scale
  cannot move it. This is a bug, not a style divergence.
- **`N5`, `N6` and `i4` are unexported** (`gui/theme.go:73,74,84`), so an app
  cannot spell the same step its own widgets sit next to.
- The mono ladder adds **+1 at every rung** (`gui/theme_maker.go:584-589`), so
  `M4` (15) does not share `N4`'s baseline (14). Deliberate optical
  compensation, but undocumented and invisible at the call site.

## 3. Label placement — there is no field-label concept

Eight field widgets have **no `Label` field at all**: `Input`, `Select`,
`Combobox`, `Slider`, `NumericInput`, `DatePicker`, `InputDate`, `ColorPicker`.
An app that wants a labelled form builds the label itself, which means every app
invents its own form layout.

Where labels do exist, three unrelated conventions:

| Convention       | Widgets                 | Detail                                |
| ---------------- | ----------------------- | ------------------------------------- |
| Trailing, beside | Switch, Toggle, Radio   | three different styles/paddings       |
| Inside, as hint  | Input, Select, Combobox | placeholder — vanishes once typed     |
| Above, centred   | ColorFields alone       | bespoke 2px spacing, own alpha + size |

Even the one convention that repeats is not shared code: `Switch` uses
`cfg.TextStyle` (`gui/view_switch.go:95`), `Toggle` a separate `textStyleLabel`
that `ThemeMaker` sets to the _same_ value (`gui/theme_maker.go:161`), and
`Radio` alone adds a gap padding (`gui/view_radio.go:59`).

`A11YLabel` fallbacks mean the **accessible** name is covered even where the
visual label is not. That is worth stating precisely: this is a visual gap, not
an accessibility one.

## 4. Spacing tiers — `SpacingLarge` is dead

`SpacingSmall 5 / SpacingMedium 10 / SpacingLarge 15` (`gui/styles.go:38-43`).
Usage inside `gui/`: Small ~10 sites, Medium ~6, **Large zero**.

The related-vs-unrelated distinction the tiers imply does not hold. `Small`
covers both a tightly related pair (the ColorFields channel row) and unrelated
stacks (Toast, RadioButtonGroup).

About ten spacings bypass the ladder entirely: `spacingHeader: 2`,
`spacingSubmenu: 1`, `cellSpacing: 2`, toast `8`
(`gui/theme_maker.go:333,455,463,230`), `blockSpacing: 12`, `nestIndent: 16`.

## 5. Borders — the healthiest axis

21 styles inherit `cfg.SizeBorder`. Two outliers only:
`gui/view_date_picker_calendar.go:153` uses `SomeF(2)`, ignoring
`sizeBorderDef = 1.5`; `gui/view_color_swatch.go:53` has its own
`colorSwatchBorder = 1`.

Recorded for completeness. No action proposed.

## 6. Interaction-state coverage

`ColorSet` (`gui/color_set.go`) is the intended abstraction for per-state
colors. It reached **5 of ~18** interactive widgets — Button, Radio, Toggle,
DatePicker, InputDate. Eleven still carry the flat `Color*` fields it replaces,
and ~10 sites build an inline `ColorSet{...}` without calling `.resolved()`.

`ThemeMaker` tally: **21** styles define a hover color, **19** a click color,
only **14** a focus color.

| Widget                  | Missing                                       |
| ----------------------- | --------------------------------------------- |
| ColorPicker family (4)  | **no hover, no press** — focus is border-only |
| ListBox, Table          | **no focus** — keyboard users get no caret    |
| ExpandPanel, Breadcrumb | no focus                                      |
| Combobox, Menubar, Tree | no active                                     |
| Scrollbar               | no focus, no active                           |

The ColorPicker row is the worst: `ColorPlane`, `ColorWheel`,
`ColorChannelSlider` and `ColorSwatch` are all focusable and draggable
(`gui/view_color_plane.go:81`, `gui/view_color_wheel.go:88`,
`gui/view_color_channel_slider.go:139`) and give **no hover or press feedback at
all**.

The ListBox row is the most tractable: `ListBoxStyle.ColorBorderFocus` already
exists and is commented _"Reserved for future focus-ring styling"_
(`gui/styles_widget_overlay.go:50`). `Shape.ColorBorderFocus` is the established
mechanism, and `gui/theme_colors.go:54-88` already fans a `borderFocus` value
out to nine styles.

This is the axis with the largest effect on perceived quality, and the ListBox
and Table gaps are an accessibility defect: a keyboard user has no indication of
where focus is.

## 7. Density — five text insets

The padding a text-bearing control puts around its text:

| Inset | Widgets                                                                   |
| ----- | ------------------------------------------------------------------------- |
| 4     | Input, NumericInput, ColorFields (`paddingTwoFour`)                       |
| 5     | ListBox, Tree, Menubar, Table, DataGrid, CommandPalette, Select, Combobox |
| 6     | Button, Badge                                                             |
| 8     | DockLayout tab                                                            |
| 16    | TextButton (`gui/view_button.go:112`) — 2.7× the theme's Button           |

The consequence is visible in any ordinary form: **Select (5) sits beside Input
(4) and the two do not match.**

Most consequential finding in this section: **Input's theme padding is dead
code.** `gui/theme_maker.go:95` sets `InputStyle.Padding` from `cfg.Padding`,
but `gui/view_input.go:300-302` falls back to a hardcoded `paddingTwoFour`
instead of `d.Padding`. A theme author editing `cfg.Padding` sees Container and
ListBox move while Input stays put.

`ColorFields` hardcodes 4/2 (`gui/view_color_fields.go:104-110`) for the stated
reason that no field-inset token is reachable.

Button (6) and TextButton (16) are noted but not counted as divergence: a button
is not a field, and `gui/view_button.go:104-107` documents its choice.

## Summary

| §   | Axis              | State                                             |
| --- | ----------------- | ------------------------------------------------- |
| 1   | Dimming           | 7 values, 4 roles, 3 notations; 1 outright misuse |
| 2   | Type steps        | 3 parallel systems; 1 theme bypass                |
| 3   | Field labels      | absent on 8 widgets; 3 conventions elsewhere      |
| 4   | Spacing           | 1 tier of 3 unused; ~10 magic values              |
| 5   | Borders           | healthy; 2 outliers                               |
| 6   | Interaction state | ColorSet at 5/18; focus missing where it matters  |
| 7   | Density           | 5 insets; Input's theme padding dead              |

The ordering that follows from this is: fix §6 and §7 first (a user sees them),
then §1 and §3 (an author trips on them), and leave §5 alone.
