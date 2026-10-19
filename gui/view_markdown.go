package gui

// view_markdown.go defines the Markdown view component.
// Parses markdown source and renders it using RTF views.

import (
	"context"
	"log"
	"sync/atomic"
	"time"

	"github.com/go-gui-org/go-gui/gui/highlight"
)

// MarkdownStyle controls rendered markdown appearance.
//
// The header (H1..H6) and emphasis (Bold, Italic, BoldItalic) faces
// form closed grids: every cell is exported because a half-exported
// grid is a broken vocabulary, the same rule the Theme N/B/I/BI/M
// ladders follow. The remaining fields stay structural — block and
// table geometry the renderer sizes, not caller vocabulary.
type MarkdownStyle struct {
	Text TextStyle
	// exportaudit:keep — closed grid, exported for completeness (see above).
	H1, H2, H3, H4, H5, H6 TextStyle
	Bold                   TextStyle
	Italic                 TextStyle
	// exportaudit:keep — closed grid, exported for completeness (see above).
	BoldItalic     TextStyle
	Code           TextStyle
	codeBlockText  TextStyle
	tableHeadStyle TextStyle
	tableCellStyle TextStyle
	// CodeHighlighter, when non-nil, is used to re-tokenize fenced
	// code blocks whose info string names a supported language.
	// When nil, the markdown parser's built-in primitive tokenizer
	// is used. Use highlight.Default() for chroma-backed coverage.
	CodeHighlighter  highlight.Highlighter
	tableRowAlt      *Color
	mathDPIDisplay   int
	mathDPIInline    int
	codeBlockPadding Padding
	tableCellPadding Padding
	blockSpacing     float32
	nestIndent       float32
	prefixCharWidth  float32
	codeBlockRadius  float32
	tableBorderSize  float32
	// exportaudit:keep — public style field; app authors override it per theme
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
	//
	// Markdown is document prose under headings, so paragraphs take
	// the secondary role: headings stay bright while body reads
	// quieter. The heading ladder has only two rungs above body, so
	// H3 (title-small) and H4 (body bold) share a size and differ by
	// nothing but order; H5 and H6 take the two rungs under body.
	text := guiTheme.TextStyleSecondary
	bold := text.Bold()
	italic := text.Italic()
	bi := text.Italic().Bold()
	// H4 and table heads stay bright: they name sections, not body.
	headBold := guiTheme.TextStyleBody.Bold()
	// Inline code sits two points under the body size (about the 85%
	// GitHub uses). The mono face alone sets it apart. Menlo (the macOS
	// mono) has a taller x-height than the system sans, so at the body
	// size, and more so at the code role's +1, each identifier in a
	// paragraph looked larger than the words around it.
	code := guiTheme.TextStyleCode
	code.Size = text.Size - 2 // ergonomics-audit:visual — mono x-height match, see above

	pal := markdownCodePaletteFor(&guiTheme)

	return MarkdownStyle{
		Text:       text,
		H1:         guiTheme.TextStyleDisplay,
		H2:         guiTheme.TextStyleTitle,
		H3:         guiTheme.TextStyleTitleSmall,
		H4:         headBold,
		H5:         guiTheme.TextStyleBodySmall.Bold(),
		H6:         guiTheme.TextStyleCaption.Bold(),
		Bold:       bold,
		Italic:     italic,
		BoldItalic: bi,
		// Fenced blocks stay on the small code role: they hold long
		// lines, and the smaller face wraps fewer of them.
		Code:          code,
		codeBlockText: guiTheme.TextStyleCodeSmall,
		// Non-text fills, exempt from the dimming roles (audit §1.2).
		CodeBlockBG:       pal.wash,
		codeKeywordColor:  pal.keyword,
		codeStringColor:   pal.str,
		codeNumberColor:   pal.number,
		codeCommentColor:  guiTheme.TextStyleSecondary.Color,
		codeOperatorColor: guiTheme.TextStyleBody.Color,
		codeTypeColor:     pal.typ,
		codeFunctionColor: pal.function,
		codeBuiltinColor:  pal.typ,
		hRColor:           guiTheme.ColorBorder,
		linkColor:         guiTheme.TextStyleLink.Color,
		blockquoteBorder:  guiTheme.ColorBorder,
		blockquoteBG:      RGBA(128, 128, 128, 20), // ergonomics-audit:visual
		// Markdown blocks are paragraphs of one flowing document, not
		// unrelated sections of a surface: Medium is the sibling gap.
		// Large (28) reads as a hole between paragraphs at body 14.
		blockSpacing:     guiTheme.SpacingMedium,
		nestIndent:       16, // structural indent, not a sibling gap — off the ladder
		prefixCharWidth:  4,
		codeBlockPadding: PadAll(10), // ergonomics-audit:spacing — fixed style default, theme-invariant
		codeBlockRadius:  3.5,
		TableBorderStyle: TableBorderHeaderOnly,
		tableBorderColor: guiTheme.ColorBorder,
		tableBorderSize:  1,
		tableHeadStyle:   headBold,
		tableCellStyle:   text,
		tableCellPadding: NewPadding(5, 10, 5, 10), // ergonomics-audit:spacing — fixed style default, theme-invariant
		highlightBG:      RGB(199, 142, 18),
		mathDPIDisplay:   150,
		mathDPIInline:    200,
		mermaidBG:        RGBA(248, 248, 255, 255),
	}
}

