package gui

import (
	"math"
	"slices"
	"testing"
)

// CanvasPath.TrimTo (#1023): the part of a path between two
// fractions of its flattened length. These tests pin the issue's
// cases — identity, a line cut at its middle, a cut across two
// segments, curves measured at the stroke tolerance — plus the
// closed-contour, aliasing and allocation rules.

// flatPathLen sums the length of p as the draw flattens it, at the
// stroke tolerance and an untransformed scale of 1.
func flatPathLen(p *CanvasPath) float64 {
	var dc DrawContext
	total := 0.0
	for _, c := range dc.flattenPathContours(p, bezierTol) {
		for i := 2; i+1 < len(c); i += 2 {
			dx := float64(c[i] - c[i-2])
			dy := float64(c[i+1] - c[i-1])
			total += math.Hypot(dx, dy)
		}
	}
	return total
}

// samePath reports whether two paths hold the same segments.
func samePath(a, b *CanvasPath) bool {
	return slices.Equal(a.cmds, b.cmds) && slices.Equal(a.pts, b.pts)
}

// mixedPath has every segment kind: a closed contour of line,
// quad, cubic and an arc reached through its connecting line, then
// an open second contour.
func mixedPath() *CanvasPath {
	var p CanvasPath
	p.MoveTo(0, 0)
	p.LineTo(40, 0)
	p.QuadTo(60, 20, 40, 40)
	p.CubicTo(30, 60, 10, 20, 0, 40)
	p.ArcTo(-20, 20, 10, 10, 0, math.Pi)
	p.Close()
	p.MoveTo(100, 100)
	p.LineTo(140, 100)
	return &p
}

func TestTrimTo_FullIsIdentity(t *testing.T) {
	p := mixedPath()
	var dst CanvasPath
	p.TrimTo(&dst, 0, 1)
	if !samePath(&dst, p) {
		t.Fatalf("TrimTo(0, 1)\n got cmds %v pts %v\nwant cmds %v pts %v",
			dst.cmds, dst.pts, p.cmds, p.pts)
	}
}

// The commands come back as they were also where a draw would
// change nothing: an arc whose start differs from the MoveTo point
// draws its own connecting line, and a zero-length LineTo is kept.
func TestTrimTo_FullKeepsCommands(t *testing.T) {
	var p CanvasPath
	p.MoveTo(0, 0)
	p.ArcTo(30, 0, 10, 10, math.Pi, math.Pi)
	p.LineTo(40, 20)
	p.LineTo(40, 20)
	p.LineTo(0, 20)
	var dst CanvasPath
	p.TrimTo(&dst, 0, 1)
	if !samePath(&dst, &p) {
		t.Fatalf("TrimTo(0, 1)\n got cmds %v pts %v\nwant cmds %v pts %v",
			dst.cmds, dst.pts, p.cmds, p.pts)
	}
}

// Past a full turn the draw stops the arc at start ± 2π and the next
// line starts from there, not from the arc's stored end. A cut inside
// that line must land on the line as drawn.
func TestTrimTo_LineAfterOverTurnArc(t *testing.T) {
	var p, dst CanvasPath
	p.ArcTo(0, 0, 10, 10, 0, 2*math.Pi+1)
	p.LineTo(30, 0)
	// The drawn line runs (10,0) to (30,0): cut 10 units before its end.
	total := flatPathLen(&p)
	p.TrimTo(&dst, float32((total-10)/total), 1)
	if len(dst.cmds) != 2 || dst.cmds[0] != pathCmdMoveTo {
		t.Fatalf("got cmds %v pts %v, want a MoveTo and a LineTo",
			dst.cmds, dst.pts)
	}
	if x, y := dst.pts[0], dst.pts[1]; f32Abs(x-20) > 0.05 || f32Abs(y) > 0.05 {
		t.Fatalf("cut starts at (%v, %v), want (20, 0) on the drawn line", x, y)
	}
}

func TestTrimTo_LineMiddle(t *testing.T) {
	var p, dst CanvasPath
	p.MoveTo(0, 0)
	p.LineTo(10, 0)
	p.TrimTo(&dst, 0, 0.5)
	want := []float32{0, 0, 5, 0}
	if !slices.Equal(dst.cmds, []canvasPathCmd{pathCmdMoveTo, pathCmdLineTo}) ||
		!slices.Equal(dst.pts, want) {
		t.Fatalf("got cmds %v pts %v, want a line to %v", dst.cmds,
			dst.pts, want)
	}
}

