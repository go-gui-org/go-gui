package gui

// canvas_draw_batch.go — batch allocation and context lifetime for
// DrawCanvas.
//
// A DrawContext is rebuilt for every redraw of every canvas, and an
// animated canvas redraws every frame. Nothing here changes what is
// tessellated; it decides where the tessellation lands, so that the
// second and later redraws of the same canvas write into the buffers
// the previous one left behind instead of allocating a new set.

// defaultBatchVerts is the vertex count a flat batch reserves for,
// matching the cap 128 the pre-pool code allocated.
const defaultBatchVerts = 64

// takeBatch appends a batch and gives it the buffers the previous
// redraw's batch at the same index left behind.
//
// Index alignment is what makes this work: one canvas emits its
// primitives in the same order every frame, so batch i is nearly always
// the same batch it was last time and its buffers are already the right
// size. When the order does shift the only cost is a resize.
//
// Ownership stays single. The pool is the outgoing cache entry, which
// renderDrawCanvas discards once the redraw returns, so a buffer that
// is claimed here has exactly one live referent afterwards.
//
// numVerts sizes a fresh allocation when nothing is poolable; gradient
// batches also claim a VertexColors buffer, flat ones leave it nil so
// they stay distinguishable from a gradient batch carrying no colors.
func (dc *DrawContext) takeBatch(color Color, gradient bool,
	numVerts int) *DrawCanvasTriBatch {
	nb := DrawCanvasTriBatch{Color: color}
	if i := len(dc.batches); i < len(dc.batchPool) {
		// Reused at any size, deliberately. A cap on it would only
		// force a reallocation: the cache entry holds this geometry
		// until the canvas is redrawn either way, so refusing to
		// recycle a large buffer releases nothing and makes the
		// heaviest canvases — the ones the pooling is for — the only
		// ones that keep allocating.
		p := &dc.batchPool[i]
		nb.Triangles = p.Triangles[:0]
		if gradient {
			nb.VertexColors = p.VertexColors[:0]
		}
	}
	if nb.Triangles == nil {
		nb.Triangles = make([]float32, 0, numVerts*2)
	}
	if gradient && nb.VertexColors == nil {
		nb.VertexColors = make([]Color, 0, numVerts)
	}
	dc.batches = append(dc.batches, nb)
	dc.currentBatchIdx = len(dc.batches) - 1
	return &dc.batches[dc.currentBatchIdx]
}

// takeGradient appends a gradient entry and gives it the stop buffer
// the previous redraw's entry at the same index left behind, on the
// same index-alignment argument as takeBatch: one canvas records its
// fills in the same order every frame.
func (dc *DrawContext) takeGradient() *DrawCanvasGradientEntry {
	var ne DrawCanvasGradientEntry
	if i := len(dc.gradients); i < len(dc.gradientPool) {
		ne.Def.Stops = dc.gradientPool[i].Def.Stops[:0]
	}
	dc.gradients = append(dc.gradients, ne)
	return &dc.gradients[len(dc.gradients)-1]
}

// resetFor rebinds this context to one canvas's redraw, reusing
// everything the previous redraw left behind so an animated canvas
// tessellates without allocating.
//
// Batches are pooled rather than overwritten: prev.Batches still holds
// the buffers the last redraw filled, which takeBatch claims one by
// one, while the writing happens in prev.spare. The two arrays swap
// roles every redraw. Texts and Images need no such care — nothing
// downstream keeps a reference into those arrays past the frame that
// emitted them, so the arrays are simply refilled.
//
// Those buffers are aliased by the previous frame's RenderCmds, which
// is safe because the frame loop is single-threaded and buildRenderers
// has already dropped that command list before this runs. See the note
// on DrawCanvasTriBatch about export paths that copy commands out.
//
// The scratch buffers keep their capacity across redraws, and across
// canvases, up to the retain cap; only a run-away one is released.
func (dc *DrawContext) resetFor(w, h, scale float32, tm TextMeasurer,
	prev drawCanvasCache) {
	dc.Width, dc.Height, dc.Scale = w, h, scale
	dc.textMeasure = tm
	dc.recorder = nil

	dc.batchPool = prev.Batches
	dc.batches = prev.spare[:0]
	dc.gradientPool = prev.Gradients
	dc.gradients = prev.gradSpare[:0]
	dc.texts = prev.Texts[:0]
	dc.images = prev.Images[:0]

	dc.lastColor = Color{}
	dc.batchIsGradient = false
	dc.currentBatchIdx = 0

	dc.arcBuf = keepScratch(dc.arcBuf)
	dc.bezierBuf = keepScratch(dc.bezierBuf)
	dc.gradTriBuf = keepScratch(dc.gradTriBuf)
	dc.gradSplitBuf = keepScratch(dc.gradSplitBuf)
	dc.gradRadialBuf = keepScratch(dc.gradRadialBuf)
	dc.gradIsolineBuf = keepScratch(dc.gradIsolineBuf)
	dc.gradOffsetBuf = keepScratch(dc.gradOffsetBuf)
	dc.gradStopBuf = keepScratch(dc.gradStopBuf)
	dc.gradSampleBuf = keepScratch(dc.gradSampleBuf)
	dc.gradRingBuf = keepScratch(dc.gradRingBuf)
}
