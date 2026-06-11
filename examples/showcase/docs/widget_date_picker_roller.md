Rolling drum-style date selection. Supports mouse
scroll, click, and keyboard input. Each date component (day, month,
year) renders as a scrollable drum.

## Usage

```go
gui.DatePickerRoller(gui.DatePickerRollerCfg{
    ID:           "dpr",
    SelectedDate: time.Now(),
    DisplayMode:  gui.RollerMonthDayYear,
    VisibleItems: 5,
    LongMonths:   true,
    OnChange: func(t time.Time, w *gui.Window) {
        gui.State[App](w).Date = t
    },
})
```

## Year-Only Roller

```go
gui.DatePickerRoller(gui.DatePickerRollerCfg{
    ID:          "dpr-year",
    DisplayMode: gui.RollerYearOnly,
    MinYear:     2020,
    MaxYear:     2030,
    OnChange: func(t time.Time, w *gui.Window) {
        gui.State[App](w).Year = t.Year()
    },
})
```

## Display Modes

| Constant           | Format    | Description              |
|--------------------|-----------|--------------------------|
| RollerDayMonthYear | DD MMM YYYY | Day, month, year drums (default) |
| RollerMonthDayYear | MMM DD YYYY | Month, day, year drums   |
| RollerMonthYear    | MMM YYYY  | Month and year only      |
| RollerYearOnly     | YYYY      | Single year drum         |

## Key Properties

| Property     | Type                       | Description                        |
|--------------|----------------------------|------------------------------------|
| SelectedDate | time.Time                  | Currently selected date            |
| DisplayMode  | DatePickerRollerDisplayMode | Drum layout mode                  |
| VisibleItems | int                        | Visible rows per drum (must be odd)|
| ItemHeight   | float32                    | Row height in pixels               |
| LongMonths   | bool                       | "January" vs "Jan"                 |
| MinYear      | int                        | Earliest year (default 1900)       |
| MaxYear      | int                        | Latest year (default 2100)         |
| IDFocus      | uint32                     | Tab-order focus ID (> 0 to enable) |
| MinWidth     | float32                    | Minimum width                      |
| MaxWidth     | float32                    | Maximum width                      |

## Appearance

| Property    | Type         | Description                        |
|-------------|--------------|------------------------------------|
| Color       | Color        | Background color                   |
| ColorBorder | Color        | Border color                       |
| SizeBorder  | Opt[float32] | Border width                       |
| Radius      | Opt[float32] | Corner radius                      |
| Padding     | Opt[Padding] | Inner padding                      |
| TextStyle   | TextStyle    | Text styling                       |

## Events

| Callback | Signature                | Fired when           |
|----------|--------------------------|----------------------|
| OnChange | func(time.Time, *Window) | Date changed         |

## Accessibility

| Property        | Type   | Description                        |
|-----------------|--------|------------------------------------|
| A11YLabel       | string | Accessible label                   |
| A11YDescription | string | Accessible description             |

## Keyboard

- **Up/Down** -- change month
- **Shift+Up/Down** -- change year
- **Alt+Up/Down** -- change day
- **Escape** -- exit roller view
