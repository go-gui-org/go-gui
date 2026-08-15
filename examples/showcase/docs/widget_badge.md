Numeric and colored pill labels for counts and status indicators. Dot mode
renders a small circle; labeled mode renders text inside a rounded pill.

## Usage

```go
gui.Badge(gui.BadgeCfg{Label: "5", Variant: gui.BadgeInfo})
```

## Dot Mode

```go
gui.Badge(gui.BadgeCfg{Dot: true, Variant: gui.BadgeSuccess})
```

## Capped Count

```go
gui.Badge(gui.BadgeCfg{Label: "150", Max: 99})
// Displays "99+"
```

## Key Properties

| Property | Type         | Description                           |
| -------- | ------------ | ------------------------------------- |
| Label    | string       | Badge text                            |
| Variant  | BadgeVariant | Color variant preset                  |
| Max      | int          | Cap value; shows "max+" when exceeded |
| Dot      | bool         | Show as a small dot instead of text   |

## Appearance

| Property  | Type         | Description                  |
| --------- | ------------ | ---------------------------- |
| Color     | Color        | Custom background color      |
| DotSize   | Opt[float32] | Dot diameter (dot mode only) |
| Padding   | Opt[Padding] | Inner padding                |
| Radius    | Opt[float32] | Corner radius                |
| TextStyle | TextStyle    | Label text styling           |

## Variants

| Variant      | Use case        |
| ------------ | --------------- |
| BadgeDefault | Custom color    |
| BadgeInfo    | Informational   |
| BadgeSuccess | Positive status |
| BadgeWarning | Needs attention |
| BadgeError   | Critical        |

## Accessibility

| Property | Type    | Description                          |
| -------- | ------- | ------------------------------------ |
| A11YCfg  | A11YCfg | Embedded: A11YLabel, A11YDescription |

Set the pair through the embed: `A11YCfg: gui.A11YCfg{A11YLabel: "Save"}`.
