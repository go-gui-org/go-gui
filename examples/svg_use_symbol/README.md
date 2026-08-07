# svg_use_symbol

Demonstrates SVG `<use href="#id">` and `<symbol>` resolution.

A `<symbol id="star">` block is defined once in `<defs>`; four `<use>`
references render the symbol at different positions, each with a per-instance
`fill` override. The result is rendered side by side with a manually duplicated
equivalent so any geometric or color delta is immediately visible.

A second sample shows `<use>` referencing a single `<circle>` element, including
per-instance `transform="scale(...)"` and `transform="rotate(...)"` overrides on
the use sites.

Run:

```
go run ./examples/svg_use_symbol/
```
