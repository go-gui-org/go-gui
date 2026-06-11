Horizontal menubar and standalone vertical menu with keyboard
navigation (arrow keys, Enter/Space to select, Escape to close),
nested submenus, separators, and subtitles. Two factory functions:
Menubar (horizontal bar) and Menu (vertical standalone).

## Menubar Usage

```go
gui.Menubar(w, gui.MenubarCfg{
    ID:      "mb",
    IDFocus: 100,
    Items: []gui.MenuItemCfg{
        gui.MenuSubmenu("file", "File", []gui.MenuItemCfg{
            gui.MenuItemText("new", "New"),
            gui.MenuItemText("open", "Open"),
            gui.MenuSeparator(),
            gui.MenuItemText("quit", "Quit"),
        }),
    },
    Action: func(id string, _ *gui.Event, w *gui.Window) {
        // handle menu action
    },
})
```

## Standalone Menu

```go
gui.Menu(w, gui.MenubarCfg{
    ID:    "ctx",
    Float: true,
    Items: []gui.MenuItemCfg{
        gui.MenuItemText("cut", "Cut"),
        gui.MenuItemText("copy", "Copy"),
        gui.MenuSubtitle("Advanced"),
        gui.MenuItemText("paste", "Paste Special"),
    },
})
```

## MenuItemCfg

| Property   | Type                                | Description                |
|------------|-------------------------------------|----------------------------|
| ID         | string                              | Action identifier          |
| Text       | string                              | Display label              |
| Submenu    | []MenuItemCfg                       | Nested submenu items       |
| CustomView | View                                | Custom rendered content    |
| Separator  | bool                                | Render as separator line   |
| Padding    | Opt[Padding]                        | Item padding override      |
| Action     | func(*MenuItemCfg, *Event, *Window) | Item-level action callback |

Helper constructors: MenuItemText, MenuSeparator, MenuSubtitle,
MenuSubmenu.

## MenubarCfg Key Properties

| Property         | Type          | Description                      |
|------------------|---------------|----------------------------------|
| ID               | string        | Unique identifier                |
| IDFocus          | uint32        | Tab-order focus ID               |
| Items            | []MenuItemCfg | Top-level menu items             |
| Sizing           | Sizing        | Combined axis sizing             |
| Disabled         | bool          | Disable interaction              |
| Invisible        | bool          | Hide without removing            |

## MenubarCfg Appearance

| Property          | Type         | Description                      |
|-------------------|--------------|----------------------------------|
| Color             | Color        | Background color                 |
| ColorBorder       | Color        | Border color                     |
| ColorSelect       | Color        | Selected item highlight          |
| TextStyle         | TextStyle    | Item text style                  |
| TextStyleSubtitle | TextStyle    | Subtitle text style              |
| Padding           | Opt[Padding] | Outer padding                    |
| PaddingMenuItem   | Opt[Padding] | Menu item padding                |
| PaddingSubmenu    | Opt[Padding] | Submenu panel padding            |
| PaddingSubtitle   | Opt[Padding] | Subtitle item padding            |
| SizeBorder        | Opt[float32] | Border width                     |
| Radius            | Opt[float32] | Outer corner radius              |
| RadiusBorder      | Opt[float32] | Border corner radius             |
| RadiusSubmenu     | Opt[float32] | Submenu panel corner radius      |
| RadiusMenuItem    | Opt[float32] | Menu item corner radius          |
| Spacing           | Opt[float32] | Top-level item spacing           |
| SpacingSubmenu    | Opt[float32] | Submenu item spacing             |
| WidthSubmenuMin   | Opt[float32] | Minimum submenu width            |
| WidthSubmenuMax   | Opt[float32] | Maximum submenu width            |

## MenubarCfg Floating

| Property      | Type        | Description                        |
|---------------|-------------|------------------------------------|
| Float         | bool        | Float above siblings               |
| FloatAutoFlip | bool        | Auto-flip when clipped             |
| FloatAnchor   | FloatAttach | Anchor attachment point            |
| FloatTieOff   | FloatAttach | Tie-off attachment point           |
| FloatOffsetX  | float32     | Horizontal float offset            |
| FloatOffsetY  | float32     | Vertical float offset              |
| FloatZIndex   | int         | Z-order for floated elements       |

## Events

| Callback                | Signature                          | Fired when               |
|-------------------------|------------------------------------|--------------------------|
| Action (on MenubarCfg)  | func(string, *Event, *Window)      | Any item selected        |
| Action (on MenuItemCfg) | func(*MenuItemCfg, *Event, *Window)| Specific item selected   |
