package gui

import "testing"

// batchLensAgree fails the test when a vertex-colored batch carries a
// color count that does not match its vertex count. validSvgCmd drops
// such a batch, so a mismatch is a whole batch missing from the frame.
func batchLensAgree(t *testing.T, batches []DrawCanvasTriBatch) {
	t.Helper()
	for i := range batches {
		b := &batches[i]
		if b.VertexColors != nil && len(b.VertexColors)*2 != len(b.Triangles) {
			t.Fatalf("batch %d: %d colors for %d vertices", i,
				len(b.VertexColors), len(b.Triangles)/2)
		}
	}
}

// colorRun is n vertices of color c.
type colorRun struct {
	c Color
	n int
}

// wantVertexColors fails the test unless b carries exactly the colors
// in runs, each repeated for its count of vertices, in order.
func wantVertexColors(t *testing.T, b *DrawCanvasTriBatch, runs ...colorRun) {
	t.Helper()
	i := 0
	for _, r := range runs {
		for k := 0; k < r.n; k++ {
			if i >= len(b.VertexColors) {
				t.Fatalf("only %d vertex colors", len(b.VertexColors))
			}
			if b.VertexColors[i] != r.c {
				t.Fatalf("vertex %d color = %v, want %v", i,
					b.VertexColors[i], r.c)
			}
			i++
		}
	}
	if i != len(b.VertexColors) {
		t.Fatalf("%d vertex colors, want %d", len(b.VertexColors), i)
	}
}

func TestDrawContextMixColorsMergesColors(t *testing.T) {
	red, blue, green := RGB(255, 0, 0), RGB(0, 0, 255), RGB(0, 255, 0)
	dc := NewDrawContext(100, 100, nil)
	dc.mixColors = true
	dc.FilledRect(0, 0, 10, 10, red)
	dc.FilledRect(20, 0, 10, 10, blue)
	dc.Line(0, 50, 50, 50, green, 2)
	// Same color again: still the same batch, still its own run.
	dc.FilledRect(40, 0, 10, 10, green)

	bs := dc.Batches()
	if len(bs) != 1 {
		t.Fatalf("batches = %d, want 1", len(bs))
	}
	batchLensAgree(t, bs)
	wantVertexColors(t, &bs[0],
		colorRun{red, 6}, colorRun{blue, 6}, colorRun{green, 12})
}

func TestDrawContextMixColorsOffByDefault(t *testing.T) {
	dc := NewDrawContext(100, 100, nil)
	dc.FilledRect(0, 0, 10, 10, RGB(255, 0, 0))
	dc.FilledRect(20, 0, 10, 10, RGB(0, 0, 255))

	bs := dc.Batches()
	if len(bs) != 2 {
		t.Fatalf("batches = %d, want 2", len(bs))
	}
	for i := range bs {
		if bs[i].VertexColors != nil {
			t.Errorf("batch %d carries vertex colors on the flat path", i)
		}
	}
}

// A batch holds one transform, so a transform change still splits the
// merged batch, and the tail of the first is settled before the split.
func TestDrawContextMixColorsSplitsOnTransform(t *testing.T) {
	red, blue := RGB(255, 0, 0), RGB(0, 0, 255)
	dc := NewDrawContext(100, 100, nil)
	dc.mixColors = true
	dc.FilledRect(0, 0, 10, 10, red)
	dc.Translate(5, 5)
	dc.FilledRect(0, 0, 10, 10, blue)

	bs := dc.Batches()
	if len(bs) != 2 {
		t.Fatalf("batches = %d, want 2", len(bs))
	}
	batchLensAgree(t, bs)
	wantVertexColors(t, &bs[0], colorRun{red, 6})
	wantVertexColors(t, &bs[1], colorRun{blue, 6})
	if !bs[1].hasXform {
		t.Error("second batch lost its transform")
	}
}

// A per-vertex fill drawn between two flat marks opens its own batch.
// The merged batch before it must be settled first, or its last mark
// is left with no colors.
func TestDrawContextMixColorsAroundVertexFill(t *testing.T) {
	red, blue, white := RGB(255, 0, 0), RGB(0, 0, 255), RGB(255, 255, 255)
	dc := NewDrawContext(100, 100, nil)
	dc.mixColors = true
	dc.FilledRect(0, 0, 10, 10, red)
	dc.FillTrianglesColors([]float32{0, 0, 10, 0, 0, 10},
		[]Color{white, white, white})
	dc.FilledRect(20, 0, 10, 10, blue)

	bs := dc.Batches()
	if len(bs) != 3 {
		t.Fatalf("batches = %d, want 3", len(bs))
	}
	batchLensAgree(t, bs)
	wantVertexColors(t, &bs[0], colorRun{red, 6})
	wantVertexColors(t, &bs[1], colorRun{white, 3})
	wantVertexColors(t, &bs[2], colorRun{blue, 6})
}

// The mode belongs to one redraw. resetFor runs before every OnDraw on
// a context shared by every canvas in the window, so a mode left on
// would merge the next canvas's batches too.
func TestDrawContextMixColorsClearedByReset(t *testing.T) {
	dc := NewDrawContext(100, 100, nil)
	dc.mixColors = true
	dc.resetFor(100, 100, 1, nil, drawCanvasCache{})
	if dc.mixColors {
		t.Fatal("resetFor left mixColors on")
	}
}
