package svg

import (
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

// --- Edge cases ---

func TestStrokeTessellateNilPolylines(t *testing.T) {
	result := tessellateStroke(nil, 2, gui.ButtCap, gui.MiterJoin)
	if len(result) != 0 {
		t.Fatalf("nil polylines should return empty, got len=%d", len(result))
	}
}

func TestStrokeTessellateEmptyPolylines(t *testing.T) {
	result := tessellateStroke([][]float32{}, 2, gui.ButtCap, gui.MiterJoin)
	if len(result) != 0 {
		t.Fatalf("empty polylines should return empty, got len=%d", len(result))
	}
}

func TestStrokeTessellateShortPolyline(t *testing.T) {
	// Less than 4 floats → skipped
	result := tessellateStroke([][]float32{{1, 2}}, 2, gui.ButtCap, gui.MiterJoin)
	if len(result) != 0 {
		t.Fatalf("short polyline should return empty, got len=%d", len(result))
	}
}

// --- Basic stroke generation ---

func TestStrokeHorizontalLine(t *testing.T) {
	// Horizontal line from (0,0) to (10,0), width 2 → halfW=1
	poly := [][]float32{{0, 0, 10, 0}}
	result := tessellateStroke(poly, 2, gui.ButtCap, gui.MiterJoin)
	// 1 quad = 2 triangles = 12 floats
	if len(result) < 12 {
		t.Fatalf("expected at least 12 floats, got %d", len(result))
	}
	// Check Y values span ±1
	hasNeg := false
	hasPos := false
	for i := 1; i < len(result); i += 2 {
		if result[i] < -0.5 {
			hasNeg = true
		}
		if result[i] > 0.5 {
			hasPos = true
		}
	}
	if !hasNeg || !hasPos {
		t.Fatalf("Y should span ±halfW, got %v", result[:12])
	}
}

func TestStrokeVerticalLine(t *testing.T) {
	poly := [][]float32{{0, 0, 0, 10}}
	result := tessellateStroke(poly, 2, gui.ButtCap, gui.MiterJoin)
	if len(result) < 12 {
		t.Fatalf("expected at least 12 floats, got %d", len(result))
	}
	// Check X values span ±1
	hasNeg := false
	hasPos := false
	for i := 0; i < len(result); i += 2 {
		if result[i] < -0.5 {
			hasNeg = true
		}
		if result[i] > 0.5 {
			hasPos = true
		}
	}
	if !hasNeg || !hasPos {
		t.Fatalf("X should span ±halfW")
	}
}

func TestStrokeLShapeMultiSegment(t *testing.T) {
	poly := [][]float32{{0, 0, 10, 0, 10, 10}}
	result := tessellateStroke(poly, 2, gui.ButtCap, gui.MiterJoin)
	// 2 segments = 24 floats for quads, plus join geometry
	if len(result) <= 12 {
		t.Fatalf("L-shape should produce more than 12 floats, got %d", len(result))
	}
}

// --- Line caps ---

func TestStrokeButtCap(t *testing.T) {
	poly := [][]float32{{0, 0, 10, 0}}
	butt := tessellateStroke(poly, 2, gui.ButtCap, gui.MiterJoin)
	// ButtCap adds nothing
	if len(butt) != 12 {
		t.Fatalf("ButtCap should have exactly 12 floats (1 quad), got %d", len(butt))
	}
}

func TestStrokeSquareCap(t *testing.T) {
	poly := [][]float32{{0, 0, 10, 0}}
	sq := tessellateStroke(poly, 2, gui.SquareCap, gui.MiterJoin)
	butt := tessellateStroke(poly, 2, gui.ButtCap, gui.MiterJoin)
	if len(sq) <= len(butt) {
		t.Fatalf("SquareCap should add more floats than ButtCap: sq=%d butt=%d",
			len(sq), len(butt))
	}
}

func TestStrokeRoundCap(t *testing.T) {
	poly := [][]float32{{0, 0, 10, 0}}
	rnd := tessellateStroke(poly, 2, gui.RoundCap, gui.MiterJoin)
	sq := tessellateStroke(poly, 2, gui.SquareCap, gui.MiterJoin)
	if len(rnd) <= len(sq) {
		t.Fatalf("RoundCap should produce more floats than SquareCap: rnd=%d sq=%d",
			len(rnd), len(sq))
	}
}

// --- Line joins ---

func TestStrokeMiterJoin(t *testing.T) {
	poly := [][]float32{{0, 0, 10, 0, 10, 10}}
	miter := tessellateStroke(poly, 2, gui.ButtCap, gui.MiterJoin)
	if len(miter) <= 24 {
		t.Fatalf("MiterJoin L-shape should have join geometry, got %d", len(miter))
	}
}

func TestStrokeBevelJoin(t *testing.T) {
	poly := [][]float32{{0, 0, 10, 0, 10, 10}}
	bevel := tessellateStroke(poly, 2, gui.ButtCap, gui.BevelJoin)
	if len(bevel) <= 24 {
		t.Fatalf("BevelJoin L-shape should have join geometry, got %d", len(bevel))
	}
}

func TestStrokeRoundJoin(t *testing.T) {
	poly := [][]float32{{0, 0, 10, 0, 10, 10}}
	rnd := tessellateStroke(poly, 2, gui.ButtCap, gui.RoundJoin)
	bevel := tessellateStroke(poly, 2, gui.ButtCap, gui.BevelJoin)
	if len(rnd) <= len(bevel) {
		t.Fatalf("RoundJoin should produce more floats than BevelJoin: rnd=%d bevel=%d",
			len(rnd), len(bevel))
	}
}

// --- Closed path ---

func TestStrokeClosedSquare(t *testing.T) {
	poly := [][]float32{{0, 0, 10, 0, 10, 10, 0, 10, 0, 0}}
	result := tessellateStroke(poly, 2, gui.ButtCap, gui.MiterJoin)
	if len(result) == 0 {
		t.Fatalf("closed square should produce output")
	}
	// Closed path: no caps, only joins
	butt := tessellateStroke(poly, 2, gui.ButtCap, gui.MiterJoin)
	sq := tessellateStroke(poly, 2, gui.SquareCap, gui.MiterJoin)
	if len(butt) != len(sq) {
		t.Fatalf("closed path: SquareCap should equal ButtCap: butt=%d sq=%d",
			len(butt), len(sq))
	}
}

// Closed polylines emit an extra segment wrapping the last point
// back to the first so the stroke does not leave a visible gap.
// Compare a closed square (last point duplicates first) to the
// same square without the closing point: closed must emit exactly
// one more segment of quad geometry.
func TestTessellateStroke_ClosedEmitsExtraSegment(t *testing.T) {
	closed := [][]float32{{0, 0, 10, 0, 10, 10, 0, 10, 0, 0}}
	open := [][]float32{{0, 0, 10, 0, 10, 10, 0, 10}}
	closedTris := tessellateStroke(closed, 2, gui.ButtCap, gui.BevelJoin)
	openTris := tessellateStroke(open, 2, gui.ButtCap, gui.BevelJoin)
	if len(closedTris) <= len(openTris) {
		t.Fatalf("closed should emit more geometry: closed=%d open=%d",
			len(closedTris), len(openTris))
	}
	// Each segment is 2 triangles × 3 verts × 2 floats = 12 floats,
	// plus a bevel-join triangle (6 floats) per interior corner.
	// We only assert the net increase is at least one segment's
	// worth of floats (12) to avoid coupling to join minutiae.
	if len(closedTris)-len(openTris) < 12 {
		t.Fatalf("expected at least one extra segment (12 floats); "+
			"closed=%d open=%d", len(closedTris), len(openTris))
	}
}
