Tabbed content panels with keyboard navigation
(Left/Right/Home/End). Controlled component: Selected is owned by app
state and updated through OnSelect. Supports drag-reorder.

## Usage

```go
gui.TabControl(gui.TabControlCfg{
    ID:       "tabs",
    IDFocus:  100,
    Selected: app.Tab,
    Items: []gui.TabItemCfg{
        {ID: "t1", Label: "General",  Content: []gui.View{general}},
        {ID: "t2", Label: "Advanced", Content: []gui.View{advanced}},
    },
    OnSelect: func(id string, _ *gui.Event, w *gui.Window) {
        gui.State[App](w).Tab = id
    },
})
```

## Reorderable Tabs

```go
gui.TabControl(gui.TabControlCfg{
    ID:          "tabs",
    Reorderable: true,
    Items:       items,
    Selected:    app.Tab,
    OnSelect:    onSelect,
    OnReorder: func(movedID, beforeID string, w *gui.Window) {
        // reorder items in app state
    },
})
```

## TabItemCfg

| Property | Type   | Description                          |
|----------|--------|--------------------------------------|
| ID       | string | Tab identifier                       |
| Label    | string | Tab header text                      |
| Content  | []View | Tab panel content                    |
| Disabled | bool   | Disable this tab                     |

## Key Properties

| Property    | Type          | Description                          |
|-------------|---------------|--------------------------------------|
| ID          | string        | Unique identifier                    |
| Items       | []TabItemCfg  | Tab definitions                      |
| Selected    | string        | Active tab ID                        |
| IDFocus     | uint32        | Tab-order focus ID (> 0 to enable)   |
| Sizing      | Sizing        | Combined axis sizing (default FillFill) |
| Reorderable | bool          | Enable drag-reorder of tabs          |
| Disabled    | bool          | Disable interaction                  |
| Invisible   | bool          | Hide without removing from layout    |
| Spacing       | Opt[float32] | Gap between header and content       |
| SpacingHeader | Opt[float32] | Gap between tab buttons              |

## Appearance

| Property           | Type         | Description                      |
|--------------------|--------------|----------------------------------|
| Color              | Color        | Outer background                 |
| ColorBorder        | Color        | Outer border color               |
| ColorHeader        | Color        | Header row background            |
| ColorHeaderBorder  | Color        | Header row border                |
| ColorContent       | Color        | Content panel background         |
| ColorContentBorder | Color        | Content panel border             |
| ColorTab           | Color        | Tab button background            |
| ColorTabHover      | Color        | Tab button hover                 |
| ColorTabFocus      | Color        | Tab button focus                 |
| ColorTabClick      | Color        | Tab button click                 |
| ColorTabSelected   | Color        | Selected tab background          |
| ColorTabDisabled   | Color        | Disabled tab background          |
| ColorTabBorder     | Color        | Tab button border                |
| ColorTabBorderFocus | Color       | Tab border when focused          |
| Padding            | Opt[Padding] | Outer padding                    |
| PaddingHeader      | Opt[Padding] | Header row padding               |
| PaddingContent     | Opt[Padding] | Content panel padding            |
| PaddingTab         | Opt[Padding] | Individual tab padding           |
| SizeBorder         | Opt[float32] | Outer border width               |
| SizeHeaderBorder   | Opt[float32] | Header border width              |
| SizeContentBorder  | Opt[float32] | Content border width             |
| SizeTabBorder      | Opt[float32] | Tab button border width          |
| Radius             | Opt[float32] | Outer corner radius              |
| RadiusHeader       | Opt[float32] | Header corner radius             |
| RadiusContent      | Opt[float32] | Content corner radius            |
| RadiusTab          | Opt[float32] | Tab button corner radius         |
| TextStyle          | TextStyle    | Default tab text style           |
| TextStyleSelected  | TextStyle    | Selected tab text style          |
| TextStyleDisabled  | TextStyle    | Disabled tab text style          |

## Events

| Callback  | Signature                                    | Fired when             |
|-----------|----------------------------------------------|------------------------|
| OnSelect  | func(string, *Event, *Window)                | Tab selection changes  |
| OnReorder | func(movedID, beforeID string, *Window)      | Tab reordered          |

## Accessibility

| Property        | Type   | Description                          |
|-----------------|--------|--------------------------------------|
| A11YLabel       | string | Accessible label                     |
| A11YDescription | string | Accessible description               |
