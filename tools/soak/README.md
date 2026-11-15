# soak

A headless soak harness for Go-Gui apps (#1262). It acts as a user on a test
window for a simulated time and measures allocations per frame and the live
heap. Use it to find a slow leak, or an allocation that starts only after many
frames, which a single-frame benchmark does not show.

## Run it

The showcase runs it from `examples/showcase/soak_test.go`:

```bash
# Smoke: 5 simulated seconds, no gates. This also runs on every PR.
go test ./examples/showcase/ -run TestSoak -v

# Full run: 60 simulated minutes, 15 minutes of warm-up, all gates.
GOGUI_SOAK=1 go test ./examples/showcase/ -run TestSoak -v

# Repeat a failure: use the seed from the report.
GOGUI_SOAK=1 go test ./examples/showcase/ -run TestSoak -v -soak.seed=42

# Random raw events, to find panics.
go test ./examples/showcase/ -run TestSoak -v -soak.mode=random -soak.duration=10m
```

| Flag             | Default                     | Meaning                         |
| ---------------- | --------------------------- | ------------------------------- |
| `-soak.duration` | 5s (60m with GOGUI_SOAK=1)  | Simulated length of the run     |
| `-soak.warmup`   | 15m for runs of 30m or more | Simulated time before measuring |
| `-soak.mode`     | scripted                    | `scripted` or `random`          |
| `-soak.seed`     | 0 (picks one)               | Seed of the random source       |

The nightly workflow `.github/workflows/soak.yml` runs both modes.

## What a run does

1. It calls `w.TestStep(0)`. From here the window clock moves only when the run
   steps it, 1/60 s per frame. No animation goroutine starts, so ten simulated
   minutes take a few seconds of wall time, and the same seed gives the same
   session.
2. It sends `EventFocused`, as a backend does at startup.
3. It draws one frame with the software renderer. This installs real glyph
   metrics and the SVG parser.
4. It takes an action, then steps 1 to 30 frames, until the time is used up.

Scripted actions: click (40 %), type text (15 %), press a key (15 %), Tab (10
%), scroll (10 %), resize (3 %), switch theme (2 %) and wait 0.5 to 5 s (5 %).
Targets come from `w.EffectiveIDs()` of the current frame. A click that fails
(no handler, scrolled away) tries another ID, up to 8 times. The typed texts
include accented Latin, emoji with modifiers, Arabic, CJK and a combining mark.

Random mode sends raw events to `w.EventFn` with random fields: keys, runes
(including invalid ones), mouse buttons and positions outside the window,
touches, IME text and resizes to sizes such as 0 and 1. It leaves out quit, file
drop and clipboard paste, because they act on the host.

Every 60 frames the frame is drawn with the software renderer, so the render
path runs too. A panic anywhere stops the run and is reported with its frame and
the last 20 actions.

## Read the report

```
soak scripted seed=1 frames=216000 measured=162000 actions=9652 wall=1m12.703s
  allocs/frame mean=67.106 p99=628  allocs/action mean=1792.7  session allocs/frame=146.2
  heap start=276103.4KiB end=248976.3KiB floor slope=-759.8KiB/min raw slope=-920.6KiB/min (270 samples)
```

- **allocs/frame**: mallocs in each `TestStep` frame after warm-up. A `Test*`
  action settles its own frame, so this counts frames that animations and timers
  drive.
- **allocs/action**: mallocs in one input action, with the frame it settles. The
  harness's own work (the action text, the list of IDs) is not counted. A click
  that misses and tries another ID counts each try. Advisory.
- **session allocs/frame**: all mallocs after warm-up, frames and actions, per
  frame. This is the cost of the session.
- **heap start / end**: the live heap after a GC, at the first and last sample
  after warm-up.
- **floor slope**: the samples are split into 8 windows of time. This is the
  slope of the lowest sample in each window, in bytes per simulated minute. The
  live heap moves with the page on screen. That noise adds peaks; only kept
  memory raises the floor. The heap gate uses this number.
- **raw slope**: the slope over all samples. Advisory.
- **render errors**: software renders that failed, for example after a random
  resize to zero size. They do not fail the run.

## Gates

`Report.Check(Gates)` fails on a panic and on each limit that is set. The
showcase limits are constants in `examples/showcase/soak_test.go`, with the
measured baselines beside them. Allocation gates are hard; wall time is only
logged.

The showcase heap grows for about the first ten simulated minutes, while the run
visits demos for the first time and their caches fill. For that reason the heap
gate applies only to runs of 30 minutes or more, with 15 minutes of warm-up.

## Use it on another app

An app in package `main` cannot be imported. Call `soak.Run` from the app's own
test:

```go
w := gui.NewTestWindow(t, appWindowCfg())
rep, err := soak.Run(w, soak.Config{Duration: 10 * time.Minute, Logf: t.Logf})
if err != nil {
	t.Fatal(err)
}
t.Log(rep.String())
if err := rep.Check(soak.Gates{MaxSessionAllocsPerFrame: 500}); err != nil {
	t.Fatal(err)
}
```

Set `Config.SkipID` to keep the scripted actions away from widgets that act on
the host, such as audio, network or native dialogs. Random mode does not use it:
its raw events land at random coordinates and can reach any widget.
