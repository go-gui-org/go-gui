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

// RichLink creates a hyperlink run with underline styling.
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

// RichAbbr creates an abbreviation run with tooltip.
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
			hash := diagramCacheHash(run.MathID)
			if cache != nil {
				if entry, ok := cache.Get(hash); ok &&
					entry.State == DiagramReady &&
					entry.DPI > 0 {
					scale := (72.0 / entry.DPI) *
						(run.Style.Size / 12.0)
					s := run.Style.ToGlyphStyle()
					h := entry.Height * scale
					s.Object = &glyph.InlineObject{
						ID:     run.MathID,
						Width:  entry.Width * scale,
						Height: h,
						// Center vertically on line.
						Offset: run.Style.Size*0.4 - h/2,
					}
					runs = append(runs, glyph.StyleRun{
						Text:  "\uFFFC",
						Style: s,
					})
					mathHashes = append(mathHashes, hash)
					continue
				}
			}
			// Fallback: show raw LaTeX source.
			runs = append(runs, glyph.StyleRun{
				Text:  run.MathLatex,
				Style: run.Style.ToGlyphStyle(),
			})
			continue
		}
		runs = append(runs, glyph.StyleRun{
			Text:  run.Text,
			Style: run.Style.ToGlyphStyle(),
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
