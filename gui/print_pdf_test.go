package gui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/go-gui-org/go-glyph"
)

func testPrintJob(t *testing.T) PrintJob {
	t.Helper()
	j := NewPrintJob()
	j.OutputPath = filepath.Join(t.TempDir(), "test.pdf")
	return j
}

func TestRenderToPDF_Empty(t *testing.T) {
	j := testPrintJob(t)
	err := renderToPDF(nil, j, 800, 600)
	if err != nil {
		t.Fatal(err)
	}
	assertPDFExists(t, j.OutputPath)
}

func TestRenderToPDF_Rect(t *testing.T) {
	j := testPrintJob(t)
	cmds := []RenderCmd{{
		Kind: RenderRect, X: 10, Y: 10, W: 100, H: 50,
		Color: RGBA(255, 0, 0, 255),
	}}
	if err := renderToPDF(cmds, j, 800, 600); err != nil {
		t.Fatal(err)
	}
	assertPDFExists(t, j.OutputPath)
}

func TestRenderToPDF_RoundedRect(t *testing.T) {
	j := testPrintJob(t)
	cmds := []RenderCmd{{
		Kind: RenderRect, X: 10, Y: 10, W: 100, H: 50,
		Color: RGBA(0, 128, 0, 255), Radius: 8,
	}}
	if err := renderToPDF(cmds, j, 800, 600); err != nil {
		t.Fatal(err)
	}
	assertPDFExists(t, j.OutputPath)
}

func TestRenderToPDF_StrokeRect(t *testing.T) {
	j := testPrintJob(t)
	cmds := []RenderCmd{{
		Kind: RenderStrokeRect, X: 10, Y: 10, W: 100, H: 50,
		Color: RGBA(0, 0, 255, 255), Thickness: 2,
	}}
	if err := renderToPDF(cmds, j, 800, 600); err != nil {
		t.Fatal(err)
	}
	assertPDFExists(t, j.OutputPath)
}

func TestRenderToPDF_Text(t *testing.T) {
	j := testPrintJob(t)
	cmds := []RenderCmd{{
		Kind: RenderText, X: 50, Y: 50,
		Color: RGBA(0, 0, 0, 255),
		Text:  "Hello PDF", FontName: "sans", FontSize: 14,
	}}
	if err := renderToPDF(cmds, j, 800, 600); err != nil {
		t.Fatal(err)
	}
	assertPDFExists(t, j.OutputPath)
}

func TestRenderToPDF_MonoFont(t *testing.T) {
	if got := pdfFontName("monospace"); got != "Courier" {
		t.Errorf("monospace -> %q, want Courier", got)
	}
	if got := pdfFontName("serif"); got != "Times" {
		t.Errorf("serif -> %q, want Times", got)
	}
	if got := pdfFontName("sans-serif"); got != "Helvetica" {
		t.Errorf("sans-serif -> %q, want Helvetica", got)
	}
	if got := pdfFontName("Consolas"); got != "Courier" {
		t.Errorf("Consolas -> %q, want Courier", got)
	}
}

func TestRenderToPDF_Line(t *testing.T) {
	j := testPrintJob(t)
	cmds := []RenderCmd{{
		Kind: RenderLine, X: 0, Y: 0, OffsetX: 100, OffsetY: 100,
		Color: RGBA(0, 0, 0, 255), Thickness: 1,
	}}
	if err := renderToPDF(cmds, j, 800, 600); err != nil {
		t.Fatal(err)
	}
	assertPDFExists(t, j.OutputPath)
}

func TestRenderToPDF_Circle(t *testing.T) {
	j := testPrintJob(t)
	cmds := []RenderCmd{{
		Kind: RenderCircle, X: 50, Y: 50, W: 40, H: 40,
		Color: RGBA(128, 0, 128, 255), Fill: true,
	}}
	if err := renderToPDF(cmds, j, 800, 600); err != nil {
		t.Fatal(err)
	}
	assertPDFExists(t, j.OutputPath)
}

func TestRenderToPDF_ImageMissing(t *testing.T) {
	j := testPrintJob(t)
	cmds := []RenderCmd{{
		Kind: RenderImage, X: 0, Y: 0, W: 100, H: 100,
		Resource: "/nonexistent/image.png",
	}}
	if err := renderToPDF(cmds, j, 800, 600); err != nil {
		t.Fatal(err)
	}
	assertPDFExists(t, j.OutputPath)
}

