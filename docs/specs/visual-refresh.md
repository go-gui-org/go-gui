# Visual refresh: palette, type, density, elevation

- **Status:** phase 3 landed (§ 4 palettes, accent ramp, semantic colors; § 5.1
  border). Phases 1, 4, 5a/5b, 6, 7, 8 pending.
- **Extends:** `docs/style-guide.md`,
  `docs/specs/widget-visual-consistency-audit.md` (issue #335)
- **Breaking:** yes, deliberately. All goldens re-record, every preset changes
  appearance, six registered theme names are removed.

## Problem

Rendered evidence, not code reading: an ordinary settings form built through
`gui/backend/soft` in `ThemeDark` and `ThemeLight` shows four defects that no
amount of per-widget polish reaches.

1. **Labelled form fields shrink-wrap to their content.** A field containing
   "Mike" is the width of the word "Mike". This is a bug, not taste — see § 1.
2. **Large type in tight boxes.** Body text is 16px (`gui/styles.go:28`) while a
   field insets its text by 5px horizontally (`gui/padding.go:38`). Glyphs touch
   the border. The ratio is the signature of a UI that was never looked at.
3. **The UI reads as outlines, not surfaces.** `SizeBorder` is 1.5 (3 device
   pixels at 2x) and the dark border `RGB(100,100,100)` sits on an interior of
   `RGB(74,74,74)` — a loud stroke — while background to interior is only 26
   steps. Separation comes from the stroke because the fills do not provide it.
4. **No hierarchy anywhere.** Primary and secondary buttons are pixel-identical.
   One fully-saturated accent (`RGB(65,105,225)`) serves focus, selection,
   slider fill and progress fill with no ramp. `ThemeCfg.FocusRing`,
   `ShadowPopover` and `ShadowDialog` exist and are nil in both default presets.

## Goals

- Smart and appealing, not decorated. No gradients, no glass, no animation work
  in this spec.
- One coherent design language across every widget, both polarities and all
  three platform themes.
- Dark stays the default an app gets with no `SetTheme` call.
- Fewer presets. Every remaining name earns its place.
- Every visual claim in this spec is verified by a re-recorded golden, never by
  reading source.

## Non-goals

- Hover/focus color transitions. The animation system (`gui/animation*.go`) is a
  frame-driven registry of `Animation` objects keyed by ID on the `Window`;
  nothing connects a shape's hover state to a color tween. Wiring that means
  per-effective-ID hover progress in `StateMap` plus color interpolation in the
  fill resolution of ~16 widgets, in a toolkit whose stated bottleneck is
  allocation. Separate project, budgeted separately.
- New widgets.
- Changing the layout engine.

---

## 1. `labelledField` discards the caller's sizing (bug)

`labelledField` (`gui/field_label.go:60`) hardcodes `Sizing: FitFit` on the
wrapper `Column`. Verified by rendering: an `InputCfg{Sizing: FillFit}` inside a
`FillFit` `Row` still arranges at the width of its text. The caller's `Sizing`
is unreachable, so **no labelled field in any app built on this toolkit can fill
its row**. Nine widgets route through this function.

**Fix.** `labelledField` takes the field's sizing and passes the horizontal axis
through, keeping the vertical axis `Fit` so the wrapper still hugs label +
field:

```go
func labelledField(
    label string, base TextStyle, align HorizontalAlign,
    sizing Sizing, field View,
) View
```

The wrapper takes the caller's width and forces its own height to Fit. **Do not
spell this as a raw `Sizing{Width: sizing.Width, Height: sizingFit, set: true}`
literal.** `Sizing` self-flags via an unexported `set`, and the rule is to build
sizings from the predefined vars only — `tools/ergonomics-audit/literals.go`
gates raw `Sizing{...}` literals outside `gui/sizing.go` and fails the build.
Map through the vars instead:

```go
// fitHeight keeps the caller's width mode and pins height to Fit, so a
// labelled field can fill its row while the label/field column stays
// content-height.
func fitHeight(s Sizing) Sizing {
    switch s.Width {
    case sizingFill:
        return FillFit
    case sizingFixed:
        return FixedFit
    default:
        return FitFit
    }
}
```

The wrapper uses `fitHeight(sizing)` when `sizing.IsSet()` and `FitFit`
otherwise — so a Cfg that never sets `Sizing` gets the wrapper it gets today.
That compatibility claim covers **the wrapper's sizing only**. § 1b deliberately
changes the default width of the field itself, so an unsized labelled field does
move — it goes from Fit-to-content to a 160px floor. The two changes ship in one
phase precisely so that move is read once, in one golden diff, rather than
twice.

Call sites to update (all pass their own `cfg.Sizing`):
`view_color_fields.go:404`, `view_color_picker.go:69`, `view_combobox.go:87`,
`view_date_picker.go:115`, `view_input_numeric.go:149`, `view_input_date.go:80`,
`view_input.go:322`, `view_select.go:80`, `view_slider.go:256`.

`ColorPickerCfg` and `SliderCfg` reach `labelledField` with no `TextStyle`. Both
already carry a `Sizing` field (`view_color_picker.go:39`, `view_slider.go`), so
the new argument resolves at every call site with no Cfg addition.

### 1b. Field minimum widths

A `Fit`-sized empty field is a stub. Add `ThemeCfg.SizeFieldMinWidth`
(default 160) consumed as the `MinWidth` floor by `Input`, `NumericInput`,
`Select`, `Combobox`, `DatePicker` and `InputDate` when the Cfg sets no
`MinWidth`.

**Zero means no floor, not "derive 160."** It is a `float32` with a meaningful
zero, and the § Opt rule says a primitive whose zero is a real choice would need
`Opt` — but here the two readings collapse: `baseCfg()` sets 160, so every
preset theme carries it, and a hand-built `ThemeCfg` that leaves it zero is
asking for today's Fit-to-content behavior. A custom theme wanting no floor sets
nothing; one wanting a different floor states it. No `Opt` wrapper. This also
retires **four** literals in `ThemeMaker`, not two: the same `MinWidth: 75` /
`MaxWidth: 200` pair appears on `selectStyle` (`theme_maker.go:188-189`) and
again on `ComboboxStyle` (`theme_maker.go:436-437`). They are the reason a
`Select` and an `Input` in one row disagree on width for reasons neither the
theme nor the caller stated.

**`MaxWidth: 200` goes too, not just `MinWidth`.** It is the same defect from
the other end: a `Select` given `FillFit` in a 900px row stops at 200px and the
caller has no way to see why. With § 1's fix in place a labelled `Select` can
finally fill its row, and the cap would silently defeat it. Drop both fields to
0 (uncapped) on `selectStyle` and `ComboboxStyle`; a caller wanting a ceiling
sets `MaxWidth` on the Cfg. `maxDropdownHeight: 200` is unrelated and stays — it
bounds the popup list, not the control.

---

## 2. Type ladder

### 2.1 Base size and platform overrides

`sizeTextMedium = 16` is a web body size, not a desktop one. Native values:
macOS 13, Windows 12, GNOME ~15.

Rather than pick one, make the base a per-theme decision and derive the ladder
from it:

```go
// textSizes returns the six-rung ladder derived from a body size.
// Offsets, not ratios: a ladder rounded from ratios lands on
// fractional pixels at some bases and hints badly.
func textSizes(body float32) (tiny, xs, s, m, l, xl float32) {
    return body - 4, body - 3, body - 2, body, body + 3, body + 8
}
```

| Theme          | Body | Ladder (tiny → xlarge) |
| -------------- | ---- | ---------------------- |
| dark, light    | 14   | 10, 11, 12, 14, 17, 22 |
| macos(-dark)   | 13   | 9, 10, 11, 13, 16, 21  |
| windows(-dark) | 12   | 8, 9, 10, 12, 15, 20   |
| gnome(-dark)   | 15   | 11, 12, 13, 15, 18, 23 |

14 is the cross-platform default: 13 reads native on macOS but thin on a
non-HiDPI Linux or Windows display, and the toolkit's own themes ship
everywhere. The platform themes take their platform's value, which is what they
exist for.

The current top of the ladder (`+4/+8`) is too flat to build a heading from;
`+3/+8` gives one clear step for section headings (17) and one for page titles
(22) while pulling the crowded bottom apart.

### 2.2 Weight, not just size

The `B1..B6` bold ladder already exists and is complete. The defect is in what
widgets _default_ to, not in what the theme offers. Audit pass: every widget
rendering a heading, a group-box title, a dialog title, a tab label or a table
header takes a `B` rung; body and value text stays `N`. Section 3 of the widget
audit (`docs/specs/widget-visual-consistency-audit.md`) is the model for how to
record this.

---

## 3. Density: padding, spacing, control height

### 3.1 Field and button inset

| Token           | Now            | New              |
| --------------- | -------------- | ---------------- |
| `paddingField`  | `(3, 5, 3, 5)` | `(5, 10, 5, 10)` |
| `paddingButton` | `PadAll(6)`    | `(5, 12, 5, 12)` |

With body 14 both land at ~31px control height, so a `Button`, `Input` and
`Select` in one row are the same height — which is the invariant
`Theme.PaddingField` was introduced to hold, now with a horizontal inset that
does not crowd the glyphs. Buttons get more horizontal room than fields because
a button's label is its whole content.

### 3.2 Padding ladder

`PadXSmall 3 → 4`, `PadSmall 5 → 6`, `PadMedium 10 → 14`, `PadLarge 15 → 22`.

### 3.3 Spacing ladder

`SpacingTight 2` (unchanged), `SpacingSmall 5 → 6`, `SpacingMedium 10 → 14`,
`SpacingLarge 15 → 28`.

The current 5/10/15 ladder is arithmetic, so label-to-field (5) and
field-to-field (10) differ by only 2x and the proximity reads ambiguously — in
the reference render the "Role" label appears equally attached to the control
below it as to its own `Select`. Roughly doubling each rung makes group
membership legible without measuring.

The rung meanings in `gui/styles.go` stay exactly as documented; only the values
move.

---

## 4. Color

One hue family across both polarities: a cool neutral at hue ~215 with low
saturation. Not "tinted gray for its own sake" — the tint is what lets a border
be a transparent white/black wash without turning the surface beneath it a
different color than the page.

### 4.1 Dark (default)

| Role                  | Value                     |
| --------------------- | ------------------------- |
| `ColorBackground`     | `#17191C`                 |
| `ColorPanel`          | `#1E2125`                 |
| `ColorInterior`       | `#262A2F`                 |
| `ColorHover`          | `#2F343A`                 |
| `ColorFocus`          | `#383E45`                 |
| `ColorActive`         | `#424951`                 |
| `ColorBorder`         | `RGBA(255, 255, 255, 28)` |
| `ColorSeparator`      | `RGBA(255, 255, 255, 20)` |
| text (`TextStyleDef`) | `#E6E8EB`                 |

Background-to-interior is now 15 steps of lightness against a border that is a
wash rather than a stroke — the inverse of today, where the stroke carries the
separation.

### 4.2 Light

| Role                  | Value     |
| --------------------- | --------- |
| `ColorBackground`     | `#F2F4F7` |
| `ColorPanel`          | `#FFFFFF` |
| `ColorInterior`       | `#FFFFFF` |
| `ColorHover`          | `#EDF0F4` |
| `ColorFocus`          | `#E7EBF1` |
| `ColorActive`         | `#E0E5EC` |
| `ColorBorder`         | `#D8DDE4` |
| `ColorSeparator`      | `#E6EAEF` |
| text (`TextStyleDef`) | `#1A1D21` |

The current light ramp (`RGB(195,195,215)` and neighbours) is a saturated
lavender on mid-value grays, and its controls are _darker_ than its page —
backwards. Controls become the light surface and the page becomes the tint, and
the ramp moves down from white as energy increases, mirroring the dark ramp
moving up from black.

### 4.3 Accent ramp

New `ThemeCfg` fields. `ColorSelect` keeps its meaning (the selection fill) and
defaults to `ColorAccent` when unset, so selection and accent stay one decision
by default and two when a theme needs them apart.

| Field                | Dark                     | Light                    |
| -------------------- | ------------------------ | ------------------------ |
| `ColorAccent`        | `#4D82F0`                | `#2F6FE0`                |
| `ColorAccentHover`   | `#85AAF5`                | `#6494E8`                |
| `ColorAccentPressed` | `#155AEB`                | `#1B53B7`                |
| `ColorAccentSubtle`  | `RGBA(77, 130, 240, 40)` | `RGBA(47, 111, 224, 30)` |
| `ColorTextOnAccent`  | `#FFFFFF`                | `#FFFFFF`                |

The hover/pressed values are what the rule below actually produces — an earlier
draft of this table was picked by eye and disagreed with the rule, so the table
was corrected to the derivation rather than the other way round (recorded in the
decisions).

`ThemeMaker` derives every unset slot from `ColorAccent`, so a custom theme
states one color and gets a working ramp. The derivation is pinned, because
"+12% lightness" has at least three defensible readings:

- **Color space: sRGB HSL**, via the existing `ColorToHSLA` / `HSLA.Color()`
  (`gui/color_hsl.go:63,86`). Not OKLCH, not linear RGB — the repo already has
  this pair, and the table values above were picked in it.
- **Offsets are absolute on `L`, not relative**: `L+0.12` for hover, `L-0.12`
  for pressed, each clamped to `[0,1]`. Relative (`L*1.12`) collapses to nothing
  on a dark accent and overshoots on a light one. `H` and `S` are carried
  through unchanged.
- **`ColorAccentSubtle`** is `ColorAccent` at alpha 40 (dark) / 30 (light) —
  fixed per polarity, taken from the theme's own polarity as `textRolesFor`
  does, not one alpha for both.
- **`ColorTextOnAccent`** is white when `srgbLuminance(ColorAccent) < 0.45`,
  black otherwise, using the existing `srgbLuminance`
  (`gui/theme_text_roles.go:69`). 0.45 rather than the midpoint because white
  text on a mid-blue reads better than black at the same luminance; both default
  accents land well clear of it, so the threshold only matters for a custom
  theme picking a yellow or lime accent.

The table values are what these rules produce from `#4D82F0` and `#2F6FE0`; a
test should assert that, so the table and the code cannot drift.

`ColorAccentSubtle` replaces full-accent fills on selected and highlighted rows
— `ListBox`, `Table`, `DataGrid`, and the dropdown rows of `Select`, `Combobox`
and the command palette — where a saturated slab behind every selected row is
the loudest thing on the screen. Selection paints the subtle tint everywhere and
in every focus state; focus is the ring (phase 5b), not a second fill (decision
2). Only the menus keep the full accent fill on their selected item, where the
row _is_ the focus indication. A subtle row's text stays the body color — the
accent/text pairing (`ColorTextOnSelect`) applies only where the full accent
fill still happens.

### 4.4 Semantic colors

`ColorSuccess`, `ColorWarning` and `ColorError` already exist on `ThemeCfg`;
only `ColorError` is set by the default presets. Fill all three in both
polarities and give each a `*Subtle` companion on the same derivation rule as
the accent, so a validation message and its field background are one decision.

---

## 5. Borders, radius, elevation, focus

### 5.1 Border

`sizeBorderDef 1.5 → 1`. A hairline is what every platform draws; 1.5 at 2x is a
3-pixel stroke and is the main reason the toolkit reads boxy. The macOS theme
already sets 1 for exactly this reason (`gui/theme_macos.go:76`).

### 5.2 Radius

| Token          | Now | New | Applies to                             |
| -------------- | --- | --- | -------------------------------------- |
| `Radius`       | 5.5 | 6   | default for controls                   |
| `RadiusSmall`  | 3.5 | 4   | checkboxes, badges, scrollbar thumbs   |
| `RadiusMedium` | 5.5 | 6   | buttons, inputs, selects               |
| `RadiusLarge`  | 7.5 | 12  | dialogs, dropdowns, popovers, tab body |

Only the large end moves materially. This ladder is very nearly the macOS
theme's existing 6/4/6/10 (`gui/theme_macos.go:79-82`) — which both validates
the proposal (a hand-tuned platform theme already landed on these numbers) and
means `macos`/`macos-dark` see only `RadiusLarge` move, 10 → 12, if that theme
drops its override.

An audit item: confirm which styles currently consume `RadiusLarge` and move any
_control_-tier consumer down to `RadiusMedium` before the bump, or a slider
track becomes a capsule by accident.

### 5.3 Elevation

`ShadowPopover` and `ShadowDialog` are nil in `ThemeDark` and `ThemeLight`
today. Fill them:

| Theme | `ShadowPopover`                   | `ShadowDialog`                     |
| ----- | --------------------------------- | ---------------------------------- |
| Dark  | `RGBA(0,0,0,140)`, +4y, blur 12   | `RGBA(0,0,0,170)`, +12y, blur 32   |
| Light | `RGBA(16,24,40,40)`, +4y, blur 12 | `RGBA(16,24,40,60)`, +12y, blur 32 |

**Elevation goes on floating surfaces only** — menus, dropdowns, tooltips,
toasts, dialogs, the command palette. Inline panels and cards separate by fill
value (§ 4), never by shadow. A toolkit that shadows its inline containers is
solving a contrast problem with the wrong tool, and the fix in § 4 removes the
problem.

### 5.4 Focus ring

`ThemeCfg.FocusRing` is nil in both default presets; only the macOS themes set
it (`gui/theme_macos.go:93`). Set it in both:
`BoxShadow{Color: ColorAccent at ~45% alpha, Spread: 2, BlurRadius: 3}`.

Two follow-ons:

- **`BoxShadow` gains a `Spread` field** — see § 5.5. A focus ring is a spread,
  not a blur; a blur-only ring is a soft glow rather than the crisp offset ring
  every platform draws. Decided: add it.
- **Wiring is smaller than a raw call-site count suggests.** Two widgets call
  `applyFocusRingShadow` directly — `Button` (`view_button.go:107`) and `Input`
  (`view_input.go:577`) — but the third site is inside `focusRingAmend`
  (`focus_ring.go:120`), which is already an `AmendLayout` hook on **five**
  more: `Select` (`view_select.go:213`), `ListBox` (`view_listbox.go:158,266`),
  `Table` (`view_table_keys.go:42`), `ExpandPanel` (`view_expand_panel.go:82`)
  and `VirtualList` (`view_virtual_list_build.go:170`). Those seven pick the
  ring up for free the moment `FocusRing` is non-nil in the defaults. The wiring
  work is the remaining focusables — `Combobox`, `DatePicker`, `InputDate`,
  `Tree`, `NumericInput`, `Radio`, `RadioButtonGroup`, `Switch`, `Toggle`,
  `Slider` — roughly eight to ten `focusRingAmend` hooks. Still mandatory (a
  ring on seven widgets and not the rest is worse than today's uniform border
  recolor), but it is hook placement, not new mechanism.

### 5.5 `BoxShadow.Spread`

`BoxShadow` carries `Color`, `OffsetX`, `OffsetY`, `BlurRadius` and nothing
else. Add `Spread float32`: the amount the shadow's own shape is grown beyond
the caster before blurring. Zero is today's behaviour, so every existing shadow
is untouched.

**This cannot be done at the emit site.** The obvious cheap implementation —
inflate `X/Y/W/H` and `Radius` in `renderContainer` (`gui/render_layout.go:200`)
and leave the backends alone — does not work: a shadow is drawn as a blurred
rounded rect with the **inverse coverage of the un-offset caster multiplied in**
(`softRoundRect(cmd, offX, offY, cutCaster: true)`,
`gui/backend/soft/draw_effects.go:63`), and the caster cut-out is derived from
the same command rect. Inflating that rect inflates the cut-out with it and
erases the ring exactly where it should appear.

So spread needs real plumbing:

- `Spread float32` on `gui.BoxShadow` (`gui/shape.go`) and on `gui.RenderCmd`
  (`gui/render_types.go:78`, beside `BlurRadius`).
- `renderContainer` passes it through, and widens its emit guard: a shadow with
  `Spread > 0` must emit even when blur and both offsets are zero, which is
  precisely the crisp-ring case the current guard rejects.
- `validShadowCmd` (`gui/render_validate.go:127`) accepts and range-checks it.
- **Six backend draw paths** grow the shadow's distance field by `Spread` while
  holding the caster cut-out at the original rect. Concretely, in each of the
  SDF shaders and in the software rasterizer, the shadow's own rect grows on
  **both** bounds and its corner radius grows with it — `w+2*Spread`,
  `h+2*Spread`, `rad+Spread`, origin back by `Spread` in x and y — while the
  caster keeps the un-inflated `w`, `h`, `rad`. Growing the bounds without
  growing the radius is the classic error: it squares off the ring's corners
  against a rounded control. In `soft` this is the `rad` argument to the two
  `coverageRoundRect` calls in `softRoundRect`
  (`gui/backend/soft/draw_effects.go:83,93`), which today share one `rad` and
  must diverge. Sites: `metal/draw.go:40`, `gl/draw.go:36`, `soft/draw.go:101`,
  `web/draw.go:63`, `ios/draw.go:39`, `android/draw.go:39`.
  `gui/print_pdf.go:133` (not under `gui/backend/`) already skips `RenderShadow`
  and needs no change.

That is six shader/rasterizer edits, not the "one field" it looks like from the
theme side. It is worth it — every widget's focus indication rides on it — but
it is its own phase (§ 9, phase 5a) and it lands before the ring is switched on
in the default presets, never after.

---

## 6. Button hierarchy

Add a variant axis to `ButtonCfg`. The zero value is today's look, so existing
call sites are unaffected in structure (they still change in appearance, via the
palette).

```go
type ButtonVariant uint8

const (
    ButtonSecondary ButtonVariant = iota // zero value: today's button
    ButtonPrimary                        // accent fill, ColorTextOnAccent
    ButtonGhost                          // no fill, no border, hover fill only
    ButtonDanger                         // ColorError fill
)
```

`Theme` grows `buttonStylePrimary`, `buttonStyleGhost` and `buttonStyleDanger`,
all derived in `ThemeMaker` from the accent and error ramps — no literals, per
the single-source rule (`docs/specs/theme-style-single-source.md`).

`ButtonCfg.Colors` (a `ColorSet`) keeps precedence over the variant, so a caller
can still take a variant and override one state.

**Text color on a filled variant needs its own mechanism — a fill alone will not
recolor the label.** `Text` resolves its style **eagerly**, in the factory: a
`TextCfg` with no `TextStyle` takes `DefaultTextStyle` at `Text(...)` call time
(`gui/view_text.go:151-155`). A button's children are therefore fully-styled
`View`s before `Button` ever runs, so `ButtonPrimary` cannot recolor them by
wrapping, and `Themed` does not help either — its builder runs at generation
time, but `Content: []View{Text(...)}` has already run. Left alone, an accent
`ButtonPrimary` in a dark theme renders near-white-on-blue by luck and
`ButtonDanger` renders the same label on red. Two paths, and the spec takes
both:

- **The button owns the label.** Add `ButtonCfg.Label string`, the field
  `TextButton` already implies; when set, `Button` builds the `Text` itself and
  applies the variant's `ColorTextOnAccent`. This is the path every variant
  example should use, and `TextButton` gains a variant argument or a sibling.
- **The caller owns the label.** For `Content`-built buttons, an explicitly
  colored `Text` must keep its color — overriding it would be worse than the
  problem. To recolor only the ones that took the default, `TextStyle` gains an
  unexported `defaultedColor bool` set by `Text` on the `DefaultTextStyle`
  fallback, following the exact precedent of `disabledRole` and `glyphRole`
  (`gui/styles.go:96-129`): unexported, set in one place, never spelled at a
  call site, and it rides along through the struct copies widgets make. `Button`
  then stamps `ColorTextOnAccent` onto descendant text shapes carrying the flag.

If the second half proves noisy in practice, ship only the first and document
that a `Content`-built filled button must color its own label — but do not ship
the variants with neither.

One accent-filled primary per surface is the convention; state it in
`docs/style-guide.md` and check it in review, not in code.

---

## 7. Preset consolidation

Fourteen names are registered today. Remove six:

| Name                 | Why removed                                                          |
| -------------------- | -------------------------------------------------------------------- |
| `dark-bordered`      | Identical to `dark` since borders became default.                    |
| `light-bordered`     | Identical to `light`.                                                |
| `dark-no-padding`    | Reachable as `ThemeDark.WithBorders(false)` plus a padding override. |
| `light-no-padding`   | Same.                                                                |
| `blue-dark`          | A taste preset; `ThemeMaker` is the supported way to make one.       |
| `blue-dark-bordered` | Same.                                                                |

Eight remain: `dark` (default), `light`, and the three platform pairs
`macos`/`macos-dark`, `gnome`/`gnome-dark`, `windows`/`windows-dark`.

Decided: the platform six stay registered. They do a different job from the
toolkit's own two — they let an app look native on its host, which `dark` and
`light` deliberately do not — and they are the mechanism § 2.1 uses to carry a
per-platform body size. Moving them behind a build tag or a subpackage is not
pursued.

`ThemePicker` (`gui/view_theme_picker.go`) lists the registry, so it shrinks
automatically. `docs/` and per-example READMEs naming removed presets must be
updated (see § 10).

`defaultTheme = &ThemeDark` (`gui/theme_defaults.go:249`) already makes dark the
default with no `SetTheme` call; that is kept and pinned by a test.

---

## 8. Widget defects visible in the reference render

- **`ProgressBar` label straddles the fill boundary.** At 45% the "45%" text is
  half over the accent fill and half over the track, unreadable on both. **Fix:
  move the readout outside the bar**, trailing it at `SpacingSmall`, in
  `TextStyleSecondary`. Not the scrim: a full-width scrim behind the label is a
  third surface color inside a 20px-tall control, it fights the § 5.3 rule that
  nothing inline separates by anything but fill value, and it still leaves the
  label's contrast depending on where the fill happens to be. Outside the bar
  the readout is legible at every percentage and the bar becomes a pure
  indicator. `TextShow` keeps its name and meaning; only the placement changes,
  and `progressBarCenterLabel` plus the `opticalCenterText` amend
  (`view_progress_bar.go:88`) are deleted with it. The fill is **already**
  radius-clipped — the fill `Row` sets `Radius: SomeF(radius)`
  (`view_progress_bar.go:76`) — so only the label is at issue.
- **Switch and toggle sizes** (`SizeSwitchWidth 36`, `SizeSwitchHeight 22`,
  `SizeRadio 16`) are tuned against 16px text. Re-tune against 14: switch 34x20,
  radio 15, and `ToggleStyle.Size` stays `ts.Size + 4`. The macOS theme keeps
  its own 38x22 (`gui/theme_macos.go:87-88`), which is deliberate knob-travel
  tuning and is not touched by this retune — a platform override is exactly the
  seam that lets the base ladder move.

---

## 9. Phasing

Each phase re-records goldens and lands independently.

| Phase | Content                                       | Surface | Status |
| ----- | --------------------------------------------- | ------- | ------ |
| 1     | § 1 `labelledField` bug + field min widths    | widget  | landed |
| 2     | § 2 type ladder, § 3 density                  | theme   | landed |
| 3     | § 4 palettes and accent ramp, § 5.1 border    | theme   | landed |
| 4     | § 5.2 radius, § 5.3 elevation                 | theme   |        |
| 5a    | § 5.5 `BoxShadow.Spread` through six backends | backend |        |
| 5b    | § 5.4 focus ring + wiring the remaining ~8    | widget  |        |
| 6     | § 6 button variants                           | widget  |        |
| 7     | § 8 widget defects                            | widget  |        |
| 8     | § 7 preset removal + all doc deliverables     | mixed   |        |

Phases 2–4 are constant edits in `gui/styles.go`, `gui/padding.go` and
`gui/theme_defaults.go` — they move every widget at once and are cheap to
revert. Phases 1, 5a, 5b and 6 are the real work.

Phase 5a lands before 5b without exception: switching the ring on in the default
presets while `Spread` is still absent ships a soft glow that then changes
appearance a phase later, and re-records every focus golden twice.

## 10. Verification

Mandatory before any phase is called done:

- `go test ./gui/ -run TestGolden -update` — **read the diff before keeping
  it.** Both `ThemeDark` and `ThemeLight` record.
- `go test ./gui/backend/soft/ -run TestPixelGolden -update` — pixel goldens,
  same two themes (`pixelThemes`, `gui/backend/soft/golden_test.go:89`).
- **Platform themes are in scope (§ Goals) and must be recorded, not assumed.**
  Both golden harnesses record `ThemeDark` and `ThemeLight` only. Extend
  `pixelThemes` to include `macos-dark`, `gnome-dark` and `windows-dark` for a
  single representative case — one form row and one button row is enough. The
  platform themes override the base ladder at exactly the points this spec moves
  (`theme_macos.go` sets its own radius ladder, switch size and focus ring), so
  an unrecorded platform theme is where a base-ladder change silently breaks a
  hand-tuned override. Their light halves are covered by the same override
  surface and need no separate case.
- New golden cases, each added in **the phase that introduces the feature** and
  carried forward from there — not all in phase 1, since three of them describe
  behavior that does not exist yet:

  | Case                                                  | Added in |
  | ----------------------------------------------------- | -------- |
  | Two-field labelled form row at `FillFit`              | 1        |
  | `ProgressBar` at 45% with `TextShow`                  | 1        |
  | Focused `Input` and focused `Select` (case `focusID`) | 5b       |
  | The four button variants side by side                 | 6        |

  The two phase-1 cases are the regression pins for § 1 and § 8; adding them
  first means every later phase re-records them and the diff shows the density
  and palette moves on a shape whose geometry is already correct.

- `make ergonomics-audit` — modes `visual`, `literals`, `theme`.
- Phase 5a only — "runs an example and looks" is not a renderer contract. Four
  layers, in order:

  1. **Emit test** (backend-independent): a container with a `Spread`-only
     shadow — zero blur, zero offset — produces exactly one `RenderShadow`
     command carrying the spread. This is the guard-widening in § 5.5 and it
     fails today; it is the cheapest possible pin on the whole feature.
  2. **Validation test**: `validShadowCmd` accepts a positive `Spread`, rejects
     NaN/Inf and negative values, and range-checks against the same ceiling as
     `BlurRadius`.
  3. **Pixel goldens** (`soft` only): a spread-only ring, a spread+blur ring,
     and a spread on an already-rounded caster — the third is the case that
     catches the radius bug named in § 5.5.
  4. **Backend checklist**, one written pass per backend against a stated
     expectation rather than a general look: for a caster at `(x,y,w,h,rad)`
     with `Spread: s`, the shadow covers `(x-s, y-s, w+2s, h+2s, rad+s)`, the
     caster's own area is not covered, and the visible ring is `s` px wide on
     every side including diagonally through the corners.

  The bar is **equivalent geometry and coverage**, not pixel-identical output.
  The six backends do not agree pixel-for-pixel today — analytic SDF coverage in
  the GPU paths against a box-blurred alpha mask in `soft` differ at the edges
  by construction — so "renders identically" would be a test no correct
  implementation could pass. See the two-sided technique in
  `gui/backend/CLAUDE.md`.

- `TestDefaultStylesMirrorThemeDark` must stay green; it is the gate that stops
  a literal being reintroduced into a `default*Style` mirror.
- `make prepush`.

Documentation deliverables, per the repo convention that a visual change is not
done until the docs say the same thing:

- `docs/style-guide.md` — the accent ramp, elevation-on-floating-only rule,
  one-primary-per-surface convention, the new ladders.
- `README.md` — screenshots re-taken.
- `CHANGELOG.md` — a breaking-change entry naming the six removed preset names.
- `examples/` — 93 files call `WithBorders(true)` (111 call sites); borders are
  the default, so these are no-ops and should be deleted in the same sweep. Any
  example naming a removed preset must be repointed.
- Per-example READMEs that show screenshots.

## Decisions taken

- **`BoxShadow.Spread`** — add it (§ 5.5). Scope is six backend draw paths, not
  one field; phase 5a.
- **Platform themes** — the six stay registered (§ 7). Eight preset names total.
- **Tab label weight (§ 2.2)** — the selected tab takes `B3`; resting tab labels
  stay `N3`. The strip keeps its quiet by default and gains hierarchy only where
  the eye already points.
- **Pixel-harness platform coverage (§ 10)** — a per-case `themes` list, not a
  global six-theme recording: the one representative case (a form row and a
  button row) records under dark, light and the three dark platform themes;
  every other case keeps the dark/light pair.
- **Body size** — phase 2 ships the spec table (dark/light 14, macOS 13, Windows
  12, GNOME 15). The 14-vs-13 check on a non-HiDPI display is still open
  (deferred question 1); if 13 wins, the dark/light ladder shifts one step and
  phases 2–3 goldens re-record before phase 4.
- **Markdown block gap** — `MarkdownStyle.blockSpacing` moves from
  `SpacingLarge` to `SpacingMedium`. Under the phase-2 ladder Large went 15 →
  28, and a markdown document is a stack of paragraphs, not a stack of unrelated
  sections; at body 14 the 28px gap reads as a hole. The extra spacer emitted
  after a closing blockquote drops to `blockSpacing / 2`, since that spacer is
  itself a child and so carries a block gap on each side (a full-height one
  totalled three gaps, ~84px).

- **`colorPickerMinPlane`** — 120 → 112 (view_color_picker.go). The refreshed
  spacing ladder drops the picker's derived plane to 118, tripping the old floor
  and breaking the picker's row-width invariant; the floor is degenerate-theme
  protection, so it now sits under the default derivation.
- **Body size** — 14 ships (deferred question 1, answered): the non-HiDPI check
  stays owed but no longer blocks; a later shift re-records phases 2–3 goldens.
- **Selection fill (deferred question 2)** — subtle + ring: selected and
  highlighted rows paint `ColorAccentSubtle` in every focus state, and their
  text stays the body color. Focus is the ring (phase 5b), never a second fill.
  Menus keep the full accent on their selected item. No focus-state plumbing in
  row renderers.
- **Light semantic colors** — success `#2E9E5B`, warning `#B07E1F`, error
  `#D64545`; dark keeps its existing error and the historic toast/badge
  success/warning values (moved from ThemeMaker literals into the preset consts,
  with the literals kept as the unset fallback so unstated themes stay
  byte-identical).
- **Accent fallback chain** — `ColorAccent` → `ColorSelect` → legacy select
  (`colorSelectDark`). `ColorSelect` defaults to the accent. Platform and taste
  themes keep their native select, which becomes their accent; their ramps
  derive.
- **Accent ramp table corrected to the derivation** — the rule in § 4.3 (sRGB
  HSL `L±0.12`) does not reproduce the table's original hover/pressed values
  (e.g. dark pressed came out `#155AEB`, not `#3A6FD8`), and no clean formula
  reproduces that table either. The rule is the mechanism the spec chose and is
  what a custom theme gets, so the table was corrected to what the rule produces
  (`#85AAF5`/`#155AEB` dark, `#6494E8`/`#1B53B7` light);
  `TestThemeMakerAccentRamp` pins both.
- **List rows drop `ColorTextOnSelect`** — with selection on the subtle tint,
  the paired foreground no longer applies to list-like rows; the accent/text
  pairing survives in menus and the full-accent fills (slider fill, selected
  tab, radio, switch). The five pairing tests were updated to pin body text on
  washed rows, and the now-dead widget- and style-level `ColorTextOnSelect`
  fields were removed from the list-like widgets — nothing paints a fill needing
  them (the theme-level token survives for menus, the selected tab and the date
  picker's selected day).
- **`ColorTextOnSelect` defaults to the accent-paired foreground** — unset
  resolves to white/black by accent luminance (the same rule as
  `ColorTextOnAccent`), not the body color. The old default drew a light theme's
  near-black body text on its blue accent in menus and the selected tab —
  reported after the palette landed. The selected tab's label additionally pairs
  over the accent fill while keeping its B3 weight. The progress bar's
  percentage stays the body text on the bar, unboxed: it straddles fill and
  track, so no single color pairs with both, and the label is secondary (a
  track-colored chip behind it was tried and rejected by review). Explicit slots
  still win.

## Deferred questions

1. **Body size 14 vs 13** — decided at phase 3: 14 ships (see decisions). The
   non-HiDPI render check is still owed; if 13 wins there, the dark/light ladder
   shifts one step and phases 2–3 goldens re-record before phase 4.
2. **Selection fill vs `ColorAccentSubtle`** — decided at phase 3: subtle + ring
   (see decisions).
