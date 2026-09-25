Modal dialog overlay with message, confirm, prompt, and custom variants. Traps
focus, dismisses on Escape, and supports Ctrl+C to copy body text.

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

## Custom Dialog

`CustomView` runs on every frame while the dialog shows. Read state inside it,
so the dialog shows changes. `CustomContent` is deprecated: its views are built
once and do not change.

```go
w.Dialog(gui.DialogCfg{
    Title:      "Progress",
    DialogType: gui.DialogCustom,
    CustomView: func(w *gui.Window) gui.View {
        return gui.Text(gui.TextCfg{Text: gui.State[App](w).Status})
    },
})
```

## API

| Method                   | Description                |
| ------------------------ | -------------------------- |
| w.Dialog(cfg)            | Show modal dialog          |
| w.DialogDismiss()        | Close current dialog       |
| w.DialogIsVisible() bool | Check if dialog is showing |

## Dialog Types

| Type          | Buttons                  |
| ------------- | ------------------------ |
| DialogMessage | OK                       |
| DialogConfirm | Yes / No                 |
| DialogPrompt  | Text input + OK / Cancel |
| DialogCustom  | User-provided CustomView |

## Key Properties

| Property     | Type               | Description                                    |
| ------------ | ------------------ | ---------------------------------------------- |
| Title        | string             | Dialog heading                                 |
| Body         | string             | Message text                                   |
| Reply        | string             | Pre-filled text (DialogPrompt)                 |
| ID           | string             | Unique identifier                              |
| DialogType   | DialogType         | Button configuration                           |
| CustomView   | func(*Window) View | Custom body, rebuilt each frame (DialogCustom) |
| FocusID      | string             | Initial focus target                           |
| AlignButtons | HorizontalAlign    | Button alignment                               |
| Width        | float32            | Dialog width                                   |
| Height       | float32            | Dialog height                                  |
| MinWidth     | float32            | Minimum width                                  |
| MinHeight    | float32            | Minimum height                                 |
| MaxWidth     | float32            | Maximum width                                  |
| MaxHeight    | float32            | Maximum height                                 |

## Appearance

| Property       | Type         | Description          |
| -------------- | ------------ | -------------------- |
| Color          | Color        | Background color     |
| ColorBorder    | Color        | Border color         |
| Padding        | Opt[Padding] | Inner padding        |
| SizeBorder     | Opt[float32] | Border width         |
| Radius         | Opt[float32] | Corner radius        |
| RadiusBorder   | Opt[float32] | Border corner radius |
| TitleTextStyle | TextStyle    | Title text styling   |
| TextStyle      | TextStyle    | Body text styling    |

## Events

| Callback   | Signature             | Fired when                    |
| ---------- | --------------------- | ----------------------------- |
| OnOkYes    | func(*Window)         | OK or Yes clicked             |
| OnCancelNo | func(*Window)         | Cancel, No, or Escape pressed |
| OnReply    | func(string, *Window) | Prompt submitted              |
