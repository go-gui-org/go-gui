package gui

import (
	"time"

	"github.com/go-gui-org/go-glyph"
	"github.com/rivo/uniseg"
)

// TextAnimKind names a canned text animation. The zero value animates
// nothing.
//
// Entrance kinds (fade, slide, pop) play once and settle. Loop kinds
// (pulse, shake, shimmer) are built to run continuously and want
// Repeat: true — a single cycle of one is a one-off flash.
// exportaudit:keep — reachable from an exported field (TextAnimCfg.Kind)
type TextAnimKind uint8

// Canned text animations. See TextAnimKind.
const (
	// exportaudit:keep — one member of a public enum; the set ships whole
	TextAnimNone TextAnimKind = iota
	// TextAnimFadeIn raises opacity from 0 to 1.
	// exportaudit:keep — one member of a public enum; the set ships whole
	TextAnimFadeIn
	// TextAnimFadeOut lowers opacity from 1 to 0. The text keeps its
	// space in the layout; it becomes invisible, not absent.
	// exportaudit:keep — one member of a public enum; the set ships whole
	TextAnimFadeOut
	// TextAnimPulse breathes opacity between 1 and 0.35 and back. Loop.
	TextAnimPulse
	// TextAnimSlideUp fades in while rising into place from below.
	// exportaudit:keep — one member of a public enum; the set ships whole
	TextAnimSlideUp
	// TextAnimSlideDown fades in while dropping into place from above.
	// exportaudit:keep — one member of a public enum; the set ships whole
	TextAnimSlideDown
	// TextAnimSlideLeft fades in while moving left into place.
	// exportaudit:keep — one member of a public enum; the set ships whole
	TextAnimSlideLeft
	// TextAnimSlideRight fades in while moving right into place.
	// exportaudit:keep — one member of a public enum; the set ships whole
	TextAnimSlideRight
	// TextAnimPop fades in while growing from 80% to full size, with a
	// small overshoot.
	// exportaudit:keep — one member of a public enum; the set ships whole
	TextAnimPop
	// TextAnimShake wobbles left and right about the resting position.
	// Loop.
	TextAnimShake
	// TextAnimTypewriter reveals the text one character (grapheme
	// cluster) at a time. The text is laid out in full and the
	// unrevealed glyphs are not painted, so the box, its wrapping and
	// its alignment stay fixed and nothing around it reflows. Text that
	// grows by appending — a streamed reply — keeps typing from where
	// it had got to.
	TextAnimTypewriter
	// TextAnimShimmer sweeps a highlight across the glyphs, like the
	// skeleton placeholder. Loop.
	TextAnimShimmer

	textAnimKindCount
)

// TextAnimFrame is one sampled frame of a text animation. A zero frame
// changes nothing.
//
// Offsets are in pixels. Scale and Rotation apply about the text's
// center, so a growing or turning string stays where it sits.
type TextAnimFrame struct {
	// Opacity multiplies the text color's alpha. Opt because a fully
	// transparent frame is a legitimate choice, not "unset".
	Opacity Opt[float32]

	// Reveal is the fraction of characters (grapheme clusters)
	// painted, 0 to 1. Opt because revealing nothing is a legitimate
	// frame. The unrevealed part is not painted, but its space is still
	// reserved.
	Reveal Opt[float32]

	OffsetX float32
	OffsetY float32

	// Scale is a multiplier about the text's center. Zero means 1 — a
	// zero-scale frame paints nothing, so it is not a useful value to
	// distinguish from "unset".
	Scale float32

	// Rotation is clockwise radians about the text's center.
	Rotation float32
}

// TextAnimCfg animates a Text view. The zero value animates nothing.
//
// The animation is registered against the text's effective ID, so
// TextCfg.ID must be set — an animated text with no ID is a silent
// no-op, reported by gui.Debug.
//
// An entrance plays once for a given ID. A loop kind runs until the
// text leaves the view tree, then retires on its own. Changing Kind,
// Custom (set or not), Duration, Delay or Repeat on the same ID starts
// the new animation from the beginning.
//
// Motion composes with the style's own RotationRadians or
// AffineTransform: the animation moves the text in its own frame, and
// the style's transform then applies to the result.
type TextAnimCfg struct {
	// Custom overrides Kind. It receives eased progress in [0,1] and
	// returns the frame to paint. It runs on the main thread during
	// layout generation, so it must not touch window state.
	Custom func(p float32) TextAnimFrame

	Kind TextAnimKind

	// Duration is one cycle. Zero takes the kind's own default, which
	// for the typewriter scales with the length of the text.
	Duration time.Duration

	// Delay holds the first frame before the animation starts. With
	// Repeat the delay is part of the cycle, so it is paid again on
	// every pass, not only on the first.
	Delay time.Duration

	// Easing shapes progress before it reaches the sampler, or the
	// shimmer's band position. Zero takes the kind's default: an
	// entrance eases out, a loop stays linear so its cycle joins up
	// smoothly.
	Easing EasingFn

	Repeat bool
}

