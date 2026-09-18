package gui

// rich_text.go defines rich text types for mixed-style
// paragraphs. These wrap glyph.RichText/StyleRun internally
// while providing a gui-native API.

import (
	"strings"

	"github.com/go-gui-org/go-glyph"
)

// RichTextRun is a styled segment within a RichText block.
type RichTextRun struct {
	Text      string
	Style     TextStyle
	Link      string // URL for hyperlinks
	Tooltip   string // tooltip for abbreviations
	MathID    string // cache key for inline math
	MathLatex string // raw LaTeX source
}

// RichText contains runs of styled text for mixed-style
// paragraphs.
type RichText struct {
	Runs []RichTextRun
}

// RichRun creates a styled text run.
func RichRun(text string, style TextStyle) RichTextRun {
	return RichTextRun{Text: text, Style: style}
}

// RichLink creates a hyperlink run with underline styling. Color and
// underline are both the link role, not caller data: a style passed
// here is for family, size and typeface. Every theme style carries a
// color (Theme.N3 and its siblings come from TextStyleDef), so
// honouring a caller color would silently draw the documented
// RichLink(text, url, t.N3) in plain body color.
//
// A run that must carry its own color is a RichTextRun literal —
// Text, Link and Style are all exported:
//
//	gui.RichTextRun{Text: "docs", Link: url, Style: myStyle}
func RichLink(
	text, url string, style TextStyle,
) RichTextRun {
	s := style
	s.Color = guiTheme.ColorSelect
	s.Underline = true
	return RichTextRun{Text: text, Link: url, Style: s}
}

// RichBr creates a line break run.
func RichBr() RichTextRun {
	return RichTextRun{Text: "\n", Style: guiTheme.N3}
}

// RichAbbr creates an abbreviation run with tooltip. Bold is
// always forced: it is the abbreviation role, not caller data.
func RichAbbr(
	text, expansion string, style TextStyle,
) RichTextRun {
	s := style
	s.Typeface = glyph.TypefaceBold
	return RichTextRun{
		Text: text, Tooltip: expansion, Style: s,
	}
}

// RichFootnote creates a footnote marker with tooltip.
// exportaudit:keep — public constructor matching RichRun/RichLink/RichBr/RichAbbr
func RichFootnote(
	id, content string, baseStyle TextStyle,
) RichTextRun {
	s := baseStyle
	s.Size = baseStyle.Size * 0.7
	return RichTextRun{
		Text:    "\u2009[" + id + "]", // thin space
		Tooltip: content,
		Style:   s,
	}
}

// Math shaping constants. Points per inch converts the diagram
// cache's pixel dimensions (at entry dPI) to points; the reference
// font size anchors the scale so a 12pt run draws at natural size;
// the baseline factor centers the image on the line.
const (
	rtfPointsPerInch = 72.0
	rtfRefFontSize   = 12.0
	rtfBaselineShift = 0.4
)

// rtfMathObject returns the on-line dimensions a math run shapes
// to as an inline object, and whether it shapes as one at all. The
// diagram entry must be ready with usable dimensions, and the run
// needs a positive finite size — otherwise the scale collapses to a
// zero or NaN object and the run falls back to its raw LaTeX
// source. Single source of truth for the object-vs-fallback branch,
// shared by shaping, flat-text building and run lookup so all three
// agree on each run's shaped length.
func rtfMathObject(
	run *RichTextRun, cache *BoundedDiagramCache,
) (objW, objH float32, ok bool) {
	if run == nil || run.MathID == "" || cache == nil {
		return 0, 0, false
	}
	entry, found := cache.Get(diagramCacheHash(run.MathID))
	if !found || entry.State != diagramReady {
		return 0, 0, false
	}
	if !f32IsFinite(entry.dPI) || entry.dPI <= 0 ||
		!f32IsFinite(entry.Width) || entry.Width <= 0 ||
		!f32IsFinite(entry.Height) || entry.Height <= 0 ||
		!f32IsFinite(run.Style.Size) || run.Style.Size <= 0 {
		return 0, 0, false
	}
	// Check the products, not only the inputs. Every input can be
	// finite and positive while the scaled size still overflows
	// float32 to +Inf (a denormal dPI against a large font size) or
	// underflows to zero, and an Inf- or zero-sized object
	// propagates through arrange and render.
	scale := (rtfPointsPerInch / entry.dPI) *
		(run.Style.Size / rtfRefFontSize)
	objW, objH = entry.Width*scale, entry.Height*scale
	if !f32IsFinite(objW) || objW <= 0 ||
		!f32IsFinite(objH) || objH <= 0 {
		return 0, 0, false
	}
	return objW, objH, true
}

