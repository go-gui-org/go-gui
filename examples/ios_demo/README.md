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

See `main.go` for the implementation.
