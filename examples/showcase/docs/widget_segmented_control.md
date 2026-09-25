A row of segments in one shared track. The user selects one segment. Use it for
a compact choice of two to six short options, for example a view mode or a time
range.

The selected segment is a rounded accent fill (the pill) inside the track. The
API is the same as `RadioButtonGroupCfg`, so you can replace one control with
the other.

## Usage

```go
gui.SegmentedControl(gui.SegmentedControlCfg{
    ID:    "range",
    Value: app.Range,
    Options: []gui.SegmentOption{
        gui.NewSegmentOption("Day", "day"),
        gui.NewSegmentOption("Week", "week"),
        gui.NewSegmentOption("Month", "month"),
    },
    OnSelect: func(v string, ctx gui.EventCtx) {
        gui.State[App](ctx.Window).Range = v
        ctx.Consume()
    },
})
```

## Icons

A segment can show an icon, a label, or both. An icon-only segment uses its
`Value` as the accessible name. Give the control an `A11YLabel` to name the
group.

```go
gui.SegmentedControl(gui.SegmentedControlCfg{
    ID:      "view",
    Value:   app.View,
    A11YCfg: gui.A11YCfg{A11YLabel: "View mode"},
    Options: []gui.SegmentOption{
        {Icon: gui.IconListBullet, Value: "list"},
        {Icon: gui.IconTable, Value: "table"},
    },
    OnSelect: onView,
})
```

## Fill Width

Set `Sizing: gui.FillFit` to stretch the control to the width of its parent.
Each segment then gets the same width. A segment does not become smaller than
its content, so a long label can stay wider when the space is not sufficient.

## Stdlib Data Binding

Use `Items []string` when the label and the value are the same. When `Items` is
set, the control ignores `Options`.

## Keyboard and Focus

The control is one tab stop. The track gets the focus, and the segments do not.
A click on a segment moves the focus to the track.

| Key                     | Result                                       |
| ----------------------- | -------------------------------------------- |
| Left, Up / Right, Down  | Select the previous / next enabled segment   |
| Home / End              | Select the first / last enabled segment      |
| Space, Enter            | Send `OnSelect` again for the current value  |

The selection wraps at the ends. The keys skip disabled segments.

## Key Properties

| Property      | Type            | Description                              |
| ------------- | --------------- | ---------------------------------------- |
| ID            | string          | Required unless `FocusDisabled` is set   |
| Value         | string          | The selected value                       |
| Items         | []string        | Simple string list (alt. to Options)     |
| Options       | []SegmentOption | Segments: Label, Value, Icon, Disabled   |
| Sizing        | Sizing          | `FitFit` (default) or `FillFit`          |
| Disabled      | bool            | Disable the whole control                |
| FocusDisabled | bool            | Remove the control from the tab order    |

## Appearance

| Property      | Type         | Description                                  |
| ------------- | ------------ | -------------------------------------------- |
| TextStyle     | TextStyle    | Label style                                  |
| TextStyleIcon | TextStyle    | Icon style                                   |
| Padding       | Padding      | Text inset of one segment                    |
| SizeBorder    | Opt[float32] | Track border width                           |
| Radius        | Opt[float32] | Track corner radius                          |
| Colors        | ColorSet     | Track: Base, Border, BorderFocus             |
| ColorsSegment | ColorSet     | Segment: Base, Hover, Click, Selected (pill) |

## Events

| Callback | Signature              | Sent when                      |
| -------- | ---------------------- | ------------------------------ |
| OnSelect | func(string, EventCtx) | The user selects a segment     |

## Accessibility

The track has the radio-group role. Each segment has the radio-button role, and
the selected segment has the selected state.

| Property | Type    | Description                          |
| -------- | ------- | ------------------------------------ |
| A11YCfg  | A11YCfg | Embedded: A11YLabel, A11YDescription |

## Sound

| Property      | Type     | Description                                |
| ------------- | -------- | ------------------------------------------ |
| Sound         | SoundCue | Replaces the theme selection cue           |
| SoundDisabled | bool     | No sound, whatever the theme and `Sound`   |