// isSet reports whether this Cfg asks for any animation.
func (a *TextAnimCfg) isSet() bool {
	return a.Custom != nil ||
		(a.Kind > TextAnimNone && a.Kind < textAnimKindCount)
}

// Default cycle lengths per kind. An entrance is quick; a loop is slow
// enough to read as ambient rather than as a demand for attention.
const (
	textAnimDurationEntrance = 300 * time.Millisecond
	textAnimDurationPulse    = 1200 * time.Millisecond
	textAnimDurationShake    = 500 * time.Millisecond
	textAnimDurationShimmer  = 1500 * time.Millisecond
	// textAnimTypeRate is the per-character cost of a typewriter
	// reveal, with a floor so a two-word string is still legible as
	// typing.
	textAnimTypeRate = 40 * time.Millisecond
	textAnimTypeMin  = 300 * time.Millisecond
)

// Motion amounts, in ems, so an effect keeps its proportions when the
// caller changes the font size.
const (
	textAnimSlideEm = 0.75
	textAnimShakeEm = 0.12
	// textAnimShakeCycles is how many left-right passes one cycle makes.
	textAnimShakeCycles = 3
	// textAnimPulseFloor is the dimmest opacity a pulse reaches.
	textAnimPulseFloor = 0.35
	// textAnimPopFrom is the starting scale of a pop.
	textAnimPopFrom = 0.8
	// textAnimShimmerBand is the half-width, in text widths, of the
	// travelling highlight.
	textAnimShimmerBand = 0.15
)

// colorToGlyph converts a gui Color to the glyph package's Color. The
// two structs hold the same four bytes; gui's carries a set flag that
// has no meaning downstream. Single conversion site — toGlyphStyle
// shares it so the two cannot drift apart.
func colorToGlyph(c Color) glyph.Color {
	return glyph.Color{R: c.R, G: c.G, B: c.B, A: c.A}
}

// textAnimDefaultDuration returns the cycle length for a kind. chars is
// the number of characters still to type, which only the typewriter
// cares about.
func textAnimDefaultDuration(k TextAnimKind, chars int) time.Duration {
	switch k {
	case TextAnimPulse:
		return textAnimDurationPulse
	case TextAnimShake:
		return textAnimDurationShake
	case TextAnimShimmer:
		return textAnimDurationShimmer
	case TextAnimTypewriter:
		return max(
			time.Duration(chars)*textAnimTypeRate, textAnimTypeMin)
	default:
		return textAnimDurationEntrance
	}
}

// textAnimDefaultEasing returns the curve a kind reads best with.
//
// A loop stays linear: an eased cycle would stall at both ends, and
// because the end wraps round to the start the seam would show as a
// stutter once per cycle. An entrance eases out, so it arrives softly.
func textAnimDefaultEasing(k TextAnimKind) EasingFn {
	switch k {
	case TextAnimPulse, TextAnimShake, TextAnimShimmer,
		TextAnimTypewriter:
		return EaseLinear
	case TextAnimPop:
		// Overshoots slightly past full size, then settles.
		return EaseOutBack
	default:
		return EaseOutCubic
	}
}

