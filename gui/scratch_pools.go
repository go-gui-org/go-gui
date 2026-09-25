package gui

import (
	"unsafe"

	"github.com/go-gui-org/go-glyph"
)

// scratch_pools.go — reusable per-frame buffers. Zero-value valid.
//
// Every pool in this file is frame-scoped and single-goroutine.
// Only the frame pass that owns the Window touches these pools.
// No other goroutine must read or write them. The animation ticker
// keeps its own state under w.animMu and never reaches in here.

// scratchSlice is a reusable slice pool with retain/shrink thresholds.
// Only one checkout must be live at a time. A second take before the
// first put truncates the same backing array and corrupts the first
// slice. Every current caller pairs one take with one put and never
// nests two takes of one pool.
//
// requiredCap must derive from a resident length, for example the
// length of a slice already in memory. The pool never amplifies an
// allocation past that size, so a corrupt length cannot reach the
// allocator through this path. It can only repeat memory that the
// caller already holds.
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

// canvasScratchRetainMax bounds the capacity a canvas scratch buffer
// may hold between redraws, so one outsized frame does not pin a
// megabyte for the life of the window. Sixteen times svgVColRetainMax,
// because the gradient split buffer for a fill covering the whole
// window already runs to a hundred thousand floats.
//
// Unlike a batch buffer — which the cache entry retains anyway, so
// capping its reuse would release nothing — these are pure overhead
// between frames and worth releasing. The trade is that a canvas
// genuinely needing more than this on *every* frame reallocates on
// every frame; the cap is set well past where any measured content
// lands.
//
// The canonical limit is canvasScratchRetainBytes below. This constant
// stays as the float32-element form of it, because most canvas buffers
// hold float32 and existing tests pin that boundary.
const canvasScratchRetainMax = 1 << 18 // 262 144 floats, ~1 MB

// canvasScratchRetainBytes is the canonical canvas retention limit.
// keepScratch measures every buffer type against it, so a buffer of
// wide structs cannot pin far more memory than a float buffer with
// the same element count.
const canvasScratchRetainBytes = 1 << 20 // 1 MB

// keepScratch returns b emptied for reuse, or nil when its backing
// array holds past the retain cap and should be released instead.
// The cap is measured in bytes, not elements.
func keepScratch[T any](b []T) []T {
	var zero T
	if uint64(cap(b))*uint64(unsafe.Sizeof(zero)) > canvasScratchRetainBytes {
		return nil
	}
	return b[:0]
}

// scratchMap is a reusable map pool with a retain threshold.
// Only one checkout must be live at a time, as with scratchSlice.
type scratchMap[K comparable, V any] struct {
	m         map[K]V
	retainMax int
	// sizedFor is the largest size the retained map was made for or
	// filled to. A Go map never shrinks, so this tracks the capacity
	// it holds. The last fill count does not: a hint that overstates
	// the fill (len(anims) for states keyed by PathID) would then
	// look too large on every frame and reallocate every frame.
	sizedFor int
}

func (s *scratchMap[K, V]) take(requiredCap int) map[K]V {
	m := s.m
	if m == nil {
		requiredCap = max(requiredCap, 8)
		m = make(map[K]V, requiredCap)
		s.sizedFor = requiredCap
	} else if scratchMapShouldRegrow(s.sizedFor, requiredCap) {
		// The retained map is far smaller than the hint. A fresh
		// map avoids repeated incremental growth as the caller
		// fills it.
		m = make(map[K]V, requiredCap)
		s.sizedFor = requiredCap
	}
	clear(m)
	return m
}

// scratchMapShouldRegrow reports whether a retained map sized for
// prevLen entries is far smaller than requiredCap. Pure function, so
// tests can pin the boundary without observing map internals.
func scratchMapShouldRegrow(prevLen, requiredCap int) bool {
	if requiredCap <= 8 {
		return false
	}
	return int64(requiredCap) > int64(prevLen)*4
}

func (s *scratchMap[K, V]) put(m map[K]V) {
	if len(m) > s.retainMax {
		s.m = nil
		s.sizedFor = 0
		return
	}
	s.m = m
	s.sizedFor = max(s.sizedFor, len(m))
}

