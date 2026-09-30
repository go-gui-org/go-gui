Joins form controls into one shape. The group draws one rounded border around
all segments and a thin divider between each pair. The segments have no border
of their own, so their edges are shared, and only the outer corners are round.
Use it for a field with a fixed prefix or suffix, such as `@`, `$` or `.00`, or
for a field with a button or a select attached.

A titled container is different. It draws a frame around separate controls, and
each control keeps its own border.

## Usage

```go
gui.InputGroup(gui.InputGroupCfg{
    ID: "price",
    Segments: []gui.InputGroupSegment{
        gui.InputGroupText(gui.InputGroupTextCfg{Text: "$"}),
        gui.InputGroupInput(gui.InputCfg{
            ID:            "amount",
            Text:          app.Amount,
            OnTextChanged: onAmount,
        }),
        gui.InputGroupText(gui.InputGroupTextCfg{Text: ".00"}),
    },
})
```

## Segments

Make each segment with a constructor:

| Constructor        | Segment                                   |
| ------------------ | ----------------------------------------- |
| `InputGroupText`   | A text addon on the theme background fill |
| `InputGroupInput`  | An `Input`, from an ordinary `InputCfg`   |
| `InputGroupSelect` | A `Select`, from an ordinary `SelectCfg`  |
| `InputGroupButton` | A `Button`, from an ordinary `ButtonCfg`  |

The constructors remove the segment's own border, radius and focus glow. You
cannot make a segment with a doubled border by mistake.

A `Label` on an input or a select does not show inside a group. It becomes the
accessible name of the segment. Put a visible label above the group.

## IDs and Focus

The group ID scopes the segment IDs. The input `amount` in the group `price` has
the effective ID `price:amount`. Use that ID with `SetFocus`.

The group is not a tab stop. Each input, select and button keeps its own place
in the tab order. When a segment has focus, the group shows the focus border and
the focus ring around the whole control.

## Fill Width

Set `Sizing: gui.FillFit` on the group, and `Sizing: gui.FillFit` on the segment
that gets the extra width, usually the input.
