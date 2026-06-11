Non-blocking notifications with severity levels, auto-dismiss,
and optional action buttons. Toasts animate in/out and pause dismissal
on hover.

## Usage

```go
w.Toast(gui.ToastCfg{
    Title:    "Saved",
    Body:     "Document saved.",
    Severity: gui.ToastSuccess,
})
```

## With Action Button

```go
w.Toast(gui.ToastCfg{
    Title:       "File deleted",
    Body:        "1 file moved to trash.",
    Severity:    gui.ToastWarning,
    ActionLabel: "Undo",
    OnAction: func(w *gui.Window) {
        // undo delete
    },
})
```

## API

| Method              | Description                      |
|---------------------|----------------------------------|
| w.Toast(cfg)        | Show toast, returns uint64 ID    |
| w.ToastDismiss(id)  | Dismiss specific toast           |
| w.ToastDismissAll() | Dismiss all toasts               |

## Key Properties

| Property    | Type          | Description                          |
|-------------|---------------|--------------------------------------|
| Title       | string        | Toast heading                        |
| Body        | string        | Toast message body                   |
| Severity    | ToastSeverity | Visual style (color accent)          |
| Duration    | time.Duration | Auto-dismiss delay (0 = 3s default)  |
| ActionLabel | string        | Optional action button text          |

## Events

| Callback | Signature      | Fired when                           |
|----------|----------------|--------------------------------------|
| OnAction | func(*Window)  | Action button clicked                |

## Severity

| Constant     | Use case                             |
|--------------|--------------------------------------|
| ToastInfo    | Informational                        |
| ToastSuccess | Positive outcome                     |
| ToastWarning | Needs attention                      |
| ToastError   | Critical failure                     |
