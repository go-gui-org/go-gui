package gui

import (
	"math"
	"testing"

	"github.com/go-gui-org/go-glyph"
)

// canvas_draw_review_test.go — regressions for the canvas review pass.
// Each test fails against the code as it stood before its fix.

// --- Save/Restore balance past maxXformDepth ---

// TestXformSaveOverflowKeepsBalance drives the Save stack past its cap
// and then unwinds it. Before the fix a push past maxXformDepth was
// dropped silently, so the matching Restore popped an ancestor and
// every level after the cap drew one step too far back.
func TestXformSaveOverflowKeepsBalance(t *testing.T) {
	const over = 4
	dc := NewDrawContext(100, 100, nil)
	for range maxXformDepth + over {
		dc.Save()
		dc.Translate(1, 0)
	}
	if got := dc.xf.tx; got != maxXformDepth+over {
		t.Fatalf("tx after nesting = %v, want %v", got, maxXformDepth+over)
	}
	// The first `over` Restores unwind the pushes the cap dropped, so
	// they must not move the transform at all.
	for i := range over {
		dc.Restore()
		if got := dc.xf.tx; got != maxXformDepth+over {
			t.Fatalf("Restore %d of the dropped pushes moved tx to %v, "+
				"want %v held", i+1, got, maxXformDepth+over)
		}
	}
	// From here each Restore unwinds one real level.
	dc.Restore()
	if got := dc.xf.tx; got != maxXformDepth-1 {
		t.Fatalf("first real Restore gave tx %v, want %v",
			got, maxXformDepth-1)
	}
	// Unwinding the rest returns to the identity, not past it.
	for range maxXformDepth {
		dc.Restore()
	}
	if dc.xf != identityXform {
		t.Fatalf("full unwind left xf %+v, want identity %+v",
			dc.xf, identityXform)
	}
}

// TestXformResetClearsDroppedSaves checks an unbalanced overflow cannot
// survive into the next redraw: the dropped-push counter resets with
// the rest of the transform state.
func TestXformResetClearsDroppedSaves(t *testing.T) {
	dc := NewDrawContext(100, 100, nil)
	for range maxXformDepth + 8 {
		dc.Save()
	}
	if dc.xfDropped == 0 {
		t.Fatal("expected dropped pushes to be counted")
	}
	dc.resetXform()
	if dc.xfDropped != 0 {
		t.Fatalf("xfDropped = %d after reset, want 0", dc.xfDropped)
	}
	// A Restore now must find an empty stack, not a stale credit.
	dc.Translate(5, 5)
	dc.Restore()
	if dc.xf.tx != 5 {
		t.Fatalf("tx = %v after reset+Translate+Restore, want 5", dc.xf.tx)
	}
}

// --- transform overflow to Inf ---

// TestScaleByRejectsOverflow checks a finite argument whose product
// with the current scale overflows is ignored. Before the fix sx went
// to +Inf, and scaleTextStyle baked that into a text entry's Size,
// which the text emit path measures through the glyph shaper before
// any render-command validation runs.
func TestScaleByRejectsOverflow(t *testing.T) {
	dc := NewDrawContext(100, 100, nil)
	dc.ScaleBy(1e38, 1e38)
	before := dc.xf
	dc.ScaleBy(1e38, 1e38) // product overflows
	if dc.xf != before {
		t.Fatalf("overflowing ScaleBy changed xf to %+v, want %+v held",
			dc.xf, before)
	}
	// A single finite ScaleBy is enough to overflow the bake itself:
	// 12 * 1e38 is +Inf with the transform still perfectly finite. The
	// entry must be dropped rather than recorded with infinities the
	// emit path would hand to the glyph shaper.
	dc.Text(10, 10, "hi", TextStyle{Size: 12})
	for i, e := range dc.Texts() {
		if !f32IsFinite(e.X) || !f32IsFinite(e.Y) ||
			!f32IsFinite(e.Style.Size) {
			t.Fatalf("text entry %d carries non-finite values: "+
				"X=%v Y=%v Size=%v", i, e.X, e.Y, e.Style.Size)
		}
	}
	// A transform that does NOT overflow still records normally.
	dc2 := NewDrawContext(100, 100, nil)
	dc2.ScaleBy(2, 2)
	dc2.Text(10, 10, "hi", TextStyle{Size: 12})
	if got := dc2.Texts(); len(got) != 1 || got[0].X != 20 ||
		got[0].Style.Size != 24 {
		t.Fatalf("ordinary scaled text = %+v, want X=20 Size=24", got)
	}
}

