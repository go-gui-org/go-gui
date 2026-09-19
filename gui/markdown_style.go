package gui

// markdown_style.go bridges the parser Block/Run types
// to gui-styled MarkdownBlock/RichText types.

import (
	"strings"

	"github.com/go-gui-org/go-glyph"

	"github.com/go-gui-org/go-gui/gui/highlight"
	"github.com/go-gui-org/go-gui/gui/markdown"
)

// mdWithScriptFeature returns a fresh FontFeatures holding base's
// features plus tag. It always allocates: shared state aliased into
// every sup/sub run means a caller mutating one run corrupts all of
// them, so each run gets its own copy.
func mdWithScriptFeature(
	base *glyph.FontFeatures, tag string,
) *glyph.FontFeatures {
	out := &glyph.FontFeatures{}
	if base != nil {
		out.OpenTypeFeatures = append(out.OpenTypeFeatures,
			base.OpenTypeFeatures...)
		out.VariationAxes = append(out.VariationAxes,
			base.VariationAxes...)
	}
	for i := range out.OpenTypeFeatures {
		if out.OpenTypeFeatures[i].Tag == tag {
			out.OpenTypeFeatures[i].Value = 1
			return out
		}
	}
	out.OpenTypeFeatures = append(out.OpenTypeFeatures,
		glyph.FontFeature{Tag: tag, Value: 1})
	return out
}

// markdownToBlocks parses source and returns styled blocks.
func markdownToBlocks(
	source string, style MarkdownStyle,
) []markdownBlock {
	blocks := markdown.Parse(source, style.hardLineBreaks)
	return styleMdBlocks(blocks, style)
}

// MarkdownToRichText parses markdown and returns a single
// RichText (exported for tests).
func markdownToRichText(
	source string, style MarkdownStyle,
) RichText {
	blocks := markdownToBlocks(source, style)
	totalRuns := 0
	for _, block := range blocks {
		totalRuns += len(block.Content.Runs)
	}
	if len(blocks) > 0 {
		totalRuns += len(blocks) - 1
	}
	allRuns := make([]RichTextRun, 0, totalRuns)
	for i, block := range blocks {
		allRuns = append(allRuns, block.Content.Runs...)
		if i < len(blocks)-1 {
			allRuns = append(allRuns, RichBr())
		}
	}
	return RichText{Runs: allRuns}
}

func styleMdBlocks(
	blocks []markdown.Block, style MarkdownStyle,
) []markdownBlock {
	result := make([]markdownBlock, 0, len(blocks))
	for _, b := range blocks {
		result = append(result, styleMdBlock(b, style))
	}
	return result
}

func styleMdBlock(
	block markdown.Block, style MarkdownStyle,
) markdownBlock {
	var baseStyle TextStyle
	switch {
	case block.HeaderLevel > 0:
		baseStyle = mdHeaderStyle(block.HeaderLevel, style)
	case block.IsDefTerm:
		baseStyle = style.Bold
	default:
		baseStyle = style.Text
	}

	mb := markdownBlock{
		HeaderLevel:     block.HeaderLevel,
		IsCode:          block.IsCode,
		IsHR:            block.IsHR,
		IsBlockquote:    block.IsBlockquote,
		IsImage:         block.IsImage,
		IsTable:         block.IsTable,
		IsList:          block.IsList,
		IsMath:          block.IsMath,
		IsDefTerm:       block.IsDefTerm,
		IsDefValue:      block.IsDefValue,
		IsTaskItem:      block.IsTaskItem,
		TaskChecked:     block.TaskChecked,
		BlockquoteDepth: block.BlockquoteDepth,
		ListPrefix:      block.ListPrefix,
		ListIndent:      block.ListIndent,
		ImageSrc:        block.ImageSrc,
		ImageAlt:        block.ImageAlt,
		ImageWidth:      block.ImageWidth,
		ImageHeight:     block.ImageHeight,
		CodeLanguage:    block.CodeLanguage,
		MathLatex:       block.MathLatex,
		AnchorSlug:      block.AnchorSlug,
		baseStyle:       baseStyle,
		Content:         styleMdRuns(block.Runs, baseStyle, style),
	}

	// Code blocks use a smaller font than inline code.
	if block.IsCode {
		sz := style.codeBlockText.Size
		for i := range mb.Content.Runs {
			mb.Content.Runs[i].Style.Size = sz
		}
	}

	// Fenced code block: replace parser's primitive tokenization
	// with the configured Highlighter when available.
	if block.IsCode && style.CodeHighlighter != nil &&
		block.CodeLanguage != "" {
		if runs := highlightCodeBlock(
			mb.Content.Runs, block.CodeLanguage, &style,
		); runs != nil {
			mb.Content.Runs = runs
		}
	}

	if block.TableData != nil {
		td := styleMdTable(*block.TableData, style)
		mb.TableData = &td
	}
	return mb
}

