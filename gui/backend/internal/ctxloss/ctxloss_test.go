package ctxloss

import (
	"errors"
	"image"
	"slices"
	"testing"

	"github.com/go-gui-org/go-glyph"
)

// recorder returns Steps that append each call's name to calls. initErr
// is what Init returns.
func recorder(calls *[]string, initErr *error) Steps {
	return Steps{
		Forget: func() { *calls = append(*calls, "forget") },
		Init: func() error {
			*calls = append(*calls, "init")
			return *initErr
		},
		Undo:   func() { *calls = append(*calls, "undo") },
		Refill: func() { *calls = append(*calls, "refill") },
	}
}

func TestRunDoesNothingWithoutLoss(t *testing.T) {
	var calls []string
	var initErr error
	var r Rebuilder
	if err := r.Run(recorder(&calls, &initErr)); err != nil {
		t.Fatal(err)
	}
	if len(calls) != 0 {
		t.Fatalf("calls = %v, want none", calls)
	}
}

// Old names are forgotten before Init makes new ones, and the textures
// that depend on Init come after it. Any other order leaves glyph IDs on
// zeroed slots or deletes live objects.
func TestRunOrder(t *testing.T) {
	var calls []string
	var initErr error
	var r Rebuilder
	r.MarkLost()
	if err := r.Run(recorder(&calls, &initErr)); err != nil {
		t.Fatal(err)
	}
	if want := []string{"forget", "init", "refill"}; !slices.Equal(calls, want) {
		t.Fatalf("calls = %v, want %v", calls, want)
	}
	if r.Pending() {
		t.Fatal("rebuild still pending after success")
	}
	// Done: a second Run does nothing.
	calls = nil
	_ = r.Run(recorder(&calls, &initErr))
	if len(calls) != 0 {
		t.Fatalf("second Run calls = %v, want none", calls)
	}
}

// A failed Init does not panic. It undoes its partial work, stays due,
// reports once, and the next Run tries again from Forget.
func TestRunRetriesAfterInitFailure(t *testing.T) {
	var calls []string
	initErr := errors.New("compile failed")
	var r Rebuilder
	r.MarkLost()
	if err := r.Run(recorder(&calls, &initErr)); err == nil {
		t.Fatal("first failure returned nil")
	}
	if want := []string{"forget", "init", "undo"}; !slices.Equal(calls, want) {
		t.Fatalf("calls = %v, want %v", calls, want)
	}
	if !r.Pending() {
		t.Fatal("rebuild not pending after failure")
	}
	if err := r.Run(recorder(&calls, &initErr)); err != nil {
		t.Fatalf("second failure returned %v, want nil (logged once)", err)
	}

	calls = nil
	initErr = nil
	if err := r.Run(recorder(&calls, &initErr)); err != nil {
		t.Fatal(err)
	}
	if want := []string{"forget", "init", "refill"}; !slices.Equal(calls, want) {
		t.Fatalf("retry calls = %v, want %v", calls, want)
	}
	if r.Pending() {
		t.Fatal("rebuild still pending after retry succeeded")
	}
}

// A new loss after a failed one reports its own first failure.
func TestMarkLostResetsReport(t *testing.T) {
	var calls []string
	initErr := errors.New("compile failed")
	var r Rebuilder
	r.MarkLost()
	_ = r.Run(recorder(&calls, &initErr))
	r.MarkLost()
	if err := r.Run(recorder(&calls, &initErr)); err == nil {
		t.Fatal("failure after a new loss returned nil")
	}
}

// fakeBackend counts texture uploads per texture.
type fakeBackend struct {
	next    glyph.TextureID
	uploads map[glyph.TextureID]int
}

func (f *fakeBackend) NewTexture(int, int) glyph.TextureID {
	f.next++
	return f.next
}

func (f *fakeBackend) UpdateTexture(id glyph.TextureID, _ []byte) {
	f.uploads[id]++
}

func (f *fakeBackend) DeleteTexture(glyph.TextureID)                                         {}
func (f *fakeBackend) DrawTexturedQuad(glyph.TextureID, glyph.Rect, glyph.Rect, glyph.Color) {}
func (f *fakeBackend) DrawFilledRect(glyph.Rect, glyph.Color)                                {}
func (f *fakeBackend) DrawTexturedQuadTransformed(glyph.TextureID, glyph.Rect, glyph.Rect,
	glyph.Color, glyph.AffineTransform) {
}
func (f *fakeBackend) DPIScale() float32 { return 1 }

// ReuploadAtlas sends every page now, whole, even a clean page or one
// whose dirty rect covers part of it: the new texture holds nothing.
func TestReuploadAtlasSendsEveryPageNow(t *testing.T) {
	fb := &fakeBackend{uploads: map[glyph.TextureID]int{}}
	atlas, err := glyph.NewGlyphAtlas(fb, 64, 64)
	if err != nil {
		t.Fatal(err)
	}
	page := &atlas.Pages[0]
	page.Dirty = true
	page.DirtyRect = image.Rect(0, 0, 4, 4)
	ReuploadAtlas(atlas)
	if got := fb.uploads[page.TextureID]; got != 1 {
		t.Fatalf("uploads = %d, want 1", got)
	}
	if page.Dirty {
		t.Fatal("page still dirty after the upload")
	}

	// A clean page is sent too.
	ReuploadAtlas(atlas)
	if got := fb.uploads[page.TextureID]; got != 2 {
		t.Fatalf("uploads of a clean page = %d, want 2", got)
	}
}

func TestReuploadAtlasNil(t *testing.T) {
	ReuploadAtlas(nil)
}