// Two segments of length 10: the middle half starts in the first
// one and stops in the second, keeping the corner between them.
func TestTrimTo_AcrossTwoSegments(t *testing.T) {
	var p, dst CanvasPath
	p.MoveTo(0, 0)
	p.LineTo(10, 0)
	p.LineTo(10, 10)
	p.TrimTo(&dst, 0.25, 0.75)
	want := []float32{5, 0, 10, 0, 10, 5}
	if !slices.Equal(dst.cmds, []canvasPathCmd{pathCmdMoveTo,
		pathCmdLineTo, pathCmdLineTo}) || !slices.Equal(dst.pts, want) {
		t.Fatalf("got cmds %v pts %v, want %v", dst.cmds, dst.pts, want)
	}
}

// For each curve kind, the first half of the path flattens to half
// the length of the whole, within the flattening tolerance. Curves
// stay curves: the cut is exact, not a polyline.
func TestTrimTo_CurveHalfLength(t *testing.T) {
	cases := map[string]func(p *CanvasPath){
		"quad": func(p *CanvasPath) {
			p.MoveTo(0, 0)
			p.QuadTo(50, 100, 100, 0)
		},
		"cubic": func(p *CanvasPath) {
			p.MoveTo(0, 0)
			p.CubicTo(0, 100, 100, -50, 100, 50)
		},
		"arc": func(p *CanvasPath) {
			p.ArcTo(0, 0, 80, 30, 0.3, 4)
		},
	}
	for name, build := range cases {
		t.Run(name, func(t *testing.T) {
			var p, first, second CanvasPath
			build(&p)
			p.TrimTo(&first, 0, 0.5)
			p.TrimTo(&second, 0.5, 1)
			full := flatPathLen(&p)
			for _, half := range []*CanvasPath{&first, &second} {
				if got := flatPathLen(half); math.Abs(got-full/2) > float64(bezierTol) {
					t.Errorf("half length %f, want %f ± %v", got,
						full/2, bezierTol)
				}
				if got := half.cmds[len(half.cmds)-1]; got != p.cmds[len(p.cmds)-1] {
					t.Errorf("last cmd %v, want the curve kind %v", got,
						p.cmds[len(p.cmds)-1])
				}
			}
		})
	}
}

// Length is measured over all subpaths together: two contours of
// length 10, total 20, so 0.25 to 0.75 is 5 to 15.
func TestTrimTo_Subpaths(t *testing.T) {
	var p, dst CanvasPath
	p.MoveTo(0, 0)
	p.LineTo(10, 0)
	p.MoveTo(0, 10)
	p.LineTo(10, 10)
	p.TrimTo(&dst, 0.25, 0.75)
	want := []float32{5, 0, 10, 0, 0, 10, 5, 10}
	if !slices.Equal(dst.cmds, []canvasPathCmd{pathCmdMoveTo,
		pathCmdLineTo, pathCmdMoveTo, pathCmdLineTo}) ||
		!slices.Equal(dst.pts, want) {
		t.Fatalf("got cmds %v pts %v, want %v", dst.cmds, dst.pts, want)
	}
}

// A closed contour cut short is open: it gets end caps when
// stroked. The closing edge counts toward the length and comes back
// as a line.
func TestTrimTo_ClosedContourPartial(t *testing.T) {
	var p, dst CanvasPath
	p.MoveTo(0, 0)
	p.LineTo(10, 0)
	p.LineTo(10, 10)
	p.LineTo(0, 10)
	p.Close()
	p.TrimTo(&dst, 0.5, 1)
	want := []float32{10, 10, 0, 10, 0, 0}
	if !slices.Equal(dst.cmds, []canvasPathCmd{pathCmdMoveTo,
		pathCmdLineTo, pathCmdLineTo}) || !slices.Equal(dst.pts, want) {
		t.Fatalf("got cmds %v pts %v, want %v", dst.cmds, dst.pts, want)
	}
}

