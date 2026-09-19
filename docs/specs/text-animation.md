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

| Frame field                    | Mechanism                                           |
| ------------------------------ | --------------------------------------------------- |
| `Opacity`                      | `Shape.Opacity`                                     |
| `OffsetX/Y`, `Scale`, `Rotate` | a render-time affine, composed with the style's own |
| `Reveal`                       | unrevealed glyphs marked unknown at render time     |
| shimmer                        | a render-time gradient, from the final text colour  |

Everything except opacity reaches `renderText` through one pointer,
`shapeTextConfig.anim` (`textAnimRender`), which is nil for a text with nothing
to apply. The layout passes never see the animation: `tc.Text` stays the full
string and the style keeps its own transform and gradient.

Three reasons put this at render time and not in the view pass:

- The transform turns about the box's center, and the box is only known after
  arrange. A wrapped or Fill-sized text measures one line of its full width
  before sizing, so a pivot taken then was far from the text.
- A filled button restamps its label colour after arrange
  (`stampButtonLabelColor`). A shimmer gradient baked in the view pass kept the
  old colour.
- A typewriter that shortened `tc.Text` was sized, wrapped and aligned by its
  prefix. go-glyph skips a glyph marked `PangoGlyphUnknownFlag` but still
  advances past it, so masking the unrevealed glyphs of the full layout paints
  the prefix exactly where it sits once all of it is shown.

## Properties worth keeping

**A motionless effect installs no transform.** Any transform pushes the text off
the fast `RenderText` path onto the glyph-layout path, which re-shapes the
string. A fade or a pulse must not pay for that.
`TestTextAnimFadeStaysOnPlainTextCommand` asserts it against the emitted
command, not against the style.

**A typewriter keeps the full text's box.** The reveal changes only what is
painted; measurement, wrapping and alignment use the whole string. A typewriter
that laid out what it paints would grow its box a line at a time, type outward
from the middle when centred, and reflow everything beside it. The reveal counts
grapheme clusters, not runes, so a character built from several runes (a
skin-tone emoji, a flag, a letter and its accent) appears whole.

**Appended text continues the reveal.** When a typewriter's text changes and the
new text starts with what was already shown — a streamed reply — the run
restarts from that point, not from zero. Other text changes start over.

**A loop's cycle joins up.** The default easing for a loop kind is linear, and
each loop sampler starts and ends at the same value. An eased loop stalls at
both ends, and because the end wraps to the start the seam shows as a stutter
once per cycle.

**A finished entrance does not register again.** The animation loop deletes an
animation as soon as it stops, but its last `OnValue` and its `OnDone` land only
at the next command flush. So the state records `started`, and a started
one-shot whose driver is gone is read as finished; reading it as "register one"
replayed the entrance. `done` then lets a finished entrance skip the driver
lookup, the lock and the animation ID on every later frame.

**The state map drops only texts that left the tree.** A capped first-in,
first-out map evicted the `done` flags of texts still on screen once more than
its capacity were animated, and each evicted entrance replayed. The map is
unbounded, each entry records the view pass that last generated it
(`Window.viewPass`), and when a new ID arrives at `textAnimPruneAt` entries,
those not generated in this pass or the last are dropped.

**Changing the Cfg restarts the animation.** The state keeps the part of the Cfg
that picks the driver (`textAnimSig`: kind, Custom set or not, duration, delay,
repeat). When it changes, the old driver is removed and a new run starts, with a
new generation number; a deferred callback from the old driver carries the old
number and is dropped.

## Identity

Registration happens in `textView.GenerateLayout`, which has the `*Window`, so
the animation ID and the state key are `ScopeID("textanim", w.EffID(cfg.ID))` —
never the bare leaf. The same animated text dropped into two panels keeps two
independent animations.

`Anim` with an empty `ID` is a silent no-op, matching the
`Focusable`-without-`ID` precedent, and is reported under `DebugMissingIDs`.

## Known limits

- An entrance plays once per ID while the text stays in the tree. A remount does
  not replay it, unless the entry was pruned while the text was gone.
  `Retrigger` can follow if a caller asks for it. Until then the way to replay
  one is to give the text a new identity — `ScopeIDN(owner, part, n)` with a
  counter — which is what the showcase's Text Animation page does.
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
