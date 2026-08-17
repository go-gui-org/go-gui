package gui

import (
	"math"
	"strings"

	glyph "github.com/go-gui-org/go-glyph"
)

// Optical centring (issue #346).
//
// A text shape is sized to FontHeight — ascent + descent — and a
// vertically-centred container centres that box. The ink is not centred
// in the box: the baseline sits at Y + FontAscent (render_text.go), so a
// run with no descender paints only in [baseline - capHeight, baseline]
// and leaves the reserved descent empty. Measured on the platform UI
// face, caps ride 0.0886em high and digits 0.0708em — about 1.4 device
// pixels at size 16 on a 2x display, scaling with em.
//
// Two rules govern the correction, and both come from evidence rather
// than from the model:
//
//  1. It is applied only where the widget owns the text. A uniform
//     correction was tried and rejected: it reads correct on a badge and
//     visibly low on an Input, because arbitrary typed text does paint
//     into the descent the correction reclaims. Measured at 48pt on the
//     real render, an uncorrected count rides 6.5 device pixels high,
//     while "gypsy" already sits 0.022em *below* centre before any
//     correction — moving it down is strictly wrong.
//
//  2. Editable text is corrected from the face alone, never from what it
//     currently says. An offset measured from live text moves the
//     baseline as the user types — the jitter that killed the first
//     attempt. That is why the two entry points differ, and the
//     difference is the whole design:
//
//     opticalCapOffset(style) is content-free, and safe for editable
//     text whose alphabet the widget constrains — a hex or channel
//     field. There is no way to hand it the live text.
//
//     opticalCenterText measures the run it is about to move, and is
//     only for text the widget owns and the user cannot type into: a
//     badge, a readout, a button or tab label.
//
// A static label may be measured because it changes when the app changes
// it, not per keystroke — and measuring it is what keeps a row of badges
// consistent. Uncorrected, "128" rides 0.071em high while "gypsy" sits
// 0.022em low: a 0.093em spread between neighbours. Correcting each on
// its own ink closes that to nothing for anything that fits the cap
// band, and leaves the descender case where it already was.
//
// There is no half-leading term. glyph reports Height as ascent+descent
// with LineGap separate (glyph/context_puregoft.go), so the line box a
// text shape occupies carries no leading to split — an earlier local fix
// carried a measured 0.047em "lead" constant that was compensating for
// this correction being applied at twice its size.

// opticalProbe is the run whose ink defines the band the content-free
// form centres on. 'H' is flat-topped in essentially every design, so
// its ink height is the cap height with no overshoot correction to undo
// — the same probe go-glyph uses for its own cap-height measurement.
//
// It is the right probe for a field whose alphabet includes caps: a hex
// colour field. It is the wrong one for a field that holds only digits,
// which measure shorter — see opticalDigitProbe.
const opticalProbe = "H"

// opticalDigitProbe is the content-free probe for a field whose alphabet
// is digits and separators — a date mask, a numeric input. Figures are a
// single height in a text face, so any digit measures the band.
//
// Probing 'H' there overshoots, and visibly: measured on screen at 16pt,
// a numeric field centred on the cap band landed 1.5 device pixels *low*
// — the same magnitude of error as the 1.5 high it started at. Matching
// the probe to the alphabet the widget guarantees is the whole reason
// the guarantee is the caller's to give.
const opticalDigitProbe = "0"

// fallbackCapOffsetRatio is the correction to apply when the ink box
// cannot be measured: no measurer (tests), a backend without the ink
// capability (WASM/canvas), or a face whose outline will not read. It is
// the offset measured on the platform UI face — asc 0.770em, desc
// 0.230em, cap 0.717em — expressed as a fraction of the font size, which
// is how the correction scales anyway.
//
// It stands in for the measurement and does not reproduce it: a face
// with an unusual ascent/descent split lands elsewhere. That is the
// point of preferring the probe wherever the backend can answer.
const fallbackCapOffsetRatio = 0.0886

// fallbackDigitOffsetRatio is the same measurement taken with a digit
// rather than a cap: figures are shorter, so the correction that centres
// them is smaller. Both numbers come from the same probe run against the
// same face; keeping them apart is what stops the fallback path from
// reintroducing the overshoot the measured path avoids.
const fallbackDigitOffsetRatio = 0.0708

