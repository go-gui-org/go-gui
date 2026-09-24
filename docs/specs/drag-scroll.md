# Spec: drag-to-scroll for scrollable containers

Status: implemented (go-gui #783) Base: `main` @ `66f655f2`

## Motivation

A scrollable canvas answers the mouse wheel and the trackpad, but not the
pointer drag. Issue #783 asks for drag scrolling on the family-tree canvas from
#582: press the background, move the pointer, and the content follows it. Touch
users already pan with one finger through the gesture fallback. Mouse users have
no path.

## Behavior

`ContainerCfg.DragScroll` opts a `Scrollable` container into the pan. It
requires `Scrollable: true` and a non-empty `ID`. A container that sets it
without one panics at generation time.

The press walk claims a left press on the container before children dispatch.
The claim takes the mouse lock and marks the event handled. Then one of two
results occurs:

- The pointer moves past the drag threshold (5 px, shared with reorder drags).
  The container scrolls 1:1 with the pointer. The press-point click never fires.
  The cursor shows the grabbing hand.
- The pointer releases before the threshold. The release replays the press
  through normal dispatch, so the child clicks exactly as a plain press clicks.
  Then the real release runs for its `OnMouseUp` handlers.

The scroll honors `ScrollMode` per axis and clamps at the content edges. A tap
on a `DragScroll` container clicks at release, not at press. That timing shift
touches only `DragScroll` subtrees.

## Touch ownership

Touch takes the same path. Its synthesized mouse events run the same press walk,
so one finger pans through the mouse lock. The gesture pan fallback skips
`DragScroll` containers, so one finger never scrolls twice. A second finger
cancels the press through `MouseCancel`: the offset stays where the pan left it,
and no click fires.

## Cursor

The hover shows the open hand only while content overflows on an axis. A
container that fits keeps the arrow. The active pan shows the closed hand.
Platform mappings:

- macOS: `openHandCursor` / `closedHandCursor`.
- X11: the `grab` / `grabbing` theme names, with the hand glyphs as fallback.
- Web: the `grab` / `grabbing` CSS values.
- Windows: no open-hand stock cursor exists, and `IDC_HAND` is the pointing
  hand. Both map to the arrow. The pan still works, but it shows no hand.

## Rejected Approaches

- **Always-on middle-button pan, no API.** It answers the wrong button. It is
  undiscoverable, dead on trackpads, and it fights the Linux paste binding.
- **App-level recipe with `OnMouseDown` + `MouseLock`, no framework change.**
  Each app then reimplements the threshold, the click suppression, the
  `ScrollMode` check, and the touch dedup. Each app gets one of them wrong, and
  a wrong one breaks child clicks with no error.
- **Kinetic scrolling with momentum.** Out of scope. The drag is 1:1 by design.
  Momentum can follow as a separate change.

## Verification

`gui/drag_scroll_test.go` covers the pan, the threshold, tap replay, click
suppression, clamping, `ScrollMode`, cancel, the gesture skip, the generation
panics, and the cursor. The full module suite passes with no other test changed.
