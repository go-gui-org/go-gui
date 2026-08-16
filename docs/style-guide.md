# Style guide — the when, not the what

Issue #335, steps 3–4. This document is the **when**: which named role a
widget should read, and when a deviation is allowed. It cites roles, never
values. Values live in exactly one place — `ThemeMaker` and the
`default*Style` mirrors — and re-spelling one here would be the second
source of truth the audit's other findings were measured against. If this
guide says "secondary text" it means `Theme.TextStyleSecondary`, whatever
it resolves to in the installed theme.

The gate that enforces these decisions is `ergonomics-audit -mode visual`
(`make ergonomics-audit`): a literal dimming alpha or type-size step in
`gui/view_*.go` fails it unless the line carries the marker (§ Deviating).

## Text — pick the role by what the text *does*

Four roles cover every de-emphasis. Choose by the text's job, not by how
quiet it should look:

| Role                    | Use when the text…                                  |
| ----------------------- | --------------------------------------------------- |
| `TextStyleSecondary`    | supports primary text beside it: a hint, a detail, a measurement |
| `TextStyleLabel`        | names a nearby value (a field's label); the one role that also steps size |
| `TextStyleDisabled`     | sits in a control the user cannot act on            |
| `TextStylePlaceholder`  | stands in for a value not yet entered               |

Read the role's style directly where the text color is the theme's
(`ts := guiTheme.TextStyleSecondary`); use `withRoleAlpha(base, role)`
where the hue is caller-supplied — the channel label on a user-picked
color keeps its hue and takes only the role's amount of quiet.

Never spell an alpha. An opaque color (`alpha 255`) is not de-emphasis and
does not need a role; a fade, a ramp or a fill is not text and is covered
by the marker (§ Deviating), not by a role.

## Type steps — size by named handle, or not at all

The size ladder is a set of named handles (`N1`..`N6`, `M1`..`M6`,
`B1`..`B6`, `Icon1`..`Icon6`). Take a step by reading the handle, never by
arithmetic on `X.Size`. A `Size ± N` that produces a text style is a
finding; two tracked exceptions carry the marker (§ Deviating).

The only automatic step in the toolkit is a field label: `TextStyleLabel`
steps its own size, so a labelled form reads correctly without the widget
spelling a number.

Distinguish a type step from geometry. A row height, icon width or splitter
dimension derived from a text size (`style.Size + 4` as a width) is
measurement, not typography, and is not a finding.

## Field labels — two placements, one per control kind

- `labelledField` — the label **above** the control: form fields
  (`Input`, `Select`, `Combobox`, `NumericInput`, `DatePicker`,
  `InputDate`, `ColorPicker`). It exists so the placement and the gap are
  decided once, not per widget.
- `trailingLabel` — the label **beside** the control: boolean controls
  (`Switch`, `Toggle`, `Radio`).

The accessible name is separate: `A11YLabel`/`A11YDescription` feed the
accessibility tree. A visual label and an accessible name are both
expected, and neither substitutes for the other.

## Spacing and insets — the tiers mean relatedness

`SpacingSmall` binds a tightly related pair (a control and its
readout); `SpacingMedium` separates members of one group; `SpacingLarge`
separates sections. Prefer a tier over a magic number; a number that
bypasses the ladder is a finding on the same basis as a dimming alpha.

A form control's text inset is `Theme.PaddingField` — that is what makes
controls in one row share a height. A structural wrapper (a container
that groups children but is not a box) must set `SizeBorder: NoBorder`
and `Padding: NoPadding`, so its reserved border and padding do not
silently add height.

## Per-state colors — ColorSet, with flat as the exception

Build per-state colors with `ColorSet` (its `resolved()` returns the
theme-matched color for the shape's state) and keep one appearance with
`Flat(c)`. Precedence: an assigned flat `Color*` field on the Cfg wins
over the `ColorSet` — the widget keeps its appearance when a set arrives.

## Focus rings — the shared helper, not a local stroke

A focusable widget's focus affordance comes from the shared ring helpers
(`colorControlFocusRing`, `focusRingAmend`), which own the ring's shape
and colors. A hand-drawn focus stroke at a call site is a second
implementation of the same affordance — the divergence the audit's §6
measured.

## Deviating

A literal dimming alpha or size step in `gui/view_*.go` is a finding
unless the line carries the deferred marker:

```go
alpha := uint8(150) // ergonomics-audit:visual
```

The marker is for the audit's documented exemptions, and they are listed
here so a new exemption is a decision, not a habit:

- **Ramps** — a graduated effect, not a state: the date-picker roller
  distance ramp (`view_date_picker_roller.go`), the math-spinner ghost and
  trail (`view_math_spinner.go`).
- **Non-text fills** — an alpha that is not de-emphasized text: markdown
  code/blockquote backgrounds, the dock zone preview, a drag ghost, a
  selection highlight, the swatch edge.
- **Tracked size steps** — the two inline floors the audit keeps open
  (`view_input_numeric.go`, `view_date_picker_roller.go`), pending named
  handles.

A new deviation carries the marker **and** a comment naming the reason;
the gate reports the line as deferred, so the exemption stays visible in
every `make ergonomics-audit` run. Anything else fails the gate — which is
the point.
