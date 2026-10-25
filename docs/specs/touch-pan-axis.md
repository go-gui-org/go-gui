# Touch pan axes

Issue #1118. Status: **implemented**.

## Problem

On a phone, scrolling a page vertically stalled while the finger was over a
child that scrolls only horizontally, such as a wide Markdown table. The page
moved a little or not at all, and the table shifted sideways a pixel at a time.
In the showcase DataSource page on an iPhone, the docs' tables stalled for
several seconds of attempts.

The touch-pan fallback (`gestureHandlerDepth`, `gui/gesture_dispatch.go`) walked
from the innermost scrollable outward and stopped at the first one that moved on
either axis. A finger never moves exactly vertically. A table that could still
move sideways took each move's 1–2 px of drift, the move was handled, and its
vertical part never reached the page.

## Decision

Two rules, as on iOS and Android (`gui/touch_pan_axis.go`):

1. **The axis lock decides which axes a pan scrolls.** At recognition (10 px of
   travel), a pan that has run more than twice as far along one axis as across
   it scrolls only on that axis until the lift. Anything closer to the diagonal
   scrolls both. The ratio is `touchPressCrossRatio`, the one that sends a held
   press across a scrollable (`touch-press-in-scroll.md`), so a pan that drags a
   slider across a list does not also scroll the list.
2. **Per-axis routing decides which scrollable takes each axis.** Each axis goes
   to the innermost scrollable under the finger that moves on it. A diagonal pan
   over the table scrolls the table sideways and the page vertically.

An axis a scrollable moved is zeroed on the pan event, so the scrollables around
it, and their `OnGesture` handlers, see only what is left. The walk stops when
nothing is left. An axis counts as taken only when an offset actually moved, so
a pan over a container already at its limit still reaches its ancestors, as
before.

The lift flings one container per axis: the last one the pan moved on that axis.
A diagonal flick over the table flings the table sideways and the page
vertically. A vertical flick flings only the page.

The lock applies to scrolling only. An `OnGesture` handler still reads the
finger's raw `GestureDX` and `GestureDY`, less any axis a scrollable inside it
took, so a canvas that pans in two dimensions keeps both.

A pan with no recognition step scrolls both axes: the finger left after a pinch
or a rotate. A `DragScroll` pan keeps its own rule (`dragPanOuterDeltas`,
`drag-scroll.md`). The scrollables around it see the locked deltas, trimmed for
the axes the `DragScroll` owns.

## Rejected Approaches

- **Per-axis routing alone.** The page scrolls, but the table still shifts 1–2
  px sideways on every vertical move, and a horizontal swipe on the table drifts
  the page vertically. The wobble was half of the reported symptom.
- **The axis lock alone.** It fixes straight pans, but a diagonal pan over the
  table still stalls the page: the table moves sideways, the move is handled,
  and the page gets nothing.
- **One container captures the pan until the lift, as Android's
  `onInterceptTouchEvent` does.** It breaks the handoff in the middle of a pan
  from an inner list at its limit to the page around it.
- **Apply the lock to the `OnGesture` event.** A map or a canvas that reads
  two-dimensional pan deltas would lose an axis.
- **A dead zone that ignores sideways moves under a few pixels.** It swallows a
  slow, deliberate horizontal drag, and the drift still adds up past the zone.

## Not covered

- A pan that reaches a scrollable's limit in the middle of a move does not pass
  the rest of that move outward. The next move goes to the ancestor.
- The lock does not change after recognition. A pan that starts vertical and
  turns sideways keeps scrolling vertically until the lift, as on iOS.
