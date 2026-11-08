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

// pdfShownFonts maps each string a page shows (Tj, escaped as in the
// PDF) to the base font it is shown in. The page selects fonts by
// resource name (/F<id> size Tf); its /Font resources map each name to
// an object, and each font object names its /BaseFont.
func pdfShownFonts(t *testing.T, path string) map[string]string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	objects := map[string]string{} // object number → base font
	objRe := regexp.MustCompile(`(?m)^([0-9]+) 0 obj\s*<</Type /Font\s*/BaseFont /(\S+)`)
	for _, m := range objRe.FindAllSubmatch(data, -1) {
		objects[string(m[1])] = string(m[2])
	}
	resources := map[string]string{} // resource name → base font
	resRe := regexp.MustCompile(`/(F[0-9a-f]+) ([0-9]+) 0 R`)
	for _, m := range resRe.FindAllSubmatch(data, -1) {
		resources[string(m[1])] = objects[string(m[2])]
	}
	shown := map[string]string{}
	font := ""
	re := regexp.MustCompile(`/(F[0-9a-f]+) [0-9.]+ Tf|\(((?:[^()\\]|\\.)*)\) Tj`)
	for _, m := range re.FindAllStringSubmatch(pdfPageContent(t, path), -1) {
		if m[1] != "" {
			font = resources[m[1]]
			continue
		}
		shown[m[2]] = font
	}
	return shown
}

// Each rich-text run prints in the PDF font closest to its own: family,
// weight from the typeface or a heavy family name, slant. A plain layout
// takes its font from the command's TextStyle.
func TestRenderToPDF_RunFonts(t *testing.T) {
	j := testPrintJob(t)
	run := func(text string, start int, font string, face glyph.Typeface) glyph.Item {
		return glyph.Item{
			Style:      glyph.TextStyle{FontName: font, Typeface: face, Size: 12},
			StartIndex: start, Length: len(text), X: float64(start) * 7, Y: 20,
			Width: float64(len(text)) * 7, Ascent: 12, Descent: 4,
			Color: glyph.Color{A: 255},
		}
	}
	const text = "code plain strong emph heavy"
	rich := &glyph.Layout{
		Text: text,
		Items: []glyph.Item{
			run("code", 0, "Consolas", glyph.TypefaceRegular),
			run("plain", 5, "Segoe UI", glyph.TypefaceRegular),
			run("strong", 11, "Segoe UI", glyph.TypefaceBold),
			run("emph", 18, "Segoe UI", glyph.TypefaceItalic),
			run("heavy", 23, "Arial Black", glyph.TypefaceRegular),
		},
		Width:  300,
		Height: 30,
	}
	menlo := &glyph.Layout{
		Text:   "menlo",
		Items:  []glyph.Item{run("menlo", 0, "", glyph.TypefaceRegular)},
		Width:  100,
		Height: 30,
	}
	ts := TextStyle{Family: "Menlo", Size: 12}
	cmds := []RenderCmd{
		{Kind: RenderRTF, X: 10, Y: 10, LayoutPtr: rich},
		{Kind: RenderLayout, X: 10, Y: 60, LayoutPtr: menlo, TextStylePtr: &ts},
	}
	if err := renderToPDF(cmds, j, 800, 600); err != nil {
		t.Fatal(err)
	}
	shown := pdfShownFonts(t, j.OutputPath)
	for run, want := range map[string]string{
		"code":   "Courier",
		"plain":  "Helvetica",
		"strong": "Helvetica-Bold",
		"emph":   "Helvetica-Oblique",
		"heavy":  "Helvetica-Bold",
		"menlo":  "Courier",
	} {
		if got := shown[run]; got != want {
			t.Errorf("%q printed in %q, want %q", run, got, want)
		}
	}
}

// A heavy weight is read as glyph reads font names: a "Heavy" among the
// trailing style keywords, or a family ending in "Black" once they are
// stripped; never a name's only word. "Black And White Picture" and
// "Blackadder ITC" are regular faces.
func TestPDFFontStyleHeavyFamilies(t *testing.T) {
	for family, want := range map[string]string{
		"Arial Black":              "B",
		"Arial Black 12":           "B",
		"Arial Black Italic":       "BI",
		"Inter-Heavy":              "B",
		"Inter Heavy Italic":       "BI",
		"Custom Heavy Condensed":   "B",
		"Custom Black Condensed":   "B",
		"Blackadder ITC":           "",
		"Black And White Picture":  "",
		"Black":                    "",
		"Heavy":                    "",
		"Segoe UI":                 "",
		"Segoe UI Light Condensed": "",
	} {
		ts := TextStyle{Family: family}
		if got := pdfFontStyle(&ts); got != want {
			t.Errorf("%s -> %q, want %q", family, got, want)
		}
	}
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