// TestTranslateRejectsOverflow is ScaleBy's sibling: the same overflow
// reached by accumulating translations instead of scales.
func TestTranslateRejectsOverflow(t *testing.T) {
	dc := NewDrawContext(100, 100, nil)
	dc.Translate(3e38, 3e38)
	before := dc.xf
	dc.Translate(3e38, 3e38) // sum overflows
	if dc.xf != before {
		t.Fatalf("overflowing Translate changed xf to %+v, want %+v held",
			dc.xf, before)
	}
	if !f32IsFinite(dc.xf.tx) || !f32IsFinite(dc.xf.ty) {
		t.Fatalf("translation went non-finite: %+v", dc.xf)
	}
}

// --- recorder point-slice retention ---

// holdRecorder keeps every points slice it is handed, which is exactly
// what DrawRecorder now documents as requiring a copy.
type holdRecorder struct {
	nopRecorder
	runs [][]float32
}

func (h *holdRecorder) Polyline(points []float32, _ Color, _ float32) {
	h.runs = append(h.runs, points)
}

// copyRecorder is the compliant shape: it copies on the way in.
type copyRecorder struct {
	nopRecorder
	runs [][]float32
}

func (c *copyRecorder) Polyline(points []float32, _ Color, _ float32) {
	c.runs = append(c.runs, append([]float32(nil), points...))
}

// TestRecorderPointsValidForCallOnly pins the documented contract from
// both sides: a recorder that retains the slice sees it overwritten
// under an active transform, and one that copies does not. It is the
// asymmetry that matters — with no transform the caller's own slice is
// passed through and retention would appear to work, which is how an
// exporter ends up correct until its first Translate.
func TestRecorderPointsValidForCallOnly(t *testing.T) {
	mk := func(r DrawRecorder) *DrawContext {
		dc := NewDrawContext(100, 100, nil)
		dc.SetRecorder(r)
		dc.Translate(10, 10)
		dc.Polyline([]float32{0, 0, 1, 1}, Blue, 1)
		dc.Polyline([]float32{5, 5, 6, 6}, Red, 1)
		return dc
	}

	held := &holdRecorder{}
	mk(held)
	if len(held.runs) != 2 {
		t.Fatalf("recorded %d runs, want 2", len(held.runs))
	}
	// Documented consequence, asserted so the contract has a witness:
	// the retained slices are the same buffer.
	if &held.runs[0][0] != &held.runs[1][0] {
		t.Error("retained slices no longer alias; the DrawRecorder " +
			"comment about copying needs revisiting")
	}

	copied := &copyRecorder{}
	mk(copied)
	want := [][]float32{{10, 10, 11, 11}, {15, 15, 16, 16}}
	for i, w := range want {
		for j, v := range w {
			if copied.runs[i][j] != v {
				t.Errorf("run %d = %v, want %v", i, copied.runs[i], w)
				break
			}
		}
	}
}

// --- cache entry tail retention ---

