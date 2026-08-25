---
name: widget
description: Create a new go-gui widget with proper Cfg struct and factory function
disable-model-invocation: true
---

# New Widget

Create a new widget in the `gui/` package. Follow established conventions.

## Arguments
- `name` (required): widget name (for example, "Slider" or "ColorPicker")

## Widget Structure

Every widget consists of:
1. A `*Cfg` struct (zero-initializable, exported fields)
2. A factory function that returns `View`
3. Event callbacks that use the `func(EventCtx)` signature

## Template

```go
package gui

// <Name>Cfg configures the <Name> widget.
type <Name>Cfg struct {
    // ID keys focus, scroll, and widget state. Focus requires a
    // non-empty ID — without one the widget is inert (never a tab
    // stop).
    ID string

    // Focusable opts into the focus system (with a non-empty ID).
    // NOTE: input controls (Input, Select, Slider, Toggle, Switch)
    // are focusable by default and expose FocusDisabled instead —
    // pick the convention that matches the widget class.
    Focusable bool

    // Widget-specific fields...

    // Event callbacks
    OnClick func(EventCtx)
}

// <Name> creates a <Name> widget.
func <Name>(cfg <Name>Cfg) View {
    // Build layout tree
    // Wire event handlers. A callback that acts on an event calls
    // ctx.Consume(). A callback that does not act lets it travel on.
    // Return root View
}
```

## Rules
- File name: `view_<lowercase_name>.go` in `gui/`
- Cfg struct must be zero-initializable (sensible defaults)
- Event callbacks use `func(EventCtx)`. A callback that acts on an event
  calls `ctx.Consume()`. A callback that does not act lets the event travel
  on. Nothing is consumed by default.
- Focus needs both `Focusable` (or default-on with no `FocusDisabled`)
  **and** a non-empty `ID`. The `requiredid` analyzer flags
  `Focusable: true` without an `ID`
- No variable shadowing (use `=` not `:=` for outer-scope vars)
- Read existing widgets (for example, `view_button.go` and `view_slider.go`)
  for patterns
- Must pass `golangci-lint run ./gui/...`
