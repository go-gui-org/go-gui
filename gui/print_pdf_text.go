package gui

import (
	"math"
	"strings"
	"unicode"

	"github.com/go-gui-org/go-glyph"
	"github.com/go-pdf/fpdf"
)

// Text rendering for the PDF print backend: run placement, text-on-
// path flattening, and the font-name/style mapping that turns a
// TextStyle into one of fpdf's built-in families.

func pdfRenderText(ctx *pdfCtx, cmd RenderCmd) {
	text := ctx.tr(stripUnprintable(cmd.Text))
	if text == "" {
		return
	}
	tc := cmd.Color
	// Gradient text: use first stop color as fallback
	// (true gradient fill not supported in built-in PDF
	// fonts).
	if cmd.TextGradient != nil &&
		len(cmd.TextGradient.Stops) > 0 {
		gc := cmd.TextGradient.Stops[0].Color
		tc = RGBA(gc.R, gc.G, gc.B, gc.A)
	}
	r, g, b := tc.R, tc.G, tc.B
	ctx.pdf.SetTextColor(int(r), int(g), int(b))
	alphaSet := setAlpha(ctx.pdf, tc)
	family := pdfFontName(cmd.FontName)
	style := pdfFontStyle(cmd.TextStylePtr)
	size := float64(cmd.FontSize * ctx.scale / ptToMM)
	ctx.pdf.SetFont(family, style, size)

	// Scale font so PDF text width matches source.
	// Built-in PDF fonts have different metrics than
	// the system font used by glyph.
	if cmd.TextWidth > 0 && !strings.Contains(text, "\n") {
		wantW := float64(cmd.TextWidth) * float64(ctx.scale)
		pdfW := ctx.pdf.GetStringWidth(text)
		if pdfW > 0 && wantW > 0 {
			size *= wantW / pdfW
			ctx.pdf.SetFont(family, style, size)
		}
	}

	// Y is top of text box; fpdf expects baseline.
	// Use actual font ascent when available; fall back
	// to 75% of em for SVG text and other paths that
	// don't populate FontAscent.
	fa := cmd.FontAscent
	if fa == 0 {
		fa = cmd.FontSize * 0.75
	}
	ascent := float64(fa * ctx.scale)
	lineH := size * ptToMM64() * 1.2
	lines := strings.Split(text, "\n")
	// Affine canvas text: apply the same transform the
	// screen backends see. Keep the transform around the
	// text origin so a skew around (X,Y) does not skew
	// around the page origin. The generic matrix is
	// sufficient; per-glyph curvature is out of scope.
	hasXform := cmd.LayoutTransform != nil
	if hasXform {
		t := *cmd.LayoutTransform
		mx := ctx.px(cmd.X)
		my := ctx.py(cmd.Y)
		ctx.pdf.TransformBegin()
		// T(mx,my) * Affine * T(-mx,-my) in PDF
		// coordinates (Y already flipped in mx/my).
		// For the common case (XX/YY near 1, XY/YX
		// skew), this keeps the text at its origin.
		// X0/Y0 are in logical px; scale to mm.
		e := mx*(1-float64(t.XX)) - my*float64(t.XY) +
			float64(t.X0*ctx.scale)
		f := my*(1-float64(t.YY)) - mx*float64(t.YX) +
			float64(t.Y0*ctx.scale)
		ctx.pdf.Transform(fpdf.TransformMatrix{
			A: float64(t.XX), B: float64(t.YX),
			C: float64(t.XY), D: float64(t.YY),
			E: e, F: f,
		})
	}
	for i, line := range lines {
		ctx.pdf.Text(ctx.px(cmd.X),
			ctx.py(cmd.Y)+ascent+float64(i)*lineH, line)
	}

	// Strikethrough — fpdf has no built-in support.
	if cmd.TextStylePtr != nil && cmd.TextStylePtr.Strikethrough {
		ctx.pdf.SetDrawColor(int(r), int(g), int(b))
		lw := max(float64(ctx.scale)*0.5, 0.1)
		ctx.pdf.SetLineWidth(lw)
		for i, line := range lines {
			lineW := ctx.pdf.GetStringWidth(line)
			sy := ctx.py(cmd.Y) + ascent*0.65 + float64(i)*lineH
			ctx.pdf.Line(ctx.px(cmd.X), sy, ctx.px(cmd.X)+lineW, sy)
		}
	}
	if hasXform {
		ctx.pdf.TransformEnd()
	}
	if alphaSet {
		resetAlpha(ctx.pdf)
	}
}

