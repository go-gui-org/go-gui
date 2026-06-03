package gui

import "github.com/go-gui-org/go-glyph"

// scratch_pools.go — reusable per-frame buffers. Zero-value valid.

// scratchSlice is a reusable slice pool with retain/shrink thresholds.
type scratchSlice[T any] struct {
	buf       []T
	retainMax int
	shrinkTo  int
}

func (s *scratchSlice[T]) take(requiredCap int) []T {
	b := s.buf
	b = b[:0]
	if cap(b) < requiredCap {
		b = make([]T, 0, requiredCap)
	}
	return b
}

func (s *scratchSlice[T]) put(b []T) {
	if cap(b) > s.retainMax {
		b = make([]T, 0, s.shrinkTo)
	}
	s.buf = b[:0]
}

// scratchMap is a reusable map pool with a retain threshold.
type scratchMap[K comparable, V any] struct {
	m         map[K]V
	retainMax int
}

func (s *scratchMap[K, V]) take(requiredCap int) map[K]V {
	m := s.m
	if m == nil {
		requiredCap = max(requiredCap, 8)
		m = make(map[K]V, requiredCap)
	}
	clear(m)
	return m
}

func (s *scratchMap[K, V]) put(m map[K]V) {
	if len(m) > s.retainMax {
		s.m = nil
		return
	}
	s.m = m
}

// scratchObjPool is a reusable pool of individually heap-allocated
// objects. Pointers handed out remain valid until reset. On reuse,
// existing allocations are overwritten; new ones are appended.
type scratchObjPool[T any] struct {
	items     []*T
	used      int
	retainMax int
	shrinkTo  int
}

func (p *scratchObjPool[T]) alloc(src T) *T {
	idx := p.used
	p.used++
	if idx < len(p.items) {
		*p.items[idx] = src
		return p.items[idx]
	}
	cp := src
	ptr := &cp
	p.items = append(p.items, ptr)
	return ptr
}

func (p *scratchObjPool[T]) reset() {
	p.used = 0
	if p.retainMax > 0 && len(p.items) > p.retainMax {
		p.items = make([]*T, 0, p.shrinkTo)
	}
}

// scratchPools holds reusable per-frame buffers.
type scratchPools struct {
	filterRenderers     scratchSlice[RenderCmd]
	focusCandidates     scratchSlice[focusCandidate]
	gradientNormStops   scratchSlice[GradientStop]
	gradientSampleStops scratchSlice[GradientStop]
	svgAnimVals         scratchSlice[float32]
	wrapRows            scratchSlice[wrapRowRange]
	layerLayouts        scratchSlice[Layout]

	focusSeen        scratchMap[uint32, struct{}]
	svgAnimStates    scratchMap[uint32, svgAnimState]
	svgAnimOverrides scratchMap[uint32, SvgAnimAttrOverride]
	svgAnimByPID     scratchMap[uint32, []float32]

	svgAnimTriangles scratchSlice[TessellatedPath]
	svgAnimContribs  scratchSlice[animContrib]

	// svgVColArena is a grow-only, frame-scoped arena for per-path
	// vertex color buffers emitted by emitSvgPathRenderer. Each
	// call reserves a subslice via takeVColors; the arena is reset
	// to len=0 in resetRenderPools. Realloc is safe because Go
	// retains the old backing array via any emitted subslices that
	// still reference it.
	svgVColArena []Color

	// View-phase pool: reuse Shape allocations across frames.
	// Reset before GenerateViewLayout; valid through buildRenderers.
	viewShapes   scratchObjPool[Shape]
	buttonColors scratchObjPool[ShapeButtonColors]

	// Render-phase pools: reuse heap objects whose addresses are
	// stored in RenderCmd pointer fields (avoids per-frame escapes).
	renderTextStyles       scratchObjPool[TextStyle]
	renderGlyphLayouts     scratchObjPool[glyph.Layout]
	renderAffineTransforms scratchObjPool[glyph.AffineTransform]

	// Reusable events for layoutHover and gesture callbacks
	// (avoids per-shape/per-gesture heap allocation of Event).
	hoverEvent   Event
	gestureEvent Event

	floatingLayouts      []*Layout
	floatingLayoutPool   []*Layout
	floatingPoolUsed     int
	placeholderShapePool []*Shape
	placeholderPoolUsed  int
}

const (
	scratchFloatingLayoutsRetainMax = 4096
	scratchFloatingLayoutsShrinkTo  = 256
	scratchFloatingPoolRetainMax    = 512
	scratchFloatingPoolShrinkTo     = 64
	scratchPlaceholderPoolRetainMax = 4096
	scratchPlaceholderPoolShrinkTo  = 256
)

