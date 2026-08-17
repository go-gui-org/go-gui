# Style guide — the when, not the what

Issue #335, steps 3–4. This document is the **when**: which named role a widget
should read, and when a deviation is allowed. It cites roles, never values.
Values live in exactly one place — `ThemeMaker` and the `default*Style` mirrors
— and re-spelling one here would be the second source of truth the audit's other
findings were measured against. If this guide says "secondary text" it means
`Theme.TextStyleSecondary`, whatever it resolves to in the installed theme.

The gate that enforces these decisions is `ergonomics-audit -mode visual`
(`make ergonomics-audit`): a literal dimming alpha or type-size step in
`gui/view_*.go` fails it unless the line carries the marker (§ Deviating).

## Text — pick the role by what the text _does_

Four roles cover every de-emphasis. Choose by the text's job, not by how quiet
it should look:

| Role                   | Use when the text…                                                        |
| ---------------------- | ------------------------------------------------------------------------- |
| `TextStyleSecondary`   | supports primary text beside it: a hint, a detail, a measurement          |
| `TextStyleLabel`       | names a nearby value (a field's label); the one role that also steps size |
| `TextStyleDisabled`    | sits in a control the user cannot act on                                  |
| `TextStylePlaceholder` | stands in for a value not yet entered                                     |

Read the role's style directly where the text color is the theme's
(`ts := guiTheme.TextStyleSecondary`); use `withRoleAlpha(base, role)` where the
hue is caller-supplied — the channel label on a user-picked color keeps its hue
and takes only the role's amount of quiet.

Never spell an alpha. An opaque color (`alpha 255`) is not de-emphasis and does
not need a role; a fade, a ramp or a fill is not text and is covered by the
marker (§ Deviating), not by a role.

## Type steps — size by named handle, or not at all

The size ladder is a set of named handles (`N1`..`N6`, `M1`..`M6`, `I1`..`I6`,
`BI1`..`BI6`, `B1`..`B6`, `Icon1`..`Icon6`). Every rung is exported — an app's
widget spells the step of the widget beside it instead of guessing. Take a step
by reading the handle, never by arithmetic on `X.Size`. A `Size ± N` that
produces a text style is a finding; two tracked exceptions carry the marker (§
Deviating).

Naming a step's _floor_ does not resolve the step. `f32Max(X.Size-4, N6.Size)`
still sizes text by arithmetic; the named rung only bounds the result. The gate
reports it either way — it keys on the arithmetic, not on how the bound is
spelled.

The mono ladder sits +1 above the roman ladder at every rung (`M4` is 15 where
`N4` is 14): an optical compensation for the mono face, uniform because it is
the same face at every size. Expect the step when mixing `M`-rungs with
`N`-rungs.

The only automatic step in the toolkit is a field label: `TextStyleLabel` steps
its own size, so a labelled form reads correctly without the widget spelling a
number.

Distinguish a type step from geometry. A row height, icon width or splitter
dimension derived from a text size (`style.Size + 4` as a width) is measurement,
not typography, and is not a finding.

## Field labels — two placements, one per control kind

- `labelledField` — the label **above** the control: form fields (`Input`,
  `Select`, `Combobox`, `NumericInput`, `DatePicker`, `InputDate`,
  `ColorPicker`). It exists so the placement and the gap are decided once, not
  per widget.
- `trailingLabel` — the label **beside** the control: boolean controls
  (`Switch`, `Toggle`, `Radio`).

The accessible name is separate: `A11YLabel`/`A11YDescription` feed the
accessibility tree. A visual label and an accessible name are both expected, and
neither substitutes for the other.

## Spacing and insets — the tiers mean relatedness

`SpacingSmall` binds a tightly related pair (a control and its readout);
`SpacingMedium` separates members of one group; `SpacingLarge` separates
sections. Prefer a tier over a magic number; a number that bypasses the ladder
is a finding on the same basis as a dimming alpha.

A form control's text inset is `Theme.PaddingField` — that is what makes
controls in one row share a height. A structural wrapper (a container that
groups children but is not a box) must set `SizeBorder: NoBorder` and
`Padding: NoPadding`, so its reserved border and padding do not silently add
height.

## Vertical centring — correct the ink only where the alphabet allows

A vertically-centred control centres the text's _line box_, which reserves
descent space the ink may not use, so short descender-free text — digits above
all — reads high. The correction is `opticalCenterText` (the `AmendLayout` form)
or `colorFieldPadding` (the padding form); never a local number.

Apply it only where **the widget owns the text**: a badge's count, a progress
bar's percentage, a button or tab label — anything built on `Button` inherits it
— **or where the widget constrains the alphabet** so nothing can descend: a
colour channel, a date mask, a numeric field. Text the user types into an
unconstrained control is not eligible — correcting it drops descender-bearing
content low, and that is measured, not predicted (issue #346).

A control that **re-labels itself** — a `Select` showing a placeholder until an
option is chosen — takes `opticalCenterLabelText`: the cap band, whatever the
label says. Measuring the run there both misses the defect (a descending label's
ink band already reads low while its cap band rides high) and would step the
label when the selection changed.

A control in a **list at a regular pitch** — a menu item — takes the cap band
for a second reason: measuring each run would move a descender-free label while
leaving its neighbour, and uneven baselines read down the whole list. The same
disagreement between two badges side by side does not.

Which form depends on what the text **is**, and the split is worth learning as
one rule:

- a **value** — a badge's count, a progress readout — is centred on its own ink;
- a **label** — a button, tab, menu item, select — on the face's cap band;
- a **glyph** — an icon, a step triangle, a `×` — always on its own ink, and it
  says so through its style: the theme's `Icon` rungs carry the mark, and
  `glyphStyle(ts)` applies it to a symbol drawn in a text face. A glyph child
  inside a cap-band container corrects itself, so an icon button needs nothing
  at the call site;
- **editable text** takes a content-free band, or none at all.

A digit-only label is the one case that needs saying out loud: figures measure
shorter than caps, so a widget that knows its label is digits opts into the
figure band (`ButtonCfg.opticalDigitLabel`, as the date picker's cells do). An
application cannot — the alphabet is a guarantee only the widget building the
label can make.

Editable text takes the content-free form — `opticalCenterFieldText` as a hook,
`colorFieldPadding` as padding — because an offset that follows the content
moves the baseline as the user types. A widget wrapping `Input` opts in with the
unexported `opticalDigitCenter`, which is what keeps the guarantee the caller's
to make — and the probe must match the alphabet that guarantee names: the figure
band for a digit field, the cap band for a hex one. Probing the taller band for
digits overshoots by as much as leaving them alone. See
`docs/specs/text-optical-centring.md`.

## Per-state colors — ColorSet, with flat as the exception

Build per-state colors with `ColorSet` (its `resolved()` returns the
theme-matched color for the shape's state) and keep one appearance with
`Flat(c)`. Precedence: an assigned flat `Color*` field on the Cfg wins over the
`ColorSet` — the widget keeps its appearance when a set arrives.

## Focus rings — the shared helper, not a local stroke

A focusable widget's focus affordance comes from the shared ring helpers
(`colorControlFocusRing`, `focusRingAmend`), which own the ring's shape and
colors. A hand-drawn focus stroke at a call site is a second implementation of
the same affordance — the divergence the audit's §6 measured.

## Deviating

A literal dimming alpha or size step in `gui/view_*.go` is a finding unless the
line carries the deferred marker:

```go
alpha := uint8(150) // ergonomics-audit:visual
```

The marker is for the audit's documented exemptions, and they are listed here so
a new exemption is a decision, not a habit:

- **Ramps** — a graduated effect, not a state: the date-picker roller distance
  ramp (`view_date_picker_roller.go`), the math-spinner ghost and trail
  (`view_math_spinner.go`).
- **Non-text fills** — an alpha that is not de-emphasized text: markdown
  code/blockquote backgrounds, the dock zone preview, a drag ghost, a selection
  highlight, the swatch edge.
- **Tracked size steps** — two, and both for the same reason: they step from a
  caller-supplied size, which lands anywhere, so no named rung is the one to
  read. The date-picker roller's center-item emphasis
  (`view_date_picker_roller.go`) and the numeric input's step triangle
  (`view_input_numeric.go`). The ladder's rungs are non-uniform (+2 at the small
  end, +4 above medium), so no fixed step equals a rung at every size. Both
  bound the result with named rungs.

A new deviation carries the marker **and** a comment naming the reason; the gate
reports the line as deferred, so the exemption stays visible in every
`make ergonomics-audit` run. Anything else fails the gate — which is the point.
