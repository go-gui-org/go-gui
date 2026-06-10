package gui

import (
	"fmt"
	"math"
	"os"
	"strings"
	"time"
	"unicode"

	"github.com/go-gui-org/go-glyph"
	"github.com/go-pdf/fpdf"
)

// ptToMM converts PostScript points to millimeters.
const ptToMM = 25.4 / 72.0

// pdfCtx holds coordinate-transform state shared by PDF render
// helpers.
type pdfCtx struct {
	pdf   *fpdf.Fpdf
	tr    func(string) string
	scale float32
	ox    float64 // margin offset X (mm)
	oy    float64 // margin offset Y (mm)
}

func (c *pdfCtx) px(x float32) float64 { return float64(x*c.scale) + c.ox }
func (c *pdfCtx) py(y float32) float64 { return float64(y*c.scale) + c.oy }
func (c *pdfCtx) pw(w float32) float64 { return float64(w * c.scale) }
func (c *pdfCtx) ph(h float32) float64 { return float64(h * c.scale) }

// renderToPDF renders a slice of RenderCmd to a PDF file at
// job.OutputPath. sourceW and sourceH are the source viewport
// dimensions in pixels (points).
func renderToPDF(renderers []RenderCmd, job PrintJob,
	sourceW, sourceH float32) error {

	pageW, pageH := PrintPageSize(job.Paper, job.Orientation)
	m := job.Margins

	printableW := (pageW - m.Left - m.Right) * ptToMM
	printableH := (pageH - m.Top - m.Bottom) * ptToMM

	if printableW <= 0 || printableH <= 0 {
		return fmt.Errorf("printable area is zero or negative")
	}

	// Scale factor: fit source viewport into printable area.
	var scale float32
	switch job.ScaleMode {
	case PrintScaleActualSize:
		// 1pt source = 1pt print (assume 72 DPI screen)
		scale = ptToMM
	default: // FitToPage
		sx := printableW / sourceW
		sy := printableH / sourceH
		scale = min(sx, sy)
	}

	orientation := "P"
	if job.Orientation == PrintLandscape {
		orientation = "L"
	}

	pdf := fpdf.NewCustom(&fpdf.InitType{
		OrientationStr: orientation,
		UnitStr:        "mm",
		Size: fpdf.SizeType{
			Wd: float64(pageW * ptToMM),
			Ht: float64(pageH * ptToMM),
		},
	})
	pdf.SetAutoPageBreak(false, 0)
	pdf.AddPage()

	// Translate UTF-8 to cp1252 for built-in PDF fonts.
	tr := pdf.UnicodeTranslatorFromDescriptor("")

	ctx := pdfCtx{
		pdf:   pdf,
		tr:    tr,
		scale: scale,
		ox:    float64(m.Left * ptToMM),
		oy:    float64(m.Top * ptToMM),
	}

	// Header/footer rendering.
	if job.Header.Enabled {
		renderHeaderFooter(pdf, tr, job.Header, job, pageW, m, true)
	}
	if job.Footer.Enabled {
		renderHeaderFooter(pdf, tr, job.Footer, job, pageW, m, false)
	}

	var clipStack []clipEntry
	var stencilClipDepth int

	for _, cmd := range renderers {
		switch cmd.Kind {
		case RenderRect:
			pdfRenderRect(&ctx, cmd)
		case RenderStrokeRect:
			pdfRenderStrokeRect(&ctx, cmd)
		case RenderText:
			pdfRenderText(&ctx, cmd)
		case RenderLine:
			pdfRenderLine(&ctx, cmd)
		case RenderCircle:
			pdfRenderCircle(&ctx, cmd)
		case RenderImage:
			pdfRenderImage(&ctx, cmd)
		case RenderClip:
			pdfRenderClip(&ctx, cmd, &clipStack)
		case RenderGradient:
			pdfRenderGradient(&ctx, cmd)
		case RenderGradientBorder:
			pdfRenderGradientBorder(&ctx, cmd)
		case RenderSvg:
			pdfRenderSvg(&ctx, cmd)
		case RenderLayout, RenderLayoutTransformed, RenderRTF:
			pdfRenderLayout(&ctx, cmd)
		case RenderTextPath:
			pdfRenderTextPath(&ctx, cmd)
		case RenderStencilBegin:
			pdfRenderStencilBegin(&ctx, cmd, &stencilClipDepth)
		case RenderStencilEnd:
			pdfRenderStencilEnd(&ctx, &stencilClipDepth)
		case RenderShadow, RenderBlur, RenderFilterBegin,
			RenderFilterEnd, RenderFilterComposite,
			RenderCustomShader,
			RenderLayoutPlaced, RenderNone:
			continue
		}
	}

	// Close any remaining clip regions.
	for range stencilClipDepth {
		pdf.ClipEnd()
	}
	for range clipStack {
		pdf.ClipEnd()
	}

	if pdf.Err() {
		return fmt.Errorf("pdf generation: %w", pdf.Error())
	}
	return pdf.OutputFileAndClose(job.OutputPath)
}