// scratchObjPool is a reusable pool of individually heap-allocated
// objects. Pointers handed out remain valid until reset. On reuse,
// existing allocations are overwritten; new ones are appended.
// Only the owning frame pass must touch a pool, as with scratchSlice.
type scratchObjPool[T any] struct {
	items     []*T
	used      int
	retainMax int
	shrinkTo  int
}

// Default bounds for a zero-value scratchObjPool. An explicit zero
// retainMax means "use these defaults", not "retain everything".
// Every pool must stay bounded, including one built without
// newScratchPools.
const (
	defaultScratchObjRetainMax = 4096
	defaultScratchObjShrinkTo  = 256
)

// effectiveLimits returns the shrink bounds in force. Explicit
// positive values win. Zero or negative values fall back to the
// defaults above.
func (p *scratchObjPool[T]) effectiveLimits() (retainMax, shrinkTo int) {
	retainMax, shrinkTo = p.retainMax, p.shrinkTo
	if retainMax <= 0 {
		retainMax = defaultScratchObjRetainMax
	}
	if shrinkTo <= 0 {
		shrinkTo = defaultScratchObjShrinkTo
	}
	return retainMax, shrinkTo
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
	retainMax, shrinkTo := p.effectiveLimits()
	if len(p.items) > retainMax {
		// Exceeded absolute cap — shrink hard.
		p.items = make([]*T, 0, shrinkTo)
	} else if len(p.items) > shrinkTo && p.used < len(p.items)/4 {
		// Usage far below capacity (< 25%); release memory.
		p.items = make([]*T, 0, shrinkTo)
	}
	p.used = 0
}

