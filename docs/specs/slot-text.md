# SlotText — Rolling Character Animation Widget

Slot-machine text animation. Each character position acts as a reel that
rolls vertically through a sequence when the displayed string changes.
Embeds in buttons, labels, containers — any `Content []View` slot.

Reference: https://www.cssscript.com/demo/slot-text-roll-animation/

## Visual behavior

When `SlotText.Set(newText)` is called:

1. Each character position transitions from old value to new value by
   scrolling through intermediate characters vertically. Imagine a
   physical slot-machine reel with characters printed around the
   circumference — the reel spins until the target lands in view.

2. Characters start with staggered timing. Position 0 begins immediately;
   position N begins after `stagger * N` milliseconds.

3. Character that is already correct can optionally stay in place
   (`KeepMatching` flag).

4. When the new string is shorter, excess positions animate out (roll
   away to nothing). When it's longer, new positions animate in from
   outside the visible area.

5. Roll direction: up (characters rise from below) or down (characters
   drop from above).

6. Bounce: overshoot at the end of each character's roll for a physical
   spring-settling effect.

7. Optional chromatic sweep: hue gradient sweeps across the text during
   animation.

## Public API

```go
package gui

// SlotTextCfg configures a slot-text widget.
type SlotTextCfg struct {
    // ID for animation tracking. Must be unique per window.
    // Tagged gui:"required" — enforced by requiredid analyzer.
    ID string `gui:"required"`

    // Style for the text. Font, size, color, etc.
    Style TextStyle

    // Initial text to display (no animation on first render).
    Text string

    // Direction of the roll animation.
    // Default: SlotDirDown
    Direction SlotDirection

    // Duration of each character's roll from start to settle.
    // Default: 300ms
    Duration Opt[time.Duration]

    // Stagger delay between consecutive character positions.
    // Default: 45ms
    Stagger Opt[time.Duration]

    // Easing function applied to each character's roll progress.
    // Default: EaseOutCubic
    Easing Opt[EasingFn]

    // Bounce intensity [0, 1]. 0 = no bounce, >0 adds overshoot at end.
    // Default: 0.6
    Bounce Opt[float32]

    // Character set used for intermediate rolling values.
    // Default: alphanumeric (A-Z, a-z, 0-9) plus common symbols.
    // nil means use the default set.
    CharSet []rune

    // If true, characters that already match the target don't animate.
    // Default: true
    KeepMatching Opt[bool]

    // If true, calling Set() while an animation is running interrupts the
    // current animation (remaining characters jump to their targets, then
    // the new animation starts from resulting state).
    // Default: true
    Interrupt Opt[bool]

    // ChromaticSweep enables a hue gradient that sweeps across the text
    // during animation. nil or zero-value disables the effect.
    // Default: nil (disabled)
    ChromaticSweep *ChromaticSweepCfg

    // Fixed width for the widget. If unset, measured from the current
    // text (may change during animation as string length changes,
    // potentially causing parent re-layout).
    // Set this to stabilize layout when text length varies.
    Width Opt[float32]

    // Sizing mode for the widget.
    // Default: FitFit (natural text size)
    Sizing Sizing

    // Opacity of the rendered text.
    // Default: 1.0
    Opacity Opt[float32]

    // If set, clips rendering to bounds — characters mid-roll outside the
    // widget bounds are hidden. Always true for slot text; exposed for
    // unusual cases.
    // Default: true
    Clip Opt[bool]
}

// SlotDirection selects which way characters roll.
type SlotDirection uint8

const (
    SlotDirDown SlotDirection = iota // characters roll downward (enter from top)
    SlotDirUp                        // characters roll upward (enter from bottom)
)

// ChromaticSweepCfg configures the hue sweep color effect.
type ChromaticSweepCfg struct {
    // Starting hue, in degrees [0, 360].
    // Default: 0
    From float32

    // Total hue spread across the text width, in degrees.
    // Default: 120
    Spread float32
}

// SlotText is the widget handle. Created by SlotText(cfg).
// Its Set method triggers the roll animation to new text.
type SlotText struct {
    // unexported fields
}

// SlotText creates a new slot-text widget. Returns a View that can be
// embedded in Content slices.
func SlotText(cfg SlotTextCfg) View

// Set animates from the current text to newText. If an animation is
// already running, behavior depends on cfg.Interrupt.
//
// Safe to call from any goroutine (enqueues the animation on the
// window's command queue).
func (st *SlotText) Set(newText string)

// Flash briefly flashes newText then returns to the original text.
// A short two-phase animation: roll to newText, pause, roll back.
// Duration is halved compared to a normal Set().
func (st *SlotText) Flash(newText string)

// Text returns the current visual text (the animation target, not
// intermediate rolling characters).
func (st *SlotText) Text() string

// SetStyle updates the text style. Takes effect on next animation
// or next frame.
func (st *SlotText) SetStyle(style TextStyle)
```

