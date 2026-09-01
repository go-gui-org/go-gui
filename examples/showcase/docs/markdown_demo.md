# Markdown Sample Document

This document demonstrates all common Markdown elements. Use it as a reference
for syntax or as a test file for rendering engines.

## Text Formatting

Regular paragraph text flows naturally across multiple lines. When you write
content in Markdown, you do not need to worry about line breaks within a
paragraph. The renderer handles text wrapping automatically. This makes it easy
to write long-form content without constantly thinking about formatting.

Here is another paragraph to show separation. Paragraphs are separated by blank
lines. You can use **bold text** for emphasis, _italic text_ for subtle
emphasis, or _**bold and italic**_ together. You can also use ~~strikethrough~~
for deleted content and `inline code` for technical terms or commands.

### Alternative Emphasis Syntax

You can also use underscores: ++_italic with underscores_++ and ++**bold with
underscores**_++. Most people prefer asterisks, but both work identically in
standard Markdown parsers. Use double plus signs for ++underlined text++ or
combine with bold: ++**bold underline**++.

## Lists

### Unordered Lists

- The first item in this unordered list demonstrates how list items wrap. List
  items wrap when they exceed the typical line length of most editors and
  viewers. That length is usually around 80 to 120 characters, depending on your
  settings.
- Second item with a shorter description but still meaningful content.
- Third item that also spans multiple lines when rendered in a narrow viewport.
  Markdown handles long list content without breaking the visual hierarchy of
  the document.
  - Nested item underneath the third item. You can create hierarchical
    structures within your lists by indenting with two spaces or a tab.
  - Another nested item with substantial content. Nested items also wrap
    properly when they contain lengthy descriptions or explanations that exceed
    the available width.
    - Deeply nested item showing three levels of nesting. Three levels is about
      as deep as you typically want to go for readability purposes.
- Fourth item back at the root level.

### Ordered Lists

1.  First ordered item with extensive explanatory text that wraps across
    multiple lines. Numbered lists behave the same way as bulleted lists for
    handling long content within individual items.
2.  Second ordered item that continues the numbering sequence automatically. The
    sequence continues regardless of what number you actually type in the source
    Markdown.
3.  Third item with enough content to wrap. The visual presentation stays clean
    and readable even when individual list items contain paragraph-length
    descriptions or instructions.
    1.  Nested numbered item showing that you can create sub-lists within
        ordered lists as well. The sub-lists maintain proper indentation and
        numbering throughout the hierarchy.
    2.  Another nested numbered item with additional explanatory content.
4.  Fourth item returning to the main list level.

### Task Lists

- [x] Completed task. You mark it as done by placing an 'x' inside the brackets.
- [x] Another finished task demonstrating the checked state. The checked state
      renders as a filled checkbox in most Markdown viewers and platforms that
      support this extension.
- [ ] Incomplete task waiting to be done, shown with empty brackets that render
      as an unchecked checkbox in compatible viewers.
- [ ] Another pending task with a longer description that explains what needs to
      be accomplished. Task list items can also contain substantial amounts of
      text that wrap naturally.

## Blockquotes

> This is a blockquote containing a substantial amount of text that spans
> multiple lines. Blockquotes are commonly used for citing external sources,
> highlighting important information, or setting apart quoted material from the
> main body of your document. They create visual distinction through indentation
> and styling.
>
> You can include multiple paragraphs within a single blockquote by continuing
> to use the greater-than symbol at the start of each line. This second
> paragraph is still part of the same blockquote.
>
> > Nested blockquotes are also possible. They allow you to show quoted material
> > within quoted material. This is useful for email threads or forum
> > discussions where multiple levels of response exist.

### Callouts

Callouts are not markdown. The showcase renders them with
`MarkdownCfg.RenderBlock`, which replaces the output of any block the app
recognises. A blockquote that opens with a marker becomes a tinted panel; every
other block falls through to the default renderer.

> [!NOTE] Blockquotes that start with a marker render as a callout. The marker
> is stripped and the rest of the quote becomes the body.

> [!WARNING] The convention is the app's, not the parser's. Another app can read
> the same source and render plain blockquotes.

## Code

### Inline Code

Use `console.log()` for debugging in JavaScript, or run
`pip install package-name` to install Python packages.

### Code Blocks

```javascript
// JavaScript example with syntax highlighting
function calculateTotal(items) {
  return items
    .filter((item) => item.active)
    .reduce((sum, item) => sum + item.price * item.quantity, 0);
}

const cart = [
  { name: "Widget", price: 25.0, quantity: 2, active: true },
  { name: "Gadget", price: 49.99, quantity: 1, active: true },
  { name: "Removed Item", price: 10.0, quantity: 1, active: false },
];

console.log(`Total: $${calculateTotal(cart).toFixed(2)}`);
```

