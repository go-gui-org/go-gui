Resizable two-pane split with a draggable divider handle.
Supports horizontal and vertical orientation, keyboard navigation,
pane collapse, and optional collapse buttons.

## Usage

```go
gui.Splitter(gui.SplitterCfg{
    ID:          "split",
    IDFocus:     100,
    Ratio:       gui.SomeF(0.3),
    Orientation: gui.SplitterHorizontal,
    First:  gui.SplitterPaneCfg{Content: []gui.View{left}},
    Second: gui.SplitterPaneCfg{Content: []gui.View{right}},
    OnChange: func(ratio float32, c gui.SplitterCollapsed,
        e *gui.Event, w *gui.Window) {
        s := gui.State[App](w)
        s.Ratio = ratio
        s.Collapsed = c
    },
})
```

## Collapsible Panes

```go
gui.Splitter(gui.SplitterCfg{
    ID:                  "split",
    ShowCollapseButtons: true,
    First: gui.SplitterPaneCfg{
        Collapsible:   true,
        CollapsedSize: 0,
        MinSize:       100,
        Content:       []gui.View{sidebar},
    },
    Second: gui.SplitterPaneCfg{Content: []gui.View{main}},
    OnChange: onChange,
})
```

## Key Properties

| Property            | Type                | Description                          |
|---------------------|---------------------|--------------------------------------|
| ID                  | string              | Unique identifier                    |
| IDFocus             | uint32              | Tab-order focus ID (> 0 to enable)   |
| Orientation         | SplitterOrientation | Horizontal or vertical split         |
| Sizing              | Sizing              | Combined axis sizing (default FillFill) |
| Ratio               | Opt[float32]        | Split position (0.0-1.0)            |
| Collapsed           | SplitterCollapsed   | Which pane is collapsed              |
| HandleSize          | Opt[float32]        | Drag handle thickness                |
| DragStep            | Opt[float32]        | Keyboard step size                   |
| DragStepLarge       | Opt[float32]        | Shift+arrow keyboard step            |
| ShowCollapseButtons | bool                | Show collapse/expand buttons         |
| Disabled            | bool                | Disable interaction                  |
| Invisible           | bool                | Hide without removing from layout    |

## SplitterPaneCfg (First / Second)

| Property      | Type    | Description                          |
|---------------|---------|--------------------------------------|
| MinSize       | float32 | Minimum pane size                    |
| MaxSize       | float32 | Maximum pane size                    |
| Collapsible   | bool    | Allow pane collapse                  |
| CollapsedSize | float32 | Size when collapsed                  |
| Content       | []View  | Pane content                         |

## Appearance

| Property          | Type    | Description                          |
|-------------------|---------|--------------------------------------|
| ColorHandle       | Color   | Handle background                    |
| ColorHandleHover  | Color   | Handle background on hover           |
| ColorHandleActive | Color   | Handle background while dragging     |
| ColorHandleBorder | Color   | Handle border color                  |
| ColorGrip         | Color   | Grip indicator color                 |
| ColorButton       | Color   | Collapse button background           |
| ColorButtonHover  | Color   | Collapse button hover                |
| ColorButtonActive | Color   | Collapse button active               |
| ColorButtonIcon   | Color   | Collapse button icon color           |
| SizeBorder        | Opt[float32] | Handle border width             |
| Radius            | Opt[float32] | Handle corner radius            |
| RadiusBorder      | Opt[float32] | Button/grip corner radius       |

## Events

| Callback | Signature                                              | Fired when                   |
|----------|--------------------------------------------------------|------------------------------|
| OnChange | func(float32, SplitterCollapsed, *Event, *Window)      | Ratio or collapse changes    |

## Accessibility

| Property        | Type   | Description                          |
|-----------------|--------|--------------------------------------|
| A11YLabel       | string | Accessible label                     |
| A11YDescription | string | Accessible description               |

## Serialization

SplitterState has JSON struct tags and text-marshaled enums,
so standard `encoding/json` works directly.

```go
// Save
state := gui.SplitterStateNormalize(gui.SplitterState{
    Ratio:     ratio,
    Collapsed: collapsed,
})
data, _ := json.Marshal(state)
// → {"ratio":0.3,"collapsed":"first"}

// Restore
var restored gui.SplitterState
json.Unmarshal(data, &restored)
```

Call SplitterStateNormalize before saving to clamp ratio to [0,1].
