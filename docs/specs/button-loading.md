# Spec: busy state on a button

Issue #1264. Status: **implemented** (unreleased).

## Problem

A button that starts slow work (log in, save, upload) had no busy state. Apps
set `Disabled` and kept the label, so the user saw no progress. To show a
spinner, an app put an `SvgSpinner` in `Content` in place of the label. Then the
button width changed when the spinner appeared, and each app wrote its own
version. go-kite's Submit button is an example.

## Decision

One field, `ButtonCfg.Loading bool`:

```go
gui.Button(gui.ButtonCfg{
    ID:      "submit",
    Label:   "Submit",
    Loading: app.LoginPending,
    OnClick: startLogin,
})
```

While `Loading` is true:

- The button puts its content and a spinner in one `Stack`. Layer 0 is the
  content in a `Row` with the button's alignment. Layer 1 is the spinner, one em
  square. A `Stack` is as large as its largest layer, so the button keeps the
  size that its content gives it.
- `buttonAmendLayout` sets opacity 0 on every shape in layer 0. The shapes stay
  in the tree, so they keep their size, and the label text still names the
  button for assistive tech. A text shape at opacity 0 emits no draw command.
- The label is one level down, so the amend pass also runs the optical
  correction on layer 0. Without it, the label moves by the correction when
  `Loading` ends.
- The spinner takes the label color: `ColorTextOnAccent` on the filled variants,
  the theme text color on the others.
- `Button` drops `OnClick` and the click sound. No wrapper closure is built, so
  the busy state adds no allocation per frame. `shapeButtonColors.loading` tells
  the amend and hover passes that the button is still interactive. They keep the
  hover color, the focus color and the focus ring. The hover pass does not set
  the hand cursor, because a click does nothing.
- The button stays focusable. Its ID does not change, so focus does not move.
- The a11y state gets `AccessStateBusy`, added to the caller's `A11YState`. The
  spinner loses its `AccessRoleImage`. An unnamed image inside the button is
  read out as "image", and the busy state already says what the spinner shows.
- The spinner is an animated `Svg`. Its animation loop stops two seconds after
  the spinner leaves the tree (`view_svg.go`), so an idle button costs nothing.

### Known limits

- Content narrower than one em (a `"+"` label) lets the button grow to the
  spinner width.
- A focused loading button does not claim Space, because it has no `OnClick`.
  The key goes on to the next handler, as it does for a click-less button.

## Testing

- `gui/view_button_loading_test.go`: size kept, click and keys refused, focus
  kept, busy state set, colors and hover color kept, label hidden, spinner
  shown, label position kept, no image node.
- `gui/golden_button_loading_cases_test.go`: `button_loading` and
  `button_loading_focus` (Primary variant, focus ring) in both themes.
- `TestAutoIDEveryWidgetTwiceHasNoDuplicates` has a loading button.

## Rejected Approaches

- **No API: document `Content` with `SvgSpinner` plus `Disabled`.** This was the
  state before. It produces a width jump and no a11y busy state, and `Disabled`
  makes the button look unavailable.
- **`Busy bool` on every control (Button, Input, Select, and more).** No second
  widget needs it now. Add it to one widget at a time when a real case appears.
- **The spinner beside the label, in the button `Row`.** The spinner adds its
  width and a gap, which is the width jump that `Loading` removes.
- **The spinner as a `Float`.** A Float also overlays without a size change. But
  a Float is for content that must leave its container, such as a dropdown. A
  `Stack` layer clips and scrolls with the button.