func TestRenderToPDF_ClipBeginEnd(t *testing.T) {
	j := testPrintJob(t)
	cmds := []RenderCmd{
		{Kind: RenderClip, X: 10, Y: 10, W: 200, H: 200},
		{Kind: RenderRect, X: 0, Y: 0, W: 400, H: 400,
			Color: RGBA(255, 0, 0, 255)},
		{Kind: RenderClip}, // end clip
	}
	if err := renderToPDF(cmds, j, 800, 600); err != nil {
		t.Fatal(err)
	}
	assertPDFExists(t, j.OutputPath)
}

func TestRenderToPDF_SkippedKinds(t *testing.T) {
	j := testPrintJob(t)
	cmds := []RenderCmd{
		{Kind: RenderShadow},
		{Kind: RenderBlur},
		{Kind: RenderFilterBegin},
		{Kind: RenderFilterEnd},
		{Kind: RenderCustomShader},
		{Kind: RenderNone},
	}
	if err := renderToPDF(cmds, j, 800, 600); err != nil {
		t.Fatal(err)
	}
	assertPDFExists(t, j.OutputPath)
}

func TestRenderToPDF_Landscape(t *testing.T) {
	j := testPrintJob(t)
	j.Orientation = PrintLandscape
	cmds := []RenderCmd{{
		Kind: RenderRect, X: 10, Y: 10, W: 200, H: 100,
		Color: RGBA(0, 0, 255, 255),
	}}
	if err := renderToPDF(cmds, j, 1024, 768); err != nil {
		t.Fatal(err)
	}
	assertPDFExists(t, j.OutputPath)
}

func TestRenderToPDF_ActualSize(t *testing.T) {
	j := testPrintJob(t)
	j.ScaleMode = PrintScaleActualSize
	cmds := []RenderCmd{{
		Kind: RenderRect, X: 0, Y: 0, W: 100, H: 100,
		Color: RGBA(128, 128, 128, 255),
	}}
	if err := renderToPDF(cmds, j, 400, 300); err != nil {
		t.Fatal(err)
	}
	assertPDFExists(t, j.OutputPath)
}

func TestRenderToPDF_Gradient(t *testing.T) {
	j := testPrintJob(t)
	cmds := []RenderCmd{{
		Kind: RenderGradient, X: 0, Y: 0, W: 200, H: 100,
		Gradient: &GradientDef{
			Direction: GradientToRight,
			Stops: []GradientStop{
				{Color: RGBA(255, 0, 0, 255), Pos: 0},
				{Color: RGBA(0, 0, 255, 255), Pos: 1},
			},
		},
	}}
	if err := renderToPDF(cmds, j, 800, 600); err != nil {
		t.Fatal(err)
	}
	assertPDFExists(t, j.OutputPath)
}

func TestRenderToPDF_GradientNilDef(t *testing.T) {
	j := testPrintJob(t)
	cmds := []RenderCmd{{
		Kind: RenderGradient, X: 0, Y: 0, W: 200, H: 100,
	}}
	if err := renderToPDF(cmds, j, 800, 600); err != nil {
		t.Fatal(err)
	}
	assertPDFExists(t, j.OutputPath)
}

func TestRenderToPDF_SvgTriangles(t *testing.T) {
	j := testPrintJob(t)
	cmds := []RenderCmd{{
		Kind: RenderSvg,
		Triangles: []float32{
			0, 0, 50, 0, 25, 50,
			50, 0, 100, 0, 75, 50,
		},
		Color: RGBA(0, 128, 0, 255),
	}}
	if err := renderToPDF(cmds, j, 800, 600); err != nil {
		t.Fatal(err)
	}
	assertPDFExists(t, j.OutputPath)
}

func TestRenderToPDF_SvgTooFewTriangles(t *testing.T) {
	j := testPrintJob(t)
	cmds := []RenderCmd{{
		Kind:      RenderSvg,
		Triangles: []float32{0, 0, 50},
		Color:     RGBA(0, 128, 0, 255),
	}}
	if err := renderToPDF(cmds, j, 800, 600); err != nil {
		t.Fatal(err)
	}
	assertPDFExists(t, j.OutputPath)
}

