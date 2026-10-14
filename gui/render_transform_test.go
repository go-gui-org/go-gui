package gui

import (
	"math"
	"testing"
)

// xfShape returns a filled rectangle at (x, y) with size w x h that
// carries tr as its transform.
func xfShape(x, y, w, h float32, tr Transform) *Shape {
	return &Shape{
		shapeType: shapeRectangle,
		Color:     RGB(100, 100, 100),
		X:         x, Y: y,
		Width: w, Height: h,
		Opacity: 1,
		xf:      &shapeTransform{static: tr},
	}
}

// xfKinds returns the kinds of the emitted commands, in order.
func xfKinds(w *Window) []renderKind {
	out := make([]renderKind, 0, len(w.renderers))
	for _, r := range w.renderers {
		out = append(out, r.Kind)
	}
	return out
}

func TestContainerTransformReachesShape(t *testing.T) {
	w := makeWindow()
	s := Row(ContainerCfg{Transform: Rotate(10)}).GenerateLayout(w).Shape
	if s.xf == nil || s.xf.static != Rotate(10) {
		t.Fatalf("Transform not on shape: %+v", s.xf)
	}
	// An identity transform costs nothing: no pointer is set.
	if Row(ContainerCfg{}).GenerateLayout(w).Shape.xf != nil {
		t.Fatal("identity transform must leave xf nil")
	}
	fn := func() Transform { return Scale(2) }
	if Row(ContainerCfg{TransformFn: fn}).GenerateLayout(w).Shape.xf == nil {
		t.Fatal("TransformFn must set xf")
	}
}

func TestRenderTransformBracketWrapsShapeAndChildren(t *testing.T) {
	w := makeWindow()
	child := Layout{Shape: &Shape{
		shapeType: shapeRectangle, Color: RGB(1, 2, 3), Opacity: 1,
		X: 20, Y: 20, Width: 10, Height: 10,
	}}
	layout := &Layout{
		Shape:    xfShape(10, 10, 60, 40, Rotate(30)),
		Children: []Layout{child},
	}
	renderLayout(layout, ColorTransparent, makeClip(0, 0, 200, 200), w)

	kinds := xfKinds(w)
	if len(kinds) != 4 || kinds[0] != RenderTransformBegin ||
		kinds[1] != RenderRect || kinds[2] != RenderRect ||
		kinds[3] != RenderTransformEnd {
		t.Fatalf("kinds = %v, want [TransformBegin Rect Rect TransformEnd]", kinds)
	}
	begin := w.renderers[0]
	if begin.Homography == nil {
		t.Fatal("TransformBegin carries no matrix")
	}
	want, _ := Rotate(30).resolve(10, 10, 60, 40)
	if *begin.Homography != [9]float32(want) {
		t.Fatalf("matrix = %v, want %v", *begin.Homography, want)
	}
}

func TestRenderTransformIdentityEmitsNoBracket(t *testing.T) {
	w := makeWindow()
	fn := func() Transform { return Transform{} }
	s := xfShape(10, 10, 60, 40, Transform{})
	s.xf.fn = fn
	renderLayout(&Layout{Shape: s}, ColorTransparent, makeClip(0, 0, 200, 200), w)
	for _, k := range xfKinds(w) {
		if k == RenderTransformBegin || k == RenderTransformEnd {
			t.Fatalf("identity emitted a bracket: %v", xfKinds(w))
		}
	}
}

func TestRenderTransformFnWinsAndIsReadEachPass(t *testing.T) {
	w := makeWindow()
	dx := float32(5)
	s := xfShape(0, 0, 10, 10, Rotate(45))
	s.xf.fn = func() Transform { return Translate(dx, 0) }
	layout := &Layout{Shape: s}
	clip := makeClip(0, 0, 200, 200)

	renderLayout(layout, ColorTransparent, clip, w)
	if got := w.renderers[0].Homography[2]; got != 5 {
		t.Fatalf("tx = %v, want 5 (TransformFn must win over Transform)", got)
	}
	// A render-only frame replays the same layout. The function is read
	// again, so a spring moves the view with no rebuild.
	dx = 9
	w.renderers = w.renderers[:0]
	w.renderOnlyPass = true
	renderLayout(layout, ColorTransparent, clip, w)
	w.renderOnlyPass = false
	if got := w.renderers[0].Homography[2]; got != 9 {
		t.Fatalf("tx = %v, want 9 after the value changed", got)
	}
	// Input reads the matrix the render pass stored.
	if s.xf.m[2] != 9 {
		t.Fatalf("stored matrix tx = %v, want 9", s.xf.m[2])
	}
}

