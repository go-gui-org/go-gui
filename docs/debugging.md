# Debugging

A few widget mistakes are silent by construction, because they produce no error
and no visual difference:

- Two widgets sharing an `ID`. `ID` is the identity key for focus, scroll
  offsets, and per-widget state, so the two collapse onto one identity.
- A focusable widget with no `ID`. It renders and it clicks, but focus traversal
  is keyed by `ID`, so it never joins the tab order.
- A scrollable widget with no `ID`. Every ID-less scrollable in a window shares
  the key `""`, so they scroll in lockstep.
- An `OnMouseLeave` on a widget with no `ID`. Leave tracking is keyed by `ID`,
  so the callback never fires. This one survives `FocusDisabled: true` — opting
  out of focus does not opt out of needing an identity.

`gui.Debug(true)` — or `GOGUI_DEBUG=1` in the environment — audits every frame
for these and writes findings to stderr, once per finding per window:

```go
gui.Debug(true)
// gui: focusable shape at 0/2/1 has no ID; focus traversal is keyed by
// ID, so it renders and clicks but never joins the tab order
```

Leave it off in production: the checks walk the whole layout tree each frame and
allocate while doing it.

For the mistakes that are visible in the source, `requiredid` reports them at
build time instead, naming the `Cfg` type:

```fish
go run github.com/go-gui-org/go-gui/tools/requiredid/cmd/requiredid ./...
```

`go vet -vettool=` and a golangci-lint custom plugin work equally well. The tool
is offered, not required — it is an internal tool whose rules may tighten
between releases, so nothing breaks if you never run it. Without it, a widget
that needs an `ID` and has none panics on its first render rather than failing
your build.

## Widgets that require an `ID`

Every input control panics when constructed without a non-empty `Cfg.ID`:
`Button`, `Input`, `InputDate`, `NumericInput`, `RadioButtonGroup`, `Radio`,
`Select`, `Switch`, `Toggle`, plus the stateful widgets that already did
(`ColorPicker`, `Combobox`, `DatePicker`, `ListBox`, `Slider`, `Tree`, `Table`,
`Form`, `Menu`, `Menubar`, `ContextMenu`, `CommandPalette`, `ProgressBar`,
`DataGrid`).

The `ID` is what focus traversal, per-widget input state, scroll offsets, and
`OnMouseLeave` dispatch are all keyed by, so a control without one is not merely
anonymous — it is unreachable by keyboard and shares state with every other
ID-less control. IDs must be unique within a window.

A decorative control that should never take focus opts out instead of inventing
an ID:

```go
gui.Button(gui.ButtonCfg{FocusDisabled: true, Disabled: true})
```

`FocusDisabled: true` satisfies the requirement for the widgets above. It does
not exempt a widget whose `ID` is tagged `gui:"required"` without the `focus`
option — those key state by `ID` regardless of focus.
