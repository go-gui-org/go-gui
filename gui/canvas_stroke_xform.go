package gui

import "math"

// canvas_stroke_xform.go — the Styled record paths for DrawContext
// (issue #905).
//
// Split out of canvas_draw_transform.go, which owns the recorder
// decorator and the unwrap helpers: both files sit under the same
// 800-line gate as canvas_draw.go, and the eight Styled forwarders
// would push either one past it. What lives here are the
// DrawStrokeRecorder methods on xformRecorder and the strokeRecorder
// unwrap helper they depend on.
//
// Each forwarder bakes coordinates exactly like its legacy twin and
// passes the style through untouched: a style holds no lengths —
// the miter limit is fixed — so there is nothing to scale. Each
// reaches the inner recorder through the extension, which holds
// because callers only arrive here after strokeRecorder reported
// true. A rotated arc or circle lowers to PolylineJoinedStyled rather
// than PolylineJoined so the style survives the lowering.

// strokeRecorder is gradientRecorder for DrawStrokeRecorder, and
// asserts against the inner recorder for the same reason: asserting
// against the decorator would always succeed — the decorator
// implements every extension — and that would quietly kill the
// unstyled-primitive degradation for a recorder without it.
func (dc *DrawContext) strokeRecorder() (DrawStrokeRecorder, bool) {
	if dc.recorder == nil {
		return nil, false
	}
	if _, ok := dc.recorder.(DrawStrokeRecorder); !ok {
		return nil, false
	}
	if _, ok := dc.activeXform(); !ok {
		return dc.recorder.(DrawStrokeRecorder), true
	}
	return dc.rec().(*xformRecorder), true
}

func (r *xformRecorder) LineStyled(x0, y0, x1, y1 float32, color Color,
	width float32, style StrokeStyle) {
	ax, ay := r.pt(x0, y0)
	bx, by := r.pt(x1, y1)
	r.inner.(DrawStrokeRecorder).LineStyled(ax, ay, bx, by, color,
		r.ln(width), style)
}

func (r *xformRecorder) PolylineStyled(points []float32, color Color,
	width float32, style StrokeStyle) {
	r.inner.(DrawStrokeRecorder).PolylineStyled(r.pts(points), color,
		r.ln(width), style)
}

func (r *xformRecorder) PolylineJoinedStyled(points []float32, color Color,
	width float32, style StrokeStyle) {
	r.inner.(DrawStrokeRecorder).PolylineJoinedStyled(r.pts(points), color,
		r.ln(width), style)
}

func (r *xformRecorder) ArcStyled(cx, cy, rx, ry, start, sweep float32,
	color Color, width float32, style StrokeStyle) {
	if r.xf.rotated() {
		pts := r.dc.arcPoints(cx, cy, rx, ry, start, sweep)
		if len(pts) < 2 {
			return
		}
		r.inner.(DrawStrokeRecorder).PolylineJoinedStyled(r.pts(pts),
			color, r.ln(width), style)
		return
	}
	x, y := r.pt(cx, cy)
	r.inner.(DrawStrokeRecorder).ArcStyled(x, y, rx*r.xf.colScaleX(),
		ry*r.xf.colScaleY(), start, sweep, color, r.ln(width), style)
}

func (r *xformRecorder) CircleStyled(cx, cy, radius float32, color Color,
	width float32, style StrokeStyle) {
	x, y := r.pt(cx, cy)
	styled := r.inner.(DrawStrokeRecorder)
	if r.xf.uniform() {
		styled.CircleStyled(x, y, radius*r.xf.colScaleX(), color,
			r.ln(width), style)
		return
	}
	if !r.xf.rotated() {
		styled.ArcStyled(x, y, radius*r.xf.colScaleX(),
			radius*r.xf.colScaleY(), 0, 2*math.Pi, color,
			r.ln(width), style)
		return
	}
	if pts := r.dc.arcPoints(cx, cy, radius, radius, 0, 2*math.Pi); len(pts) >= 2 {
		styled.PolylineJoinedStyled(r.closedLoop(pts), color,
			r.ln(width), style)
	}
}

func (r *xformRecorder) RoundedRectStyled(x, y, w, h, radius float32,
	color Color, width float32, style StrokeStyle) {
	if r.xf.rotated() {
		if pts := r.roundedOutline(x, y, w, h, radius); len(pts) >= 4 {
			r.inner.(DrawStrokeRecorder).PolylineJoinedStyled(
				r.closedLoop(pts), color, r.ln(width), style)
		}
		return
	}
	nx, ny, nw, nh := r.dc.xfRect(x, y, w, h)
	r.inner.(DrawStrokeRecorder).RoundedRectStyled(nx, ny, nw, nh,
		r.ln(radius), color, r.ln(width), style)
}

func (r *xformRecorder) QuadBezierStyled(x0, y0, cx, cy, x1, y1 float32,
	color Color, width float32, style StrokeStyle) {
	ax, ay := r.pt(x0, y0)
	bx, by := r.pt(cx, cy)
	ex, ey := r.pt(x1, y1)
	r.inner.(DrawStrokeRecorder).QuadBezierStyled(ax, ay, bx, by, ex, ey,
		color, r.ln(width), style)
}

func (r *xformRecorder) CubicBezierStyled(x0, y0, c1x, c1y, c2x, c2y,
	x1, y1 float32, color Color, width float32, style StrokeStyle) {
	ax, ay := r.pt(x0, y0)
	bx, by := r.pt(c1x, c1y)
	cx2, cy2 := r.pt(c2x, c2y)
	ex, ey := r.pt(x1, y1)
	r.inner.(DrawStrokeRecorder).CubicBezierStyled(ax, ay, bx, by, cx2, cy2,
		ex, ey, color, r.ln(width), style)
}
