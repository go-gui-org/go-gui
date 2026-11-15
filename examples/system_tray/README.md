# System Tray

> **Framework:** system, platform **Description:** Tray icon with menu, re-show,
> and quit. Closing the window keeps the app alive via tray.

![Preview](screenshot.png)

<!-- explorer: tags=system,platform category=system run=go -->

---

## Run

```sh
go run ./examples/system_tray/
```

## What it demonstrates

Tray icon with menu, re-show, and quit. Closing the window keeps the app alive
via tray.

Closing hides the window with `w.Hide()`. On native Wayland a window cannot be
hidden yet, so `Hide` does nothing there and the window stays open; the tray
menu then reports it as already visible. Run with `GOGUI_WAYLAND=0` to see the
hide on a Wayland desktop.

See `main.go` for the implementation.
