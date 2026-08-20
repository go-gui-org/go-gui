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
	h1             TextStyle
	H2             TextStyle
	h3             TextStyle
	h4             TextStyle
	h5             TextStyle
	h6             TextStyle
	Bold           TextStyle
	Italic         TextStyle
	boldItalic     TextStyle
	Code           TextStyle
	codeBlockText  TextStyle
	tableHeadStyle TextStyle
	tableCellStyle TextStyle
	// CodeHighlighter, when non-nil, is used to re-tokenize fenced
	// code blocks whose info string names a supported language.
	// When nil, the markdown parser's built-in primitive tokenizer
	// is used. Use highlight.Default() for chroma-backed coverage.
	CodeHighlighter   highlight.Highlighter
	tableRowAlt       *Color
	mathDPIDisplay    int
	mathDPIInline     int
	codeBlockPadding  Padding
	tableCellPadding  Padding
	blockSpacing      float32
	nestIndent        float32
	prefixCharWidth   float32
	codeBlockRadius   float32
	tableBorderSize   float32
	CodeBlockBG       Color
	codeKeywordColor  Color
	codeStringColor   Color
	codeNumberColor   Color
	codeCommentColor  Color
	codeOperatorColor Color
	codeTypeColor     Color
	codeFunctionColor Color
	codeBuiltinColor  Color
	hRColor           Color
	linkColor         Color
	blockquoteBorder  Color
	blockquoteBG      Color
	tableBorderColor  Color
	highlightBG       Color
	mermaidBG         Color
	h1Separator       bool
	h2Separator       bool
	TableBorderStyle  TableBorderStyle
	hardLineBreaks    bool
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
		Text:          text,
		h1:            guiTheme.B1,
		H2:            guiTheme.B2,
		h3:            guiTheme.B3,
		h4:            guiTheme.B4,
		h5:            guiTheme.B5,
		h6:            guiTheme.B6,
		Bold:          bold,
		Italic:        italic,
		boldItalic:    bi,
		Code:          guiTheme.M5,
		codeBlockText: guiTheme.M5,
		// Non-text fills, exempt from the dimming roles (audit §1.2).
		CodeBlockBG:       RGBA(0, 0, 0, 50), // ergonomics-audit:visual
		codeKeywordColor:  guiTheme.ColorSelect,
		codeStringColor:   RGB(75, 125, 75),
		codeNumberColor:   RGB(169, 114, 62),
		codeCommentColor:  guiTheme.ColorBorder,
		codeOperatorColor: guiTheme.N3.Color,
		codeTypeColor:     RGB(78, 140, 178),
		codeFunctionColor: RGB(160, 100, 170),
		codeBuiltinColor:  RGB(78, 140, 178),
		hRColor:           guiTheme.ColorBorder,
		linkColor:         guiTheme.ColorSelect,
		blockquoteBorder:  guiTheme.ColorBorder,
		blockquoteBG:      RGBA(128, 128, 128, 20), // ergonomics-audit:visual
		// Markdown blocks are paragraphs of one flowing document, not
		// unrelated sections of a surface: Medium is the sibling gap.
		// Large (28) reads as a hole between paragraphs at body 14.
		blockSpacing:     SpacingMedium,
		nestIndent:       16, // structural indent, not a sibling gap — off the ladder
		prefixCharWidth:  4,
		codeBlockPadding: PadAll(10),
		codeBlockRadius:  3.5,
		TableBorderStyle: TableBorderHeaderOnly,
		tableBorderColor: guiTheme.ColorBorder,
		tableBorderSize:  1,
		tableHeadStyle:   guiTheme.B4,
		tableCellStyle:   guiTheme.N4,
		tableCellPadding: NewPadding(5, 10, 5, 10),
		highlightBG:      RGB(199, 142, 18),
		mathDPIDisplay:   150,
		mathDPIInline:    200,
		mermaidBG:        RGBA(248, 248, 255, 255),
	}
}

