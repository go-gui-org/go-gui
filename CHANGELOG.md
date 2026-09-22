# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to
[Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- **Per-widget theme overrides (#754)** — `Theme.With` applies a geometry patch
  for one widget class (`ButtonPatch`, `InputPatch`, `SelectPatch`,
  `DialogPatch`, `ContainerPatch`), so an app states "all buttons use radius 2"
  once in the theme instead of on each call site. Each patch holds padding,
  border and radius; colors stay with `WithColors`. The patch applies at build
  time to the private styles, so frames cost nothing new, and it survives
  `WithPadding`, `WithBorders`, `AdjustFontSize` and `WithColors`. An unknown
  patch type panics and names the type. A NaN, infinite or negative length in a
  patch becomes 0. Start with these five classes; more land on demand.
- **Animated theme switch (#753)** — `Window.SetThemeTransition(d)` makes later
  `SetTheme` calls and `FollowSystemAppearance` changes fade over `d` instead of
  switching in one frame. Only colors blend; sizes, radii, fonts and `WithExt`
  values take the new theme's values on the first frame, so the fade never
  re-runs layout mid-way. `Window.Theme` returns the new theme at once. A fade
  frame allocates nothing (one reused scratch theme per window). The fade is
  skipped before a window's first frame, and when the backend reports
  `PrefersReducedMotion`; no backend reports it yet. Zero, the default, keeps
  the old one-frame switch.
- **Windows follow the OS light/dark setting (#752)** —
  `FollowSystemAppearance(light, dark)` pins the matching theme from the current
  OS setting and re-pins on every OS change, until an explicit `SetTheme` ends
  following. The titlebar does not follow yet: every backend's `TitlebarDark` is
  a no-op. The `SystemAppearance` query plus `OnSystemAppearance` callback stay
  public for apps with custom logic. Queries where the OS reports no setting
  (nil platform, non-GNOME Linux desktops) keep the app theme. Sources: macOS
  `effectiveAppearance` (KVO), Windows registry + `WM_SETTINGCHANGE`, Linux
  `gsettings` + monitor child, web `prefers-color-scheme`, iOS trait collection,
  Android uiMode push from the Kotlin host. The Windows, X11, web and Android
  paths are unverified on hardware.

### Changed

- **BREAKING: `ColorSet` has `Selected` and `Disabled` slots; the flat tab and
  crumb fields are deleted (#741)** — selected and disabled colors were the last
  interaction colors outside `ColorSet`, so each widget ordered them by hand. A
  selected tab pinned its selected color into every fill slot and did not react
  to hover or press. `ColorSet.Selected` is now the resting fill of a selected
  element, and hover and press on it are derived with the same OKLCH lightness
  step as the accent ramp (#732), so a selected tab now reacts to the pointer.
  `ColorSet.Disabled` is the fill of a disabled element, and it **replaces** the
  renderer's half-alpha dim for that fill: the color is painted as given. The
  border and text still dim. With `Disabled` unset, the old rule stays (Base,
  dimmed). `Flat` leaves both new slots unset. `Disabled` is honored by `Button`
  and everything built on it (tabs included) and by `Breadcrumb`, also when the
  disabled flag comes from an ancestor. Other widgets ignore it for now. The
  theme states the disabled tab color pre-dimmed, so default tabs look the same
  (the golden tests pass unchanged). Migration:

  | Before                              | After                                                                               |
  | ----------------------------------- | ----------------------------------------------------------------------------------- |
  | `TabControlCfg.ColorTabSelected`    | `ColorsTab.Selected`                                                                |
  | `TabControlCfg.ColorTabDisabled: c` | `ColorsTab.Disabled: c` (now undimmed; to keep the old look, use `c` at half alpha) |
  | `BreadcrumbCfg.ColorCrumbSelected`  | `ColorsCrumb.Selected`                                                              |
  | `BreadcrumbCfg.ColorCrumbDisabled`  | `ColorsCrumb.Disabled`                                                              |

  The scan over go-charts, go-edit, go-kite, go-map, go-term, go-speedtest and
  go-shirei found no uses of the deleted fields.

- **BREAKING: `ColorSet.Pick` and `ColorSet.Resolved` are exported; data grid
  rows and list rows use them (#741)** — a widget outside `gui/` could not call
  the state picker, so the data grid kept its own copy of the color fallback and
  its own hover rule, and a selected row did not react to the pointer.
  `ColorSet.Pick(PickState)` returns the fill and border for one state in the
  same order every widget in `gui/` uses, and `ColorSet.Resolved(theme)` applies
  the fallbacks `Pick` needs first. A selected grid row now takes a lighter wash
  under the pointer. Combobox and command palette rows use the same rule: the
  pointer wins over the keyboard highlight, and a selected row hovered moves one
  lightness step. Migration:

  | Before                               | After                                                                         |
  | ------------------------------------ | ----------------------------------------------------------------------------- |
  | `DataGridCfg.ColorRowSelectedSubtle` | `DataGridCfg.ColorsRow.Selected`                                              |
  | `DataGridCfg.ColorRowSelected`       | `DataGridCfg.ColorsRow.Selected` (the full accent is no longer used for rows) |
  | `DataGridStyle.ColorRowSelected*`    | `DataGridStyle.ColorsRow.Selected`                                            |

  The scan over the siblings found no uses of the removed fields.

- **BREAKING: ListBox and Table rows use `Colors.Selected`; the
  `ColorSelectSubtle` fields are deleted (#744)** — both widgets painted the
  selected row by hand, so a selected row ignored the pointer. The row fill now
  goes through `ColorSet.pick` with the selected and hovered states: a selected
  row hovers one OKLCH step lighter, the same as data grid rows. The wash is
  `Colors.Selected`, which the theme sets from the accent subtle color, so
  resting rows look the same (the golden tests pass unchanged). The Table
  keyboard-active row is the Focus state, and the theme sets `Colors.Focus` to
  the hover fill, so it keeps its tint. Migration:

  | Before                           | After                          |
  | -------------------------------- | ------------------------------ |
  | `ListBoxCfg.ColorSelectSubtle`   | `ListBoxCfg.Colors.Selected`   |
  | `TableCfg.ColorSelectSubtle`     | `TableCfg.Colors.Selected`     |
  | `ListBoxStyle.ColorSelectSubtle` | `ListBoxStyle.Colors.Selected` |
  | `TableStyle.ColorSelectSubtle`   | `TableStyle.Colors.Selected`   |

  `ListBoxCfg.ColorSelect` and `TableCfg.ColorSelect` still work: a set value
  becomes `Colors.Selected` when that slot is unset. Prefer `Colors.Selected`
  directly.

  The scan over go-charts, go-edit, go-kite, go-map, go-term, go-speedtest and
  go-shirei found no uses of the deleted fields (the one `ColorSelect` hit in
  go-charts reads the theme-level color, which is unchanged).

### Fixed

- **Windows notifications use native APIs and application identity (#745)** —
  Windows 10 and later send WinRT notifications under an identity bound to the
  source executable. Notifications keep the application name in Notification
  Center after exit, and clicking one opens that executable. Registration needs
  no Start menu shortcut, reuses developer identities across `go run` builds,
  and retries after a failure. Sending a notification no longer launches
  PowerShell or creates temporary tray icons; unit tests no longer display
  desktop notifications.

## [v0.78.0] - 2026-09-21

### Added

- **`gui.WithExt` and `gui.Ext` (#733)** — a sibling widget package can now
  store its own style value in the `Theme`, keyed by the exact value type.
  `WithExt` returns a new theme with a fresh id, so the install fast path and
  `Themed` scoping cover the value. `Ext` reports a missing value as the zero
  value and false. Stored values must be immutable: theme copies share the
  backing map. Every rebuild path (`WithColors`, `WithPadding`, `WithBorders`,
  `AdjustFontSize`) carries the values, so a sibling derives once at build time
  instead of syncing a parallel struct by hand.
- **`gui.ThemeRegister` and `gui.ThemeRegisteredNames` (#713)** — an app that
  builds a custom theme with `ThemeMaker` can now register it, so `ThemePicker`
  lists it, and can enumerate registered names to build its own picker. Empty
  and whitespace-only names are rejected (registration reports false);
  duplicates keep the long-standing overwrite.
- **`gui.RichFootnote`** — the footnote-marker constructor matches its siblings
  (`RichRun`, `RichLink`, `RichBr`, `RichAbbr`) and is now exported: it was the
  only one stuck unexported, so an app could not build a footnote marker outside
  `gui`.
- **`gui.Debug` reports an `Interactive` button that the keyboard cannot use
  (#658)** — a custom button built with `gui.Interactive` needs `Focusable`,
  `ClickOnSpace` and `ClickOnEnter` on its root next to `OnClick`. Without one
  of them the button worked with the mouse and did nothing from the keyboard,
  and nothing said so. `gui.Debug` now reports the root under `DebugMissingIDs`
  and names the missing fields.
- **Custom toggle, checkbox and radio examples (#661)** — three new examples,
  `custom_toggles`, `custom_checkboxes` and `custom_radios`, port go-shirei's
  custom-control demos. Each look is a `gui.Interactive` that reads hover and
  press state, while the app owns the checked or selected value. They show that
  these controls need no API beyond `gui.Interactive` and `ContainerCfg`:
  switch, checkbox and radio roles with their checked or selected state,
  arrow-key navigation in a radio group, and one Tab stop per group.
- **Custom slider and text input examples** — `custom_sliders` ports go-shirei's
  Apple, Material and Windows XP sliders with a mouse lock drag, arrow keys and
  floating parts placed from the value. `custom_textinputs` wraps a `gui.Input`
  with its chrome turned off in Material and Windows XP fields that restyle when
  the inner Input has focus. Both use only the public API, and each lists what
  it had to write by hand, the input for a later helper design.
- **`ContainerCfg.OnMouseScroll` (#664)** — a plain container now takes the
  mouse wheel. `OnScroll` fires only on a scrollable container, so a custom
  slider, stepper or picker had no way to react to the wheel. The new field is
  called with shape-relative coordinates and the wheel amount in
  `ctx.Event.ScrollX` and `ScrollY`, the same as `DrawCanvasCfg.OnMouseScroll`.
  A handler that does not call `ctx.Consume()` lets the wheel also scroll the
  scroll container below.
- **`ContainerCfg.A11YValue` (#664)** — a custom range control built on a
  container, such as a slider with `AccessRoleSlider`, can now tell a screen
  reader its value. Set `A11YValue: gui.AccessValue{Now: v, Min: 0, Max: 1}`.
  Before, only the stock `Slider` and `ProgressBar` could report a value,
  because the field that carries it was unexported. An `AccessValue` with `Min`
  equal to `Max`, including the zero value, reports no value. The
  `custom_sliders` example now sets it.
- **`InteractionState.FocusWithin` (#664)** — a `gui.Interactive` look can now
  see that a widget inside it has keyboard focus. It is true when the widget or
  one of its ID-bearing descendants is focused, the same rule `Hovered` and
  `Pressed` use. `Focused` is unchanged and still means the widget itself. A
  wrapper around an `Input` no longer builds the Input's effective ID by hand to
  draw its focus look; the `custom_textinputs` example now reads `FocusWithin`.
- **`SliderCfg.Look` (#664)** — a slider can now be drawn from parts the app
  builds. `Look` gets the hover, press and focus state and the value as a
  fraction, and returns `SliderParts{Track, Fill, Handle}`. The track stays in
  the layout; the slider moves the handle to the value and sets the fill's
  length after layout, so a slider with `Fill` sizing works. Drag, keys, wheel,
  rounding, vertical mode and the screen reader value stay the stock slider's,
  so a custom look no longer writes them again. The `custom_sliders` example now
  uses it, and gains the mouse wheel. See `docs/specs/slider-look-hook.md`.
- **`ScrollbarCfg.Thumb` and `ScrollbarCfg.Track` (#664)** — a scrollbar's thumb
  and gutter can now be drawn by views the app builds. Each hook gets
  `ScrollbarState` with the hover and press state and the size of its part. The
  scrollbar still sizes and moves the thumb, runs the drag, jumps on a gutter
  press and hides the thumb when nothing overflows. Before, only colors and
  radii could change, so a raised or gripped thumb could not be built. The new
  `custom_scrollbars` example shows three looks. See
  `docs/specs/scrollbar-look-hook.md`.
- **Public locale API, as `widget_locale.md` always showed** — the format types
  (`NumberFormat`, `DateFormat`, `CurrencyFormat`, `TextDirection`,
  `NumericAffixPosition` with `AffixPrefix`/`AffixSuffix`), the ten preset
  locales (`LocaleEnUS`, `LocaleDeDE` and friends), every UI string (`StrOK`,
  `StrYes`, `StrSearch`, `StrPage` and the rest), package-level `SetLocale`,
  `LocaleAutoDetect`, `LocaleRegisteredNames` and `LocaleT` are now exported.
  Dialog buttons read the locale, so a German app shows German buttons. The
  doc's snippets now compile as written.
- **Settable print job options** — every `PrintJob` field the backends honor is
  now settable: `Paper` (`PaperLetter`, `PaperLegal`, `PaperA4`, `PaperA3`),
  `Orientation` (`PrintPortrait`, `PrintLandscape`), `Margins`, `Source`
  (`PrintSourceCurrentView`, `PrintSourcePDFPath`), `ScaleMode`
  (`PrintScaleFitToPage`, `PrintScaleActualSize`), `Duplex` (`PrintDuplexOff`,
  `PrintDuplexLongEdge`, `PrintDuplexShortEdge`), `ColorMode`
  (`PrintColorModeColor`, `PrintColorModeGrayscale`) and the
  `SourceWidth`/`SourceHeight` viewport overrides, plus an exported
  `PrintExportStatus` (`PrintExportOK`, `PrintExportError`) with
  `PrintExportResult.IsOk`. Before, these fields and values were unexported, so
  callers got `NewPrintJob` defaults with no way to change them. The write-only
  raster knobs (`rasterDPI`, `jPEGQuality`), which no renderer read, are removed
  instead of frozen.
- **`gui.FloatMiddleLeft`** — the middle-left float anchor was the only one of
  the nine stuck unexported, so an app could not anchor a float to its parent's
  middle-left edge. It is now exported like its siblings; the set ships whole.
- **DatePicker month/year roller gains a confirm button** — the roller could
  only be dismissed with Escape (or the undiscoverable month-label toggle),
  while the header kept two prev/next arrows that did nothing. A check button
  now takes the arrows' place while the roller is open and closes it without
  moving the view. Focus returns to the picker, so keyboard navigation continues
  where it left off.
- **`gui.OpticalCenterText`** — the `AmendLayout` hook `Button` and `Badge`
  centre their labels with is now exported, so an app-owned static label in a
  centred container can sit on its optical centre too. It moves each direct text
  child down by its own measured ink offset, never by padding, so the control
  height is unchanged. Only for text the app owns and the user cannot type into;
  the showcase layout demos use it for their fixed-size box labels.
- **`ToastAnchor` and the markdown header/emphasis grids are exported (#698)** —
  `ToastStyle.Anchor` was an exported field of an unexported type, so an app
  could only set it with a magic number. The type and its four constants are now
  `ToastAnchor`, `ToastTopLeft`, `ToastTopRight`, `ToastBottomLeft`,
  `ToastBottomRight`. `MarkdownStyle` had the same half-export in miniature:
  `H2` was public while `h1`, `h3`–`h6` and `boldItalic` were not. The full
  header grid (`H1`–`H6`) and `BoldItalic` are now exported, matching the closed
  Theme face grids; block and table geometry stays structural and unexported.

### Changed

- **Accent and danger ramps derive in OKLCH instead of HSL (#732)** — hover and
  pressed states move `±0.10` on the OKLCH lightness axis, keeping chroma and
  hue, instead of `±0.12` on the HSL axis. HSL lightness is not perceptual, so
  the old step read large on a blue accent and small on a yellow or green one;
  one OKLCH step reads the same on every hue. The step is calibrated so the
  default accents keep their magnitude (`#89A7DE`/`#3163CE` dark,
  `#7394CC`/`#0D4FBE` light), and a shifted color that leaves the sRGB gamut
  sheds chroma until it fits rather than clipping its hue. No migration: no
  field or function changes, only derived values. `ColorAccentSubtle`
  (alpha-only) and the `textOnAccent` threshold are untouched.

- **BREAKING: numbered text rungs are semantic roles (#734)** — the closed 6x6
  grid (`Theme.N1`–`N6`, `B1`–`B6`, `I1`–`I6`, `BI1`–`BI6`, `M1`–`M6`,
  `Icon1`–`Icon6`) is removed. A number said how big the text was, never what it
  was for, so two callers with the same purpose picked different rungs and
  drifted apart. Each purpose is now one role, derived from the same size
  ladder, so every migrated call site renders pixel-identically (the golden
  tests pass unchanged). Migration:

  | Before                        | After                                                                                                                        |
  | ----------------------------- | ---------------------------------------------------------------------------------------------------------------------------- |
  | `theme.B1`                    | `theme.TextStyleDisplay`                                                                                                     |
  | `theme.B2`                    | `theme.TextStyleTitle`                                                                                                       |
  | `theme.B3`                    | `theme.TextStyleTitleSmall`                                                                                                  |
  | `theme.N2`                    | `theme.TextStyleBodyLarge`                                                                                                   |
  | `theme.N3`                    | `theme.TextStyleBody`                                                                                                        |
  | `theme.N4`                    | `theme.TextStyleBodySmall`                                                                                                   |
  | `theme.N5`                    | `theme.TextStyleCaption`                                                                                                     |
  | `theme.N6`                    | `theme.TextStyleCaptionSmall`                                                                                                |
  | `theme.M3`                    | `theme.TextStyleCode`                                                                                                        |
  | `theme.M5`                    | `theme.TextStyleCodeSmall`                                                                                                   |
  | `theme.M6`                    | `theme.TextStyleCodeTiny`                                                                                                    |
  | `theme.Icon1`–`Icon6`         | `theme.TextStyleIconXLarge` … `theme.TextStyleIconTiny`                                                                      |
  | `theme.N1`                    | `theme.TextStyleDisplay.Roman()`                                                                                             |
  | `theme.B4`                    | `theme.TextStyleBodySmall.Bold()`                                                                                            |
  | `theme.B5`                    | `theme.TextStyleCaption.Bold()`                                                                                              |
  | `theme.B6`                    | `theme.TextStyleCaptionSmall.Bold()`                                                                                         |
  | `theme.I3`                    | `theme.TextStyleBody.Italic()`                                                                                               |
  | `theme.I4`                    | `theme.TextStyleBodySmall.Italic()`                                                                                          |
  | `theme.BI3`                   | `theme.TextStyleBody.Italic().Bold()`                                                                                        |
  | `theme.BI4`                   | `theme.TextStyleBodySmall.Italic().Bold()`                                                                                   |
  | `theme.I1`/`I2`/`I5`/`I6`     | `theme.TextStyleDisplay`/`Title`/`Caption`/`CaptionSmall` + `.Italic()`                                                      |
  | `theme.BI1`/`BI2`/`BI5`/`BI6` | as `I1`/`I2`/`I5`/`I6`, + `.Bold()` (order-independent)                                                                      |
  | `theme.M1`/`M2`/`M4`          | `theme.Mono(theme.TextStyleDisplay.Roman())`, `theme.Mono(theme.TextStyleBodyLarge)`, `theme.Mono(theme.TextStyleBodySmall)` |

  `Bold`, `Italic` and `Roman` are methods on `TextStyle`: pure face maps,
  order-independent and idempotent, leaving size and color alone. `Mono` is a
  method on `Theme` (only the theme knows `ThemeCfg.MonoFontFamily`) and applies
  the mono +1 optical compensation; overriding the size afterwards discards it.
  Decomposing a role (`role.Color`, `role.Size`) works exactly as decomposing a
  rung did. The sibling migration (go-edit and go-term read `M5`/`M6`, go-charts
  reads `N`/`B` rungs) rides the breaking-branch sync-siblings pass before this
  ships.

- **BREAKING: per-widget `Theme` styles are private, customize through
  `ThemeCfg` (#735)** — `Theme.ButtonStyle` (with `ButtonStylePrimary`,
  `ButtonStyleGhost`, `ButtonStyleDanger`), `Theme.ContainerStyle`,
  `Theme.InputStyle` and `Theme.MenubarStyle` are now unexported fields, like
  every other widget style. A theme is customized through `ThemeCfg` tokens,
  never by field assignment. `Theme.ScrollbarStyle` and `Theme.TextStyleDef`
  stay public: external layout code reads the scrollbar gutter size, and
  `TextStyleDef` is a `ThemeMaker` input, not a widget style. Migration:

  | Before                                 | After                                                                                  |
  | -------------------------------------- | -------------------------------------------------------------------------------------- |
  | `theme.ButtonStyle.Colors.BorderFocus` | `theme.Cfg.ColorBorderFocus`, or `theme.ColorSelect` when unset (the fallback it uses) |
  | `theme.ContainerStyle.Color`           | `gui.ColorTransparent` (the style's constant value)                                    |
  | `theme.InputStyle.Padding`             | `theme.PaddingField` (the token it derives from)                                       |
  | `theme.MenubarStyle.TextStyle`         | `theme.TextStyleDef` (the identical body text)                                         |
  | `theme.ScrollbarStyle.*`               | unchanged                                                                              |

  The scan over go-charts, go-edit, go-kite, go-map, go-term and go-speedtest
  found reads only and no writes in library code; the one outside reader that
  breaks is the go-charts showcase example (`MenubarStyle.TextStyle`, same
  one-line migration to `TextStyleDef`) on main. A reflect gate
  (`TestThemeStyleFieldsPrivate`) fails any newly exported widget style.

- **BREAKING: secondary sub-element colors are `ColorSet`s, not flat fields
  (#720)** — the four interaction-state groups #716 left flat are now named
  `ColorSet` fields, so every interactive sub-element reaches `ColorSet.pick`
  the same way the primary element does: `TabControlStyle.ColorsTab`,
  `BreadcrumbStyle.ColorsCrumb`, `SplitterStyle.ColorsHandle` / `ColorsButton`,
  and `DataGridStyle.ColorsHeader` / `ColorsRow` / `ColorsResize` (with matching
  `ColorsTab`, `ColorsCrumb`, `ColorsHandle`, `ColorsButton` on the `gui` `Cfg`s
  and `ColorsHeader`, `ColorsRow`, `ColorsResize` on `datagrid.DataGridCfg`).
  The splitter's active color maps to `Click` (a drag is a held press) and so
  does the data-grid resize-active color; the splitter button's `Focus` slot
  takes the hover color, which is what it always painted. Selected and disabled
  stay flat (`ColorTabSelected`, `ColorTabDisabled`, `ColorCrumbSelected`,
  `ColorCrumbDisabled`, `ColorRowSelected`): `ColorSet` has no slot for them.
  Migration, per struct:

  | Before                                                                                                       | After                                                            |
  | ------------------------------------------------------------------------------------------------------------ | ---------------------------------------------------------------- |
  | `DataGridStyle.ColorHeader` / `DataGridCfg.ColorHeader`                                                      | `ColorsHeader.Base`                                              |
  | `DataGridStyle.ColorHeaderHover` / `DataGridCfg.ColorHeaderHover`                                            | `ColorsHeader.Hover`                                             |
  | `DataGridStyle.ColorRowHover` / `DataGridCfg.ColorRowHover`                                                  | `ColorsRow.Hover`                                                |
  | `DataGridStyle.ColorBorder` / `DataGridCfg.ColorBorder`                                                      | `ColorsRow.Border`                                               |
  | `DataGridStyle.ColorResizeHandle` / `DataGridCfg.ColorResizeHandle`                                          | `ColorsResize.Base`                                              |
  | `DataGridStyle.ColorResizeActive` / `DataGridCfg.ColorResizeActive`                                          | `ColorsResize.Click`                                             |
  | `TabControlCfg.ColorTabHover` / `ColorTabFocus` / `ColorTabClick` / `ColorTabBorder` / `ColorTabBorderFocus` | `ColorsTab.Hover` / `Focus` / `Click` / `Border` / `BorderFocus` |
  | `BreadcrumbCfg.ColorCrumbHover` / `ColorCrumbClick`                                                          | `ColorsCrumb.Hover` / `Click`                                    |
  | `SplitterCfg.ColorHandleHover` / `ColorHandleActive` / `ColorHandleBorder`                                   | `ColorsHandle.Hover` / `Click` / `Border`                        |
  | `SplitterCfg.ColorButtonHover` / `ColorButtonActive`                                                         | `ColorsButton.Hover` / `Click`                                   |

  `ColorTab`, `ColorCrumb`, `ColorHandle` and `ColorButton` stay as the
  shorthand for their set's `Base`, the same as `Color` on the primary sets.
  `DataGridCfg` carries no flat shorthand: a single header, row or resize color
  spells the set (`ColorsHeader: gg.ColorSet{Base: c}`). The consumer scan over
  go-charts, go-edit, go-kite, go-map, go-term, go-speedtest, go-shirei and
  falcon.go-gui.com found no hits. No color changes: the golden tests pass
  unrecorded.

- **BREAKING: widget `Cfg` flat state-color fields deleted, `Colors` is the only
  spelling (#721)** — the twelve widgets that fanned their `ColorSet` out into
  flat `Cfg` fields (`Input`, `NumericInput`, `Combobox`, `Select`, `ListBox`,
  `VirtualList`, `Tree`, `Table`, `Menubar`, `ContextMenu`, `ExpandPanel`,
  `Slider`) now resolve straight into `Colors` and pick every interaction state
  through `ColorSet.pick`, closing the class #690 exists for. The flat field
  used to win over the set; now there is no second spelling to win. `Color`
  stays as the shorthand for `Colors.Base`. Migration, per widget:

  | Before                                                                          | After                                               |
  | ------------------------------------------------------------------------------- | --------------------------------------------------- |
  | `InputCfg.ColorHover` / `ColorBorder` / `ColorBorderFocus`                      | `Colors.Hover` / `Border` / `BorderFocus`           |
  | `NumericInputCfg.ColorHover` / `ColorBorder` / `ColorBorderFocus`               | `Colors.Hover` / `Border` / `BorderFocus`           |
  | `ComboboxCfg.ColorHover` / `ColorFocus` / `ColorBorder` / `ColorBorderFocus`    | `Colors.Hover` / `Focus` / `Border` / `BorderFocus` |
  | `SelectCfg.ColorFocus` / `ColorBorder` / `ColorBorderFocus`                     | `Colors.Focus` / `Border` / `BorderFocus`           |
  | `ListBoxCfg.ColorHover` / `ColorBorder` / `ColorBorderFocus`                    | `Colors.Hover` / `Border` / `BorderFocus`           |
  | `VirtualListCfg.ColorBorder` / `ColorBorderFocus`                               | `Colors.Border` / `BorderFocus`                     |
  | `TreeCfg.ColorHover` / `ColorFocus` / `ColorBorder`                             | `Colors.Hover` / `Focus` / `Border`                 |
  | `TableCfg.ColorHover` / `ColorBorder` / `ColorBorderFocus`                      | `Colors.Hover` / `Border` / `BorderFocus`           |
  | `MenubarCfg.ColorBorder`, `ContextMenuCfg.ColorBorder`                          | `Colors.Border`                                     |
  | `ExpandPanelCfg.ColorHover` / `colorClick` / `ColorBorder` / `ColorBorderFocus` | `Colors.Hover` / `Click` / `Border` / `BorderFocus` |
  | `SliderCfg.ColorHover` / `ColorClick` / `ColorFocus` / `ColorBorder`            | `Colors.Hover` / `Click` / `Focus` / `Border`       |

  One migration trap: `Colors.BorderFocus` falls back to `Colors.Border`, which
  the flat `ColorBorder` never did. A call site that sets only `Border` now also
  pins the focused border to that color instead of taking the theme's — spell
  `BorderFocus` as well wherever the theme's focus border should stay.

  The consumer scan over go-charts, go-edit, go-kite, go-map, go-term,
  go-speedtest, go-shirei and falcon.go-gui.com found one hit:
  `go-charts/chart/data_table.go` sets `TableCfg{ColorBorder: ...}`, which
  becomes `Colors: gui.ColorSet{Border: ...}`. Behavior is unchanged except
  where the set now rules: a caller-set `Focus`/`Click` tints a focused or
  pressed widget the way the theme always said, and `ExpandPanel` shows the
  click color while Space is held, like `Button`.

- **Hovering a focused control now shows the hover color (#690)** — `Button`,
  `Toggle`, `Switch` and `Radio` pick their interaction-state colors through one
  internal rule instead of each writing its own. The fill follows the pointer
  (`disabled` > `pressed` > `hovered` > `focused` > resting) and the border
  follows focus (`disabled` > `focused` > resting), so a focused control under
  the pointer shows its hover fill while keeping its focus ring and focus
  border. `Button`, `Input` and `Tree` rows change: they used to hold the focus
  fill until the pointer left, which reads as stuck after a click focuses the
  control. `Toggle`, `Switch`, `Radio` and `ExpandPanel` already behaved this
  way and are unchanged. The focus fill (`ColorSet.Focus`) is now what a control
  reached by the keyboard looks like — focused with the pointer elsewhere.
  Nothing exported moved.

- **BREAKING: theme style structs carry a `ColorSet`, not six flat color fields
  (#716)** — the 24 widget style structs on `Theme` (`InputStyle`,
  `SelectStyle`, `ListBoxStyle`, `MenubarStyle` and the rest) replace their
  `Color`, `ColorHover`, `ColorFocus`, `colorClick`, `ColorBorder` and
  `ColorBorderFocus` fields with one `Colors ColorSet`. The same six colors had
  three spellings — flat fields on the theme, a `ColorSet` on the `Cfg`, and the
  `themeColorSet`/`themeButtonSet` repackers that converted one into the other
  on the view path every frame. Both repackers are gone. Each widget now hands
  its theme defaults straight to `resolved`, which is what #690's shared state
  picker needs to exist at all.

  Rename at the call site, one to one, on every style struct: `X.Color` →
  `X.Colors.Base`, `X.ColorHover` → `X.Colors.Hover`, `X.ColorFocus` →
  `X.Colors.Focus`, `X.ColorBorder` → `X.Colors.Border`, `X.ColorBorderFocus` →
  `X.Colors.BorderFocus`. Per-widget colors that are not interaction state —
  `ColorSelect`, `ColorSelectSubtle`, `ColorHighlight`, `ScrollbarStyle` and
  `SplitterStyle` — do not move. `Cfg` fields do not move either:
  `Button(gui.Cfg{Color: c})` and `Cfg{Colors: gui.Flat(c)}` are unchanged. No
  color changes: the golden tests pass unrecorded.

- **Pointer hit testing skips subtrees outside the pointer (#704)** —
  mouse-move, mouse-down, mouse-up and scroll-fallback dispatch no longer walk
  descendants whose inherited clip cannot contain the pointer. Non-clipping
  quarter-turn containers keep their wider child hit region, and mouse locks
  still receive pointer events outside the locked shape.
- **Custom control examples merged into `examples/custom_controls`** — the
  `custom_buttons`, `custom_checkboxes`, `custom_radios`, `custom_toggles`,
  `custom_sliders`, `custom_textinputs` and `custom_scrollbars` examples are now
  one program with a tab per control. Each look lives in its own package and
  subfolder with its README. Run `go run ./examples/custom_controls/`, or pass
  `-tab <name>` to open one tab. `custom_shader` stays separate.
- **`Window.TestRender` runs a second layout pass when the first asks for one
  (#664)** — as `FrameFn` does on screen. A test or a `soft.RenderToPNG` capture
  now sees the settled frame, for example a look that reads hover state, without
  calling `TestRender` twice.
- **Space clicks on release, and a held Space shows as pressed (#658)** — a
  focused widget with `ClickOnSpace` (`Button`, `Toggle`, `Switch`, `Radio`,
  `ExpandPanel`, `ColorSwatch`, and custom containers) used to fire `OnClick`
  from the space character, on press, with no pressed state. Space key down now
  presses the widget and key up clicks it, as a mouse press and release do and
  as HTML, GTK and Win32 buttons do. While the key is held, `Window.IsPressed`
  and `InteractionState.Pressed` are true, `InteractionState.Armed` is true, and
  `Button` shows its click color. A focus change, a window blur or Escape
  cancels the press without a click. Enter still clicks on key down. A test that
  clicked with `w.TestType(id, " ")` or a bare space `EventChar` must now send
  the key: `w.TestKey(id, gui.KeySpace, gui.ModNone)`.
- **Locale bundles fail fast instead of falling back silently** — unknown JSON
  fields, `first_day_of_week` outside 0-6, `decimals` outside 0-20,
  multi-character separators, non-positive group sizes, a `text_dir` other than
  `ltr`/`rtl`, a currency `position` other than `prefix`/`suffix` and
  wrong-length weekday/month lists now return an error naming the key. An
  explicit empty string still falls back to en-US, and bundles over 1 MiB are
  refused.
- **`LocaleLoadDir` loads atomically and reports full paths** — every file must
  parse before anything registers, so one bad bundle no longer leaves a
  half-loaded registry. Errors carry the file path, and a missing directory
  errors instead of succeeding silently.
- **Row, page and match counts use locale digit grouping** — `LocaleRowsFmt`,
  `LocalePageFmt` and `LocaleMatchesFmt` group digits (`1.234.567` under de-DE,
  `1,23,45,678` with `[3,2]` sizes). Counts under 1000 render exactly as before.
- **Locales without `text_dir` are LTR, not `Auto`** — most built-in presets
  reported `Auto`; they now report `TextDirLTR` as the doc table says. A global
  `Auto` still resolves to LTR wherever direction is read.
- **The active locale is locked and copied** — `SetLocale` and `CurrentLocale`
  take a read/write mutex, and the registry, `CurrentLocale` and `LocaleGet`
  hand out deep copies, so mutating a returned `Locale` no longer corrupts the
  stored one. Date parsing also reads `HH`/`mm`/`ss` and two-digit years, and
  rejects month-name formats with an explicit error instead of a wrong date.
- **Showcase Data Source grid fits its contents** — the DataGrid on the Data
  Source demo page now uses `FitFit` sizing, so the grid hugs its columns plus
  gutter and borders instead of stretching to the panel width.
- **Showcase demo boxes center their labels** — the fixed-size boxes in the
  layout demos (scrollable containers, row, column, wrap panel) now opt out of
  theme container padding, which filled the whole box and pinned each label to
  its bottom edge, and centre each label on its own ink with the newly exported
  `gui.OpticalCenterText` hook.
- **Showcase synth pads center their lines** — the Live Synthesis pads had the
  same padding-filled fixed box, pushing note name and frequency to the bottom
  edge. They now opt out of the padding and centre each line on its own ink.
- **Date picker header and adjacent-day buttons are `ButtonGhost` (#718)** — the
  month label, confirm, previous/next, blank spacer, and adjacent-month day
  cells hand-built the ghost button colors instead of asking for
  `Variant: ButtonGhost`. They now use the variant, so they follow the theme. A
  keyboard-focused header button no longer shows the focused-button fill, only
  the focus ring and focus border.

### Fixed

- **`VirtualList` dragged its new scrollbars off the track (#730)** — the
  re-anchor correction that holds a row still while the rows above it are
  measured shifted every child of the list, and `virtualListMeasure` runs from
  `layoutAmend`, which fires children-first. The scrollbars had therefore
  already placed themselves against the list, and the shift moved them a second
  time: on a deep scroll the vertical bar landed wholly outside the list. The
  shift now skips out-of-flow children.
- **`Theme.AdjustFontSize` flattened the size ladder on a theme that did not
  state its rungs** — the tune was added to `ThemeCfg.SizeText*`, which
  `ThemeMaker` leaves as the caller wrote them, so a theme built from a Cfg
  stating only `TextStyleDef.Size` had six zeros there and one zoom step took a
  10/11/12/14/17/22 ladder to a flat 6. The rungs are resolved before the tune
  is applied, so a zoom now moves the whole ladder by the delta and clamps at
  the legibility floor as documented.
- **Subtle washes took their alpha from the color being washed** — `subtleFor`
  read the dark-or-light polarity off its subject instead of off the theme, so a
  dark-leaning accent on a dark theme got the light theme's weaker wash, and two
  status colors of opposite lightness in one theme washed at two different
  strengths. Polarity is now read off the theme's text against its background,
  the same comparison the text roles make, so every derived `Color*Subtle` slot
  of a theme shares one alpha.
- **Rung 3 of the text grid disagreed between faces** — `N3` took
  `TextStyleDef.Size` directly while `B3`, `I3`, `M3` and `BI3` took the
  `SizeTextMedium` rung. A theme stating both and disagreeing rendered a bold
  word at a different size from the sentence around it, and a table header
  outgrew the rows it headed. `N3` now takes the same rung as its siblings; the
  presets state the two as one value, so nothing moves for them.
- **`VirtualList` never painted scrollbars (#730)** — `GenerateLayout` built its
  container through `buildContainerShape` and bypassed the `container()` path
  that appends the auto scrollbar pair to every other scrollable, so the list
  scrolled by wheel and drag with no position indicator while its own docstring
  promised one. It now appends the stock auto bars (hidden when the rows fit);
  the goldens record the thumb.
- **`Select` with `Invisible` kept a live, focusable shape (#691)** — the
  factory built its container directly and bypassed the invisible-to-singleton
  mapping, so a hidden field parked focus on a dead ID and still answered hover.
  It now returns the singleton like the other widgets, and the
  interaction-contract matrix pins the hidden column.
- **Held interaction survived eligibility loss and window blur (#691)** — a
  press whose target was disabled, hidden, removed or replaced stayed armed, so
  the release could click whatever took its place; open select/combobox popups
  never closed; a drag kept its lock when the window lost focus. A per-frame
  repair next to `fixupFocusLocked` now clears a dead or disabled press target,
  closes orphaned popups, and a modal dialog ends pointer gestures outright;
  window-unfocus cancels the lock without committing and dismisses popups. The
  interaction-contract matrix is the gate.
- **Second theme review: `WithColors` drift, badge contrast, install cost** —
  from a second review of the `theme*` code, five defects and their gates:
  - **`Theme.WithColors` had drifted from `ThemeMaker`.** It restates the whole
    cfg-to-style mapping by hand, so a slot added to `ThemeMaker` and not to it
    silently kept the old theme's color. Fourteen slots had drifted on a
    same-polarity recolor and twenty-eight across a polarity flip. Now fixed:
    the spell-check underline, the radio, switch and toggle pressed, focus and
    select colors, the focus ring on the primary, ghost and danger buttons, the
    ghost button's pressed color, the skeleton shimmer, the tree's border (it
    stays transparent, as `ThemeMaker` builds it) and the inspector panel
    (deliberately theme-independent, so it no longer follows a recolor). The
    four named text roles are re-derived for the new polarity along with every
    placeholder, secondary and disabled style built from them, and the selected
    tab's label pairs with the new select fill — a light accent used to keep a
    white label on it. A theme that forked a slot by hand still keeps its fork.
    `TestWithColorsMatchesThemeMaker` is the new gate: a full recolor through
    both paths, compared field by field across four polarity combinations.
  - **A badge drew its label in a hardcoded white (#373).** Every light preset
    fills a neutral badge with `ColorActive`, a near-white, so the label sat on
    it at contrast 1.26 to 1.32. The label now takes the theme's existing
    pairing rule against the fill it actually paints — per variant and per
    caller-supplied `Color` — while a caller-stated `TextStyle` still wins. The
    semantic fills are brand colors at mid-luminance and reach no better than
    about 3.5 with either foreground; that is a palette question, not a pairing
    one, and is unchanged.
  - **Theme install copied ~12 KB per window per frame.** The steady-state frame
    does nothing but compare one id, and it copied the whole `Theme` out of the
    window to reach it: 705 ns/op for a no-op. By reference it is 9.3 ns/op,
    zero allocations (`BenchmarkInstallThemeSteadyState`).
  - **`AdjustFontSize` reported the wrong rule and never checked its range.**
    The guard is `minSize >= 1` but the error said `> 0`, and `maxSize` was
    never compared to `minSize`, so an inverted range rejected every size and
    blamed the size for it.
  - **The app-default theme aliased the exported `ThemeDark` var.** It was
    published as `&ThemeDark`, so assigning `gui.ThemeDark` wrote through a
    pointer live readers hold — the one thing the no-copy theme path promises
    cannot happen. It publishes a copy now.
  - Also: dropped two dead stores in `ThemeMaker` that read the size config raw
    and were overwritten by the weight rungs, gave every `GOOS` without a named
    system font a default (`freebsd` and friends failed to compile on two
    missing constants, now gated by `TestDefaultFontConstantsCoverEveryGOOS`
    rather than by a cross-compile target, since `go-glyph` has no port to build
    against there), and renamed `basegnomeCfg`/`basewindowsCfg` to match
    `baseMacOSCfg`.
- **Theme correctness fixes** — from a review of the `theme*` code:
  - **`Theme.WithColors` is now public and covers the full recolor story.**
    Overrides are plain `Color` values where unset means "keep" (an explicit
    fully-transparent override stays honorable). New slots join the original
    ten: the accent ramp, the semantic colors and their subtle washes, and the
    separator. Derived slots follow their parent while the two agree and stay
    put once forked, so overriding select alone moves the accent (the state
    every preset ships in) while stating both sets them apart; the same rule
    carries border-focus with select, the separator with border, text-on-select
    with text-on-accent, and every subtle wash with its color. Semantic
    overrides fan out to the toast, badge and danger-button styles, and touched
    fields sync into `Cfg` so a later rebuild (`WithPadding`, `WithBorders`,
    `AdjustFontSize`) keeps the overrides. Like every `with*Style` helper, the
    result carries a fresh id so the install fast path reinstalls instead of
    keeping stale mirrors.
  - `WithPadding(false).WithPadding(true)` round-trips: the stripped theme
    remembers the pre-strip configuration instead of rebuilding from its own
    zeroed `Cfg`.
  - An explicit fully-transparent focus border stays honorable instead of
    falling back to the select color.
  - `ThemeMaker` isolates elevation pointers per theme, so mutating a caller cfg
    or the package-level presets after the build no longer moves built themes.
  - `unregisterWindow` clears the vacated slot so a closed window is not
    retained by the live-window set.
  - The install fast-path word is atomic, so eager `SetTheme` calls outside the
    frame pass no longer race the frame thread on it.
  - Preset palette literals fail at init on a typo (`mustThemeColor` panics)
    instead of rendering as opaque black.
  - The theme audit now flags frame-cache reads in `EventCtx` handlers and scans
    `view_*.go` handler closures; the two `view_tree.go` sound reads now use
    `ctx.Window.Theme()`.
- **A dialog now opens at the `Width` and `Height` it is given (#708)** — a
  `DialogCfg` with `Width: 400` and `MinWidth: 400` opened 300 wide, because the
  theme's `MaxWidth` of 300 won over the caller's values. `Width` and `Height`
  also added the content size on top of themselves instead of setting the size.
  A positive `Width` or `Height` now fixes that side of the dialog, and a
  `MinWidth` above the theme's `MaxWidth` raises the max when `MaxWidth` is not
  set. A dialog with no size fields keeps the theme's 200–300 width range.
- **Closing a window on Linux (X11) no longer crashes intermittently (#701)** —
  the event-pump goroutine re-read the backend's X connection on every
  iteration, while shutdown closed that connection and set it to `nil` on the
  main goroutine. A pump that lost the race panicked with a nil pointer
  dereference in `xgb.(*Conn).WaitForEvent`. The pump now holds the connection
  it was started with and ends cleanly when it closes. In a multi-window
  `RunApp`, events still queued for a window that just closed are now dropped
  instead of handled by its destroyed backend.
- **Text animation, layout and optical centring fixes** — from a second review
  of the `text*` code:
  - With more than 100 animated text IDs, finished entrances no longer play
    again in a loop. Their state lived in a 100-entry first-in, first-out map;
    it is now unbounded and drops only texts that left the view tree.
  - A typewriter no longer changes its layout while it types. The text is laid
    out in full and only the unrevealed glyphs are hidden, so a wrapped
    typewriter keeps its height and a centred one types in place. It reveals
    whole characters (grapheme clusters), so emoji with skin tones, flags and
    accented letters no longer flash in parts. Text that grows by appending
    keeps typing from where it had got to.
  - Changing `TextCfg.Anim` on the same ID now starts the new animation. A
    finished entrance blocked every later one, and a running loop kept running.
  - An entrance no longer replays once when a frame is built between the end of
    the animation and the delivery of its last callback.
  - `TextAnimPop` and other scale or rotate effects turn about the text's final
    box. Wrapped and Fill-sized text slid sideways because the center came from
    the size measured before layout.
  - Animation motion now composes with `RotationRadians` and `AffineTransform`
    instead of replacing them, so a rotated label no longer flickers upright.
  - A one-shot `TextAnimShimmer` returns the text to its own colour when it
    ends, follows `TextAnimCfg.Easing`, and takes the label colour of a filled
    button.
  - A wrapped text with `LineSpacing` is no longer too short for its last line.
    The box height comes from the last line, not from the average line.
  - `OpticalCenterText` no longer moves a typewriter's baseline as it types. Its
    memo now keys on `Features`, `EmojiBoxWidth` and `CellHeight`, holds 1024
    entries instead of 100, clears when the text measurer changes, and never
    stores a NaN offset from a bad ink box.
  - Faded, pulsing and disabled text on the glyph-layout path is shaped once per
    frame, not twice: the render pass recolours the cached layout. A finished
    entrance, an animated text with no ID and a text the shaper refuses no
    longer allocate every frame.
- **Text review fixes** — from a review of the `text*` code:
  - A negative animation delay no longer shortens the run or breaks keyframe
    order; it is clamped to zero.
  - Headless `fontHeight` now uses the shared 1.4em fallback, so layout, caret,
    and optical centring agree on one box. Golden caret recordings updated.
  - A non-finite font size corrects nothing in optical centring instead of
    missing the memo (NaN) or caching infinity.
  - `cursorEndOfParagraph` clamps a negative position like its start twin, and
    `truncatePreview` treats a negative budget as zero so empty input stays
    empty.
  - `toGlyphStyle` shares the single color conversion; empty-text height,
    ink-hook allocation, memo key choice, and typewriter cost are documented.
- **SVG hardening** — from a review of the `svg*` code:
  - File-backed SVGs no longer serve stale art after an edit. The render cache
    keyed the file path alone; it now mixes in file size and mtime, and the
    dimension cache checks the same mark before a hit. The file is checked again
    at most every 500 ms, so an edit shows up within that time and a cache hit
    in the render pass makes no file system call and no allocation.
  - The vertex-count cache cap now counts filtered-group paths. Before, a
    filter-heavy file skipped the cap while it still filled the vertex budget.
  - Defs-path flattening for text-on-a-path is bounded (token and point caps),
    rejects a bad scale, and drops non-finite geometry instead of caching it.
    `parseFloat` maps overflow to 0, and arc sampling guards a zero-vector
    angle, a bad sweep angle, and bad sampler inputs.
  - Tessellation maps a NaN, Inf, or non-positive scale to 1. Before, NaN forced
    full subdivision of each curve. Ear clipping drops polygons with non-finite
    vertices instead of emitting them.
  - A gradient keeps the first 256 color stops. Before, an uncapped stop list
    slowed the per-vertex gradient scan without bound.
  - A missing or malformed opacity keeps its fallback instead of turning the
    element transparent.
  - Display-size and flatness cache keys map NaN, Inf, and overflow to a fixed
    value instead of an undefined integer cast, and ID truncation keeps rune
    boundaries the same way the parser does.
  - File reads are size-capped at read time, so growth between the size probe
    and the read cannot widen the buffer.
- **Root view panic leaked `genDepth` (#689)** — `updateLocked` incremented
  `viewState.genDepth` around the root view function without `defer`, so a panic
  skipped the decrement. The leaked depth stuck across later good frames
  (`idScope` stopped resetting; after 256 panicked frames the tree became
  placeholders). The root bracket now defers its decrement, matching
  `generateViewLayout`.
- **Spellcheck hardening** — from a review of the `spellcheck*` code:
  - All Hunspell handle use on Linux is now serialized by a mutex. Before,
    concurrent `Check`/`Suggest`/`Learn` calls raced inside libhunspell, which
    documents no thread-safety guarantee.
  - `Suggest` clamps out-of-range offsets to the text remainder on both
    platforms, the same as the `nativehost` forwarder, using a subtraction
    instead of `startByte+lenBytes`, which overflows for a hostile length.
    Before, the Linux form panicked on a negative length and passed the check on
    overflow.
  - The macOS backend no longer loses the result buffer when `realloc` fails,
    checks every allocation including `strdup`, and learns words by explicit
    byte length so an embedded NUL cannot truncate the word. `Learn("")` is
    ignored instead of learning the empty string.
  - `Learn` rejects words with control characters on both platforms. Before, a
    word containing a newline was added to the live session and then persisted
    as several lines, injecting extra dictionary entries on the next load.
  - The Linux personal dictionary load is bounded in total bytes, skips overlong
    lines instead of aborting the scan, caps loaded words, and only treats a
    leading number as a count header when words follow it. Before, a file
    holding only a learned numeric word forgot that word on restart.
    `C.UTF-8`-style locales and `LANGUAGE` priority lists now resolve instead of
    disabling the engine, and empty `DICPATH` entries no longer probe the
    working directory.
  - The pure dictionary helpers moved to `spellcheck_linux_dict.go` without the
    Hunspell dependency, so their tests run on every Linux build; the Hunspell
    engine itself stays behind `-tags hunspell`, which no CI job builds.
- **InputDate calendar icon sits on the field middle line (#346)** — the icon
  took the face cap band while the date text beside it takes the figure band, so
  the two optical corrections disagreed and the icon rode off centre. A color
  emoji's ink metrics do not say where backends draw it, so no measured
  correction can centre it either. The icon now sits in a wrapper row inside its
  button, out of the button amend's direct text children, so it keeps its
  arranged position instead. It also inherits the field text style rather than
  always drawing the default.
- **Widget sound hardening** — from a review of the `sound*` code:
  - `SetSoundPlayer`, `SetSoundVolume`, `SoundPlayer`, `SoundVolume` and the
    dispatch read are now guarded by their own lock, the same split `themeMu`
    gives the window theme. Before, a volume or player change from any goroutine
    raced the event-dispatch read.
  - The showcase synth player no longer panics on a nil window and ignores mute,
    negative, NaN and Inf gain without spending a mixer channel. The system
    player ignores `SoundNone` instead of forwarding it to the platform.
  - The synth voice validates its envelope once at construction and snapshots
    the release flag per audio buffer, instead of validating and atomically
    loading per sample.
  - The out-of-range channel error printed `[0, N-1)` for a valid range of
    `0..N-1`; it now prints `[0, N)`.
- **Scrollbar and scroll API hardening** — these fixes come from a review of the
  scroll code:
  - A press on the scrollbar gutter now reaches the end of the range when the
    scrollable has padding. Before, it stopped short by the padding. Over
    content that fits, it no longer writes an offset that pushes the content
    down.
  - With the default `GapEnd`, the thumb now reaches the end of its track at the
    end of the range. Before, with a small overflow, it stopped well short (at
    4px of overflow, it stopped halfway).
  - A thumb drag now keeps the thumb under the pointer when `MinThumbSize`
    enlarges the thumb. Before, on long content, the thumb moved away from the
    pointer.
  - `ScrollVerticalTo`, `ScrollHorizontalTo` and the `*ToPct` setters now ignore
    NaN and Inf. Before, one such value made every child position NaN until the
    app wrote a new offset. The layout pass now resets a non-finite stored
    offset to 0.
  - `ScrollVerticalToPct`, `ScrollHorizontalToPct` and markdown anchor links now
    fire `OnScroll`, as `ScrollVerticalTo` does.
  - `ScrollAnchorReveal` now eases to the top when nothing was inserted above
    the anchor, as its doc says.
  - `ScrollVerticalOffset` and the `*Pct` getters no longer create the scroll
    maps. `ScrollVerticalPct` and `ScrollHorizontalTo` now report a near miss
    under `DebugUnknownLookup`, as their twins do.
- **Fill children get all of a row's or column's space** — the loop that shares
  space among Fill children stopped early in three cases. The row or column then
  overflowed, or left space unused, and nothing reported it. First, in a very
  small container, one step moved less than the float tolerance, so the loop
  took the step for a stall and stopped with most of the space still unshared.
  Second, after one child grew to meet a sibling, float rounding left the two
  sizes a few millionths apart. The loop then saw two sizes, took a step too
  small to change either, and stopped. Sizes within a small relative band now
  count as one size. Third, when the widest Fill child was already at its
  `MinWidth` or `MinHeight`, it could not shrink. The loop stopped before the
  narrower siblings shrank, so the row still overflowed. The loop now stops only
  when a step moves no space and removes no child.
- **RTF selection, links and tooltips agree with the shaped text when inline
  math is present** — the flat text used for rune/byte mapping concatenated only
  the source run texts, while shaping emits the LaTeX fallback or the object
  placeholder for math runs. Every click, drag, key nav, highlight and
  link/tooltip lookup past the first math run was therefore offset, and markdown
  block offsets drifted the same way. Flat text, rune counts and run lookup now
  share one shaped-domain helper with the shaper, so a Loading→Ready transition
  moves all of them together. A ready cache entry no longer produces an unusable
  inline object either: an unset run size, non-finite dimensions, or a scale
  that overflows the object to +Inf or collapses it to zero all fall the run
  back to its LaTeX source. Single-line RTF also joins the cross-frame layout
  cache the wrap path already had, so a static block shapes once instead of
  every frame. Related hardening in the same pass: the drag re-resolves its
  shape each step (a mid-drag re-layout no longer steers it with stale
  geometry), the tooltip popup ID is a hash instead of raw tooltip text (which
  may hold a colon that `ScopeID` forbids in a part), unsafe links no longer
  offer the pointing-hand cursor the click path refuses, over-long links
  truncate by rune in diagnostics, and double-click timing runs on the monotonic
  clock so an NTP step cannot forge or miss one.
- **Filter and stencil brackets stay balanced, whatever the emitted command** —
  an SVG `stdDeviation` is only checked for NaN/Inf and `> 0` at parse time, so
  a six-digit value times an ordinary tessellation scale overflowed to `+Inf`.
  The validator then dropped `RenderFilterBegin` while the unconditional
  `RenderFilterEnd` stayed, and the GPU backends composite on an end whether or
  not they saw a begin — a stale glow layer painted over the frame. Filter blur
  is now folded into the range every backend supports (non-finite and negative
  to 0, capped at the soft backend's own 512-pixel limit), and both filter
  brackets — the SVG one and a container's `ColorFilter` — emit their end only
  when the begin was appended. A container whose `ColorFilter` matrix is
  non-finite now draws unfiltered instead of corrupting the frame, and no longer
  suppresses a descendant's own filter. A stencil clip bracket past the 255-deep
  saturation point is skipped whole rather than emitted at its parent's depth,
  where its end decremented the coverage the parent still needed and every later
  sibling clipped against a value one too low; the scissor rect still bounds the
  subtree.
- **Render robustness: warn-once SVG errors, hardened gradient borders, float64
  animation time** — a broken SVG source logged on every frame; it now logs once
  per source while the magenta placeholder still emits each frame, in a bounded
  window store keyed by source hash, so a page that mints a new broken source
  every frame cannot grow it without limit. `GradientBorderRects` no longer
  panics on a nil command or gradient, and clamps and sorts stops before
  sampling like every gradient fill path, so unsorted or out-of-range stops stop
  tinting borders wrong. Neither path allocates: an already-sorted list is
  sampled in place and a misordered one is normalized on the stack, because
  every backend calls this once per gradient-border command per frame. An
  oversized stop list is truncated instead of scanned four times per frame. SVG
  filter layers floor at 1, so a hostile `BlurLayers` cannot drop
  `RenderFilterBegin` against a kept `RenderFilterEnd` and unbalance the
  bracket. Animation phase math runs in float64 so a long-lived page keeps
  sub-frame precision, and a fill-backwards SMIL animation with a cycle poses at
  its first keyframe before begin instead of freezing at an earlier cycle's end.
  A denormal `Cycle` or `DurSec` from an untrusted document no longer overflows
  the phase math: the activation and the iteration index stay in float64, so
  neither an infinite interpolation factor nor an implementation-defined
  float-to-int conversion reaches the lerps. `lerpU8` rounds like
  `f32ToU8Saturated` instead of truncating, closing a 1-LSB drift between color
  tweens and gradient sampling. Headless `RenderToImage` treats a NaN or
  infinite scale as 1 and refuses a render whose device pixels exceed 128M — the
  16384x16384 corner the per-side cap still allows, well above any real display
  render. The render-guard bitmask is documented as silent (it never logged) and
  compile-asserted to fit 32 kinds.
- **DataGrid cell content no longer paints over the next column (#679)** — a
  committed value wider than its column kept drawing past the cell edge after
  the editor closed, so a long value covered its neighbour. A display cell now
  clips its contents to the cell box. The cell being edited does not clip: the
  editor fills the cell exactly and the theme draws focus as a glow outside the
  control, so a clip there would cut the focus ring off the cell in use. Select
  dropdowns and date pickers are floating layers and still open past the cell.
- **Printing keeps its temp PDF alive for async backends and validates its
  inputs** — `RunPrintJob` on the current view deleted its temp PDF on return,
  while Linux `xdg-open` and Windows `ShellExecute` open the file after the call
  returns, so the viewer could find nothing. On success the temp path is now
  returned in `PrintRunResult.PDFPath` for the caller to remove; on cancel or
  error it is removed and the path is empty. `pdf_path` jobs now fail fast with
  `io_error` for unreadable files instead of a vague backend error, and paths
  that are empty, contain NUL or start with `-` are rejected before they reach
  `lpr` or the C string boundary. `Copies` is capped at 9999, unknown paper and
  orientation values are rejected instead of silently printing A4, and the
  Windows backend reports the `ShellExecute` code and no longer drops NUL paths
  silently. A `pdf_path` that is a directory, FIFO or device node is rejected
  rather than handed to the spooler, unknown `ScaleMode`, `Duplex`, `ColorMode`
  and `Source` values are rejected instead of being silently dropped by the
  backend, `PageRanges` is capped at 1024 ranges and page 1000000, and the
  returned `PDFPath` is filled in even for a backend that reports success
  without echoing the path.
- **PDF export writes atomically, guards bad geometry and caps images** — a
  failed `ExportPrintJob` could leave a truncated PDF at the output path; the
  file is now staged in the same directory and renamed on success. Zero or
  negative source dimensions are rejected instead of producing an infinite
  scale, NaN margins and NaN or Inf `SourceWidth`/`SourceHeight` are rejected
  instead of reaching the PDF as NaN coordinates, corrupt glyph-layout indices
  skip the run instead of panicking, file images over 64 MiB or outside the
  filesystem are skipped instead of OOMing the exporter, and the export
  snapshots layouts, text styles and canvas transforms alongside the triangle
  data so a concurrent frame cannot rewrite them mid-export. `Copies` and
  `PageRanges` are documented as native-dialog options: export always writes one
  page.
- **No-op native backends stop reporting success for work they did not do** —
  the headless noop platform returned zero values that read as `DialogOK`,
  `NotificationOK` and `PrintRunOK`, and the web, Android, iOS and non-Linux
  stubs handed out tray ID 0 for every icon, so two trays shared one handle. The
  noop now reports cancel or error, and every stub hands out unique tray IDs.
- **Web, Android and iOS validate `OpenURI` and spell input like the desktop
  backends** — the web scheme prefix check is now a length-capped allowlist
  parse matching `nativehost.ValidateOpenURI`, the mobile backends cap URI
  length, and both cap spell-check text and clamp offsets instead of passing
  unbounded input to the spell engine. Notification text and macOS accessibility
  announcements are truncated on a rune boundary on every platform, so capping
  can no longer split a multi-byte rune.
- **Web file pickers and printing fail cleanly instead of wedging or blanking**
  — a picker thrown without a user gesture now returns a dialog error instead of
  panicking the wasm instance, callback and element cleanup is deferred, a
  tainted canvas reports a print error instead of throwing, and printing waits
  for the snapshot image to decode instead of printing a blank page.
- **LaTeX math sanitizing uses an allowlist, not a substring blocklist** — the
  blocklist matched inside longer command names, so `\theta` rendered as `ta`,
  `\iff` as `f`, and `\longrightarrow` and `\coprod` lost their prefixes.
  Commands are now tokenized and only known-safe math commands pass; any other
  command is dropped while its arguments stay, so unknown macros fail closed
  instead of executing on the renderer. The fetcher also rejects empty source up
  front rather than requesting a blank formula. `\begin` and `\end` carry a
  second allowlist for the environment name, so a file-writing environment such
  as `filecontents` cannot ride in on an allowed command.
- **MathSpinner survives hostile parameters and renders Fourier correctly** — a
  NaN or infinite `Speed` reached the animation duration conversion, where a
  float-to-int conversion is implementation-defined; it now falls back to the
  default speed with the period clamped to 50ms–1h. `Size`, `Width`, `Height`,
  `StrokeWidth` and `TrailLength` take the same treatment, because a plain
  `<= 0` test passes NaN and `+Inf` straight into the layout tree. The Fourier
  curve normalized y by the x amplitudes, so silencing x erased y; each axis now
  normalizes by its own amplitudes. Negative display norms no longer mirror
  their curves, fractional butterfly powers stay on the positive branch, and
  setting only one of `Width`/`Height` keeps the default for the other instead
  of collapsing it to zero.
- **Float helpers define their edge cases** — `f32Mod` used an int truncation
  that was undefined for large quotients and a zero divisor; it now matches
  `math.Mod`, returning NaN where `math.Mod` does. `f32Min` and `f32Max`
  propagate NaN from either side instead of resolving to one operand depending
  on argument order, so a NaN size poisons loudly for the invariant checker
  instead of passing silently. The passthrough contracts (NaN in clamp, the
  absolute pixel epsilon in `f32AreClose`) are documented on the functions.
- **Markdown abbreviations and footnote refs no longer rewrite code** — an
  abbreviation inside an inline code span or fenced code block gained a tooltip,
  and a footnote pattern in inline code expanded to a superscript ref. Code runs
  are now left alone, and replacements keep the surrounding formatting (bold,
  strikethrough, highlight, underline, subscript) instead of dropping it.
- **Markdown links block control-character scheme evasions** — a URL such as
  `java\tscript:alert(1)` failed scheme validation and fell through to the
  plain-relative-path rule, so it read as safe. ASCII control characters now
  reject a URL outright, including percent-encoded ones.
- **Markdown images apply the extension allowlist to remote URLs** — any
  `http(s)` URL was accepted as an image source while local paths needed a known
  image extension. Remote paths now pass the same check, matched against the
  path without query or fragment.
- **Markdown tables align unmarked columns left** — a column without an explicit
  marker produced a distinct start alignment that rendered differently from an
  explicit left alignment under RTL. Unmarked columns now align left, matching
  GFM and the explicit marker.
- **The mouse wheel moves a `Slider` by `Step` (#668)** — the wheel added
  `ScrollY` to the value and ignored `Step`. One wheel notch is about 3 lines
  and a trackpad sends points, so a 0 to 1 slider reached an end on the first
  notch, and a slider with a large `Step` needed many notches for one step. The
  wheel now moves the value by one `Step` per event in the direction of the
  scroll, the same distance as an arrow key, and `RoundValue` still applies.
  `NumericInput` already worked this way. A `SliderCfg.Look` slider follows the
  same rule.

- **An ID-less float no longer hides hover from its widget (#661)** — a float
  with no `ID` inside an ID-bearing widget, such as a switch knob that floats
  over its track, gave no hover or press target. `IsHovered`, `IsPressed` and
  `InteractionState` read false for the widget while a click at the same point
  still reached its `OnClick`. The target under such a float is now the nearest
  enabled ID-bearing ancestor of the tree the float was lifted from, so hover
  and click agree. The float no longer needs an `ID` as a workaround.

- **`gui.Debug` no longer calls a slider a button (#664)** — the `Interactive`
  keyboard check reported every root with `OnClick` that lacked `ClickOnSpace`
  and `ClickOnEnter`. A custom slider or stepper handles its own keys, so the
  report was wrong and pushed apps to use `OnMouseDown` instead. A focusable
  root with its own `OnKeyDown` now passes. A root that cannot take focus is
  still reported.

- **A `ListBox` with no resolved height builds a probe, not every row** — under
  `Fill` sizing the first frame built all rows once, hanging large lists, while
  `VirtualList` built a bounded 64-row probe. `ListBox` now probes the same way
  and virtualizes from the second frame once arrange records a height. A list
  that stays at height 0 still warns under `Debug` once.
- **`ListBox` answers `PageUp` and `PageDown`** — the keys fell through to the
  shared navigator, which has no page action, so they did nothing. They now move
  by a viewport of rows, or ten rows when no height resolved yet, and keep the
  new focus row in view.
- **Disabled list rows stay silent on hover** — a disabled row blocked clicks
  but still fired its hover callback, and a reorderable `ListBox` with no
  `OnSelect` still painted the hover cursor. Both now stay inert, matching the
  click gate.
- **`ListBox` rows render single-line** — a row used multiline text while the
  spacers and the scroll model assume one uniform row height, so a wrapped label
  drifted every position below it. Rows now match combobox, select and menu,
  which were single-line already.
- **`SetVirtualListFocusedIndex` clamps to the corpus** — an index past the end
  sat out of bounds, and the next arrow key jumped from a row that was never on
  screen. It now pins to the last row, like the low side already pinned to 0.
- **Filtered lists keep row order on tied scores** — equal fuzzy scores came
  back in whatever order the unstable sort left behind, so a filter could
  shuffle matching rows between frames. Ties now break by row index.
- **Custom-shader hardening: guarded blur, cached failures, one wrapper copy** —
  a review of the shader code:
  - The blur shader called `smoothstep(-blur, blur, d)`, which is undefined when
    both edges are equal, and a sub-quarter-pixel blur packs to exactly 0. It
    now guards with `max(1.0, blur)` like the shadow shader, in the GLSL, MSL
    and Android copies.
  - A body that failed to compile recompiled on every frame. Failures are now
    cached (a zero pipeline on GL, a negative index on Metal/iOS/Android, the
    existing nil slot on web), so a broken body compiles once. The GL backend
    also drops its unused prebuilt custom pipeline.
  - The wrapper around a custom body lived in five copies. Desktop and ES GLSL
    now share one core behind `BuildGLSLFragment`/`BuildGLSLESFragment`, and
    Metal/iOS share `msl.BuildCustom`; new agreement tests pin all three to the
    same SDF clipping and varyings.
  - Render validation now rejects a shader with neither body and non-finite
    params. `Params` past the first 16 are documented as ignored,
    `ShaderHash(nil)` returns 0 instead of panicking, and its platform-dependent
    key is documented.
  - The web backend kept an exact-fit offscreen canvas, reallocating once per
    widget per frame when two custom shaders differed in size. It now retains
    the backing store (`gpu.RetainBackingSize`, shrink past 8x waste) and blits
    the drawn sub-rectangle.
- **Shape review hardening** — a review of the shape code:
  - `AccessState.Has(AccessStateNone)` was always true, because any value masked
    with zero reads zero. It now reports whether no state is set.
  - `newShape` drew a `math/rand` ID for every shape on every frame, under the
    frame lock, for a field nothing read. The field and the draw are removed.
  - `layoutSetShapeClips` panicked on a hand-built mid-tree node with no
    `Shape`, while the other tree walks descend past such nodes. It now clips
    nothing there and keeps descending, so a valid subtree below stays
    reachable.
  - The soft backend's rect, circle and line guards passed NaN into the region
    math, whose float-to-int conversion is implementation-defined, and built a
    region before rejecting a degenerate box. The guards are now NaN-safe and
    reject before the region.
  - SVG shapes with negative sizes painted with reversed winding instead of
    following SVG 2. A negative rect `width`/`height` or circle `r` now stops
    the element from rendering. A negative `rx`/`ry` now reads as auto: a rect
    keeps its shape and takes its corner radius from the other value, and an
    ellipse takes the other radius, so it draws a circle. A missing or
    unparseable ellipse radius is auto too, so `<ellipse rx="20">` draws a
    circle where SVG 1.1 drew nothing. When both ellipse radii are auto, the
    ellipse does not render. A zero size is kept, because the tessellator drops
    it when static and keeps it as an animated placeholder when an `<animate>`
    child drives the size. An unparseable fill now falls back to inherit instead
    of transparent, and the path tokenizer caps its upfront capacity.
- **Styles review fixes (#698)** — from a review of the `styles*` and markdown
  style code:
  - Superscript and subscript runs grew by 1.2x before the shaper synthesized
    them, so they rendered oversized and over-raised. Markdown no longer scales
    these runs at all: it tags the run and the shaper derives 0.58x size and
    baseline shift from the base size. The runs also shared one global
    `FontFeatures`, so mutating a run corrupted every sup/sub run; each run now
    gets a fresh copy that merges with the base features.
  - `mergeTextStyle` merged only color and size while promising all zero fields,
    so the radio/switch/toggle label merges dropped family, spacing, background
    and pointer fields. Every unset field now inherits, and the
    disabled/icon/defaulted role flags ride along, closing a double-dim hole
    through the merge path (issue #335).
  - A bold, italic or code run inside a heading lost the heading size and color.
    Formatted runs now keep the base geometry outside body text, while a custom
    Bold color still applies to body runs and token colors stay semantic.
  - Fenced code blocks over 64 KiB now keep the parser's own token runs instead
    of handing unbounded text to the syntax highlighter.

## [v0.77.0] - 2026-09-16

### Added

- **`gui.Interactive` gives a view builder its hover, press and focus state
  (#650)** — a custom look that read `IsHovered` / `IsPressed` had to be a named
  view type with its own `GenerateLayout`, call `w.EffID` itself, and spell
  "pressed and still hovered" again in every look.
  `gui.Interactive(id, func(gui.InteractionState) gui.View)` does the deferral
  and the ID resolution, and passes `Hovered`, `Pressed`, `Armed` and `Focused`.
  The caller names only the leaf ID. When the built root does not carry that ID,
  `gui.Debug` reports it under `DebugMissingIDs` instead of the state staying
  false silently. `examples/custom_buttons` now uses it.

- **`DebugLayoutInvariants` reports a frame that breaks a sizing rule (#634)** —
  wrong geometry was silent: the frame rendered, nothing errored, and the defect
  surfaced much later as something looking a few pixels off. The new category
  walks the arranged tree and reports a child positioned outside a non-clipping
  parent's bounds, a minimum left above its maximum, and any non-finite or
  negative size. It is opt-in, outside `DebugAll` for the reason
  `DebugUnscopedIDs` is: escaping a parent is sometimes the design, as a slider
  thumb overhangs its track. Ask for it by name through
  `(*Window).TestFindings(gui.DebugAll | gui.DebugLayoutInvariants)` or
  `gui.DebugCategories`. The rules it checks are written down for the first time
  in `docs/specs/layout-sizing-rules.md`, and the same checks back the layout
  fuzz targets so the document and the code cannot drift.
- **`MaskNone` names the no-mask preset** — `InputMaskPreset`'s zero value meant
  "no mask" but had no exported name, unlike every other preset. `MaskNone`
  spells it explicitly; existing code is unaffected.
- **`Checkbox` alias is back** — `Checkbox` calls `Toggle` with the same config,
  so code that uses the checkbox name builds again. The alias stays; later
  export cuts must keep it.
- **Custom input mask tokens and the remaining mask presets are public** —
  `InputCfg.MaskTokens` carries caller-defined `MaskTokenDef` tables into the
  compiled mask the keystroke path reads, `MaskTokenDef.Matcher` is exported so
  a table can actually be built outside the package, and `MaskCreditCard16`,
  `MaskCreditCardAmex` and `MaskCVC` join the preset list. A custom token
  without a `Matcher` fails `compileInputMask` instead of installing a slot no
  keystroke can ever fill.
- **`DebugLayoutInvariants` checks Fill distribution (#638)** — a Fill row that
  could not fit its minimums, or that left space undistributed, gave up
  silently: the frame rendered and nothing said so. The category now reports a
  main-axis container whose in-flow children plus spacing do not sum to its
  content box while a Fill child is there to take the slack. A row with no Fill,
  a clipping or scrolling parent, a Wrap or Overflow row, and a gap left while
  every Fill child sits at its maximum stay quiet: each is alignment slack or a
  supported outcome, not undistributed space. The rule is written down as
  invariant 4 in `docs/specs/layout-sizing-rules.md`.
- **`DebugSizing` warns when Fixed sizing discards Min/Max (#635)** — a Fixed
  axis pins `Min = Max = size`, so a stated `MinWidth`/`MaxWidth` (or height
  equivalent) never took effect and nothing said so. The new category, on by
  default in `DebugAll`, reports a conflicting stated bound at generation time;
  a bound equal to the size and a Fixed axis with no positive size stay quiet.
  The `Shape` and `ContainerCfg` bound fields now document the rule.

### Changed

- **go-glyph bumped to v1.25.2.** Brings typesetting v0.3.5 and x/text v0.42
  with no replace directive, so go-gui and glyph resolve the same text stack.
- **BREAKING: `OverflowPanel` and Overflow containers require an `ID`** — the
  overflow pass stores the visible-item count keyed by the container's ID. Two
  panels without one shared a single slot, overwrote each other's count every
  frame and forced a relayout on every frame, and each menu listed items with
  the other panel's count. `OverflowPanelCfg.ID` is now `gui:"required"`, and
  `OverflowPanel` and a `ContainerCfg` with `Overflow: true` panic on an empty
  ID. Migration: give each overflow panel or container a unique `ID`.
- **BREAKING: `SidebarCfg.Clip` is gone; sidebars always clip (#641)** — the
  sidebar animates its width from 0, so unclipped content stuck out for the
  whole slide, and the showcase sidebar drew 27.6px past its own edge at rest.
  The inner container now clips unconditionally. Migration: delete the `Clip`
  line from any `SidebarCfg`; content that must escape the panel belongs in a
  float or overlay, not in the sidebar tree.

- **`DrawRecorder` now documents that a points slice is valid for the duration
  of the call only** — under an active canvas transform the `[]float32` a
  recorder receives is a mapped copy in one shared scratch buffer, so two
  retained polylines both ended up holding the second one's coordinates. With no
  transform in force the caller's own slice is passed straight through and
  retention appeared to work, which is how an exporter ends up correct until its
  first `Translate`. No signature changed: an implementation that queues
  commands to serialize after the redraw must copy the points it is handed.

- **BREAKING: NumericInput with Min > Max now panics** — the bounds previously
  swapped silently and the field clamped the wrong way. Pass them in order; a
  NaN bound still counts as unset on its side.

### Fixed

- **A `Select` label keeps its descenders** — the closed-field label sits in a
  clipping wrapper (so a long value cannot push the disclosure arrow out) and
  takes the cap-band optical correction, which moves the text box down past the
  wrapper's bottom. The wrapper's clip then cut the tail of labels like "Cogs".
  The wrapper now grows by the applied shift, so the label stays inside the clip
  while the field's own height is unchanged.
- **A list marker no longer spills over the text beside it (#634)** — a bullet
  or number column was `Fixed` at a width computed from `prefixCharWidth`, a
  nominal per-character guess, while the marker text inside it was sized by the
  real text measurer. Any font whose glyphs were wider than the guess left the
  marker hanging outside its own column and over the item's text. The column is
  now `Fit` with that width as a `MinWidth`, so it grows to hold the marker and
  still keeps markers aligned down the list. The width is also counted in runes
  rather than bytes, which drops a hand-tuned correction that happened to be
  right for `"• "` and wrong for every other multi-byte marker.

- **`OnMouseLeave` no longer fires for a hover that ended frames ago** — the
  per-shape hover record was a flag, and a shape that stopped being walked
  (disabled, or not generated that frame) left its flag set. When the shape came
  back with the pointer elsewhere, it fired a leave for a hover that was long
  over. The record is now the frame the pointer was last inside, and only the
  current frame or the one before counts as still hovered.
- **An Overflow row counts the gap after a zero-width item** — the decision to
  reserve spacing read the width accumulated so far, so an in-flow child of zero
  width dropped one gap that positioning still applied, and the row kept a
  trailing item that did not fit.
- **A cached rich-text layout is shared, not copied** — every hit on the
  cross-frame RTF layout cache moved a copy of the shaped layout to the heap,
  once per rich-text shape per frame, which a Markdown page paid for every block
  it drew. The cache now hands out the layout it already holds.
- **An RTL scrollable row can reach the content it hides** — an RTL row runs
  leftward from its right edge, so what does not fit sits off the left, but the
  scroll offset moved the children further left still and no input could bring
  it back. The offset keeps its meaning and its range — a distance from the
  start edge, in `[maxOffset, 0]` — while the wheel, trackpad, keyboard, pan,
  thumb drag and gutter click now read mirrored for such a row, and the
  scrollbar thumb rests at the right end when unscrolled. RTL columns are
  unchanged.
- **A centered or end-aligned Wrap container places its rows correctly** — the
  rows a wrap builds were never given a cached content width, so alignment read
  0 and moved each row by its whole slack: a centered row started halfway across
  and its last item ran past the edge. Rows now align against the width of the
  items they hold.
- **A rotated widget no longer collapses an empty Canvas** — when a rotation
  changed a child's size, the re-fit of its Fit ancestors set an axis-less
  container (Canvas) with no in-flow child to size 0. The sizing pass keeps the
  current size in that case, and the re-fit now does the same.
- **MaxWidth and MaxHeight win over a larger minimum at every sizing step** —
  when a minimum was larger than the maximum, most sizing steps capped the size
  at the maximum, but the fill pass that shrinks a Fill child raised it to the
  minimum. The clamp used for Fill children could do either, depending on the
  starting size. Every step now caps at the maximum, and a container's computed
  minimum can no longer exceed its maximum on the cross axis.
- **A Fit scroll container no longer takes its children's minimum size** —
  sizing read `Clip` before the layout pass set it on scroll containers, so a
  Fit `Scrollable` container took its content's minimum and could not shrink and
  scroll. Sizing now treats a scroll container as clipped on each axis it
  scrolls. An axis that `ScrollMode` excludes keeps the content minimum, as
  before.
- **Fill children shrink beside a wider fixed sibling** — when a row was too
  narrow, the shrink pass searched for its largest child among Fixed and Fit
  siblings too. If that child was not Fill, nothing shrank and the row
  overflowed its parent: a wide label next to a Fill input in a narrow window.
  Only Fill children are now searched and share the shortfall.
- **Wrap and overflow rows align and scroll against their real content width** —
  a Fixed or Fill wrap that broke into rows kept its one-row content width, so a
  scrollable wrap scrolled sideways into empty space with a wrong-size thumb. An
  overflow row kept the width from before its hidden children were removed, so a
  centered row started left of its edge. Both now re-measure after the change.
- **An overflow row shows its last item when it fits without the trigger** —
  room for the trigger was reserved even for the last item, so a row where every
  item fit exactly hid the last one and showed an overflow menu.
- **Rotated RTL rows, and children of rotated containers, stay in place** — a
  quarter-turn RTL row moved its children in the wrong direction and drew them
  outside the container. Children of a rotated container ignored ancestor clips,
  so content scrolled out of a viewport still took clicks and hover.
- **Floats inside a disabled container are disabled** — a popover or menu lifted
  out of a disabled container took clicks, hover and Tab focus. It now inherits
  the disabled state of its host.
- **Float layout edge cases** — an auto-flipped float now keeps its offset as a
  gap on the new side instead of overlapping the anchor. Extreme `FloatZIndex`
  values no longer overflow the sort. A nested float with a lower Z than its
  host now anchors to the host's final position, not to a stale one.
- **Clip containers and Canvas refits agree with the width rules** — a Clip
  container no longer takes its children's minimum height, matching how widths
  already worked. A Fit Canvas that holds a rotated child keeps enclosing
  children placed at an offset instead of shrinking and clipping them.
- **`FindLayout` stops at the depth cap** — like every other layout walk, it no
  longer recurses without limit on a very deep tree.

- **Windows resize cursors stay visible between frames (#631)** — the frame loop
  overwrote the system resize cursor with the application's cursor, making
  window edges difficult to find. Cursor updates now apply only over app content
  or during a captured widget drag, leaving Windows in control over the native
  window frame.
- **Multiline Enter goes through the text filter** — pressing Enter in a
  multiline `Input` inserted the newline directly, bypassing the field's `Mask`
  and `PreTextChange` validator, so a validator never saw that rune and a masked
  field accepted a character no slot matches. The newline now passes through the
  same choke point as every other insertion: a veto leaves the text unchanged
  and a mask refuses it.
- **An uncompilable input mask panics instead of running unmasked** — a `Mask`
  whose custom token lacks a `Matcher` logged and fell back to a nil mask, so
  the field silently edited plain text. It now panics at construction like the
  inverted-bounds and bad-date-format checks, failing the typo loudly instead of
  dropping validation.
- **`NumericInput` treats ±Inf values as unset** — a `Value` or `Min` of ±Inf
  seeded stepping and committed text that formatted as `+Inf` (`-+Inf` with a
  sign), which no locale parses back. Non-finite seeds now fall through to the
  typed text, `Min`, and zero the way NaN already did, and a commit against one
  yields an empty value.
- **Gradient fills split at every stop when stops are out of order** — the
  gradient tessellator searched its stop breakpoints as a sorted list, but only
  sorted it for `SpreadReflect`. With pad or repeat spread and stops not in
  ascending offset order (an SVG `<linearGradient>` in document order, or a
  `CanvasGradient` built that way), triangles that crossed a stop were not
  split, and the fill smeared one color segment across them. The breakpoints are
  now sorted for every spread.
- **Overflow containers count placeholder children when hiding items** — an
  empty, floating or `OverDraw` child before the trigger made the overflow pass
  store too small an item index. When every item fit, the trigger stayed
  visible. When some did not, the `OverflowPanel` menu also listed items that
  were still in the row. The stored value is now the child index where hiding
  starts, which is the index `OverflowPanel` reads.
- **NumericInput re-parses its own output for multi-size `GroupSizes`** — with
  `GroupSizes: []int{3, 2}` the formatter repeats the last size for every
  further group and shows `1,23,45,67,890`, but the parser used size 3 past the
  end of the list and rejected that string. Commit, edit and arrow-key steps on
  a displayed value then failed with no error. Both paths now repeat the last
  configured size.
- **IME input is sanitized the same way on every backend** — the X11 preedit,
  the web composition/commit and the Android commit reached the widgets without
  the length cap and replacement-character strip the Win32 and X11 commit paths
  apply, so a hostile input source could push an unbounded string or visible
  decoding failures into the render path. All three now share the same
  strip-and-cap. The X11 preedit still emits when empty (that event ends the
  composition), and a missing web `data` value yields an empty string instead of
  the literal `"null"`. The Win32 caret pixel now rounds to nearest like
  `gui.imeCoord` and the X11 scaler instead of truncating, so one caret lands in
  one place on every backend. The X11 `IMEStart`/`IMEStop` calls are explicit
  no-ops with no input method present, matching `IMESetRect` and `drainIME`.
- **Rotated children no longer leave stale content sizes on their parents
  (#622)** — a child with `QuarterTurns` 1 or 3 swaps its width and height after
  the fill passes have cached each container's content width and height. Those
  caches were not updated, so a scrolling parent clamped its scroll range and
  sized its scrollbar thumb from the pre-swap extent, and a Fit container with
  centered or end alignment offset its children by the old difference. Every
  ancestor the swap re-fits, and the Fixed or Fill parent where the re-fit
  stops, now refreshes both caches.
- **Windows tray reports a failed window init on every call** — the tray's
  hidden message window is built once. If that build failed, only the first
  `Create` saw the error; every later `Create` returned success and registered
  an icon against no window, so its clicks and menu never arrived. The error is
  now kept on the tray, and every `Create` returns it.
- **Windows tray clicks arrive when the tray is made off the main thread
  (#616)** — Win32 gives a window's messages only to the thread that made the
  window. The tray made its window on the caller's thread but read messages on a
  different thread, so its own loop never got a message and held one OS thread
  for the life of the process. Clicks worked only because the gl backend's pump
  ran on the same thread as `OnInit`. If `SetSystemTray` was called from another
  goroutine, clicks and menu picks never arrived and no error was reported. The
  tray now makes its window and reads its messages on one thread that it owns,
  so it works from any goroutine. Each tray also registers its own window class,
  so a second tray no longer fails to register or sends its clicks to the first.
- **Windows tray reads its click events correctly and works from the keyboard
  (#617)** — the tray asked for `NOTIFYICON_VERSION_4` but decoded its callback
  in the older layout, where `wParam` is the icon ID. In version 4 `wParam` is a
  screen position and the icon ID sits in `lParam`, so clicks could miss their
  icon, a right click could fire the default action, and mouse moves could fire
  it too. The version request was also sent before the icon existed, and its
  result was not checked. The tray now sets the version after adding the icon,
  decodes the version 4 layout, fires the default action when the icon is
  selected by mouse or keyboard, and opens the menu, by mouse or keyboard, at
  the position the shell reports. If the shell refuses version 4,
  `SetSystemTray` returns an error and no icon is left behind.
- **Windows tray updates and removes the icon it added (#619)** — the tray added
  each icon by a new random GUID but changed and deleted it by its numeric ID.
  The shell matches an icon added with a GUID by that GUID, so
  `UpdateSystemTray` and `RemoveSystemTray` could miss the icon. Also, each
  launch could leave one more stale entry in the notification area settings. The
  tray now names the icon by window and numeric ID in every call.
  `UpdateSystemTray` frees the old icon handle only after the shell accepts the
  new one. An update with no icon no longer frees the icon still on screen,
  which `RemoveSystemTray` then freed a second time.
- **macOS frame pump survives a nested runloop inside a nested runloop** — the
  pump that repaints windows during a modal dialog, live resize or open menu
  shared one snapshot buffer across calls. When app code run by a pumped frame
  entered a deeper runloop, the timer called the pump again, which truncated and
  cleared that buffer while the outer call still walked it. The outer call then
  read a nil window and panicked, or pumped the wrong windows. Each call now
  holds the buffer for itself; only real re-entry allocates, so the 60 Hz tick
  stays allocation-free.

- **Linux screen readers get the correct widget states** — the AT-SPI2 bridge
  sent thirteen of its fourteen state flags at the wrong bit positions. Orca
  read every node as editable, multi-line and pressed, a focused widget as
  defunct, a busy one as checked, and a read-only field as a default button.
  Each position now matches `AtspiStateType` in at-spi2-core, and read-only uses
  `READ_ONLY` (43). A test checks each position against the numbers in the
  header.

- **`audio.Init` is safe to call from many goroutines at once** — `Init`, `quit`
  and `PlaySource` read and wrote the `initialized` flag with no lock. Two first
  calls to `Init` could both see it unset and open the output sink twice, and a
  shutdown racing `Init` could leave the flag out of step with the backend. A
  package mutex now covers the check, the backend call and the flag write, so
  the documented "call from any goroutine, idempotent" promise holds.
- **Volume and music calls no longer race the audio thread, and do nothing
  before `audio.Init`** — the audio thread read the master and music volumes and
  the music track state every buffer while the app wrote them with no
  synchronization. Volumes are now atomics, and the music track has a lock that
  both sides take. Before `Init`, `HaltMusic`, `FadeOutMusic` and the other
  music controls dereferenced a nil track and panicked; they now do nothing, and
  `Music.Play` and `Music.FadeIn` return an "audio: not initialized" error.
  `Music.Free` still closes the decoder before `Init`. Under `-race`, the
  package now runs its `TestRace*` tests instead of skipping everything.
- **Sound calls no longer race `audio.Init` or the audio thread, and do nothing
  before `Init`** — `Sound.Play`, `Sound.FadeIn` and the channel helpers read
  the mixer while `Init` built it, and a sound's volume was written by the app
  while the audio thread read it. Before `Init` they dereferenced a nil mixer
  and panicked. `Sound.Play`, `Sound.PlayOnce` and `Sound.FadeIn` now return an
  "audio: not initialized" error before `Init`, and `HaltChannel`, `IsPlaying`
  and the other channel helpers do nothing. A sound's volume and the output
  sample rate are atomics, so `LoadSoundBytes` and `SampleRate` still take no
  lock and a long decode does not block other audio calls.
- **Android accessibility getters reject negative indices** — the
  gomobile-exported `A11yNode*` getters checked only the upper bound, so a
  negative index from Kotlin (such as the `-1` root sentinel `A11yNodeParent`
  returns) panicked on the slice index and killed the app process. Every getter
  now checks both bounds through one helper and returns its fallback (`0`, `""`,
  or `-1` for `A11yNodeParent`).
- **Custom shaders render on Android** — `glesSetCustomPipeline` marked the
  bound program as "no pipeline", so `glesSetMVP` and `glesSetTM` returned early
  and never wrote the custom program's `mvp` and `tm` uniforms. `mvp` stayed
  zero, every vertex collapsed to the origin, and a custom shader drew nothing;
  its `Params` were lost too. The GLES backend now records which custom program
  is bound and writes both uniforms through its cached locations. Deleting the
  bound custom pipeline also unbinds it, so a rebuilt program that reuses the
  slot does not get stale writes.
- **Rounded clips show their content on Android** — `glesBeginStencilClip` and
  `glesEndStencilClip` bound the stencil program and drew the clip mask without
  writing its `mvp` uniform. Uniforms belong to one program, so the matrix set
  on the solid pipeline just before did not reach it. The stencil program kept a
  zero matrix, the mask covered no pixels, and every child of a rounded clip was
  clipped away. Both functions now take the frame MVP and write it after they
  bind the stencil program, as the desktop GL backend already does.
- **GPU clip rects round outward (#601)** — the GL and Metal backends converted
  a clip box to device pixels by truncating `x`, `y`, `w` and `h` on their own,
  putting the far edge at `floor(x) + floor(w)`. At a fractional DPI scale, or a
  fractional layout coordinate, that is up to a full device pixel short, so
  scrolled and clipped content lost a line of pixels along its right and bottom
  edges. Both backends now floor the near edge and ceil the far edge through a
  shared helper, the rule the software backend already used, so the device rect
  always contains the box. An empty clip stays empty.
- **`BoundedMap` keeps one ordering slot per key after `Delete`** — `Delete`
  left the key's slot in the order list on small maps, because compaction runs
  only past a size threshold. Setting the same key again appended a second slot:
  `Keys` and `Range` returned the key twice, clones copied the duplicate, and
  eviction removed the re-inserted key as the oldest entry, before keys that
  were older. `Delete` now removes the slot in place, with no allocation.
- **NumericInput inner identities join their scope** — the field and step-button
  IDs were composed from the unresolved leaf, stamping window-global identities
  that collided across scopes. They are now composed from the resolved ID (the
  datagrid pattern), the wrapper leaves the tab order to the field with the
  focus ring following it, and a click on the frame focuses the field. A step
  lost to float precision no longer sounds the refusal cue, and the step buttons
  take the `Click` color slot and consume their click.
- **Masked edits keep vertical navigation and local state stays private** —
  masked insert, paste and delete left `cursorOffset` at 0, pinning the next
  Up/Down to the left edge instead of recomputing the column from the caret;
  `formatRaw` scanned forward per literal instead of one suffix pass; and
  `numericLocaleNormalize` aliased the caller's `GroupSizes` and the package
  default instead of cloning.

- **IME preedit and commit input bounded against hostile input methods** — the
  preedit stored per window is capped at 4096 runes in `imeUpdate`, so an
  unbounded composition from any backend cannot grow memory or the per-frame
  render cost; X11 commit strings are stripped of replacement characters and
  capped the same way. X11 commits now arrive as a single `EventChar` carrying
  the whole string, matching the documented contract and every other backend, so
  a multi-rune CJK commit is one insert and one undo step instead of one event
  per rune.
- **IME candidate window follows the caret and rect reports stay finite** — the
  reported rect is anchored to the composition caret instead of the preedit
  start, `IMESetRect` rounds to nearest and clamps centrally (NaN lands on zero,
  infinities on the bound) instead of truncating, and the X11 backend caches the
  scaled rect like Win32 so the per-frame re-report costs a comparison instead
  of a D-Bus call. The web backend reports the selected clause from the hidden
  input's selection during `compositionupdate` instead of always sending a zero
  range.
- **Per-frame focus gates share one tree walk** — the render pass resolved the
  IME edit context and the caret-blink gate in two full walks; both now resolve
  off one depth-capped walk, so a focused frame pays half the traversal.

- **Faded and disabled images now fade the pixels, not just the backdrop** —
  `ImageCfg.Opacity` and the disabled dim reached only the `BgColor` fill while
  every backend painted the texels fully opaque, because the image shaders
  ignored the vertex color and `RenderCmd` carried no image alpha. Commands now
  carry `Opacity` (folded from shape opacity and the disabled dim at emit), the
  GL/Metal/GLES shaders multiply texel alpha by it, and the soft, web and PDF
  backends apply it the same way. A remote image that resolves to SVG keeps its
  ID, click handler, assistive label and sound instead of dropping them at the
  `svgView` handoff.
- **Remote image downloads land atomically with a strict type allowlist** —
  concurrent windows fetching one URL truncated each other's cache file, a
  crashed partial stayed servable under its final name, cache files were
  world-readable, and any `image/*` body (webp, gif) was stored under `.png`
  only to fail decode every frame after. Bodies now stream to a `0600` temp file
  that is renamed into place (a rename loser serves the winner), only
  PNG/JPEG/SVG content types are admitted, and an in-flight URL skips the
  per-frame filesystem probe.
- **Image validation, logging and placeholder text hardened** — `Image` warned
  on every frame for a missing file and embedded the full source in the
  `[missing: …]` layout; warnings are now once per window and the source is
  capped at 80 chars. `validateImagePath` rejects NUL and empty/dot paths like
  the backend gate, remote `.SVG` suffixes match case-insensitively, and the
  downloading placeholder allocates through the shape pool.

- **Canvas `Save` past its depth cap no longer unbalances `Restore`** — a
  `DrawContext.Save` beyond `maxXformDepth` (256) was dropped silently, so the
  matching `Restore` popped an ancestor instead and every nesting level after
  the cap drew at the wrong offset for the rest of the redraw. Dropped pushes
  are now counted and their `Restore`s are no-ops, which keeps the stack
  balanced however deep the nest goes. Nests shallower than 256 are unaffected.

- **Canvas transforms reject an overflow to infinity** — `ScaleBy` and
  `Translate` screened their arguments but not the result, so two
  `ScaleBy(1e38, 1e38)` calls left the matrix at `+Inf`. `DrawContext.Text` also
  drops an entry whose baked position or font size overflows, which a single
  finite `ScaleBy` can cause, as well as one handed non-finite coordinates or
  px-valued style fields with no transform in force. The cell and emoji-box
  widths are screened too, since the shaper reads them. This one matters beyond
  the usual non-finite screening because the canvas text emit path measures a
  style through the glyph shaper before render-command validation runs, so an
  infinite font size reached the shaper's cache and rasterizer.

- **A cached canvas no longer has its emitted geometry overwritten inside one
  render pass** — the cache entry stamped the render pass that last _redrew_ it
  but not one that merely _hit_ it, so two shapes sharing an effective ID and
  differing only in `Version` or size — the first hitting the cache, the second
  redrawing — let the redraw recycle the triangle buffers the first shape's
  already-emitted command still pointed at. A hit now claims the pass as a
  redraw does. Only reachable with duplicate effective IDs, which
  `TestDuplicateIDs` and `gui.Debug` already report.

- **A canvas no longer pins the text and images of its largest past redraw** —
  the recycled `Texts` and `Images` arrays were truncated with `[:0]`, leaving
  every entry past the new length live in the backing array for the life of the
  canvas, including each `Text` string and each image `Src` and `ImageFetcher`
  (which can close over an arbitrary graph). The entries are cleared on reuse;
  the capacity is still kept.
- **A data grid filter input stays inside a narrow column (#640)** — the filter
  `Input` took the theme field min-width floor (160), so any column narrower
  than that drew its input over the next column. The cell already sizes the
  input, so the input now opts out through the new exported
  `InputCfg.NoMinWidthFloor`, previously an in-package-only flag shared with
  `NumericInput` and `InputDate`. Leave it false on a standalone form field,
  where the floor keeps an empty field the width of a filled one.
- **Container alignment stays pinned when content overflows (#636)** — a
  centered or end-aligned row or column with content larger than the container
  moved the content off the start edge. This hid the first items. The alignment
  now treats negative leftover space as zero. This matches the cross-axis
  helpers. The content starts at the edge and clips only at the far side.
- **A horizontal-only scroll column keeps its height floor (#637)** — the Column
  main-axis reset dropped a Scrollable Fill container's minimum to 5px without
  checking `ScrollMode`, so a horizontal-only column could be squeezed below its
  content with no way to scroll to the rest. It now routes through
  `scrollFillResetMin` like every other axis, and the excluded axis keeps its
  floor.

## [v0.76.1] - 2026-09-13

### Fixed

- **Escape dismisses a dialog whose focused content holds its own key handler**
  — The per-dispatch dedup suppressed every focused target after the first, so a
  focused child with an `OnKeyDown` that declined Escape vetoed the dialog
  root's Escape handling: the dialog stayed open and `OnCancelNo` never fired.
  The dialog root now skips the dedup marks. A child that consumes Escape still
  overrides, since post-order dispatch reaches the child first and its consume
  short-circuits the dialog.

## Older releases

v0.76.0 and earlier are in [CHANGELOG-archive.md](CHANGELOG-archive.md).
