Text input with calendar popup for date entry. Combines a
text field with an inline date picker dropdown. Displays the selected
date formatted via the current locale.

## Usage

```go
gui.InputDate(gui.InputDateCfg{
    ID:          "id",
    IDFocus:     100,
    Date:        app.Date,
    Sizing:      gui.FillFit,
    Placeholder: "Select date...",
    OnSelect: func(dates []time.Time, _ *gui.Event, w *gui.Window) {
        gui.State[App](w).Date = dates[0]
    },
})
```

## With Filtering

```go
gui.InputDate(gui.InputDateCfg{
    ID:   "id-weekday",
    Date: app.Date,
    AllowedWeekdays: []gui.DatePickerWeekdays{
        gui.DatePickerMonday, gui.DatePickerTuesday,
        gui.DatePickerWednesday, gui.DatePickerThursday,
        gui.DatePickerFriday,
    },
    OnSelect: func(dates []time.Time, _ *gui.Event, w *gui.Window) {
        gui.State[App](w).Date = dates[0]
    },
})
```

## Key Properties

| Property             | Type                 | Description                        |
|----------------------|----------------------|------------------------------------|
| Date                 | time.Time            | Current date value                 |
| Placeholder          | string               | Hint text shown when empty         |
| SelectMultiple       | bool                 | Allow multiple date selection      |
| MondayFirstDayOfWeek | bool                 | Start week on Monday               |
| ShowAdjacentMonths   | bool                 | Show prev/next month days          |
| HideTodayIndicator   | bool                 | Hide today border highlight        |
| WeekdaysLen          | DatePickerWeekdayLen | Weekday header label length        |
| IDFocus              | uint32               | Tab-order focus ID (> 0 to enable) |
| Disabled             | bool                 | Disable interaction                |
| Invisible            | bool                 | Hide without removing from layout  |
| Sizing               | Sizing               | Combined axis sizing mode          |
| Width                | float32              | Fixed width                        |
| Height               | float32              | Fixed height                       |
| MinWidth             | float32              | Minimum width                      |
| MaxWidth             | float32              | Maximum width                      |

## Filtering

| Property        | Type                 | Description                        |
|-----------------|----------------------|------------------------------------|
| AllowedWeekdays | []DatePickerWeekdays | Restrict to specific days          |
| AllowedMonths   | []DatePickerMonths   | Restrict to specific months        |
| AllowedYears    | []int                | Restrict to specific years         |
| AllowedDates    | []time.Time          | Restrict to specific dates         |

## Appearance

| Property         | Type         | Description                        |
|------------------|--------------|------------------------------------|
| Padding          | Opt[Padding] | Inner padding                      |
| SizeBorder       | Opt[float32] | Border width                       |
| CellSpacing      | Opt[float32] | Gap between calendar day cells     |
| Radius           | Opt[float32] | Corner radius                      |
| RadiusBorder     | Opt[float32] | Outer border radius                |
| Color            | Color        | Background color                   |
| ColorHover       | Color        | Background on hover                |
| ColorFocus       | Color        | Background when focused            |
| ColorClick       | Color        | Background on click                |
| ColorBorder      | Color        | Border color                       |
| ColorBorderFocus | Color        | Border color when focused          |
| ColorSelect      | Color        | Selected date highlight            |
| TextStyle        | TextStyle    | Text styling                       |
| PlaceholderStyle | TextStyle    | Placeholder text styling           |

## Events

| Callback | Signature                            | Fired when           |
|----------|--------------------------------------|----------------------|
| OnSelect | func([]time.Time, *Event, *Window)   | Date(s) selected     |
| OnEnter  | func(*Layout, *Event, *Window)       | Enter pressed        |

## Accessibility

| Property        | Type   | Description                        |
|-----------------|--------|------------------------------------|
| A11YLabel       | string | Accessible label                   |
| A11YDescription | string | Accessible description             |
