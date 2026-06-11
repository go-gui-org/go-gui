Determinate and indeterminate progress indicators with
optional percentage text overlay and vertical orientation.

## Usage

```go
gui.ProgressBar(gui.ProgressBarCfg{
    ID:      "loading",
    Percent: 0.75,
    Sizing:  gui.FillFit,
})
```

## Indefinite

```go
gui.ProgressBar(gui.ProgressBarCfg{
    ID:         "sync",
    Indefinite: true,
    Sizing:     gui.FillFit,
})
```

## Key Properties

| Property   | Type    | Description                          |
|------------|---------|--------------------------------------|
| ID         | string  | Unique identifier (required)         |
| Percent    | float32 | Progress 0.0–1.0                     |
| Text       | string  | Custom overlay text                  |
| TextShow   | bool    | Show percentage text overlay         |
| Indefinite | bool    | Animated looping mode                |
| Vertical   | bool    | Vertical orientation                 |
| Disabled   | bool    | Disable interaction                  |
| Invisible  | bool    | Hide without removing from layout    |
| Sizing     | Sizing  | Combined axis sizing mode            |
| Width      | float32 | Fixed width                          |
| Height     | float32 | Fixed height                         |
| MinWidth   | float32 | Minimum width                        |
| MaxWidth   | float32 | Maximum width                        |
| MinHeight  | float32 | Minimum height                       |
| MaxHeight  | float32 | Maximum height                       |

## Appearance

| Property       | Type         | Description                      |
|----------------|--------------|----------------------------------|
| Color          | Color        | Track background color           |
| ColorBar       | Color        | Fill bar color                   |
| TextBackground | Color        | Background behind percentage text|
| TextPadding    | Opt[Padding] | Padding around percentage text   |
| TextStyle      | TextStyle    | Percentage text styling          |
| Radius         | float32      | Corner radius                    |

## Accessibility

| Property        | Type   | Description                      |
|-----------------|--------|----------------------------------|
| A11YLabel       | string | Accessible label                 |
| A11YDescription | string | Accessible description           |