// scratchPools holds reusable per-frame buffers.
type scratchPools struct {
	focusSeen        scratchMap[string, struct{}]
	svgAnimStates    scratchMap[uint32, svgAnimState]
	svgAnimOverrides scratchMap[uint32, SvgAnimAttrOverride]
	svgAnimByPID     scratchMap[uint32, []float32]

	// svgVColArena is a grow-only, frame-scoped arena for per-path
	// vertex color buffers emitted by emitSvgPathRenderer. Each
	// call reserves a subslice via takeVColors; the arena is reset
	// to len=0 in resetRenderPools. Realloc is safe because Go
	// retains the old backing array via any emitted subslices that
	// still reference it.
	svgVColArena []Color

	// textItemArena and textGlyphArena are render-phase arenas for text
	// layouts that renderText hands the backend in an altered form: a
	// cached layout recoloured for a faded or disabled frame
	// (recolorLayoutItems) and a typewriter's partly revealed glyph run
	// (revealLayoutGlyphs). The cached layout itself must stay as it
	// was shaped, so the altered slices are copies. Reset with
	// svgVColArena, and safe to realloc for the same reason.
	textItemArena  []glyph.Item
	textGlyphArena []glyph.Glyph

	// layoutChildrenArena is a grow-only, frame-scoped arena for the
	// []Layout child slices built by generateViewLayout. Each node
	// reserves a pinned subslice via takeLayoutChildren; the arena is
	// reset to len=0 in resetViewPools. Realloc mid-build is safe by
	// the same argument as svgVColArena: a parent that reserved before
	// a realloc keeps the old backing array alive through its slice
	// header, and per-node Children headers stay internally consistent.
	layoutChildrenArena []Layout

	// viewArena is a grow-only, frame-scoped arena for the []View
	// slices a widget builds for its children during the view phase.
	// It exists beside layoutChildrenArena rather than reusing it
	// because that one hands out []Layout, and beside a scratchSlice
	// because a scratchSlice hands out one shared buffer that nested
	// lists would alias. Reset to len=0 in resetViewPools; the
	// realloc-safety argument is layoutChildrenArena's.
	viewArena []View

	floatingLayouts      []*Layout
	floatingLayoutPool   []*Layout
	placeholderShapePool []*Shape
	focusCandidates      scratchSlice[focusCandidate]
	wrapRows             scratchSlice[wrapRowRange]
	// layerLayouts hands out one slice per frame in layoutArrange.
	// That slice stays live as w.layout.Children until the next
	// frame puts it back (see window_update.go). Take must run at
	// most once per frame. A second take would truncate the same
	// backing array and corrupt the live tree.
	layerLayouts scratchSlice[Layout]

	svgAnimTriangles scratchSlice[TessellatedPath]
	svgAnimContribs  scratchSlice[animContrib]

	// Layout sizing: reusable slices for distributeSpace's fill
	// candidate collection. Allocated once per fill-widths/fill-heights
	// pass and reused across all recursive nodes in the tree walk.
	// Sharing is safe because distributeSpace consumes the candidates
	// before it returns. No caller holds them across a nested call,
	// so each node starts from an empty slice.
	fillCandidates scratchSlice[int]
	fillBufs       fillBuffers // bundles the candidate slice for fill pipeline

	// View-phase pool: reuse Shape allocations across frames.
	// Reset before generateViewLayout; valid through buildRenderers.
	viewShapes   scratchObjPool[Shape]
	buttonColors scratchObjPool[shapeButtonColors]
	viewEvents   scratchObjPool[eventHandlers]
	viewEffects  scratchObjPool[shapeEffects]

	// Render-phase pools: reuse heap objects whose addresses are
	// stored in RenderCmd pointer fields (avoids per-frame escapes).
	renderTextStyles       scratchObjPool[TextStyle]
	renderGlyphLayouts     scratchObjPool[glyph.Layout]
	renderAffineTransforms scratchObjPool[glyph.AffineTransform]
	renderTextShimmers     scratchObjPool[textAnimShimmer]

	// Reusable events for layoutHover and gesture callbacks
	// (avoids per-shape/per-gesture heap allocation of Event).
	hoverEvent   Event
	gestureEvent Event

	// canvasCtx is the DrawContext every DrawCanvas redraw runs
	// through. It is parked here rather than built per redraw for two
	// reasons: it escapes to the heap the moment it is handed to an
	// OnDraw callback, and every tessellation buffer hanging off it
	// (arc points, bezier flattening, gradient subdivision) would
	// otherwise restart from nil and regrow by doubling on every frame
	// of an animated canvas.
	//
	// One shared context is correct because renderDrawCanvas is called
	// sequentially from the renderLayout walk and never nests — a
	// canvas cannot draw another canvas mid-draw. resetFor rebinds it
	// to each canvas in turn.
	canvasCtx DrawContext

	// orb holds the ThinkingOrb frame buffers. An orb builds its
	// frame inside its canvas OnDraw, which renderDrawCanvas calls
	// one canvas at a time, and the frame is drawn before OnDraw
	// returns, so one set serves every orb in the window.
	orb orbScratch

	floatingPoolUsed    int
	placeholderPoolUsed int

	// fillGen increments at the start of each fill pass
	// (layoutFillWidths + layoutFillHeights). Shapes cache
	// content and sibling-sum dimensions keyed to this
	// generation, avoiding per-frame cache-invalidation walks.
	fillGen uint32
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
		focusCandidates:        scratchSlice[focusCandidate]{retainMax: 4096, shrinkTo: 512},
		wrapRows:               scratchSlice[wrapRowRange]{retainMax: 4096, shrinkTo: 256},
		layerLayouts:           scratchSlice[Layout]{retainMax: 4096, shrinkTo: 256},
		focusSeen:              scratchMap[string, struct{}]{retainMax: 4096},
		svgAnimStates:          scratchMap[uint32, svgAnimState]{retainMax: 4096},
		svgAnimOverrides:       scratchMap[uint32, SvgAnimAttrOverride]{retainMax: 4096},
		svgAnimByPID:           scratchMap[uint32, []float32]{retainMax: 256},
		svgAnimTriangles:       scratchSlice[TessellatedPath]{retainMax: 1024, shrinkTo: 64},
		svgAnimContribs:        scratchSlice[animContrib]{retainMax: 1024, shrinkTo: 64},
		fillCandidates:         scratchSlice[int]{retainMax: 256, shrinkTo: 32},
		viewShapes:             scratchObjPool[Shape]{retainMax: 16384, shrinkTo: 1024},
		buttonColors:           scratchObjPool[shapeButtonColors]{retainMax: 512, shrinkTo: 32},
		viewEvents:             scratchObjPool[eventHandlers]{retainMax: 4096, shrinkTo: 256},
		viewEffects:            scratchObjPool[shapeEffects]{retainMax: 4096, shrinkTo: 256},
		renderTextStyles:       scratchObjPool[TextStyle]{retainMax: 4096, shrinkTo: 256},
		renderGlyphLayouts:     scratchObjPool[glyph.Layout]{retainMax: 1024, shrinkTo: 64},
		renderAffineTransforms: scratchObjPool[glyph.AffineTransform]{retainMax: 256, shrinkTo: 16},
		renderTextShimmers:     scratchObjPool[textAnimShimmer]{retainMax: 256, shrinkTo: 16},
	}
}

