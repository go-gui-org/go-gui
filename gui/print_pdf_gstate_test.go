package gui

import (
	"bytes"
	"math"
	"testing"

	"github.com/go-pdf/fpdf"
)

// pdfGRec simulates the PDF graphics state the ops build: q saves the
// matrix and clips, Q restores them, cm multiplies the matrix, and a
// clip is kept with the matrix it was drawn under.
type pdfGRec struct {
	t     *testing.T
	ctm   pdfAffine
	clips []pdfRecClip
	saved []pdfGRecState
	kinds []bool // true: a clip's q
}

type pdfRecClip struct {
	x, y, w, h float64
	ctm        pdfAffine
}

type pdfGRecState struct {
	ctm   pdfAffine
	clips []pdfRecClip
}

func newPDFGRec(t *testing.T) *pdfGRec {
	return &pdfGRec{t: t, ctm: pdfAffineIdentity}
}

func (r *pdfGRec) q(clip bool) {
	r.saved = append(r.saved, pdfGRecState{r.ctm, append([]pdfRecClip(nil), r.clips...)})
	r.kinds = append(r.kinds, clip)
}

func (r *pdfGRec) bigQ(clip bool) {
	n := len(r.saved)
	if n == 0 {
		r.t.Fatal("Q with no q")
	}
	if r.kinds[n-1] != clip {
		r.t.Fatalf("Q of the wrong kind (clip=%v)", clip)
	}
	r.ctm, r.clips = r.saved[n-1].ctm, r.saved[n-1].clips
	r.saved, r.kinds = r.saved[:n-1], r.kinds[:n-1]
}

func (r *pdfGRec) TransformBegin() { r.q(false) }
func (r *pdfGRec) TransformEnd()   { r.bigQ(false) }
func (r *pdfGRec) ClipEnd()        { r.bigQ(true) }

func (r *pdfGRec) Transform(tm fpdf.TransformMatrix) {
	if len(r.kinds) == 0 {
		r.t.Fatal("cm outside a transform")
	}
	r.ctm = r.ctm.mul(pdfAffine{tm.A, tm.B, tm.C, tm.D, tm.E, tm.F})
}

func (r *pdfGRec) ClipRect(x, y, w, h float64, _ bool) {
	r.q(true)
	r.clips = append(r.clips, pdfRecClip{x, y, w, h, r.ctm})
}

// newTestPDFGState uses a page where logical and native space differ
// (scale, margin, y flip), so a wrong conjugation shows.
func newTestPDFGState(t *testing.T) (*pdfGState, *pdfGRec) {
	rec := newPDFGRec(t)
	ctx := &pdfCtx{scale: 0.5, ox: 10, oy: 20}
	return newPDFGState(rec, ctx, 2.8, 300), rec
}

func affNear(a, b pdfAffine) bool {
	for i := range a {
		if math.Abs(a[i]-b[i]) > 1e-4 {
			return false
		}
	}
	return true
}

// nativeOf is the PDF matrix the state must have in force for logical
// matrix m.
func nativeOf(g *pdfGState, m pdfAffine) pdfAffine {
	return g.phi.mul(m).mul(g.phiInv)
}

