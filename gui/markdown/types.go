package markdown

// types.go defines parser output types for the markdown
// pipeline. These are the intermediate types between the
// goldmark AST and the gui styling layer.

// Limits for multi-line constructs.
const (
	maxFootnoteContinuationLines = 20
)

// Source length caps for external API submissions.
const (
	MaxLatexSourceLen   = 2000
	MaxMermaidSourceLen = 10000
)

// Metadata collection limits.
const (
	maxAbbreviationDefs = 1000
	maxFootnoteDefs     = 10000
)

// Highlight limits.
const (
	maxCodeBlockHighlightBytes  = 131072
	maxInlineCodeHighlightBytes = 2048
	maxHighlightTokensPerBlock  = 16384
	maxHighlightTokenBytes      = 4096
	maxHighlightCommentDepth    = 16
	maxHighlightStringScanBytes = 32768
	maxHighlightIdentifierBytes = 256
	maxHighlightNumberBytes     = 128
)

// Align represents column alignment in tables.
type Align uint8

// Align constants.
const (
	AlignStart Align = iota
	AlignEnd
	AlignCenter
	AlignLeft
	AlignRight
)

// Format represents inline text formatting.
type Format uint8

// Format constants.
const (
	FormatPlain Format = iota
	FormatBold
	FormatItalic
	FormatBoldItalic
	FormatCode
)

// CodeTokenKind classifies syntax highlighting tokens.
type CodeTokenKind uint8

// CodeTokenKind constants.
const (
	TokenPlain CodeTokenKind = iota
	TokenKeyword
	TokenString
	TokenNumber
	TokenComment
	TokenOperator
)

// CodeLanguage identifies a programming language for
// syntax highlighting.
type CodeLanguage uint8

// CodeLanguage constants.
const (
	LangGeneric CodeLanguage = iota
	LangV
	LangJavaScript
	LangTypeScript
	LangPython
	LangJSON
	LangGo
	LangRust
	LangC
	LangShell
	LangHTML
)

// CodeToken represents a highlighted token span.
type CodeToken struct {
	Kind  CodeTokenKind
	Start int
	End   int
}

// Run is a single inline text segment with format and flags.
type Run struct {
	Text          string
	Link          string
	Tooltip       string // abbreviation or footnote
	MathID        string
	MathLatex     string
	Format        Format
	Strikethrough bool
	Underline     bool
	Highlight     bool // ==text==
	Superscript   bool
	Subscript     bool
	CodeToken     CodeTokenKind
}

// Block is a parsed block of markdown content.
type Block struct {
	TableData       *Table
	ListPrefix      string
	ImageSrc        string
	ImageAlt        string
	CodeLanguage    string
	MathLatex       string
	AnchorSlug      string
	Runs            []Run
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
}

// Table holds parsed table data.
type Table struct {
	Headers    [][]Run
	Alignments []Align
	Rows       [][][]Run
	ColCount   int
}