func TestRenderTransformDoesNotCullMovedIntoView(t *testing.T) {
	// The box sits outside the clip in its own frame, but the translate
	// moves it into view. Culling must not drop it.
	w := makeWindow()
	s := xfShape(400, 10, 50, 50, Translate(-300, 0))
	renderLayout(&Layout{Shape: s}, ColorTransparent, makeClip(0, 0, 200, 200), w)
	found := false
	for _, r := range w.renderers {
		if r.Kind == RenderRect {
			found = true
		}
	}
	if !found {
		t.Fatalf("translated box was culled: %v", xfKinds(w))
	}
}

func TestRenderTransformClipMappedUnderTranslate(t *testing.T) {
	// A clipping child inside a translated parent: its scissor is a
	// window rect, so it must move by the translate.
	w := makeWindow()
	inner := &Shape{
		shapeType: shapeRectangle, Opacity: 1, Clip: true,
		X: 20, Y: 20, Width: 30, Height: 30,
	}
	inner.shapeClip = shapeBounds(inner)
	layout := &Layout{
		Shape:    xfShape(0, 0, 100, 100, Translate(40, 5)),
		Children: []Layout{{Shape: inner}},
	}
	renderLayout(layout, ColorTransparent, makeClip(0, 0, 200, 200), w)
	var clips []RenderCmd
	for _, r := range w.renderers {
		if r.Kind == RenderClip {
			clips = append(clips, r)
		}
	}
	if len(clips) != 2 {
		t.Fatalf("got %d clips, want 2 (set + restore)", len(clips))
	}
	if c := clips[0]; c.X != 60 || c.Y != 25 || c.W != 30 || c.H != 30 {
		t.Fatalf("clip = (%v,%v %vx%v), want (60,25 30x30)", c.X, c.Y, c.W, c.H)
	}
	// The restore returns to the scissor in effect when the transform
	// opened, not to a mapped copy of an unbounded rect.
	if c := clips[1]; c.X != 0 || c.Y != 0 || c.W != 200 || c.H != 200 {
		t.Fatalf("restore = (%v,%v %vx%v), want (0,0 200x200)", c.X, c.Y, c.W, c.H)
	}
}

func TestRenderTransformClipUnderRotateUsesStencil(t *testing.T) {
	// Under a rotation a scissor cannot follow the turned box. The
	// scissor becomes the bounding box, and a stencil, which goes
	// through the transform, clips exactly.
	w := makeWindow()
	inner := &Shape{
		shapeType: shapeRectangle, Opacity: 1, Clip: true,
		X: 0, Y: 0, Width: 100, Height: 100,
	}
	inner.shapeClip = shapeBounds(inner)
	layout := &Layout{
		Shape:    xfShape(0, 0, 100, 100, Rotate(45)),
		Children: []Layout{{Shape: inner}},
	}
	renderLayout(layout, ColorTransparent, makeClip(0, 0, 400, 400), w)
	var clip *RenderCmd
	begins, ends := 0, 0
	for i := range w.renderers {
		r := &w.renderers[i]
		switch r.Kind {
		case RenderClip:
			if clip == nil {
				clip = r
			}
		case RenderStencilBegin:
			begins++
		case RenderStencilEnd:
			ends++
		}
	}
	if begins != 1 || ends != 1 {
		t.Fatalf("stencil begins=%d ends=%d, want 1 each", begins, ends)
	}
	// A 100x100 box turned 45° about (50,50) spans 50 ± 70.7.
	if clip == nil || !xfNear(clip.X, 0) || !xfNear(clip.Y, 0) ||
		!xfNear(clip.W, 50+70.7107) || !xfNear(clip.H, 50+70.7107) {
		t.Fatalf("clip = %+v, want the box's bounds cut at the window (0,0)", clip)
	}
	if w.stencilDepth != 0 {
		t.Fatalf("stencilDepth = %d after render, want 0", w.stencilDepth)
	}
}

func TestRenderTransformStackBalanced(t *testing.T) {
	w := makeWindow()
	layout := &Layout{
		Shape: xfShape(0, 0, 100, 100, Rotate(10)),
		Children: []Layout{{
			Shape: xfShape(10, 10, 20, 20, Scale(2)),
		}},
	}
	renderLayout(layout, ColorTransparent, makeClip(0, 0, 200, 200), w)
	if len(w.xfStack) != 0 {
		t.Fatalf("xfStack len = %d after render, want 0", len(w.xfStack))
	}
	b, e := 0, 0
	for _, k := range xfKinds(w) {
		switch k {
		case RenderTransformBegin:
			b++
		case RenderTransformEnd:
			e++
		}
	}
	if b != 2 || e != 2 {
		t.Fatalf("begins=%d ends=%d, want 2 each", b, e)
	}
}

