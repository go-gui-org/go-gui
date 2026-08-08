# Styling widgets

A widget that should keep one appearance through hover, press and focus used to
mean assigning six `Color*` fields. `ColorSet` groups them, and `Flat` covers
that case in a line:

```go
gui.Button(gui.ButtonCfg{
    ID:     "add-todo",
    Colors: gui.Flat(colorAccent),
})
```

`ColorSet` has six fields — `Base`, `Hover`, `Click`, `Focus`, `Border`,
`BorderFocus`. `Base` backs the three interactive states when they are unset, so
`ColorSet{Base: c}` gives a widget that does not react to the pointer while
keeping its themed border. `Flat(c)` additionally pins both borders, which is
what makes it visually inert rather than merely uniform.

Anything left unset falls through to the theme, and an unassigned `ColorSet`
changes nothing.

Two rules worth knowing:

- **An assigned flat `Color*` field wins over the `ColorSet`.** The direction is
  deliberate: code that sets flat fields today must keep its current appearance
  when a `ColorSet` arrives from a preset or a half-finished edit.
- **Colors are plain `Color`, not `Opt[Color]`.** `Color` already tracks whether
  it was set, so `gui.ColorTransparent` is a real choice and `Color{}` means
  "unspecified".

`ColorSet` is on the six widgets that carried the full set of state colors —
`Button`, `Switch`, `Toggle`, `Radio`, `InputDate`, `DatePicker`. On those, the
old `ColorHover` / `ColorFocus` / `ColorClick` / `ColorBorder` /
`ColorBorderFocus` fields are gone; `Color` stays. Widgets that only ever had
one or two of those fields keep them as they were.

`Color` sets the resting color and leaves hover, click and focus to the theme —
it does **not** pin them. Only `Flat(c)` does that, which is the point of having
it.
