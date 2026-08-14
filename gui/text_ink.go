package gui

import "math"

// InkBounds is the painted (ink) box of a text run, in logical pixels
// relative to the top-left of the run's advance box — the box a text
// shape is sized to (TextWidth × FontHeight).
//
// The two differ by the font's unpainted margins: the advance box spans
// ascent and descent plus every glyph's side bearings, whether or not
// the glyph paints there. Centring a single glyph on the advance box
// therefore leaves it visibly off-centre — a check mark, which has no
// descender, floats above the middle of its box by roughly half the
// descent. Widgets that centre one glyph inside a drawn frame centre on
// this box instead.
type InkBounds struct {
	X      float32
	Y      float32
	Width  float32
	Height float32
}

// textInkMeasurer is an optional capability on TextMeasurer backends.
// Backends wrapping a *glyph.TextSystem opt in; others need no change
// and their callers fall back to the advance box.
type textInkMeasurer interface {
	TextInkBounds(text string, style TextStyle) (InkBounds, bool)
}

// textInkBounds measures the ink box of text in style. ok is false when
// the backend cannot measure it — no measurer (tests), a backend without
// the capability, or a glyph whose outline cannot be read — and callers
// must then fall back to advance-box centring.
func (w *Window) textInkBounds(text string, style TextStyle) (
	InkBounds, bool,
) {
	if w == nil || text == "" {
		return InkBounds{}, false
	}
	tm, ok := w.textMeasurer.(textInkMeasurer)
	if !ok {
		return InkBounds{}, false
	}
	return tm.TextInkBounds(text, style)
}

// centerGlyphOnInk returns an AmendLayout hook that re-centres a
// container's text child on the glyph's ink rather than on the font's
// advance box. Use it wherever a single glyph is centred inside a drawn
// frame — a check inside a checkbox, an arrow inside a square button.
// The advance box spans the descent the glyph never paints into, and its
// side bearings are not symmetric, so an advance-centred glyph sits
// visibly high and off to one side; the taller the frame, the more
// obvious.
//
// The correction runs in AmendLayout, after sizing, for two reasons: it
// needs a measurer (so it cannot happen in the factory) and it must not
// feed back into sizing (so it moves the arranged shape rather than
// padding it into place — padding large enough to centre a big glyph
// would overflow a fixed frame and stop centring at all). The glyph is a
// leaf, so moving it moves nothing else. Without a measurer (tests) or
// without the ink capability the glyph stays where advance-box centring
// put it.
func centerGlyphOnInk(txt string, style TextStyle) func(EventCtx) {
	return func(ctx EventCtx) {
		if ctx.Layout == nil || ctx.Window == nil {
			return
		}
		glyph := firstTextChild(ctx.Layout)
		if glyph == nil {
			return
		}
		ink, ok := ctx.Window.textInkBounds(txt, style)
		if !ok {
			return
		}
		box := ctx.Layout.Shape
		// Place the ink box, not the shape: solve for the shape origin
		// that puts the ink's centre on the frame's centre. Positioning
		// from the frame rather than nudging the arranged position also
		// drops whatever rounding flow centring left behind.
		glyph.X = box.X + (box.Width-ink.Width)/2 - ink.X
		glyph.Y = box.Y + (box.Height-ink.Height)/2 - ink.Y
		// The text backend rounds each glyph's baseline to a whole
		// physical pixel, so a fractional Y is resolved in a direction
		// this code did not choose — at a default 17px checkbox that is a
		// visible half-pixel of the check's ~7px ink. Land the baseline
		// on the grid deliberately instead. X needs no such snap: the
		// backend places glyphs on quarter-pixel subpixel bins. A
		// non-finite scale (unset window, NaN from a corrupted backend)
		// skips the snap, like render's own scale guard.
		if s := ctx.Window.BackingScale; s > 0 && !math.IsInf(float64(s), 0) {
			baseline := glyph.Y + ctx.Window.textMeasurer.FontAscent(style)
			snapped := float32(math.Round(float64(baseline*s))) / s
			glyph.Y += snapped - baseline
		}
	}
}

// firstTextChild returns the shape of the container's first direct text
// child, or nil. Direct children only: the hook centres the glyph a
// factory put in the frame, not text nested in some sub-container that
// owns its own layout.
func firstTextChild(l *Layout) *Shape {
	for i := range l.Children {
		if l.Children[i].Shape == nil {
			continue
		}
		if l.Children[i].Shape.shapeType == shapeText {
			return l.Children[i].Shape
		}
	}
	return nil
}
