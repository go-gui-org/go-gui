# Widget visual consistency — audit

Issue #335, step 1. This document is **measurements, not proposals**: the
evidence the theme-role work is justified from. Every row cites a file and line
so a later reader can check whether the finding still holds.

## Problem

Nothing in the repo tells an author how a control must look. There is no named
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

The audit below covers seven axes. Dimming — the axis the issue was filed about
— looked like the smallest until it was measured at the renderer rather than in
source: the same "disabled text" state renders at three different alphas
spanning 2× in either direction (§1.1). The other three with a large effect on
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

### 1.1 The table above is the source. The renderer disagrees with it.

`dimAlpha` is reached from `render_layout.go` (fills, borders),
`render_text.go`, `render_svg.go` and `view_container.go`. Read as source it
looks per-shape — and that reading is wrong, because `layoutDisables`
(`gui/layout.go:44`, run from `gui/layout_pipeline.go:36`) walks the composed
tree and **stamps `Disabled` onto every descendant of a disabled shape** before
rendering.

So a themed `textStyleDisabled` alpha inside a widget that also sets `Disabled`
on an ancestor container is applied and then halved again. Whether that happens
is an accident of how each widget was built.

Measured at the renderer, via the golden harness (`gui/golden_test.go`), for the
one semantic state "disabled text":

| Rendered   | Widget     | How it got there                                |
| ---------- | ---------- | ----------------------------------------------- |
| 65 (0.25)  | tab        | themed 130, then halved — ancestor `Disabled`   |
| 127 (0.50) | Input      | base 255, halved — no themed alpha at all       |
| 130 (0.51) | breadcrumb | themed 130, not halved — no ancestor `Disabled` |

**Three widgets, one state, three values, spanning 2× in either direction.**
This is a stronger finding than the source table above, and it is only visible
at the renderer: reading `theme_maker.go` suggests tab and breadcrumb agree at
130, and they do not.

It also means a `TextStyleDisabled` role cannot be dropped in. Unifying the
themed value while `layoutDisables` still halves it leaves tab at half of
whatever the role says. The role and the `Disabled` stamp have to be reconciled,
not just deduplicated.

The one unambiguous misuse is `gui/view_menu_item.go:106`, which applies the
**disabled** dim to a **live** shortcut hint. A live control and a dead one
render identically there.

### 1.1.1 The alphas do not vary by theme

Same sites recorded under `ThemeLight`: tab `#20202041`, breadcrumb `#20202082`,
Input disabled `#2020207f`, placeholder `#20202064` — byte-identical alphas to
the dark theme, over a base color of `#202020` instead of `#e1e1e1`.

De-emphasis today is a pure alpha, applied without reference to what it is
blending toward. Alpha 65 over `#e1e1e1` on a dark ground is faint but legible.
Alpha 65 over `#202020` on a light ground is nearly gone. This is the direct
evidence that a role must be a **per-theme value**, not one derived multiplier.

### 1.1.2 What the roles did and did not close

The four roles unify every site that _named_ a de-emphasis value, and the
`disabledRole` marker (`TextStyle`, `gui/styles.go`) stops the renderer halving
a color the theme has already quieted — so tab and breadcrumb now agree.

They do not reach widgets that never themed their disabled text at all. `Input`
is the example: it carries no disabled text style, so `dimAlpha` still halves
its base color to 127. On the dark theme that is within one step of the role's
128 and invisible. On the light theme the role is 149, so Input's disabled text
sits 22 alpha below the role's value — the light-theme contrast gap of §1.1.1,
surviving in the widgets the roles have not reached.

**Closed by issue #341 — the renderer now asks the theme.** `renderText`
(`gui/render_text.go`) replaced `dimAlpha` with `w.Theme().disabledTextColor(c)`
for text shapes stamped `Disabled`: the caller's hue at the theme's disabled
amount, per theme. Every disabled text shape in the package — Input, Select,
Combobox, ListBox, NumericInput, InputDate, DatePicker, Button, Switch, Toggle,
Radio, Tree, menu items, plain `Text` — renders at 128 on dark and 149 on light,
in both the direct-disabled and ancestor-disabled cases, which the per-widget
sweep the issue originally described cannot reach. The verdict on the open
question: `TextStyleDisabled` **replaces** `dimAlpha` for text. `dimAlpha`
survives for non-text surfaces — fills, borders, `renderText`'s Bg/Stroke
colors, and the group-box title eraser.

