# Commands and Hotkeys

The command system provides a centralized way to define actions with
keyboard shortcuts, menu integration, command palette access, and
automatic enable/disable logic. Commands are registered per-window and
dispatched through the keyboard event pipeline.

## Core Types

### Shortcut

A key + modifier combination.

```go
type Shortcut struct {
    Key       KeyCode
    Modifiers Modifier
}
```

| Method     | Description                                                  |
| ---------- | ------------------------------------------------------------ |
| `IsSet()`  | Returns true if a key is assigned (not KeyInvalid)           |
| `String()` | Human-readable label: macOS glyphs (`⌘S`) or text (`Ctrl+S`) |

### Modifier

Bitmask flags for keyboard modifiers. Combine with `|`.

| Constant   | Value |
| ---------- | ----- |
| `ModNone`  | 0     |
| `ModShift` | 1     |
| `ModCtrl`  | 2     |
| `ModAlt`   | 4     |
| `ModSuper` | 8     |

Pre-combined convenience constants:

| Constant          | Combination                     |
| ----------------- | ------------------------------- |
| `ModCtrlShift`    | `ModCtrl \| ModShift`           |
| `ModCtrlAlt`      | `ModCtrl \| ModAlt`             |
| `ModCtrlAltShift` | `ModCtrl \| ModAlt \| ModShift` |
| `ModCtrlSuper`    | `ModCtrl \| ModSuper`           |
| `ModAltShift`     | `ModAlt \| ModShift`            |
| `ModAltSuper`     | `ModAlt \| ModSuper`            |
| `ModSuperShift`   | `ModSuper \| ModShift`          |

`Modifier.Has(mod)` tests if a flag is set; `HasAny(mods...)` tests
any of several.

### Command

```go
type Command struct {
    ID         string                  // unique identifier
    Label      string                  // display name (menus, palette, buttons)
    Icon       string                  // optional icon reference
    Group      string                  // grouping for palette organization
    Shortcut   Shortcut                // keyboard binding
    Execute    func(*Event, *Window)   // action callback
    CanExecute func(*Window) bool      // nil = always enabled
    Global     bool                    // dispatch priority (see below)
}
```

**`Global` field** controls when the command fires in the keyboard
dispatch pipeline:

- `Global: true` — fires _before_ focus dispatch. Use for app-wide
  shortcuts (save, new, command palette toggle) that should work
  regardless of which widget has focus.
- `Global: false` (default) — fires _after_ focus dispatch, as a
  fallback. Use for commands that should yield to focused widgets
  (e.g., an undo command that should not fire while a text input
  has focus and handles its own undo).

**`CanExecute`** — when non-nil, the command is skipped during dispatch
if it returns false. Widgets bound to the command (buttons, menu items)
auto-disable.

## Registration

Commands are registered on a `*Window` and stored in its registry.

```go
w.RegisterCommand(gui.Command{
    ID:       "file.save",
    Label:    "Save",
    Group:    "File",
    Shortcut: gui.Shortcut{Key: gui.KeyS, Modifiers: gui.ModSuper},
    Global:   true,
    Execute: func(_ *gui.Event, w *gui.Window) {
        // save logic
    },
})
```

**Batch registration:**

```go
w.RegisterCommands(cmd1, cmd2, cmd3)
```

**Constraints:**

- Duplicate command ID → panic
- Duplicate shortcut (same key + modifiers on two commands) → panic

### Window Methods

| Method                                       | Description                                |
| -------------------------------------------- | ------------------------------------------ |
| `RegisterCommand(cmd)`                       | Register one command; panics on duplicates |
| `RegisterCommands(cmds...)`                  | Register multiple commands                 |
| `UnregisterCommand(id)`                      | Remove by ID; no-op if not found           |
| `CommandByID(id) (Command, bool)`            | Lookup by ID                               |
| `CommandCanExecute(id) bool`                 | Check CanExecute; false if not found       |
| `CommandPaletteItems() []CommandPaletteItem` | Export commands with Labels for palette    |

## Keyboard Dispatch Pipeline

When a `KeyDown` event arrives, the window processes it in this order:

```
1. Global command dispatch     (Global=true commands)
2. Focus dispatch              (focused widget's OnKeyDown)
3. Tab navigation              (Tab / Shift+Tab for focus cycling)
4. Non-global command dispatch  (Global=false commands as fallback)
```

At each stage, if a handler sets `e.IsHandled = true`, subsequent
stages are skipped. This means:

- Global commands always take priority.
- Focused widgets can intercept keys before non-global commands.
- Non-global commands act as a catch-all for unhandled keys.

## Widget Integration

### CommandButton

Creates a button automatically wired to a registered command.

```go
gui.CommandButton(w, "edit.undo", gui.ButtonCfg{IDFocus: 10})
```

**Auto behaviors:**

- **Label** — if `cfg.Content` is nil, fills from `Command.Label` +
  shortcut hint (e.g., `"Undo  ⌘Z"`)
- **OnClick** — wired to `Command.Execute` (re-checks `CanExecute`
  at click time)
- **Disabled** — auto-disables when `CanExecute` returns false

Panics if the command ID is not registered.

### Menu Items

Set `CommandID` on `MenuItemCfg` to auto-resolve from the registry:

```go
gui.MenuItemCfg{
    ID:        "edit.undo",
    CommandID: "edit.undo",
}
```

**Auto behaviors:**

- **Text** — fills from `Command.Label` if `Text` is empty
- **Shortcut hint** — displays `Command.Shortcut.String()` right-aligned
- **Disabled** — auto-disables when `CanExecute` returns false
- **Action** — wired to `Command.Execute` if `Action` is nil

## Command Palette

A searchable, filterable overlay that lists all labeled commands.

### Setup

Include the palette view in the view tree (hidden by default):

```go
gui.CommandPalette(gui.CommandPaletteCfg{
    ID:         "palette",
    Scrollable: true,
    Items:      w.CommandPaletteItems(),
    OnAction:   paletteAction,
    OnDismiss:  func(_ *gui.Window) {},
})
```

Register a command to toggle it:

```go
gui.Command{
    ID:       "view.palette",
    Label:    "Command Palette",
    Shortcut: gui.Shortcut{Key: gui.KeyP, Modifiers: gui.ModSuperShift},
    Global:   true,
    Execute: func(_ *gui.Event, w *gui.Window) {
        gui.CommandPaletteToggle("palette", w)
    },
}
```

Handle palette selection:

```go
func paletteAction(id string, e *gui.Event, w *gui.Window) {
    cmd, ok := w.CommandByID(id)
    if ok && cmd.Execute != nil {
        cmd.Execute(e, w)
    }
}
```

### Visibility Functions

| Function                                         | Description          |
| ------------------------------------------------ | -------------------- |
| `CommandPaletteShow(id, w)`               | Show and focus input |
| `CommandPaletteDismiss(id, w)`                   | Hide and reset query |
| `CommandPaletteToggle(id, w)`             | Toggle visibility    |
| `CommandPaletteIsVisible(id, w) bool`            | Check if showing     |

### Palette Keyboard Navigation

| Key     | Action                                  |
| ------- | --------------------------------------- |
| Up/Down | Move highlight through filtered items   |
| Enter   | Execute highlighted command and dismiss |
| Escape  | Dismiss palette                         |

Typing in the input field filters items by fuzzy match. The highlight
resets to the first item on each keystroke.

### CommandPaletteCfg Fields

| Field            | Type                            | Description                   |
| ---------------- | ------------------------------- | ----------------------------- |
| `ID`             | `string`                        | Palette instance ID           |
| `Items`          | `[]CommandPaletteItem`          | Items to display              |
| `OnAction`       | `func(string, *Event, *Window)` | Called with selected item ID  |
| `OnDismiss`      | `func(*Window)`                 | Called when palette closes    |
| `Placeholder`    | `string`                        | Input placeholder text        |
| `TextStyle`      | `TextStyle`                     | Item label style              |
| `DetailStyle`    | `TextStyle`                     | Shortcut hint style           |
| `Color`          | `Color`                         | Panel background              |
| `ColorBorder`    | `Color`                         | Panel border                  |
| `ColorHighlight` | `Color`                         | Highlight/hover color         |
| `SizeBorder`     | `Opt[float32]`                  | Border thickness              |
| `Radius`         | `Opt[float32]`                  | Corner radius                 |
| `Width`          | `float32`                       | Panel width (default 500)     |
| `MaxHeight`      | `float32`                       | Max list height (default 400) |
| `BackdropColor`  | `Color`                         | Semi-transparent overlay      |
| `IDFocus`        | `uint32`                        | Focus ID for the input        |
| `Scrollable`     | `bool`                          | Opt results list into scroll (key `ID+":scroll"`) |
| `FloatZIndex`    | `int`                           | Z-order (default 1000)        |

