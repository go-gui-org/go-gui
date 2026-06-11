Generate a complete theme from a `ThemeCfg` struct using
`ThemeMaker`. Provides full control over colors, sizing, padding,
radius, spacing, text sizes, and per-widget styles. Six built-in
themes are included as starting points.

## Usage

```go
cfg := gui.ThemeCfg{
    Name:            "ocean",
    ColorBackground: gui.ColorFromHSV(220, 0.15, 0.19),
    TextStyleDef:    gui.ThemeDark.Cfg.TextStyleDef,
    TitlebarDark:    true,
}
gui.SetTheme(gui.ThemeMaker(cfg))
```

## Built-in Themes

| Theme                | Description                          |
|----------------------|--------------------------------------|
| ThemeDark            | Dark with padding and radius         |
| ThemeDark.WithBorders(true)    | Dark with visible borders            |
| ThemeDark.WithPadding(false)   | Dark, compact                        |
| ThemeLight           | Light with padding and radius        |
| ThemeLight.WithBorders(true)   | Light with visible borders           |
| ThemeLight.WithPadding(false)  | Light, compact                       |

## ThemeCfg Color Properties

| Property         | Type  | Description                          |
|------------------|-------|--------------------------------------|
| ColorBackground  | Color | Window background                    |
| ColorPanel       | Color | Panel/card background                |
| ColorInterior    | Color | Input field interior                 |
| ColorHover       | Color | Hover highlight                      |
| ColorFocus       | Color | Focus ring color                     |
| ColorActive      | Color | Active/pressed state                 |
| ColorBorder      | Color | Default border color                 |
| ColorBorderFocus | Color | Border when focused                  |
| ColorSelect      | Color | Selection highlight                  |
| ColorSuccess     | Color | Success state color                  |
| ColorWarning     | Color | Warning state color                  |
| ColorError       | Color | Error state color                    |

## ThemeCfg Layout Properties

| Property      | Type    | Description                          |
|---------------|---------|--------------------------------------|
| Fill          | bool    | Fill widget backgrounds              |
| FillBorder    | bool    | Show widget borders                  |
| Padding       | Padding | Default padding                      |
| SizeBorder    | float32 | Default border width                 |
| Radius        | float32 | Default corner radius                |
| TitlebarDark  | bool    | Request dark OS titlebar             |

## ThemeCfg Size Scales

| Property       | Type    | Description                          |
|----------------|---------|--------------------------------------|
| PaddingSmall   | Padding | Small padding preset                 |
| PaddingMedium  | Padding | Medium padding preset                |
| PaddingLarge   | Padding | Large padding preset                 |
| RadiusSmall    | float32 | Small radius                         |
| RadiusMedium   | float32 | Medium radius                        |
| RadiusLarge    | float32 | Large radius                         |
| SpacingSmall   | float32 | Small spacing                        |
| SpacingMedium  | float32 | Medium spacing                       |
| SpacingLarge   | float32 | Large spacing                        |
| SizeTextTiny   | float32 | Tiny text size                       |
| SizeTextXSmall | float32 | XSmall text size                     |
| SizeTextSmall  | float32 | Small text size                      |
| SizeTextMedium | float32 | Medium text size                     |
| SizeTextLarge  | float32 | Large text size                      |
| SizeTextXLarge | float32 | XLarge text size                     |

## Color Strategies (Showcase)

| Strategy   | Description                          |
|------------|--------------------------------------|
| Mono       | Single-hue variations                |
| Complement | Opposite hue accent                  |
| Analogous  | Adjacent hue palette                 |
| Triadic    | Three evenly-spaced hues             |
| Warm       | Warm-shifted palette                 |
| Cool       | Cool-shifted palette                 |

Use the tint slider to control surface saturation. Dark mode
is derived automatically from the same seed color.