func TestTrimTo_EmptyResults(t *testing.T) {
	line := func() *CanvasPath {
		var p CanvasPath
		p.MoveTo(0, 0)
		p.LineTo(10, 0)
		return &p
	}
	nan := float32(math.NaN())
	cases := []struct {
		name     string
		p        *CanvasPath
		from, to float32
	}{
		{"empty path", &CanvasPath{}, 0, 1},
		{"move only", func() *CanvasPath {
			var p CanvasPath
			p.MoveTo(3, 4)
			return &p
		}(), 0, 1},
		{"from > to", line(), 0.6, 0.4},
		{"from == to", line(), 0.5, 0.5},
		{"NaN from", line(), nan, 1},
		{"NaN to", line(), 0, nan},
		{"both below range", line(), -2, -1},
		{"NaN point", func() *CanvasPath {
			var p CanvasPath
			p.MoveTo(0, 0)
			p.LineTo(nan, 0)
			return &p
		}(), 0, 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// A stale dst must come back empty, not keep its segments.
			var dst CanvasPath
			dst.MoveTo(1, 1)
			dst.LineTo(2, 2)
			c.p.TrimTo(&dst, c.from, c.to)
			if !dst.IsEmpty() {
				t.Fatalf("got cmds %v pts %v, want empty", dst.cmds, dst.pts)
			}
		})
	}
}

// Out-of-range values clamp to the ends of the path.
func TestTrimTo_Clamps(t *testing.T) {
	p := mixedPath()
	var dst CanvasPath
	p.TrimTo(&dst, float32(math.Inf(-1)), 7)
	if !samePath(&dst, p) {
		t.Fatalf("clamped trim is not the path: cmds %v", dst.cmds)
	}
}

// A dst that is p, or a struct copy sharing p's arrays, must not
// corrupt p while it is read.
func TestTrimTo_Aliasing(t *testing.T) {
	p := mixedPath()
	want := mixedPath()
	p.TrimTo(p, 0.25, 0.75)
	if !samePath(p, want) {
		t.Fatal("TrimTo(p, …) changed p")
	}
	alias := *p
	p.TrimTo(&alias, 0.25, 0.75)
	if !samePath(p, want) {
		t.Fatal("TrimTo into a copy of p changed p")
	}
	var fresh CanvasPath
	p.TrimTo(&fresh, 0.25, 0.75)
	if !samePath(&alias, &fresh) {
		t.Fatal("TrimTo into a copy of p gave a different result")
	}
}

// A nil dst is a no-op, not a panic.
func TestTrimTo_NilDst(t *testing.T) {
	p := mixedPath()
	p.TrimTo(nil, 0, 1)
	if !samePath(p, mixedPath()) {
		t.Fatal("TrimTo(nil, …) changed p")
	}
}

// A counterclockwise arc cut at half its length keeps its direction:
// the cut sweep is negative and about half the whole.
func TestTrimTo_NegativeSweepArc(t *testing.T) {
	var p, dst CanvasPath
	p.ArcTo(0, 0, 20, 20, 0, -math.Pi)
	p.TrimTo(&dst, 0, 0.5)
	if len(dst.cmds) != 2 || dst.cmds[1] != pathCmdArcTo {
		t.Fatalf("got cmds %v, want a MoveTo and an ArcTo", dst.cmds)
	}
	start, sweep := dst.pts[6], dst.pts[7]
	if start != 0 || math.Abs(float64(sweep)+math.Pi/2) > 0.01 {
		t.Fatalf("got start %v sweep %v, want 0 and -π/2", start, sweep)
	}
}

// A closed contour that lies whole inside the range stays closed,
// also when the range starts in an earlier contour.
func TestTrimTo_WholeClosedContourLater(t *testing.T) {
	var p, dst CanvasPath
	p.MoveTo(0, 0)
	p.LineTo(40, 0)
	p.MoveTo(0, 10)
	p.LineTo(10, 10)
	p.LineTo(10, 20)
	p.Close()
	// Total 40 + 10 + 10 + 14.14: from 0.5 is 32, in the first contour.
	p.TrimTo(&dst, 0.5, 1)
	if got := dst.cmds[len(dst.cmds)-1]; got != pathCmdClose {
		t.Fatalf("got cmds %v, want the second contour closed", dst.cmds)
	}
}

