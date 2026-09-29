# Spacing and padding roles: snap call sites, gate numbers

Issue #851.

- **Status:** landed in go-gui (audit mode, examples migrated). Sibling repos
  migrate and gate one at a time after a go-gui release.
- **Extends:** `docs/style-guide.md` (spacing section),
  `docs/specs/visual-refresh.md` §3.3.
- **Breaking:** no. No exported API or default value changes.

## Problem

The theme names its gaps (`SpacingTight/Small/Medium/Large` = 2 / 6 / 14 / 28)
and its insets (`PaddingSmall/Medium/Large` = 6 / 14 / 22, plus `PaddingField`
and `PaddingButton`). Call sites wrote numbers instead. In go-gui's examples
there were about 186 `Spacing: gui.SomeF(n)` numbers and 178 padding numbers.
The usual gaps were 8, 12 and 16, and none of them is a step. Because 53 of the
63 files with spacing numbers were examples, the examples taught the habit.

No audit looked at gaps. `-mode visual` checks dimming alphas and type-size
steps only, and only in `gui/view_*.go`.

## Decision

Keep the scale. Move each call site to the step that fits its meaning. Then gate
new numbers with `ergonomics-audit -mode spacing`.

| Design                                  | Visual change                                       | API added                         | Failure when misused                                     |
| --------------------------------------- | --------------------------------------------------- | --------------------------------- | -------------------------------------------------------- |
| **A. Keep 2/6/14/28, snap by meaning**  | 0 of 286 `gui/` goldens; examples move a few pixels | none                              | a non-gap number needs a marker and a reason             |
| B. Add in-between steps (2/4/6/10/14/…) | none in `gui/` if old values stay                   | 3 consts, 3 Theme and 3 Cfg seeds | close steps bring back pick-by-number; no rule for 10/14 |
| C. Even 4-pt scale (4/8/12/16/24)       | most goldens; documented values break               | renames or new steps              | undoes §3.3; label and field read as equally close       |

A was picked. The scale has a stated reason (visual-refresh §3.3: each step
about doubles, so group membership reads at a glance). The call sites were out
of line, not the scale.

## The rule

`tools/ergonomics-audit/spacing.go`. Three findings:

1. `Spacing: SomeF(n)` or `Some[float32](n)` with a number n > 0. Zero passes.
2. `PadAll`, `NewPadding` or `PadVH` whose arguments are all numbers, at least
   one > 0. A call that mixes in a step constant passes, because it already
   names its step.