func mdHeaderStyle(
	level int, style MarkdownStyle,
) TextStyle {
	switch level {
	case 1:
		return style.H1
	case 2:
		return style.H2
	case 3:
		return style.H3
	case 4:
		return style.H4
	case 5:
		return style.H5
	default:
		return style.H6
	}
}

func styleMdRuns(
	runs []markdown.Run, base TextStyle, style MarkdownStyle,
) RichText {
	styled := make([]RichTextRun, 0, len(runs))
	for _, r := range runs {
		styled = append(styled,
			styleMdRun(r, base, style))
	}
	return RichText{Runs: styled}
}

func styleMdRun(
	run markdown.Run, base TextStyle, style MarkdownStyle,
) RichTextRun {
	s := mdFormatToStyle(run.Format, base, style)

	// Code token coloring.
	if run.Format == markdown.FormatCode &&
		run.CodeToken != markdown.TokenPlain {
		s = mdCodeTokenStyle(run.CodeToken, base, style)
	}

	if run.Strikethrough {
		s.Strikethrough = true
	}
	if run.Highlight {
		s.BgColor = style.highlightBG
	}
	// Script runs keep the base size and only gain their feature tag:
	// the shaper derives 0.58x size and baseline shift from the run
	// size, so scaling here would double-shrink and under-shift.
	if run.Superscript {
		s.Features = mdWithScriptFeature(s.Features, "sups")
	}
	if run.Subscript {
		s.Features = mdWithScriptFeature(s.Features, "subs")
	}
	if run.Underline {
		s.Underline = true
	}
	if run.Link != "" {
		s.Color = style.linkColor
		s.Underline = true
	}

	// Footnote marker — reduce size.
	if run.Tooltip != "" && run.Link == "" &&
		strings.HasPrefix(run.Text, "\u2009[") {
		s.Size *= 0.7
	}

	// Abbreviation — bold typeface.
	if run.Tooltip != "" && run.Link == "" &&
		!strings.HasPrefix(run.Text, "\u2009[") {
		s.Typeface = glyph.TypefaceBold
	}

	return RichTextRun{
		Text:      run.Text,
		Style:     s,
		Link:      run.Link,
		Tooltip:   run.Tooltip,
		MathID:    run.MathID,
		MathLatex: run.MathLatex,
	}
}

func mdFormatToStyle(
	f markdown.Format, base TextStyle, style MarkdownStyle,
) TextStyle {
	// Bold and italic runs keep the style face and take the base
	// geometry: inside a heading a bold run stays heading-sized and
	// heading-colored. The base color wins only outside body text,
	// so a custom Bold color still applies to body runs.
	switch f {
	case markdown.FormatBold:
		s := style.Bold
		mdInheritBaseGeometry(&s, base, style)
		return s
	case markdown.FormatItalic:
		s := style.Italic
		mdInheritBaseGeometry(&s, base, style)
		return s
	case markdown.FormatBoldItalic:
		s := style.BoldItalic
		mdInheritBaseGeometry(&s, base, style)
		return s
	case markdown.FormatCode:
		s := style.Code
		s.Typeface = glyph.TypefaceBold
		mdInheritBaseGeometry(&s, base, style)
		return s
	default:
		return base
	}
}

// mdInheritBaseGeometry folds a base style's geometry into a formatted
// run: size always (guarded against zero), background when set, color
// only outside body text. The guards keep a custom style face intact
// for body runs while headings keep their own size and color.
func mdInheritBaseGeometry(
	s *TextStyle, base TextStyle, style MarkdownStyle,
) {
	if base.Size != 0 && base.Size != style.Text.Size {
		s.Size = base.Size
	}
	if base.BgColor.IsSet() {
		s.BgColor = base.BgColor
	}
	if base.Color.IsSet() && base.Color != style.Text.Color {
		s.Color = base.Color
	}
}