func TestRenderToPDF_HeaderFooter(t *testing.T) {
	j := testPrintJob(t)
	j.Title = "Test Document"
	j.JobName = "test-job"
	j.Header = PrintHeaderFooterCfg{
		Enabled: true,
		Left:    "{title}",
		Center:  "Page {page}",
		Right:   "{date}",
	}
	j.Footer = PrintHeaderFooterCfg{
		Enabled: true,
		Center:  "{job}",
	}
	if err := renderToPDF(nil, j, 800, 600); err != nil {
		t.Fatal(err)
	}
	assertPDFExists(t, j.OutputPath)
}

func TestRenderToPDF_AlphaTransparency(t *testing.T) {
	j := testPrintJob(t)
	cmds := []RenderCmd{{
		Kind: RenderRect, X: 10, Y: 10, W: 100, H: 50,
		Color: RGBA(255, 0, 0, 128),
	}}
	if err := renderToPDF(cmds, j, 800, 600); err != nil {
		t.Fatal(err)
	}
	assertPDFExists(t, j.OutputPath)
}

func TestRenderToPDF_GradientBorder(t *testing.T) {
	j := testPrintJob(t)
	cmds := []RenderCmd{{
		Kind: RenderGradientBorder, X: 10, Y: 10, W: 100, H: 50,
		Thickness: 2, Radius: 4,
		Gradient: &GradientDef{
			Stops: []GradientStop{
				{Color: RGBA(255, 0, 0, 255)},
				{Color: RGBA(0, 0, 255, 255)},
			},
		},
	}}
	if err := renderToPDF(cmds, j, 800, 600); err != nil {
		t.Fatal(err)
	}
	assertPDFExists(t, j.OutputPath)
}

func TestRenderToPDF_RTF(t *testing.T) {
	j := testPrintJob(t)
	layout := &glyph.Layout{
		Text: "Hello RTF",
		Items: []glyph.Item{
			{
				StartIndex: 0,
				Length:     9,
				X:          0,
				Y:          20,
				Width:      80,
				Ascent:     12,
				Descent:    4,
				Color:      glyph.Color{R: 0, G: 0, B: 0, A: 255},
			},
		},
		Width:  200,
		Height: 40,
	}
	cmds := []RenderCmd{{
		Kind:      RenderRTF,
		X:         10,
		Y:         10,
		LayoutPtr: layout,
	}}
	if err := renderToPDF(cmds, j, 800, 600); err != nil {
		t.Fatal(err)
	}
	assertPDFExists(t, j.OutputPath)
	// Text is zlib-compressed in the PDF stream, so verify
	// the PDF is larger than a minimal empty page (~910 bytes).
	info, _ := os.Stat(j.OutputPath)
	if info.Size() < 1000 {
		t.Errorf("PDF too small (%d bytes), likely missing text", info.Size())
	}
}

func TestRenderToPDF_RTF_NilLayout(t *testing.T) {
	j := testPrintJob(t)
	cmds := []RenderCmd{{Kind: RenderRTF, X: 10, Y: 10}}
	if err := renderToPDF(cmds, j, 800, 600); err != nil {
		t.Fatal(err)
	}
	assertPDFExists(t, j.OutputPath)
}

func TestRenderToPDF_TextPath(t *testing.T) {
	j := testPrintJob(t)
	// Simple horizontal polyline: (0,0) -> (100,0)
	polyline := []float32{0, 0, 100, 0}
	table := []float32{0, 100}
	cmds := []RenderCmd{{
		Kind: RenderTextPath,
		X:    10, Y: 10,
		Text: "AB",
		TextStylePtr: &TextStyle{
			Family: "sans",
			Size:   14,
			Color:  RGBA(0, 0, 0, 255),
		},
		TextPath: &TextPathData{
			Polyline: polyline,
			Table:    table,
			TotalLen: 100,
		},
	}}
	if err := renderToPDF(cmds, j, 800, 600); err != nil {
		t.Fatal(err)
	}
	assertPDFExists(t, j.OutputPath)
}

