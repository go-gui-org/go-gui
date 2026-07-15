Hierarchical expandable node display with virtualization,
lazy-loading, keyboard navigation, and drag-reorder support.

## Usage

```go
gui.Tree(gui.TreeCfg{
    ID:        "project-tree",
    IDFocus:    2001,
    Scrollable: true,
    MaxHeight: 240,
    OnSelect: func(nodeID string, _ *gui.Event, w *gui.Window) {
        gui.State[AppState](w).SelectedNode = nodeID
    },
    Nodes: []gui.TreeNodeCfg{
        {
            ID:   "src",
            Text: "src",
            Icon: gui.IconFolder,
            Nodes: []gui.TreeNodeCfg{
                {Text: "main.go"},
                {Text: "view_tree.go"},
            },
        },
    },
})
```

## Lazy Loading

```go
gui.Tree(gui.TreeCfg{
    ID: "lazy-tree",
    OnLazyLoad: func(treeID, nodeID string, w *gui.Window) {
        // Fetch children async, update state, call w.UpdateWindow()
    },
    Nodes: []gui.TreeNodeCfg{
        {ID: "remote", Text: "remote", Lazy: true},
    },
})
```

`TreeNodeCfg.ID` defaults to `Text` when omitted. Node IDs must be unique within a tree.

## Virtualization

Renders only visible nodes (flattened) when height is constrained.
Requires `Scrollable: true` and `Height` or `MaxHeight > 0`.
Scroll state is keyed by the widget's string ID.
The main usage example above shows the required configuration.

## Keyboard Navigation

`Up` / `Down` / `Left` (collapse) / `Right` (expand) / `Home` / `End` / `Enter` / `Space`

## Stdlib Data Binding

Use `ItemPaths []string` for flat path strings. Slash-separated
paths auto-expand into nested nodes:

```go
gui.Tree(gui.TreeCfg{
    ID:        "simple-tree",
    ItemPaths: []string{"src/main.go", "src/lib.go", "docs/readme.md"},
    OnSelect: func(nodeID string, _ *gui.Event, w *gui.Window) { ... },
})
```

When `ItemPaths` is set, `Nodes` is ignored.

## Key Properties

| Property    | Type            | Description                          |
|-------------|-----------------|--------------------------------------|
| ItemPaths   | []string        | Flat slash-separated paths (alt.)    |
| Nodes       | []TreeNodeCfg   | Root-level tree nodes                |
| Indent      | float32         | Indent per nesting level             |
| Spacing     | float32         | Vertical spacing between rows        |
| IDFocus     | uint32          | Tab-order focus ID (> 0 to enable)   |
| Scrollable  | bool            | Opt into the scroll system (state keyed by ID)|
| Reorderable | bool            | Enable drag-reorder of siblings      |
| Disabled    | bool            | Disable interaction                  |
| Invisible   | bool            | Hide without removing from layout    |
| Sizing      | Sizing          | Combined axis sizing mode            |
| Width       | float32         | Fixed width                          |
| Height      | float32         | Fixed height                         |
| MinWidth    | float32         | Minimum width                        |
| MaxWidth    | float32         | Maximum width                        |
| MinHeight   | float32         | Minimum height                       |
| MaxHeight   | float32         | Maximum height                       |

## TreeNodeCfg

| Property      | Type          | Description                          |
|---------------|---------------|--------------------------------------|
| ID            | string        | Node identifier (defaults to Text)   |
| Text          | string        | Display text                         |
| Icon          | string        | Icon string (e.g. IconFolder)        |
| Lazy          | bool          | Load children on expand              |
| Nodes         | []TreeNodeCfg | Child nodes                          |
| TextStyle     | TextStyle     | Text styling                         |
| TextStyleIcon | TextStyle     | Icon text styling                    |

## Appearance

| Property    | Type         | Description                          |
|-------------|--------------|--------------------------------------|
| Color       | Color        | Background color                     |
| ColorHover  | Color        | Hover background                     |
| ColorFocus  | Color        | Focused node background              |
| ColorBorder | Color        | Border color                         |
| Padding     | Opt[Padding] | Inner padding                        |
| SizeBorder  | Opt[float32] | Border width                         |
| Radius      | Opt[float32] | Corner radius                        |

## Events

| Callback   | Signature                                    | Fired when              |
|------------|----------------------------------------------|-------------------------|
| OnSelect   | func(string, *Event, *Window)                | Node selected           |
| OnLazyLoad | func(string, string, *Window)                | Lazy node expanded      |
| OnReorder  | func(movedID, beforeID string, w *Window)    | Node drag-reordered     |

## Accessibility

| Property        | Type   | Description                          |
|-----------------|--------|--------------------------------------|
| A11YLabel       | string | Accessible label                     |
| A11YDescription | string | Accessible description               |
