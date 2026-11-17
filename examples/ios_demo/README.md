# iOS Demo

> **Framework:** platform **Description:** Go-gui on iOS via c-archive and
> Xcode. Demonstrates platform integration and lifecycle.

![Preview](screenshot.png)

<!-- explorer: tags=platform category=platform run=mobile -->

---

## Run

```sh
go run ./examples/ios_demo/
```

## What it demonstrates

Go-gui on iOS via c-archive and Xcode. Demonstrates platform integration and
lifecycle.

The view pads by `w.SafeAreaPadding()` and `w.SoftKeyboardInset()`, so its
content stays clear of the notch, the home bar and the soft keyboard. Go-Gui
reports both but moves no layout for them: an app opts in like this.

The Fullscreen button calls `w.SetFullscreen`, which hides the status bar and
lets the home indicator fade out. The view keeps its size. The button toggles
the app's own request rather than `IsFullscreen`: a windowed iPad cannot fill
the screen, so `IsFullscreen` stays false there until the window does.

The "Keep the screen on" switch holds a `w.KeepAwake` request, which turns off
the idle timer that locks the screen. It is on only while the app is in the
foreground.

The line under the heading counts how often the app left the screen and how long
the last absence was. It comes from `EventVisibilityChanged` and
`w.IsOccluded()`: a player would stop decoding and pause its audio there.
Control Center and the notification pull-down do not count, because the app
stays on screen behind them.

See `main.go` for the implementation.
