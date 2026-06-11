Single-style text rendering with theme typography. Supports wrapping,
alignment, gradients, outlines, rotation, custom colors, and text selection
via IDFocus. Use for labels, headings, or larger blocks of text.

## Usage

```go
t := gui.CurrentTheme()

// Basic text
gui.Text(gui.TextCfg{Text: "Hello", TextStyle: t.N3})

// Wrapping text
gui.Text(gui.TextCfg{
    Text:      "Long paragraph that wraps to container width.",
    TextStyle: t.N4,
    Mode:      gui.TextModeWrap,
})

// Custom color
gui.Text(gui.TextCfg{
    Text: "Colored",
    TextStyle: gui.TextStyle{
        Color: gui.ColorFromString("#3b82f6"),
        Size:  t.N3.Size,
    },
})
```

## Theme Style Shortcuts

| Prefix | Meaning   | Sizes                  |
|--------|-----------|------------------------|
| N      | Normal    | N1–N6 (XLarge–Tiny)    |
| B      | Bold      | B1–B6                  |
| I      | Italic    | I1–I6                  |
| M      | Monospace | M1–M6                  |
| Icon   | Icon font | Icon1–Icon6            |

## Text Modes

| Mode                 | Behavior                       |
|----------------------|--------------------------------|
| TextModeSingleLine   | No wrapping (default)          |
| TextModeMultiline    | Honors `\\n` line breaks        |
| TextModeWrap         | Word-wraps to container width  |
| TextModeWrapKeepSpaces | Wraps preserving whitespace  |

## Key Properties

| Property          | Type      | Description                          |
|-------------------|-----------|--------------------------------------|
| Text              | string    | Content to display                   |
| TextStyle         | TextStyle | Font, size, color, decorations       |
| Mode              | TextMode  | Wrapping behavior                    |
| IDFocus           | uint32    | Tab-order focus ID (enables select)  |
| TabSize           | uint32    | Tab width in spaces                  |
| MinWidth          | float32   | Minimum text width                   |
| Clip              | bool      | Clip text to bounds                  |
| Opacity           | float32   | Text opacity (0.0–1.0)              |
| Hero              | bool      | Animate text transitions             |
| IsPassword        | bool      | Mask characters                      |
| PlaceholderActive | bool      | Render as placeholder style          |
| Sizing            | Sizing    | Override default sizing              |
| Disabled          | bool      | Disable interaction                  |
| Invisible         | bool      | Hide without removing from layout    |
| FocusSkip         | bool      | Skip in tab-order navigation         |

## TextStyle Fields

| Field           | Type                  | Description                    |
|-----------------|-----------------------|--------------------------------|
| Color           | Color                 | Text fill color                |
| Size            | float32               | Font size in points            |
| Align           | TextAlignment         | Left, center, or right         |
| Underline       | bool                  | Underline decoration           |
| Strikethrough   | bool                  | Strikethrough decoration       |
| LetterSpacing   | float32               | Extra space between characters |
| LineSpacing     | float32               | Extra space between lines      |
| Gradient        | *glyph.GradientConfig | Gradient text fill             |
| StrokeWidth     | float32               | Text outline width             |
| StrokeColor     | Color                 | Text outline color             |
| RotationRadians | float32               | Text rotation                  |

## Accessibility

| Property        | Type   | Description                          |
|-----------------|--------|--------------------------------------|
| A11YLabel       | string | Accessible label                     |
| A11YDescription | string | Accessible description               |