// descenderRunes are the ASCII letters that paint below the baseline.
// They are consulted only on the fallback path, where no ink box can be
// measured: a rune test cannot know what an arbitrary face does, but it
// is right for the Latin text a headless test renders, and being wrong
// there costs a fraction of a pixel in a recording rather than a defect
// on screen.
const descenderRunes = "gjpqy"

// opticalBand names which band a hook centres on. It exists because the
// choice is not binary: a run's own ink, the face's cap band, or the
// face's figure band are three different answers, and which one is right
// is a property of the widget's text rather than of the correction.
type opticalBand int

const (
	// opticalBandRun measures the run about to be moved. Only for text
	// the widget owns and that does not change with the control's state.
	opticalBandRun opticalBand = iota
	// opticalBandCap centres the face's cap band, ignoring whatever
	// descends. For a label whose alphabet is unconstrained but whose
	// content changes — a Select's placeholder and selected option.
	opticalBandCap
	// opticalBandDigit centres the face's figure band. For an editable
	// field whose mask admits digits and separators only.
	opticalBandDigit
)

// opticalKey identifies a face+size, and for the measured form the run
// as well, for the offset memo. TextStyle itself cannot be a map key —
// it holds pointer fields (Features, Gradient, AffineTransform) that
// vary per call without changing the metrics — and these are what the
// ink measurement depends on.
//
// probe is the run that was measured, and contentFree says which rule
// asked for it — the two forms can name the same run ("0" as a digit
// probe and as a badge's whole label) while falling back differently, so
// the flag is what keeps them out of each other's entry.
type opticalKey struct {
	family      string
	probe       string
	typeface    glyph.Typeface
	size        float32
	spacing     float32
	contentFree bool
}

// opticalCapOffset returns the downward shift, in logical pixels, that
// puts the cap band of style's face on the centre of the line box a text
// shape in that style occupies.
//
// The result depends only on the face and its size — never on what the
// text says — so a field corrected with it cannot move as its content
// changes. This is the form for editable text whose alphabet the widget
// constrains.
func (w *Window) opticalCapOffset(style TextStyle) float32 {
	return w.opticalOffset(style, opticalProbe, true, fallbackCapOffsetRatio)
}

// opticalDigitOffset is opticalCapOffset for a field that holds no caps:
// it centres the figure band instead. Content-free in exactly the same
// way — the probe is a constant, not the field's value — so it carries
// the same no-jitter guarantee.
func (w *Window) opticalDigitOffset(style TextStyle) float32 {
	return w.opticalOffset(
		style, opticalDigitProbe, true, fallbackDigitOffsetRatio)
}

// opticalTextOffset returns the shift that centres this run's own ink,
// bounded below by zero and above by the cap-band shift.
//
// The lower bound is the descender case: "gypsy" already sits below the
// box's centre, so the correction for it is none. Moving text up is not
// this function's job — the metric centring it would undo is what every
// uncorrected widget in the toolkit does.
//
// Only for text the widget owns. See the file comment for why measuring
// a run is safe there and not in an Input.
func (w *Window) opticalTextOffset(style TextStyle, text string) float32 {
	// A run with no ink has no centre to solve for. Whitespace reaches
	// here from real widgets — a date picker pads its empty leading
	// cells with " " — and correcting it would move a shape whose
	// glyphs paint nothing, which is motion without a reason on the
	// measured path and a wrong guess on the fallback one.
	if strings.TrimSpace(text) == "" {
		return 0
	}
	off := w.opticalOffset(style, text, false, fallbackCapOffsetRatio)
	return f32Min(off, w.opticalCapOffset(style))
}

