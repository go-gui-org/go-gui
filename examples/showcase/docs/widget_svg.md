Scalable vector graphics from file or inline SVG string.
Faithful rendering of designer-authored static and animated SVG —
not a complete SVG 1.1/2 implementation. See `docs/svg-support.md`
for the authoritative feature matrix.

## From File

```go
gui.Svg(gui.SvgCfg{
    FileName: "diagram.svg",
    Width:    200,
    Height:   150,
})
```

## Inline SVG

```go
gui.Svg(gui.SvgCfg{
    SvgData: "<svg viewBox=\"0 0 24 24\">...</svg>",
    Width:   48,
    Height:  48,
    Color:   gui.White,
})
```

Provide either `FileName` or `SvgData`, not both. When width/height
are omitted, native SVG dimensions are used.

## Supported Features

**Shapes:** `<path>`, `<rect>` (rx/ry), `<circle>`,
`<ellipse>`, `<line>`, `<polygon>`,
`<polyline>`, `<g>` / `<a>`, `<defs>`,
`<use>` / `<symbol>` (href + xlink:href).

**Text:** `<text>`, `<tspan>` (positioned runs),
`<textPath>` (text along path).

**Paint:** solid colors (named, `#rgb`, `#rrggbb`,
`rgb()`, `rgba()`), `currentColor`,
linear + radial gradients including `spreadMethod` =
`pad` / `reflect` / `repeat`, focal
`fx`/`fy`, `gradientUnits` (objectBoundingBox + userSpaceOnUse).

**Stroke:** width, dasharray + dashoffset, linecap, linejoin (incl. miter limit),
non-scaling-stroke flatness control.

**Transform & Clip:** `transform` (translate/rotate/scale/skew/matrix),
`clipPath`, single `feGaussianBlur` filter.

**Animation:** SMIL `<animate>`, `<animateTransform>`,
`<animateMotion>` plus CSS `@keyframes` (`NoAnimate`
disables both).

**CSS:** inline `style=\"\"`, `<style>` blocks, class +
attribute selectors, descendant + sibling combinators (`>`, `+`,
`~`), pseudo-classes `:hover` / `:focus` /
`:not()`, custom properties (`--name` + `var(--name, fallback)`),
`calc()` arithmetic in property values.

```svg
<svg viewBox="0 0 100 100" xmlns="http://www.w3.org/2000/svg">
  <style>
    svg { --accent: #06b6d4; }
    .dot { fill: #475569; r: 10; }
    .dot[data-state="on"] { fill: var(--accent); }
    .dot + .dot { r: calc(10 + 2); }
    .dot:hover { fill: #ec4899; }
  </style>
  <circle class="dot" cx="20" cy="50"/>
  <circle class="dot" cx="50" cy="50" data-state="on"/>
  <circle class="dot" cx="80" cy="50"/>
</svg>
```

**A11y:** `<title>`, `<desc>`, `aria-label`,
`aria-roledescription`, `aria-hidden` (surfaced via
`SvgParsed.A11y` and `SvgCfg.A11Y*` overrides).

**Not supported:** `<image>`, `<foreignObject>`,
`<switch>`, `<pattern>`, `<filter>` graph,
`gradientTransform`. Unsupported elements render as the static
first frame or are ignored.

## Key Properties

| Property  | Type         | Description                          |
|-----------|--------------|--------------------------------------|
| FileName  | string       | SVG file path                        |
| SvgData   | string       | Inline SVG string                    |
| Width     | float32      | Display width (0 = native)           |
| Height    | float32      | Display height (0 = native)          |
| Color     | Color        | Override fill (for monochrome icons) |
| NoAnimate | bool         | Disable SMIL + CSS animation         |
| Sizing    | Sizing       | Combined axis sizing mode            |
| Padding   | Opt[Padding] | Inner padding                        |

## Events

| Callback | Signature                          | Fired when       |
|----------|------------------------------------|------------------|
| OnClick  | func(*Layout, *Event, *Window)     | SVG clicked      |

## Accessibility

| Property        | Type   | Description                          |
|-----------------|--------|--------------------------------------|
| A11YLabel       | string | Accessible label (overrides title)   |
| A11YDescription | string | Accessible description (overrides desc) |

`<title>`/`<desc>` and `aria-*` attributes inside the SVG
populate `SvgParsed.A11y` and feed the platform a11y tree when no
`A11YLabel` override is set.
