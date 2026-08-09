package gui

import (
	"math"
	"testing"
)

func TestFlattenDefsPathEmpty(t *testing.T) {
	result := flattenDefsPath("", 1.0)
	if len(result) != 0 {
		t.Fatal("expected empty for empty path")
	}
}

func TestFlattenDefsPathMoveTo(t *testing.T) {
	result := flattenDefsPath("M 10 20", 1.0)
	if len(result) != 2 {
		t.Fatalf("expected 2 coords, got %d", len(result))
	}
	if result[0] != 10 || result[1] != 20 {
		t.Fatalf("expected (10,20), got (%f,%f)",
			result[0], result[1])
	}
}

func TestFlattenDefsPathLineTo(t *testing.T) {
	result := flattenDefsPath("M 0 0 L 100 0 L 100 100", 1.0)
	if len(result) != 6 {
		t.Fatalf("expected 6 coords, got %d", len(result))
	}
	if result[2] != 100 || result[3] != 0 {
		t.Fatalf("expected L(100,0), got (%f,%f)",
			result[2], result[3])
	}
}

func TestFlattenDefsPathRelativeLineTo(t *testing.T) {
	result := flattenDefsPath("M 10 10 l 5 0 l 0 5", 1.0)
	// M(10,10), l(15,10), l(15,15)
	if len(result) != 6 {
		t.Fatalf("expected 6 coords, got %d", len(result))
	}
	if result[2] != 15 || result[3] != 10 {
		t.Fatalf("expected (15,10), got (%f,%f)",
			result[2], result[3])
	}
}

func TestFlattenDefsPathScale(t *testing.T) {
	result := flattenDefsPath("M 10 20 L 30 40", 2.0)
	if result[0] != 20 || result[1] != 40 {
		t.Fatalf("expected scaled (20,40), got (%f,%f)",
			result[0], result[1])
	}
	if result[2] != 60 || result[3] != 80 {
		t.Fatalf("expected scaled (60,80), got (%f,%f)",
			result[2], result[3])
	}
}

func TestFlattenDefsPathHV(t *testing.T) {
	result := flattenDefsPath("M 0 0 H 50 V 30", 1.0)
	if len(result) != 6 {
		t.Fatalf("expected 6 coords, got %d", len(result))
	}
	if result[2] != 50 || result[3] != 0 {
		t.Fatalf("expected H(50,0), got (%f,%f)",
			result[2], result[3])
	}
	if result[4] != 50 || result[5] != 30 {
		t.Fatalf("expected V(50,30), got (%f,%f)",
			result[4], result[5])
	}
}

func TestFlattenDefsPathCubic(t *testing.T) {
	result := flattenDefsPath(
		"M 0 0 C 10 0 10 10 0 10", 1.0)
	// Should have M(2) + adaptive cubic steps (min 4 → 8 coords).
	if len(result) < 10 {
		t.Fatalf("expected many coords for cubic, got %d",
			len(result))
	}
	// Last point should be near (0,10).
	lastX := result[len(result)-2]
	lastY := result[len(result)-1]
	if f32Abs(lastX) > 0.01 || f32Abs(lastY-10) > 0.01 {
		t.Fatalf("expected end near (0,10), got (%f,%f)",
			lastX, lastY)
	}
}

func TestFlattenDefsPathQuadratic(t *testing.T) {
	result := flattenDefsPath("M 0 0 Q 50 50 100 0", 1.0)
	if len(result) < 10 {
		t.Fatalf("expected many coords for quadratic, got %d",
			len(result))
	}
	lastX := result[len(result)-2]
	lastY := result[len(result)-1]
	if f32Abs(lastX-100) > 0.01 || f32Abs(lastY) > 0.01 {
		t.Fatalf("expected end near (100,0), got (%f,%f)",
			lastX, lastY)
	}
}

func TestFlattenDefsPathClose(t *testing.T) {
	result := flattenDefsPath("M 0 0 L 10 0 L 10 10 Z", 1.0)
	lastX := result[len(result)-2]
	lastY := result[len(result)-1]
	if lastX != 0 || lastY != 0 {
		t.Fatalf("Z should return to origin, got (%f,%f)",
			lastX, lastY)
	}
}

