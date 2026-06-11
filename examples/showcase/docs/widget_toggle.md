Checkbox-style toggle with optional label. `Checkbox()` is
an alias for `Toggle()`.

## Usage

```go
gui.Toggle(gui.ToggleCfg{
    ID:       "accept",
    IDFocus:  300,
    Label:    "Accept terms",
    Selected: app.Accepted,
    OnClick: func(_ *gui.Layout, _ *gui.Event, w *gui.Window) {
        s := gui.State[App](w)
        s.Accepted = !s.Accepted
    },
})
```

## Custom Check Text

```go
gui.Toggle(gui.ToggleCfg{
    ID:           "star",
    TextSelect:   "★",
    TextUnselect: "☆",
    Selected:     app.Starred,
})
```

## Key Properties

| Property     | Type         | Description                          |
|--------------|--------------|--------------------------------------|
| Label        | string       | Label text beside the checkbox       |
| Selected     | bool         | Checked state                        |
| TextSelect   | string       | Text when selected (default "✓")     |
| TextUnselect | string       | Text when unselected                 |
| IDFocus      | uint32       | Tab-order focus ID (> 0 to enable)   |
| MinWidth     | float32      | Minimum width                        |
| Disabled     | bool         | Disable interaction                  |
| Invisible    | bool         | Hide without removing from layout    |

## Appearance

| Property         | Type         | Description                      |
|------------------|--------------|----------------------------------|
| Padding          | Opt[Padding] | Inner padding                    |
| Radius           | Opt[float32] | Corner radius                    |
| SizeBorder       | Opt[float32] | Border width                     |
| Color            | Color        | Background color                 |
| ColorHover       | Color        | Background on hover              |
| ColorFocus       | Color        | Background when focused          |
| ColorClick       | Color        | Background on click              |
| ColorBorder      | Color        | Border color                     |
| ColorBorderFocus | Color        | Border color when focused        |
| ColorSelect      | Color        | Background when selected         |
| TextStyle        | TextStyle    | Check mark text styling          |
| TextStyleLabel   | TextStyle    | Label text styling               |

## Events

| Callback | Signature                        | Fired when       |
|----------|----------------------------------|------------------|
| OnClick  | func(*Layout, *Event, *Window)   | Toggle clicked   |

## Accessibility

| Property        | Type   | Description                          |
|-----------------|--------|--------------------------------------|
| A11YLabel       | string | Accessible label                     |
| A11YDescription | string | Accessible description               |
