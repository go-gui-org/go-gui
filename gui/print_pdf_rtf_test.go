package gui

import (
	"bytes"
	"math"
	"os"
	"regexp"
	"strconv"
	"testing"

	"github.com/go-gui-org/go-glyph"
)

// pdfTextSizes maps each string a page shows (Tj, as written in the
// PDF, so parentheses are escaped) to the font size in force for it.
func pdfTextSizes(content string) map[string]float64 {
	re := regexp.MustCompile(`([0-9.]+) Tf|\(((?:[^()\\]|\\.)*)\) Tj`)
	sizes := map[string]float64{}
	size := 0.0
	for _, m := range re.FindAllStringSubmatch(content, -1) {
		if m[1] != "" {
			size, _ = strconv.ParseFloat(m[1], 64)
			continue
		}
		sizes[m[2]] = size
	}
	return sizes
}

// Every platform's default MonoFontFamily must print in Courier: a
// monospace run drawn in a proportional font is fitted token by token
// to its width and comes out at mixed sizes.
func TestPDFFontNameDefaultMonoFamilies(t *testing.T) {
	for _, family := range []string{"monospace", "Menlo", "Monaco", "Consolas"} {
		if got := pdfFontName(family); got != "Courier" {
			t.Errorf("%s -> %q, want Courier", family, got)
		}
	}
}

// A rich-text run keeps its own font and size. A markdown code block's
// runs are monospace on a line whose ascent is larger than their font.
// Drawn in Helvetica, sized from that ascent and then fitted to each
// run's width, a narrow token such as "if" came out much larger than its
// neighbours and ran into the lines above and below.
func TestRenderToPDF_RTFRunsUseTheirOwnFont(t *testing.T) {
	j := testPrintJob(t)
	mono := glyph.TextStyle{FontName: "Consolas", Size: 12}
	black := glyph.Color{A: 255}
	// Consolas advances 0.55 em: 6.6 px per glyph at 12 px.
	layout := &glyph.Layout{
		Text: "if (x)",
		Items: []glyph.Item{
			{Style: mono, StartIndex: 0, Length: 2, X: 0, Y: 20,
				Width: 13.2, Ascent: 18, Descent: 5, Color: black},
			{Style: mono, StartIndex: 3, Length: 3, X: 19.8, Y: 20,
				Width: 19.8, Ascent: 18, Descent: 5, Color: black},
		},
		Width:  200,
		Height: 30,
	}
	cmds := []RenderCmd{{Kind: RenderRTF, X: 10, Y: 10, LayoutPtr: layout}}
	if err := renderToPDF(cmds, j, 800, 600); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(j.OutputPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(data, []byte("/BaseFont /Courier")) {
		t.Error("the monospace runs were not drawn in Courier")
	}
	sizes := pdfTextSizes(pdfPageContent(t, j.OutputPath))
	ifSize, parenSize := sizes["if"], sizes[`\(x\)`]
	if ifSize == 0 || parenSize == 0 {
		t.Fatalf("runs not found in the page: %v", sizes)
	}
	if math.Abs(ifSize-parenSize) > 0.01*parenSize {
		t.Errorf("runs of one monospace style print at %.2f and %.2f pt, want one size",
			ifSize, parenSize)
	}
}