func TestFlattenDefsPathArc(t *testing.T) {
	// Simple semicircle arc.
	result := flattenDefsPath(
		"M 0 0 A 50 50 0 0 1 100 0", 1.0)
	if len(result) < 6 {
		t.Fatalf("expected coords for arc, got %d",
			len(result))
	}
	lastX := result[len(result)-2]
	lastY := result[len(result)-1]
	if f32Abs(lastX-100) > 1 || f32Abs(lastY) > 1 {
		t.Fatalf("expected end near (100,0), got (%f,%f)",
			lastX, lastY)
	}
}

func TestFlattenDefsPathSmoothCubic(t *testing.T) {
	// C then S: S reflects last control point of C through
	// current point.
	result := flattenDefsPath(
		"M 0 0 C 0 10 10 10 10 0 S 20 -10 20 0", 1.0)
	if len(result) < 10 {
		t.Fatalf("expected many coords for C+S, got %d",
			len(result))
	}
	// Last point should be near (20,0).
	lastX := result[len(result)-2]
	lastY := result[len(result)-1]
	if f32Abs(lastX-20) > 0.01 || f32Abs(lastY) > 0.01 {
		t.Fatalf("expected end near (20,0), got (%f,%f)",
			lastX, lastY)
	}
}

func TestFlattenDefsPathRelativeSmoothCubic(t *testing.T) {
	result := flattenDefsPath(
		"M 0 0 c 0 10 10 10 10 0 s 10 -10 10 0", 1.0)
	if len(result) < 10 {
		t.Fatalf("expected many coords, got %d", len(result))
	}
	lastX := result[len(result)-2]
	lastY := result[len(result)-1]
	if f32Abs(lastX-20) > 0.01 || f32Abs(lastY) > 0.01 {
		t.Fatalf("expected end near (20,0), got (%f,%f)",
			lastX, lastY)
	}
}

func TestFlattenDefsPathSmoothQuadratic(t *testing.T) {
	// Q then T: T reflects control point of Q.
	result := flattenDefsPath(
		"M 0 0 Q 5 10 10 0 T 20 0", 1.0)
	if len(result) < 8 {
		t.Fatalf("expected many coords for Q+T, got %d",
			len(result))
	}
	lastX := result[len(result)-2]
	lastY := result[len(result)-1]
	if f32Abs(lastX-20) > 0.01 || f32Abs(lastY) > 0.01 {
		t.Fatalf("expected end near (20,0), got (%f,%f)",
			lastX, lastY)
	}
}

func TestFlattenDefsPathRelativeSmoothQuadratic(t *testing.T) {
	result := flattenDefsPath(
		"M 0 0 q 5 10 10 0 t 10 0", 1.0)
	if len(result) < 8 {
		t.Fatalf("expected many coords, got %d", len(result))
	}
	lastX := result[len(result)-2]
	lastY := result[len(result)-1]
	if f32Abs(lastX-20) > 0.01 || f32Abs(lastY) > 0.01 {
		t.Fatalf("expected end near (20,0), got (%f,%f)",
			lastX, lastY)
	}
}

func TestAdaptiveStepsLargeCurveMoreSegments(t *testing.T) {
	small := adaptiveSteps(0, 0, 10, 0, 1.0)
	large := adaptiveSteps(0, 0, 500, 0, 1.0)
	if large <= small {
		t.Fatalf("large curve (%d) should have more segments "+
			"than small (%d)", large, small)
	}
}

func TestAdaptiveStepsClampRange(t *testing.T) {
	tiny := adaptiveSteps(0, 0, 1, 0, 1.0)
	if tiny < 4 {
		t.Fatalf("minimum steps should be 4, got %d", tiny)
	}
	huge := adaptiveSteps(0, 0, 10000, 0, 1.0)
	if huge > 64 {
		t.Fatalf("maximum steps should be 64, got %d", huge)
	}
}

func TestBuildArcLengthTableEmpty(t *testing.T) {
	table, total := buildArcLengthTable(nil)
	if len(table) != 0 || total != 0 {
		t.Fatal("expected empty")
	}
}

