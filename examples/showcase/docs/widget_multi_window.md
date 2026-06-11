Manage multiple OS windows from a single application. An
App instance acts as a registry for all windows, enabling cross-window
communication via Broadcast and QueueCommand.

## Setup

```go
app := gui.NewApp()
app.ExitMode = gui.ExitOnMainClose

main := gui.NewWindow(gui.WindowCfg{
    State: &MainState{},
    Title: "Main",
    OnInit: func(w *gui.Window) {
        w.UpdateView(mainView)
    },
})
backend.RunApp(app, main)
```

## Opening a Window at Runtime

```go
w.App().OpenWindow(gui.WindowCfg{
    State:  &ChildState{},
    Title:  "Child",
    Width:  300,
    Height: 200,
    OnInit: func(child *gui.Window) {
        child.UpdateView(childView)
    },
})
```

## Cross-Window Communication

```go
// Send a message from child to parent.
parent.QueueCommand(func(p *gui.Window) {
    gui.State[ParentState](p).Message = "hello"
    p.UpdateWindow()
})

// Broadcast to all windows.
w.App().Broadcast(func(other *gui.Window) {
    other.QueueCommand(func(o *gui.Window) {
        // update other window state
        o.UpdateWindow()
    })
})
```

## Key Types

| Type       | Description                                     |
|------------|-------------------------------------------------|
| App        | Window registry with exit-mode policy           |
| ExitMode   | ExitOnLastClose (default) or ExitOnMainClose    |
| WindowCfg  | Configuration for window creation               |

## App Methods

| Method      | Signature                        | Description                          |
|-------------|----------------------------------|--------------------------------------|
| Register    | (id uint32, w *Window)           | Associate platform ID with window    |
| Unregister  | (id uint32) bool                 | Remove window; returns exit signal   |
| Window      | (id uint32) *Window              | Lookup by platform ID                |
| Windows     | () []*Window                     | Snapshot of all windows in order      |
| OpenWindow  | (cfg WindowCfg)                  | Queue window creation (next frame)   |
| Broadcast   | (fn func(*Window))               | Call fn for every registered window  |

## Window Methods

| Method       | Signature              | Description                          |
|--------------|------------------------|--------------------------------------|
| App          | () *App                | Parent app (nil in single-window)    |
| PlatformID   | () uint32              | OS window ID (0 if unregistered)     |
| Close        | ()                     | Request close on next frame          |
| QueueCommand | (fn func(*Window))     | Thread-safe deferred callback        |

## Notes

- OpenWindow buffers up to 16 pending requests per frame.
- QueueCommand is safe to call from any goroutine.
- Close is atomic and safe to call from any goroutine.
- Each window has its own typed state slot via State[T](w).
