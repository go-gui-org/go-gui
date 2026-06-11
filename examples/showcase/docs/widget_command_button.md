Button wired to a registered command. Auto-fills label
and shortcut hint from the command, auto-disables when CanExecute returns
false, and routes clicks through the command registry.

## Usage

```go
// Register the command once (e.g. in OnInit).
w.RegisterCommand(gui.Command{
    ID:       "file.save",
    Label:    "Save",
    Icon:     gui.IconSave,
    Shortcut: gui.Shortcut{Key: gui.KeyS, Modifiers: gui.ModCtrl},
    Execute: func(_ *gui.Event, w *gui.Window) {
        // save logic
    },
})

// Use the button anywhere in a view function.
gui.CommandButton(w, "file.save", gui.ButtonCfg{ID: "btn-save"})
```

## Auto-Disabled via CanExecute

```go
w.RegisterCommand(gui.Command{
    ID:         "edit.delete",
    Label:      "Delete",
    CanExecute: func(w *gui.Window) bool { return hasSelection(w) },
    Execute:    func(_ *gui.Event, w *gui.Window) { deleteSelection(w) },
})

// Button greys out automatically when CanExecute returns false.
gui.CommandButton(w, "edit.delete", gui.ButtonCfg{ID: "btn-del"})
```

## Behavior

| Behavior          | Description                                        |
|-------------------|----------------------------------------------------|
| Auto-label        | Label + shortcut hint from Command if no Content   |
| Auto-disable      | Disabled when CanExecute returns false              |
| Click routing     | OnClick delegates to Command.Execute               |
| Custom content    | Supply ButtonCfg.Content to override auto-label    |
| Custom OnClick    | Supply ButtonCfg.OnClick to override command wiring |

## Key Properties

CommandButton accepts a standard ButtonCfg. The most relevant fields:

| Property    | Type         | Description                                |
|-------------|--------------|--------------------------------------------|
| ID          | string       | Unique identifier (required)               |
| Content     | []View       | Custom content (overrides auto-label)      |
| OnClick     | func(...)    | Custom handler (overrides command wiring)  |
| Disabled    | bool         | Force disable (also set by CanExecute)     |
| IDFocus     | uint32       | Tab-order focus ID (> 0 to enable)         |
| Sizing      | Sizing       | Combined axis sizing mode                  |

## Appearance

Inherits all ButtonCfg appearance properties (Color, Radius, Padding,
SizeBorder, etc.). See Button docs for the full list.

## Accessibility

| Property        | Type        | Description                            |
|-----------------|-------------|----------------------------------------|
| A11YRole        | AccessRole  | Accessible role override               |
| A11YState       | AccessState | Accessible state override              |
| A11YLabel       | string      | Accessible label                       |
| A11YDescription | string      | Accessible description                 |
