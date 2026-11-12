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

In text that mixes directions, the arrow keys move the caret in stored order, so
inside a right-to-left run Left moves it to the right. Digits right after
right-to-left text are drawn as part of it, as the Unicode bidi rules say:
delete the line break after a line that ends in Arabic, and the next line's
number moves to the left of the Arabic.

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

| Property    | Type            | Description                           |
| ----------- | --------------- | ------------------------------------- |
| Text        | string          | Current text value                    |
| Placeholder | string          | Hint text shown when empty            |
| MaxLength   | int             | Rune budget; 0 means 1,048,576        |
| IsPassword  | bool            | Mask characters for password entry    |
| Keyboard    | KeyboardKind    | Soft-keyboard layout on touch devices |
| Mode        | InputMode       | InputSingleLine or InputMultiline     |
| MaskPreset  | InputMaskPreset | Built-in mask (phone, card, and more) |
| Mask        | string          | Custom mask pattern                   |
| MaskTokens  | []MaskTokenDef  | Custom token definitions for mask     |
| Disabled    | bool            | Disable interaction                   |
| Height      | float32         | Height (useful for multiline)         |
| MinWidth    | float32         | Minimum width                         |
| MaxWidth    | float32         | Maximum width                         |

## Appearance

| Property         | Type      | Description               |
| ---------------- | --------- | ------------------------- |
| Padding          | Padding   | Inner padding             |
| Radius           | Radius    | Corner radius             |
| SizeBorder       | Border    | Border width              |
| Color            | Color     | Background color          |
| ColorHover       | Color     | Background on hover       |
| ColorBorder      | Color     | Border color              |
| ColorBorderFocus | Color     | Border color when focused |
| TextStyle        | TextStyle | Text styling              |
| PlaceholderStyle | TextStyle | Placeholder text styling  |

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
| OnPasteRejected     | func(text string, EventCtx)                   | A paste is refused (whole text)  |

An edit that would take the text past `MaxLength` is refused whole, and a
refused paste reaches `OnPasteRejected`. Text the app sets past the budget shows
in full; the field then takes only edits that do not grow it. Large text costs
little: only the lines near the view stay laid out.

## Accessibility

| Property | Type    | Description                          |
| -------- | ------- | ------------------------------------ |
| A11YCfg  | A11YCfg | Embedded: A11YLabel, A11YDescription |

Set the pair through the embed: `A11YCfg: gui.A11YCfg{A11YLabel: "Save"}`.
