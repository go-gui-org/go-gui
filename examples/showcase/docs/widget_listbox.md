Scrollable list with single or multi-select, optional
subheadings, and drag-reorder support.

## Usage

```go
gui.ListBox(gui.ListBoxCfg{
    ID:          "lb",
    IDFocus:     700,
    Multiple:    true,
    Height:      200,
    SelectedIDs: app.Selected,
    Data: []gui.ListBoxOption{
        gui.NewListBoxSubheading("hdr", "Languages"),
        gui.NewListBoxOption("go", "Go", "go"),
        gui.NewListBoxOption("rs", "Rust", "rust"),
    },
    OnSelect: func(ids []string, _ *gui.Event, w *gui.Window) {
        gui.State[App](w).Selected = ids
    },
})
```

## Reorderable List

```go
gui.ListBox(gui.ListBoxCfg{
    ID:          "reorder",
    Reorderable: true,
    Data:        items,
    OnReorder: func(movedID, beforeID string, w *gui.Window) {
        reorderItems(movedID, beforeID, w)
    },
})
```

## Virtualization

Renders only visible rows when height is constrained. Requires
`Scrollable: true` and `Height` or `MaxHeight > 0`.
Scroll state is keyed by the widget's string ID.

```go
gui.ListBox(gui.ListBoxCfg{
    ID:         "virt-lb",
    Scrollable: true,
    MaxHeight:  200,
    Data:       items,
})
```

## Stdlib Data Binding

Use `Items []string` instead of `Data` for simple lists:

```go
gui.ListBox(gui.ListBoxCfg{
    ID:    "simple",
    Items: []string{"Go", "Rust", "Zig"},
    OnSelect: func(ids []string, _ *gui.Event, w *gui.Window) {
        gui.State[App](w).Selected = ids
    },
})
```

When `Items` is set, `Data` is ignored.

## Key Properties

| Property    | Type             | Description                          |
|-------------|------------------|--------------------------------------|
| SelectedIDs | []string         | Selected item IDs                    |
| Items       | []string         | Simple string list (alt. to Data)    |
| Data        | []ListBoxOption  | Items (ID, Name, Value, IsSubhead)   |
| Multiple    | bool             | Allow multi-select                   |
| Height      | float32          | Fixed height (enables virtualization)|
| MinWidth    | float32          | Minimum width                        |
| MaxWidth    | float32          | Maximum width                        |
| MinHeight   | float32          | Minimum height                       |
| MaxHeight   | float32          | Maximum height                       |
| Scrollable  | bool             | Opt into the scroll system (state keyed by ID)|
| IDFocus     | uint32           | Tab-order focus ID (> 0 to enable)   |
| Reorderable | bool             | Enable drag-reorder                  |
| Sizing      | Sizing           | Combined axis sizing mode            |
| Disabled    | bool             | Disable interaction                  |
| Invisible   | bool             | Hide without removing from layout    |

## Appearance

| Property        | Type         | Description                      |
|-----------------|--------------|----------------------------------|
| Padding         | Opt[Padding] | Inner padding                    |
| Radius          | Opt[float32] | Corner radius                    |
| SizeBorder      | Opt[float32] | Border width                     |
| Color           | Color        | Background color                 |
| ColorHover      | Color        | Item hover highlight             |
| ColorBorder     | Color        | Border color                     |
| ColorSelect     | Color        | Selected item highlight          |
| TextStyle       | TextStyle    | Item text styling                |
| SubheadingStyle | TextStyle    | Subheading text styling          |

## Events

| Callback  | Signature                                  | Fired when        |
|-----------|--------------------------------------------|-------------------|
| OnSelect  | func([]string, *Event, *Window)            | Selection changes  |
| OnReorder | func(movedID, beforeID string, *Window)    | Item reordered     |

## Accessibility

| Property        | Type   | Description                          |
|-----------------|--------|--------------------------------------|
| A11YLabel       | string | Accessible label                     |
| A11YDescription | string | Accessible description               |