## Usage examples

### Basic

```go
st := gui.SlotText(gui.SlotTextCfg{
    ID:    "score",
    Text:  "0000",
    Style: gui.TextStyle{FontFamily: "mono", Size: 24, Color: gui.White},
})
// Later, on some event:
gui.SlotTextFromView(w, "score").Set("1234")
```

### In a button

```go
gui.Button(gui.ButtonCfg{
    Content: []gui.View{
        gui.SlotText(gui.SlotTextCfg{
            ID:    "btn-label",
            Text:  "Submit",
            Style: gui.TextStyle{Size: 16, Color: gui.White},
        }),
    },
    OnClick: func(l *gui.Layout, e *gui.Event, w *gui.Window) {
        gui.SlotTextFromView(w, "btn-label").Set("Sending…")
        ctx.Consume()
    },
})
```

### With chromatic sweep

```go
gui.SlotText(gui.SlotTextCfg{
    ID:   "hero",
    Text: "Welcome",
    ChromaticSweep: &gui.ChromaticSweepCfg{From: 24, Spread: 120},
})
```

## Deliverables checklist

- [ ] `gui/view_slot_text.go` — widget implementation
- [ ] `gui/view_slot_text_test.go` — tests
- [ ] `examples/slot_text/` — standalone example app
- [ ] `examples/showcase/state.go` — demo entry in `demoEntries`
- [ ] `examples/showcase/demo_text.go` (or new `demo_slot_text.go`) — demo view
- [ ] `README.md` — widget count bump (50+ → 51+)
- [ ] `CHANGELOG.md` — entry under next version

## Implementation plan

### Phase 1 — Core widget + animation engine

**Files:** `gui/view_slot_text.go` (~300 lines), `gui/view_slot_text_test.go`

- `slotTextView` struct implementing `View`
- `GenerateLayout` returns a `DrawCanvas` that renders the slot text
- Per-character animation state stored in `StateMap` keyed by `ID`
- `slotTextAnimation` struct implementing `Animation` interface:
  - Tracks per-character progress, direction, start times
  - Each `Update` call recomputes character states and enqueues
    an `OnValue` callback to bump the DrawCanvas version
- Character roll logic:
  - For each position, compute `t = elapsed / duration` clamped to [0, 1]
  - Apply staggered start: `t = (elapsed - stagger*i) / duration`
  - Apply easing: `et = easing(t)`
  - Apply bounce: overshoot at end using a secondary spring term
  - Map to vertical offset: `y_offset = (1 - et) * slot_height * direction_sign`
  - Select visible character: interpolate in char set between old and new

**Edge cases:**
- Zero-length text → empty widget (zero width, handled gracefully)
- New string longer than old → new slots animate in from outside
- New string shorter than old → excess slots animate out
- Unicode (multi-byte, CJK wide, emoji) → measure via `TextMeasurer.TextWidth`,
  not byte/runecount. Each slot corresponds to one *character position*
  (grapheme cluster or code point, TBD).
- Very fast consecutive `Set()` calls → interrupt behavior per `Interrupt` flag
- Window closed mid-animation → auto-cancelled (view-bound animation)

### Phase 2 — Refinements

**File:** `gui/view_slot_text.go` (additions)

- Chromatic sweep: compute per-character hue offset in OnDraw, pass to
  per-character `dc.Text()` calls with per-character color
- Character set configuration: pluggable `CharSet` slice
- Flash mode: two-phase animation (forward + brief hold + reverse)
- `SetStyle` support: write new style to StateMap, bump version

