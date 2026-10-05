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

// getBatch returns the open batch when the run-length merge key
// still holds — same color and same transform — and opens a fresh
// one otherwise.
func (dc *DrawContext) getBatch(color Color) *DrawCanvasTriBatch {
	// The transform joins the run-length merge key: a batch carries
	// one matrix for all its triangles, so a transform change must
	// start a new batch. Compared against the live batch rather than
	// a mirror field so there is one source of truth.
	xf, hasXf := dc.activeXform()
	if dc.mixColors {
		return dc.getMixedBatch(color, xf, hasXf)
	}
	if len(dc.batches) > 0 && !dc.batchIsGradient &&
		dc.lastColor == color &&
		dc.batches[dc.currentBatchIdx].hasXform == hasXf &&
		dc.batches[dc.currentBatchIdx].xf == xf {
		return &dc.batches[dc.currentBatchIdx]
	}
	b := dc.takeBatch(color, false, defaultBatchVerts)
	dc.lastColor = color
	dc.batchIsGradient = false
	return b
}

// getMixedBatch is getBatch with mixColors on (#945): the merge key is
// the transform alone, and color moves into the per-vertex channel.
//
// The primitive appends its triangles after this returns, so their
// colors cannot be written here. The previous primitive's vertices
// are padded now, from lastColor, and this one's color becomes the
// new lastColor for the next settle. The draw order is kept exactly:
// marks land in the batch in the order they were drawn, and a GPU
// blends the triangles of one draw call in submit order, so the frame
// matches the one a batch per color would have drawn.
//
// The mode is off by default and resetFor turns it off before every
// redraw. An out-of-package reader of Batches() that only looks at
// Color (go-charts' PNG export) would paint a merged batch in one
// color, so only a widget that owns its canvas turns it on.
//
// A merged batch's VertexColors lag its Triangles: the vertices the
// last primitive appended have no colors until settleMixed pads them.
func (dc *DrawContext) getMixedBatch(color Color, xf canvasXform,
	hasXf bool) *DrawCanvasTriBatch {
	if len(dc.batches) > 0 && dc.batchIsMixed &&
		dc.batches[dc.currentBatchIdx].hasXform == hasXf &&
		dc.batches[dc.currentBatchIdx].xf == xf {
		dc.settleMixed()
		dc.lastColor = color
		return &dc.batches[dc.currentBatchIdx]
	}
	// takeBatch settles the batch being closed. The gradient flag is
	// what claims a VertexColors buffer from the pool; the batch is
	// not a gradient batch to the merge key, batchIsMixed is.
	b := dc.takeBatch(color, true, defaultBatchVerts)
	b.flatColors = true
	dc.lastColor = color
	dc.batchIsGradient = false
	dc.batchIsMixed = true
	return b
}

// settleMixed pads the open merged batch's VertexColors up to its
// vertex count with lastColor, the color of the primitive that
// appended those vertices. It is a no-op when the open batch is not a
// merged one, and when it is already settled.
//
// Every path that leaves the open batch must call it first: opening
// another batch (takeBatch), breaking the run (breakBatchRun), and
// handing the batches out (Batches, and renderDrawCanvas after
// OnDraw). A batch left unsettled has fewer colors than vertices, and
// validSvgCmd drops it whole.
func (dc *DrawContext) settleMixed() {
	if !dc.batchIsMixed || len(dc.batches) == 0 {
		return
	}
	b := &dc.batches[dc.currentBatchIdx]
	for n := len(b.Triangles) / 2; len(b.VertexColors) < n; {
		b.VertexColors = append(b.VertexColors, dc.lastColor)
	}
}

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
	// The transform is stamped once per batch, not per vertex: the
	// primitives append local coordinates and the matrix travels with
	// the command. getBatch keeps a transform change from merging two
	// matrices into one batch.
	// Settled while it is still the open batch: after the append below
	// currentBatchIdx names the new one.
	dc.settleMixed()
	dc.batchIsMixed = false
	nb := DrawCanvasTriBatch{Color: color}
	nb.xf, nb.hasXform = dc.activeXform()
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

