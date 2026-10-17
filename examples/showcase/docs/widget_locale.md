Global locale system controlling number formatting, date formatting, currency
symbols, UI strings (OK, Cancel, and more), weekday/month names, text direction,
and app-level translations. Ten locales are registered by default. You can add
custom locales via `LocaleRegister` or load them from JSON bundles.

## Set Locale

```go
// Set directly
gui.SetLocale(gui.LocaleDeDE)

// Set by ID on a window (also refreshes)
w.SetLocaleID("de-DE")

// Auto-detect from OS
gui.LocaleAutoDetect()
```

## Register Custom Locale

```go
gui.LocaleRegister(gui.Locale{
    ID:      "nl-NL",
    TextDir: gui.TextDirLTR,
    Number: gui.NumberFormat{
        DecimalSep: ',',
        GroupSep:   '.',
        GroupSizes: []int{3},
        MinusSign:  '-',
        PlusSign:   '+',
    },
    Date: gui.DateFormat{
        ShortDate:      "D-M-YYYY",
        LongDate:       "D MMMM YYYY",
        MonthYear:      "MMMM YYYY",
        FirstDayOfWeek: 1,
    },
    StrOK:     "OK",
    StrCancel: "Annuleren",
    StrYes:    "Ja",
    StrNo:     "Nee",
})
```

## Load from JSON

```go
locale, err := gui.LocaleLoad("locales/nl-NL.json")
if err == nil {
    gui.LocaleRegister(locale)
}
```

## Built-in Locales

| ID    | Language             | Text Dir |
| ----- | -------------------- | -------- |
| en-US | English (US)         | LTR      |
| de-DE | German               | LTR      |
| fr-FR | French               | LTR      |
| es-ES | Spanish              | LTR      |
| pt-BR | Portuguese (Brazil)  | LTR      |
| ja-JP | Japanese             | LTR      |
| zh-CN | Chinese (Simplified) | LTR      |
| ko-KR | Korean               | LTR      |
| ar-SA | Arabic               | RTL      |
| he-IL | Hebrew               | RTL      |

## Locale Struct

| Field         | Type              | Description                              |
| ------------- | ----------------- | ---------------------------------------- |
| ID            | string            | Locale identifier (for example, "en-US") |
| TextDir       | TextDirection     | TextDirLTR or TextDirRTL                 |
| Number        | NumberFormat      | Number formatting rules                  |
| Date          | DateFormat        | Date formatting rules                    |
| Currency      | CurrencyFormat    | Currency formatting rules                |
| Translations  | map[string]string | App-level translation keys               |
| WeekdaysShort | [7]string         | Sun..Sat short names                     |
| WeekdaysFull  | [7]string         | Sun..Sat full names                      |
| MonthsShort   | [12]string        | Jan..Dec short names                     |
| MonthsFull    | [12]string        | Jan..Dec full names                      |

## NumberFormat

| Field      | Type  | Description                        |
| ---------- | ----- | ---------------------------------- |
| DecimalSep | rune  | Decimal separator (default '.')    |
| GroupSep   | rune  | Thousands separator (default ',')  |
| GroupSizes | []int | Digit grouping sizes (default [3]) |
| MinusSign  | rune  | Minus sign (default '-')           |
| PlusSign   | rune  | Plus sign (default '+')            |

## DateFormat

| Field          | Type   | Description                                  |
| -------------- | ------ | -------------------------------------------- |
| ShortDate      | string | Short date pattern (for example, "M/D/YYYY") |
| LongDate       | string | Long date pattern                            |
| MonthYear      | string | Month-year pattern                           |
| FirstDayOfWeek | uint8  | 0=Sunday, 1=Monday                           |
| Use24H         | bool   | Use 24-hour time format                      |

## CurrencyFormat

| Field    | Type                 | Description                        |
| -------- | -------------------- | ---------------------------------- |
| Symbol   | string               | Currency symbol (for example, "$") |
| Code     | string               | ISO code (for example, "USD")      |
| Position | NumericAffixPosition | AffixPrefix or AffixSuffix         |
| Spacing  | bool                 | Space between symbol and number    |
| Decimals | int                  | Decimal places (default 2)         |

## UI Strings

| Field     | Default  | Description         |
| --------- | -------- | ------------------- |
| StrOK     | "OK"     | OK button label     |
| StrCancel | "Cancel" | Cancel button label |
| StrYes    | "Yes"    | Yes button label    |
| StrNo     | "No"     | No button label     |
| StrSave   | "Save"   | Save action         |
| StrDelete | "Delete" | Delete action       |
| StrSearch | "Search" | Search action       |

## Format Numbers and Currency

`LocaleFormatNumber(v, decimals)` and `LocaleFormatCurrency(v)` format a value
with the active locale. Use them for text in a `Text`, a table cell or a label.
They read the locale without a copy, so a view can call them every frame.

| Call                                 | en-US          | de-DE          |
| ------------------------------------ | -------------- | -------------- |
| `LocaleFormatNumber(1234567.891, 2)` | `1,234,567.89` | `1.234.567,89` |
| `LocaleFormatNumber(-0.001, 2)`      | `0.00`         | `0,00`         |
| `LocaleFormatCurrency(-1234.5)`      | `-$1,234.50`   | `-1.234,50 €`  |

Currency uses `Currency.Decimals`, `Symbol`, `Position` and `Spacing`. NaN and
±Inf come back as `NaN`, `+Inf` and `-Inf`.

## What Changes

- Date picker day/month names and first-day-of-week
- Numeric input decimal/thousands separators
- Dialog button labels (OK, Cancel, Yes, No)
- RTL text direction (ar-SA, he-IL)
- Currency symbols and placement

## API

| Function                        | Description                          |
| ------------------------------- | ------------------------------------ |
| SetLocale(l)                    | Set active global locale             |
| CurrentLocale()                 | Get active locale                    |
| w.SetLocale(l)                  | Set locale and refresh window        |
| w.SetLocaleID(id)               | Set locale by ID and refresh         |
| LocaleAutoDetect()              | Detect OS locale, set best match     |
| LocaleRegister(l)               | Add locale to registry               |
| LocaleGet(id)                   | Look up locale by ID                 |
| LocaleRegisteredNames()         | List all registered locale IDs       |
| LocaleLoad(path)                | Load locale from JSON file           |
| LocaleParse(json)               | Parse locale from JSON string        |
| LocaleFormatDate(t, format)     | Format date with locale month names  |
| LocaleFormatNumber(v, decimals) | Format number with locale separators |
| LocaleFormatCurrency(v)         | Format amount with locale currency   |
| LocaleT(key)                    | Look up translation key              |