func TestRenderToPDF_TextMultilineStrikethrough(t *testing.T) {
	j := testPrintJob(t)
	cmds := []RenderCmd{{
		Kind: RenderText,
		X:    50, Y: 50,
		Color:    RGBA(0, 0, 0, 255),
		Text:     "Line1\nLine2Long",
		FontName: "sans", FontSize: 14,
		TextStylePtr: &TextStyle{Strikethrough: true},
	}}
	if err := renderToPDF(cmds, j, 800, 600); err != nil {
		t.Fatal(err)
	}
	assertPDFExists(t, j.OutputPath)
}

func TestRenderToPDF_TextGradientFallback(t *testing.T) {
	j := testPrintJob(t)
	cmds := []RenderCmd{{
		Kind: RenderText,
		X:    10, Y: 10,
		Color:    RGBA(0, 0, 0, 255),
		Text:     "Gradient",
		FontName: "sans", FontSize: 14,
		TextGradient: &glyph.GradientConfig{
			Stops: []glyph.GradientStop{
				{Color: glyph.Color{R: 255, G: 0, B: 0, A: 200}, Position: 0},
				{Color: glyph.Color{R: 0, G: 0, B: 255, A: 255}, Position: 1},
			},
		},
	}}
	if err := renderToPDF(cmds, j, 800, 600); err != nil {
		t.Fatal(err)
	}
	assertPDFExists(t, j.OutputPath)
}

func TestRenderToPDF_Layout(t *testing.T) {
	j := testPrintJob(t)
	layout := &glyph.Layout{
		Text: "Wrapped text line",
		Items: []glyph.Item{
			{
				StartIndex: 0,
				Length:     17,
				X:          0,
				Y:          20,
				Width:      120,
				Ascent:     12,
				Descent:    4,
				Color:      glyph.Color{R: 225, G: 225, B: 225, A: 255},
			},
		},
		Width:  200,
		Height: 40,
	}
	ts := TextStyle{Family: "sans", Size: 14}
	ts.Color = RGBA(225, 225, 225, 255)
	cmds := []RenderCmd{
		// Dark background.
		{Kind: RenderRect, X: 0, Y: 0, W: 200, H: 100,
			Color: RGBA(50, 50, 50, 255)},
		// Layout text (white on dark).
		{Kind: RenderLayout, X: 10, Y: 10,
			LayoutPtr: layout, TextStylePtr: &ts},
	}
	if err := renderToPDF(cmds, j, 800, 600); err != nil {
		t.Fatal(err)
	}
	assertPDFExists(t, j.OutputPath)
	info, _ := os.Stat(j.OutputPath)
	if info.Size() < 1000 {
		t.Errorf("PDF too small (%d bytes), likely missing text", info.Size())
	}
}

func TestPrintJobResolvePDFPath_UnknownSource(t *testing.T) {
	job := NewPrintJob()
	job.Source.Kind = PrintJobSourceKind(99)
	_, err := printJobResolvePDFPath(nil, job)
	if err == nil {
		t.Fatal("expected error for unknown source kind")
	}
}