// rtfMathReady reports whether a math run shapes as an inline
// object placeholder rather than its LaTeX fallback.
func rtfMathReady(
	run *RichTextRun, cache *BoundedDiagramCache,
) bool {
	_, _, ok := rtfMathObject(run, cache)
	return ok
}

// rtfShapedRunText returns the text a run contributes to the shaped
// glyph layout: the object placeholder for ready math, the raw LaTeX
// fallback otherwise, plain text for non-math runs.
func rtfShapedRunText(
	run *RichTextRun, cache *BoundedDiagramCache,
) string {
	if run.MathID != "" {
		if rtfMathReady(run, cache) {
			return "\uFFFC"
		}
		return run.MathLatex
	}
	return run.Text
}

// toGlyphRichText converts RichText to glyph.RichText.
func (rt RichText) toGlyphRichText() glyph.RichText {
	grt, _ := rt.toGlyphRichTextWithMath(nil)
	return grt
}

// toGlyphRichTextWithMath converts RichText to
// glyph.RichText, emitting InlineObject for math runs
// when the cache has dimensions. Returns the glyph
// RichText and a slice of cache hashes for each inline
// math object (in order), used by renderRtf to bypass
// the unreliable Pango ObjectID round-trip.
func (rt RichText) toGlyphRichTextWithMath(
	cache *BoundedDiagramCache,
) (glyph.RichText, []int64) {
	runs := make([]glyph.StyleRun, 0, len(rt.Runs))
	var mathHashes []int64
	for _, run := range rt.Runs {
		if run.MathID != "" {
			if objW, objH, ok := rtfMathObject(
				&run, cache); ok {
				hash := diagramCacheHash(run.MathID)
				s := run.Style.toGlyphStyle()
				s.Object = &glyph.InlineObject{
					ID:     run.MathID,
					Width:  objW,
					Height: objH,
					// Center vertically on line.
					Offset: run.Style.Size*rtfBaselineShift - objH/2,
				}
				runs = append(runs, glyph.StyleRun{
					Text:  "\uFFFC",
					Style: s,
				})
				mathHashes = append(mathHashes, hash)
				continue
			}
			// Fallback: show raw LaTeX source.
			runs = append(runs, glyph.StyleRun{
				Text:  run.MathLatex,
				Style: run.Style.toGlyphStyle(),
			})
			continue
		}
		runs = append(runs, glyph.StyleRun{
			Text:  run.Text,
			Style: run.Style.toGlyphStyle(),
		})
	}
	return glyph.RichText{Runs: runs}, mathHashes
}

// richTextPlain returns the plain text content of a RichText,
// falling back to MathLatex for math runs with empty Text.
func richTextPlain(rt RichText) string {
	if len(rt.Runs) == 0 {
		return ""
	}
	if len(rt.Runs) == 1 {
		if rt.Runs[0].Text != "" {
			return rt.Runs[0].Text
		}
		return rt.Runs[0].MathLatex
	}
	var sb strings.Builder
	for _, r := range rt.Runs {
		if r.Text != "" {
			sb.WriteString(r.Text)
		} else {
			sb.WriteString(r.MathLatex)
		}
	}
	return sb.String()
}