// The PDF matrix moves a point where the logical matrix moves it.
func TestPDFNativeConjugates(t *testing.T) {
	g, _ := newTestPDFGState(t)
	m := pdfAffine{0.8, 0.6, -0.6, 0.8, 12, -7}
	tm := g.native(m)
	n := pdfAffine{tm.A, tm.B, tm.C, tm.D, tm.E, tm.F}
	px, py := 30.0, 40.0
	// phi(m(p)) and n(phi(p)) must agree.
	lx, ly := m[0]*px+m[2]*py+m[4], m[1]*px+m[3]*py+m[5]
	want := [2]float64{g.phi[0]*lx + g.phi[2]*ly + g.phi[4], g.phi[1]*lx + g.phi[3]*ly + g.phi[5]}
	fx, fy := g.phi[0]*px+g.phi[2]*py+g.phi[4], g.phi[1]*px+g.phi[3]*py+g.phi[5]
	got := [2]float64{n[0]*fx + n[2]*fy + n[4], n[1]*fx + n[3]*fy + n[5]}
	if math.Abs(got[0]-want[0]) > 1e-6 || math.Abs(got[1]-want[1]) > 1e-6 {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestPDFTransformSetsMatrix(t *testing.T) {
	g, rec := newTestPDFGState(t)
	h := [9]float32{1, 0, 10, 0, 1, 5, 0, 0, 1}
	g.beginTransform(&RenderCmd{Homography: &h})
	if want := nativeOf(g, pdfAffine{1, 0, 0, 1, 10, 5}); !affNear(rec.ctm, want) {
		t.Fatalf("ctm %v, want %v", rec.ctm, want)
	}
	g.pop()
	if !affNear(rec.ctm, pdfAffineIdentity) || len(rec.saved) != 0 {
		t.Fatalf("after end ctm %v, %d q open", rec.ctm, len(rec.saved))
	}
}

// Regression: a RenderClip inside a rotation ended the rotation's q,
// so everything after it drew unturned.
func TestPDFClipInsideRotationKeepsRotation(t *testing.T) {
	g, rec := newTestPDFGState(t)
	g.setClip(&RenderCmd{X: 0, Y: 0, W: 100, H: 100})
	g.beginRotate(&RenderCmd{RotAngle: 90})
	g.setClip(&RenderCmd{X: 10, Y: 10, W: 50, H: 50})
	if want := nativeOf(g, pdfAffine{0, 1, -1, 0, 0, 0}); !affNear(rec.ctm, want) {
		t.Fatalf("ctm %v, want the turn %v", rec.ctm, want)
	}
	// The clip is a window rect: drawn with no transform.
	if len(rec.clips) != 1 || !affNear(rec.clips[0].ctm, pdfAffineIdentity) {
		t.Fatalf("clips %+v, want one window rect", rec.clips)
	}
}

// A stencil goes through the transform it opened under, and keeps it
// when a later rect clip rebuilds the stack.
func TestPDFStencilKeepsItsTransform(t *testing.T) {
	g, rec := newTestPDFGState(t)
	h := [9]float32{1, 0, 10, 0, 1, 0, 0, 0, 1}
	g.beginTransform(&RenderCmd{Homography: &h})
	g.beginStencil(&RenderCmd{X: 0, Y: 0, W: 5, H: 5})
	g.setClip(&RenderCmd{X: 1, Y: 1, W: 9, H: 9})
	move := nativeOf(g, pdfAffine{1, 0, 0, 1, 10, 0})
	if len(rec.clips) != 2 {
		t.Fatalf("clips %+v, want 2", rec.clips)
	}
	if !affNear(rec.clips[0].ctm, move) {
		t.Fatalf("stencil ctm %v, want %v", rec.clips[0].ctm, move)
	}
	if !affNear(rec.clips[1].ctm, pdfAffineIdentity) {
		t.Fatalf("rect clip ctm %v, want identity", rec.clips[1].ctm)
	}
	if !affNear(rec.ctm, move) {
		t.Fatalf("content ctm %v, want %v", rec.ctm, move)
	}
	g.endStencil()
	if len(rec.clips) != 1 {
		t.Fatalf("after stencil end clips %+v, want 1", rec.clips)
	}
}

// A degenerate RenderClip ends the clip outside a view transform, and
// clips everything inside one.
func TestPDFEmptyClip(t *testing.T) {
	g, rec := newTestPDFGState(t)
	g.setClip(&RenderCmd{X: 0, Y: 0, W: 50, H: 50})
	g.setClip(&RenderCmd{})
	if len(rec.clips) != 0 {
		t.Fatalf("clips %+v, want none", rec.clips)
	}
	h := [9]float32{1, 0, 10, 0, 1, 0, 0, 0, 1}
	g.beginTransform(&RenderCmd{Homography: &h})
	g.setClip(&RenderCmd{X: 5, Y: 5})
	if len(rec.clips) != 1 || rec.clips[0].w != 0 || rec.clips[0].h != 0 {
		t.Fatalf("clips %+v, want one empty clip", rec.clips)
	}
}

// Under perspective a rect clip is drawn in the view's frame under the
// flat stand-in, so it holds the whole view.
func TestPDFClipUnderPerspective(t *testing.T) {
	g, rec := newTestPDFGState(t)
	h := xfMat{1, 0, 0, 0, 1, 0, 0.01, 0, 1}
	hp := [9]float32(h)
	g.beginTransform(&RenderCmd{Homography: &hp, W: 100, H: 40})
	f := xfFrame{acc: h, clip: xfUnboundedClip}
	win := f.toWindow(drawClip{Width: 100, Height: 40})
	g.setClip(&RenderCmd{X: win.X, Y: win.Y, W: win.Width, H: win.Height})
	c := g.clips[len(g.clips)-1]
	if c.xf != g.view || c.x > 0.01 || c.y > 0.01 || c.x+c.w < 99.99 ||
		c.y+c.h < 39.99 {
		t.Fatalf("clip %+v does not hold the view under its flat map", c)
	}
	if !affNear(rec.clips[0].ctm, nativeOf(g, g.view)) {
		t.Fatalf("clip drawn under %v", rec.clips[0].ctm)
	}
	g.pop()
	if c = g.clips[len(g.clips)-1]; c.xf != pdfAffineIdentity || c.w != win.Width {
		t.Fatalf("after end clip %+v, want the window rect", c)
	}
}

// A perspective view inside a quarter turn: gui maps the clip out
// through the turn and the view's matrix (#1019), so the clip is mapped
// back through both and starts at the view's corner.
func TestPDFClipUnderPerspectiveInTurn(t *testing.T) {
	g, _ := newTestPDFGState(t)
	g.beginRotate(&RenderCmd{RotAngle: 90, RotCX: 50, RotCY: 20})
	h := xfMat{1, 0, 0, 0, 1, 0, 0.01, 0, 1}
	hp := [9]float32(h)
	g.beginTransform(&RenderCmd{Homography: &hp, W: 100, H: 40})
	turn := quarterTurnForward(&Shape{Width: 100, Height: 40, QuarterTurns: 1})
	f := xfFrame{acc: turn.mul(&h), clip: xfUnboundedClip}
	win := f.toWindow(drawClip{Width: 100, Height: 40})
	g.setClip(&RenderCmd{X: win.X, Y: win.Y, W: win.Width, H: win.Height})
	c := g.clips[len(g.clips)-1]
	// Without the turn in h, the rect maps back to x 42..233, y -100..67.
	if c.xf != g.view || c.x < -0.01 || c.x > 0.01 || c.y < -0.01 ||
		c.y > 0.01 || c.x+c.w < 99.99 || c.y+c.h < 39.99 {
		t.Fatalf("clip %+v does not hold the view under its flat map", c)
	}
}

// A view whose center is behind the viewer draws nothing: an empty clip
// stands in, and the state still balances.
func TestPDFTransformBehindViewer(t *testing.T) {
	g, rec := newTestPDFGState(t)
	h := [9]float32{1, 0, 0, 0, 1, 0, 0, 0, -1}
	g.beginTransform(&RenderCmd{Homography: &h, W: 10, H: 10})
	if n := len(rec.clips); n != 1 || rec.clips[0].w != 0 {
		t.Fatalf("clips %+v, want one empty clip", rec.clips)
	}
	g.pop()
	g.unwind()
	if len(rec.saved) != 0 {
		t.Fatalf("%d q still open", len(rec.saved))
	}
}

// End to end: a stream that mixes clips, rotations and transforms in
// the order the render walk emits them writes a PDF with no fpdf error.
func TestRenderToPDF_TransformBracket(t *testing.T) {
	j := testPrintJob(t)
	h := [9]float32{0.93, 0, 7, 0.1, 1, -11, 0.0016, 0, 0.84}
	cmds := []RenderCmd{
		{Kind: RenderClip, X: 0, Y: 0, W: 320, H: 240},
		{Kind: RenderTransformBegin, Homography: &h, X: 40, Y: 40, W: 120, H: 60},
		{Kind: RenderRect, X: 40, Y: 40, W: 120, H: 60, Color: RGB(60, 120, 200)},
		{Kind: RenderClip, X: 60, Y: 50, W: 70, H: 30},
		{Kind: RenderStencilBegin, X: 55, Y: 55, W: 90, H: 30},
		{Kind: RenderRotateBegin, RotAngle: 90, RotCX: 60, RotCY: 60},
		{Kind: RenderRect, X: 55, Y: 55, W: 20, H: 20, Color: RGB(200, 60, 60)},
		{Kind: RenderClip, X: 0, Y: 0, W: 320, H: 240},
		{Kind: RenderRotateEnd},
		{Kind: RenderStencilEnd, X: 55, Y: 55, W: 90, H: 30},
		{Kind: RenderClip, X: 0, Y: 0, W: 320, H: 240},
		{Kind: RenderTransformEnd},
		{Kind: RenderRect, X: 0, Y: 0, W: 10, H: 10, Color: RGB(0, 0, 0)},
	}
	if err := renderToPDF(cmds, j, 320, 240); err != nil {
		t.Fatal(err)
	}
	assertPDFExists(t, j.OutputPath)
}

// pdfGState ends and reopens its q's on every clip and transform
// change, and a Q drops the font PDF keeps in its graphics state. That
// is safe only because every PDF draw call sets its font again and
// fpdf's SetFont always writes Tf, even for the font already set. This
// guards the second half against an fpdf upgrade that caches the font.
func TestFPDFSetFontAlwaysWritesTf(t *testing.T) {
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetCompression(false)
	pdf.AddPage()
	pdf.SetFont("Helvetica", "", 12)
	pdf.TransformBegin()
	pdf.TransformEnd()
	pdf.SetFont("Helvetica", "", 12)
	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		t.Fatal(err)
	}
	if n := bytes.Count(buf.Bytes(), []byte(" Tf ET")); n < 2 {
		t.Fatalf("Tf written %d times, want 2: fpdf now skips an unchanged font", n)
	}
}