func TestRenderToPDF_Diagnostic(t *testing.T) {
	j := testPrintJob(t)
	j.OutputPath = filepath.Join(t.TempDir(), "go-gui-diag.pdf")
	cmds := []RenderCmd{
		// Dark background.
		{Kind: RenderRect, X: 0, Y: 0, W: 800, H: 600,
			Color: RGBA(45, 45, 48, 255)},

		// 1. White text on dark bg (no clip).
		{Kind: RenderText, X: 20, Y: 20,
			Color: RGBA(225, 225, 225, 255),
			Text:  "1. White text no clip", FontName: "sans",
			FontSize: 14},

		// 2. Black text on dark bg (should be invisible).
		{Kind: RenderText, X: 20, Y: 50,
			Color: RGBA(0, 0, 0, 255),
			Text:  "2. Black text (invisible)", FontName: "sans",
			FontSize: 14},

		// 3. Light rect + dark text inside clip.
		{Kind: RenderClip, X: 20, Y: 80, W: 400, H: 200},
		{Kind: RenderRect, X: 20, Y: 80, W: 400, H: 200,
			Color: RGBA(60, 60, 63, 255)},
		{Kind: RenderText, X: 30, Y: 100,
			Color: RGBA(225, 225, 225, 255),
			Text:  "3. White text inside clip", FontName: "sans",
			FontSize: 14},
		{Kind: RenderText, X: 30, Y: 130,
			Color: RGBA(225, 225, 225, 255),
			Text:  "4. Second line in clip", FontName: "sans",
			FontSize: 13},
		{Kind: RenderClip}, // end clip

		// 5. Text after clip ends.
		{Kind: RenderText, X: 20, Y: 320,
			Color: RGBA(225, 225, 225, 255),
			Text:  "5. Text after clip", FontName: "sans",
			FontSize: 14},

		// 6. Semi-transparent text.
		{Kind: RenderText, X: 20, Y: 360,
			Color: RGBA(225, 225, 225, 128),
			Text:  "6. Semi-transparent text", FontName: "sans",
			FontSize: 14},
	}
	if err := renderToPDF(cmds, j, 800, 600); err != nil {
		t.Fatal(err)
	}
	t.Logf("Diagnostic PDF: %s", j.OutputPath)
	assertPDFExists(t, j.OutputPath)
}

// TestRenderToPDF_TwoPanel simulates the showcase's two-panel
// layout with nested clip-restore pattern used by renderLayout.
func TestRenderToPDF_TwoPanel(t *testing.T) {
	j := testPrintJob(t)
	j.OutputPath = filepath.Join(t.TempDir(), "go-gui-twopanel.pdf")

	windowClip := func() RenderCmd {
		return RenderCmd{Kind: RenderClip, X: 0, Y: 0, W: 800, H: 600}
	}

	cmds := []RenderCmd{
		// Window background.
		{Kind: RenderRect, X: 0, Y: 0, W: 800, H: 600,
			Color: RGBA(45, 45, 48, 255)},

		// --- Left panel (clip enter) ---
		{Kind: RenderClip, X: 0, Y: 0, W: 350, H: 600},
		{Kind: RenderRect, X: 0, Y: 0, W: 350, H: 600,
			Color: RGBA(55, 55, 58, 255)},
		{Kind: RenderText, X: 20, Y: 30,
			Color: RGBA(225, 225, 225, 255),
			Text:  "LEFT PANEL", FontName: "sans", FontSize: 16},

		// Nested clip inside left panel (child scroll area).
		{Kind: RenderClip, X: 10, Y: 60, W: 330, H: 500},
		{Kind: RenderText, X: 20, Y: 80,
			Color: RGBA(200, 200, 200, 255),
			Text:  "Left nested clip content", FontName: "sans",
			FontSize: 12},
		// Restore left panel clip (exit child).
		{Kind: RenderClip, X: 0, Y: 0, W: 350, H: 600},

		// More left panel content after nested clip.
		{Kind: RenderText, X: 20, Y: 580,
			Color: RGBA(200, 200, 200, 255),
			Text:  "Left footer", FontName: "sans", FontSize: 10},
		// Restore window clip (exit left panel).
		windowClip(),

		// --- Right panel (clip enter) ---
		{Kind: RenderClip, X: 350, Y: 0, W: 450, H: 600},
		{Kind: RenderRect, X: 350, Y: 0, W: 450, H: 600,
			Color: RGBA(65, 65, 68, 255)},
		{Kind: RenderText, X: 370, Y: 30,
			Color: RGBA(225, 225, 225, 255),
			Text:  "RIGHT PANEL", FontName: "sans", FontSize: 16},

		// Nested clip inside right panel (detail scroll area).
		{Kind: RenderClip, X: 360, Y: 60, W: 430, H: 500},
		{Kind: RenderText, X: 370, Y: 80,
			Color: RGBA(200, 200, 200, 255),
			Text:  "Right nested clip content", FontName: "sans",
			FontSize: 12},
		{Kind: RenderText, X: 370, Y: 100,
			Color: RGBA(200, 200, 200, 255),
			Text:  "Export PDF  |  Print", FontName: "sans",
			FontSize: 12},
		// Restore right panel clip (exit child).
		{Kind: RenderClip, X: 350, Y: 0, W: 450, H: 600},

		// Restore window clip (exit right panel).
		windowClip(),
	}
	if err := renderToPDF(cmds, j, 800, 600); err != nil {
		t.Fatal(err)
	}
	t.Logf("Two-panel PDF: %s", j.OutputPath)
	assertPDFExists(t, j.OutputPath)
}