func pdfRenderRect(ctx *pdfCtx, cmd RenderCmd) {
	r, g, b := cmd.Color.R, cmd.Color.G, cmd.Color.B
	ctx.pdf.SetFillColor(int(r), int(g), int(b))
	alphaSet := setAlpha(ctx.pdf, cmd.Color)
	if cmd.Radius > 0 {
		ctx.pdf.RoundedRect(ctx.px(cmd.X), ctx.py(cmd.Y),
			ctx.pw(cmd.W), ctx.ph(cmd.H),
			float64(cmd.Radius*ctx.scale), "1234", "F")
	} else {
		ctx.pdf.Rect(ctx.px(cmd.X), ctx.py(cmd.Y),
			ctx.pw(cmd.W), ctx.ph(cmd.H), "F")
	}
	if alphaSet {
		resetAlpha(ctx.pdf)
	}
}

func pdfRenderStrokeRect(ctx *pdfCtx, cmd RenderCmd) {
	r, g, b := cmd.Color.R, cmd.Color.G, cmd.Color.B
	ctx.pdf.SetDrawColor(int(r), int(g), int(b))
	alphaSet := setAlpha(ctx.pdf, cmd.Color)
	lw := max(cmd.Thickness, 1)
	ctx.pdf.SetLineWidth(float64(lw * ctx.scale))
	if cmd.Radius > 0 {
		ctx.pdf.RoundedRect(ctx.px(cmd.X), ctx.py(cmd.Y),
			ctx.pw(cmd.W), ctx.ph(cmd.H),
			float64(cmd.Radius*ctx.scale), "1234", "D")
	} else {
		ctx.pdf.Rect(ctx.px(cmd.X), ctx.py(cmd.Y),
			ctx.pw(cmd.W), ctx.ph(cmd.H), "D")
	}
	if alphaSet {
		resetAlpha(ctx.pdf)
	}
}

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
		tc = Color{gc.R, gc.G, gc.B, gc.A, true}
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
	if alphaSet {
		resetAlpha(ctx.pdf)
	}
}

func pdfRenderLine(ctx *pdfCtx, cmd RenderCmd) {
	r, g, b := cmd.Color.R, cmd.Color.G, cmd.Color.B
	ctx.pdf.SetDrawColor(int(r), int(g), int(b))
	alphaSet := setAlpha(ctx.pdf, cmd.Color)
	lw := max(cmd.Thickness, 1)
	ctx.pdf.SetLineWidth(float64(lw * ctx.scale))
	ctx.pdf.Line(ctx.px(cmd.X), ctx.py(cmd.Y),
		ctx.px(cmd.OffsetX), ctx.py(cmd.OffsetY))
	if alphaSet {
		resetAlpha(ctx.pdf)
	}
}

func pdfRenderCircle(ctx *pdfCtx, cmd RenderCmd) {
	r, g, b := cmd.Color.R, cmd.Color.G, cmd.Color.B
	radius := cmd.W / 2
	cx := cmd.X + radius
	cy := cmd.Y + radius
	style := "F"
	if cmd.Fill {
		ctx.pdf.SetFillColor(int(r), int(g), int(b))
	} else {
		ctx.pdf.SetDrawColor(int(r), int(g), int(b))
		style = "D"
	}
	alphaSet := setAlpha(ctx.pdf, cmd.Color)
	ctx.pdf.Circle(ctx.px(cx), ctx.py(cy), ctx.pw(radius), style)
	if alphaSet {
		resetAlpha(ctx.pdf)
	}
}

