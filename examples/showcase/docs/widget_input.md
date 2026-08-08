Single-line, password, and multiline text input with IME composition, keyboard
focus, masked input, and accessibility support.

## Usage

```go
gui.Input(gui.InputCfg{
    ID:          "name",
    Sizing:      gui.FillFit,
    Text:        app.Name,
    Placeholder: "Enter name...",
    OnTextChanged: func(s string, ctx gui.EventCtx) {
        gui.State[App](ctx.Window).Name = s
    },
})
```

## Password

```go
gui.Input(gui.InputCfg{
    ID:         "pw",
    IsPassword: true,
    Text:       app.Password,
    OnTextChanged: func(s string, ctx gui.EventCtx) {
        gui.State[App](ctx.Window).Password = s
    },
})
```

## Multiline

```go
gui.Input(gui.InputCfg{
    ID:     "notes",
    Mode:   gui.InputMultiline,
    Height: 90,
    Text:   app.Notes,
    OnTextChanged: func(s string, ctx gui.EventCtx) {
        gui.State[App](ctx.Window).Notes = s
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

Presets: MaskPhoneUS, MaskCreditCard16, MaskCreditCardAmex, MaskExpiryMMYY,
MaskCVC. For custom masks, set Mask (pattern string) and MaskTokens (custom
token definitions).

## Key Properties

| Property    | Type            | Description                        |
| ----------- | --------------- | ---------------------------------- |
| Text        | string          | Current text value                 |
| Placeholder | string          | Hint text shown when empty         |
| IsPassword  | bool            | Mask characters for password entry |
| Mode        | InputMode       | InputSingleLine or InputMultiline  |
| MaskPreset  | InputMaskPreset | Built-in mask (phone, card, etc.)  |
| Mask        | string          | Custom mask pattern                |
| MaskTokens  | []MaskTokenDef  | Custom token definitions for mask  |
| Disabled    | bool            | Disable interaction                |
| Height      | float32         | Height (useful for multiline)      |
| MinWidth    | float32         | Minimum width                      |
| MaxWidth    | float32         | Maximum width                      |

## Appearance

| Property         | Type         | Description               |
| ---------------- | ------------ | ------------------------- |
| Padding          | Opt[Padding] | Inner padding             |
| Radius           | Opt[float32] | Corner radius             |
| SizeBorder       | Opt[float32] | Border width              |
| Color            | Color        | Background color          |
| ColorHover       | Color        | Background on hover       |
| ColorBorder      | Color        | Border color              |
| ColorBorderFocus | Color        | Border color when focused |
| TextStyle        | TextStyle    | Text styling              |
| PlaceholderStyle | TextStyle    | Placeholder text styling  |

## Events

| Callback            | Signature                                     | Fired when                       |
| ------------------- | --------------------------------------------- | -------------------------------- |
| OnTextChanged       | func(string, EventCtx)                        | Text changes                     |
| OnTextCommit        | func(string, InputCommitReason, EventCtx)     | Enter pressed or focus lost      |
| OnEnter             | func(EventCtx)                                | Enter pressed (single-line)      |
| OnKeyDown           | func(EventCtx)                                | Unhandled key event              |
| OnBlur              | func(EventCtx)                                | Focus lost                       |
| PreTextChange       | func(current, proposed string) (string, bool) | Validate/transform before change |
| PostCommitNormalize | func(text string, InputCommitReason) string   | Normalize text on commit         |

## Accessibility

| Property        | Type   | Description            |
| --------------- | ------ | ---------------------- |
| A11YLabel       | string | Accessible label       |
| A11YDescription | string | Accessible description |
