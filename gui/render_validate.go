package gui

// Security caps for render commands. Invalid oversized commands
// are dropped by emitRendererIfValid before they reach backends,
// which would otherwise allocate from attacker-controlled lengths
// (DrawCanvas batches, crafted RenderCmd).
const (
	// maxSvgTriangleFloats caps RenderSvg.Triangles length in
	// floats. 200k triangles (~4.8MB) sits above the SVG
	// maxPathSegments=100k budget with headroom for tessellation
	// fan-out, while bounding the per-frame vertex allocation in
	// every backend's drawSvg path.
	maxSvgTriangleFloats = 1_200_000
	// maxRenderTextLen caps RenderText.Text in bytes. SVG text runs
	// cap at 64KB; the non-SVG path had no cap, letting one string
	// drive unbounded shaping work.
	maxRenderTextLen = 1 << 20
	// maxFilterCompositeLayers mirrors soft maxFilterLayers: past a
	// handful the glow is saturated and each extra pass is a
	// full-layer blend.
	maxFilterCompositeLayers = 32
	// maxShadowSpread bounds RenderShadow.Spread, which was
	// finite-checked but unbounded and drives blur working-rect
	// growth in every backend.
	maxShadowSpread = float32(1000000)
)

// rendererValidForDraw checks whether a RenderCmd has valid
// parameters for drawing. Returns false for NaN/Inf coordinates,
// negative sizes, nil pointers, etc.
func rendererValidForDraw(r RenderCmd) bool {
	switch r.Kind {
	case RenderClip:
		return validClipCmd(r)
	case RenderRect:
		return validRectCmd(r)
	case RenderStrokeRect:
		return validStrokeRectCmd(r)
	case RenderGradient:
		return validGradientCmd(r)
	case RenderCircle:
		return validCircleCmd(r)
	case RenderText:
		return validTextCmd(r)
	case RenderLine:
		return validLineCmd(r)
	case RenderLayout:
		return validLayoutCmd(r)
	case RenderRTF:
		return validRTFCmd(r)
	case RenderTextPath:
		return validTextPathCmd(r)
	case RenderTermGrid:
		return validTermGridCmd(r)
	case RenderLayoutTransformed:
		return validLayoutTransformedCmd(r)
	case RenderImage:
		return validImageCmd(r)
	case RenderSvg:
		return validSvgCmd(r)
	case RenderFilterBegin:
		return validFilterBeginCmd(r)
	case RenderFilterComposite:
		return validFilterCompositeCmd(r)
	case RenderStencilBegin, RenderStencilEnd:
		return validStencilCmd(r)
	case RenderGradientBorder:
		return validGradientBorderCmd(r)
	case RenderShadow:
		return validShadowCmd(r)
	case RenderBlur:
		return validBlurCmd(r)
	case RenderCustomShader:
		return validCustomShaderCmd(r)
	case RenderRotateBegin:
		return validRotateBeginCmd(r)
	// Bracket ends and markers carry no validatable payload beyond
	// the kind itself. Listed explicitly so a new kind is a
	// compile-visible decision rather than a silent default-true.
	case RenderNone, RenderFilterEnd, RenderRotateEnd,
		RenderLayoutPlaced:
		return true
	default:
		return true
	}
}

func validClipCmd(r RenderCmd) bool {
	return f32AllFinite4(r.X, r.Y, r.W, r.H) &&
		r.W >= 0 && r.H >= 0
}

func validRectCmd(r RenderCmd) bool {
	return f32AllFinite5(r.X, r.Y, r.W, r.H, r.Radius) &&
		r.W >= 0 && r.H >= 0
}

func validStrokeRectCmd(r RenderCmd) bool {
	return f32AllFinite6(r.X, r.Y, r.W, r.H, r.Radius, r.Thickness) &&
		r.W >= 0 && r.H >= 0 && r.Thickness > 0
}

func validGradientCmd(r RenderCmd) bool {
	return f32AllFinite5(r.X, r.Y, r.W, r.H, r.Radius) &&
		r.W >= 0 && r.H >= 0 && r.Gradient != nil
}

