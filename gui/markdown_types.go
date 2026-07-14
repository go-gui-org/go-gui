package gui

// markdown_types.go defines styled markdown block types.
// These are the output of the styling bridge that converts
// parser MdBlocks into GUI-ready MarkdownBlocks.

// MarkdownBlock is a parsed, styled block of markdown.
type MarkdownBlock struct {
	BaseStyle TextStyle
	TableData *ParsedTable
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
type ParsedTable struct {
	Headers    []RichText
	Alignments []HorizontalAlign
	Rows       [][]RichText
}
