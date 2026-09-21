Send native OS notifications through the platform notification center. The
notification runs asynchronously. The result arrives through the callback on the
main thread. `NotificationOK` means the platform accepted the request;
notification settings can still prevent it from appearing.

Windows 10 and later use native WinRT notifications with a stable application
identity. Notifications appear under the executable's name and remain in
Notification Center after the application exits. Clicking one opens that
executable with no additional arguments; it can start another instance if the
application is already running. Sending a notification does not launch
PowerShell or create a tray icon.

On first use, the backend registers a per-user `GoGui.App.<hash>` identity and a
`gogui-notification-<hash>` URL protocol bound to the executable. It creates no
Start menu shortcut. Failed registration can be retried by a later notification.
Registration persists to support notifications after exit. For built
applications, identity follows the executable path: moving it creates a
different identity; rebuilding at the same path preserves notification settings.
An uninstaller can remove the corresponding keys under
`HKCU\Software\Classes\AppUserModelId` and `HKCU\Software\Classes`.

For recognized `go run` temporary and cached executables, identity instead uses
the launch directory, Go main-package path and executable basename. Repeated
runs from that directory reuse one registration, updating activation to the
latest run's executable. Different working directories or main packages have
separate identities. File-list builds use Go's `command-line-arguments` package
name and the executable basename. A notification cannot reopen an executable
after Go deletes it; build to a stable path when post-exit activation is needed.

Windows versions older than 10 are unsupported, matching this module's Go
toolchain requirements.

## Usage

```go
w.NativeNotification(gui.NativeNotificationCfg{
    Title: "App",
    Body:  "Task completed!",
    OnDone: func(r gui.NativeNotificationResult, w *gui.Window) {
        if r.Status == gui.NotificationOK {
            // accepted by the platform
        }
    },
})
```

## Key Properties

| Property | Type   | Description                   |
| -------- | ------ | ----------------------------- |
| Title    | string | Notification title (required) |
| Body     | string | Notification body text        |

## Events

| Callback | Signature                               | Fired when                     |
| -------- | --------------------------------------- | ------------------------------ |
| OnDone   | func(NativeNotificationResult, *Window) | Notification request completed |

## NativeNotificationResult

| Field        | Type                     | Description          |
| ------------ | ------------------------ | -------------------- |
| Status       | NativeNotificationStatus | Outcome status       |
| ErrorCode    | string                   | Platform error code  |
| ErrorMessage | string                   | Human-readable error |

## Result Status

| Status             | Meaning                 |
| ------------------ | ----------------------- |
| NotificationOK     | Request accepted        |
| NotificationDenied | Permission denied by OS |
| NotificationError  | Platform error          |