func TestBuildArcLengthTableSingle(t *testing.T) {
	table, total := buildArcLengthTable([]float32{0, 0})
	if len(table) != 1 || total != 0 {
		t.Fatal("expected single entry with 0 total")
	}
}

func TestBuildArcLengthTableStraightLine(t *testing.T) {
	// 3 points along X axis: 0, 10, 30.
	polyline := []float32{0, 0, 10, 0, 30, 0}
	table, total := buildArcLengthTable(polyline)
	if len(table) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(table))
	}
	if total != 30 {
		t.Fatalf("expected total 30, got %f", total)
	}
	if table[1] != 10 {
		t.Fatalf("expected table[1]=10, got %f", table[1])
	}
}

func TestBuildArcLengthTableDiagonal(t *testing.T) {
	polyline := []float32{0, 0, 3, 4}
	table, total := buildArcLengthTable(polyline)
	if len(table) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(table))
	}
	if f32Abs(total-5) > 0.001 {
		t.Fatalf("expected total 5, got %f", total)
	}
}

func TestSamplePathAtEmpty(t *testing.T) {
	x, y, a := samplePathAt(nil, nil, 0)
	if x != 0 || y != 0 || a != 0 {
		t.Fatal("expected zeros")
	}
}

func TestSamplePathAtSinglePoint(t *testing.T) {
	poly := []float32{5, 10}
	table := []float32{0}
	x, y, a := samplePathAt(poly, table, 0)
	if x != 5 || y != 10 || a != 0 {
		t.Fatalf("expected (5,10,0), got (%f,%f,%f)", x, y, a)
	}
}

func TestSamplePathAtStart(t *testing.T) {
	poly := []float32{0, 0, 10, 0}
	table := []float32{0, 10}
	x, y, angle := samplePathAt(poly, table, 0)
	if x != 0 || y != 0 {
		t.Fatalf("expected (0,0), got (%f,%f)", x, y)
	}
	if angle != 0 {
		t.Fatalf("expected angle 0, got %f", angle)
	}
}

func TestSamplePathAtEnd(t *testing.T) {
	poly := []float32{0, 0, 10, 0}
	table := []float32{0, 10}
	x, y, angle := samplePathAt(poly, table, 10)
	if x != 10 || y != 0 {
		t.Fatalf("expected (10,0), got (%f,%f)", x, y)
	}
	if angle != 0 {
		t.Fatalf("expected angle 0, got %f", angle)
	}
}

func TestSamplePathAtMidpoint(t *testing.T) {
	poly := []float32{0, 0, 10, 0}
	table := []float32{0, 10}
	x, y, _ := samplePathAt(poly, table, 5)
	if f32Abs(x-5) > 0.01 || f32Abs(y) > 0.01 {
		t.Fatalf("expected (5,0), got (%f,%f)", x, y)
	}
}

func TestSamplePathAtVertical(t *testing.T) {
	poly := []float32{0, 0, 0, 10}
	table := []float32{0, 10}
	_, _, angle := samplePathAt(poly, table, 5)
	expected := float32(math.Pi / 2)
	if f32Abs(angle-expected) > 0.01 {
		t.Fatalf("expected angle %f, got %f", expected, angle)
	}
}

func TestSamplePathAtMultiSegment(t *testing.T) {
	poly := []float32{0, 0, 10, 0, 20, 0}
	table := []float32{0, 10, 20}
	x, y, _ := samplePathAt(poly, table, 15)
	if f32Abs(x-15) > 0.01 || f32Abs(y) > 0.01 {
		t.Fatalf("expected (15,0), got (%f,%f)", x, y)
	}
}

func TestArcLengthAccuracy(t *testing.T) {
	// Triangle path: 3-4-5 right triangle.
	poly := []float32{0, 0, 3, 0, 3, 4}
	table, total := buildArcLengthTable(poly)
	_ = table
	// Total should be 3 + 4 = 7.
	if f32Abs(total-7) > 0.001 {
		t.Fatalf("expected total 7, got %f", total)
	}
}