// sampleTextAnim returns the frame for a kind at eased progress p.
// em is the font size in pixels, which scales every motion amount.
//
// Shimmer returns a zero frame: it paints through a gradient rather
// than through frame fields. See textAnimShimmerGradient.
func sampleTextAnim(k TextAnimKind, p, em float32) TextAnimFrame {
	switch k {
	case TextAnimFadeIn:
		return TextAnimFrame{Opacity: SomeF(p)}

	case TextAnimFadeOut:
		return TextAnimFrame{Opacity: SomeF(1 - p)}

	case TextAnimPulse:
		// cos starts and ends at full brightness, so the cycle joins
		// up with no visible seam when it repeats.
		wave := 0.5 + 0.5*f32Cos(2*f32Pi*p)
		op := textAnimPulseFloor + (1-textAnimPulseFloor)*wave
		return TextAnimFrame{Opacity: SomeF(op)}

	case TextAnimSlideUp:
		return TextAnimFrame{
			Opacity: SomeF(p),
			OffsetY: (1 - p) * textAnimSlideEm * em,
		}

	case TextAnimSlideDown:
		return TextAnimFrame{
			Opacity: SomeF(p),
			OffsetY: -(1 - p) * textAnimSlideEm * em,
		}

	case TextAnimSlideLeft:
		return TextAnimFrame{
			Opacity: SomeF(p),
			OffsetX: (1 - p) * textAnimSlideEm * em,
		}

	case TextAnimSlideRight:
		return TextAnimFrame{
			Opacity: SomeF(p),
			OffsetX: -(1 - p) * textAnimSlideEm * em,
		}

	case TextAnimPop:
		return TextAnimFrame{
			// Opacity uses raw progress, clamped: easeOutBack can
			// carry p past 1, and an alpha above 1 is not a color.
			Opacity: SomeF(f32Clamp(p, 0, 1)),
			Scale:   textAnimPopFrom + (1-textAnimPopFrom)*p,
		}

	case TextAnimShake:
		return TextAnimFrame{
			OffsetX: textAnimShakeEm * em *
				f32Sin(2*textAnimShakeCycles*f32Pi*p),
		}

	case TextAnimTypewriter:
		return TextAnimFrame{Reveal: SomeF(f32Clamp(p, 0, 1))}

	default:
		return TextAnimFrame{}
	}
}

// applyTextAnim registers the animation for an animated text, folds the
// current frame's opacity into the shape, and hands the reveal, the
// shimmer and the motion to the renderer through tv.anim.
//
// None of them changes the text or the style the layout passes see:
// tc.Text stays the full string and the style keeps its own transform
// and gradient. Sizing, wrapping and alignment therefore work from the
// full text every frame, and renderText applies the frame to what it
// paints (see textAnimRender).
//
// A text with no ID is left alone: the animation is keyed by identity,
// and there is nothing to key on. gui.Debug reports that case.
func applyTextAnim(tv *textView, w *Window, sh *Shape) TextAnimFrame {
	cfg := &tv.cfg.Anim
	if !cfg.isSet() {
		return TextAnimFrame{}
	}
	if tv.cfg.ID == "" {
		// Gated at the call site: an animated text with no ID is
		// generated every frame, and the variadic args allocate
		// whether or not the check is on.
		if DebugCategory(debugMask.Load())&DebugMissingIDs != 0 {
			// Subject is the text itself: it is the only thing that
			// tells two ID-less animated labels apart in a warn-once
			// report.
			w.debugWarn(debugCheckTextAnimNoID, tv.cfg.Text,
				"animated text %q has no ID; the animation and its "+
					"progress are keyed by ID, so nothing animates",
				tv.cfg.Text)
		}
		return TextAnimFrame{}
	}

	// Key by the effective ID, so the same animated text dropped into
	// two panels keeps two independent animations.
	key := w.EffID(tv.cfg.ID)
	st := syncTextAnimDriver(tv, w, key)

	easing := cfg.Easing
	if easing == nil {
		easing = textAnimDefaultEasing(cfg.Kind)
	}
	p := easing(st.progress)

	var frame TextAnimFrame
	if cfg.Custom != nil {
		frame = cfg.Custom(p)
	} else {
		frame = sampleTextAnim(cfg.Kind, p, tv.cfg.TextStyle.Size)
	}

	anim := &tv.anim
	anim.reset()
	// Finite checks, not just clamps: f32Clamp passes a NaN straight
	// through, and a NaN alpha or reveal fraction comes from a Custom
	// hook doing arithmetic on a zero. Either one paints garbage, so a
	// non-finite value is dropped and the frame renders unanimated.
	if op, ok := frame.Opacity.Value(); ok && f32IsFinite(op) {
		sh.Opacity *= f32Clamp(op, 0, 1)
	}
	if rev, ok := frame.Reveal.Value(); ok && f32IsFinite(rev) &&
		!tv.cfg.IsPassword {
		// A password paints bullets, whose bytes do not line up with
		// the text's, so it is never revealed in part.
		from := 0
		if cfg.Custom == nil {
			from = st.from
		}
		end := textAnimRevealEnd(tv.cfg.Text, from, rev)
		if end < len(tv.cfg.Text) {
			anim.revealEnd = end
			anim.revealOn = true
		}
	}

	// The shimmer is built at render time, from the colour the text
	// ends up with: a filled button restamps its label's colour after
	// this pass (stampButtonLabelColor). A one-shot shimmer that has
	// finished paints no gradient, so the text returns to its own
	// colour. f32IsFinite: a NaN band position bakes NaN stops.
	if cfg.Custom == nil && cfg.Kind == TextAnimShimmer &&
		f32IsFinite(p) && (cfg.Repeat || st.progress < 1) {
		anim.shimmerP = p
		anim.shimmerOn = true
	}
	if anim.revealOn || anim.shimmerOn {
		tv.tc.anim = anim
	}
	return frame
}

