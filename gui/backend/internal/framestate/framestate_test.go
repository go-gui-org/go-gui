package framestate

import (
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-gui-org/go-glyph"

	"github.com/go-gui-org/go-gui/gui"
)

// stubBridge records bridge calls for assertions.
type stubBridge struct {
	pipes   []int
	mvps    [][16]float32
	resizes [][2]int32
}

func (s *stubBridge) SetPipeline(pipe int, mvp *[16]float32) {
	s.pipes = append(s.pipes, pipe)
	copy := *mvp
	s.mvps = append(s.mvps, copy)
}

func (s *stubBridge) Resize(physW, physH int32) {
	s.resizes = append(s.resizes, [2]int32{physW, physH})
}

func newTestFS() (*FrameState, *stubBridge) {
	br := &stubBridge{}
	return New(7, 9, br), br
}

func TestSetPipelineDedupe(t *testing.T) {
	fs, br := newTestFS()

	fs.SetPipeline(7)
	fs.SetPipeline(7) // same pipe, no invalidation: skipped
	fs.InvalidatePipelineState()
	fs.SetPipeline(7) // invalidated: re-issued
	fs.SetPipeline(8)
	fs.SetPipeline(8) // same pipe, skipped

	if len(br.pipes) != 3 {
		t.Fatalf("expected 3 bridge calls, got %d: %v",
			len(br.pipes), br.pipes)
	}
	want := []int{7, 7, 8}
	for i, p := range want {
		if br.pipes[i] != p {
			t.Errorf("call %d: pipe = %d, want %d", i, br.pipes[i], p)
		}
		if br.mvps[i] != fs.MVP {
			t.Errorf("call %d: mvp not passed through", i)
		}
	}
	if fs.LastPipe != 8 {
		t.Errorf("LastPipe = %d, want 8", fs.LastPipe)
	}
	if fs.MVPDirty {
		t.Error("MVPDirty should be false after SetPipeline")
	}
}

func TestSetPipelineMVPDirtyBypassesDedupe(t *testing.T) {
	fs, br := newTestFS()
	fs.SetPipeline(7)
	fs.MVPDirty = true
	fs.SetPipeline(7) // same pipe, but dirty: re-issued
	if len(br.pipes) != 2 {
		t.Fatalf("expected 2 bridge calls, got %d: %v",
			len(br.pipes), br.pipes)
	}
	if fs.MVPDirty {
		t.Error("MVPDirty not cleared after SetPipeline")
	}
}

func TestUseGlyphPipeline(t *testing.T) {
	fs, br := newTestFS()
	fs.UseGlyphPipeline()
	if len(br.pipes) != 1 || br.pipes[0] != 9 {
		t.Fatalf("UseGlyphPipeline issued pipe %v, want 9", br.pipes)
	}
}

func TestUpdateProjection(t *testing.T) {
	fs, _ := newTestFS()
	fs.PhysW = 800
	fs.PhysH = 600
	fs.UpdateProjection()

	m := fs.MVP
	if math.Abs(float64(m[0]-2.0/800)) > 1e-6 {
		t.Errorf("m[0] = %v, want %v", m[0], 2.0/800)
	}
	if math.Abs(float64(m[5]+2.0/600)) > 1e-6 {
		t.Errorf("m[5] = %v, want %v", m[5], -2.0/600)
	}
	if m[12] != -1 || m[13] != 1 || m[15] != 1 {
		t.Errorf("translation/scale wrong: m[12..15] = %v", m[12:16])
	}
	if m[10] != -1 {
		t.Errorf("m[10] = %v, want -1", m[10])
	}
}

func TestUpdateProjectionDegenerateDims(t *testing.T) {
	fs, _ := newTestFS()
	// Zero-area viewport (e.g. surface not yet sized) must produce
	// an identity matrix, never NaN/Inf.
	fs.PhysW = 0
	fs.PhysH = 0
	fs.UpdateProjection()
	if fs.MVP != [16]float32{0: 1, 5: 1, 10: 1, 15: 1} {
		t.Errorf("degenerate MVP = %v, want identity", fs.MVP)
	}
}

