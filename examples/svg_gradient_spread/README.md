# svg_gradient_spread

Demonstrates the `spreadMethod` attribute on `<linearGradient>` and
`<radialGradient>`.

For each gradient kind the same source is rendered three times, once per spread
mode:

- `pad` — values outside [0,1] clamp to the first/last stop. Default.
- `reflect` — triangle wave: the gradient mirrors back and forth.
- `repeat` — sawtooth: the gradient wraps as a tile.

Stops sit on a short segment (`x1=0% x2=40%`) leaving room outside the
gradient's own range so reflect/repeat have somewhere to wrap.

Run:

```
go run ./examples/svg_gradient_spread/
```
