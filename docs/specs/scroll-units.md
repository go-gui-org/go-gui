# Scroll units and touch momentum

Issue #1110. Status: **implemented**.

## Problem

A touch pan scrolled 20 px per pixel of finger travel. Scroll input arrives in
two units: a mouse wheel reports lines of text, which `scrollVertical` /
`scrollHorizontal` turn into pixels with `Theme.ScrollMultiplier` (20), and a
finger reports pixels. The touch-pan fallback (`gui/gesture_dispatch.go`) passed
its pixel delta into the line functions. Wayland touchpads had the same defect:
`wlAxisAcc.take` reported raw points as a precise scroll, and the gui side
multiplied them by 20. macOS did not, because its backend pre-scaled precise
deltas by 0.075 (`kPreciseScrollScale`) to undo the multiplier (#22).

On every touch platform the content also stopped dead at the lift. The gesture
recognizer measured the pan velocity and emitted `GestureSwipe`, but nothing
used it.

## Decision

**Units.**

- `Event.ScrollX` / `ScrollY` are lines of text on every platform. A discrete
  wheel reports whole notches (about three lines), eased by `scrollSmoothBy`. A
  precise (trackpad) delta is fractional lines, applied at once: a backend that
  gets points of finger travel converts them at 0.075 lines per point (macOS
  `kPreciseScrollScale`, Wayland `wlPreciseLinesPerPoint`). Under the default
  multiplier a trackpad moves 1.5 px per point.
- A touch pan is not a scroll event. It moves the content by the finger's
  distance through `scrollVerticalPixels` / `scrollHorizontalPixels`, the
  multiplier-free core that `scrollVertical` / `scrollHorizontal` now wrap.
  `DragScroll` containers (`drag-scroll.md`) already moved 1:1.
- The first move after the pan threshold (10 px) catches up the distance to it,
  so the content sits under the finger. Android drops that distance instead; the
  gesture's own deltas are unchanged, so `OnGesture` handlers see what they did.
  The catch-up is timed from the press, not from the recognition, so a slow
  crawl past the threshold is not read as one fast frame and flung.

**Momentum.** A pan that lifts while the finger is still moving (last move
within `gestureSwipeStale`) flings the container it last scrolled, on each axis
it moved (`gui/scroll_momentum.go`):

- The glide is an exponential decay of the release velocity `v` with time
  constant 0.4 s: it starts at exactly `v` and travels `v · 0.4 s`. It runs on
  the wheel smoother's animation; a smoother entry with `tau > 0` decays this
  way.
- A fling aims at its unclamped target and stops on a scroll edge at full speed,
  instead of slowing early because an edge is near.
- Below 100 px/s nothing flings; above 8000 px/s the velocity is capped.
- A new touch catches the content where it is. A direct offset write (pan,
  keyboard, scrollbar) cancels the fling through `scrollSmoothCancel`. A wheel
  notch during a fling eases from the displayed offset.
- Only the finger that moved a container flings it: when a pinch hands over to a
  pan, or a remaining finger is reseeded after another lifts, the recorded
  container is cleared.

## Rejected Approaches

- **Points as the precise unit, with a theme `ScrollPreciseMultiplier`.** Honest
  units and no backend coupling, but every macOS trackpad `ScrollY` an app reads
  would grow about 13x. Apps that read raw scroll events (a terminal grid
  scrolling rows) would break. Prototyped on the #1110 branch, then dropped.
- **Each backend divides pixel input by `ScrollMultiplier`.** Spreads the
  coupling that caused both bugs to iOS, Android, web and Wayland; any new pixel
  source must remember it.
- **Only the touch pan bypasses the multiplier, Wayland left alone.** Fixes
  phones and leaves Linux touchpads at 20x.
- **A spring or physics fling (Android `OverScroller`).** More code for a glide
  the exponential decay already gives, and a second animation beside the
  smoother.

## Not covered

- A tap that catches a fling still reaches the widget under the finger; native
  scroll views swallow it.
- The release velocity is a smoothed average of the move deltas, timed when Go
  handles each event. Android Chrome feels slightly less smooth than iOS Safari;
  an estimator over the last ~80 ms of samples, using event timestamps, is the
  likely next step.
- The web backend reports trackpad pixels as lines (÷17.7) and eases them like a
  wheel.