func TestRotationStackRestores(t *testing.T) {
	fs, br := newTestFS()
	fs.PhysW = 100
	fs.PhysH = 100
	fs.UpdateProjection()
	before := fs.MVP

	fs.BeginRotation(90, 50, 50)
	if len(fs.MVPStack) != 1 {
		t.Fatalf("stack depth = %d, want 1", len(fs.MVPStack))
	}
	if fs.MVP == before {
		t.Error("MVP unchanged after rotation")
	}
	// The dirty flag is consumed by the immediate SetPipeline, so
	// the bridge must have received the rotated MVP with the solid
	// pipe.
	if n := len(br.pipes); n != 1 || br.pipes[n-1] != 7 {
		t.Errorf("BeginRotation issued pipes %v, want [7]", br.pipes)
	}
	if br.mvps[0] == before {
		t.Error("bridge received the unrotated MVP")
	}

	fs.EndRotation()
	if fs.MVP != before {
		t.Error("EndRotation did not restore the original MVP")
	}
	if len(fs.MVPStack) != 0 {
		t.Errorf("stack depth = %d, want 0", len(fs.MVPStack))
	}

	// Empty-stack EndRotation must be a no-op, not a panic.
	fs.EndRotation()
}

func TestRotationStackNested(t *testing.T) {
	fs, _ := newTestFS()
	fs.PhysW = 100
	fs.PhysH = 100
	fs.UpdateProjection()
	before := fs.MVP

	// Nested rotations: inner End restores the outer's matrix, the
	// outer End restores the base matrix (LIFO).
	fs.BeginRotation(90, 50, 50)
	outer := fs.MVP
	fs.BeginRotation(45, 50, 50)
	if len(fs.MVPStack) != 2 {
		t.Fatalf("stack depth = %d, want 2", len(fs.MVPStack))
	}

	fs.EndRotation()
	if fs.MVP != outer {
		t.Error("inner EndRotation did not restore the outer matrix")
	}
	fs.EndRotation()
	if fs.MVP != before {
		t.Error("outer EndRotation did not restore the base matrix")
	}
	if len(fs.MVPStack) != 0 {
		t.Errorf("stack depth = %d, want 0", len(fs.MVPStack))
	}
}

func TestHandleResize(t *testing.T) {
	fs, br := newTestFS()
	fs.HandleResize(400, 300, 2)

	if fs.DPIScale != 2 || fs.PhysW != 800 || fs.PhysH != 600 {
		t.Errorf("dims = %v/%v@%v, want 800/600@2",
			fs.PhysW, fs.PhysH, fs.DPIScale)
	}
	if len(br.resizes) != 1 || br.resizes[0] != [2]int32{800, 600} {
		t.Errorf("bridge resizes = %v, want [[800 600]]", br.resizes)
	}
	if fs.MVP[0] != 2.0/800 || fs.MVP[5] != -2.0/600 {
		t.Errorf("projection not rebuilt on resize: %v", fs.MVP)
	}

	// A later resize with a different scale recomputes dims,
	// notifies the bridge, and rebuilds the projection.
	fs.HandleResize(500, 400, 1)
	if fs.DPIScale != 1 || fs.PhysW != 500 || fs.PhysH != 400 {
		t.Errorf("dims after resize = %v/%v@%v, want 500/400@1",
			fs.PhysW, fs.PhysH, fs.DPIScale)
	}
	if len(br.resizes) != 2 || br.resizes[1] != [2]int32{500, 400} {
		t.Errorf("bridge resizes = %v, want second [[500 400]]", br.resizes)
	}
	if fs.MVP[0] != 2.0/500 || fs.MVP[5] != -2.0/400 {
		t.Errorf("projection not rebuilt after second resize: %v", fs.MVP)
	}
}

func TestFrameBgDefaultsToTheme(t *testing.T) {
	fs, _ := newTestFS()
	w := gui.NewWindow(gui.WindowCfg{})
	w.Config.BgColor = gui.Color{} // explicit zero: unset
	r, g, b, a := fs.FrameBg(w)
	theme := gui.CurrentTheme()
	if r != float32(theme.ColorBackground.R)/255.0 {
		t.Errorf("red = %v, want %v", r, float32(theme.ColorBackground.R)/255.0)
	}
	if g != float32(theme.ColorBackground.G)/255.0 {
		t.Errorf("green = %v, want %v", g, float32(theme.ColorBackground.G)/255.0)
	}
	if b != float32(theme.ColorBackground.B)/255.0 {
		t.Errorf("blue = %v, want %v", b, float32(theme.ColorBackground.B)/255.0)
	}
	if a != float32(theme.ColorBackground.A)/255.0 {
		t.Errorf("alpha = %v, want %v", a, float32(theme.ColorBackground.A)/255.0)
	}
}

