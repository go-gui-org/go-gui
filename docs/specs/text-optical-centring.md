# Optical centring of vertically-centred text

Issue #346. This document records **why the correction is opt-in**, which is the
part a later reader will be tempted to undo.

## The error

A text shape is sized to `FontHeight` (`gui/view_text.go:100`) and a
vertically-centred container centres that box. The ink is not centred inside it:
the baseline sits at `Y + FontAscent` (`gui/render_text.go:151-166`), so a run
with no descender paints only in `[baseline - capHeight, baseline]` and the
reserved descent stays empty.

Measured on the platform UI face, at 16, 24 and 48pt (the ratios are identical
at all three, so the error scales with em):

| quantity         | value     |
| ---------------- | --------- |
| ascent           | 0.770 em  |
| descent          | 0.230 em  |
| line gap         | 0.000 em  |
| cap height ("H") | 0.717 em  |
| offset, caps     | 0.0886 em |
| offset, digits   | 0.0708 em |

About 1.4 device pixels at size 16 on a 2× display.

## There is no half-leading term

`glyph.TextMetrics.Height` is `Ascender + Descender` by construction
(`go-glyph/context_puregoft.go:164-182`), with `LineGap` reported separately and
excluded. The line box a text shape occupies therefore carries **no leading to
split**, and a "half-leading" correction computes zero on every face.

This matters because the first version of this fix carried a
`colorFieldLeadRatio = 0.047` constant, commented as measured half-leading. It
was not half-leading. It was compensating for the correction being applied at
twice its size — the padding form spends `top - bottom`, which is two offsets,
and the box's own centre moves with it.

## Why it cannot be applied everywhere

A single toolkit-wide correction was implemented, tried and rejected on
evidence. Badges read correct; single-line inputs read visibly low, and editable
text moved vertically as the user typed. Those are two distinct failures:

1. **Wrong for arbitrary text.** The correction reclaims the descent on the
   assumption nothing paints there. A user typing `gy` paints there. Measured at
   48pt on the real render, "gypsy" sits 0.022 em _below_ centre before any
   correction, so shifting it down is strictly wrong.
2. **Jitter.** An offset measured from the _current_ string changes when the
   string does, so the baseline moves as the user types.

Both failures are about editable text, and the fix is two entry points with
different guarantees:

| entry point                 | measures           | use for                                                    |
| --------------------------- | ------------------ | ---------------------------------------------------------- |
| `opticalCapOffset(style)`   | the face's caps    | editable text whose alphabet includes caps (hex)           |
| `opticalDigitOffset(style)` | the face's figures | editable text that is digits and separators                |
| `opticalCenterFieldText`    | the face's figures | the same, as an `AmendLayout` hook                         |
| `opticalCenterLabelText`    | the face's caps    | a widget-owned label the control swaps as it changes state |
| `opticalCenterText`         | the run            | text the widget owns and the user cannot type into         |

`opticalCenterChildren` takes an `opticalBand` — run, cap or digit — rather than
a boolean, because the choice was never binary: those are three different
answers, and which is right is a property of the widget's text, not of the
correction.

## The rule, once the widgets had all been judged

Each widget was decided on its own evidence, and the same split kept coming
back. It is worth stating as a rule, because it is what a new widget should be
read against:

- **A value takes its own ink.** A badge's count, a progress bar's readout: the
  thing being centred is that string, and centring each on its own ink is what
  keeps two of them side by side consistent. A descender is left where metric
  centring put it.
- **A label takes the face's cap band.** A button, a tab, a menu item, a select:
  the text names a control, the eye reads it by its caps, and a row or a list of
  them must agree whatever any one of them happens to say. A descender hangs
  below centre, which is how a control label is set.
- **A glyph takes its own ink, always.** An icon or a symbol has no cap band to
  centre on. `TextStyle.glyphRole` marks one; the theme's `Icon` rungs carry the
  mark, and `glyphStyle(ts)` is the spelling for a symbol drawn in an ordinary
  text face — a numeric input's step triangles, a toast's `×`, an overflow
  panel's `⋮`. A glyph child inside a container correcting on the cap band opts
  itself back onto its own ink, so an icon button needs no special-casing at the
  call site.
- **Editable text takes a content-free band or nothing**, per the rule above.

