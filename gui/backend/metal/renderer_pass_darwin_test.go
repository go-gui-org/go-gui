//go:build darwin && arm64 && !ios

package metal

import (
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

// TestFilterTexturesRetryIncomplete: a filter texture that fails to
// allocate must not pin the failure. The next ensure reallocates the
// missing attachments at the same size instead of failing every later
// beginFilter.
func TestFilterTexturesRetryIncomplete(t *testing.T) {
	o := newOffscreen(t, 32)
	o.c.ensureFilterTextures(32, 32)
	if o.c.filterTexA == 0 || o.c.filterTexB == 0 {
		t.Fatal("filter textures did not allocate")
	}
	// Simulate a partial failure at the current size: TexB is lost.
	release(o.c.filterTexB)
	o.c.filterTexB = 0
	o.c.ensureFilterTextures(32, 32)
	if o.c.filterTexB == 0 {
		t.Fatal("incomplete filter textures did not retry")
	}
	o.finish(t)
}

// TestEndFilterWithoutBeginIsNoOp: without a matching beginFilter the
// main encoder is still open, and endFilter must leave it alone.
// Ending it would close the main pass as if it were the filter pass.
func TestEndFilterWithoutBeginIsNoOp(t *testing.T) {
	o := newOffscreen(t, 32)
	o.c.endFilter(0, 1, nil)
	if o.c.enc == 0 {
		t.Fatal("endFilter without begin closed the main encoder")
	}
	o.rect(0, 0, 32, 32, gui.RGB(0, 0, 255))
	px := o.finish(t)
	if r, g, b := rgbAt(px, 32, 8, 8); r != 0 || g != 0 || b != 255 {
		t.Fatalf("pixel = (%d,%d,%d), want (0,0,255)", r, g, b)
	}
}

// TestFilterRestoresScissor: suspending the main pass drops the bound
// scissor. The resumed pass binds the saved rect again, so a rect
// drawn after the filter stays clipped.
func TestFilterRestoresScissor(t *testing.T) {
	o := newOffscreen(t, 32)
	o.c.setScissor(0, 0, 16, 16)
	if rc := o.c.beginFilter(32, 32); rc != 0 {
		t.Fatalf("beginFilter = %d, want 0", rc)
	}
	o.c.endFilter(0, 1, nil)
	// The composite leaves the filter pipeline bound, so draw like the
	// backend wrapper does: restore the pipeline and MVP on resume.
	o.c.setPipeline(pipeSolid)
	var mvp [16]float32
	orthoInto(&mvp, 32, 32)
	o.c.setMVP(&mvp)
	if !o.c.st.scissOK || o.c.st.scissor != [4]uint{0, 0, 16, 16} {
		t.Fatalf("resumed scissor = %v, want [0 0 16 16]", o.c.st.scissor)
	}
	o.rect(0, 0, 32, 32, gui.RGB(0, 0, 255))
	px := o.finish(t)
	if r, g, b := rgbAt(px, 32, 8, 8); r != 0 || g != 0 || b != 255 {
		t.Fatalf("clipped pixel = (%d,%d,%d), want (0,0,255)", r, g, b)
	}
	if r, g, b := rgbAt(px, 32, 24, 24); r != 0 || g != 0 || b != 0 {
		t.Fatalf("unclipped pixel = (%d,%d,%d), want (0,0,0)", r, g, b)
	}
}