One text path never reaches the renderer's branch: the group-box title
(`addGroupBoxTitle`, `gui/view_container.go`) is a Float shape, so
`layoutRemoveFloatingLayouts` extracts it from the tree before `layoutDisables`
stamps it. It applies `guiTheme.disabledTextColor` at generation instead, and
its recording moved with the rest of the sweep.

The sweep is recorded per widget in `gui/golden_cases_test.go` (the `*_disabled`
cases, added before the fix): each moved `#7f` → `#80` on dark and `#7f` → `#95`
on light. `tab_control` and `breadcrumb` did not move — the `disabledRole`
marker still prevents a second quieting. A placeholder or secondary-role text
inside a disabled control now renders at the disabled amount rather than a
halved version of its own — a deliberate consequence of one amount for every
text in a dead control.

### 1.2 Out of scope

These are ramps and non-text alphas, not de-emphasis roles, and a later gate
must exempt them: the roller distance ramp
(`gui/view_date_picker_roller.go:204-211`), the math spinner ghost
(`gui/view_math_spinner.go:283,304`), and every non-text alpha — command-palette
scrim (`gui/theme_maker.go:434`), markdown block backgrounds, drag ghost,
selection highlight, swatch edge (`gui/view_color_swatch.go:64`).

## 2. Type-size steps — three parallel systems

The ladder (`gui/styles.go:27-35`) is tiny 10, xsmall 11, small 12, **medium
14**, large 17, xlarge 22 — non-uniform: −1, −1, −2 going down, +3, +5 going up.
The rungs derive from a per-theme body size via `textSizes` (visual-refresh
§2.1), so these are the dark/light values. `ThemeMaker` builds
`N/B/i/bI/M/Icon 1..6` handles from it (`gui/theme_maker.go:545-599`).

Three ways to reach a size coexist:

1. **Named handles** — markdown uses the full `B1..B6` ladder. `N3`/`N4` appears
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
  `M4` (13) does not share `N4`'s baseline (12). Deliberate optical
  compensation, but undocumented and invisible at the call site.

## 3. Label placement — there is no field-label concept

Eight field widgets have **no `Label` field at all**: `Input`, `Select`,
`Combobox`, `Slider`, `NumericInput`, `DatePicker`, `InputDate`, `ColorPicker`.
An app that wants a labeled form builds the label itself, which means every app
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

## 4. Spacing tiers — closed by issue #344

`SpacingSmall 6 / SpacingMedium 14 / SpacingLarge 28` (`gui/styles.go`,
refreshed with the density ladder, visual-refresh §3.3), with a lowest rung
**`SpacingTight 2`**. The tiers now state a meaning, decided once and documented
at the const block and in `docs/style-guide.md`:

- `SpacingTight` — inside one composite control, between parts that read as a
  single unit. `spacingHeader` (tab strip), `spacingSubmenu` (submenu items) and
  `cellSpacing` (calendar cells) all fold to it.
- `SpacingSmall` — members of one visual group that share a container. The
  ColorFields channel row stays on it.
- `SpacingMedium` — sibling controls in a stack. The toast stack (`8`) and
  RadioButtonGroup's default (was `Small`) migrate up to it.
- `SpacingLarge` — unrelated sections. Markdown `blockSpacing` (12) folds to it,
  giving the tier its first caller inside `gui/`.

`nestIndent` (16) is a structural indent for nested blockquotes and list depths,
not a gap between siblings. It stays off the ladder and says so in a comment.
`Theme.PaddingField` remains a separate concept: it sizes a control, the tiers
size the gaps between them.

## 5. Borders — the healthiest axis

21 styles inherit `cfg.SizeBorder`. Two outliers only:
`gui/view_date_picker_calendar.go:153` uses `SomeF(2)`, ignoring
`sizeBorderDef = 1.5`. `gui/view_color_swatch.go:53` has its own
`colorSwatchBorder = 1`.

Recorded for completeness. No action proposed.

## 6. Interaction-state coverage

