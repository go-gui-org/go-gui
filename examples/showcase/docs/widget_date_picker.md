Calendar-style date selection with month/year navigation.
Supports single and multi-select, weekday/month/year filtering,
and locale-aware weekday headers.

## Usage

```go
gui.DatePicker(gui.DatePickerCfg{
    ID:    "dp",
    Dates: app.SelectedDates,
    OnSelect: func(dates []time.Time, _ *gui.Event, w *gui.Window) {
        gui.State[App](w).SelectedDates = dates
    },
})
```

## Multiple Selection

```go
gui.DatePicker(gui.DatePickerCfg{
    ID:             "dp-multi",
    Dates:          app.SelectedDates,
    SelectMultiple: true,
    OnSelect: func(dates []time.Time, _ *gui.Event, w *gui.Window) {
        gui.State[App](w).SelectedDates = dates
    },
})
```

## Weekday Filtering

```go
gui.DatePicker(gui.DatePickerCfg{
    ID: "dp-weekdays",
    AllowedWeekdays: []gui.DatePickerWeekdays{
        gui.DatePickerMonday, gui.DatePickerWednesday, gui.DatePickerFriday,
    },
    OnSelect: func(dates []time.Time, _ *gui.Event, w *gui.Window) {
        gui.State[App](w).SelectedDates = dates
    },
})
```

## Key Properties

| Property             | Type                 | Description                        |
|----------------------|----------------------|------------------------------------|
| Dates                | []time.Time          | Currently selected date(s)         |
| SelectMultiple       | bool                 | Allow multiple date selection      |
| MondayFirstDayOfWeek | bool                 | Start week on Monday               |
| ShowAdjacentMonths   | bool                 | Show prev/next month days          |
| HideTodayIndicator   | bool                 | Hide today border highlight        |
| WeekdaysLen          | DatePickerWeekdayLen | Header label length                |
| IDFocus              | uint32               | Tab-order focus ID (> 0 to enable) |
| Disabled             | bool                 | Disable interaction                |
| Invisible            | bool                 | Hide without removing from layout  |

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
| CellSpacing      | Opt[float32] | Gap between day cells              |
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

## Events

| Callback | Signature                            | Fired when           |
|----------|--------------------------------------|----------------------|
| OnSelect | func([]time.Time, *Event, *Window)   | Date(s) selected     |

## Accessibility

| Property        | Type   | Description                        |
|-----------------|--------|------------------------------------|
| A11YLabel       | string | Accessible label                   |
| A11YDescription | string | Accessible description             |

## Weekday Label Lengths

| Constant           | Example |
|--------------------|---------|
| WeekdayOneLetter   | S       |
| WeekdayThreeLetter | Sun     |
| WeekdayFull        | Sunday  |

## Keyboard

- **Left/Right** -- navigate months
- Click month/year header to toggle roller picker
- In roller mode: **Up/Down** = month, **Shift+Up/Down** = year

## API

`w.DatePickerReset(id)` clears picker state.