// opticalOffset measures probe's ink and solves for the shift that lands
// its centre on the box centre. ratio is what stands in when no ink can
// be measured, and contentFree records which rule asked, so the two
// forms cannot share a memo entry.
func (w *Window) opticalOffset(
	style TextStyle, probe string, contentFree bool, ratio float32,
) float32 {
	if w == nil || style.Size <= 0 {
		return 0
	}
	key := opticalKey{
		family:      style.Family,
		probe:       probe,
		typeface:    style.Typeface,
		size:        style.Size,
		spacing:     style.LineSpacing,
		contentFree: contentFree,
	}
	// Memoized per window, not per package: the measurement comes from
	// the window's own backend, and two windows need not share one. It
	// costs a shaping pass and a glyph-outline read, while the pass that
	// consumes it is allocation-free and runs every frame — so it must
	// happen once per face, size and run.
	cache := StateMap[opticalKey, float32](w, nsOpticalOffset, capMany)
	if off, ok := cache.Get(key); ok {
		return off
	}

	off := fallbackOffset(style, probe, ratio)
	if ink, inkOK := w.textInkBounds(probe, style); inkOK &&
		ink.Height > 0 {
		// Solve for the shift that lands the ink band's centre on the
		// box centre. Both are measured from the top of the same box:
		// textInkBounds reports Y relative to the advance box top, and
		// the box is what fontHeight returns, so the two share the
		// baseline-at-ascent convention render_text.go draws with.
		off = fontHeight(style, w)/2 - (ink.Y + ink.Height/2)
	}
	// A run whose ink already sits centred or low needs no correction;
	// a negative shift would move it further from centre, not closer.
	off = f32Max(off, 0)
	cache.Set(key, off)
	return off
}

// fallbackOffset stands in where no ink box can be measured. It applies
// the caller's measured platform ratio to a run that cannot descend and
// nothing to one that can, which is the same decision the ink path makes
// — just taken from the runes instead of from the outline.
func fallbackOffset(style TextStyle, probe string, ratio float32) float32 {
	if strings.ContainsAny(probe, descenderRunes) {
		return 0
	}
	return style.Size * ratio
}

// glyphStyle marks a style whose runs are symbol glyphs rather than
// text: a triangle, a multiplication sign standing in for a close
// button, a vertical ellipsis. Such a run has no cap band to centre on
// and must keep its own ink, the way an icon-font glyph does — the
// theme's Icon rungs carry the mark already, and this is the spelling
// for a symbol drawn in an ordinary text face (issue #346).
//
// It is a function rather than a field a caller sets so the mark stays
// unexported and searchable: every glyph-in-a-button in the toolkit
// goes through here.
func glyphStyle(ts TextStyle) TextStyle {
	ts.glyphRole = true
	return ts
}

// opticalCenterText is an AmendLayout hook that moves a container's
// direct text children down onto their optical centre. Use it on a
// container that centres text the widget itself owns — a badge's count,
// a progress bar's percentage, a button or tab label. Never on text the
// user types.
//
// It runs in AmendLayout, after sizing, for the same two reasons
// centerGlyphOnInk does: it needs a measurer, so it cannot happen in a
// factory (Badge and Button build eagerly, with no *Window in hand), and
// it must not feed back into sizing, so it moves the arranged shape
// rather than padding it into place — a control's height is set by the
// theme's inset and has to keep matching the controls beside it.
//
// The run and its style come from the shape about to be moved, not from
// the call site. Button takes arbitrary Content, so its label is not a
// string the factory holds; reading the shape also removes any chance of
// correcting one run by another's measurement.
//
// Each text child is centred on its own ink, so a label carrying a
// descender stays where metric centring put it rather than being pushed
// below the middle. See the file comment for why measuring the run is
// safe here and not on an editable field.
func opticalCenterText(ctx EventCtx) {
	opticalCenterChildren(ctx, opticalBandRun)
}

// opticalCenterLabelText centres a widget-owned label on the face's cap
// band, ignoring what the label currently says. It is for a control that
// *swaps* its label as its state changes — a Select showing a
// placeholder until an option is chosen, then showing the option.
//
// Two reasons it is not opticalCenterText, and both were measured on the
// real render rather than argued (issue #346):
//
//   - The measured form is a no-op on the labels this control actually
//     carries. "Pick a language" descends, so its ink band already sits
//     low and the clamp leaves it alone — while its cap band, which is
//     what the eye reads a control's label by, still rode 2.5 device
//     pixels high at size 16.
//   - An offset taken from the run would change when the selection does,
//     so picking a different option would step the label vertically in a
//     control that has not moved. Content-free is what a control that
//     re-labels itself needs, for the same reason an editable field
//     needs it.
//
// The consequence is deliberate: a descender hangs below centre instead
// of pulling the whole label up, which is how a control label is set.
func opticalCenterLabelText(ctx EventCtx) {
	opticalCenterChildren(ctx, opticalBandCap)
}