func pdfRenderImage(ctx *pdfCtx, cmd RenderCmd) {
	path := cmd.Resource
	if path == "" {
		return
	}
	if _, err := os.Stat(path); err != nil {
		return
	}
	opts := fpdf.ImageOptions{ReadDpi: true}
	ctx.pdf.ImageOptions(path, ctx.px(cmd.X), ctx.py(cmd.Y),
		ctx.pw(cmd.W), ctx.ph(cmd.H), false, opts, 0, "")
}

type clipEntry struct{}

func pdfRenderClip(ctx *pdfCtx, cmd RenderCmd,
	clipStack *[]clipEntry) {
	// Each RenderClip replaces the current clip (not
	// nested). Close previous before opening a new one.
	if len(*clipStack) > 0 {
		ctx.pdf.ClipEnd()
		*clipStack = (*clipStack)[:len(*clipStack)-1]
	}
	if cmd.W > 0 && cmd.H > 0 {
		ctx.pdf.ClipRect(ctx.px(cmd.X), ctx.py(cmd.Y),
			ctx.pw(cmd.W), ctx.ph(cmd.H), false)
		*clipStack = append(*clipStack, clipEntry{})
	}
}

func pdfRenderGradient(ctx *pdfCtx, cmd RenderCmd) {
	// fpdf only supports two-color linear gradients;
	// use first and last stop colors as approximation.
	if cmd.Gradient == nil || len(cmd.Gradient.Stops) == 0 {
		return
	}
	stops := cmd.Gradient.Stops
	c1 := stops[0].Color
	c2 := stops[len(stops)-1].Color
	x := ctx.px(cmd.X)
	y := ctx.py(cmd.Y)
	w := ctx.pw(cmd.W)
	h := ctx.ph(cmd.H)
	gx1, gy1, gx2, gy2 := gradientCoords(cmd.Gradient.Direction)
	ctx.pdf.LinearGradient(x, y, w, h,
		int(c1.R), int(c1.G), int(c1.B),
		int(c2.R), int(c2.G), int(c2.B),
		gx1, gy1, gx2, gy2)
}

func pdfRenderGradientBorder(ctx *pdfCtx, cmd RenderCmd) {
	// fpdf has no gradient stroke; use first stop color
	// as a solid border approximation.
	if cmd.Gradient == nil || len(cmd.Gradient.Stops) == 0 {
		return
	}
	c := cmd.Gradient.Stops[0].Color
	ctx.pdf.SetDrawColor(int(c.R), int(c.G), int(c.B))
	alphaSet := setAlpha(ctx.pdf, Color{c.R, c.G, c.B, c.A, true})
	lw := max(cmd.Thickness, 1)
	ctx.pdf.SetLineWidth(float64(lw * ctx.scale))
	if cmd.Radius > 0 {
		ctx.pdf.RoundedRect(ctx.px(cmd.X), ctx.py(cmd.Y),
			ctx.pw(cmd.W), ctx.ph(cmd.H),
			float64(cmd.Radius*ctx.scale), "1234", "D")
	} else {
		ctx.pdf.Rect(ctx.px(cmd.X), ctx.py(cmd.Y),
			ctx.pw(cmd.W), ctx.ph(cmd.H), "D")
	}
	if alphaSet {
		resetAlpha(ctx.pdf)
	}
}

func pdfRenderSvg(ctx *pdfCtx, cmd RenderCmd) {
	if len(cmd.Triangles) < 6 {
		return
	}
	// Render SVG triangles as filled polygons.
	// Vertices are in local SVG space; transform with
	// cmd.X/Y offset and cmd.Scale (matching GPU backends).
	svgScale := cmd.Scale
	if svgScale == 0 {
		svgScale = 1
	}
	for i := 0; i+5 < len(cmd.Triangles); i += 6 {
		x1 := cmd.X + cmd.Triangles[i]*svgScale
		y1 := cmd.Y + cmd.Triangles[i+1]*svgScale
		x2 := cmd.X + cmd.Triangles[i+2]*svgScale
		y2 := cmd.Y + cmd.Triangles[i+3]*svgScale
		x3 := cmd.X + cmd.Triangles[i+4]*svgScale
		y3 := cmd.Y + cmd.Triangles[i+5]*svgScale

		// Use vertex color if available, else cmd color.
		var vc Color
		vi := i / 2 // vertex index
		if vi < len(cmd.VertexColors) {
			vc = cmd.VertexColors[vi]
		} else {
			vc = cmd.Color
		}
		ctx.pdf.SetFillColor(int(vc.R), int(vc.G), int(vc.B))
		ctx.pdf.SetDrawColor(int(vc.R), int(vc.G), int(vc.B))
		ctx.pdf.SetLineWidth(0.1)
		alphaSet := setAlpha(ctx.pdf, vc)

		pts := []fpdf.PointType{
			{X: ctx.px(x1), Y: ctx.py(y1)},
			{X: ctx.px(x2), Y: ctx.py(y2)},
			{X: ctx.px(x3), Y: ctx.py(y3)},
		}
		ctx.pdf.Polygon(pts, "FD")
		if alphaSet {
			resetAlpha(ctx.pdf)
		}
	}
}