### CommandPaletteItem

| Field      | Type     | Description                   |
| ---------- | -------- | ----------------------------- |
| `ID`       | `string` | Matched to command ID         |
| `Label`    | `string` | Display text                  |
| `Detail`   | `string` | Right-aligned hint (shortcut) |
| `Icon`     | `string` | Optional icon reference       |
| `Group`    | `string` | Group header                  |
| `Disabled` | `bool`   | Grayed out, not selectable    |

## Complete Example

See `examples/command_demo/` for a working example that demonstrates:

- Global commands (`⌘N`, `⌘S`) that fire regardless of focus
- Non-global commands (`⌘Z`, `⌘=`, `⌘-`) with `CanExecute` guards
- `CommandButton` with auto-label and auto-disable
- Menubar with `CommandID` auto-resolution
- Command palette toggled via `⌘⇧P`

```go
func registerCommands(w *gui.Window) {
    w.RegisterCommands(
        gui.Command{
            ID:       "file.new",
            Label:    "New",
            Group:    "File",
            Shortcut: gui.Shortcut{Key: gui.KeyN, Modifiers: gui.ModSuper},
            Global:   true,
            Execute: func(_ *gui.Event, w *gui.Window) {
                app := gui.State[App](w)
                app.Counter = 0
                app.Log = "New document created."
            },
        },
        gui.Command{
            ID:       "edit.undo",
            Label:    "Undo",
            Group:    "Edit",
            Shortcut: gui.Shortcut{Key: gui.KeyZ, Modifiers: gui.ModSuper},
            Execute: func(_ *gui.Event, w *gui.Window) {
                app := gui.State[App](w)
                if app.Counter > 0 {
                    app.Counter--
                }
            },
            CanExecute: func(w *gui.Window) bool {
                return gui.State[App](w).Counter > 0
            },
        },
        gui.Command{
            ID:       "view.palette",
            Label:    "Command Palette",
            Group:    "View",
            Shortcut: gui.Shortcut{Key: gui.KeyP, Modifiers: gui.ModSuperShift},
            Global:   true,
            Execute: func(_ *gui.Event, w *gui.Window) {
                gui.CommandPaletteToggle("palette", w)
            },
        },
    )
}
```

## Key Code Reference

The `keyName()` function provides display names for all supported keys:

| Category    | Keys                                                                                 |
| ----------- | ------------------------------------------------------------------------------------ |
| Letters     | `KeyA` – `KeyZ` → `"A"` – `"Z"`                                                      |
| Numbers     | `Key0` – `Key9` → `"0"` – `"9"`                                                      |
| Function    | `KeyF1` – `KeyF25` → `"F1"` – `"F25"`                                                |
| Keypad      | `KeyKP0` – `KeyKP9` → `"KP0"` – `"KP9"`                                              |
| Navigation  | `KeyUp`, `KeyDown`, `KeyLeft`, `KeyRight`                                            |
| Page        | `KeyHome`, `KeyEnd`, `KeyPageUp`, `KeyPageDown`                                      |
| Editing     | `KeyBackspace`, `KeyDelete`, `KeyInsert`, `KeyTab`                                   |
| Special     | `KeySpace`, `KeyEnter`, `KeyEscape`                                                  |
| Punctuation | `KeyMinus`, `KeyEqual`, `KeyComma`, `KeyPeriod`, `KeySlash`, `KeyBackslash`,`KeyLeftBracket`, `KeyRightBracket`, `KeyApostrophe`, `KeySemicolon`,`KeyGraveAccen` |
