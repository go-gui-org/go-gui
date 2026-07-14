package gui

// markdown_style.go bridges the parser Block/Run types
// to gui-styled MarkdownBlock/RichText types.

import (
	"strings"

	"github.com/go-gui-org/go-glyph"

	"github.com/go-gui-org/go-gui/gui/highlight"
	"github.com/go-gui-org/go-gui/gui/markdown"
)

var mdSuperscriptFeatures = &glyph.FontFeatures{
	OpenTypeFeatures: []glyph.FontFeature{
		{Tag: "sups", Value: 1},
	},
}

var mdSubscriptFeatures = &glyph.FontFeatures{
	OpenTypeFeatures: []glyph.FontFeature{
		{Tag: "subs", Value: 1},
	},
}

// markdownToBlocks parses source and returns styled blocks.
func markdownToBlocks(
	source string, style MarkdownStyle,
) []MarkdownBlock {
	blocks := markdown.Parse(source, style.HardLineBreaks)
	return styleMdBlocks(blocks, style)
}

// MarkdownToRichText parses markdown and returns a single
// RichText (exported for tests).
func MarkdownToRichText(
	source string, style MarkdownStyle,
) RichText {
	blocks := markdownToBlocks(source, style)
	totalRuns := 0
	for _, block := range blocks {
		totalRuns += len(block.Content.Runs)
	}
	if n := len(blocks) - 1; n > 0 {
		totalRuns += n
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
) []MarkdownBlock {
	result := make([]MarkdownBlock, 0, len(blocks))
	for _, b := range blocks {
		result = append(result, styleMdBlock(b, style))
	}
	return result
}

func styleMdBlock(
	block markdown.Block, style MarkdownStyle,
) MarkdownBlock {
	var baseStyle TextStyle
	switch {
	case block.HeaderLevel > 0:
		baseStyle = mdHeaderStyle(block.HeaderLevel, style)
	case block.IsDefTerm:
		baseStyle = style.Bold
	default:
		baseStyle = style.Text
	}

	mb := MarkdownBlock{
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
		BaseStyle:       baseStyle,
		Content:         styleMdRuns(block.Runs, baseStyle, style),
	}

	// Code blocks use a smaller font than inline code.
	if block.IsCode {
		sz := style.CodeBlockText.Size
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
		s = mdCodeTokenStyle(run.CodeToken, style)
	}

	if run.Strikethrough {
		s.Strikethrough = true
	}
	if run.Highlight {
		s.BgColor = style.HighlightBG
	}
	if run.Superscript {
		s.Size *= 1.2
		s.Features = mdSuperscriptFeatures
	}
	if run.Subscript {
		s.Size *= 1.2
		s.Features = mdSubscriptFeatures
	}
	if run.Underline {
		s.Underline = true
	}
	if run.Link != "" {
		s.Color = style.LinkColor
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
	switch f {
	case markdown.FormatBold:
		s := style.Bold
		s.Size = base.Size
		s.BgColor = base.BgColor
		return s
	case markdown.FormatItalic:
		s := style.Italic
		s.Size = base.Size
		s.BgColor = base.BgColor
		return s
	case markdown.FormatBoldItalic:
		s := style.BoldItalic
		s.Size = base.Size
		s.BgColor = base.BgColor
		return s
	case markdown.FormatCode:
		s := style.Code
		s.Typeface = glyph.TypefaceBold
		return s
	default:
		return base
	}
}

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
	base.Color = style.CodeOperatorColor
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
		return style.CodeKeywordColor
	case highlight.KindString:
		return style.CodeStringColor
	case highlight.KindNumber:
		return style.CodeNumberColor
	case highlight.KindComment:
		return style.CodeCommentColor
	case highlight.KindOperator, highlight.KindPunctuation:
		return style.CodeOperatorColor
	case highlight.KindType:
		return style.CodeTypeColor
	case highlight.KindFunction:
		return style.CodeFunctionColor
	case highlight.KindBuiltin:
		return style.CodeBuiltinColor
	}
	return style.CodeOperatorColor
}

func mdCodeTokenStyle(
	kind markdown.CodeTokenKind, style MarkdownStyle,
) TextStyle {
	s := style.Code
	switch kind {
	case markdown.TokenKeyword:
		s.Color = style.CodeKeywordColor
	case markdown.TokenString:
		s.Color = style.CodeStringColor
	case markdown.TokenNumber:
		s.Color = style.CodeNumberColor
	case markdown.TokenComment:
		s.Color = style.CodeCommentColor
	case markdown.TokenOperator:
		s.Color = style.CodeOperatorColor
	}
	return s
}

func styleMdTable(
	table markdown.Table, style MarkdownStyle,
) ParsedTable {
	headers := make([]RichText, 0, len(table.Headers))
	for _, h := range table.Headers {
		headers = append(headers,
			styleMdRuns(h, style.TableHeadStyle, style))
	}

	rows := make([][]RichText, 0, len(table.Rows))
	for _, row := range table.Rows {
		sr := make([]RichText, table.ColCount)
		for j, cell := range row {
			if j < table.ColCount {
				sr[j] = styleMdRuns(cell, style.TableCellStyle, style)
			}
		}
		rows = append(rows, sr)
	}

	return ParsedTable{
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