// beginFillPass increments the fill generation counter. Called
// before layoutFillWidths + layoutFillHeights so Shape caches from
// the previous frame are invalidated without a tree walk.
func (p *scratchPools) beginFillPass() {
	p.fillGen++
	if p.fillGen == 0 {
		p.fillGen = 1 // never wrap to 0; 0 is the cache-invalid sentinel
	}
}

// resetViewPools resets the view-phase object pools and truncates
// the view-phase arenas. Called before generateViewLayout. Each arena
// shrinks only when it has grown past its retain cap, so a one-off
// deep frame does not hold the capacity indefinitely.
func (p *scratchPools) resetViewPools() {
	p.viewShapes.reset()
	p.buttonColors.reset()
	p.viewEvents.reset()
	p.viewEffects.reset()
	if cap(p.layoutChildrenArena) > layoutChildrenRetainMax {
		p.layoutChildrenArena = make([]Layout, 0, layoutChildrenShrinkTo)
	} else {
		p.layoutChildrenArena = p.layoutChildrenArena[:0]
	}
	if cap(p.viewArena) > viewArenaRetainMax {
		p.viewArena = make([]View, 0, viewArenaShrinkTo)
	} else {
		p.viewArena = p.viewArena[:0]
	}
}

const (
	viewArenaRetainMax = 1 << 14 // 16 384 View interface values
	viewArenaShrinkTo  = 1 << 10 // 1 024

	// maxViewReservation bounds a single reservation, matching
	// maxLayoutChildrenReservation's reasoning: beyond it a standalone
	// slice keeps arena memory from being held across frames.
	maxViewReservation = 1 << 20
)

// takeViews reserves a pinned, zero-length subslice with capacity n
// from the frame-scoped view arena. The cap is pinned to n so the
// caller's appends cannot bleed into a later reservation, which is
// what makes nested lists safe. Callers must not retain the slice
// past the frame — appendChildViews copies out of it.
func (p *scratchPools) takeViews(n int) []View {
	return takeArena(&p.viewArena, n, maxViewReservation, false)
}

const (
	layoutChildrenRetainMax = 1 << 14 // 16 384 Layout values
	layoutChildrenShrinkTo  = 1 << 10 // 1 024 Layout values

	// maxLayoutChildrenReservation bounds a single reservation. Beyond
	// this a standalone slice is returned so arena memory is not held
	// across frames and no arithmetic overflow can occur.
	maxLayoutChildrenReservation = 1 << 20
)

// takeLayoutChildren reserves a pinned, zero-length subslice with
// capacity n from the frame-scoped layout-children arena. The cap is
// pinned to n so the caller's appends cannot bleed into a later
// reservation. Realloc of the underlying arena is safe: prior
// reservations stay valid because their slice headers keep the old
// backing array alive. Non-positive n returns nil; pathological sizes
// bypass the arena entirely.
func (p *scratchPools) takeLayoutChildren(n int) []Layout {
	return takeArena(&p.layoutChildrenArena, n, maxLayoutChildrenReservation, false)
}

// resetRenderPools resets the render-phase object pools and truncates
// the render-phase arena. Called at the start of each frame before
// building the render command list. svgVColArena shrinks only when it
// has grown past svgVColRetainMax, so a one-off spike frame does not
// hold hundreds of KB of vertex-color capacity indefinitely.
func (p *scratchPools) resetRenderPools() {
	p.renderTextStyles.reset()
	p.renderGlyphLayouts.reset()
	p.renderAffineTransforms.reset()
	p.renderTextShimmers.reset()
	if cap(p.svgVColArena) > svgVColRetainMax {
		p.svgVColArena = make([]Color, 0, svgVColShrinkTo)
	} else {
		p.svgVColArena = p.svgVColArena[:0]
	}
	// Same one-off-spike rule as svgVColArena. Items are large, so the
	// cap is in elements of a size that suits labels, not documents.
	if cap(p.textItemArena) > textItemRetainMax {
		p.textItemArena = nil
	} else {
		p.textItemArena = p.textItemArena[:0]
	}
	if cap(p.textGlyphArena) > textGlyphRetainMax {
		p.textGlyphArena = nil
	} else {
		p.textGlyphArena = p.textGlyphArena[:0]
	}
}

