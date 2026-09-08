Canned animations for a `Text` view. Set `TextCfg.Anim`: a `Kind` names an
effect, or `Custom` supplies one. The zero `TextAnimCfg` animates nothing, so an
unanimated `Text` pays no cost.

An animated text needs a non-empty `ID`. The animation and its progress are
keyed by the effective ID, so the same animated label dropped into two panels
keeps two independent animations. Without an `ID` nothing animates, and
`gui.Debug` reports it under `DebugMissingIDs`.

## Usage

```go
// A canned entrance.
gui.Text(gui.TextCfg{
    ID:   "headline",
    Text: "Welcome",
    Anim: gui.TextAnimCfg{Kind: gui.TextAnimSlideUp},
})

// A loop. Repeat keeps the driver alive while the text is on screen.
gui.Text(gui.TextCfg{
    ID:   "status",
    Text: "Working",
    Anim: gui.TextAnimCfg{Kind: gui.TextAnimPulse, Repeat: true},
})

// The escape hatch: a function of eased progress in [0,1].
gui.Text(gui.TextCfg{
    ID:   "sway",
    Text: "Custom",
    Anim: gui.TextAnimCfg{
        Duration: 2 * time.Second,
        Repeat:   true,
        Custom: func(p float32) gui.TextAnimFrame {
            phase := float64(p) * 2 * math.Pi
            return gui.TextAnimFrame{
                OffsetY: 6 * float32(math.Sin(phase)),
            }
        },
    },
})
```

## Kinds

| Kind               | Effect                        | Plays |
| ------------------ | ----------------------------- | ----- |
| TextAnimFadeIn     | Transparent to opaque         | Once  |
| TextAnimFadeOut    | Opaque to transparent         | Once  |
| TextAnimSlideUp    | Rises into place              | Once  |
| TextAnimSlideDown  | Drops into place              | Once  |
| TextAnimSlideLeft  | Enters from the right         | Once  |
| TextAnimSlideRight | Enters from the left          | Once  |
| TextAnimPop        | Overshoots past its size      | Once  |
| TextAnimTypewriter | Reveals rune by rune          | Once  |
| TextAnimPulse      | Opacity breathes              | Loop  |
| TextAnimShake      | Horizontal wobble             | Loop  |
| TextAnimShimmer    | A highlight sweeps the string | Loop  |

A "Once" kind needs no `Repeat`. A "Loop" kind runs until the text leaves the
view tree; set `Repeat: true` to keep the driver alive.

## Key Properties

| Property | Type                        | Description                          |
| -------- | --------------------------- | ------------------------------------ |
| Kind     | TextAnimKind                | The canned effect to play            |
| Custom   | func(float32) TextAnimFrame | Overrides Kind; takes eased progress |
| Duration | time.Duration               | Defaults per kind                    |
| Delay    | time.Duration               | Wait before the first frame          |
| Easing   | EasingFn                    | Defaults per kind                    |
| Repeat   | bool                        | Restart on completion                |

## TextAnimFrame

The value a `Custom` function returns, and what the canned kinds produce.

| Field    | Type         | Description                          |
| -------- | ------------ | ------------------------------------ |
| Opacity  | Opt[float32] | Multiplies `TextCfg.Opacity`         |
| Reveal   | Opt[float32] | Fraction of runes painted            |
| OffsetX  | float32      | Pixels, horizontal                   |
| OffsetY  | float32      | Pixels, vertical                     |
| Scale    | float32      | Zero means 1; turns about the center |
| Rotation | float32      | Radians, about the center            |

## Defaults

| Kind               | Duration                     | Easing       |
| ------------------ | ---------------------------- | ------------ |
| Entrances          | 300ms                        | EaseOutCubic |
| TextAnimPop        | 300ms                        | Overshoot    |
| TextAnimPulse      | 1200ms                       | Linear       |
| TextAnimShake      | 500ms                        | Linear       |
| TextAnimShimmer    | 1500ms                       | Linear       |
| TextAnimTypewriter | 40ms per rune, 300ms minimum | Linear       |

A loop's easing stays linear on purpose. Each loop sampler starts and ends at
the same value, so an eased loop would stall at both ends and the seam would
show as a stutter once per cycle.

## Two behaviors worth knowing

**A typewriter reserves the full string's width.** The reveal changes what is
painted; measurement still uses the whole string, so nothing beside it reflows
as the text types itself out.

**An effect with no motion installs no transform.** A fade or a pulse stays on
the fast text render path. Only offset, scale and rotation push the text onto
the glyph-layout path, which re-shapes the string each frame.

## Replaying an entrance

An entrance plays once per identity and then sits at its resting appearance.
There is no `Retrigger` field yet. To replay one, give the text a new identity:

```go
gui.Text(gui.TextCfg{
    ID:   gui.ScopeIDN("banner", "title", app.ReplayCount),
    Text: "Welcome",
    Anim: gui.TextAnimCfg{Kind: gui.TextAnimFadeIn},
})
```

Bumping the counter retires the finished animation and registers a fresh one.
The Replay button on this page does exactly that.