func pdfRenderLayout(ctx *pdfCtx, cmd RenderCmd) {
	if cmd.LayoutPtr == nil {
		return
	}
	// RenderLayout/Transformed use TextStylePtr for
	// font family/style; RenderRTF uses plain Helvetica.
	family := "Helvetica"
	fontStyle := ""
	if cmd.Kind != RenderRTF && cmd.TextStylePtr != nil {
		family = pdfFontName(cmd.TextStylePtr.Family)
		fontStyle = pdfFontStyle(cmd.TextStylePtr)
	}
	layoutText := cmd.LayoutPtr.Text
	for i := range cmd.LayoutPtr.Items {
		item := &cmd.LayoutPtr.Items[i]
		if item.IsObject {
			continue
		}
		// Text lives in Layout.Text; Item references
		// a substring via StartIndex/Length.
		end := item.StartIndex + item.Length
		if end > len(layoutText) {
			continue
		}
		text := ctx.tr(stripUnprintable(
			layoutText[item.StartIndex:end]))
		if text == "" {
			continue
		}
		r, g, b := item.Color.R, item.Color.G, item.Color.B
		ctx.pdf.SetTextColor(int(r), int(g), int(b))
		itemAlpha := item.Color.A < 255
		if itemAlpha {
			ctx.pdf.SetAlpha(float64(item.Color.A)/255.0, "Normal")
		}
		// Derive font size from ascent (≈75% of em for
		// standard PDF fonts).
		srcSize := item.Ascent / 0.75
		size := srcSize * float64(ctx.scale) / float64(ptToMM)
		ctx.pdf.SetFont(family, fontStyle, size)

		// Scale font so PDF text width matches the
		// glyph-computed item width. Built-in PDF
		// fonts have different metrics than the
		// system font used by glyph.
		wantW := float64(item.Width) * float64(ctx.scale)
		pdfW := ctx.pdf.GetStringWidth(text)
		if pdfW > 0 && wantW > 0 {
			size *= wantW / pdfW
			ctx.pdf.SetFont(family, fontStyle, size)
		}

		ix := ctx.px(cmd.X + float32(item.X))
		iy := ctx.py(cmd.Y + float32(item.Y))
		ctx.pdf.Text(ix, iy, text)

		if item.HasUnderline {
			ctx.pdf.SetDrawColor(int(r), int(g), int(b))
			lw := max(item.UnderlineThickness*float64(ctx.scale), 0.1)
			ctx.pdf.SetLineWidth(lw)
			uy := ctx.py(cmd.Y + float32(item.Y+item.UnderlineOffset))
			ctx.pdf.Line(ix, uy,
				ix+ctx.pw(float32(item.Width)), uy)
		}
		if item.HasStrikethrough {
			ctx.pdf.SetDrawColor(int(r), int(g), int(b))
			lw := max(item.StrikethroughThickness*float64(ctx.scale), 0.1)
			ctx.pdf.SetLineWidth(lw)
			sy := ctx.py(cmd.Y + float32(item.Y+item.StrikethroughOffset))
			ctx.pdf.Line(ix, sy,
				ix+ctx.pw(float32(item.Width)), sy)
		}
		if itemAlpha {
			resetAlpha(ctx.pdf)
		}
	}
}