### Phase 3 — Performance + integration

- Pool-allocate animation state arrays to avoid per-frame allocs
- Benchmark: target 0 allocs per frame during idle (no animation running)
- Benchmark: target < 10 allocs per frame during animation
- Reuse `[]GlyphPlacement` or intermediate buffers across frames
- **Example app:** `examples/slot_text/main.go` — demonstrates Set, Flash,
  chromatic sweep, embedding in a button, and config variants
- **Showcase integration:**
  - `examples/showcase/state.go`: add `DemoEntry` under `groupText`
    (or `groupAnimations` if a new group is warranted)
  - `examples/showcase/demo_text.go` (or new `demo_slot_text.go`):
    demo view function showing the widget with interactive controls
- **README.md:** bump widget count reference
- **CHANGELOG.md:** add entry under next version

## Design decisions

1. **DrawCanvas, not multiple Text shapes.** Per-character `Text` shapes
   would mean N shapes for N characters (alloc-heavy for long strings).
   A single `DrawCanvas` with `OnDraw` renders all characters in one pass
   with full position control.

2. **AnimationRefreshLayout during roll, idle when settled.**
   `slotTextAnimation.RefreshKind()` returns `AnimationRefreshLayout`
   while any character is still rolling, then `AnimationRefreshNone` once
   settled. This avoids rebuilding the layout tree when the widget is
   static.

3. **Cache bust via shape.Version.** The `OnDraw` callback bumps
   `shape.Version` each frame during animation to defeat the DrawCanvas
   tessellation cache. When idle, version is stable and the cache hits.

4. **View-bound animation.** Uses `animationAddViewBound` so the animation
   auto-cancels if the widget leaves the view tree. The animation ID is
   `"slot-text-" + cfg.ID`.

5. **StateMap for animation state.** Per-character progress stored in a
   `slotTextState` struct keyed by `ID` in `StateMap`. The animation
   writes to it; `OnDraw` reads from it.

6. **Character set.** Default covers A-Z, a-z, 0-9, and common symbols
   (`.` `,` `!` `?` `-` `_` `/` `:` `;` ` `). When a target character
   isn't in the set, use its Unicode codepoint to index into a
   reasonable range (e.g., wrap within the same Unicode block).

7. **Variable-width character handling.** Each slot's width is the max
   of the old character width and new character width for that position,
   measured via `TextMeasurer.TextWidth()`. This prevents horizontal
   jitter during animation. The overall widget width is the sum of
   per-slot widths (or fixed via `cfg.Width`).

8. **`gui:"required"` on ID.** The `requiredid` analyzer enforces that
   `ID` is non-empty, preventing silent animation bugs from empty IDs.

## Out of scope (v1)

- RTL text direction
- Multi-line text (single line only)
- Continuous looping mode (like a loading spinner)
- Per-character custom easing
- Sound effects
- Vertical text layout (CJK tategaki)
- Grapheme-cluster-aware slot boundaries (v1 uses code points)

## Unresolved questions

1. **Slot boundaries — code points or grapheme clusters?** Treating each
   Unicode code point as a slot is simplest but breaks for emoji with
   ZWJ sequences (e.g., 👨‍👩‍👧‍👦 is 7 code points, 1 grapheme cluster).
   Grapheme-cluster-aware splitting requires `uniseg` (already used by
   go-term). Recommendation: start with code points, document the
   limitation, add grapheme support in v2.

2. **Should the intermediate rolling characters be deterministic or
   random?** The reference uses deterministic scrolling through a
   character set. Random would look different (more chaotic, less
   predictable). Recommendation: deterministic, matching the reference.

3. **Should `charSet` include uppercase, lowercase, or both?** When
   rolling from 'A' to 'z', the distance is 57 codepoints. If the
   char set is A-Z then a-z then 0-9, it's a shorter distance.
   Recommendation: uppercase block, lowercase block, digit block,
   then symbols — in that order. Each block is contiguous.

4. **Should the widget expose `*Layout` directly for advanced use?**
   The reference allows direct DOM manipulation. In go-gui, the widget
   returns a `View`; advanced layout manipulation goes through
   `AmendLayout` on the parent container. Recommendation: no direct
   layout exposure; use `SlotTextCfg` for all configuration.