// A closed contour whose last point is its start has a closing edge of
// length 0. A range that ends exactly at the contour's end still holds
// the contour whole, so it comes back closed (round join, no caps).
func TestTrimTo_ClosedContourEndsAtRangeEnd(t *testing.T) {
	var p, dst CanvasPath
	p.MoveTo(0, 0)
	p.LineTo(10, 0)
	p.LineTo(10, 10)
	p.LineTo(0, 10)
	p.LineTo(0, 0)
	p.Close()
	p.MoveTo(100, 100)
	p.LineTo(140, 100)
	// Two contours of length 40: half of the total ends the first one.
	p.TrimTo(&dst, 0, 0.5)
	want := []canvasPathCmd{pathCmdMoveTo, pathCmdLineTo, pathCmdLineTo,
		pathCmdLineTo, pathCmdLineTo, pathCmdClose}
	if !slices.Equal(dst.cmds, want) {
		t.Fatalf("got cmds %v, want the first contour whole and closed", dst.cmds)
	}
}

// Past a full turn an arc stores its end at start+sweep, and a curve
// that follows starts there. A cut that keeps the arc's tail must not
// move that start: dst gets a MoveTo to the stored end for the curve.
func TestTrimTo_CurveAfterOverTurnArc(t *testing.T) {
	var p, dst CanvasPath
	p.ArcTo(0, 0, 10, 10, 0, 2*math.Pi+1)
	p.QuadTo(30, 10, 40, 0)
	p.TrimTo(&dst, 0.1, 1)
	n := len(dst.cmds)
	if n < 2 || dst.cmds[n-1] != pathCmdQuadTo || dst.cmds[n-2] != pathCmdMoveTo {
		t.Fatalf("got cmds %v, want a MoveTo before the QuadTo", dst.cmds)
	}
	// The source quad starts at the stored end of the arc, angle 2π+1.
	wx, wy := float32(10*math.Cos(1)), float32(10*math.Sin(1))
	mx, my := dst.pts[len(dst.pts)-6], dst.pts[len(dst.pts)-5]
	if f32Abs(mx-wx) > 0.01 || f32Abs(my-wy) > 0.01 {
		t.Fatalf("quad starts at (%v, %v), want (%v, %v)", mx, my, wx, wy)
	}
}

// A trim redone every frame into the same dst stops allocating
// after the first one.
func TestTrimTo_DoesNotAllocate(t *testing.T) {
	p := mixedPath()
	var dst CanvasPath
	progress := float32(0)
	run := func() {
		progress += 0.01
		if progress > 1 {
			progress = 0
		}
		p.TrimTo(&dst, 0, progress)
	}
	p.TrimTo(&dst, 0, 1)
	if n := testing.AllocsPerRun(100, run); n != 0 {
		t.Errorf("allocs per run = %v, want 0", n)
	}
}

// Any input trims without a panic, and a trim of a finite path
// holds only finite points.
func FuzzCanvasPathTrimTo(f *testing.F) {
	f.Add(float32(0), float32(1), []byte{0, 1, 2, 3, 4, 5},
		float32(10), float32(20))
	f.Add(float32(math.NaN()), float32(math.Inf(1)), []byte{1, 5, 3},
		float32(-1), float32(0))
	f.Add(float32(0.2), float32(0.8), []byte{}, float32(0), float32(0))
	f.Fuzz(func(t *testing.T, from, to float32, ops []byte, a, b float32) {
		var p, dst CanvasPath
		for i, op := range ops {
			x := a + float32(i)*b
			y := b - float32(i)*a
			switch op % 6 {
			case 0:
				p.MoveTo(x, y)
			case 1:
				p.LineTo(x, y)
			case 2:
				p.QuadTo(y, x, x, -y)
			case 3:
				p.CubicTo(x, x, y, y, -x, y)
			case 4:
				p.ArcTo(x, y, a, b, x, y)
			case 5:
				p.Close()
			}
		}
		p.TrimTo(&dst, from, to)
		if f32AllFinite(p.pts) && !f32AllFinite(dst.pts) {
			t.Fatalf("non-finite trim of a finite path: %v", dst.pts)
		}
	})
}
