# Family Tree

> **Framework:** layout **Description:** Scrollable family tree with clickable
> names and orthogonal connector lines. Demonstrates Canvas, DrawCanvas, and
> absolute child positions inside a scroll container.

![Preview](screenshot.png)

<!-- explorer: tags=layout,graphics category=layout run=go -->

---

## Run

```sh
go run ./examples/family_tree/
```

## What it demonstrates

A diagram larger than the window, built from three parts:

- `gui.Canvas` does not arrange its children. Each name is placed at its own
  `X`/`Y`.
- `gui.DrawCanvas` is the first child, so it paints the connector lines under
  the names.
- A scrollable `gui.Column` around the canvas scrolls on both axes.

The names are buttons, so click, hover, focus and accessibility work without
hit-testing code.

The canvas needs no explicit size. A canvas with Fit sizing grows to enclose its
children at their `X`/`Y`, so the scroll range covers the whole diagram.

To scroll sideways, swipe sideways on a trackpad, use Shift with the mouse
wheel, or drag the scrollbar.

See `main.go` for the implementation.
