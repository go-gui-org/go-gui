# svg_flatness

Demonstrates `SvgCfg.FlatnessTolerance` — the tessellation tolerance floor in
viewBox units.

A curve-heavy SVG is rendered five times with increasing tolerance: default (0),
0.5, 1.5, 4, 10. Higher values produce coarser polylines and fewer triangles,
visible as faceting on the Bezier edges. Use this knob to trade visual fidelity
for vertex count on icon walls or large path-heavy renders.

Default `FlatnessTolerance=0` keeps the renderer's built-in 0.15 floor —
fingerprint-stable with prior versions.

Run:

```
go run ./examples/svg_flatness/
```
