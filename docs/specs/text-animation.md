# Text animation

Issue: #543 Status: Landed

## Problem

`gui/` had a full animation subsystem — tween, spring, keyframe, FLIP layout,
hero — and no way to animate text. Every animated effect in the repo hand-rolled
the same view-bound registration inside its own widget: the skeleton shimmer,
the indefinite progress bar, the toast enter and exit, the math spinner. An app
author who wanted a headline to fade and slide in had no supported route.

## Decision

`TextCfg.Anim` takes a `TextAnimCfg`. A `Kind` names a canned effect;
`Custom func(p float32) TextAnimFrame` is the escape hatch. The exported surface
is three names plus the one field.

```go
gui.Text(gui.TextCfg{
    ID:   "headline",
    Text: "Welcome",
    Anim: gui.TextAnimCfg{
        Kind:     gui.TextAnimSlideUp,
        Duration: 400 * time.Millisecond,
    },
})
```

Named effects rather than composable primitives, because the expensive boundary
here is not the API surface. It is whether an effect needs per-glyph render
commands. A `Kind` enum keeps that boundary inside `gui/`, so the effects that
need no new plumbing shipped first and the per-glyph ones can land later without
an API change.

### Scope

Whole-string effects only: fade in, fade out, pulse, slide from four directions,
pop, shake, typewriter, shimmer.

None need new render plumbing:

| Frame field                    | Mechanism                              |
| ------------------------------ | -------------------------------------- |
| `Opacity`                      | `Shape.Opacity`                        |
| `OffsetX/Y`, `Scale`, `Rotate` | `TextStyle.AffineTransform`            |
| `Reveal`                       | slices `cfg.Text` before it is painted |
| shimmer                        | `TextStyle.Gradient`                   |

Offset, scale and rotation collapse into the one 2x3 affine, so no new `Shape`
field was needed.

## Properties worth keeping

**A motionless effect installs no transform.** Any transform pushes the text off
the fast `RenderText` path onto the glyph-layout path, which re-shapes the
string. A fade or a pulse must not pay for that.
`TestTextAnimFadeStaysOnPlainTextCommand` asserts it against the emitted
command, not against the style.

**A typewriter reserves the full string's width.** The reveal changes what is
painted; measurement still uses the whole string. A typewriter that measured
what it paints would grow its box rune by rune and reflow everything beside it.

**A loop's cycle joins up.** The default easing for a loop kind is linear, and
each loop sampler starts and ends at the same value. An eased loop stalls at
both ends, and because the end wraps to the start the seam shows as a stutter
once per cycle.

**A finished entrance does not register again.** The animation loop deletes an
animation as soon as it stops, so a one-shot needs a `done` flag in its state;
without it the next frame finds no animation and starts the entrance over, for
ever.

## Identity

Registration happens in `textView.GenerateLayout`, which has the `*Window`, so
the animation ID and the state key are `ScopeID("textanim", w.EffID(cfg.ID))` —
never the bare leaf. The same animated text dropped into two panels keeps two
independent animations.

`Anim` with an empty `ID` is a silent no-op, matching the
`Focusable`-without-`ID` precedent, and is reported under `DebugMissingIDs`.

## Known limits

- An entrance plays once per ID for the life of the window's state entry. A
  remount does not replay it. `Retrigger` can follow if a caller asks for it.
  Until then the way to replay one is to give the text a new identity —
  `ScopeIDN(owner, part, n)` with a counter — which is what the showcase's Text
  Animation page does.
- Text selection and caret rects read shape geometry and ignore the animation
  transform, so a transformed `Focusable` text has a desynced selection
  highlight. `Input` never sets `Anim`, so no input widget is affected.
- The golden harness runs with a nil `TextMeasurer`, so it cannot exercise the
  glyph-layout path. The goldens pin alpha and the painted string; the transform
  is pinned by a separate render test that installs a stub measurer.

## Rejected Approaches

- **Exported primitive tracks** (`TrackOpacity`, `TrackOffsetY`, composed by the
  caller). More power, but every sibling repo would re-derive "fade in"
  differently and `ergonomics-audit` has no way to police a composition. The
  `Custom` hook covers the same ground with one field.
- **A canned enum with no escape hatch.** Every one-off effect becomes an
  exported constant and a change inside `gui/`.
- **Per-character effects in this change** — wave, staggered fade, rainbow.
  These need `Placements []glyph.GlyphPlacement` on `RenderCmd`, emission of the
  declared-but-unused `RenderLayoutPlaced` kind, and a case in the six backend
  switches plus `print_pdf.go`.
- **Marquee as a `TextAnimKind`.** It needs a clipping container and a box
  narrower than its text, so it is a widget, not a per-`Text` effect.
- **Count-up.** It animates a number and its formatting, not the text's
  appearance. Different concern, different widget.
