---
name: widget
description: Create a new go-gui widget with proper Cfg struct and factory function
disable-model-invocation: true
---

# New Widget

Create a new widget in the `gui/` package following established conventions.

## Arguments
- `name` (required): widget name (e.g., "Slider", "ColorPicker")

## Widget Structure

Every widget consists of:
1. A `*Cfg` struct (zero-initializable, exported fields)
2. A factory function returning `View`
3. Event callbacks using `func(*Layout, *Event, *Window)` signature

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
    OnClick func(*Layout, *Event, *Window)
}

// <Name> creates a <Name> widget.
func <Name>(cfg <Name>Cfg) View {
    // Build layout tree
    // Wire event handlers (set e.IsHandled = true when consumed)
    // Return root View
}
```

## Rules
- File name: `view_<lowercase_name>.go` in `gui/`
- Cfg struct must be zero-initializable (sensible defaults)
- Event callbacks: `func(*Layout, *Event, *Window)`, set `e.IsHandled = true`
- Focus needs both `Focusable` (or default-on with no `FocusDisabled`)
  **and** a non-empty `ID`; the `requiredid` analyzer flags
  `Focusable: true` without an `ID`
- No variable shadowing (use `=` not `:=` for outer-scope vars)
- Read existing widgets (e.g., `view_button.go`, `view_slider.go`) for patterns
- Must pass `golangci-lint run ./gui/...`
