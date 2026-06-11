Modal dialog overlay with message, confirm, prompt, and custom
variants. Traps focus, dismisses on Escape, and supports Ctrl+C to
copy body text.

## Usage

```go
w.Dialog(gui.DialogCfg{
    Title:      "Confirm",
    Body:       "Delete this item?",
    DialogType: gui.DialogConfirm,
    OnOkYes: func(w *gui.Window) {
        // confirmed
    },
    OnCancelNo: func(w *gui.Window) {
        // cancelled
    },
})
```

## Prompt Dialog

```go
w.Dialog(gui.DialogCfg{
    Title:      "Rename",
    Body:       "Enter a new name:",
    DialogType: gui.DialogPrompt,
    Reply:      "Untitled",
    OnReply: func(text string, w *gui.Window) {
        gui.State[App](w).Name = text
    },
})
```

## API

| Method                   | Description                      |
|--------------------------|----------------------------------|
| w.Dialog(cfg)            | Show modal dialog                |
| w.DialogDismiss()        | Close current dialog             |
| w.DialogIsVisible() bool | Check if dialog is showing       |

## Dialog Types

| Type          | Buttons                          |
|---------------|----------------------------------|
| DialogMessage | OK                               |
| DialogConfirm | Yes / No                         |
| DialogPrompt  | Text input + OK / Cancel         |
| DialogCustom  | User-provided CustomContent      |

## Key Properties

| Property      | Type            | Description                      |
|---------------|-----------------|----------------------------------|
| Title         | string          | Dialog heading                   |
| Body          | string          | Message text                     |
| Reply         | string          | Pre-filled text (DialogPrompt)   |
| ID            | string          | Unique identifier                |
| DialogType    | DialogType      | Button configuration             |
| CustomContent | []View          | Custom views (DialogCustom)      |
| IDFocus       | uint32          | Initial focus target             |
| AlignButtons  | HorizontalAlign | Button alignment                 |
| Width         | float32         | Dialog width                     |
| Height        | float32         | Dialog height                    |
| MinWidth      | float32         | Minimum width                    |
| MinHeight     | float32         | Minimum height                   |
| MaxWidth      | float32         | Maximum width                    |
| MaxHeight     | float32         | Maximum height                   |

## Appearance

| Property       | Type         | Description                      |
|----------------|--------------|----------------------------------|
| Color          | Color        | Background color                 |
| ColorBorder    | Color        | Border color                     |
| Padding        | Opt[Padding] | Inner padding                    |
| SizeBorder     | Opt[float32] | Border width                     |
| Radius         | Opt[float32] | Corner radius                    |
| RadiusBorder   | Opt[float32] | Border corner radius             |
| TitleTextStyle | TextStyle    | Title text styling               |
| TextStyle      | TextStyle    | Body text styling                |

## Events

| Callback   | Signature                | Fired when                       |
|------------|--------------------------|----------------------------------|
| OnOkYes    | func(*Window)            | OK or Yes clicked                |
| OnCancelNo | func(*Window)            | Cancel, No, or Escape pressed    |
| OnReply    | func(string, *Window)    | Prompt submitted                 |
