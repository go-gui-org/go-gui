package gui

// markdown_types.go defines styled markdown block types.
// These are the output of the styling bridge that converts
// parser MdBlocks into GUI-ready MarkdownBlocks.

// MarkdownBlock is a parsed, styled block of markdown.
type markdownBlock struct {
	baseStyle TextStyle
	TableData *parsedTable
	// ListPrefix is the visible list marker ("1. ", "• ") for ordinary
	// list items. Task-list items (IsTaskItem) leave this empty and
	// carry their checked state in TaskChecked instead.
	ListPrefix      string
	ImageSrc        string
	ImageAlt        string
	CodeLanguage    string
	MathLatex       string
	AnchorSlug      string
	Content         RichText
	HeaderLevel     int
	BlockquoteDepth int
	ListIndent      int
	ImageWidth      float32
	ImageHeight     float32
	IsCode          bool
	IsHR            bool
	IsBlockquote    bool
	IsImage         bool
	IsTable         bool
	IsList          bool
	IsMath          bool
	IsDefTerm       bool
	IsDefValue      bool
	IsTaskItem      bool
	TaskChecked     bool
}

// ParsedTable is a parsed, styled markdown table.
type parsedTable struct {
	Headers    []RichText
	Alignments []HorizontalAlign
	Rows       [][]RichText
}

// MarkdownBlockKind names a parsed markdown block for
// MarkdownElement.Kind. Paragraph is zero — it is the render switch's
// default branch, so a zero MarkdownElement reads as the generic case
// rather than as a heading.
// exportaudit:keep — caller-facing config (issue #484)
type MarkdownBlockKind int

// The eleven kinds markdownBuildContent dispatches on. Mermaid has no
// kind of its own: renderMdMermaid dispatches from inside the code
// branch on CodeLanguage == "mermaid", so a hook sees
// MarkdownKindCode with that language.
//
// exportaudit:keep — caller-facing config (issue #484). The set is the
// render switch, so a kind an app happens not to match on today is
// still part of the contract; dropping the unmatched ones would make
// the constant a hook writer needs next the one that is missing.
const (
	MarkdownKindParagraph MarkdownBlockKind = iota
	MarkdownKindHeading
	MarkdownKindCode
	MarkdownKindTable
	MarkdownKindHR
	MarkdownKindBlockquote
	MarkdownKindImage
	MarkdownKindMath
	MarkdownKindList
	MarkdownKindDefTerm
	MarkdownKindDefValue
)

// MarkdownElement is one parsed, styled markdown block, passed to
// MarkdownCfg.RenderBlock. It mirrors markdownBlock field-for-field
// except the unexported baseStyle, plus Kind, Index, DocID, PlainText
// and the exported table. A curated subset would omit fields real
// hooks need — list indent, task state, image dimensions, code
// language — and force the writer to re-derive them.
// exportaudit:keep — caller-facing config (issue #484)
type MarkdownElement struct {
	// TableData is nil for every kind but MarkdownKindTable.
	TableData *MarkdownTable
	// DocID is the document's effective ID, already resolved by
	// Markdown before the build pass. Compose inner IDs from it with
	// ScopeID / ScopeIDN; do not call EffID on it again.
	DocID string
	// PlainText is Content with the styling dropped. It is derived per
	// hooked block rather than cached on the block, so the cost lands
	// on the documents that install a hook.
	PlainText    string
	ListPrefix   string
	ImageSrc     string
	ImageAlt     string
	CodeLanguage string
	MathLatex    string
	AnchorSlug   string
	// Content is the styled runs, so a hook reuses MarkdownStyle
	// instead of re-styling raw text.
	Content RichText
	Kind    MarkdownBlockKind
	// Index is the block's source-order position among the cached
	// styled blocks, so it shifts when the source changes. Headings
	// should compose IDs from AnchorSlug instead.
	Index           int
	HeaderLevel     int
	BlockquoteDepth int
	ListIndent      int
	ImageWidth      float32
	ImageHeight     float32
	IsCode          bool
	IsHR            bool
	IsBlockquote    bool
	IsImage         bool
	IsTable         bool
	IsList          bool
	IsMath          bool
	IsDefTerm       bool
	IsDefValue      bool
	IsTaskItem      bool
	TaskChecked     bool
}

// MarkdownTable mirrors parsedTable exactly. No column count: that
// belongs to the parser's table, not the styled form. Exported
// because a table's cells cannot be rebuilt from RichText alone.
// exportaudit:keep — caller-facing config (issue #484)
type MarkdownTable struct {
	Headers    []RichText
	Alignments []HorizontalAlign
	Rows       [][]RichText
}