3. Rule 2 as the `Padding` of a `ButtonCfg`. The message says to delete the
   field, so `Theme.PaddingButton` applies (#850).

A number whose nearest enclosing composite literal is a `DrawCanvasCfg` passes.
It is a plot margin (room for axes and labels), not a ladder inset. The list of
exempt Cfg types is `spacingExemptCfgs`.

A number that is not a gap (a 1px hairline, a bevel edge, a skin that imitates
another platform, card-face geometry) carries the same-line marker
`// ergonomics-audit:spacing` and a reason. It prints as deferred and does not
gate.

Scope: in go-gui, `gui/view_*.go` and `examples/`. The rest of `gui/` defines
the steps, so a number there is a definition. In any other repo, every non-test
file is scanned. The mode reads `go.mod` to tell the two apart.

## How call sites were snapped

Every site was read, not rewritten by a script from its number.

- Gaps: 2–3 → Tight. 4–6 → Small. 8 → Small for a tight pair (a label and its
  value, a caption and its image, a keypad), else Medium. 10–16 → Medium, except
  a control and its label (Small). 20–40 → Large, except a row of sibling
  controls (Medium).
- Insets: 4 → `PaddingXSmall`, 6–8 → `PaddingSmall`, 10–16 → `PaddingMedium`,
  20–40 → `PaddingLarge`. An uneven inset uses the `Pad*` constants per side.
- Plain example buttons: the `Padding` field is deleted.
- Styled buttons in the games (large call-to-action buttons with their own
  colors and borders) keep a larger inset, built from `Pad*` constants.
- Examples use the fixed constants and presets (`gui.SpacingMedium`,
  `gui.PaddingMedium`), not `t.SpacingMedium`. Four examples run on
  `WithPadding(false)`, which sets the theme's padding steps to zero; the
  presets keep their value there.
- The nine `gui/view_*.go` sites are all density geometry (a tab strip edge, a
  separator inset, 1px hairlines, a copy-button inset). They carry the marker,
  so no golden moves.

## Rejected Approaches

- **C, an even 4-pt scale.** It undoes visual-refresh §3.3, moves almost every
  golden and breaks documented values. It matches the call sites only because
  they never used the steps.
- **B, in-between steps.** It adds API that must stay stable (#846) to cover
  about 42 uses, and it gives up one meaning per step.
- **A codemod to the nearest step.** It puts the right number on the wrong
  meaning: a gap of 8 between two buttons is Medium (14), not Small (6).
- **A marker on every canvas margin.** The rule knows the Cfg type, so an
  exemption list is shorter and cannot be forgotten at a new chart.
- **A third rule inside `-mode visual`.** Visual scans only `gui/view_*.go`.
  This rule must also scan whole sibling repos, so it is its own mode.
- **Theme-following steps (`t.SpacingMedium`) in the examples.** The padding
  steps go to zero under `WithPadding(false)`, which four examples use.

## Follow-up: steps follow the theme (#866)

- **Status:** landed in go-gui. Siblings migrate after the release (about 48
  sites, mechanical).
- **Breaking:** yes. Every Cfg spacing field changes type.

After #851 the examples wrote `gui.SomeF(gui.SpacingLarge)`. That is awkward to
read, and it has a real defect: the float constant is copied into the Cfg, so a
`ThemeCfg` that changes `SpacingLarge` does not move the gap.
`ThemeCfg.Spacing*` could be set, but no call site that named a step used it.

| Design                                                                | Allocs | Surface added                      | Migration                     | Misuse failure                         |
| --------------------------------------------------------------------- | ------ | ---------------------------------- | ----------------------------- | -------------------------------------- |
| A. Pre-wrapped `Opt` values (`GapLarge = SomeF(SpacingLarge)`)        | 0      | 4 vars                             | none                          | still ignores a custom theme, silently |
| **B. Self-flagging `Spacing` type with roles resolved at build time** | 0      | 1 type, `SpacingPx`, 4 role values | 15 fields, ~470 in-repo sites | a raw float does not compile           |
| C. `t.Gap(gui.Large)` method that returns an `Opt`                    | 0      | 1 method, 1 enum                   | none                          | needs the theme at every site; wordier |

B was picked. It fixes the theme bypass, not only the spelling. It also matches
`Padding`, `Color` and `Sizing`, which self-flag. `Spacing` holds a fixed px
value, a role and a set flag. `Or(def)` resolves a role against `guiTheme`,
which is the theme installed for the window being generated (the same read that
the `default*Style` mirrors use). So `Themed` scopes work too. The seeds are the
unexported `gapTight/Small/Medium/Large`.

This does not undo the rejection of theme-following steps above. That rejection
was about padding: `WithPadding(false)` sets the padding steps to zero. It does
not touch the spacing steps.

### Rejected Approaches (follow-up)

- **A, pre-wrapped `Opt` values.** It fixes the spelling and hides the theme
  bypass.
- **C, a theme method.** It is not shorter than `SomeF`, and it needs the theme
  in hand at every call site.
- **A role marker inside `Opt[float32]`, such as a negative value.** `Get(def)`
  would return the marker to any code that reads the value directly.
- **Keep the float constants exported next to the roles.** Two spellings of one
  step lead back to the fixed copy. Code that needs arithmetic reads
  `w.Theme().SpacingLarge`.
