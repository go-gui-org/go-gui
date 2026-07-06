package gui

import "github.com/go-gui-org/go-glyph"

// RenderKind identifies the type of drawing command stored in a
// RenderCmd. The renderer pipeline emits a flat []RenderCmd slice;
// the dispatch loop switches on Kind to draw each command.
type RenderKind uint8

// RenderKind values.
const (
	RenderNone RenderKind = iota
	RenderClip
	RenderRect
	RenderStrokeRect
	RenderCircle
	RenderImage
	RenderText
	RenderLine
	RenderShadow
	RenderBlur
	RenderGradient
	RenderGradientBorder
	RenderSvg
	RenderLayout
	RenderLayoutTransformed
	RenderLayoutPlaced
	RenderFilterBegin
	RenderFilterEnd
	RenderFilterComposite
	RenderCustomShader
	RenderTextPath
	RenderRTF
	RenderRotateBegin
	RenderRotateEnd
	RenderStencilBegin
	RenderStencilEnd
	RenderTermGrid
)

// RenderCmd is a flat discriminated struct holding all draw
// command variants. Kind selects which fields are meaningful.
// Stored in a pre-allocated slice reused via renderers[:0] each
// frame to minimize heap allocations.
type RenderCmd struct {
	ColorMatrix *[16]float32 // FilterBegin: color transform

	// Pointer fields.
	Shader          *Shader
	Gradient        *GradientDef
	TextStylePtr    *TextStyle            // full text style (typeface, etc.)
	TextGradient    *glyph.GradientConfig // text gradient for glyph-layout draws
	TextPath        *TextPathData         // SVG textPath placement data
	TermGrid        *TermGridData         // terminal grid buffer (RenderTermGrid)
	LayoutPtr       *glyph.Layout         // pre-shaped glyph layout
	LayoutTransform *glyph.AffineTransform

	// String data.
	Text     string // Text
	FontName string // Text font family
	Resource string // Image file path

	// Slice data (Svg).
	Triangles    []float32
	VertexColors []Color
	ClipGroup    int // Svg clip group id
	Layers       int // FilterComposite
	GroupIdx     int // FilterBegin

	// Position/size — used by most kinds.
	X, Y float32
	W, H float32

	Radius float32

	// Type-specific numerics.
	Thickness  float32 // StrokeRect, GradientBorder
	BlurRadius float32 // Shadow, Blur
	Scale      float32 // Svg, FilterBegin
	OffsetX    float32 // Shadow; Line X1
	OffsetY    float32 // Shadow; Line Y1
	ClipRadius float32 // Image
	FontSize   float32 // Text font size (points)
	FontAscent float32 // Text font ascent (pixels)
	TextWidth  float32 // Text source width (pixels)

	// SVG animation rotation (degrees, center in SVG space).
	RotAngle float32
	RotCX    float32
	RotCY    float32

	// SVG animateTransform translate + scale. Applied to each
	// vertex as v' = (vx*ScaleX + TransX, vy*ScaleY + TransY)
	// before rotation. Only honored when HasXform is true.
	TransX float32
	TransY float32
	ScaleX float32
	ScaleY float32
	// Optional multiplier for SVG vertex alpha (0..1) to avoid
	// per-frame vertex color copies when animating opacity.
	VertexAlphaScale float32

	// Visual properties.
	Color Color
	Kind  RenderKind

	StencilDepth uint8 // StencilBegin/End

	// Flags.
	Fill       bool // Rect fill, Circle fill
	IsClipMask bool // Svg stencil mask
	HasXform   bool

	HasVertexAlpha bool
}

// TextPathData holds pre-computed path data for RenderTextPath.
type TextPathData struct {
	Polyline []float32 // flattened path [x0,y0, x1,y1, ...]
	Table    []float32 // cumulative arc-length table
	TotalLen float32
	Offset   float32           // resolved start offset (screen coords)
	Anchor   SvgTextAnchor     // text-anchor alignment
	Method   SvgTextPathMethod // glyph placement method
}

// filterBracketRange describes a matched DrawFilterBegin..DrawFilterEnd
// range within the renderers slice.
type filterBracketRange struct {
	StartIdx int
	EndIdx   int
	NextIdx  int
	FoundEnd bool
}

// findFilterBracketRange scans renderers from startIdx looking for
// a DrawFilterBegin..DrawFilterEnd pair.
// Precondition: filter brackets do not nest. w.inFilter prevents
// nested RenderFilterBegin emissions during layout rendering.
func findFilterBracketRange(renderers []RenderCmd, startIdx int) filterBracketRange {
	for i := startIdx; i < len(renderers); i++ {
		if renderers[i].Kind == RenderFilterEnd {
			return filterBracketRange{
				StartIdx: startIdx,
				EndIdx:   i,
				NextIdx:  i + 1,
				FoundEnd: true,
			}
		}
	}
	return filterBracketRange{
		StartIdx: startIdx,
		EndIdx:   len(renderers),
		NextIdx:  len(renderers),
		FoundEnd: false,
	}
}
