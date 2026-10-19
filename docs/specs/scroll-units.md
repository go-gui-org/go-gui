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
  multiplier a trackpad moves 1.5 px per point. A Wayland seat before
  `wl_pointer` v5 has no wheel clicks and no axis source, so a wheel and a
  touchpad look alike; it is read as a wheel (10 units per click).
- A touch pan is not a scroll event. It moves the content by the finger's
  distance through `scrollVerticalPixels` / `scrollHorizontalPixels`, the
  multiplier-free core that `scrollVertical` / `scrollHorizontal` now wrap.
  `DragScroll` containers (`drag-scroll.md`) already moved 1:1.
- The first move after the pan threshold (10 px) catches up the distance to it,
  so the content sits under the finger. Android drops that distance instead; the
  gesture's own deltas are unchanged, so `OnGesture` handlers see what they did.

**Momentum.** A pan that lifts while the finger is still moving (last move
within `gestureSwipeStale`) flings the container it last scrolled, on each axis
it moved (`gui/scroll_momentum.go`):

- The release velocity `v` is the finger's displacement over its last 100 ms of
  positions (`panReleaseVelocity`), like Android's velocity tracker. It is not
  the pan's smoothed velocity (`Event.VelocityX/Y`, unchanged): that one starts
  at 0 and lags, so a short flick read about half its speed, and it counts the
  threshold catch-up as one frame, so a slow crawl past the threshold read as
  fast.
- The glide is an exponential decay of `v` with time constant 0.4 s: it starts
  at exactly `v` and travels `v · 0.4 s`. It runs on the wheel smoother's
  animation; both use `scrollEaseStep` with their own time constant (the wheel's
  25% per 16 ms is tau ≈ 55.6 ms).
- A fling aims at its unclamped target and stops on a scroll edge at full speed,
  instead of slowing early because an edge is near. The edge is read from the
  layout in the apply pass of every frame (`scrollFlingEdge`), not kept from the
  lift: the animation tick cannot read the layout and does not clamp. A glide
  follows rows a list appends and stops on the new end of content that shrank.
- A scroll-anchor or re-measure correction mid-fling shifts the glide's target
  with the content (`scrollSmoothShiftY`), so it continues instead of turning
  back. A wheel ease keeps its absolute target.
- Below 100 px/s nothing flings; above 8000 px/s the velocity is capped.
- A new touch catches the content where it is. A direct offset write (pan,
  keyboard, scrollbar, drag-select autoscroll, the command palette opening, the
  inspector) cancels the fling through `scrollSmoothCancel`; layout corrections
  (the range clamp, an input's caret-into-view) do not. A wheel notch during a
  fling eases from the displayed offset; a wheel request the smoother rejects
  leaves the fling running.
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
- The samples are timed when Go handles each touch event, not by the event's own
  timestamp (`TouchPoint` carries none). Android Chrome feels slightly less
  smooth than iOS Safari.
- The web backend reports trackpad pixels as lines (÷17.7) and eases them like a
  wheel.
