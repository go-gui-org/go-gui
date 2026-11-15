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

Closing hides the window with `w.Hide()`, and "Show Window" in the tray menu
brings it back. "Quit" in the tray menu ends the app: it removes the tray and
closes every window.

Where hiding to the tray cannot work, closing the window closes it and the app
ends:

- **No tray.** A desktop may have none. Stock GNOME shows tray icons only with
  the "AppIndicator and KStatusNotifierItem Support" extension. The status line
  then reads "Running without tray".
- **Native Wayland.** A window cannot be hidden there yet. Run with
  `GOGUI_WAYLAND=0` to see the hide on a Wayland desktop.

See `main.go` for the implementation.