// MarkdownCfg configures a Markdown View. Mode defaults to wrapped text.
type MarkdownCfg struct {
	ID     string
	Source string
	Style  MarkdownStyle
	// MermaidWidth is the max pixel width for mermaid diagrams
	// (0 = 600).
	// exportaudit:keep — caller-facing config (issue #372)
	MermaidWidth int
	Padding      Padding
	SizeBorder   Opt[float32]
	Radius       Opt[float32]
	Focusable    bool
	MinWidth     float32
	Color        Color
	ColorBorder  Color
	Mode         Opt[textMode]
	Invisible    bool
	Clip         bool
	FocusSkip    bool
	Disabled     bool
	// DisableExternalAPIs blocks CodeCogs/Kroki fetches, so math
	// and mermaid render only when a custom fetcher is installed.
	// exportaudit:keep — caller-facing config (issue #372)
	DisableExternalAPIs bool

	// MathFetcher replaces the default CodeCogs renderer.
	// When nil, uses the default CodeCogs endpoint. Ignored
	// when DisableExternalAPIs is true.
	// exportaudit:keep — caller-facing config (issue #372)
	MathFetcher MathFetcher

	// MermaidFetcher replaces the default Kroki renderer.
	// When nil, uses the default Kroki endpoint. Ignored
	// when DisableExternalAPIs is true.
	// exportaudit:keep — caller-facing config (issue #372)
	MermaidFetcher MermaidFetcher
}

// MathFetcher fetches a LaTeX math expression as a PNG image.
// When nil, defaults to the CodeCogs API. The implementation
// must be safe for concurrent use. Ignored when
// DisableExternalAPIs is true.
// exportaudit:keep — caller-facing config (issue #372)
type MathFetcher func(ctx context.Context, latex string, dpi int,
	fgColor Color) ([]byte, error)

// MermaidFetcher fetches a Mermaid diagram as a PNG image.
// When nil, defaults to the Kroki API. The implementation must
// be safe for concurrent use. Ignored when
// DisableExternalAPIs is true.
// exportaudit:keep — caller-facing config (issue #372)
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
			newBoundedDiagramCache(50)
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
	parsed parsedTable, _ MarkdownStyle,
) []TableRowCfg {
	rows := make([]TableRowCfg, 0, len(parsed.Rows)+1)

	// Header row. Honor per-column alignment (GFM: the delimiter
	// row's alignment applies to header and body alike) instead of
	// the table's default head alignment.
	hCells := make([]TableCellCfg, 0, len(parsed.Headers))
	for i, h := range parsed.Headers {
		rt := h
		cell := TableCellCfg{
			Value:    richTextPlain(h),
			HeadCell: true,
			RichText: &rt,
			Content: RTF(RTFCfg{
				RichText: h,
				Mode:     TextModeSingleLine,
			}),
		}
		if i < len(parsed.Alignments) {
			a := parsed.Alignments[i]
			cell.HAlign = &a
		}
		hCells = append(hCells, cell)
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
				Content: RTF(RTFCfg{
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
	return &markdownView{cfg: cfg}
}

// markdownView is the widget node for a Markdown document. It is a
// struct view (like the splitter, issue #264) because the document has
// several identities that must agree: the container claims cfg.ID, and
// every block references the widget through TC.markdownID — both keyed
// by the *effective* ID, which only exists during generation. The
// factory runs before generation (no ancestor scope yet), so the
// resolution happens here, in GenerateLayout, where w.EffID can see
// the enclosing panel.
type markdownView struct {
	cfg MarkdownCfg
}

func (mv *markdownView) GenerateLayout(w *Window) Layout {
	cfg := &mv.cfg
	// The document's identity is the effective ID: the container claims
	// it, the blocks stamp it into TC.markdownID, and every state slot
	// (nsMdSel, nsMdBlocks) and focus move keys on it. Under an
	// ID-bearing ancestor this resolves to "outer:md", so two documents
	// written with the same leaf under different panels keep separate
	// selections; flat, it is cfg.ID unchanged. The result contains
	// IDSep, which the resolve pass treats as absolute, so the
	// container's effID stays exactly what is stamped here — resolving
	// is idempotent. The overwrite fixes the identity at first
	// generation: a cached instance moved between differently-scoped
	// ancestors keeps the old path (safe here — the view rebuilds its
	// subtree from cfg every frame, and an app that re-roots a document
	// creates a fresh MarkdownCfg).
	cfg.ID = w.EffID(cfg.ID)

	mode := cfg.Mode.Get(TextModeWrap)

	// Cache lookup; invalidate on theme change.
	hash := int64(markdown.MathHash(cfg.Source))
	themeID := guiTheme.id
	if w.viewState.markdownCache == nil ||
		w.viewState.markdownTheme != themeID {
		w.viewState.markdownCache =
			NewBoundedMap[int64, []markdownBlock](100)
		w.viewState.markdownTheme = themeID
	}
	blocks, ok := w.viewState.markdownCache.Get(hash)
	if !ok {
		blocks = markdownToBlocks(cfg.Source, cfg.Style)
		w.viewState.markdownCache.Set(hash, blocks)
	}

	allowExternalAPIs := markdownExternalAPIsEnabled &&
		!cfg.DisableExternalAPIs
	if allowExternalAPIs {
		markdownTriggerMathFetches(blocks, *cfg, w)
	}

	content := markdownBuildContent(blocks, *cfg, mode, w)

	sizing := FitFit
	if mode == TextModeWrap ||
		mode == TextModeWrapKeepSpaces {
		sizing = FillFit
	}

	// Document-level copy button. Scoped to the document: a constant
	// here gave every Markdown widget in a window the same copy button.
	docAnimID := ScopeID(cfg.ID, "copy-doc")
	source := cfg.Source
	content = append(content, mdCopyButton(docAnimID, w,
		func(ctx EventCtx) {
			ctx.Window.SetClipboard(source)
			ctx.Window.AnimationAdd(&Animate{
				AnimID:   docAnimID,
				Delay:    2 * time.Second,
				Callback: func(*Animate, *Window) {},
			})
			ctx.Consume()
		}))

	colCfg := ContainerCfg{
		// The container is the widget: sole claimant of cfg.ID, focus
		// target for document selection, and what FindByID resolves to.
		// Blocks reference it through TC.MarkdownID instead.
		ID:          cfg.ID,
		A11YRole:    AccessRoleGroup,
		Color:       cfg.Color,
		ColorBorder: cfg.ColorBorder,
		SizeBorder:  cfg.SizeBorder,
		Radius:      cfg.Radius,
		Padding:     cfg.Padding,
		Spacing:     Some(cfg.Style.blockSpacing),
		Sizing:      sizing,
		Content:     content,
	}
	if cfg.Focusable {
		colCfg.Focusable = cfg.Focusable
		colCfg.FocusSkip = true
		colCfg.AmendLayout = markdownContainerAmendLayout
		colCfg.OnKeyDown = markdownContainerOnKeyDown
	}
	// Generate the container subtree here so the inner IDs (heading
	// anchors, copy buttons) compose under the resolved scope, exactly
	// as a composite widget's GenerateLayout does.
	return generateViewLayout(Column(colCfg), w)
}

// markdownTriggerMathFetches starts async fetches for inline math
// expressions that are not already cached.
func markdownTriggerMathFetches(
	blocks []markdownBlock, cfg MarkdownCfg, w *Window,
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
					State:     diagramLoading,
					RequestID: reqID,
				})
			fetchMathAsync(w, run.MathLatex, mhash,
				reqID, cfg.Style.mathDPIInline,
				cfg.Style.Text.Color, cfg.MathFetcher)
		}
	}
}

