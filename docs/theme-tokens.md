# Theme tokens

Issue #755. `ThemeCfg` holds 71 fields in one flat struct. This file maps each
field to its group. The struct stays flat. The groups below are the map.

`ThemeCfg` is the input to `ThemeMaker`. Zero means default, except where the
field doc names zero as a valid choice (`SizeBorder`, `SizeScrollbarGap`,
`SizeScrollbarGapEnd`). `Color`, `Padding`, and `Sizing` flag themselves. Use
the constructors for them.

## Type

Base text style, font families, and the size ladder.

- `TextStyleDef` — base text style. All derived text roles fall back to it.
- `MonoFontFamily` — font family for code and mono text.
- `IconFontFamily` — font family for themed icon styles. Empty falls back to
  `IconFontName`.
- `SizeTextTiny`, `SizeTextXSmall`, `SizeTextSmall`, `SizeTextMedium`,
  `SizeTextLarge`, `SizeTextXLarge` — text size ladder. Zero takes the built-in
  default.

## Shape

Border width and the corner radius ladder.

- `SizeBorder` — border width. Zero is a valid choice (no border).
- `Radius`, `RadiusSmall`, `RadiusMedium`, `RadiusLarge` — radius ladder.
  `RadiusLarge` is reserved. No widget reads it yet.

## Spacing

Padding ladder, form density, and the gap ladder.

- `Padding`, `PaddingSmall`, `PaddingMedium`, `PaddingLarge` — padding ladder.
  These size the gap between things.
- `PaddingField` — text inset inside a form control. It sets the control height.
  It is not part of the ladder.
- `SizeFieldMinWidth` — minimum width floor for a text-bearing form control.
- `SpacingTight`, `SpacingSmall`, `SpacingMedium`, `SpacingLarge` — gap ladder
  between things.

## Control sizes

Scroll behavior and per-control geometry.

- `ScrollMultiplier`, `ScrollDeltaLine`, `ScrollDeltaPage` — scroll distance
  scaling.
- `SizeSwitchWidth`, `SizeSwitchHeight` — switch geometry.
- `SizeRadio` — radio geometry.
- `SizeScrollbar`, `SizeScrollbarMin` — scrollbar thickness and minimum length.
- `SizeScrollbarGap`, `SizeScrollbarGapEnd` — scrollbar insets. These are `Opt`
  because zero is a valid choice.
- `SizeProgressBar` — progress bar thickness.
- `SizeSlider`, `SizeSliderThumb` — slider track thickness and thumb size.
- `SizeSeparator` — separator thickness.

## Color scheme

Surfaces, borders, text roles, selection, accent ramp, and semantic colors.
Unset derives. A theme states only the override.

- `ColorBackground`, `ColorPanel`, `ColorInterior` — surfaces.
- `ColorHover`, `ColorFocus`, `ColorActive` — interaction states.
- `ColorBorder`, `ColorBorderFocus`, `ColorSeparator` — borders and dividers.
- `ColorTextSecondary`, `ColorTextLabel`, `ColorTextDisabled`,
  `ColorTextPlaceholder` — de-emphasis roles. Unset derives from
  `TextStyleDef.Color` and the theme polarity.
- `ColorSelect`, `ColorTextOnSelect` — selection fill and the text on it.
  Contrast is a property of the pair.
- `ColorAccent`, `ColorAccentHover`, `ColorAccentPressed`, `ColorAccentSubtle`,
  `ColorTextOnAccent` — accent ramp. `ColorAccent` is the single decision. The
  rest derive from it. Unset `ColorAccent` resolves to `ColorSelect` and back.
- `ColorSuccess`, `ColorWarning`, `ColorError` — semantic colors.
- `ColorSuccessSubtle`, `ColorWarningSubtle`, `ColorErrorSubtle` — subtle
  companions for validation fills. No consumers yet.

## Elevation

- `ShadowPopover` — shadow for menus, dropdowns, tooltips, and toasts.
- `ShadowDialog` — shadow for modals and the command palette.
- Nil means the theme does not describe elevation. Do not write through the
  pointer. One value is shared by all shapes in the window.

## Focus

- `FocusRing` — focus glow drawn outside the control bounds. Nil leaves the
  border ring in charge.

## Window chrome and flags

- `Name` — theme name. Names are not unique. Key theme-dependent state on
  `Theme.id`.
- `Sounds` — sound set. Zero is silent.
- `TitlebarDark` — dark titlebar flag.
- `FillBorder` — fill border flag.