// opticalCenterFieldText is the content-free sibling of
// opticalCenterText, for an *editable* field whose alphabet is digits
// and separators: a date field, a numeric input. It centres on the
// face's figure band and never reads what the shape currently says, so
// the baseline cannot move as the user types — the jitter that ruled out
// correcting Input in general.
//
// A field that also admits caps (a hex colour field) wants the cap band
// instead; probing the taller band for a digit-only field overshoots by
// as much as leaving it alone.
//
// Prefer the padding form (colorFieldPadding) where the widget generates
// with a *Window in hand and sizes its own row. This hook is for the
// widgets that wrap Input, whose text shape is arranged inside a control
// they do not own the padding of: shifting the arranged shape leaves the
// field's height — and so its match with the Input beside it —
// untouched, where spending padding there would grow the control.
//
// Never use it on a general Input. The alphabet being constrained is the
// whole licence: it is what makes the reserved descent provably empty.
func opticalCenterFieldText(ctx EventCtx) {
	opticalCenterChildren(ctx, opticalBandDigit)
}

// opticalCenterChildren moves a container's direct text children down by
// the offset the caller's band chooses.
func opticalCenterChildren(ctx EventCtx, band opticalBand) {
	if ctx.Layout == nil || ctx.Window == nil {
		return
	}
	for i := range ctx.Layout.Children {
		txt := ctx.Layout.Children[i].Shape
		if txt == nil || txt.shapeType != shapeText || txt.TC == nil {
			continue
		}
		// An icon glyph beside a label is a text run too, and centring
		// it on its own ink is what the toolkit already does for single
		// glyphs (centerGlyphOnInk). Correcting each child separately
		// keeps the two consistent instead of moving one and not the
		// other.
		//
		// The style comes from the shape, so a field showing its
		// placeholder is corrected for the placeholder's face rather
		// than the text style it will have once typed into.
		// A blank run is *not* skipped on a content-free band, and
		// that is the point of one: an empty digit field still shows a
		// caret, whose position comes from this shape, so declining to
		// move it would step the caret the moment the first character
		// arrived. The measured band declines a blank run inside
		// opticalTextOffset, where the question is about ink.
		style := textStyleOrDefault(txt)
		// A content-free band is a claim about an alphabet, and an icon
		// glyph is not in one. Centring an arrow on the icon face's cap
		// band would shift it by whatever that face's "H" measures — a
		// notdef box, or nothing at all, in a face drawn for symbols —
		// where its own ink is the answer the toolkit already gives a
		// single glyph elsewhere (centerGlyphOnInk). The style says
		// which it is, because no measurement can.
		effective := band
		if style.glyphRole && band != opticalBandRun {
			effective = opticalBandRun
		}
		var off float32
		switch effective {
		case opticalBandDigit:
			off = ctx.Window.opticalDigitOffset(style)
		case opticalBandCap:
			off = ctx.Window.opticalCapOffset(style)
		default:
			off = ctx.Window.opticalTextOffset(style, txt.TC.Text)
		}
		if off == 0 {
			continue
		}
		txt.Y += off
		// Land the baseline on a whole device pixel. The text backend
		// rounds each glyph's baseline anyway, so a fractional shift is
		// otherwise resolved in a direction this code did not choose —
		// and at these magnitudes (about 1.4 device px) that rounding is
		// a large fraction of the correction itself. Mirrors the snap in
		// centerGlyphOnInk; a non-finite scale skips it, as render's own
		// scale guard does.
		if s := ctx.Window.BackingScale; s > 0 &&
			!math.IsInf(float64(s), 0) && ctx.Window.textMeasurer != nil {
			baseline := txt.Y + ctx.Window.textMeasurer.FontAscent(style)
			snapped := float32(math.Round(float64(baseline*s))) / s
			txt.Y += snapped - baseline
		}
	}
}
