package gui

import (
	"time"
	"unicode/utf8"

	"github.com/go-gui-org/go-glyph"
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
	// TextAnimTypewriter reveals the text one rune at a time. The box
	// keeps the full string's width, so nothing around it reflows.
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

	// Reveal is the fraction of runes painted, 0 to 1. Opt because
	// revealing nothing is a legitimate frame. The unrevealed part is
	// not painted, but its width is still reserved.
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
// text leaves the view tree, then retires on its own.
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

	// Easing shapes progress before it reaches the sampler. Zero takes
	// the kind's default: an entrance eases out, a loop stays linear so
	// its cycle joins up smoothly.
	Easing EasingFn

	Repeat bool
}

// isSet reports whether this Cfg asks for any animation.
func (a *TextAnimCfg) isSet() bool {
	return a.Custom != nil ||
		(a.Kind > TextAnimNone && a.Kind < textAnimKindCount)
}

// textAnimState is what one animated text keeps between frames.
//
// done marks a finished one-shot. Without it the view-bound animation,
// which the loop deletes as soon as it stops, would be missing on the
// next frame and get registered again — an entrance that replays for
// ever.
type textAnimState struct {
	progress float32
	done     bool
}

// Default cycle lengths per kind. An entrance is quick; a loop is slow
// enough to read as ambient rather than as a demand for attention.
const (
	textAnimDurationEntrance = 300 * time.Millisecond
	textAnimDurationPulse    = 1200 * time.Millisecond
	textAnimDurationShake    = 500 * time.Millisecond
	textAnimDurationShimmer  = 1500 * time.Millisecond
	// textAnimTypeRate is the per-rune cost of a typewriter reveal,
	// with a floor so a two-word string is still legible as typing.
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
// has no meaning downstream.
func colorToGlyph(c Color) glyph.Color {
	return glyph.Color{R: c.R, G: c.G, B: c.B, A: c.A}
}

// textAnimDefaultDuration returns the cycle length for a kind. runes is
// the rune count of the text, which only the typewriter cares about.
func textAnimDefaultDuration(k TextAnimKind, runes int) time.Duration {
	switch k {
	case TextAnimPulse:
		return textAnimDurationPulse
	case TextAnimShake:
		return textAnimDurationShake
	case TextAnimShimmer:
		return textAnimDurationShimmer
	case TextAnimTypewriter:
		return max(
			time.Duration(runes)*textAnimTypeRate, textAnimTypeMin)
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
// current frame's opacity and reveal into the shape, and returns the
// frame so the caller can install its transform once the box has been
// measured.
//
// It runs from textView.GenerateLayout before measuring, so a
// typewriter reveal reaches tc.Text in time to shorten what is painted.
// Measurement deliberately keeps using the full string, so a reveal
// never reflows the text around it.
//
// A text with no ID is left alone: the animation is keyed by identity,
// and there is nothing to key on. gui.Debug reports that case.
func applyTextAnim(tv *textView, w *Window, sh *Shape) TextAnimFrame {
	cfg := &tv.cfg.Anim
	if !cfg.isSet() {
		return TextAnimFrame{}
	}
	if tv.cfg.ID == "" {
		// Subject is the text itself: it is the only thing that tells
		// two ID-less animated labels apart in a warn-once report.
		w.debugWarn(debugCheckTextAnimNoID, tv.cfg.Text,
			"animated text %q has no ID; the animation and its progress "+
				"are keyed by ID, so nothing animates", tv.cfg.Text)
		return TextAnimFrame{}
	}

	// Key by the effective ID, so the same animated text dropped into
	// two panels keeps two independent animations.
	key := w.EffID(tv.cfg.ID)
	animID := ScopeID("textanim", key)

	st := StateReadOr(w, nsTextAnim, key, textAnimState{})
	if !w.touchViewBoundAnimation(animID) && !st.done {
		// Resolve the duration only when registering the driver: the
		// typewriter default counts runes (O(n)), and this frame runs
		// on every tick of the animation.
		dur := cfg.Duration
		if dur <= 0 {
			// RuneCountInString, not len([]rune(...)): the latter
			// allocates a rune slice.
			dur = textAnimDefaultDuration(
				cfg.Kind, utf8.RuneCountInString(tv.cfg.Text))
		}
		w.animationAddViewBound(newTextAnimDriver(
			animID, key, dur, cfg.Delay, cfg.Repeat,
		))
	}

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
	// Finite checks, not just clamps: f32Clamp passes a NaN straight
	// through, and a NaN alpha or reveal fraction comes from a Custom
	// hook doing arithmetic on a zero. Either one paints garbage, so a
	// non-finite value is dropped and the frame renders unanimated.
	if op, ok := frame.Opacity.Value(); ok && f32IsFinite(op) {
		sh.Opacity *= f32Clamp(op, 0, 1)
	}
	if rev, ok := frame.Reveal.Value(); ok && f32IsFinite(rev) {
		tv.tc.Text = textAnimReveal(tv.cfg.Text, rev)
	}

	// f32Clamp passes a NaN through, so a non-finite progress
	// would bake NaN stop positions into the gradient. Progress
	// only comes from the keyframe driver, but the check is cheap
	// and a NaN gradient paints garbage.
	if cfg.Custom == nil && cfg.Kind == TextAnimShimmer &&
		f32IsFinite(st.progress) {
		tv.cfg.TextStyle.Gradient = textAnimShimmerGradient(
			&tv.shimmer, tv.cfg.TextStyle.Color, st.progress,
		)
	}
	return frame
}

// newTextAnimDriver builds the keyframe animation that advances one
// animated text.
//
// The driver always produces linear progress; the caller eases it. A
// delay is expressed as a flat leading segment rather than as new
// machinery: the animation runs for delay+duration and holds 0 until
// the delay is spent.
func newTextAnimDriver(
	animID, key string,
	dur, delay time.Duration,
	repeat bool,
) *KeyframeAnimation {
	total := dur + delay
	frames := []Keyframe{{At: 0, Value: 0}}
	if delay > 0 {
		frames = append(frames, Keyframe{
			At:    float32(delay) / float32(total),
			Value: 0,
		})
	}
	frames = append(frames, Keyframe{At: 1, Value: 1, Easing: EaseLinear})

	return &KeyframeAnimation{
		AnimID:    animID,
		Duration:  total,
		Repeat:    repeat,
		Keyframes: frames,
		OnValue: func(v float32, w *Window) {
			pm := StateMap[string, textAnimState](
				w, nsTextAnim, capMany)
			prev := pm.GetOr(key, textAnimState{})
			prev.progress = v
			pm.Set(key, prev)
		},
		OnDone: func(w *Window) {
			pm := StateMap[string, textAnimState](
				w, nsTextAnim, capMany)
			prev := pm.GetOr(key, textAnimState{})
			prev.done = true
			pm.Set(key, prev)
		},
	}
}

// applyTextAnimTransform installs the frame's transform on the style.
// It runs after the shape is measured, because the transform turns
// about the box's center.
//
// The transform is only installed when it is not the identity. Any
// transform pushes the text off the fast RenderText path and onto the
// glyph-layout path (see plainTextNeedsGlyphLayout), which re-shapes
// the string, so a fade or a pulse must not pay for one.
func applyTextAnimTransform(tv *textView, sh *Shape, f TextAnimFrame) {
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

	tv.affine = textAnimTransform(
		scale, f.Rotation, f.OffsetX, f.OffsetY,
		sh.Width/2, sh.Height/2,
	)
	tv.cfg.TextStyle.AffineTransform = &tv.affine
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

// textAnimReveal returns the leading fraction of s, by runes.
//
// Runes, not bytes: a byte cut lands mid-character and paints a
// replacement glyph. The count rounds down, so a reveal only ever
// shows a character that is fully due.
func textAnimReveal(s string, frac float32) string {
	if frac >= 1 {
		return s
	}
	if frac <= 0 || s == "" {
		return ""
	}
	total := utf8.RuneCountInString(s)
	want := int(float32(total) * frac)
	if want <= 0 {
		return ""
	}
	if want >= total {
		return s
	}
	return s[:runeToByteIndex(s, want)]
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

// textAnimShimmer is the shimmer's per-view scratch: a fixed stop array
// and the config that points at it, so neither is heap-allocated
// separately from the view.
type textAnimShimmer struct {
	cfg   glyph.GradientConfig
	stops [5]glyph.GradientStop
}
