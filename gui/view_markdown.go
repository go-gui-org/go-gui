package gui

// view_markdown.go defines the Markdown view component.
// Parses markdown source and renders it using RTF views.

import (
	"context"
	"log"
	"time"

	"github.com/go-gui-org/go-gui/gui/highlight"
	"github.com/go-gui-org/go-gui/gui/markdown"
)

// MarkdownStyle controls rendered markdown appearance.
type MarkdownStyle struct {
	Text           TextStyle
	H1             TextStyle
	H2             TextStyle
	H3             TextStyle
	H4             TextStyle
	H5             TextStyle
	H6             TextStyle
	Bold           TextStyle
	Italic         TextStyle
	BoldItalic     TextStyle
	Code           TextStyle
	CodeBlockText  TextStyle
	TableHeadStyle TextStyle
	TableCellStyle TextStyle
	// CodeHighlighter, when non-nil, is used to re-tokenize fenced
	// code blocks whose info string names a supported language.
	// When nil, the markdown parser's built-in primitive tokenizer
	// is used. Use highlight.Default() for chroma-backed coverage.
	CodeHighlighter   highlight.Highlighter
	TableRowAlt       *Color
	MathDPIDisplay    int
	MathDPIInline     int
	CodeBlockPadding  Opt[Padding]
	TableCellPadding  Opt[Padding]
	BlockSpacing      float32
	NestIndent        float32
	PrefixCharWidth   float32
	CodeBlockRadius   float32
	TableBorderSize   float32
	CodeBlockBG       Color
	CodeKeywordColor  Color
	CodeStringColor   Color
	CodeNumberColor   Color
	CodeCommentColor  Color
	CodeOperatorColor Color
	CodeTypeColor     Color
	CodeFunctionColor Color
	CodeBuiltinColor  Color
	HRColor           Color
	LinkColor         Color
	BlockquoteBorder  Color
	BlockquoteBG      Color
	TableBorderColor  Color
	HighlightBG       Color
	MermaidBG         Color
	H1Separator       bool
	H2Separator       bool
	TableBorderStyle  TableBorderStyle
	HardLineBreaks    bool
}

// DefaultMarkdownStyle returns a MarkdownStyle using the
// current theme.
func DefaultMarkdownStyle() MarkdownStyle {
	// Line spacing comes from go-glyph's recommended line height (font
	// leading, floored to a minimum em ratio); no manual LineSpacing needed.
	text := guiTheme.N4
	bold := guiTheme.B4
	italic := guiTheme.I4
	bi := guiTheme.BI4

	return MarkdownStyle{
		Text:              text,
		H1:                guiTheme.B1,
		H2:                guiTheme.B2,
		H3:                guiTheme.B3,
		H4:                guiTheme.B4,
		H5:                guiTheme.B5,
		H6:                guiTheme.B6,
		Bold:              bold,
		Italic:            italic,
		BoldItalic:        bi,
		Code:              guiTheme.M5,
		CodeBlockText:     guiTheme.M5,
		CodeBlockBG:       RGBA(0, 0, 0, 50),
		CodeKeywordColor:  guiTheme.ColorSelect,
		CodeStringColor:   RGB(75, 125, 75),
		CodeNumberColor:   RGB(169, 114, 62),
		CodeCommentColor:  guiTheme.ColorBorder,
		CodeOperatorColor: guiTheme.N3.Color,
		CodeTypeColor:     RGB(78, 140, 178),
		CodeFunctionColor: RGB(160, 100, 170),
		CodeBuiltinColor:  RGB(78, 140, 178),
		HRColor:           guiTheme.ColorBorder,
		LinkColor:         guiTheme.ColorSelect,
		BlockquoteBorder:  guiTheme.ColorBorder,
		BlockquoteBG:      RGBA(128, 128, 128, 20),
		BlockSpacing:      12,
		NestIndent:        16,
		PrefixCharWidth:   4,
		CodeBlockPadding:  Some(PadAll(10)),
		CodeBlockRadius:   3.5,
		TableBorderStyle:  TableBorderHeaderOnly,
		TableBorderColor:  guiTheme.ColorBorder,
		TableBorderSize:   1,
		TableHeadStyle:    guiTheme.B4,
		TableCellStyle:    guiTheme.N4,
		TableCellPadding:  SomeP(5, 10, 5, 10),
		HighlightBG:       RGB(199, 142, 18),
		MathDPIDisplay:    150,
		MathDPIInline:     200,
		MermaidBG:         RGBA(248, 248, 255, 255),
	}
}

