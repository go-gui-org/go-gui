Render markdown strings with syntax highlighting, tables, blockquotes,
math (LaTeX via CodeCogs), and mermaid diagrams (via Kroki). Uses the
built-in parser and renders via RTF views internally.

## Usage

```go
w.Markdown(gui.MarkdownCfg{
    Source: "# Hello\n**Bold** and *italic*",
    Style:  gui.DefaultMarkdownStyle(),
})
```

## Custom Style

```go
style := gui.DefaultMarkdownStyle()
style.LinkColor = gui.ColorFromString("#3b82f6")
style.CodeBlockBG = gui.RGBA(30, 30, 30, 255)
w.Markdown(gui.MarkdownCfg{
    Source: src,
    Style:  style,
})
```

## Supported Elements

- Headings (H1–H6), paragraphs, line breaks
- **Bold**, *italic*, ~~strikethrough~~, `code`
- Ordered and unordered lists
- Tables with column alignment
- Fenced code blocks with syntax highlighting
- Blockquotes, horizontal rules
- Links and images
- Mermaid diagrams (` ```mermaid `)
- LaTeX math (requires `SetMarkdownExternalAPIsEnabled(true)`)

## Key Properties

| Property            | Type           | Description                          |
|---------------------|----------------|--------------------------------------|
| Source              | string         | Markdown source string               |
| Style               | MarkdownStyle  | Typography and color configuration   |
| Mode                | Opt[TextMode]  | Text wrapping mode                   |
| IDFocus             | uint32         | Tab-order focus ID (enables select)  |
| MinWidth            | float32        | Minimum view width                   |
| MermaidWidth        | int            | Max mermaid diagram width (0 = 600)  |
| DisableExternalAPIs | bool           | Disable CodeCogs/Kroki APIs          |
| Disabled            | bool           | Disable interaction                  |
| Invisible           | bool           | Hide without removing from layout    |
| Clip                | bool           | Clip content to bounds               |
| FocusSkip           | bool           | Skip in tab-order navigation         |

## Appearance

| Property    | Type         | Description                          |
|-------------|--------------|--------------------------------------|
| Color       | Color        | Background color                     |
| ColorBorder | Color        | Border color                         |
| SizeBorder  | float32      | Border width                         |
| Radius      | float32      | Corner radius                        |
| Padding     | Opt[Padding] | Inner padding                        |

## MarkdownStyle

| Field            | Type             | Description                       |
|------------------|------------------|-----------------------------------|
| Text             | TextStyle        | Body text style                   |
| H1–H6           | TextStyle        | Heading styles                    |
| Bold             | TextStyle        | Bold text style                   |
| Italic           | TextStyle        | Italic text style                 |
| BoldItalic       | TextStyle        | Bold+italic text style            |
| Code             | TextStyle        | Inline code style                 |
| CodeBlockText    | TextStyle        | Code block text style             |
| CodeBlockBG      | Color            | Code block background             |
| CodeBlockPadding | Opt[Padding]     | Code block padding                |
| CodeBlockRadius  | float32          | Code block corner radius          |
| LinkColor        | Color            | Hyperlink color                   |
| HRColor          | Color            | Horizontal rule color             |
| BlockquoteBorder | Color            | Blockquote left border color      |
| BlockquoteBG     | Color            | Blockquote background             |
| BlockSpacing     | float32          | Spacing between blocks            |
| NestIndent       | float32          | Indent per nesting level          |
| H1Separator      | bool             | Show line under H1                |
| H2Separator      | bool             | Show line under H2                |
| TableBorderStyle | TableBorderStyle | Table border style                |
| TableBorderColor | Color            | Table border color                |
| TableHeadStyle   | TextStyle        | Table header text style           |
| TableCellStyle   | TextStyle        | Table cell text style             |
| HighlightBG      | Color            | Highlight background              |
| HardLineBreaks   | bool             | Treat newlines as hard breaks     |
| MermaidBG        | Color            | Mermaid diagram background        |