// markdownCodePalette is the code fill and the syntax colors derived
// from one theme. Deriving the syntax colors walks lightness in OKLCH
// (readableOn) and costs about 12µs, while DefaultMarkdownStyle runs
// every frame for each document without its own style. So the result
// is kept for the last theme id. A theme fade gives each blended frame
// a new id, so a fade recomputes once per frame and never reads a
// stale palette.
type markdownCodePalette struct {
	themeID  uint64
	wash     Color
	keyword  Color
	str      Color
	number   Color
	typ      Color
	function Color
}

var markdownCodePaletteCache atomic.Pointer[markdownCodePalette]

// markdownCodePaletteFor returns the palette for t, from the cache when
// t is the theme it was built for. A miss allocates one palette.
func markdownCodePaletteFor(t *Theme) *markdownCodePalette {
	if p := markdownCodePaletteCache.Load(); p != nil && p.themeID == t.id {
		return p
	}
	// Syntax colors keep their hue but move on lightness until they
	// read as body text (4.5:1) on the code fill, over both surfaces a
	// document sits on. The base hues are mid-tones: as they were, they
	// fell to 2.5..4.4 on every preset, light and dark.
	wash := markdownCodeWash(t)
	fills := [2]Color{wash.Over(t.ColorBackground), wash.Over(t.ColorPanel)}
	syntax := func(c Color) Color {
		return readableOn(c, contrastText, fills[:]...)
	}
	p := &markdownCodePalette{
		themeID:  t.id,
		wash:     wash,
		keyword:  syntax(t.ColorSelect),
		str:      syntax(RGB(75, 125, 75)),
		number:   syntax(RGB(169, 114, 62)),
		typ:      syntax(RGB(78, 140, 178)),
		function: syntax(RGB(160, 100, 170)),
	}
	markdownCodePaletteCache.Store(p)
	return p
}

// markdownCodeWash is the fill under code: a fenced block and inline
// code. It follows the theme's polarity, the same comparison
// textRolesFor makes. A light theme takes a faint dark wash and a dark
// theme a faint light one, so code lifts off the page by about the same
// small step on both (contrast near 1.1 against the page). One black
// wash for both read as a gray slab on light pages (1.58) and was near
// invisible on dark ones (1.05).
func markdownCodeWash(t *Theme) Color {
	if srgbLuminance(t.TextStyleBody.Color) <
		srgbLuminance(t.ColorBackground) {
		return RGBA(0, 0, 0, 12) // ergonomics-audit:visual
	}
	return RGBA(255, 255, 255, 10) // ergonomics-audit:visual
}

