# Spec: animations tick at the display refresh rate

Issue: #1018

Status: **implemented** on branch `issue-1018-display-rate-ticks`, not yet
released.

## Problem

The animation loop ticked on `time.NewTicker(16ms)`. Each tick asked for a
frame, so an animated value changed at most 62.5 times each second. On a 120 Hz
or 240 Hz display the same value showed for 2 to 4 frames.

## Decision

The frame loop is the display clock. The backends already present at the refresh
rate of the display:

- Metal: `nextDrawable` blocks once for each refresh (measured in #907,
  `gui/backend/metal/backend.go`).
- GL on X11 and Windows: swap interval 1.
- Wayland: the frame callback.
- Web: `requestAnimationFrame`.

`FrameFn` sends a pulse on `animationPulseCh` when the previous `FrameFn` asked
the backend to present. The backend presented that frame before it called again,
so the pulses come at the display rate. The animation loop ticks on each pulse.
The tick asks for a frame, and that frame sends the next pulse.

Two rules keep the loop correct where no frames come or frames come too fast:

1. **Watchdog.** A timer fires `animationCycle` (16 ms) after the last tick when
   no pulse came. A hidden window (#943) presents nothing, so it ticks on the
   watchdog alone. Its clocks and callbacks keep their timing.
2. **Floor.** A pulse sooner than `animationMinTick` (2 ms) after the last tick
   sets the timer to the end of the floor. A backend that presents without vsync
   ticks at most 500 times each second.

`Update` gets the real time since the last tick as `dt`. The first tick after
the loop parks gets 16 ms. The built-in animations read the wall clock and do
not use `dt`; springs measure their own step since #1017.

The timing rules are in `animationPacer` (`gui/animation_loop.go`), so they test
with a fake clock.

## Rejected Approaches

- **A display-clock interface on each platform** (`CVDisplayLink`, `DwmFlush`,
  `wl_surface.frame`, rAF). It repeats the timing the present already gives. It
  needs native code on four platforms, a new `NativePlatform` method that each
  sibling platform must add, and cgo callbacks on the display-link thread.
  Revisit only if the pulse measures badly on one platform.
- **Report the refresh rate and set the ticker to 1/Hz.** The ticker is not
  locked to vsync. The two clocks drift, so now and then a frame gets no new
  value or two. The rate must also change when the window moves to another
  monitor.
- **Tick animations on the main thread in `FrameFn`.** A hidden window must
  still tick without frames, so the goroutine stays. It also puts `animMu` work
  on the frame path, under the same thread as `w.mu`.
