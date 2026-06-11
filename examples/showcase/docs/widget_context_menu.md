Right-click context menu that opens at the cursor position.
Wraps any content view and intercepts right-click to show a floating
menu. Supports submenus, separators, subtitles, and keyboard navigation.

## Usage

```go
gui.ContextMenu(w, gui.ContextMenuCfg{
    ID:     "ctx",
    Sizing: gui.FillFit,
    Items: []gui.MenuItemCfg{
        {ID: "cut", Text: "Cut"},
        {ID: "copy", Text: "Copy"},
        {ID: "paste", Text: "Paste"},
        gui.MenuSeparator(),
        {ID: "delete", Text: "Delete"},
    },
    Action: func(id string, e *gui.Event, w *gui.Window) {
        // handle selected item
        e.IsHandled = true
    },
    Content: []gui.View{
        gui.Text(gui.TextCfg{Text: "Right-click here"}),
    },
})
```

## With Submenus

```go
gui.ContextMenu(w, gui.ContextMenuCfg{
    ID:     "ctx-fmt",
    Sizing: gui.FillFit,
    Items: []gui.MenuItemCfg{
        gui.MenuSubtitle("Edit"),
        {ID: "cut", Text: "Cut"},
        {ID: "copy", Text: "Copy"},
        gui.MenuSeparator(),
        gui.MenuSubmenu("format", "Format", []gui.MenuItemCfg{
            {ID: "bold", Text: "Bold"},
            {ID: "italic", Text: "Italic"},
        }),
    },
    Action: func(id string, e *gui.Event, w *gui.Window) {
        e.IsHandled = true
    },
    Content: []gui.View{...},
})
```

## Key Properties

| Property    | Type            | Description                         |
|-------------|-----------------|-------------------------------------|
| ID          | string          | Unique identifier                   |
| Items       | []MenuItemCfg   | Menu items to display               |
| Content     | []View          | Child views wrapped by context menu |
| IDFocus     | uint32          | Focus ID (auto-generated if 0)      |
| FloatZIndex | int             | Z-index for float layering          |
| Sizing      | Sizing          | Container sizing mode               |
| Width       | float32         | Container width                     |
| Height      | float32         | Container height                    |
| HAlign      | HorizontalAlign | Horizontal content alignment        |
| VAlign      | VerticalAlign   | Vertical content alignment          |
| Padding     | Opt[Padding]    | Inner padding                       |

## Appearance

| Property          | Type         | Description                      |
|-------------------|--------------|----------------------------------|
| Color             | Color        | Menu background color            |
| ColorBorder       | Color        | Menu border color                |
| ColorSelect       | Color        | Highlighted item color           |
| SizeBorder        | Opt[float32] | Border width                     |
| Radius            | Opt[float32] | Menu corner radius               |
| RadiusMenuItem    | Opt[float32] | Item corner radius               |
| TextStyle         | TextStyle    | Menu item text styling           |
| TextStyleSubtitle | TextStyle    | Subtitle text styling            |
| PaddingMenuItem   | Opt[Padding] | Menu item padding                |
| PaddingSubmenu    | Opt[Padding] | Submenu padding                  |
| SpacingSubmenu    | Opt[float32] | Submenu item spacing             |
| WidthSubmenuMin   | Opt[float32] | Minimum submenu width            |
| WidthSubmenuMax   | Opt[float32] | Maximum submenu width            |

## Events

| Callback   | Signature                          | Fired when                |
|------------|------------------------------------|---------------------------|
| Action     | func(string, *Event, *Window)      | Menu item selected        |
| OnAnyClick | func(*Layout, *Event, *Window)     | Any click before menu     |

## Menu Item Helpers

| Helper                                  | Description                     |
|-----------------------------------------|---------------------------------|
| MenuItemText(id, text)                  | Standard menu item              |
| MenuSeparator()                         | Horizontal divider              |
| MenuSubtitle(text)                      | Non-interactive section heading |
| MenuSubmenu(id, text, []MenuItemCfg)    | Nested submenu                  |

## Keyboard Navigation

| Key         | Action                              |
|-------------|-------------------------------------|
| Escape      | Close menu                          |
| Up / Down   | Move selection                      |
| Enter/Space | Activate selected item              |
| Right       | Open submenu                        |
| Left        | Close submenu / return to parent    |
