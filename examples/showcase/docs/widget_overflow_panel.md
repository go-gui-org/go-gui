Toolbar that hides items that don't fit and shows them in a
dropdown menu. Useful for responsive toolbars and action bars. Items
that overflow are shown via a trigger button that opens a dropdown
Menu.

## Usage

```go
gui.OverflowPanel(w, gui.OverflowPanelCfg{
    ID:      "toolbar",
    IDFocus: 100,
    Items: []gui.OverflowItem{
        {ID: "cut",   View: cutBtn,   Text: "Cut"},
        {ID: "copy",  View: copyBtn,  Text: "Copy"},
        {ID: "paste", View: pasteBtn, Text: "Paste"},
    },
})
```

## Custom Trigger Button

```go
gui.OverflowPanel(w, gui.OverflowPanelCfg{
    ID:    "tb",
    Items: items,
    Trigger: []gui.View{
        gui.Text(gui.TextCfg{Text: "More..."}),
    },
})
```

## OverflowItem

| Property | Type                                  | Description                |
|----------|---------------------------------------|----------------------------|
| ID       | string                                | Item identifier            |
| View     | View                                  | Visible toolbar view       |
| Text     | string                                | Label in overflow dropdown |
| Action   | func(*MenuItemCfg, *Event, *Window)   | Overflow item action       |

## Key Properties

| Property     | Type         | Description                          |
|--------------|--------------|--------------------------------------|
| ID           | string       | Unique identifier                    |
| Items        | []OverflowItem | Ordered toolbar items              |
| Trigger      | []View       | Custom overflow button content       |
| Padding      | Opt[Padding] | Inner padding                        |
| Spacing      | float32      | Gap between items                    |
| IDFocus      | uint32       | Tab-order focus ID (> 0 to enable)   |
| Disabled     | bool         | Disable interaction                  |
| FloatAnchor  | FloatAttach  | Dropdown anchor point                |
| FloatTieOff  | FloatAttach  | Dropdown tie-off point               |
| FloatOffsetX | float32      | Dropdown horizontal offset           |
| FloatOffsetY | float32      | Dropdown vertical offset             |
| FloatZIndex  | int          | Dropdown z-order                     |