// breakBatchRun closes the current batch to the run-length merge, so
// the next primitive opens a fresh one whatever its color.
//
// It exists for the lowered radial gradient, which records no batch of
// its own but does take a position in the emit order. Without it the
// merge key — color plus transform — cannot see that something was
// drawn in between, and geometry recorded after the fill lands in a
// batch emitted before it.
func (dc *DrawContext) breakBatchRun() {
	// Before lastColor is cleared: it is the color the open merged
	// batch's last vertices are owed.
	dc.settleMixed()
	dc.batchIsMixed = false
	dc.lastColor = Color{}
	// A gradient batch never merges, so this alone stops the next
	// primitive reaching the batch that is open now.
	dc.batchIsGradient = true
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

// carryUnclaimed moves the pooled buffers this redraw did not claim
// into the backing arrays past the emitted lengths, and reports how far
// each array's owned headers now reach (#940).
//
// Without it a redraw that emits fewer batches than the one before
// drops the rest of the pool: it is the array the next redraw writes
// headers over. A canvas whose batch count moves every frame — the
// ThinkingOrb, whose ink alpha changes the color run-lengths — then
// allocates the dropped buffers new each time the count goes back up.
//
// The carried buffers are disjoint from the emitted ones, since each
// pool entry is claimed at most once, so ownership stays single. The
// cost is that a canvas keeps the buffers of its largest redraw for as
// long as its cache entry lives, the same retention takeBatch already
// accepts for a single large buffer.
//
// append grows the header array only when the peak count grows, so the
// steady state is allocation-free.
func (dc *DrawContext) carryUnclaimed() (batchHigh, gradHigh int) {
	if n := len(dc.batches); n < len(dc.batchPool) {
		dc.batches = append(dc.batches, dc.batchPool[n:]...)
		batchHigh = len(dc.batches)
		dc.batches = dc.batches[:n]
	}
	if n := len(dc.gradients); n < len(dc.gradientPool) {
		dc.gradients = append(dc.gradients, dc.gradientPool[n:]...)
		gradHigh = len(dc.gradients)
		dc.gradients = dc.gradients[:n]
	}
	return batchHigh, gradHigh
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

	// The pool runs past len(prev.Batches) to the headers the last
	// redraw carried forward unclaimed; see carryUnclaimed.
	dc.batchPool = prev.Batches[:max(len(prev.Batches), prev.batchHigh)]
	dc.batches = prev.spare[:0]
	dc.gradientPool = prev.Gradients[:max(len(prev.Gradients), prev.gradHigh)]
	dc.gradients = prev.gradSpare[:0]
	// Cleared, not just truncated. Both entry types hold pointer-shaped
	// fields — a Text string, an Image Src and its ImageFetcher, which
	// can be a closure over an arbitrary graph — and the entries past
	// this redraw's length stay in the backing array for as long as the
	// canvas lives. A canvas that drew ten thousand labels once would
	// otherwise pin all ten thousand strings forever. The capacity is
	// kept either way, which is the whole point of reusing the array.
	clear(prev.Texts)
	clear(prev.Images)
	dc.texts = prev.Texts[:0]
	dc.images = prev.Images[:0]

	dc.lastColor = Color{}
	dc.batchIsGradient = false
	dc.mixColors = false
	dc.batchIsMixed = false
	dc.currentBatchIdx = 0
	// The single reset point for the transform. It runs immediately
	// before every OnDraw, so an unbalanced Save cannot survive into
	// the next redraw or into another canvas sharing this context.
	dc.resetXform()

	dc.arcBuf = keepScratch(dc.arcBuf)
	dc.xfPtBuf = keepScratch(dc.xfPtBuf)
	dc.roundRectBuf = keepScratch(dc.roundRectBuf)
	dc.joinNormalBuf = keepScratch(dc.joinNormalBuf)
	dc.joinOffsetBuf = keepScratch(dc.joinOffsetBuf)
	dc.bezierBuf = keepScratch(dc.bezierBuf)
	dc.gradTriBuf = keepScratch(dc.gradTriBuf)
	dc.gradSplitBuf = keepScratch(dc.gradSplitBuf)
	dc.gradRadialBuf = keepScratch(dc.gradRadialBuf)
	dc.gradIsolineBuf = keepScratch(dc.gradIsolineBuf)
	dc.gradOffsetBuf = keepScratch(dc.gradOffsetBuf)
	dc.gradStopBuf = keepScratch(dc.gradStopBuf)
	dc.gradSampleBuf = keepScratch(dc.gradSampleBuf)
	dc.gradRampBuf = keepScratch(dc.gradRampBuf)
	dc.gradRingBuf = keepScratch(dc.gradRingBuf)
	dc.pathFlatBuf = keepScratch(dc.pathFlatBuf)
	dc.pathContourBuf = keepScratch(dc.pathContourBuf)
	dc.pathScratch.Edges = keepScratch(dc.pathScratch.Edges)
	dc.pathScratch.Ys = keepScratch(dc.pathScratch.Ys)
	dc.pathScratch.Active = keepScratch(dc.pathScratch.Active)
	dc.pathScratch.Indices = keepScratch(dc.pathScratch.Indices)
}
