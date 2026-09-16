# Spec: build-time hover and press state (`IsHovered` / `IsPressed`)

Issue: #587

Status: **implemented** on branch `build-time-hover-state`, not yet released.

## Motivation

A look that depends on hover or press could only change `Shape.Color`, after
layout, from `OnHover` or `AmendLayout` (`gui/view_button.go`). A look that
changes structure or geometry could not be built: a Windows 98 button that moves
its label 1px on press, an XP button whose gradient flips, an outline button
whose text color changes on hover.

go-shirei builds these looks from state read while the view is built, using the
previous frame's geometry. go-gui already keeps everything a query needs: the
last arranged tree, the pointer position, the held button, and effective IDs.

## API

```go
func (w *Window) IsHovered(effectiveID string) bool
func (w *Window) IsPressed(effectiveID string) bool
```

Both take the effective ID, like `IsFocus`. Inside `GenerateLayout` pass
`w.EffID(cfg.ID)`. Both are main-thread only and allocate nothing.
`examples/custom_buttons` is the reference consumer.

## Semantics

1. **Hover target.** At the end of `layoutArrange`, after the `OnHover` and
   `OnMouseLeave` passes, `recordHoverTarget` walks the layers topmost first.
   The first layer with any shape under the pointer decides. Inside it the
   deepest hit shape is found, children topmost first, with the same inverse
   rotation as `layoutHoverDepth`. The target is the nearest enabled ID-bearing
   shape on that path, or none.
2. **Covering shapes block.** A float with no ID and no handlers still decides
   its layer, so the shape under it is not hovered.
3. **Dialog rule.** While a dialog is visible only the dialog layer is tried,
   the same rule event dispatch uses.
4. **Within.** `IsHovered(id)` is true when the target equals `id` or starts
   with `id + ":"`. An ID-bearing ancestor of the target is hovered too.
5. **Disabled** shapes are never a target; their enabled ancestor can be.
6. **Mouse lock.** While the mouse is locked the target is frozen, not cleared,
   so a drag keeps its look.
7. **No pointer yet.** Nothing is hovered until the first mouse move. The
   pointer starts at (0,0), which would otherwise hover the top-left widget.
8. **Press target.** A left press records the target under the event point, from
   backend presses (`handleMouseDownEvent`) and touch-synthesized presses
   (`synthMouse`) alike. A press during a mouse lock records nothing. It clears
   on release (either path), on `MouseCancel`, and on `SetView`.
9. **Press persists off-shape** until release. "Pressed and still over it" is
   `IsPressed(id) && IsHovered(id)`.
10. **Same-frame update.** Generation reads the target recorded by the previous
    arrange. When `recordHoverTarget` stores a different target it calls
    `InvalidateLayout`. `FrameFn` already runs a second `Update` in the same
    frame while `refreshLayout` is set, so the new look is on screen in the
    frame the pointer moved. Cost: one extra generation per hover change.
11. **A held touch hovers; lift clears it.** A touch press or drag records its
    point in `pointerX/Y`, which the hover record reads, so
    `IsPressed(id) && IsHovered(id)` holds under a finger. `mousePosX/Y` is not
    moved, so `OnHover` dispatch is unchanged. When the last touch ends or is
    cancelled, `pointerLifted` clears the target.
12. **Window exit clears hover.** `EventMouseLeave` is handled in `EventFn`:
    `pointerLeftWindow` clears the target and moves the pointer position
    off-window, so `OnHover` stops firing and `OnMouseLeave` fires on the next
    arrange. The event is allowed in an unfocused window. Backends that emit it:
    - Metal: an `NSTrackingArea` on the content view posts
      `NSEventTypeMouseExited`, mapped to `METAL_EVENT_MOUSE_LEAVE`.
    - X11: `EventMaskLeaveWindow`; `LeaveNotify` with mode Normal and a detail
      other than Inferior.
    - Win32: `TrackMouseEvent(TME_LEAVE)` armed on mouse move; `WM_MOUSELEAVE`.
    - Web already emitted it. iOS and Android emit no mouse moves; touch lift
      covers them.

## Rule for looks

Change only what is inside the widget's bounds: inner padding, colors,
gradients, children. A look that moves or resizes the hovered shape itself can
pull it out from under a still pointer: hovered, shrinks, not hovered, grows,
hovered. The frame loop then rebuilds every frame and the look flickers. No
guard exists, because detecting it needs geometry history for every ID. The rule
is stated in the `IsHovered` doc comment and pinned by
`TestInteractionStateBuildTimeReadSettles`.

## `IsHovered` and `OnHover` differ on purpose

| Question                          | `OnHover`                                | `IsHovered`                  |
| --------------------------------- | ---------------------------------------- | ---------------------------- |
| What is it?                       | Event dispatch                           | Visible state                |
| Which shapes answer?              | The deepest one with an `OnHover`        | The target and its ancestors |
| Does a covering shape block it?   | Only if the covering shape has `OnHover` | Yes, any shape               |
| Does it give pointer coordinates? | Yes                                      | No                           |

Covered shape: `OnHover` fires, `IsHovered` is false. Pinned by
`TestInteractionStateFloatBlocksButOnHoverFires`. Which to use is in
`docs/dx-cheat-sheet.md`.

## Known limitations

- **Keyboard press.** `ClickOnSpace` and `ClickOnEnter` fire `OnClick` with no
  held state, so `IsPressed` is false for keyboard activation.
- **Background windows** get no mouse moves (`eventAllowed`), so they record no
  new hover, same as `OnHover`. A window exit still clears it.
- **Absolute-ID children** (an ID containing `:`) do not make their parent
  hovered.

## Out of scope

Built-in `Button` and `Slider` keep their `OnHover` / `AmendLayout` looks.
Converting `OnHover` uses that only pick a look, here and in the siblings, is a
follow-up investigation (#588).

## Rejected Approaches

- **Remove `OnHover`.** It cannot be replaced where a callback reads the pointer
  position or sets a per-region cursor (go-charts `chart/gauge.go`). It would
  break four Cfgs and about 90 call sites across the siblings.
- **Live hit-test per query** (`FindByID` plus `PointInShape` at call time).
  O(n) per query, so O(n²) per frame at the 5000-widget baseline, and the
  covering and dialog rules re-derived at every call.
- **A wrapper view** `Interactive(id, func(InteractionState) View)`, now. A
  state struct designed before behavior helpers exist would be redesigned with
  them. It stays a candidate for that work. Landed as that behavior-helper
  design in #650, `docs/specs/interactive-behavior-helper.md`.
- **`mouseButtonHeld` as "pressed".** A press started elsewhere and dragged over
  a widget would read as pressed.
- **Hover written onto `Shape` during generation.** No geometry exists then.
- **Re-running generation inside the same `Update`** on a hover change. It
  re-enters generation after the view pools reset; the existing second `FrameFn`
  pass does the same job.
