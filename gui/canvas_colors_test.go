package gui

import "testing"

// vcolTris is a two-triangle quad, the smallest mesh that has an
// interior edge.
func vcolTris() []float32 {
	return []float32{
		0, 0, 10, 0, 10, 10,
		0, 0, 10, 10, 0, 10,
	}
}

// vcolColors is one color per vertex of vcolTris.
func vcolColors() []Color {
	return []Color{
		RGB(255, 0, 0), RGB(0, 255, 0), RGB(0, 0, 255),
		RGB(255, 0, 0), RGB(0, 0, 255), RGB(255, 255, 255),
	}
}

func TestFillTrianglesColorsProducesVertexColors(t *testing.T) {
	dc := NewDrawContext(100, 100, nil)
	tris, cols := vcolTris(), vcolColors()
	dc.FillTrianglesColors(tris, cols)

	if len(dc.batches) != 1 {
		t.Fatalf("got %d batches, want 1", len(dc.batches))
	}
	b := dc.batches[0]
	if len(b.VertexColors)*2 != len(b.Triangles) {
		t.Fatalf("VertexColors=%d, Triangles=%d: lengths must agree",
			len(b.VertexColors), len(b.Triangles))
	}
	// Nothing is evaluated: the caller's geometry and colors go through
	// verbatim, in order.
	for i := range tris {
		if b.Triangles[i] != tris[i] {
			t.Fatalf("triangle float %d = %v, want %v",
				i, b.Triangles[i], tris[i])
		}
	}
	for i := range cols {
		if b.VertexColors[i] != cols[i] {
			t.Fatalf("vertex color %d = %v, want %v",
				i, b.VertexColors[i], cols[i])
		}
	}
	if b.Color != meanColor(cols) {
		t.Errorf("batch color = %v, want the mesh mean %v",
			b.Color, meanColor(cols))
	}
}

func TestFillTrianglesColorsNoops(t *testing.T) {
	cases := []struct {
		name   string
		tris   []float32
		colors []Color
	}{
		{"empty tris", nil, nil},
		{"partial triangle", []float32{0, 0, 1, 0}, make([]Color, 2)},
		{"too few colors", vcolTris(), make([]Color, 5)},
		{"too many colors", vcolTris(), make([]Color, 7)},
		{"no colors", vcolTris(), nil},
	}
	for _, c := range cases {
		dc := NewDrawContext(100, 100, nil)
		dc.FillTrianglesColors(c.tris, c.colors)
		if len(dc.batches) != 0 {
			t.Errorf("%s: got %d batches, want 0", c.name, len(dc.batches))
		}
	}
}

// TestFlatFillAfterVertexColorsStartsNewBatch is the same invariant the
// gradient path rests on: a batch is flat or vertex-colored, never
// both, so the per-batch length relation holds and validSvgCmd can
// enforce it downstream.
func TestFlatFillAfterVertexColorsStartsNewBatch(t *testing.T) {
	dc := NewDrawContext(100, 100, nil)
	dc.FillTrianglesColors(vcolTris(), vcolColors())
	// Draw flat in exactly the color the mesh batch recorded, which is
	// what a naive lastColor comparison would merge on.
	dc.FilledRect(0, 0, 10, 10, dc.batches[0].Color)
	if len(dc.batches) != 2 {
		t.Fatalf("got %d batches, want 2", len(dc.batches))
	}
	b := dc.batches[0]
	if len(b.VertexColors)*2 != len(b.Triangles) {
		t.Errorf("mesh batch corrupted: VertexColors=%d, Triangles=%d",
			len(b.VertexColors), len(b.Triangles))
	}
	if dc.batches[1].VertexColors != nil {
		t.Error("flat batch carries vertex colors")
	}
}

// TestVertexColorFillSurvivesValidation walks the emitted batch through
// the same validator the render path applies.
func TestVertexColorFillSurvivesValidation(t *testing.T) {
	dc := NewDrawContext(100, 100, nil)
	dc.FillTrianglesColors(vcolTris(), vcolColors())
	b := dc.batches[0]
	cmd := RenderCmd{
		Kind:         RenderSvg,
		Scale:        1,
		Color:        b.Color,
		Triangles:    b.Triangles,
		VertexColors: b.VertexColors,
	}
	if !validSvgCmd(cmd) {
		t.Error("a vertex-colored canvas batch failed validSvgCmd")
	}
}

// vertexColorRecorderStub implements the optional extension.
type vertexColorRecorderStub struct {
	nopRecorder
	calls  int
	tris   int
	colors int
}

func (r *vertexColorRecorderStub) FillTrianglesColors(
	tris []float32, colors []Color) {
	r.calls++
	r.tris = len(tris)
	r.colors = len(colors)
}

func TestVertexColorRecorderExtensionReceivesGeometry(t *testing.T) {
	dc := NewDrawContext(100, 100, nil)
	rec := &vertexColorRecorderStub{}
	dc.SetRecorder(rec)
	dc.FillTrianglesColors(vcolTris(), vcolColors())

	if rec.calls != 1 {
		t.Fatalf("FillTrianglesColors called %d times, want 1", rec.calls)
	}
	if rec.tris != 12 || rec.colors != 6 {
		t.Errorf("recorder got %d floats and %d colors, want 12 and 6",
			rec.tris, rec.colors)
	}
	if len(dc.batches) != 0 {
		t.Error("a recorded fill must not also tessellate into a batch")
	}
}

// TestVertexColorFlatRecorderFallback pins the degrade path: a recorder
// that cannot take per-vertex color still gets every triangle, each at
// its own mean rather than at one color for the whole mesh.
func TestVertexColorFlatRecorderFallback(t *testing.T) {
	dc := NewDrawContext(100, 100, nil)
	rec := &meanPolyRecorder{}
	dc.SetRecorder(rec)
	cols := vcolColors()
	dc.FillTrianglesColors(vcolTris(), cols)

	if len(rec.colors) != 2 {
		t.Fatalf("recorded %d polygons, want 2 (one per triangle)",
			len(rec.colors))
	}
	for i := range rec.colors {
		want := meanColor(cols[i*3 : i*3+3])
		if rec.colors[i] != want {
			t.Errorf("triangle %d recorded %v, want its own mean %v",
				i, rec.colors[i], want)
		}
	}
	if rec.colors[0] == rec.colors[1] {
		t.Error("both triangles recorded the same color; the fallback " +
			"is per-triangle, not per-mesh")
	}
	if len(dc.batches) != 0 {
		t.Error("a recorded fill must not also tessellate into a batch")
	}
}

// meanPolyRecorder keeps every polygon color it is handed, so the
// per-triangle fallback can be checked entry by entry.
type meanPolyRecorder struct {
	nopRecorder
	colors []Color
}

func (r *meanPolyRecorder) FilledPolygon(_ []float32, c Color) {
	r.colors = append(r.colors, c)
}

func TestMeanColor(t *testing.T) {
	if got := meanColor(nil); got != (Color{}) {
		t.Errorf("meanColor(nil) = %v, want the unset color", got)
	}
	got := meanColor([]Color{
		RGBA(0, 0, 0, 0), RGBA(100, 200, 40, 200),
	})
	want := RGBA(50, 100, 20, 100)
	if got != want {
		t.Errorf("meanColor = %v, want %v", got, want)
	}
}
