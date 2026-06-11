IDE-style docking layout with splits, tabs, and drag-and-drop
panel rearrangement. The layout is a user-owned binary tree of
DockNode values: internal nodes are splits, leaves are panel groups.

## Usage

```go
gui.DockLayout(gui.DockLayoutCfg{
    ID:     "dock",
    Root:   app.Root,
    Panels: panels,
    OnLayoutChange: func(root *gui.DockNode, w *gui.Window) {
        gui.State[App](w).Root = root
    },
    OnPanelSelect: func(groupID, panelID string, w *gui.Window) {
        a := gui.State[App](w)
        a.Root = gui.DockTreeSelectPanel(a.Root, groupID, panelID)
    },
    OnPanelClose: func(panelID string, w *gui.Window) {
        a := gui.State[App](w)
        a.Root = gui.DockTreeRemovePanel(a.Root, panelID)
    },
})
```

## Building the Tree

```go
root := gui.DockSplit("root", gui.DockSplitHorizontal, 0.2,
    gui.DockPanelGroup("left", []string{"explorer"}, "explorer"),
    gui.DockSplit("right", gui.DockSplitVertical, 0.75,
        gui.DockPanelGroup("top", []string{"editor", "preview"}, "editor"),
        gui.DockPanelGroup("bottom", []string{"console"}, "console"),
    ),
)
```

## DockLayoutCfg

| Property          | Type                          | Description                            |
|-------------------|-------------------------------|----------------------------------------|
| ID                | string                        | Unique identifier                      |
| Root              | *DockNode                     | Layout tree root                       |
| Panels            | []DockPanelDef                | Panel definitions                      |
| OnLayoutChange    | func(*DockNode, *Window)      | Fired on layout change                 |
| OnPanelSelect     | func(string, string, *Window) | Fired on tab select (groupID, panelID) |
| OnPanelClose      | func(string, *Window)         | Fired on panel close (panelID)         |
| Sizing            | Sizing                        | Combined axis sizing                   |
| ColorZonePreview  | Color                         | Drop zone highlight color              |
| ColorTab          | Color                         | Tab background                         |
| ColorTabActive    | Color                         | Active tab background                  |
| ColorTabHover     | Color                         | Tab hover background                   |
| ColorTabBar       | Color                         | Tab bar background                     |
| ColorTabSeparator | Color                         | Tab separator color                    |
| ColorContent      | Color                         | Content area background                |

## DockPanelDef

| Property | Type   | Description                 |
|----------|--------|-----------------------------|
| ID       | string | Unique panel identifier     |
| Label    | string | Tab label text              |
| Content  | []View | Panel content views         |
| Closable | bool   | Allow user to close panel   |

## Tree Operations

| Function                  | Description                         |
|---------------------------|-------------------------------------|
| DockTreeRemovePanel       | Remove panel; collapse empty splits |
| DockTreeAddTab            | Add panel as tab in existing group  |
| DockTreeSplitAt           | Split a group, insert panel at edge |
| DockTreeMovePanel         | Remove + insert (drag-and-drop)     |
| DockTreeSelectPanel       | Set active tab in a group           |
| DockTreeWrapRoot          | Dock panel at window edge           |
| DockTreeCollectPanelNodes | Collect all panel group nodes       |
| DockTreeFindGroupByPanel  | Find group containing a panel       |
| DockTreeFindGroupByID     | Find group by its ID                |

## Serialization

DockNode has JSON struct tags and text-marshaled enums, so standard
encoding/json works directly.

```go
// Save
data, _ := json.Marshal(app.Root)
os.WriteFile("layout.json", data, 0644)

// Restore
root := new(gui.DockNode)
data, _ = os.ReadFile("layout.json")
json.Unmarshal(data, root)
```

Example output:

```json
{
  "kind": "split",
  "id": "root",
  "dir": "horizontal",
  "ratio": 0.2,
  "first": {
    "kind": "panelGroup",
    "id": "left",
    "panelIDs": ["explorer"],
    "selectedID": "explorer"
  },
  "second": { ... }
}
```

## Events

| Callback       | Signature                         | Fired when              |
|----------------|-----------------------------------|-------------------------|
| OnLayoutChange | func(*DockNode, *Window)          | Drag-drop rearrangement |
| OnPanelSelect  | func(string, string, *Window)     | Tab selected            |
| OnPanelClose   | func(string, *Window)             | Panel closed            |