`ColorSet` (`gui/color_set.go`) is the intended abstraction for per-state
colors. At audit time it reached **5 of ~18** interactive widgets — Button,
Radio, Toggle, DatePicker, InputDate. Eleven carried the flat `Color*` fields it
replaces, and ~10 sites built an inline `ColorSet{...}` without calling
`.resolved()`. **Closed by issue #342**: the eleven now carry `Colors` too, with
their flat fields retained and winning via `applyTo`, and the inline sites
resolve at construction. (Table and ColorSwatch remain unfocusable — see §6.1
and issue #345.)

`ThemeMaker` tally: **21** styles define a hover color, **19** a click color,
only **14** a focus color.

### 6.1 Two different problems wear the same label

"No focus" turned out to cover two unrelated situations, and only one of them is
a styling bug. Sorted by what is actually true in the code:

| Widget                                | State                                     |
| ------------------------------------- | ----------------------------------------- |
| ListBox                               | focusable, key-navigable, **no ring**     |
| ColorPlane, ColorWheel, ChannelSlider | focusable, draggable, **no indication**   |
| Table                                 | **not focusable** — no ID, no key handler |
| ColorSwatch                           | **not focusable**                         |
| ExpandPanel header                    | **not focusable** — has OnClick, no focus |
| Combobox, Menubar, Tree               | no active state                           |
| Scrollbar                             | no focus, no active                       |

The first two rows are the defect this audit is about: the widget participates
in the focus system, the user can tab to it, and nothing on screen says so. That
is an accessibility failure with a styling fix.

The middle rows are a different thing. `Table` has no `Focusable`, no key
handler and a transparent, borderless outer container. `ExpandPanel`'s header
row has `OnClick` and `OnChar` but never joins the tab order. Giving these a
focus ring means first designing keyboard navigation for them — a feature, and
out of scope for a consistency pass. Recording them here as _not focusable_
rather than _unstyled_ is the point: the fix is different work.

The ColorPicker controls are also the awkward case for hover and press. A
gradient surface cannot tint on hover the way a button fill can, because the
fill **is** the value being edited — so the affordance has to go on the border
or the marker.

**Correction:** an earlier draft of this section said
`ListBoxStyle.ColorBorderFocus` already existed and was merely unwired. It does
not exist. The "Reserved for future focus-ring styling" comment at
`gui/styles_widget_overlay.go:50` belongs to `DialogStyle`. The field had to be
added to `ListBoxStyle`, `ListBoxCfg`, `ThemeMaker` and the `theme_colors.go`
fan.

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

### 7.1 The inset was only half of it

Unifying the inset was not enough to make a form row line up. With Input, Select
and Combobox all on the shared tier, Input still arranged 3px taller.

The cause was invisible: Input's inner row leaves `SizeBorder` unset, so it
inherited the theme's container border of 1.5, and a container reserves space
for its border whether or not the border is painted. That row's border is
transparent, so 3px of height went to a border nobody saw.

Only the arranged geometry showed it — the configured paddings were equal and
the source read as correct. `TestFieldControlsShareHeight` therefore asserts the
_arranged_ height through the real pipeline rather than the configured padding,
which is the same lesson as §1.1: measure the render, not the source.

Button (6) and TextButton (16) are noted but not counted as divergence: a button
is not a field, and `gui/view_button.go:104-107` documents its choice.

## Summary

| §   | Axis              | State                                                                               |
| --- | ----------------- | ----------------------------------------------------------------------------------- |
| 1   | Dimming           | 7 source values, disabled text renders at 3, over a 2× spread                       |
| 2   | Type steps        | 3 parallel systems, 1 theme bypass                                                  |
| 3   | Field labels      | absent on 8 widgets, 3 conventions elsewhere                                        |
| 4   | Spacing           | 1 tier of 3 unused, ~10 magic values                                                |
| 5   | Borders           | healthy, 2 outliers                                                                 |
| 6   | Interaction state | ColorSet on 17/17 interactive widgets, focus missing where a widget cannot take one |
| 7   | Density           | 5 insets, Input's theme padding dead                                                |

The ordering that follows from this is: fix §6 and §7 first (a user sees them),
then §1 and §3 (an author trips on them), and leave §5 alone.

## Method

The source-literal findings were read from the tree at commit `b6adf899`. The
rendered values in §1.1 come from `gui/golden_test.go`, which drives the real
frame pipeline and records the emitted `[]RenderCmd` per widget per theme into
`gui/testdata/`.

That distinction is the audit's own lesson. §1.1 originally recorded the
opposite conclusion — that disabled text is dimmed once — reached by walking a
widget's `GenerateLayout` output directly. That snapshot is taken _before_
`layoutDisables` runs, so it does not show what the renderer sees. The golden
harness caught it. Any future claim in this document about what a widget looks
like must be recorded, not read.

## Resolution

What this branch changed, per axis. The measurements above describe the state at
`b6adf899` and are kept as the "before". This section is the "after".

| §   | Axis              | Outcome                                                                                                                |
| --- | ----------------- | ---------------------------------------------------------------------------------------------------------------------- |
| 1   | Dimming           | closed — four named roles, per-theme and contrast-matched. Disabled text routed through the role at render (#341)      |
| 2   | Type steps        | mostly — full ladder exported, mono +1 documented, two steps tracked                                                   |
| 3   | Field labels      | closed — `Label` on all eight, one shared convention                                                                   |
| 4   | Spacing           | closed — four tiers with stated meanings. The tight values, toast stack, and block/radio-group defaults fold in (#344) |
| 5   | Borders           | untouched, by decision                                                                                                 |
| 6   | Interaction state | closed — `ColorSet` on all 17 interactive widgets (#342). Focus rings for the four that can take one                   |
| 7   | Density           | closed — one field-inset tier, two latent bugs fixed                                                                   |

### Left open

- **§1.1.2** — closed: disabled text is themed at the renderer
  (`Theme.disabledTextColor`, unexported: downstream widgets get the role
  through the renderer and never ask), and the group-box title at generation.
  `dimAlpha` remains for non-text shapes only.
- **§2** — the export half is closed: every ladder rung is exported (the
  `N1`..`N6`, `B1`..`B6`, `I1`..`I6`, `BI1`..`BI6`, `M1`..`M6`, `Icon1`..`Icon6`
  sets), and the mono `+1` is documented at the declaration in `ThemeMaker`.
  `ColorFields`' label now steps through the `TextStyleLabel` role, so that call
  site is gone. Two arithmetic steps remain, both marked and both stepping from
  a caller-supplied size that lands on no rung: the roller's `+2` center
  emphasis, and `NumericInput`'s `Size-4` triangle. The triangle's magic floor
  `8` is gone — it is bounded by `N6.Size` below and the field text above — but
  naming the bounds does not name the step. Closing these needs a
  ladder-relative "one rung down from an arbitrary size" operation the ladder
  does not currently offer.
- **§6** — the ColorSet half is closed: all eleven flat-`Color*` widgets —
  Input, NumericInput, Select, Combobox, ListBox, Tree, Slider, ContextMenu,
  Menubar, Table, ExpandPanel — gained `Colors ColorSet` (issue #342). Additive
  and zero visual change: their flat fields survive and win over the set via
  `applyTo`, so the #335 goldens recorded no diff. The inline `ColorSet{...}`
  constructions in gui/ and gui/datagrid/ now resolve at the construction site
  rather than leaning on the receiving widget. The keyboard side is closed too
  (issue #345): `Table` (opt-in `Focusable`, arrow/Home/End movement with an
  active-row tint, Shift range under `MultiSelect`, Enter/Space activation,
  row-level like datagrid's), `ColorSwatch` (opt-in `Focusable` + `OnClick`,
  Space/Enter activate) and the `ExpandPanel` header (always a tab stop,
  Space/Enter toggle) all gained a `ColorBorderFocus` ring, and the audit's
  theme tally is 15 styles with a focus color.
- **#335 steps 3 and 4** — closed: `docs/style-guide.md` (the _when_, citing
  roles rather than values) and `ergonomics-audit -mode visual`, which gates raw
  dimming and size-step literals in `gui/view_*.go`. The §1.2 ramps and the two
  §2 size steps carry the deferred marker (`ergonomics-audit:visual`), so the
  exemptions are visible in every audit run.
