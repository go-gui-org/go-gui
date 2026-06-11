Single-line, password, and multiline text input with IME
composition, keyboard focus, masked input, and accessibility support.

## Usage

```go
gui.Input(gui.InputCfg{
    ID:          "name",
    IDFocus:     100,
    Sizing:      gui.FillFit,
    Text:        app.Name,
    Placeholder: "Enter name...",
    OnTextChanged: func(_ *gui.Layout, s string, w *gui.Window) {
        gui.State[App](w).Name = s
    },
})
```

## Password

```go
gui.Input(gui.InputCfg{
    ID:         "pw",
    IDFocus:    101,
    IsPassword: true,
    Text:       app.Password,
    OnTextChanged: func(_ *gui.Layout, s string, w *gui.Window) {
        gui.State[App](w).Password = s
    },
})
```

## Multiline

```go
gui.Input(gui.InputCfg{
    ID:     "notes",
    IDFocus: 102,
    Mode:   gui.InputMultiline,
    Height: 90,
    Text:   app.Notes,
    OnTextChanged: func(_ *gui.Layout, s string, w *gui.Window) {
        gui.State[App](w).Notes = s
    },
})
```

## Masked Input

```go
gui.Input(gui.InputCfg{
    ID:         "phone",
    MaskPreset: gui.MaskPhoneUS,
    Text:       app.Phone,
})
```

Presets: MaskPhoneUS, MaskCreditCard16, MaskCreditCardAmex,
MaskExpiryMMYY, MaskCVC. For custom masks, set Mask (pattern string)
and MaskTokens (custom token definitions).

## Key Properties

| Property         | Type            | Description                          |
|------------------|-----------------|--------------------------------------|
| Text             | string          | Current text value                   |
| Placeholder      | string          | Hint text shown when empty           |
| IsPassword       | bool            | Mask characters for password entry   |
| Mode             | InputMode       | InputSingleLine or InputMultiline    |
| MaskPreset       | InputMaskPreset | Built-in mask (phone, card, etc.)    |
| Mask             | string          | Custom mask pattern                  |
| MaskTokens       | []MaskTokenDef  | Custom token definitions for mask    |
| Disabled         | bool            | Disable interaction                  |
| Height           | float32         | Height (useful for multiline)        |
| MinWidth         | float32         | Minimum width                        |
| MaxWidth         | float32         | Maximum width                        |
| IDFocus          | uint32          | Tab-order focus ID (> 0 to enable)   |

## Appearance

| Property         | Type      | Description                          |
|------------------|-----------|--------------------------------------|
| Padding          | Opt[Padding] | Inner padding                     |
| Radius           | Opt[float32] | Corner radius                     |
| SizeBorder       | Opt[float32] | Border width                      |
| Color            | Color     | Background color                     |
| ColorHover       | Color     | Background on hover                  |
| ColorBorder      | Color     | Border color                         |
| ColorBorderFocus | Color     | Border color when focused            |
| TextStyle        | TextStyle | Text styling                         |
| PlaceholderStyle | TextStyle | Placeholder text styling             |

## Events

| Callback            | Signature                                          | Fired when                           |
|---------------------|----------------------------------------------------|--------------------------------------|
| OnTextChanged       | func(*Layout, string, *Window)                     | Text changes                         |
| OnTextCommit        | func(*Layout, string, InputCommitReason, *Window)  | Enter pressed or focus lost          |
| OnEnter             | func(*Layout, *Event, *Window)                     | Enter pressed (single-line)          |
| OnKeyDown           | func(*Layout, *Event, *Window)                     | Unhandled key event                  |
| OnBlur              | func(*Layout, *Window)                             | Focus lost                           |
| PreTextChange       | func(current, proposed string) (string, bool)      | Validate/transform before change     |
| PostCommitNormalize | func(text string, InputCommitReason) string        | Normalize text on commit             |

## Accessibility

| Property        | Type   | Description                          |
|-----------------|--------|--------------------------------------|
| A11YLabel       | string | Accessible label                     |
| A11YDescription | string | Accessible description               |
