Trail navigation with clickable path segments and optional
content panels. Supports keyboard navigation (Left/Right/Home/End),
custom separators, and per-crumb content.

## Usage

```go
gui.Breadcrumb(gui.BreadcrumbCfg{
    ID:      "nav",
    IDFocus: 100,
    Items: []gui.BreadcrumbItemCfg{
        {ID: "home",     Label: "Home"},
        {ID: "settings", Label: "Settings"},
        {ID: "display",  Label: "Display"},
    },
    Selected: app.Page,
    OnSelect: func(id string, _ *gui.Event, w *gui.Window) {
        gui.State[App](w).Page = id
    },
})
```

## With Content Panels

```go
gui.Breadcrumb(gui.BreadcrumbCfg{
    Items: []gui.BreadcrumbItemCfg{
        {ID: "home", Label: "Home", Content: []gui.View{homeView}},
        {ID: "docs", Label: "Docs", Content: []gui.View{docsView}},
    },
    Selected: app.Page,
    OnSelect: onSelect,
})
```

## BreadcrumbItemCfg

| Property | Type   | Description                          |
|----------|--------|--------------------------------------|
| ID       | string | Segment identifier                   |
| Label    | string | Display text                         |
| Content  | []View | Panel content for this segment       |
| Disabled | bool   | Disable this segment                 |

## Key Properties

| Property          | Type               | Description                      |
|-------------------|--------------------|----------------------------------|
| ID                | string             | Unique identifier                |
| Items             | []BreadcrumbItemCfg | Path segments                   |
| Selected          | string             | Active segment ID                |
| Separator         | string             | Separator character              |
| IDFocus           | uint32             | Tab-order focus ID               |
| Sizing            | Sizing             | Combined axis sizing             |
| Disabled          | bool               | Disable interaction              |
| Invisible         | bool               | Hide without removing            |

## Appearance

| Property           | Type         | Description                      |
|--------------------|--------------|----------------------------------|
| Color              | Color        | Outer background                 |
| ColorBorder        | Color        | Outer border color               |
| ColorTrail         | Color        | Trail row background             |
| ColorCrumb         | Color        | Crumb background                 |
| ColorCrumbHover    | Color        | Crumb hover background           |
| ColorCrumbClick    | Color        | Crumb click background           |
| ColorCrumbSelected | Color        | Selected crumb background        |
| ColorCrumbDisabled | Color        | Disabled crumb background        |
| ColorContent       | Color        | Content panel background         |
| ColorContentBorder | Color        | Content panel border             |
| Padding            | Opt[Padding] | Outer padding                    |
| PaddingTrail       | Opt[Padding] | Trail row padding                |
| PaddingCrumb       | Opt[Padding] | Individual crumb padding         |
| PaddingContent     | Opt[Padding] | Content panel padding            |
| Radius             | Opt[float32] | Outer corner radius              |
| RadiusCrumb        | Opt[float32] | Crumb corner radius              |
| RadiusContent      | Opt[float32] | Content panel corner radius      |
| Spacing            | Opt[float32] | Outer spacing                    |
| SpacingTrail       | Opt[float32] | Trail item spacing               |
| SizeBorder         | Opt[float32] | Outer border width               |
| SizeContentBorder  | Opt[float32] | Content panel border width       |
| TextStyle          | TextStyle    | Default crumb text style         |
| TextStyleSelected  | TextStyle    | Selected crumb text style        |
| TextStyleDisabled  | TextStyle    | Disabled crumb text style        |
| TextStyleSeparator | TextStyle    | Separator text style             |

## Events

| Callback | Signature                          | Fired when               |
|----------|------------------------------------|--------------------------|
| OnSelect | func(string, *Event, *Window)      | Segment clicked          |

## Accessibility

| Property        | Type   | Description                          |
|-----------------|--------|--------------------------------------|
| A11YLabel       | string | Accessible label                     |
| A11YDescription | string | Accessible description               |
