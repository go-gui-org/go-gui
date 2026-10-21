// Package ctxloss rebuilds a backend's GPU resources after the context
// that held them is lost. The Android backend uses it: GLSurfaceView
// destroys its EGL context while the app is in the background, and a
// resume runs in a new one.
//
// The package holds no GL code, so its order and retry rules are tested
// on the host. The android package is built for Android only, and its
// tests do not run in CI.
package ctxloss

import (
	"image"

	"github.com/go-gui-org/go-glyph"
)

// Steps are the parts of one rebuild, called in this order: Forget,
// Init, then Refill. Undo runs in place of Refill when Init fails.
type Steps struct {
	// Forget drops every name from the lost context and deletes none:
	// in the new context an old name means nothing, or names an object
	// made since.
	Forget func()
	// Init builds the new context's base resources: programs, buffers.
	Init func() error
	// Undo deletes what a failed Init built. Those names are in the
	// current context, so a delete is safe. Forget runs again on the
	// next try.
	Undo func()
	// Refill makes what depends on Init: new textures for the glyph
	// atlas, its pixels sent again, reset pipeline state.
	Refill func()
}

// Rebuilder tracks whether a rebuild is due. The zero value has none
// due.
type Rebuilder struct {
	pending bool
	// failed is set when a rebuild for the current loss has failed, so
	// a retry each frame reports the error once.
	failed bool
}

// MarkLost records that the context was lost and a rebuild is due.
func (r *Rebuilder) MarkLost() {
	r.pending = true
	r.failed = false
}

// Pending reports whether a rebuild is due.
func (r *Rebuilder) Pending() bool { return r.pending }

// Run does the rebuild when one is due. When Init fails, Run calls Undo
// and the rebuild stays due, so the caller can try again on the next
// frame. A failed Init does not panic: the app and its state outlive a
// frame with nothing drawn. Run returns the Init error on the first
// failure for a loss and nil on later ones, so a caller that logs it
// logs once.
func (r *Rebuilder) Run(s Steps) error {
	if !r.pending {
		return nil
	}
	s.Forget()
	if err := s.Init(); err != nil {
		s.Undo()
		if r.failed {
			return nil
		}
		r.failed = true
		return err
	}
	s.Refill()
	r.pending = false
	r.failed = false
	return nil
}

// ReuploadAtlas sends every atlas page to its texture at once. Call it
// after the page textures were made again in a new context. The atlas
// keeps each page's pixels on the CPU (StagingBack).
//
// The upload must not wait for the end of the frame: GL samples a
// texture when the draw is issued, so the first frame's text would read
// the new, empty textures and draw blank.
func ReuploadAtlas(atlas *glyph.GlyphAtlas) {
	if atlas == nil {
		return
	}
	for i := range atlas.Pages {
		atlas.Pages[i].Dirty = true
		// An empty rect with Dirty set is the whole page.
		atlas.Pages[i].DirtyRect = image.Rectangle{}
	}
	atlas.SwapAndUpload()
}