// TestResetForClearsEntryTails checks the recycled Texts/Images arrays
// do not pin the previous redraw's strings and fetchers past the new
// length. Before the fix the arrays were truncated with [:0], leaving
// every entry beyond the new length intact in the backing array for
// the life of the canvas.
func TestResetForClearsEntryTails(t *testing.T) {
	dc := NewDrawContext(100, 100, nil)
	for i := range 8 {
		dc.Text(float32(i), 0, "a label that should not be pinned", TextStyle{Size: 10})
		dc.Image(0, 0, 10, 10, "https://example.invalid/tile.png",
			Opt[float32]{}, Red)
	}
	prev := drawCanvasCache{Texts: dc.texts, Images: dc.images}
	texts, images := prev.Texts[:cap(prev.Texts)], prev.Images[:cap(prev.Images)]

	// One short redraw reusing those arrays.
	dc.resetFor(100, 100, 1, nil, prev)
	dc.Text(0, 0, "short", TextStyle{Size: 10})

	for i := 1; i < len(texts); i++ {
		if texts[i].Text != "" {
			t.Fatalf("text entry %d still pins %q", i, texts[i].Text)
		}
	}
	for i := range images {
		if images[i].Src != "" {
			t.Fatalf("image entry %d still pins %q", i, images[i].Src)
		}
	}
}

// --- cache entry liveness within one render pass ---

// TestDrawCanvasCacheHitClaimsPass covers the duplicate-effective-ID
// case the entry's pass field exists for, reached through a cache HIT
// rather than a redraw.
//
// Two shapes share one key and differ only in Version, so the first
// hits the cache and the second redraws. Before the fix the hit left
// the entry's pass at its previous value, so the redraw concluded the
// buffers belonged to an earlier pass and wrote over the triangles the
// first shape's already-emitted command points at.
func TestDrawCanvasCacheHitClaimsPass(t *testing.T) {
	w := makeWindowWithScratch()
	clip := makeClip(0, 0, 200, 200)

	mkShape := func(version uint64, tri float32) *Shape {
		return &Shape{
			shapeType: shapeDrawCanvas,
			ID:        "dup",
			Width:     100, Height: 100,
			Version: version,
			Opacity: 1,
			Color:   RGB(100, 100, 100),
			events: &eventHandlers{
				OnDraw: func(dc *DrawContext) {
					dc.FilledRect(tri, tri, 10, 10, Red)
				},
			},
		}
	}
	first := mkShape(1, 1)
	second := mkShape(2, 99)

	// Pass one: populate the cache for key "dup".
	w.renderPass = 1
	renderDrawCanvas(first, clip, w)

	// Pass two: the same shape hits the cache, then a second shape with
	// the same key but a new Version redraws.
	w.renderPass = 2
	w.renderers = w.renderers[:0]
	renderDrawCanvas(first, clip, w)

	var hit []float32
	for i := range w.renderers {
		if w.renderers[i].Kind == RenderSvg {
			hit = w.renderers[i].Triangles
			break
		}
	}
	if hit == nil {
		t.Fatal("cache hit emitted no RenderSvg command")
	}
	if hit[0] != 1 {
		t.Fatalf("cache hit emitted triangles starting %v, want 1", hit[0])
	}

	renderDrawCanvas(second, clip, w)

	// The redraw must not have written through the slice the hit's
	// command still references.
	if hit[0] != 1 {
		t.Errorf("redraw clobbered the cache hit's emitted geometry: "+
			"first vertex is now %v, want 1 (the second shape's is 99)",
			hit[0])
	}
}

// --- text record-time screening beyond the bake ---

// TestTextRejectsNonFiniteUntransformed checks the record-time screen
// covers raw inputs too, not just baked ones: validTextCmd never looks
// at Size, and the emit path measures through the glyph shaper before
// that check runs, so an untransformed Text with an infinite size would
// reach the shaper's cache and rasterizer exactly as the baked one did.
func TestTextRejectsNonFiniteUntransformed(t *testing.T) {
	dc := NewDrawContext(100, 100, nil)
	dc.Text(10, 10, "hi", TextStyle{Size: 12})
	dc.Text(0, 0, "bad size", TextStyle{Size: float32(math.Inf(1))})
	dc.Text(float32(math.NaN()), 0, "bad x", TextStyle{Size: 12})
	dc.Text(0, 0, "bad cell",
		TextStyle{Size: 12, CellWidth: float32(math.Inf(1))})
	if got := len(dc.Texts()); got != 1 {
		t.Fatalf("recorded %d text entries, want 1 (only the finite one)",
			got)
	}
	if got := dc.Texts()[0].Text; got != "hi" {
		t.Fatalf("kept entry is %q, want %q", got, "hi")
	}
}