func pdfRenderTextPath(ctx *pdfCtx, cmd RenderCmd) {
	tp := cmd.TextPath
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
	if tp.Method == SvgTextPathMethodStretch && totalAdv > 0 {
		remaining := float64(tp.TotalLen*ctx.scale) - offset
		if remaining > 0 {
			advScale = remaining / totalAdv
		}
	}

	// Place each character along the path.
	cumAdv := 0.0
	for i, ch := range runes {
		adv := advances[i] * advScale
		centerDist := float32((offset + cumAdv + adv/2) / float64(ctx.scale))
		pathX, pathY, angle := SamplePathAt(
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

func pdfRenderStencilBegin(ctx *pdfCtx, cmd RenderCmd,
	depth *int) {
	// Approximate rounded-rect stencil with rect clip.
	if cmd.W > 0 && cmd.H > 0 {
		ctx.pdf.ClipRect(ctx.px(cmd.X), ctx.py(cmd.Y),
			ctx.pw(cmd.W), ctx.ph(cmd.H), false)
		*depth++
	}
}

func pdfRenderStencilEnd(ctx *pdfCtx, depth *int) {
	if *depth > 0 {
		ctx.pdf.ClipEnd()
		*depth--
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

// ptToMM64 returns ptToMM as float64 (used for font baseline).
func ptToMM64() float64 { return float64(ptToMM) }

// setAlpha applies alpha transparency to the PDF state. Returns true
// if state was changed (caller should call resetAlpha). Unset colors
// (Color.IsSet() == false) are treated as fully opaque.
func setAlpha(pdf *fpdf.Fpdf, c Color) bool {
	if c.IsSet() && c.A < 255 {
		pdf.SetAlpha(float64(c.A)/255.0, "Normal")
		return true
	}
	return false
}

// resetAlpha restores full opacity.
func resetAlpha(pdf *fpdf.Fpdf) {
	pdf.SetAlpha(1.0, "Normal")
}

// gradientCoords returns (x1,y1,x2,y2) for fpdf.LinearGradient
// based on gradient direction.
func gradientCoords(dir GradientDirection) (float64, float64, float64, float64) {
	switch dir {
	case GradientToRight:
		return 0, 0, 1, 0
	case GradientToBottom:
		return 0, 0, 0, 1
	case GradientToLeft:
		return 1, 0, 0, 0
	case GradientToTop:
		return 0, 1, 0, 0
	case GradientToBottomRight:
		return 0, 0, 1, 1
	case GradientToBottomLeft:
		return 1, 0, 0, 1
	case GradientToTopRight:
		return 0, 1, 1, 0
	case GradientToTopLeft:
		return 1, 1, 0, 0
	default:
		return 0, 0, 0, 1
	}
}

// renderHeaderFooter draws a header or footer line on the page.
func renderHeaderFooter(pdf *fpdf.Fpdf, tr func(string) string,
	cfg PrintHeaderFooterCfg,
	job PrintJob, pageW float32, m PrintMargins, isHeader bool) {

	fontSize := 8.0 // points
	pdf.SetFont("Helvetica", "", fontSize)
	pdf.SetTextColor(0, 0, 0)

	var yPt float32
	if isHeader {
		yPt = m.Top * 0.5 // center in top margin
	} else {
		_, pageH := PrintPageSize(job.Paper, job.Orientation)
		yPt = pageH - m.Bottom*0.5
	}
	y := float64(yPt * ptToMM)

	leftX := float64(m.Left * ptToMM)
	rightX := float64((pageW - m.Right) * ptToMM)
	centerX := float64(pageW * ptToMM / 2)

	replacer := headerFooterReplacer(job)

	if cfg.Left != "" {
		text := tr(replacer.Replace(cfg.Left))
		pdf.Text(leftX, y, text)
	}
	if cfg.Center != "" {
		text := tr(replacer.Replace(cfg.Center))
		tw := pdf.GetStringWidth(text)
		pdf.Text(centerX-tw/2, y, text)
	}
	if cfg.Right != "" {
		text := tr(replacer.Replace(cfg.Right))
		tw := pdf.GetStringWidth(text)
		pdf.Text(rightX-tw, y, text)
	}
}

// headerFooterReplacer builds a strings.Replacer for print tokens.
func headerFooterReplacer(job PrintJob) *strings.Replacer {
	return strings.NewReplacer(
		"{page}", "1",
		"{pages}", "1",
		"{date}", time.Now().Format("2006-01-02"),
		"{title}", job.Title,
		"{job}", job.JobName,
	)
}
