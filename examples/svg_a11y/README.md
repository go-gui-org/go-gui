# svg_a11y

Demonstrates accessibility metadata parsing on SVG documents. The viewer renders
an icon and prints its parsed `<title>`, `<desc>`, and `aria-*` attributes
side-by-side.

## Run

```
go run ./examples/svg_a11y
```

Metadata is read from `cached.Parsed.A11y` (`SvgParsed.A11y`), set by the parser
after `LoadSvg`.