// TestTextRejectsCellWidthOverflow checks the bake screen covers every
// px-valued field scaleTextStyle touches: with a huge-but-finite scale
// and a zero Size, the previously checked fields stay finite while
// CellWidth overflows, and the entry must still be dropped rather than
// handed to the shaper through glyphconv.
func TestTextRejectsCellWidthOverflow(t *testing.T) {
	dc := NewDrawContext(100, 100, nil)
	dc.ScaleBy(1e38, 1e38)
	dc.Text(0, 0, "grid", TextStyle{CellWidth: 10})
	if got := len(dc.Texts()); got != 0 {
		t.Fatalf("recorded %d text entries, want 0 "+
			"(CellWidth baked to +Inf)", got)
	}
}

// finiteSizeMeasurer is a TextMeasurer spy: it records every Size it is
// asked to measure and fails the test on a non-finite one.
type finiteSizeMeasurer struct {
	t     *testing.T
	sizes []float32
}

func (m *finiteSizeMeasurer) note(s TextStyle) {
	m.sizes = append(m.sizes, s.Size)
	if !f32AllFinite7(s.Size, s.LineSpacing, s.StrokeWidth, s.LetterSpacing,
		s.CellWidth, s.CellHeight, s.EmojiBoxWidth) {
		m.t.Errorf("shaper asked to measure non-finite font size %+v",
			s)
	}
}

func (m *finiteSizeMeasurer) TextWidth(_ string, s TextStyle) float32 {
	m.note(s)
	return 10
}

func (m *finiteSizeMeasurer) TextHeight(_ string, s TextStyle) float32 {
	m.note(s)
	return 10
}

func (m *finiteSizeMeasurer) FontHeight(s TextStyle) float32 {
	m.note(s)
	return 10
}

func (m *finiteSizeMeasurer) FontAscent(s TextStyle) float32 {
	m.note(s)
	return 10
}

func (m *finiteSizeMeasurer) LayoutText(_ string, s TextStyle,
	_ float32) (glyph.Layout, error) {
	m.note(s)
	return glyph.Layout{}, nil
}

// TestCanvasOverflowTextNeverReachesShaper pins the end-to-end property
// the record-time screen exists for: the text emit path measures through
// the shaper before validation, so a dropped overflow entry must never
// arrive as a measurement. The canvas draws one finite control and one
// entry whose bake overflows; the spy must see exactly the finite Size.
func TestCanvasOverflowTextNeverReachesShaper(t *testing.T) {
	w := makeWindowWithScratch()
	spy := &finiteSizeMeasurer{t: t}
	w.textMeasurer = spy
	w.renderPass = 3
	shape := &Shape{
		shapeType: shapeDrawCanvas,
		ID:        "shaperspy",
		Width:     100, Height: 100,
		Opacity: 1,
		Color:   RGB(100, 100, 100),
		events: &eventHandlers{
			OnDraw: func(dc *DrawContext) {
				dc.Text(10, 10, "ok", TextStyle{Size: 12})
				dc.Save()
				dc.ScaleBy(1e38, 1e38)
				dc.Text(10, 10, "boom", TextStyle{Size: 12})
				// Zero Size keeps the old four-field screen quiet
				// while CellWidth still bakes to +Inf.
				dc.Text(0, 0, "grid", TextStyle{CellWidth: 10})
				dc.Restore()
			},
		},
	}
	renderDrawCanvas(shape, makeClip(0, 0, 200, 200), w)
	if len(spy.sizes) == 0 {
		t.Fatal("shaper measured nothing; the finite control text " +
			"should have been measured")
	}
	for _, size := range spy.sizes {
		if size != 12 {
			t.Errorf("shaper measured size %v, want only the finite "+
				"control's 12", size)
		}
	}
}