// TestRenderToPDF_FullPipeline exercises the complete layout→render→PDF
// pipeline with two side-by-side scroll panels, matching the showcase
// structure.
func TestRenderToPDF_FullPipeline(t *testing.T) {
	w := &Window{}
	w.windowWidth = 800
	w.windowHeight = 600

	view := Row(ContainerCfg{
		Width:      800,
		Height:     600,
		Sizing:     FixedFixed,
		Padding:    NoPadding,
		Spacing:    NoSpacing,
		SizeBorder: NoBorder,
		Content: []View{
			// Left panel (scrollable).
			Column(ContainerCfg{
				ID:         "pdf-left",
				Scrollable: true,
				Width:      300,
				Sizing:     FixedFill,
				Padding:    SomeP(8, 8, 8, 8),
				SizeBorder: NoBorder,
				Color:      RGBA(55, 55, 58, 255),
				Content: []View{
					Text(TextCfg{Text: "LEFT PANEL TITLE"}),
					Text(TextCfg{Text: "Left item 1"}),
					Text(TextCfg{Text: "Left item 2"}),
					Text(TextCfg{Text: "Left item 3"}),
				},
			}),
			// Right panel (scrollable).
			Column(ContainerCfg{
				ID:         "pdf-right",
				Scrollable: true,
				Sizing:     FillFill,
				Padding:    SomeP(8, 8, 8, 8),
				SizeBorder: NoBorder,
				Color:      RGBA(65, 65, 68, 255),
				Content: []View{
					Text(TextCfg{Text: "RIGHT PANEL TITLE"}),
					Text(TextCfg{Text: "Right content line 1"}),
					Text(TextCfg{Text: "Right content line 2"}),
				},
			}),
		},
	})

	rootLayout := generateViewLayout(view, w)
	layers := layoutArrange(&rootLayout, w)
	w.layout = composeLayout(layers, w)
	w.buildRenderers(RGBA(45, 45, 48, 255), w.windowRect())

	// Count render commands by kind and check for right panel.
	var hasRightText bool
	clipCount := 0
	for _, cmd := range w.renderers {
		if cmd.Kind == RenderClip {
			clipCount++
		}
		// Right panel content starts at X >= 300.
		if cmd.Kind == RenderText && cmd.X >= 300 {
			hasRightText = true
		}
	}

	t.Logf("Total renderers: %d, clips: %d", len(w.renderers), clipCount)

	if !hasRightText {
		// Dump render commands for debugging.
		for i, cmd := range w.renderers {
			if cmd.Kind == RenderClip {
				t.Logf("[%d] Clip xy=(%.0f,%.0f) wh=(%.0f,%.0f)",
					i, cmd.X, cmd.Y, cmd.W, cmd.H)
			}
			if cmd.Kind == RenderText {
				t.Logf("[%d] Text xy=(%.0f,%.0f) %q",
					i, cmd.X, cmd.Y, cmd.Text)
			}
			if cmd.Kind == RenderRect {
				t.Logf("[%d] Rect xy=(%.0f,%.0f) wh=(%.0f,%.0f) color=(%d,%d,%d)",
					i, cmd.X, cmd.Y, cmd.W, cmd.H,
					cmd.Color.R, cmd.Color.G, cmd.Color.B)
			}
		}
		t.Fatal("no text render commands in right panel (X >= 300)")
	}

	// Export to PDF and verify.
	j := testPrintJob(t)
	j.OutputPath = filepath.Join(t.TempDir(), "go-gui-fullpipeline.pdf")
	if err := renderToPDF(w.renderers, j, 800, 600); err != nil {
		t.Fatal(err)
	}
	t.Logf("Full pipeline PDF: %s", j.OutputPath)
	assertPDFExists(t, j.OutputPath)
}

func assertPDFExists(t *testing.T, path string) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("PDF not created: %v", err)
	}
	if info.Size() == 0 {
		t.Fatal("PDF file is empty")
	}
}
