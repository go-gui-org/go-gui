package soft

import (
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

// A stroke must land on whole device pixels: each edge one or more
// full-strength rows (columns), never partial ones (#1334). Two causes
// leave partial rows, and the cases separate them:
//   - a fractional device edge: layout centring produces y = n+0.5
//     (TestMiddleAlignedBorderStrokeKeepsFractionalY);
//   - a fractional device thickness: Thickness 1 at scale 1.25 is
//     1.25 px, so the fix rounds thickness too (minimum 1 px).
func TestStrokeRectEdgesSnapToDevicePixels(t *testing.T) {
	cases := []struct {
		name      string
		x, y      float32
		scale     float32
		wantEdgPx int // full rows per edge after snapping
	}{
		{"whole", 10, 10, 1, 1},
		{"y quarter", 10, 10.25, 1, 1},
		{"y half", 10, 10.5, 1, 1},
		{"y three quarters", 10, 10.75, 1, 1},
		{"x half", 10.5, 10, 1, 1},
		{"x and y half", 10.5, 10.5, 1, 1},
		// Device edges fractional, thickness whole (2 px).
		{"scale 2 y quarter", 10, 10.25, 2, 2},
		// Device edges whole (8*1.25 = 10), thickness 1.25 px.
		{"scale 1.25 thickness", 8, 8, 1.25, 1},
	}
	for _, c := range cases {
		const w, h = 40, 20
		r := newRenderer(140, 100, c.scale)
		r.drawAll([]gui.RenderCmd{{Kind: gui.RenderStrokeRect,
			X: c.x, Y: c.y, W: w, H: h, Radius: 4, Thickness: 1,
			Color: gui.RGB(255, 255, 255)}})
		// Scan through the box's middle, clear of the rounded corners:
		// down one column for the top and bottom edges, across one row
		// for the left and right edges.
		midX := int((c.x + w/2) * c.scale)
		midY := int((c.y + h/2) * c.scale)
		for _, scan := range []struct {
			edges string
			n     int
			px    func(i int) uint8
		}{
			{"top/bottom", 100, func(i int) uint8 {
				_, g, _, _ := at(r.buf.img, midX, i)
				return g
			}},
			{"left/right", 140, func(i int) uint8 {
				_, g, _, _ := at(r.buf.img, i, midY)
				return g
			}},
		} {
			full, partial := 0, 0
			for i := range scan.n {
				switch g := scan.px(i); {
				case g >= 250:
					full++
				case g > 5:
					partial++
				}
			}
			if full != 2*c.wantEdgPx || partial != 0 {
				t.Errorf("%s: %s edges: %d full, %d partial; want %d full, 0 partial",
					c.name, scan.edges, full, partial, 2*c.wantEdgPx)
			}
		}
	}
}

// A fill and its border snap by the same rule, so no fill pixel lands
// outside the border (#1334).
func TestFillStaysInsideSnappedBorder(t *testing.T) {
	for _, s := range []float32{1, 1.25, 1.5} {
		for _, y := range []float32{10, 10.25, 10.5, 10.75} {
			box := func(k gui.RenderKind, c gui.Color) gui.RenderCmd {
				return gui.RenderCmd{Kind: k, X: 10, Y: y, W: 40, H: 20,
					Radius: 4, Thickness: 1, Fill: true, Color: c}
			}
			grad := box(gui.RenderGradient, gui.Color{})
			grad.Gradient = &gui.GradientDef{Stops: []gui.GradientStop{
				{Color: gui.RGB(0, 255, 0), Pos: 0},
				{Color: gui.RGB(0, 255, 0), Pos: 1},
			}}
			for _, fill := range []gui.RenderCmd{
				box(gui.RenderRect, gui.RGB(0, 255, 0)), grad,
			} {
				r := newRenderer(100, 80, s)
				r.drawAll([]gui.RenderCmd{
					fill, box(gui.RenderStrokeRect, gui.RGB(255, 0, 0)),
				})
				name := "solid"
				if fill.Kind == gui.RenderGradient {
					name = "gradient"
				}
				// The border's outer bounds, then every fill pixel must
				// lie inside them: above, below and beside.
				x0, y0, x1, y1 := 100, 80, -1, -1
				for py := range 80 {
					for px := range 100 {
						if red, _, _, _ := at(r.buf.img, px, py); red > 0 {
							x0, y0 = min(x0, px), min(y0, py)
							x1, y1 = max(x1, px), max(y1, py)
						}
					}
				}
				if x1 < 0 {
					t.Fatalf("scale %v y=%v %v: no border drawn", s, y, name)
				}
				leaks := 0
				for py := range 80 {
					for px := range 100 {
						_, green, _, _ := at(r.buf.img, px, py)
						if green > 0 && (px < x0 || px > x1 || py < y0 || py > y1) {
							leaks++
						}
					}
				}
				if leaks > 0 {
					t.Errorf("scale %v y=%v %v: %d fill pixels outside the border (%d,%d)-(%d,%d)",
						s, y, name, leaks, x0, y0, x1, y1)
				}
			}
		}
	}
}

// Gradient border strips snap like a solid border: each edge whole
// device pixels, no partial rows, and top and bottom the same width
// at a fractional scale (#1334).
func TestGradientBorderSnapsToDevicePixels(t *testing.T) {
	for _, s := range []float32{1, 1.25, 1.5} {
		for _, y := range []float32{10, 10.25, 10.5, 10.75} {
			r := newRenderer(100, 80, s)
			r.drawAll([]gui.RenderCmd{{Kind: gui.RenderGradientBorder,
				X: 10, Y: y, W: 40, H: 20, Thickness: 1,
				Gradient: &gui.GradientDef{Stops: []gui.GradientStop{
					{Color: gui.RGB(255, 255, 255), Pos: 0},
					{Color: gui.RGB(255, 255, 255), Pos: 1},
				}}}})
			// Device rows of the top and bottom edges, split at the
			// box's middle.
			mid := int((y + 10) * s)
			var top, bottom, partial int
			for py := range 80 {
				switch _, g, _, _ := at(r.buf.img, int(30*s), py); {
				case g >= 250 && py < mid:
					top++
				case g >= 250:
					bottom++
				case g > 5:
					partial++
				}
			}
			want := 1
			if s == 1.5 {
				want = 2 // 1.5 px rounds up
			}
			if top != want || bottom != want || partial != 0 {
				t.Errorf("scale %v y=%v: top %d, bottom %d, partial %d rows; want %d, %d, 0",
					s, y, top, bottom, partial, want, want)
			}
		}
	}
}
