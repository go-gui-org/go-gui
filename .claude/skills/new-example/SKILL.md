---
name: new-example
description: Scaffold a new go-gui example app with standard boilerplate
disable-model-invocation: true
---

# New Example App Scaffold

Create a new example app under `examples/<name>/main.go`.

## Arguments
- `name` (required): directory name for the example (lowercase, underscores)
- `description` (optional): one-line description for the package comment

## Template

Every example follows this structure:
1. Package comment that describes the example
2. `App` state struct
3. `main()` that sets the theme, creates `gui.NewWindow`, and calls
   `backend.Run`
4. `mainView` function that returns `gui.View`

## Reference Pattern

Use `examples/get_started/main.go` as the canonical template:

```go
package main

import (
    "github.com/go-gui-org/go-gui/gui"
    "github.com/go-gui-org/go-gui/gui/backend"
)

type App struct {
    // state fields
}

func main() {
    gui.SetTheme(gui.ThemeDark)

    w := gui.NewWindow(gui.WindowCfg{
        State:  &App{},
        Title:  "<name>",
        Width:  800,
        Height: 600,
        OnInit: func(w *gui.Window) {
            w.UpdateView(mainView)
        },
    })

    backend.Run(w)
}

func mainView(w *gui.Window) gui.View {
    ww, wh := w.WindowSize()
    app := gui.State[App](w)
    _ = app

    return gui.Column(gui.ContainerCfg{
        Width:  float32(ww),
        Height: float32(wh),
        Sizing: gui.FixedFixed,
        // build UI here
    })
}
```

## Rules
- Place in `examples/<name>/main.go`
- Use `gui.ThemeDark` unless the user specifies otherwise
- Use `gui.FixedFixed` sizing for the root container
- Follow all conventions in CLAUDE.md (no variable shadowing, clean lint)