// MarkdownCfg configures a Markdown View. Mode defaults to wrapped text.
type MarkdownCfg struct {
	ID     string
	Source string
	// Style sets the document's faces and block geometry. Leave it unset
	// (zero Text.Size) for DefaultMarkdownStyle() under the active theme.
	// A zero Text.Size discards every other field except CodeHighlighter.
	// To change a few fields, start from DefaultMarkdownStyle() and edit it.
	Style MarkdownStyle
	// MermaidWidth is the max pixel width for mermaid diagrams
	// (0 = 600).
	// exportaudit:keep — caller-facing config (issue #372)
	MermaidWidth int
	Padding      Padding
	SizeBorder   Border
	Radius       Radius
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

	// AllowRemoteImages downloads http(s) images. Off by default:
	// markdown is often untrusted, and a fetch tells the host the
	// reader's IP and open time. Blocked images show alt text (#1105).
	// exportaudit:keep — caller-facing config (issue #1105)
	AllowRemoteImages bool
	// AllowLocalImages reads image paths from disk (still limited by
	// WindowCfg.AllowedImageRoots). Off by default. Data URLs and
	// UseImage sources do no I/O and always show (#1105).
	// exportaudit:keep — caller-facing config (issue #1105)
	AllowLocalImages bool

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

	// RenderBlock, when non-nil, is consulted for every block during
	// layout. Return ok=false to fall back to the default renderer;
	// ok=true replaces the block's output, with a view or with nil to
	// drop the block entirely.
	//
	// It runs once per block per frame over the cached styled blocks,
	// so it must be cheap and pure: build View structs, do not fetch
	// or parse. It runs during GenerateLayout, under the frame lock,
	// so SetFocus, ClearFocus, SetView, ClearDrawCanvasCache and
	// Window.Lock all panic from it; QueueCommand is the remedy and
	// is permitted. Hook writers own their IDs — compose them with
	// ScopeID(el.DocID, …) or ScopeIDN — and their a11y roles.
	// exportaudit:keep — caller-facing config (issue #484)
	RenderBlock func(w *Window, el MarkdownElement) (View, bool)
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
// every block references the widget through TC.rtf.markdownID — both keyed
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
	// it, the blocks stamp it into TC.rtf.markdownID, and every state slot
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

	// An unset Style takes the theme default, so a zero MarkdownCfg
	// renders like every other zero Cfg (#998). Text.Size is the sentinel:
	// no usable style has a zero body size, and before this fallback a
	// zero size reached the renderer and go-glyph guessed one. The style
	// is resolved here, not in the factory, so a Themed ancestor applies.
	// A caller who set only CodeHighlighter on an otherwise empty style
	// keeps it.
	//
	// The resolved style goes into a per-call copy, never into mv.cfg.
	// Written back, it would no longer read as unset: a later generation
	// of the same view after a theme flip, or under another Themed
	// ancestor, would skip this branch and rebuild with the old theme's
	// faces. Only the ID write above persists. The copy stays on the
	// stack: nothing below captures cfg in a closure.
	rc := *cfg
	if rc.Style.Text.Size == 0 {
		highlighter := rc.Style.CodeHighlighter
		rc.Style = DefaultMarkdownStyle()
		rc.Style.CodeHighlighter = highlighter
	}
	cfg = &rc

	mode := cfg.Mode.Get(TextModeWrap)

	// Cache lookup; invalidate on theme change.
	//
	// The key names the source and the style that styled it. The
	// blocks hold resolved run styles, so the same source under two
	// styles shares nothing, and restyling a document with an
	// unchanged source rebuilds its blocks (issue #1000). The key
	// carries the highlighter's type; the entry carries the
	// highlighter, so two of one type with different token tables
	// compare apart.
	key := newMarkdownCacheKey(cfg.Source, &cfg.Style)
	themeID := guiTheme.id
	if w.viewState.markdownCache == nil ||
		w.viewState.markdownTheme != themeID {
		w.viewState.markdownCache =
			NewBoundedMap[markdownCacheKey, markdownCacheEntry](100)
		w.viewState.markdownTheme = themeID
	}
	entry, ok := w.viewState.markdownCache.Get(key)
	blocks := entry.blocks
	if !ok || !sameMarkdownHighlighter(
		entry.highlighter, cfg.Style.CodeHighlighter,
	) {
		blocks = markdownToBlocks(cfg.Source, cfg.Style)
		w.viewState.markdownCache.Set(key, markdownCacheEntry{
			highlighter: cfg.Style.CodeHighlighter,
			blocks:      blocks,
		})
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

	// The document copy button floats over the top-right corner, on
	// top of the first block. A first block that runs the full width
	// (a long paragraph, a code block) would have its first line drawn
	// under the button, so pull that block's right edge in by the
	// width the button covers. Later blocks start below the button and
	// keep the full width.
	if len(content) > 0 {
		content[0] = Row(ContainerCfg{
			Sizing:     FillFit,
			SizeBorder: NoBorder,
			Padding:    NewPadding(0, mdCopyButtonReserve(), 0, 0),
			Content:    []View{content[0]},
		})
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
		Spacing:     SpacingPx(cfg.Style.blockSpacing),
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

// mdKindOf names the branch markdownBuildContent's render switch
// would take for a block. The flags are not mutually exclusive — a
// quoted fence is both code and quote — so the test order here
// copies that switch exactly; a second hand-written order would
// silently disagree with the fallthrough dispatch. Kind follows the
// inner kind, and the quote survives on IsBlockquote/BlockquoteDepth.
// Mermaid is deliberately absent: it dispatches from inside the code
// branch on CodeLanguage, so it reads as code here.
func mdKindOf(block markdownBlock) MarkdownBlockKind {
	switch {
	case block.IsMath:
		return MarkdownKindMath
	case block.IsCode:
		return MarkdownKindCode
	case block.IsTable:
		return MarkdownKindTable
	case block.IsHR:
		return MarkdownKindHR
	case block.IsImage:
		return MarkdownKindImage
	case block.HeaderLevel > 0:
		return MarkdownKindHeading
	case block.IsDefTerm:
		return MarkdownKindDefTerm
	case block.IsDefValue:
		return MarkdownKindDefValue
	case block.IsList:
		return MarkdownKindList
	case block.IsBlockquote:
		return MarkdownKindBlockquote
	default:
		return MarkdownKindParagraph
	}
}

// mdKindSelectable reports whether a kind's default renderer takes a
// per-block ctx, and so advances the document's rune offsets. Six of
// the eleven do; math, code, tables, rules and images do not. It sits
// beside mdKindOf so the two lists cannot drift.
func mdKindSelectable(kind MarkdownBlockKind) bool {
	switch kind {
	case MarkdownKindBlockquote, MarkdownKindHeading,
		MarkdownKindDefTerm, MarkdownKindDefValue,
		MarkdownKindList, MarkdownKindParagraph:
		return true
	default:
		return false
	}
}

// mdElementOf copies a styled block into the exported form the render
// hook sees. PlainText and the table copy are built here rather than
// cached on markdownBlock, so a document with no hook pays neither.
func mdElementOf(
	block markdownBlock, kind MarkdownBlockKind, index int, docID string,
) MarkdownElement {
	el := MarkdownElement{
		DocID:           docID,
		PlainText:       richTextPlain(block.Content),
		ListPrefix:      block.ListPrefix,
		ImageSrc:        block.ImageSrc,
		ImageAlt:        block.ImageAlt,
		CodeLanguage:    block.CodeLanguage,
		MathLatex:       block.MathLatex,
		AnchorSlug:      block.AnchorSlug,
		Content:         block.Content,
		Kind:            kind,
		Index:           index,
		HeaderLevel:     block.HeaderLevel,
		BlockquoteDepth: block.BlockquoteDepth,
		ListIndent:      block.ListIndent,
		ImageWidth:      block.ImageWidth,
		ImageHeight:     block.ImageHeight,
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
	}
	if block.TableData != nil {
		el.TableData = &MarkdownTable{
			Headers:    block.TableData.Headers,
			Alignments: block.TableData.Alignments,
			Rows:       block.TableData.Rows,
		}
	}
	return el
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
		runeOffset += uint32(rtfRuneCountFromRuns(
			&block.Content, w.viewState.diagramCache))
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

		if cfg.RenderBlock != nil {
			kind := mdKindOf(block)
			v, ok := cfg.RenderBlock(w,
				mdElementOf(block, kind, i, cfg.ID))
			if ok {
				// Rune offsets belong to the document, not to the
				// renderer. Advancing them here for a kind whose
				// default branch never would -- code, a table -- gives
				// a hooked document different offsets from the same
				// document unhooked, so selection breaks in the hooked
				// case only. makeCtx is what advances them; the ctx
				// itself is unused because a hooked block is not
				// selectable text.
				if selEnabled && mdKindSelectable(kind) {
					makeCtx(block)
				}
				// A hooked list item skips the accumulator, so any
				// items already pending have to land before it or they
				// render after their own siblings.
				if len(listItems) > 0 {
					content = append(content,
						mdFlushListItems(listItems, cfg))
					listItems = nil
				}
				if v != nil {
					content = append(content, v)
				}
				continue
			}
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
			content = append(content,
				mdWrapQuote(block, mdRenderHR(cfg), cfg))
		case block.IsImage:
			content = append(content, mdRenderImage(block, cfg))
		case block.HeaderLevel > 0:
			content = append(content,
				mdRenderHeading(block, cfg, mode, makeCtx(block))...)
		case block.IsDefTerm:
			content = append(content, mdWrapQuote(block,
				mdRenderDefTerm(block, mode, makeCtx(block)), cfg))
		case block.IsDefValue:
			content = append(content,
				mdRenderDefValue(block, cfg, mode, makeCtx(block)))
		case block.IsList:
			listItems = append(listItems,
				mdRenderListItem(block, cfg, mode, makeCtx(block)))
		case block.IsBlockquote:
			content = append(content,
				mdRenderBlockquote(block, cfg, mode, makeCtx(block)))
		default:
			content = append(content,
				mdRenderParagraph(block, cfg, mode, makeCtx(block)))
		}
	}

	// A document that ends inside a list has no following block to
	// flush it, so the tail flush lives here rather than in the list
	// case: every way out of the loop reaches it, including a block
	// the render hook took over.
	if len(listItems) > 0 {
		content = append(content, mdFlushListItems(listItems, cfg))
	}
	return content
}