func pdfRenderTextPath(ctx *pdfCtx, cmd RenderCmd) {
	tp := cmd.textPath
	if tp == nil || cmd.TextStylePtr == nil || cmd.Text == "" {
		return
	}
	text := ctx.tr(stripUnprintable(cmd.Text))
	if text == "" {
		return
	}
	ts := cmd.TextStylePtr
	r, g, b := ts.Color.R, ts.Color.G, ts.Color.B
	ctx.pdf.SetTextColor(int(r), int(g), int(b))
	alphaSet := setAlpha(ctx.pdf, ts.Color)
	family := pdfFontName(ts.Family)
	style := pdfFontStyle(ts)
	size := float64(ts.Size * ctx.scale / ptToMM)
	ctx.pdf.SetFont(family, style, size)

	// Measure per-character advances with fpdf.
	runes := []rune(text)
	advances := make([]float64, len(runes))
	var totalAdv float64
	for i, ch := range runes {
		advances[i] = ctx.pdf.GetStringWidth(string(ch))
		totalAdv += advances[i]
	}

	// Apply text-anchor offset.
	offset := float64(tp.Offset * ctx.scale)
	switch tp.Anchor {
	case SvgTextAnchorMiddle:
		offset -= totalAdv / 2
	case SvgTextAnchorEnd:
		offset -= totalAdv
	}

	// Method=stretch: scale advances to fill path.
	advScale := 1.0
	if tp.method == svgTextPathMethodStretch && totalAdv > 0 {
		remaining := float64(tp.totalLen*ctx.scale) - offset
		if remaining > 0 {
			advScale = remaining / totalAdv
		}
	}

	// Place each character along the path.
	cumAdv := 0.0
	for i, ch := range runes {
		adv := advances[i] * advScale
		centerDist := float32((offset + cumAdv + adv/2) / float64(ctx.scale))
		pathX, pathY, angle := samplePathAt(
			tp.Polyline, tp.Table, centerDist)
		halfAdv := float32(adv / 2 / float64(ctx.scale))
		cosA := float32(math.Cos(float64(angle)))
		sinA := float32(math.Sin(float64(angle)))
		gx := pathX + cmd.X - halfAdv*cosA
		gy := pathY + cmd.Y - halfAdv*sinA

		angleDeg := float64(angle) * 180 / math.Pi
		mx := ctx.px(gx)
		my := ctx.py(gy)
		ctx.pdf.TransformBegin()
		ctx.pdf.TransformRotate(-angleDeg, mx, my)
		ctx.pdf.Text(mx, my, string(ch))
		ctx.pdf.TransformEnd()
		cumAdv += adv
	}
	if alphaSet {
		resetAlpha(ctx.pdf)
	}
}

// pdfFontStyle returns an fpdf style string ("B", "I", "BI", "U",
// etc.) derived from a TextStyle. Returns "" when ts is nil.
func pdfFontStyle(ts *TextStyle) string {
	if ts == nil {
		return ""
	}
	bold := ts.Typeface == glyph.TypefaceBold ||
		ts.Typeface == glyph.TypefaceBoldItalic
	italic := ts.Typeface == glyph.TypefaceItalic ||
		ts.Typeface == glyph.TypefaceBoldItalic

	// SVG text encodes weight/style in the Pango font name
	// (e.g. "Sans Bold") rather than in Typeface.
	if !bold || !italic {
		lower := strings.ToLower(ts.Family)
		if !bold && strings.Contains(lower, "bold") {
			bold = true
		}
		if !italic && strings.Contains(lower, "italic") {
			italic = true
		}
	}

	var s string
	if bold {
		s += "B"
	}
	if italic {
		s += "I"
	}
	if ts.Underline {
		s += "U"
	}
	return s
}

// pdfFontName maps a gui font family name to a built-in PDF font.
func pdfFontName(name string) string {
	lower := strings.ToLower(name)
	switch {
	case strings.Contains(lower, "mono"),
		strings.Contains(lower, "courier"),
		strings.Contains(lower, "consol"):
		return "Courier"
	case strings.Contains(lower, "serif") &&
		!strings.Contains(lower, "sans"):
		return "Times"
	case strings.Contains(lower, "georgia"),
		strings.Contains(lower, "times"),
		strings.Contains(lower, "palatino"),
		strings.Contains(lower, "garamond"):
		return "Times"
	default:
		return "Helvetica"
	}
}

// stripUnprintable removes Private Use Area and other non-printable
// runes that built-in PDF fonts cannot render.
func stripUnprintable(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.In(r, unicode.Co) { // Co = Private Use
			return -1
		}
		return r
	}, s)
}
