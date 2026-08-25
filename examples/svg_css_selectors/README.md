# svg_css_selectors

Six tiles demonstrate v0.14.0 CSS Selectors L4 additions:

- **Adjacent (`+`)** — `rect + circle` paints only the circle that immediately
  follows a `rect`.
- **General sibling (`~`)** — `rect ~ circle` paints every circle that follows a
  `rect` in document order.
- **Attribute equal (`[data-state=active]`)** — picks elements whose attribute
  exactly matches.
- **Attribute prefix (`[data-kind^=hot]`)** — picks elements whose attribute
  starts with the needle.
- **`:not(.skip)`** — inverts a class selector (single-compound negation).
- **Compound mix (`rect + [tag=ok]`)** — combines a combinator with an attribute
  selector.

## Run

```
go run ./examples/svg_css_selectors
```