const (
	textItemRetainMax  = 256
	textGlyphRetainMax = 1 << 13
	// maxTextArenaReservation matches glyph's own scratch bound: a
	// layout from the public API holds at most that many glyphs, so a
	// larger request is a host-built layout and gets its own slice.
	maxTextArenaReservation = 1 << 14
)

// takeTextItems reserves n glyph.Items from the render-phase arena,
// full length. The caller copies a layout's items in and overwrites
// every slot.
func (p *scratchPools) takeTextItems(n int) []glyph.Item {
	return takeArena(&p.textItemArena, n, maxTextArenaReservation, true)
}

// takeTextGlyphs reserves n glyph.Glyphs from the render-phase arena,
// full length, under the same contract as takeTextItems.
func (p *scratchPools) takeTextGlyphs(n int) []glyph.Glyph {
	return takeArena(&p.textGlyphArena, n, maxTextArenaReservation, true)
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
// scoped vertex-color arena. Unlike takeLayoutChildren and takeViews,
// the returned slice has length n, not zero. Both callers overwrite
// every slot by index, so no stale color from a previous frame can
// leak through. A caller that only writes some slots must not use
// this function. The cap is pinned so appends cannot bleed into the
// next reservation. Realloc of the underlying arena is safe: prior
// reservations remain valid because their slice headers keep the old
// backing array alive. Non-positive n returns nil; pathological sizes
// bypass the arena entirely.
func (p *scratchPools) takeVColors(n int) []Color {
	return takeArena(&p.svgVColArena, n, maxVColReservation, true)
}

// arenaNeed adds a reservation to an arena length. It reports false
// when start+n overflows, so the caller can fall back to a standalone
// slice instead of growing the arena to a wrapped length. Pure
// function, so tests can pin the overflow boundary without any
// allocation.
func arenaNeed(start, n int) (int, bool) {
	need := start + n
	if need < start {
		return 0, false
	}
	return need, true
}

// takeArena reserves n elements from a frame-scoped arena. It holds
// the single copy of the growth logic that takeViews,
// takeLayoutChildren and takeVColors share.
//
// fullLen selects the returned length. False hands back len 0 and cap
// n for callers that append. True hands back len n and cap n for
// callers that overwrite every slot by index.
//
// A reservation past maxReservation bypasses the arena with a
// standalone slice, so one pathological request cannot pin arena
// memory across frames. The standalone size stays proportional to n,
// which always derives from a resident length. The pool never
// amplifies an allocation past memory the caller already holds.
func takeArena[T any](arena *[]T, n, maxReservation int, fullLen bool) []T {
	if n <= 0 {
		return nil
	}
	start := len(*arena)
	need, ok := arenaNeed(start, n)
	if n > maxReservation || !ok {
		if fullLen {
			return make([]T, n)
		}
		return make([]T, 0, n)
	}
	if cap(*arena) < need {
		grown := make([]T, need, growCap(cap(*arena), need))
		copy(grown, *arena)
		*arena = grown
	} else {
		*arena = (*arena)[:need]
	}
	if fullLen {
		return (*arena)[start:need:need]
	}
	return (*arena)[start:start:need]
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
	// Taking the slice also rewinds both object pools. Their cursors
	// are scoped to the same extraction pass as the slice, so one
	// take opens the whole floating group and one put closes it.
	p.floatingPoolUsed = 0
	p.placeholderPoolUsed = 0
	return s
}

func (p *scratchPools) putFloatingLayouts(s []*Layout) {
	if cap(s) > scratchFloatingLayoutsRetainMax {
		s = make([]*Layout, 0, scratchFloatingLayoutsShrinkTo)
	}
	p.floatingLayouts = s[:0]
	// The slice cap bounds retained backing memory, but the object
	// pools below hold pointers, so their length counts retained
	// allocations. Each uses its own bound for that reason.
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
		*reused = Shape{shapeType: shapeNone}
		return reused
	}
	allocated := &Shape{shapeType: shapeNone}
	p.placeholderShapePool = append(p.placeholderShapePool, allocated)
	return allocated
}