func validCircleCmd(r RenderCmd) bool {
	return f32AllFinite3(r.X, r.Y, r.Radius) && r.Radius > 0
}

func validTextCmd(r RenderCmd) bool {
	if !f32AllFinite2(r.X, r.Y) || len(r.Text) == 0 {
		return false
	}
	if len(r.Text) > maxRenderTextLen {
		return false
	}
	if r.LayoutTransform != nil {
		t := *r.LayoutTransform
		if !f32AllFinite2(t.XX, t.XY) ||
			!f32AllFinite2(t.YX, t.YY) ||
			!f32AllFinite2(t.X0, t.Y0) {
			return false
		}
	}
	return true
}

func validLayoutCmd(r RenderCmd) bool {
	return f32AllFinite2(r.X, r.Y) && r.LayoutPtr != nil
}

func validLayoutTransformedCmd(r RenderCmd) bool {
	if !f32AllFinite2(r.X, r.Y) ||
		r.LayoutPtr == nil || r.LayoutTransform == nil {
		return false
	}
	t := *r.LayoutTransform
	if !f32AllFinite2(t.XX, t.XY) ||
		!f32AllFinite2(t.YX, t.YY) ||
		!f32AllFinite2(t.X0, t.Y0) {
		return false
	}
	return true
}

func validImageCmd(r RenderCmd) bool {
	return f32AllFinite4(r.X, r.Y, r.W, r.H) &&
		r.W > 0 && r.H > 0 && f32IsFinite(r.ClipRadius)
}

func validLineCmd(r RenderCmd) bool {
	return f32AllFinite4(r.X, r.Y, r.OffsetX, r.OffsetY)
}

func validRTFCmd(r RenderCmd) bool {
	return f32AllFinite2(r.X, r.Y) && r.LayoutPtr != nil
}

func validTextPathCmd(r RenderCmd) bool {
	if !f32AllFinite2(r.X, r.Y) || len(r.Text) == 0 {
		return false
	}
	if len(r.Text) > maxRenderTextLen {
		return false
	}
	if r.textPath != nil && !f32AllFinite(r.textPath.Polyline) {
		return false
	}
	return true
}

func validTermGridCmd(r RenderCmd) bool {
	tg := r.TermGrid
	if tg == nil {
		return false
	}
	// Mirrors the soft backend's validTermGrid, including the
	// division form of the cell-count check so a hostile
	// Cols*Rows cannot overflow the check itself.
	return tg.Cols > 0 && tg.Rows > 0 &&
		f32IsFinite(tg.CellW) && tg.CellW > 0 &&
		f32IsFinite(tg.CellH) && tg.CellH > 0 &&
		tg.Rows <= len(tg.Cells)/tg.Cols
}

func validSvgCmd(r RenderCmd) bool {
	if !f32AllFinite3(r.X, r.Y, r.Scale) || r.Scale <= 0 {
		return false
	}
	// The xform is applied per vertex in every backend, so a NaN
	// scale would poison every triangle rather than drop one command.
	if r.HasXform &&
		!f32AllFinite4(r.ScaleX, r.ScaleY, r.TransX, r.TransY) {
		return false
	}
	if r.HasVertexAlpha &&
		(!f32IsFinite(r.VertexAlphaScale) ||
			r.VertexAlphaScale < 0 ||
			r.VertexAlphaScale > 1) {
		return false
	}
	if len(r.Triangles) == 0 || len(r.Triangles)%6 != 0 {
		return false
	}
	if len(r.Triangles) > maxSvgTriangleFloats {
		return false
	}
	if !f32AllFinite(r.Triangles) {
		return false
	}
	if len(r.VertexColors) > 0 &&
		len(r.VertexColors)*2 != len(r.Triangles) {
		return false
	}
	return true
}

func validFilterCompositeCmd(r RenderCmd) bool {
	return f32AllFinite4(r.X, r.Y, r.W, r.H) &&
		r.W > 0 && r.H > 0 && r.Layers > 0 &&
		r.Layers <= maxFilterCompositeLayers
}