Nothing here is measurable from the run: an arrow's ink can sit inside the cap
band exactly as a word's does. That is why the glyph case is a marker on the
style rather than a test on the string.

The content-free form cannot jitter: there is no way to pass it the live text.
`ColorFields` uses it — it is editable, but its alphabet is digits and hex — and
so do `InputDate` (a date mask: digits and separators) and `NumericInput` (its
pre-commit transform admits digits, the locale's separators and a sign).

The constrained alphabet is the entire licence, and it is the caller's to give:
`InputCfg.opticalDigitCenter` is unexported and opt-in, so a plain `Input`
cannot acquire the correction by default and an application cannot switch it on
for text it does not control.

## A control that re-labels itself takes the cap band, not the run

`Select` owns its text — a placeholder until an option is chosen, then the
options joined — so the ownership rule admits it. The _measured_ form is still
wrong there, for two reasons, both measured on the real render at size 16:

- **It is a no-op on the labels this control carries.** "Pick a language"
  descends, so its full ink band already sits about 1 device pixel low and the
  clamp leaves it alone. Its **cap band** — which is what the eye reads a
  control's label by — rode 2.5 device pixels high all the same. That is the
  reported defect, and the measured form does not touch it.
- **It would step the label on selection.** An offset taken from the run changes
  when the run does, so picking an option with a descender after one without
  would move the label vertically in a control that has not moved. Content-free
  is what a re-labelling control needs, for the same reason an editable field
  needs it.

So `Select` takes `opticalCenterLabelText`: the cap band, whatever the label
says. Measured after, the cap band of "Pick a language" and of "PICK" both land
0.5 device pixels low — level with each other, which is the property the hook
exists for. The descender then hangs below centre instead of pulling the whole
label up; that is how a control label is set, and it is deliberate.

A **menu item** takes the same band, for a different reason. Its label is the
app's and static per call site, so the ownership rule would admit the measured
form — but items stack in a list at a regular pitch, and measuring each run
would move a descender-free label down while leaving its descending neighbour
where it was. The unevenness reads down the whole menu, where the same
disagreement between two badges side by side does not. Cap band, whatever the
item says.

Two details follow from the widget rather than the rule. A menu item's label and
its shortcut hint share one row, so the correction is attached to that row and
reaches both — they have to move together or the pair reads skewed. And a
_wrapping_ label is included here, unlike `Select`'s and `Input`'s: the
exclusion there is about a block whose later lines are placed against a
top-aligned box, while a menu item's box hugs its label, so the reserved descent
that goes unused is the last line's either way. Submenu items are the wrapping
spelling, and they are exactly the ones the defect was reported on.

Measured on the real render (an open submenu, device pixels): before, the
menubar item's ink centre sat 2.0 above its highlight box's centre; after, 0.5
below — the same residual an exactly centred badge records. All three submenu
labels, descending and not, moved by the same 3 device pixels.

## Button and the tabs followed, and what it cost

`Button` measured its run until the menu work made the disagreement plain: a
descending label sat about 1.4 logical pixels higher in a `Button` than in the
`Select` or menu item beside it. It now takes the cap band too, and with it
`TabControl`, `CommandButton` and the date picker's cells, which are all built
on `Button`.

Measured at 48pt on the real render, in device pixels: a `PICK` button's ink is
dead centre in its box before and after — a cap-only label is where the two
bands already agreed — while the `gypsy` button moved down 8, landing on the
same baseline as `PICK`. That shared baseline is the whole point, and it is what
`button` and `button_descender` now record as a pair.

Two costs, both real and both bounded:

- **A digit-only button label sits slightly low** — 1.5 device pixels at 48pt,
  0.5 at 16 — because figures measure shorter than caps. The widgets that _know_
  their label is digits opt into the figure band with the unexported
  `ButtonCfg.opticalDigitLabel`: the date picker's day cells and its
  adjacent-month cells do. An application's own button cannot, deliberately —
  the alphabet is a guarantee only the widget building the label can make, the
  same reasoning as `InputCfg.opticalDigitCenter`.
- **An icon button needed the glyph role** to keep its arrow on its own ink.
  Without it the cap band would have shifted the glyph by whatever the icon
  face's `"H"` measures, or — where that face has no `H` — by the blind fallback
  ratio. The date picker's month-nav arrows are the case in the recordings: they
  must not move, and they do not.

The measured form is safe precisely where jitter cannot arise: a badge's label
changes when the app changes it, not per keystroke. Measuring the run there is
not a concession, it is what keeps neighbours consistent. Uncorrected, "128"
rides 0.071 em high while "gypsy" sits 0.022 em low — a 0.093 em spread between
two badges side by side. Centring each on its own ink, clamped so nothing is
ever pushed downward past centre, closes that to nothing for any run inside the
cap band and leaves the descender case exactly where the toolkit already had it.

Measured on screen at 24pt, before and after, in device pixels (positive = ink
rides high):

| badge label | uncorrected | cap-band for all | measured per run |
| ----------- | ----------- | ---------------- | ---------------- |
| `128`       | high        | −0.5             | +0.5             |
| `gypsy`     | −5.5        | −9.0             | −5.5             |

The middle column is the version that was rejected: it fixes the count and
drives the word 3.5 px lower than leaving it alone would.

## How it is measured

Both forms go through `(*Window).opticalOffset` (`gui/text_optical.go`), which
measures an ink box through the existing optional `textInkMeasurer` capability
(`gui/text_ink.go:23-45`) that all five backends implement, and solves for the
shift that lands the ink band's centre on the box centre. No new method on
`TextMeasurer`, and no backend change.

The content-free form probes `"H"`, flat-topped in nearly every design, so its
ink height is cap height with no overshoot to undo — the same probe go-glyph
uses internally (`fallback_fit_puregoft.go:185`).

**The probe must match the alphabet the widget guarantees.** Digits measure
shorter than caps, so a digit-only field centred on the cap band lands low. That
is not theoretical: `NumericInput` corrected with the `"H"` probe measured 1.5
device pixels low on screen at 16pt, having started 1.5 high — the same error,
reflected. Probing `"0"` instead brought it to 0.5 low, and `InputDate` to 0.0.
`ColorFields` keeps the cap probe, because a hex field does admit `A`–`F`.

The measured form has no such residual by construction, which is why the badge
at 48pt reads 0.0 px skew.

The result is memoized per window in `nsOpticalOffset`, keyed by family,
typeface, size, line spacing and — for the measured form — the run. Per window,
not per package: the measurement comes from that window's backend. The
measurement costs a shaping pass and an outline read; the pass that consumes it
is allocation-free and runs every frame.

Where the capability is absent — nil measurer in tests, WASM/canvas, an
unreadable outline — the offset falls back to the ratio measured above for the
band in question (`fallbackCapOffsetRatio` 0.0886 em, `fallbackDigitOffsetRatio`
0.0708 em), applied only to a run holding none of `descenderRunes`. A rune test
cannot know what a face does; on the fallback path it is right for Latin text
and its cost is a fraction of a pixel in a recording rather than a defect on
screen.

## How it is applied

Two forms, because widgets reach the correction at two different times:

- **`opticalCenterText(style, text)`** — an `AmendLayout` hook, a sibling of
  `centerGlyphOnInk`. Moves the arranged text child; does not feed back into
  sizing, so a control's height stays what the theme set. This is the only form
  available to eager factories such as `Badge`, which build with no `*Window`
  and therefore cannot measure.
- **`opticalCenterFieldText`** — the same hook, content-free and on the figure
  band, for an editable field holding digits and separators. `InputDate` and
  `NumericInput` reach it by setting `opticalDigitCenter` on the `Input` they
  wrap; `Input` puts it on the inner row that centres the text shape, and skips
  it for multiline, which aligns to the top and has nothing to correct. A hook
  rather than padding because these widgets do not own the field's inset:
  shifting the arranged shape leaves the control's height — and its match with
  the `Input` beside it — untouched, where spending padding from `NoPadding`
  would have grown it by twice the offset.
- **`colorFieldPadding`** — the padding form, for a widget that generates with a
  `*Window` in hand and sizes its row around the result. It gives the offset to
  the top and takes the same from the bottom, so the **total inset — and the
  field's height — is unchanged** and a channel field keeps matching the `Input`
  beside it. Spending it one-sided instead resizes the control and delivers only
  half the shift to the glyphs.

## Gate

`gui/golden_cases_test.go` records `badge`, `badge_capped`, `badge_descender`,
`button`, `button_descender`, `numeric_input`, `input_date`,
`select_placeholder_descender`, `menu_open_descender` and `progress_bar`;
`color_fields`, `button_disabled`, `tab_control`, `datepicker_disabled`,
`inputdate_disabled`, `numericinput_disabled` and the five `select_*` cases
moved. `select_placeholder_descender` records level with `select_placeholder`,
which is what a revert to the measured form would break, and `button_descender`
records level with `button` for the same reason. `badge_descender` is the
counter-case that must **not** move: it is the value rule, and it is what would
catch the cap band being applied to everything. The plain `input` case is the
second counter-case, and the one that pins the opt-in: general text must stay
metrically centred. `TextMeasurer` is nil under the golden harness, so those
files pin the fallback-ratio path: that the correction happens and by how much,
not what a given font measures.

**That nil measurer is also the harness's blind spot, and it is why the band
choices are pinned by unit tests as well.** With no ink to measure, the run, cap
and figure bands all fall back to a ratio keyed on the run's letters, so they
coincide for a cap-only label and for an icon glyph alike — a golden cannot tell
which band a widget took. `gui/text_optical_test.go` supplies a measurer that
reports a different ink box per run, which makes the three bands disagree by
construction, and asserts that a glyph-role child keeps its own ink under the
cap band and that `opticalDigitLabel` reaches the figure band. An icon-button
golden was written and then dropped for exactly this reason: it recorded the
same numbers either way. `menu_open_descender` opens a menu for one frame (menu
selection is `nsMenu` keyed by the menubar's ID) and records `Copy` and `Paste`
at the same offset from their own row: the measured form would move one and not
the other, which is the pitch defect the band exists to avoid. Its shortcut-hint
sibling is a unit test rather than a golden, because `Shortcut.String()` renders
macOS glyphs on darwin and words elsewhere, so the recording would not be
portable. `examples/optical_centring/` is the probe for what a golden cannot
show.

Two goldens changed for a reason that is not the correction: the golden harness
now pins the window clock (`setVirtualNow`) and `datePickerMonth` reads
`w.Now()` rather than `time.Now()`. The calendar rings _today_, so
`datepicker_disabled` recorded a different cell every day and reddened
overnight; the same read is what a time-travel scrub needs, since a snapshot's
calendar should ring the scrubbed day.

## Where it is applied

`Select` (cap band, via `opticalCenterLabelText` on its outer row, composed with
the focus ring through `amendAll`; the disclosure arrow sits in its own wrapper
and carries its own nudge, so the hook reaches the label only). A wrapping
multi-select is excluded — its label is a block whose later lines are placed by
the text layout, the same exclusion `Input` makes for multiline.

The **menu item** (cap band as well, on the item's own column, or on the
label+shortcut row where there is a hint; a `CustomView` item is skipped, since
its content is the app's to place). `Menubar`'s top-level items and every
submenu item are the same factory, so both take it.

**`Button`** (cap band; figure band where `opticalDigitLabel` says the label is
digits) — which is what `TabControl`, `CommandButton` and the date picker's
cells are built from, so all of those inherit it. Measured on screen, a button
label at 48pt went from 6.5 device pixels high to dead centre.

`Button` corrects from `buttonAmendLayout` rather than through
`cv.userAmendLayout`, and does it before that function's early return. Both
details matter for one reason: the path returns early for a disabled or
click-less button, so routing the correction through it would leave a disabled
label sitting a pixel above the enabled one beside it. `button` and
`button_disabled` are recorded as a pair to keep that honest. The
`AmendLayout: opticalCenterText` still on the inner `ContainerCfg` is there only
to guarantee the shape gets an events record, which a bubble-text `Button`
otherwise has no reason to allocate.

`Badge` and `ProgressBar`'s readout (their own ink — they are values, not
labels), `ColorFields`, `InputDate`, `NumericInput`.

Still uncorrected: `Input` and every editable control whose alphabet is _not_
constrained (by the rule above), and the widgets that centre text without going
through `Button` or the menu item — breadcrumb, expand panel. Those are a
straightforward extension when wanted; each needs the hook on the container that
holds its label.