// MarkdownCfg configures a Markdown View. Mode defaults to wrapped text.
type MarkdownCfg struct {
	ID                  string
	Source              string
	Style               MarkdownStyle
	MermaidWidth        int // max pixel width for mermaid diagrams (0 = 600)
	Padding             Opt[Padding]
	SizeBorder          Opt[float32]
	Radius              Opt[float32]
	IDFocus             uint32
	MinWidth            float32
	Color               Color
	ColorBorder         Color
	Mode                Opt[TextMode]
	Invisible           bool
	Clip                bool
	FocusSkip           bool
	Disabled            bool
	DisableExternalAPIs bool

	// MathFetcher replaces the default CodeCogs renderer.
	// When nil, uses the default CodeCogs endpoint. Ignored
	// when DisableExternalAPIs is true.
	MathFetcher MathFetcher

	// MermaidFetcher replaces the default Kroki renderer.
	// When nil, uses the default Kroki endpoint. Ignored
	// when DisableExternalAPIs is true.
	MermaidFetcher MermaidFetcher
}

// MathFetcher fetches a LaTeX math expression as a PNG image.
// When nil, defaults to the CodeCogs API. The implementation
// must be safe for concurrent use. Ignored when
// DisableExternalAPIs is true.
type MathFetcher func(ctx context.Context, latex string, dpi int,
	fgColor Color) ([]byte, error)

// MermaidFetcher fetches a Mermaid diagram as a PNG image.
// When nil, defaults to the Kroki API. The implementation must
// be safe for concurrent use. Ignored when
// DisableExternalAPIs is true.
type MermaidFetcher func(ctx context.Context, source string) ([]byte, error)

var markdownExternalAPIsEnabled bool

// SetMarkdownExternalAPIsEnabled toggles external markdown API usage
// (CodeCogs/Kroki). Default is disabled.
func SetMarkdownExternalAPIsEnabled(enabled bool) {
	markdownExternalAPIsEnabled = enabled
}

func ensureDiagramCache(w *Window) *BoundedDiagramCache {
	if w.viewState.diagramCache == nil {
		w.viewState.diagramCache =
			NewBoundedDiagramCache(50)
	}
	return w.viewState.diagramCache
}

func nextDiagramRequestID(w *Window) uint64 {
	w.viewState.diagramRequestSeq++
	return w.viewState.diagramRequestSeq
}

func markdownWarnExternalAPIOnce(w *Window) {
	if w.viewState.externalAPIWarningLogged {
		return
	}
	w.viewState.externalAPIWarningLogged = true
	log.Println("markdown: external APIs enabled; " +
		"content may be sent to codecogs.com and kroki.io")
}

func markdownDiagramErrorView(
	errText string, baseStyle TextStyle,
) View {
	errStyle := baseStyle
	errStyle.Color = RGBA(200, 50, 50, 255)
	return Text(TextCfg{
		Text:      errText,
		TextStyle: errStyle,
		Mode:      TextModeWrap,
	})
}

// buildMarkdownTableData converts ParsedTable to
// TableRowCfg array.
func buildMarkdownTableData(
	parsed ParsedTable, _ MarkdownStyle,
) []TableRowCfg {
	rows := make([]TableRowCfg, 0, len(parsed.Rows)+1)

	// Header row.
	hCells := make([]TableCellCfg, 0, len(parsed.Headers))
	for _, h := range parsed.Headers {
		rt := h
		hCells = append(hCells, TableCellCfg{
			Value:    richTextPlain(h),
			HeadCell: true,
			RichText: &rt,
			Content: RTF(RtfCfg{
				RichText: h,
				Mode:     TextModeSingleLine,
			}),
		})
	}
	rows = append(rows, TableRowCfg{Cells: hCells})

	// Data rows.
	for _, r := range parsed.Rows {
		cells := make([]TableCellCfg, 0, len(r))
		for _, cell := range r {
			rt := cell
			cells = append(cells, TableCellCfg{
				Value:    richTextPlain(cell),
				RichText: &rt,
				Content: RTF(RtfCfg{
					RichText: cell,
					Mode:     TextModeSingleLine,
				}),
			})
		}
		rows = append(rows, TableRowCfg{Cells: cells})
	}
	return rows
}

