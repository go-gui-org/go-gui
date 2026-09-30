# Theme tokens

Issue #755. `ThemeCfg` holds 71 fields in one flat struct. This file maps each
field to its group. The struct stays flat. The groups below are the map.

`ThemeCfg` is the input to `ThemeMaker`. Zero means default, except where the
field doc names zero as a valid choice (`SizeBorder`, `SizeScrollbarGap`,
`SizeScrollbarGapEnd`). `Color`, `Padding`, `Sizing`, `Spacing`, `Radius` and
`Border` flag themselves. Use the constructors or roles for them.

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
  `RadiusLarge` is the top tier for floating surfaces (dialogs, dropdowns,
  popovers). Cfg fields take it as the `RadiusLarge` role, which resolves to the
  active theme's step at build time.

## Spacing

Padding ladder, form density, and the gap ladder.

- `Padding`, `PaddingSmall`, `PaddingMedium`, `PaddingLarge` — padding ladder.
  These size the gap between things.
- `PaddingField` — text inset inside a form control. It sets the control height.
  It is not part of the ladder.
- `PaddingButton` — label inset inside a Button. At body size it gives the same
  height as `PaddingField`. It is not part of the ladder.
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

## Text roles (contract)

Issue #846. The text roles are fields on `Theme`. Apps read them to style their
own widgets. `ThemeMaker` fills them from the `ThemeCfg` type tokens above. The
role names are a contract: code outside this repo uses them.

De-emphasis roles. Each one names why text is quiet (issue #335).

| Role                   | Purpose                                  | Added   |
| ---------------------- | ---------------------------------------- | ------- |
| `TextStyleSecondary`   | Supporting text beside primary text      | v0.62.0 |
| `TextStyleLabel`       | Text that names a nearby value           | v0.62.0 |
| `TextStyleDisabled`    | Text in a control the user cannot use    | v0.62.0 |
| `TextStylePlaceholder` | Text in place of a value not yet entered | v0.62.0 |

Purpose roles. Each one names what the text is for (issue #734). Size is the
`ThemeCfg` ladder rung.

| Role                    | Purpose                     | Face    | Rung     | Added   |
| ----------------------- | --------------------------- | ------- | -------- | ------- |
| `TextStyleDisplay`      | Largest heading             | bold    | XLarge   | v0.78.0 |
| `TextStyleTitle`        | Heading, dialog title       | bold    | Large    | v0.78.0 |
| `TextStyleTitleSmall`   | Group, toast, table heading | bold    | Medium   | v0.78.0 |
| `TextStyleBodyLarge`    | Large body text             | regular | Large    | v0.78.0 |
| `TextStyleBody`         | Body and value text         | regular | Medium   | v0.78.0 |
| `TextStyleBodySmall`    | Small body text             | regular | Small    | v0.78.0 |
| `TextStyleCaption`      | Caption, badge              | regular | XSmall   | v0.78.0 |
| `TextStyleCaptionSmall` | Smallest caption            | regular | Tiny     | v0.78.0 |
| `TextStyleCode`         | Code                        | mono    | Medium+1 | v0.78.0 |
| `TextStyleCodeSmall`    | Small code                  | mono    | XSmall+1 | v0.78.0 |
| `TextStyleCodeTiny`     | Smallest code               | mono    | Tiny+1   | v0.78.0 |
| `TextStyleIconXLarge`   | Icon glyph                  | icon    | XLarge   | v0.78.0 |
| `TextStyleIconLarge`    | Icon glyph                  | icon    | Large    | v0.78.0 |
| `TextStyleIconMedium`   | Icon glyph beside body text | icon    | Medium   | v0.78.0 |
| `TextStyleIconSmall`    | Icon glyph                  | icon    | Small    | v0.78.0 |
| `TextStyleIconXSmall`   | Icon glyph                  | icon    | XSmall   | v0.78.0 |
| `TextStyleIconTiny`     | Icon glyph                  | icon    | Tiny     | v0.78.0 |

Status roles. Each one is body text in a status hue (issue #861). The color is
the status color (`ColorError`, `ColorSuccess`, `ColorWarning`) moved on
lightness until it reaches 4.5:1 contrast on `ColorBackground` and `ColorPanel`.
The status colors are tuned as fills and are too faint as text on light themes.

| Role               | Purpose                          | Face    | Rung   | Added  |
| ------------------ | -------------------------------- | ------- | ------ | ------ |
| `TextStyleError`   | Validation message, failure text | regular | Medium | Unrel. |
| `TextStyleSuccess` | Saved or completed notice        | regular | Medium | Unrel. |
| `TextStyleWarning` | Caution text                     | regular | Medium | Unrel. |
| `TextStyleLink`    | Link text, underlined            | regular | Medium | Unrel. |

`TextStyleLink` is the same rule applied to `ColorSelect` (issue #863).
`ColorSelect` stays as it is for fills. `RichLink` and markdown links take only
the link color, so a link keeps the size of the text around it.

Contrast floors (issue #863). Every text role has a minimum contrast, listed in
`contrastFloors` (`gui/theme_contrast.go`): 4.5:1 on `ColorBackground` and
`ColorPanel`, and 3:1 for `TextStylePlaceholder` on `ColorInterior`.
`TextStyleDisabled` has no floor. `ThemeMaker` and `WithColors` raise a derived
color to its floor: the quiet roles by alpha, the status and link roles by
lightness. A color the app states (`TextStyleDef.Color`, a `ColorText*` field, a
role set by hand) is never moved. `DebugLowContrast` reports it instead.

Rules:

1. Adding a role is not a breaking change.
2. Removing a role, renaming it, or changing its purpose is a breaking change.
3. To rename a role, keep the old field for one release. Mark it
   `// Deprecated: use <new>.` and fill it in `fillTextRungs`
   (`gui/theme_maker_rungs.go`) next to the new field. Remove it in the next
   release, with a `**BREAKING:` entry.
4. At 1.0 the role set is frozen.

The gate: `gui/testdata/theme_surface.golden` lists every exported field of
`Theme`, `ThemeCfg` and `TextStyle`. `TestThemeSurface` keeps it in step with
the code. `make theme-surface-check` (part of `make check` and CI) fails a
branch that removes a line from the golden, unless the branch adds a
`**BREAKING:` entry under `## [Unreleased]` in `CHANGELOG.md`. Re-recording the
golden with `-update` does not pass the gate; the removed line stays in the
diff. The gate cannot see a change of purpose. Rule 2 covers that; review it by
hand.
