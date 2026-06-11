Send native OS notifications via the platform notification
center. Runs asynchronously; result is delivered via callback on the
main thread.

## Usage

```go
w.NativeNotification(gui.NativeNotificationCfg{
    Title: "App",
    Body:  "Task completed!",
    OnDone: func(r gui.NativeNotificationResult, w *gui.Window) {
        if r.Status == gui.NotificationOK {
            // delivered successfully
        }
    },
})
```

## Key Properties

| Property | Type   | Description                              |
|----------|--------|------------------------------------------|
| Title    | string | Notification title (required)            |
| Body     | string | Notification body text                   |

## Events

| Callback | Signature                                    | Fired when               |
|----------|----------------------------------------------|--------------------------|
| OnDone   | func(NativeNotificationResult, *Window)      | Notification delivered   |

## NativeNotificationResult

| Field        | Type                     | Description              |
|--------------|--------------------------|--------------------------|
| Status       | NativeNotificationStatus | Outcome status           |
| ErrorCode    | string                   | Platform error code      |
| ErrorMessage | string                   | Human-readable error     |

## Result Status

| Status             | Meaning                              |
|--------------------|--------------------------------------|
| NotificationOK     | Delivered successfully               |
| NotificationDenied | Permission denied by OS              |
| NotificationError  | Platform error                       |