// maxHighlightBytes bounds the text handed to CodeHighlighter: a
// hostile fenced block otherwise spends unbounded time and memory in
// the highlighter. Over the cap the caller keeps the parser's own
// runs, which are already tokenized.
const maxHighlightBytes = 1 << 16

// highlightCodeBlock re-tokenizes a fenced code block's text using
// style.CodeHighlighter. Returns nil on failure so the caller keeps
// the parser's fallback runs. Base font/size come from the existing
// run; color is assigned per token Kind.
func highlightCodeBlock(
	existing []RichTextRun, lang string, style *MarkdownStyle,
) []RichTextRun {
	if len(existing) == 0 {
		return nil
	}
	total := 0
	for _, r := range existing {
		total += len(r.Text)
	}
	if total > maxHighlightBytes {
		return nil
	}
	var src strings.Builder
	src.Grow(total)
	for _, r := range existing {
		src.WriteString(r.Text)
	}
	toks := style.CodeHighlighter.Tokenize(lang, src.String())
	if len(toks) == 0 {
		return nil
	}
	base := existing[0].Style
	base.Color = style.codeOperatorColor
	out := make([]RichTextRun, len(toks))
	for i, tk := range toks {
		s := base
		s.Color = colorForKind(tk.Kind, style)
		out[i] = RichTextRun{Text: tk.Text, Style: s}
	}
	return out
}

func colorForKind(k highlight.Kind, style *MarkdownStyle) Color {
	switch k {
	case highlight.KindKeyword:
		return style.codeKeywordColor
	case highlight.KindString:
		return style.codeStringColor
	case highlight.KindNumber:
		return style.codeNumberColor
	case highlight.KindComment:
		return style.codeCommentColor
	case highlight.KindOperator, highlight.KindPunctuation:
		return style.codeOperatorColor
	case highlight.KindType:
		return style.codeTypeColor
	case highlight.KindFunction:
		return style.codeFunctionColor
	case highlight.KindBuiltin:
		return style.codeBuiltinColor
	}
	return style.codeOperatorColor
}

func mdCodeTokenStyle(
	kind markdown.CodeTokenKind, base TextStyle, style MarkdownStyle,
) TextStyle {
	s := style.Code
	switch kind {
	case markdown.TokenKeyword:
		s.Color = style.codeKeywordColor
	case markdown.TokenString:
		s.Color = style.codeStringColor
	case markdown.TokenNumber:
		s.Color = style.codeNumberColor
	case markdown.TokenComment:
		s.Color = style.codeCommentColor
	case markdown.TokenOperator:
		s.Color = style.codeOperatorColor
	}
	// Token colors are semantic and stay; the size follows the base
	// outside body text, the same rule mdInheritBaseGeometry applies.
	if base.Size != 0 && base.Size != style.Text.Size {
		s.Size = base.Size
	}
	return s
}

func styleMdTable(
	table markdown.Table, style MarkdownStyle,
) parsedTable {
	headers := make([]RichText, 0, len(table.Headers))
	for _, h := range table.Headers {
		headers = append(headers,
			styleMdRuns(h, style.tableHeadStyle, style))
	}

	rows := make([][]RichText, 0, len(table.Rows))
	for _, row := range table.Rows {
		sr := make([]RichText, table.ColCount)
		for j, cell := range row {
			// GFM drops cells past the delimiter column count;
			// short rows keep zero (empty) cells from the make.
			if j < table.ColCount {
				sr[j] = styleMdRuns(cell, style.tableCellStyle, style)
			}
		}
		rows = append(rows, sr)
	}

	return parsedTable{
		Headers:    headers,
		Alignments: mdAlignsToHAligns(table.Alignments),
		Rows:       rows,
	}
}

func mdAlignsToHAligns(aligns []markdown.Align) []HorizontalAlign {
	result := make([]HorizontalAlign, len(aligns))
	for i, a := range aligns {
		result[i] = mdAlignToHAlign(a)
	}
	return result
}

func mdAlignToHAlign(a markdown.Align) HorizontalAlign {
	switch a {
	case markdown.AlignEnd:
		return HAlignEnd
	case markdown.AlignCenter:
		return HAlignCenter
	case markdown.AlignLeft:
		return HAlignLeft
	case markdown.AlignRight:
		return HAlignRight
	default:
		return HAlignStart
	}
}