func TestFrameBgExplicitColor(t *testing.T) {
	fs, _ := newTestFS()
	w := gui.NewWindow(gui.WindowCfg{})
	w.Config.BgColor = gui.RGB(255, 128, 64)
	r, g, b, a := fs.FrameBg(w)
	// 128/255 and 64/255 do not round-trip through float32
	// exactly; compare with a tolerance.
	if r != 1 || math.Abs(float64(g)-0.5) > 0.003 ||
		math.Abs(float64(b)-0.25) > 0.003 || a != 1 {
		t.Errorf("components = %v/%v/%v/%v, want 1/0.5/0.25/1", r, g, b, a)
	}
}

func TestResolveImagePath(t *testing.T) {
	fs, _ := newTestFS()
	root := t.TempDir()
	fs.AllowedImageRoots = []string{root}

	good := filepath.Join(root, "a.png")
	if err := os.WriteFile(good, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	// ResolveValidatedPath canonicalizes symlinks, so resolve the
	// expected path the same way.
	want, err := filepath.EvalSymlinks(good)
	if err != nil {
		t.Fatal(err)
	}
	if p := fs.ResolveImagePath(good, "test:"); p != want {
		t.Errorf("path = %q, want %q", p, want)
	}

	// A missing file inside a root resolves fine (path validation
	// only); decode failure is handled at texture upload time.
	// ResolveWithParentFallback canonicalizes the parent dir.
	rootReal, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	missing := filepath.Join(root, "nope.png")
	wantMissing := filepath.Join(rootReal, "nope.png")
	if p := fs.ResolveImagePath(missing, "test:"); p != wantMissing {
		t.Errorf("missing path = %q, want %q", p, wantMissing)
	}

	// Paths outside the allowed roots are rejected.
	if p := fs.ResolveImagePath(filepath.Join(t.TempDir(), "x.png"), "test:"); p != "-" {
		t.Errorf("outside-root path = %q, want %q", p, "-")
	}

	// A NUL byte is a validation failure.
	if p := fs.ResolveImagePath("a\x00b.png", "test:"); p != "-" {
		t.Errorf("NUL path = %q, want %q", p, "-")
	}

	// Cache: the sentinel is stored, so a second resolve of the
	// same key must not re-validate (same result, no error).
	if p := fs.ResolveImagePath(filepath.Join(t.TempDir(), "x.png"), "test:"); p != "-" {
		t.Errorf("cached outside-root path = %q, want %q", p, "-")
	}
}

// stubGlyphBackend implements glyph.DrawBackend without any GPU.
type stubGlyphBackend struct{}

func (s *stubGlyphBackend) NewTexture(width, height int) glyph.TextureID  { return 0 }
func (s *stubGlyphBackend) UpdateTexture(id glyph.TextureID, data []byte) {}
func (s *stubGlyphBackend) DeleteTexture(id glyph.TextureID)              {}
func (s *stubGlyphBackend) DrawTexturedQuad(id glyph.TextureID, src, dst glyph.Rect, c glyph.Color) {
}
func (s *stubGlyphBackend) DrawFilledRect(dst glyph.Rect, c glyph.Color) {}
func (s *stubGlyphBackend) DrawTexturedQuadTransformed(
	id glyph.TextureID, src, dst glyph.Rect, c glyph.Color, t glyph.AffineTransform) {
}
func (s *stubGlyphBackend) DPIScale() float32 { return 1 }

func TestFlushText(t *testing.T) {
	fs, br := newTestFS()

	// No text queued: nothing happens, even with a nil system.
	fs.FlushText(nil)
	if len(br.pipes) != 0 {
		t.Fatalf("flush issued pipes with nothing queued: %v", br.pipes)
	}

	// Queued text: commits and clears the flag.
	textSys, err := glyph.NewTextSystem(&stubGlyphBackend{})
	if err != nil {
		t.Fatal(err)
	}
	fs.TextQueued = true
	fs.FlushText(textSys)
	if len(br.pipes) != 1 || br.pipes[0] != 9 {
		t.Errorf("flush issued pipes %v, want [9] (glyph tex)", br.pipes)
	}
	if fs.TextQueued {
		t.Error("TextQueued still set after flush")
	}
}