// Markdown creates a markdown view. Method on *Window to
// access viewState for caching.
func (w *Window) Markdown(cfg MarkdownCfg) View {
	if cfg.Invisible {
		return invisibleContainerView()
	}
	mode := cfg.Mode.Get(TextModeWrap)

	// Cache lookup; invalidate on theme change.
	hash := int64(markdown.MathHash(cfg.Source))
	themeName := guiTheme.Name
	if w.viewState.markdownCache == nil ||
		w.viewState.markdownTheme != themeName {
		w.viewState.markdownCache =
			NewBoundedMap[int64, []MarkdownBlock](100)
		w.viewState.markdownTheme = themeName
	}
	blocks, ok := w.viewState.markdownCache.Get(hash)
	if !ok {
		blocks = markdownToBlocks(cfg.Source, cfg.Style)
		w.viewState.markdownCache.Set(hash, blocks)
	}

	allowExternalAPIs := markdownExternalAPIsEnabled &&
		!cfg.DisableExternalAPIs
	if allowExternalAPIs {
		markdownTriggerMathFetches(blocks, cfg, w)
	}

	content := markdownBuildContent(blocks, cfg, mode, w)

	sizing := FitFit
	if mode == TextModeWrap ||
		mode == TextModeWrapKeepSpaces {
		sizing = FillFit
	}

	// Document-level copy button.
	docAnimID := "md_cp_doc"
	source := cfg.Source
	content = append(content, mdCopyButton(docAnimID, w,
		func(_ *Layout, e *Event, w *Window) {
			w.SetClipboard(source)
			w.AnimationAdd(&Animate{
				AnimID:   docAnimID,
				Delay:    2 * time.Second,
				Callback: func(*Animate, *Window) {},
			})
			e.IsHandled = true
		}))

	colCfg := ContainerCfg{
		A11YRole:    AccessRoleGroup,
		Color:       cfg.Color,
		ColorBorder: cfg.ColorBorder,
		SizeBorder:  cfg.SizeBorder,
		Radius:      cfg.Radius,
		Padding:     cfg.Padding,
		Spacing:     Some(cfg.Style.BlockSpacing),
		Sizing:      sizing,
		Content:     content,
	}
	if cfg.IDFocus > 0 {
		colCfg.IDFocus = cfg.IDFocus
		colCfg.FocusSkip = true
		colCfg.AmendLayout = markdownContainerAmendLayout
		colCfg.OnKeyDown = markdownContainerOnKeyDown
	}
	return Column(colCfg)
}

// markdownTriggerMathFetches starts async fetches for inline math
// expressions that are not already cached.
func markdownTriggerMathFetches(
	blocks []MarkdownBlock, cfg MarkdownCfg, w *Window,
) {
	markdownWarnExternalAPIOnce(w)
	inlineCache := ensureDiagramCache(w)
	for _, block := range blocks {
		for _, run := range block.Content.Runs {
			if run.MathID == "" {
				continue
			}
			mhash := diagramCacheHash(run.MathID)
			if _, ok := inlineCache.Get(mhash); ok {
				continue
			}
			if inlineCache.LoadingCount() >=
				maxConcurrentDiagramFetches {
				continue
			}
			reqID := nextDiagramRequestID(w)
			inlineCache.Set(mhash,
				DiagramCacheEntry{
					State:     DiagramLoading,
					RequestID: reqID,
				})
			fetchMathAsync(w, run.MathLatex, mhash,
				reqID, cfg.Style.MathDPIInline,
				cfg.Style.Text.Color, cfg.MathFetcher)
		}
	}
}

// markdownBuildContent converts parsed blocks into views,
// handling list accumulation and blockquote spacing.
func markdownBuildContent(
	blocks []MarkdownBlock, cfg MarkdownCfg,
	mode TextMode, w *Window,
) []View {
	content := make([]View, 0, len(blocks))
	var listItems []View
	prevWasBQ := false
	runeOffset := uint32(0)
	selEnabled := cfg.IDFocus > 0

	makeCtx := func(block MarkdownBlock) *mdBlockCtx {
		if !selEnabled {
			return nil
		}
		ctx := &mdBlockCtx{ID: cfg.IDFocus, Start: runeOffset}
		runeOffset += uint32(rtfRuneCountFromRuns(&block.Content))
		return ctx
	}

	for i, block := range blocks {
		if prevWasBQ && !block.IsBlockquote {
			content = append(content, Rectangle(RectangleCfg{
				Sizing: FillFixed,
				Height: cfg.Style.BlockSpacing,
			}))
		}
		prevWasBQ = block.IsBlockquote

		if !block.IsList && len(listItems) > 0 {
			content = append(content,
				mdFlushListItems(listItems, cfg))
			listItems = nil
		}

		switch {
		case block.IsMath:
			content = append(content, mdRenderMathBlock(block, cfg, w))
		case block.IsCode:
			content = append(content, mdRenderCodeBlock(block, cfg, w, i))
		case block.IsTable:
			if v := mdRenderTable(block, cfg, w, i); v != nil {
				content = append(content, v)
			}
		case block.IsHR:
			content = append(content, mdRenderHR(cfg))
		case block.IsBlockquote:
			content = append(content,
				mdRenderBlockquote(block, cfg, mode, makeCtx(block)))
		case block.IsImage:
			content = append(content, mdRenderImage(block))
		case block.HeaderLevel > 0:
			content = append(content,
				mdRenderHeading(block, cfg, mode, makeCtx(block))...)
		case block.IsDefTerm:
			content = append(content,
				mdRenderDefTerm(block, mode, makeCtx(block)))
		case block.IsDefValue:
			content = append(content,
				mdRenderDefValue(block, cfg, mode, makeCtx(block)))
		case block.IsList:
			listItems = append(listItems,
				mdRenderListItem(block, cfg, mode, makeCtx(block)))
			if i == len(blocks)-1 && len(listItems) > 0 {
				content = append(content,
					mdFlushListItems(listItems, cfg))
				listItems = nil
			}
		default:
			content = append(content,
				mdRenderParagraph(block, cfg, mode, makeCtx(block)))
		}
	}
	return content
}