func TestLayoutShapeClipTransformedStillCutByAncestor(t *testing.T) {
	// A transformed shape keeps the clip its ancestors pass, in the
	// parent's frame, and the pointer is tested against it before it is
	// mapped in. Without that, a card scrolled out of a viewport would
	// take clicks over the header above it.
	parent := &Shape{
		shapeType: shapeRectangle, Opacity: 1, Clip: true,
		X: 0, Y: 0, Width: 200, Height: 100,
	}
	layout := &Layout{
		Shape:    parent,
		Children: []Layout{{Shape: xfShape(0, 150, 100, 100, Rotate(20))}},
	}
	child := layout.Children[0].Shape
	child.xf.update(child, makeWindow())
	layoutSetShapeClips(layout, makeClip(0, 0, 400, 400))
	if x, y := xfCoordsInverse(child, 50, 200); child.PointInShape(x, y) {
		t.Fatal("transformed child outside its clipping parent took the pointer")
	}
}

func TestRenderTransformClipNestsInsideTransform(t *testing.T) {
	// Inside a transform the layout clip is in the wrong frame, so an
	// inner clipping container is cut by the outer one's content box, as
	// the render walk passes it, not by its layout clip.
	w := makeWindow()
	inner := &Shape{
		shapeType: shapeRectangle, Opacity: 1, Clip: true,
		X: 50, Y: 0, Width: 100, Height: 50,
	}
	// A layout clip from the window frame that would hide it.
	inner.shapeClip = drawClip{}
	outer := xfShape(0, 0, 100, 100, Translate(10, 0))
	outer.Clip = true
	layout := &Layout{Shape: outer, Children: []Layout{{Shape: inner}}}
	renderLayout(layout, ColorTransparent, makeClip(0, 0, 400, 400), w)
	var clips []RenderCmd
	for _, r := range w.renderers {
		if r.Kind == RenderClip {
			clips = append(clips, r)
		}
	}
	// outer set, inner set, inner restore, outer restore.
	if len(clips) != 4 {
		t.Fatalf("got %d clips, want 4", len(clips))
	}
	// Inner: x 50..150 cut to the outer box 0..100, moved by 10.
	if c := clips[1]; c.X != 60 || c.W != 50 || c.Y != 0 || c.H != 50 {
		t.Fatalf("inner clip = (%v,%v %vx%v), want (60,0 50x50)", c.X, c.Y, c.W, c.H)
	}
}

func TestRenderTransformBackfaceHiddenDrawsNothing(t *testing.T) {
	w := makeWindow()
	layout := &Layout{
		Shape: xfShape(0, 0, 100, 100, RotateY(180).BackfaceHidden()),
		Children: []Layout{{Shape: &Shape{
			shapeType: shapeRectangle, Color: RGB(1, 2, 3), Opacity: 1,
			X: 10, Y: 10, Width: 10, Height: 10,
		}}},
	}
	renderLayout(layout, ColorTransparent, makeClip(0, 0, 200, 200), w)
	if len(w.renderers) != 0 {
		t.Fatalf("hidden back face emitted %v", xfKinds(w))
	}
	// The front face of the same card draws.
	layout.Shape.xf.static = RotateY(0).BackfaceHidden()
	renderLayout(layout, ColorTransparent, makeClip(0, 0, 200, 200), w)
	if len(w.renderers) == 0 {
		t.Fatal("front face drew nothing")
	}
}

func TestRenderTransformInvalidMatrixDropsBracket(t *testing.T) {
	// A non-finite position makes a non-finite matrix. The bracket is
	// dropped whole, never half-opened.
	w := makeWindow()
	s := xfShape(0, 0, 10, 10, Rotate(30))
	s.X = float32(math.Inf(1))
	renderLayout(&Layout{Shape: s}, ColorTransparent, makeClip(0, 0, 200, 200), w)
	for _, k := range xfKinds(w) {
		if k == RenderTransformBegin || k == RenderTransformEnd {
			t.Fatalf("invalid matrix emitted a bracket: %v", xfKinds(w))
		}
	}
}
