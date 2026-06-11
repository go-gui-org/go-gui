Locale-aware numeric input with optional step buttons,
currency/percent modes, and min/max validation.

## Usage

```go
gui.NumericInput(gui.NumericInputCfg{
    ID:          "qty",
    IDFocus:     200,
    Placeholder: "Enter number",
    Decimals:    2,
    Min:         gui.SomeD(0),
    Max:         gui.SomeD(999),
    OnValueCommit: func(_ *gui.Layout, v gui.Opt[float64], s string, w *gui.Window) {
        gui.State[App](w).Qty = v
    },
})
```

## Currency Mode

```go
gui.NumericInput(gui.NumericInputCfg{
    ID:   "price",
    Mode: gui.NumericCurrency,
    CurrencyCfg: gui.NumericCurrencyModeCfg{Symbol: "$"},
})
```

## Key Properties

| Property    | Type                   | Description                          |
|-------------|------------------------|--------------------------------------|
| Text        | string                 | Current text value                   |
| Value       | Opt[float64]           | Parsed numeric value                 |
| Placeholder | string                 | Hint text shown when empty           |
| Decimals    | int                    | Decimal places                       |
| Min         | Opt[float64]           | Minimum allowed value                |
| Max         | Opt[float64]           | Maximum allowed value                |
| Mode        | NumericInputMode       | Number, currency, or percent         |
| StepCfg     | NumericStepCfg         | Step button configuration            |
| CurrencyCfg | NumericCurrencyModeCfg | Currency mode settings               |
| PercentCfg  | NumericPercentModeCfg  | Percent mode settings                |
| Locale      | NumericLocaleCfg       | Locale formatting rules              |
| IDFocus     | uint32                 | Tab-order focus ID (> 0 to enable)   |
| Disabled    | bool                   | Disable interaction                  |
| Invisible   | bool                   | Hide without removing from layout    |
| Sizing      | Sizing                 | Combined axis sizing mode            |
| Width       | float32                | Fixed width                          |
| Height      | float32                | Fixed height                         |
| MinWidth    | float32                | Minimum width                        |
| MaxWidth    | float32                | Maximum width                        |
| MinHeight   | float32                | Minimum height                       |
| MaxHeight   | float32                | Maximum height                       |

## Appearance

| Property         | Type         | Description                      |
|------------------|--------------|----------------------------------|
| Padding          | Opt[Padding] | Inner padding                    |
| Radius           | Opt[float32] | Corner radius                    |
| SizeBorder       | Opt[float32] | Border width                     |
| Color            | Color        | Background color                 |
| ColorHover       | Color        | Background on hover              |
| ColorBorder      | Color        | Border color                     |
| ColorBorderFocus | Color        | Border color when focused        |
| TextStyle        | TextStyle    | Text styling                     |
| PlaceholderStyle | TextStyle    | Placeholder text styling         |

## Events

| Callback      | Signature                                        | Fired when                   |
|---------------|--------------------------------------------------|------------------------------|
| OnTextChanged | func(*Layout, string, *Window)                   | Text changes                 |
| OnValueCommit | func(*Layout, Opt[float64], string, *Window)     | Value committed (blur/enter) |

## Accessibility

| Property        | Type   | Description                          |
|-----------------|--------|--------------------------------------|
| A11YLabel       | string | Accessible label                     |
| A11YDescription | string | Accessible description               |
