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

See `main.go` for the implementation.
