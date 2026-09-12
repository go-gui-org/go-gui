Text input with calendar popup for date entry. Combines a text field with an
inline date picker dropdown. The date shows in the current locale's format
unless `DateFormat` sets one for the field.

## Usage

```go
gui.InputDate(gui.InputDateCfg{
    ID:          "id",
    Date:        app.Date,
    Sizing:      gui.FillFit,
    Placeholder: "Select date...",
    OnSelect: func(dates []time.Time, ctx gui.EventCtx) {
        gui.State[App](ctx.Window).Date = dates[0]
    },
})
```

## Date Format

`DateFormat` sets the format for one field. It controls the text the field
shows, the input mask, the placeholder hint, and the parse of what the user
types. Use the tokens `YYYY`, `MM`, `M`, `DD`, `D` and the separator you want.

```go
gui.InputDate(gui.InputDateCfg{
    ID:         "id-de",
    Date:       app.Date,
    DateFormat: "DD.MM.YYYY", // 24.12.2026
})
```

Leave `DateFormat` empty to use the locale. To change every date field at once,
set the locale instead: `ctx.Window.SetLocaleID("de-DE")`.

Month-name tokens (`MMM`, `MMMM`), a 2-digit year (`YY`) and time tokens (`HH`,
`mm`, `ss`) are not permitted. The field accepts digits and separators only and
the parse reads `YYYY`, `MM`, `M`, `DD`, `D`, so anything else could be shown
but never typed back.

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
    OnSelect: func(dates []time.Time, ctx gui.EventCtx) {
        gui.State[App](ctx.Window).Date = dates[0]
    },
})
```

## Key Properties

| Property             | Type                 | Description                       |
| -------------------- | -------------------- | --------------------------------- |
| Date                 | time.Time            | Current date value                |
| Placeholder          | string               | Hint text shown when empty        |
| DateFormat           | string               | Date format for this field        |
| SelectMultiple       | bool                 | Allow multiple date selection     |
| MondayFirstDayOfWeek | bool                 | Start week on Monday              |
| ShowAdjacentMonths   | bool                 | Show prev/next month days         |
| HideTodayIndicator   | bool                 | Hide today border highlight       |
| WeekdaysLen          | DatePickerWeekdayLen | Weekday header label length       |
| Disabled             | bool                 | Disable interaction               |
| Invisible            | bool                 | Hide without removing from layout |
| Sizing               | Sizing               | Combined axis sizing mode         |
| Width                | float32              | Fixed width                       |
| Height               | float32              | Fixed height                      |
| MinWidth             | float32              | Minimum width                     |
| MaxWidth             | float32              | Maximum width                     |

## Filtering

| Property        | Type                 | Description                 |
| --------------- | -------------------- | --------------------------- |
| AllowedWeekdays | []DatePickerWeekdays | Restrict to specific days   |
| AllowedMonths   | []DatePickerMonths   | Restrict to specific months |
| AllowedYears    | []int                | Restrict to specific years  |
| AllowedDates    | []time.Time          | Restrict to specific dates  |

## Appearance

| Property         | Type         | Description                                                      |
| ---------------- | ------------ | ---------------------------------------------------------------- |
| Padding          | Opt[Padding] | Inner padding                                                    |
| SizeBorder       | Opt[float32] | Border width                                                     |
| CellSpacing      | Opt[float32] | Gap between calendar day cells                                   |
| Radius           | Opt[float32] | Corner radius                                                    |
| RadiusBorder     | Opt[float32] | Outer border radius                                              |
| Color            | Color        | Background color (shorthand for `Colors.Base`)                   |
| Colors           | ColorSet     | Per-state colors: Base, Hover, Click, Focus, Border, BorderFocus |
| ColorSelect      | Color        | Selected date highlight                                          |
| TextStyle        | TextStyle    | Text styling                                                     |
| PlaceholderStyle | TextStyle    | Placeholder text styling                                         |

## Events

| Callback | Signature                   | Fired when       |
| -------- | --------------------------- | ---------------- |
| OnSelect | func([]time.Time, EventCtx) | Date(s) selected |
| OnEnter  | func(EventCtx)              | Enter pressed    |

## Accessibility

| Property | Type    | Description                          |
| -------- | ------- | ------------------------------------ |
| A11YCfg  | A11YCfg | Embedded: A11YLabel, A11YDescription |

Set the pair through the embed: `A11YCfg: gui.A11YCfg{A11YLabel: "Save"}`.
