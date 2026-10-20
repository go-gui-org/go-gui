# Touch presses inside scrollables

Issue #1116. Status: **implemented**.

## Problem

A finger-down was a click. The gesture recognizer synthesized the mouse press at
touch-down (`handleTouchBegan`), and a press dispatches `OnClick`. A finger that
landed on a row and then scrolled the list picked the row first, then scrolled.
In the showcase catalog on a phone the pick also closed the drawer, so the only
way to scroll it without picking was the 7 px scrollbar.

The scrollbar did not work for touch either. A press on the thumb took the drag
lock, but the mouse moves synthesized from touch carried no `MouseDX/DY`, which
the thumb drag reads, so the thumb never moved. Past the pan threshold the
touch-pan fallback scrolled the content with the finger: pulling the thumb down
scrolled the list up.

## Decision

A finger down inside a scrollable that can scroll holds its synthesized press
back until the finger shows a tap or a scroll (`gui/touch_press.go`):

1. A lift before the pan threshold (10 px) is a tap: the press goes out at the
   touch point, then the release. A short press clicks as before, at the lift.
2. A pan along an axis the scrollable can scroll is a scroll: the press is
   dropped, and nothing under the finger sees it. A finger rarely moves
   straight, so a diagonal pan counts as along the axis: a pan goes across only
   when it runs more than twice as far across the axis as along it
   (`touchPressCrossRatio`).
3. A pan clearly along the other axis goes to the widget: the press goes out at
   the touch point, then the moves. A horizontal slider in a vertical list still
   drags.
4. A long press (500 ms) sends the press before the long-press gesture, as a
   press at touch-down always came first.
5. A second finger drops it: a pinch is not a click.
6. A tap that lands on content still gliding from a fling only stops the fling,
   as on iOS and Android: the press is dropped, and the row under the finger is
   not picked. Only a tap on the gliding container: a fling in one list does not
   swallow a tap on another. The next tap clicks as usual.

A press on a scrollbar waits too, but the bar's axis settles it: a pan along the
bar sends the press, so the thumb drags; a pan across it drops the press, so the
content scrolls; a tap sends it at the lift, so the track still jumps. A bar is
thin, and with the press sent at touch-down a scroll that started on one was
lost: the track jumped, and the drag lock kept the content from panning. In the
showcase's DataSource page on a phone, the data grid's horizontal bar was such a
strip.

The press goes out at touch-down, as before, inside a `DragScroll` container
(which claims its own press, `drag-scroll.md`), outside any scrollable, in a
scrollable whose content fits, and while the mouse is locked. Mouse input is
unchanged. The scan walks the paths under the finger that a press can reach,
topmost first. The press walk falls through a child that does not take the press
to the siblings under it, so a path that holds no scrollable or scrollbar and
takes no press (a passive overlay) does not end the scan. A path that takes the
press (a focusable control, or one with a press or click handler, such as a
slider in an overlay) ends it, as it ends the press walk. The scan and the press
walk read the same checks (`pressFocuses`, `pressOnMouseDown`, `pressOnClick`),
so a click filter that refuses a left press (a right-click-only target) does not
end the scan. A held press still makes the touch point the pointer position, so
the moves after it carry deltas from the finger, not from where a mouse last
was.

The scrollbar works for touch:

- A touch-synthesized move carries `MouseDX/DY`, the distance from the previous
  pointer position, as a backend move does.
- The touch-pan fallback stands down while a press holds the mouse lock (a
  scrollbar thumb, a slider): the press owns the finger's moves, and the content
  no longer pans under it as well. A `DragScroll` lock is the exception: it
  moves only its own container, so a scrollable around it still pans on an axis
  the `DragScroll` does not scroll. A pan along an axis the `DragScroll` scrolls
  moves no outer scrollable, so one finger never moves two containers.

## Rejected Approaches

- **A wider scrollbar in the showcase catalog.** It treats one app's symptom:
  every list of clickable rows keeps picking a row on a scroll, and the thumb
  drag still ran backwards.
- **A larger touch hit area around the scrollbar.** It takes taps meant for the
  rows beside it.
- **Click on release for touch, per widget.** `OnClick` timing would depend on
  the input source, every clickable widget (Button, ListBox, Tree, Menu, tabs)
  would need the change, and a custom container with `OnClick` would keep
  clicking on a scroll.
- **Send the held press after a short hold (100 ms), as Android and iOS do.**
  Those platforms click on release, so a press shown early can still be
  cancelled when the finger scrolls. Here the press is the click: a finger that
  rests and then scrolls would pick the row again.

## Not covered

- A quick tap shows no pressed color: the press and the release arrive together
  at the lift.
- A tap clicks at the lift, typically 50–100 ms later than at touch-down.
- A vertical slider in a vertical list pans the list; it drags after a long
  press.
- A finger that rests 500 ms before it moves becomes a long press, and a long
  press never turns into a pan, so that touch scrolls nothing until the lift.
  This predates the held press (the recognizer ignores moves after a long press)
  and is left to its own issue (#1117).
- A vertical pan over a child that scrolls only horizontally (a wide Markdown
  table) barely scrolls the page: the pan fallback stops at the first scrollable
  that moves on either axis, and the child takes the finger's sideways drift.
  This also predates the held press and is left to its own issue (#1118).