// markdownBuildContent converts parsed blocks into views,
// handling list accumulation and blockquote spacing.
func markdownBuildContent(
	blocks []markdownBlock, cfg MarkdownCfg,
	mode textMode, w *Window,
) []View {
	content := make([]View, 0, len(blocks))
	var listItems []View
	prevWasBQ := false
	runeOffset := uint32(0)
	selEnabled := cfg.Focusable

	// Every block is stamped with the document's identity (markdownID,
	// which anchor resolution keys on), but the per-block rune offsets
	// and selection flag belong to the focusable path only. Non-focusable
	// documents therefore reuse one ctx for all blocks — one allocation
	// per frame per document instead of one per block.
	var sharedCtx *mdBlockCtx
	makeCtx := func(block markdownBlock) *mdBlockCtx {
		if !selEnabled {
			if sharedCtx == nil {
				sharedCtx = &mdBlockCtx{ID: cfg.ID}
			}
			return sharedCtx
		}
		ctx := &mdBlockCtx{ID: cfg.ID, Start: runeOffset, Sel: true}
		runeOffset += uint32(rtfRuneCountFromRuns(&block.Content))
		return ctx
	}

	for i, block := range blocks {
		if prevWasBQ && !block.IsBlockquote {
			// A quote closing gets extra air, but the spacer is a child:
			// it carries a block gap on each side, so a full-height one
			// totals three gaps. Half keeps the emphasis without a hole.
			content = append(content, Rectangle(RectangleCfg{
				Sizing: FillFixed,
				Height: cfg.Style.blockSpacing / 2,
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
