//go:build darwin && cgo && !ios

package metal

import "testing"

// TestTriangleEdgesAntialiased guards #823. SVG fills reach the GPU
// as bare triangles whose fragment shader returns full alpha, so the
// only source of edge antialiasing is multisampling in the main
// pass. With a single-sample pass every pixel on a diagonal edge is
// either fully in or fully out, and a large-viewBox icon scaled to
// 25 px renders as hard pixel stairs with borders snapped to whole
// pixels.
//
// The probe draws a right triangle across a 16x16 target; its
// hypotenuse crosses about 16 pixels. 4x MSAA gives those pixels
// 25%, 50% or 75% coverage.
func TestTriangleEdgesAntialiased(t *testing.T) {
	n := testEdgeCoverage()
	switch {
	case n == -1:
		t.Skip("no Metal device (headless or virtualized runner)")
	case n < 0:
		t.Fatalf("edge coverage probe setup failed (rc=%d)", n)
	case n < 8:
		t.Fatalf("diagonal triangle edge has %d partly covered "+
			"pixels, want >= 8: the main pass is not multisampled", n)
	}
}
