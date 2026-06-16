package gui

// DispatchCloseRequest routes an OS window-close event. If the
// window has a non-nil OnCloseRequest hook, it is invoked and the
// hook owns calling Window.Close() to proceed. Otherwise the window
// is marked for destruction via the closeReq loop.
//
// Intended for backend use. Safe for a nil window (no-op).
func DispatchCloseRequest(w *Window) {
	if w == nil {
		return
	}
	if cb := w.Config.OnCloseRequest; cb != nil {
		cb(w)
		return
	}
	w.Close()
}

// DispatchQuitRequest routes an OS app-quit event across all
// registered windows. Each window with a non-nil OnCloseRequest
// hook is invoked (vetoed=true is returned if any such hook fires).
// Windows without a hook are marked for destruction immediately.
//
// The caller keeps the outer loop running when vetoed is true;
// any hook that ends up calling Window.Close() is picked up by the
// closeReq teardown pass on the next iteration.
//
// Intended for backend use. Safe for a nil app (returns false).
func DispatchQuitRequest(app *App) (vetoed bool) {
	if app == nil {
		return false
	}
	for _, w := range app.Windows() {
		if w.DialogIsVisible() {
			vetoed = true
			continue
		}
		if cb := w.Config.OnCloseRequest; cb != nil {
			cb(w)
			vetoed = true
			continue
		}
		w.Close()
	}
	return vetoed
}