func newScratchPools() scratchPools {
	return scratchPools{
		filterRenderers:        scratchSlice[RenderCmd]{retainMax: 131_072, shrinkTo: 8192},
		focusCandidates:        scratchSlice[focusCandidate]{retainMax: 4096, shrinkTo: 512},
		gradientNormStops:      scratchSlice[GradientStop]{retainMax: 64, shrinkTo: 8},
		gradientSampleStops:    scratchSlice[GradientStop]{retainMax: 64, shrinkTo: 8},
		svgAnimVals:            scratchSlice[float32]{retainMax: 32, shrinkTo: 8},
		wrapRows:               scratchSlice[wrapRowRange]{retainMax: 4096, shrinkTo: 256},
		layerLayouts:           scratchSlice[Layout]{retainMax: 4096, shrinkTo: 256},
		focusSeen:              scratchMap[uint32, struct{}]{retainMax: 4096},
		svgAnimStates:          scratchMap[uint32, svgAnimState]{retainMax: 4096},
		svgAnimOverrides:       scratchMap[uint32, SvgAnimAttrOverride]{retainMax: 4096},
		svgAnimByPID:           scratchMap[uint32, []float32]{retainMax: 256},
		svgAnimTriangles:       scratchSlice[TessellatedPath]{retainMax: 1024, shrinkTo: 64},
		svgAnimContribs:        scratchSlice[animContrib]{retainMax: 1024, shrinkTo: 64},
		viewShapes:             scratchObjPool[Shape]{retainMax: 16384, shrinkTo: 1024},
		buttonColors:           scratchObjPool[ShapeButtonColors]{retainMax: 512, shrinkTo: 32},
		renderTextStyles:       scratchObjPool[TextStyle]{retainMax: 4096, shrinkTo: 256},
		renderGlyphLayouts:     scratchObjPool[glyph.Layout]{retainMax: 1024, shrinkTo: 64},
		renderAffineTransforms: scratchObjPool[glyph.AffineTransform]{retainMax: 256, shrinkTo: 16},
	}
}

// resetViewPools resets the view-phase object pools. Called
// before GenerateViewLayout.
func (p *scratchPools) resetViewPools() {
	p.viewShapes.reset()
	p.buttonColors.reset()
}

// resetRenderPools resets the render-phase object pools. Called at the
// start of each frame before building the render command list.
// svgVColArena is shrunk when it has grown past svgVColRetainMax so a
// one-off spike frame does not hold hundreds of KB of vertex-color
// capacity indefinitely.
func (p *scratchPools) resetRenderPools() {
	p.renderTextStyles.reset()
	p.renderGlyphLayouts.reset()
	p.renderAffineTransforms.reset()
	if cap(p.svgVColArena) > svgVColRetainMax {
		p.svgVColArena = make([]Color, 0, svgVColShrinkTo)
	} else {
		p.svgVColArena = p.svgVColArena[:0]
	}
}

const (
	svgVColRetainMax = 1 << 14 // 16 384 colors (~64KB)
	svgVColShrinkTo  = 1 << 10 // 1 024 colors (~4KB)
)

// maxVColReservation bounds how much the arena is allowed to
// grow for a single reservation. Beyond this, a standalone slice
// is returned so arena memory is not held across frames and no
// arithmetic overflow can occur. A reasonable tessellated path
// carries hundreds-to-low-thousands of vertices; the cap is
// generous enough that normal content never hits it.
const maxVColReservation = 1 << 20

// takeVColors reserves a subslice of n Colors from the frame-
// scoped vertex-color arena. The returned slice has its cap
// pinned so subsequent appends by the caller cannot bleed into
// the next reservation. Realloc of the underlying arena is safe:
// prior reservations remain valid because their slice headers
// keep the old backing array alive. Non-positive n returns nil;
// pathological sizes bypass the arena entirely.
func (p *scratchPools) takeVColors(n int) []Color {
	if n <= 0 {
		return nil
	}
	if n > maxVColReservation {
		return make([]Color, n)
	}
	start := len(p.svgVColArena)
	need := start + n
	if cap(p.svgVColArena) < need {
		grown := make([]Color, need, growCap(cap(p.svgVColArena), need))
		copy(grown, p.svgVColArena)
		p.svgVColArena = grown
	} else {
		p.svgVColArena = p.svgVColArena[:need]
	}
	return p.svgVColArena[start:need:need]
}

// growCap returns a new capacity at least need, roughly doubling
// from oldCap to amortize arena growth. Guards against overflow
// of oldCap*2 on 32-bit platforms or pathological sizes.
func growCap(oldCap, need int) int {
	doubled := oldCap * 2
	if doubled < oldCap {
		doubled = need
	}
	return max(doubled, need)
}

func (p *scratchPools) takeFloatingLayouts(requiredCap int) []*Layout {
	s := p.floatingLayouts
	s = s[:0]
	if cap(s) < requiredCap {
		s = make([]*Layout, 0, requiredCap)
	}
	p.floatingPoolUsed = 0
	p.placeholderPoolUsed = 0
	return s
}

func (p *scratchPools) putFloatingLayouts(s []*Layout) {
	if cap(s) > scratchFloatingLayoutsRetainMax {
		s = make([]*Layout, 0, scratchFloatingLayoutsShrinkTo)
	}
	p.floatingLayouts = s[:0]
	if len(p.floatingLayoutPool) > scratchFloatingPoolRetainMax {
		p.floatingLayoutPool = make([]*Layout, 0, scratchFloatingPoolShrinkTo)
	}
	if len(p.placeholderShapePool) > scratchPlaceholderPoolRetainMax {
		p.placeholderShapePool = make([]*Shape, 0, scratchPlaceholderPoolShrinkTo)
	}
}

func (p *scratchPools) allocFloatingLayout(src Layout) *Layout {
	idx := p.floatingPoolUsed
	p.floatingPoolUsed++
	if idx < len(p.floatingLayoutPool) {
		reused := p.floatingLayoutPool[idx]
		*reused = src
		return reused
	}
	cp := src
	allocated := &cp
	p.floatingLayoutPool = append(p.floatingLayoutPool, allocated)
	return allocated
}

func (p *scratchPools) allocPlaceholderShape() *Shape {
	idx := p.placeholderPoolUsed
	p.placeholderPoolUsed++
	if idx < len(p.placeholderShapePool) {
		reused := p.placeholderShapePool[idx]
		*reused = Shape{ShapeType: ShapeNone}
		return reused
	}
	allocated := &Shape{ShapeType: ShapeNone}
	p.placeholderShapePool = append(p.placeholderShapePool, allocated)
	return allocated
}