```python
# Python example demonstrating classes
class ShoppingCart:
    def __init__(self):
        self.items = []

    def add_item(self, name, price, quantity=1):
        self.items.append({
            'name': name,
            'price': price,
            'quantity': quantity
        })

    def total(self):
        return sum(item['price'] * item['quantity'] for item in self.items)
```

## Links and Images

### Links

Here is a [link to OpenAI](https://openai.com) inline in text. You can also use
[reference-style links](https://example.com "Example Website"). They define the
URL elsewhere in the document, which keeps paragraphs cleaner when you have many
links.

Autolinks work for URLs: <https://www.github.com> and email addresses:
<email@example.com>.

### Images

![gopher](https://raw.githubusercontent.com/avelino/awesome-go/main/tmpl/assets/logo.png)

## Tables

| Feature    | Basic | Extended | Notes                              |
| ---------- | ----- | -------- | ---------------------------------- |
| Headers    | ✓     | ✓        | Six levels available (h1-h6)       |
| Emphasis   | ✓     | ✓        | Bold, italic, and combinations     |
| Lists      | ✓     | ✓        | Ordered, unordered, and nested     |
| Task Lists | ✗     | ✓        | GitHub Flavored Markdown extension |
| Tables     | ✗     | ✓        | Alignment options with colons      |
| Footnotes  | ✗     | ✓        | Not universally supported          |

_Table columns can be left-aligned, center-aligned, or right-aligned using
colons in the separator row._

## Horizontal Rules

Content above the horizontal rule.

---

Content below the horizontal rule. You can create horizontal rules with three or
more hyphens, asterisks, or underscores.

---

Another section after a different style of horizontal rule.

## Advanced Elements

### Footnotes

Here is a sentence with a footnote[^1] and here is another one[^2].

### Definition Lists

Term 1 : Definition for the first term, which can be quite detailed and span
multiple lines if necessary to fully explain the concept being defined.

Term 2 : Primary definition for the second term. : Alternative definition
showing that terms can have multiple definitions.

### Abbreviations

The HTML specification is maintained by the W3C. HTMLX is not the same as HTML.

*[HTML]: Hyper Text Markup Language *[W3C]: World Wide Web Consortium

## Escaping Characters

Use backslashes to display literal characters. Without backslashes, Markdown
interprets those characters as formatting:

\*This text is surrounded by literal asterisks\*

\# This is not a heading

## Math Expressions

### Display Math

$$ E = mc^2 $$

$$ \int_0^\infty e^{-x^2} dx = \frac{\sqrt{\pi}}{2} $$

```math
\sum_{n=1}^{\infty} \frac{1}{n^2} = \frac{\pi^2}{6}
```

### Inline Math

The quadratic formula $x = \frac{-b \pm \sqrt{b^2-4ac}}{2a}$ gives the roots of
$ax^2+bx+c=0$. Note that $10 is a price, not math.

## Mermaid Diagrams

```mermaid
graph TD
    A[User Input] --> B{Valid?}
    B -->|Yes| C[Process]
    B -->|No| D[Error]
    C --> E[Output]
```

## Highlighted Text

Use double equals signs to ==highlight important text== within a paragraph. You
can combine it with other formatting: ==**bold highlight**== and ==_italic
highlight_== both work.

## Emoji Shortcodes

Emoji shortcodes use the `:name:` syntax: :smile: :heart: :rocket: :thumbsup:
:wave: :star: :fire: :white_check_mark:

Some common ones: :warning: for caution, :bulb: for ideas, and :+1: for
approval.

## Superscript and Subscript

Water is H~~2~~O and Einstein's famous equation is E = mc^2^. The notation x^n^
means x raised to the nth power. CO~~2~~ emissions are measured in parts per
million.

## Indented Code Blocks

Four spaces or a tab at the start of a line creates a code block:

    fn main() {
        println('Hello from indented code!')
    }

Regular text resumes after the indented block.

## Internal Links

Click this link to jump to the [Text Formatting](#text-formatting) section
above, or go to [Lists](#lists) or [Tables](#tables).

## More Syntax Highlighting

```go
func fibonacci(n int) int {
    if n <= 1 {
        return n
    }
    return fibonacci(n-1) + fibonacci(n-2)
}
```

```rust
fn main() {
    let numbers: Vec<i32> = (1..=10).collect();
    let sum: i32 = numbers.iter().sum();
    println!('Sum: {}', sum);
}
```

```bash
#!/bin/bash
for file in *.txt; do
    echo $file
    wc -l $file
done
```

## Conclusion

This document has covered all the common Markdown elements you are likely to
encounter or need in everyday writing. Different Markdown processors can support
additional features or have slight variations in syntax. Always test your
content in the target environment.

For more information, consult the
[CommonMark Specification](https://commonmark.org/) or the documentation for
your specific Markdown processor.

[^1]:
    This is the first footnote, providing additional context or citation
    information.

[^2]: This is a longer footnote with multiple paragraphs.

    Indent subsequent paragraphs to include them in the footnote.
