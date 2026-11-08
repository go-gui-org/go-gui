package gui

import (
	"strings"
	"testing"
)

// A huge but finite length under a page scale above 1 overflowed when it
// was scaled in float32, and fpdf wrote the resulting Inf into the page,
// which PDF readers reject. A 10 px source viewport fits to the page at
// about 18 mm per pixel.
func TestRenderToPDF_HugeGeometryStaysFinite(t *testing.T) {
	j := testPrintJob(t)
	const huge = 3e38
	ink := RGB(1, 2, 3)
	cmds := []RenderCmd{
		{Kind: RenderRect, X: 1, Y: 1, W: huge, H: 2, Color: ink},
		{Kind: RenderRect, X: 1, Y: 4, W: huge, H: 2, Color: ink, Radius: huge},
		{Kind: RenderStrokeRect, X: 1, Y: 7, W: 2, H: 2, Color: ink,
			Thickness: huge, Radius: 1},
		{Kind: RenderLine, X: huge, Y: 1, OffsetX: 1, Color: ink, Thickness: 1},
		{Kind: RenderText, X: huge, Y: 1, Color: ink, Text: "x",
			FontName: "sans", FontSize: 12},
	}
	if err := renderToPDF(cmds, j, 10, 10); err != nil {
		t.Fatal(err)
	}
	content := pdfPageContent(t, j.OutputPath)
	for _, bad := range []string{"Inf", "NaN"} {
		if n := strings.Count(content, bad); n > 0 {
			t.Errorf("page content holds %s %d times", bad, n)
		}
	}
}