// textAnimRender is what renderText needs to paint one frame of a text
// animation. It lives on the textView, which outlives the frame's
// render, and the shape reaches it through shapeTextConfig.anim. A nil
// pointer is a text with nothing to apply.
//
// Every field describes paint only. The layout passes size and wrap
// the full, untransformed text, so the box never follows the
// animation: that was the typewriter reflow and the drifting pivot.
type textAnimRender struct {
	// shimmerP is the eased progress of the shimmer's band.
	shimmerP float32
	// revealEnd is the byte length of the painted prefix.
	revealEnd int
	// scale, rot, dx and dy are the frame's motion. renderText turns
	// them into a transform about the arranged box's center.
	scale     float32
	rot       float32
	dx        float32
	dy        float32
	shimmerOn bool
	revealOn  bool
	xformOn   bool
}

// reset clears the frame.
func (a *textAnimRender) reset() { *a = textAnimRender{} }

// needsGlyphLayout reports whether the frame paints through a glyph
// layout. Nil-safe, so a caller asks without checking for an animation.
func (a *textAnimRender) needsGlyphLayout() bool {
	return a != nil && (a.shimmerOn || a.revealOn || a.xformOn)
}

// transform returns the frame's motion as a transform about the center
// of the shape's content box, or false when the frame does not move.
// It runs at render time, after arrange, so the center is where the
// box landed: a wrapped or Fill-sized text is not the size it measured
// before sizing.
func (a *textAnimRender) transform(sh *Shape) (glyph.AffineTransform, bool) {
	if a == nil || !a.xformOn {
		return glyph.AffineTransform{}, false
	}
	cx := (sh.Width - sh.paddingWidth()) / 2
	cy := (sh.Height - sh.paddingHeight()) / 2
	if !f32IsFinite(cx) || !f32IsFinite(cy) {
		return glyph.AffineTransform{}, false
	}
	return textAnimTransform(a.scale, a.rot, a.dx, a.dy, cx, cy), true
}

// gradient returns the shimmer for base, or nil when the frame does
// not shimmer. Its stops live in a render-phase pool rather than on
// the view: storage on every textView pushed each Text, animated or
// not, into a larger allocation size class.
func (a *textAnimRender) gradient(
	base Color, w *Window,
) *glyph.GradientConfig {
	if a == nil || !a.shimmerOn {
		return nil
	}
	dst := w.scratch.renderTextShimmers.alloc(textAnimShimmer{})
	return textAnimShimmerGradient(dst, base, a.shimmerP)
}

// revealLayoutGlyphs returns l with every glyph whose cluster starts at
// or past end marked as unknown. glyph's renderer skips an unknown
// glyph but still advances past it, so the unrevealed text keeps its
// place and nothing moves: not the wrap, not the alignment, not the
// revealed part. The glyphs are copied into the render arena; the
// cached layout is left as shaped.
//
// The mark is by cluster start, and a cluster never starts inside the
// revealed prefix unless all of it is revealed, so no glyph is painted
// in part.
func revealLayoutGlyphs(l glyph.Layout, end int, w *Window) glyph.Layout {
	gs := w.scratch.takeTextGlyphs(len(l.Glyphs))
	copy(gs, l.Glyphs)
	for i := range gs {
		idx := gs[i].Index &^ glyph.PangoGlyphUnknownFlag
		if int(idx) >= end {
			gs[i].Index |= glyph.PangoGlyphUnknownFlag
		}
	}
	l.Glyphs = gs
	return l
}

// applyTextAnimTransform records the frame's motion for renderText.
// The transform itself is built at render time, about the arranged
// box's center (textAnimRender.transform).
//
// Nothing is recorded for the identity. Motion pushes the text off the
// fast RenderText path and onto the glyph-layout path (see
// plainTextNeedsGlyphLayout), so a fade or a pulse must not pay for it.
func applyTextAnimTransform(tv *textView, f TextAnimFrame) {
	scale := f.Scale
	if scale == 0 {
		scale = 1
	}
	if scale == 1 && f.Rotation == 0 &&
		f.OffsetX == 0 && f.OffsetY == 0 {
		return
	}
	// Guard the whole transform: one non-finite component makes the
	// command invalid and the text vanishes for that frame.
	if !f32AllFinite4(scale, f.Rotation, f.OffsetX, f.OffsetY) {
		return
	}
	a := &tv.anim
	a.scale, a.rot, a.dx, a.dy = scale, f.Rotation, f.OffsetX, f.OffsetY
	a.xformOn = true
	tv.tc.anim = a
}

