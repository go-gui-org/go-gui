package gui

// LocalToWindow maps a point in a widget's own box to the window, where
// the widget is drawn. x and y are relative to the box's top-left corner,
// as e.MouseX and e.MouseY are in OnClick. Every view transform and
// quarter turn above the widget, and its own, is applied (#1019), so an
// app can place an overlay, a popover or a drag ghost by a moved or
// turned view.
//
// effectiveID is the widget's effective ID, as FindByID takes it. ok is
// false when the last laid-out frame has no widget with that ID, when x
// or y is not finite, or when the point has no place on screen: a hidden
// back face, or a perspective ray behind the viewer. The ancestors' clips are not tested: a point a
// clip cuts away still maps.
//
// It reads the last laid-out frame. A transform that TransformFn changed
// in a render-only frame is read as that frame drew it.
// exportaudit:keep — app API for placing overlays by transformed views (#1019)
func (w *Window) LocalToWindow(effectiveID string, x, y float32) (wx, wy float32, ok bool) {
	ly, found := w.coordsLayout("LocalToWindow", effectiveID)
	if !found || !f32IsFinite(x) || !f32IsFinite(y) {
		return 0, 0, false
	}
	return xfLocalToWindowClip(ly, ly.Shape.X+x, ly.Shape.Y+y, false)
}

// WindowToLocal is the inverse of LocalToWindow: it maps a window point
// into a widget's own box, relative to the box's top-left corner. ok is
// false when there is no widget with that ID, when x or y is not finite,
// or when the point is on no drawn part of the views above it (a hidden
// back face, a view flattened to a line, a ray that meets the view's
// plane behind the viewer).
// exportaudit:keep — app API for placing overlays by transformed views (#1019)
func (w *Window) WindowToLocal(effectiveID string, x, y float32) (lx, ly float32, ok bool) {
	l, found := w.coordsLayout("WindowToLocal", effectiveID)
	if !found || !f32IsFinite(x) || !f32IsFinite(y) {
		return 0, 0, false
	}
	fx, fy, mapped := xfWindowToLocal(l, x, y, false)
	if !mapped {
		return 0, 0, false
	}
	return fx - l.Shape.X, fy - l.Shape.Y, true
}

// coordsLayout finds the widget a coordinate query names, and reports a
// near-miss spelling through DebugUnknownLookup like the other ID APIs.
func (w *Window) coordsLayout(api, effectiveID string) (*Layout, bool) {
	if w == nil || w.layout.Shape == nil {
		return nil, false
	}
	ly, ok := w.layout.findByID(effectiveID)
	if !ok || ly.Shape == nil {
		debugLookupMiss(&w.layout, api, effectiveID)
		return nil, false
	}
	return ly, true
}