func validFilterBeginCmd(r RenderCmd) bool {
	if !f32IsFinite(r.BlurRadius) || r.Layers < 1 {
		return false
	}
	// Layers has no upper bound here: the SVG emitter clamps to
	// maxFilterCompositeLayers and every backend clamps again, so
	// a hand-built command degrades to capped glow passes. Dropping
	// the Begin instead would unbalance the Begin/End bracket.
	if r.ColorMatrix != nil && !f32AllFinite(r.ColorMatrix[:]) {
		return false
	}
	return true
}

func validStencilCmd(r RenderCmd) bool {
	return f32AllFinite5(r.X, r.Y, r.W, r.H, r.Radius) &&
		r.W > 0 && r.H > 0 && r.StencilDepth > 0
}

func validGradientBorderCmd(r RenderCmd) bool {
	return f32AllFinite6(r.X, r.Y, r.W, r.H, r.Radius, r.Thickness) &&
		r.W >= 0 && r.H >= 0 && r.Thickness > 0 && r.Gradient != nil
}

func validShadowCmd(r RenderCmd) bool {
	return f32AllFinite6(r.X, r.Y, r.W, r.H, r.BlurRadius, r.Radius) &&
		r.W >= 0 && r.H >= 0 &&
		f32AllFinite2(r.OffsetX, r.OffsetY) &&
		f32IsFinite(r.Spread) && r.Spread >= 0 && r.Spread <= maxShadowSpread
}

func validBlurCmd(r RenderCmd) bool {
	return f32AllFinite5(r.X, r.Y, r.W, r.H, r.BlurRadius) &&
		r.W >= 0 && r.H >= 0
}

func validCustomShaderCmd(r RenderCmd) bool {
	return f32AllFinite4(r.X, r.Y, r.W, r.H) &&
		r.W > 0 && r.H > 0 && r.Shader != nil
}

func validRotateBeginCmd(r RenderCmd) bool {
	return f32AllFinite3(r.RotAngle, r.RotCX, r.RotCY)
}

// f32AllFinite checks if all values in a slice are finite.
func f32AllFinite(values []float32) bool {
	for _, v := range values {
		if !f32IsFinite(v) {
			return false
		}
	}
	return true
}

func f32AllFinite2(a, b float32) bool {
	return f32IsFinite(a) && f32IsFinite(b)
}

func f32AllFinite3(a, b, c float32) bool {
	return f32IsFinite(a) && f32IsFinite(b) && f32IsFinite(c)
}

func f32AllFinite4(a, b, c, d float32) bool {
	return f32IsFinite(a) && f32IsFinite(b) &&
		f32IsFinite(c) && f32IsFinite(d)
}

func f32AllFinite5(a, b, c, d, e float32) bool {
	return f32AllFinite4(a, b, c, d) && f32IsFinite(e)
}

func f32AllFinite6(a, b, c, d, e, f float32) bool {
	return f32AllFinite4(a, b, c, d) &&
		f32IsFinite(e) && f32IsFinite(f)
}

// guardRendererOrSkip returns true if valid. Logs a warning (once
// per kind) for invalid renderers.
func guardRendererOrSkip(r RenderCmd, w *Window) bool {
	if rendererValidForDraw(r) {
		return true
	}
	bit := uint32(1) << r.Kind
	if w.renderGuardWarned&bit == 0 {
		w.renderGuardWarned |= bit
	}
	return false
}

// emitRendererIfValid appends r to the window's renderers if valid.
// Returns true if appended.
func emitRendererIfValid(r RenderCmd, w *Window) bool {
	if !rendererValidForDraw(r) {
		return false
	}
	w.renderers = append(w.renderers, r)
	return true
}

// emitRenderer appends r to the window's renderers, logging a
// warning if the renderer is invalid.
func emitRenderer(r RenderCmd, w *Window) {
	if emitRendererIfValid(r, w) {
		return
	}
	guardRendererOrSkip(r, w)
}