// textAnimTransform builds scale-and-rotate about (cx, cy) followed by
// a translation. Written out rather than composed from three matrix
// multiplies to keep it allocation-free and readable as one step.
//
// The transform runs in layout-local coordinates — go-glyph adds the
// draw origin afterwards — so cx, cy are offsets into the text box.
func textAnimTransform(
	scale, rot, dx, dy, cx, cy float32,
) glyph.AffineTransform {
	c := f32Cos(rot) * scale
	s := f32Sin(rot) * scale
	return glyph.AffineTransform{
		XX: c,
		XY: -s,
		YX: s,
		YY: c,
		// Move the center to the origin, transform, put it back, then
		// apply the frame's own translation.
		X0: cx - c*cx + s*cy + dx,
		Y0: cy - s*cx - c*cy + dy,
	}
}

// textAnimRevealEnd returns the byte length of the revealed part of s:
// the first from graphemes, plus frac of the rest.
//
// Graphemes, not runes: a rune cut splits a character built from
// several runes — a skin-tone or flag emoji, a ZWJ family, a base
// letter and its combining accent — and paints a wrong partial glyph
// for a frame. The count rounds down, so a character only appears once
// it is fully due. Linear in the text length per frame — fine for
// labels, not for multi-kilobyte bodies, which want no typewriter or a
// capped one.
func textAnimRevealEnd(s string, from int, frac float32) int {
	if frac >= 1 || s == "" {
		return len(s)
	}
	total := uniseg.GraphemeClusterCount(s)
	return graphemePrefixBytes(s, textAnimRevealCount(total, from, frac))
}

// textAnimRevealCount returns how many of total graphemes are shown
// when from were shown already and frac of the rest is due.
func textAnimRevealCount(total, from int, frac float32) int {
	from = min(max(from, 0), total)
	if frac >= 1 {
		return total
	}
	if frac <= 0 {
		return from
	}
	return min(from+int(float32(total-from)*frac), total)
}

// graphemePrefixBytes returns the byte length of the first n grapheme
// clusters of s, or len(s) when s holds fewer. Allocation-free.
func graphemePrefixBytes(s string, n int) int {
	end := 0
	state := -1
	rest := s
	for i := 0; i < n && rest != ""; i++ {
		var cluster string
		cluster, rest, _, state = uniseg.FirstGraphemeClusterInString(
			rest, state)
		end += len(cluster)
	}
	return end
}

// textAnimShimmerGradient sweeps a highlight band across the text.
//
// The stops are held in dst and rewritten in place, so a shimmering
// text allocates its stop slice once per frame at most — the same
// approach the skeleton placeholder takes.
func textAnimShimmerGradient(
	dst *textAnimShimmer, base Color, p float32,
) *glyph.GradientConfig {
	// Sweep from before the start to past the end, so the band enters
	// and leaves cleanly instead of appearing at the first glyph.
	pos := -textAnimShimmerBand +
		p*(1+2*textAnimShimmerBand)

	// The highlight is the base color at full strength; the rest of
	// the run is quieted, so the band reads as a moving light rather
	// than as a color change.
	dim := base.WithOpacity(textAnimPulseFloor)

	dst.stops[0] = glyph.GradientStop{
		Color: colorToGlyph(dim), Position: 0,
	}
	dst.stops[1] = glyph.GradientStop{
		Color:    colorToGlyph(dim),
		Position: f32Clamp(pos-textAnimShimmerBand, 0, 1),
	}
	dst.stops[2] = glyph.GradientStop{
		Color:    colorToGlyph(base),
		Position: f32Clamp(pos, 0, 1),
	}
	dst.stops[3] = glyph.GradientStop{
		Color:    colorToGlyph(dim),
		Position: f32Clamp(pos+textAnimShimmerBand, 0, 1),
	}
	dst.stops[4] = glyph.GradientStop{
		Color: colorToGlyph(dim), Position: 1,
	}

	dst.cfg.Stops = dst.stops[:]
	dst.cfg.Direction = glyph.GradientHorizontal
	return &dst.cfg
}

// textAnimShimmer is one shimmer gradient: a fixed stop array and the
// config that points at it, handed out whole by a render-phase pool
// (scratchPools.renderTextShimmers), so neither is allocated per frame.
type textAnimShimmer struct {
	cfg   glyph.GradientConfig
	stops [5]glyph.GradientStop
}
