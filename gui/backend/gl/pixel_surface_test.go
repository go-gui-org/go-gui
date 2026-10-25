//go:build !js && !darwin && !android && !cgo

package gl

import (
	"fmt"
	"testing"
	"unsafe"

	"github.com/go-gui-org/go-gui/gui"
	gogl "github.com/go-gui-org/go-gui/gui/backend/internal/glbind"
)

func TestPixelSurfaceTextureReuseAndPixels(t *testing.T) {
	b := newBatchProbeBackend(t)
	target := newReadbackTarget(t, b)
	surface, err := gui.NewPixelSurface(gui.PixelSurfaceCfg{Width: 1, Height: 1})
	if err != nil {
		t.Fatal(err)
	}
	defer surface.Close()
	cmd := gui.RenderCmd{Kind: gui.RenderImage, Surface: surface,
		Resource: "missing-file-must-not-load", W: float32(b.physW) / b.dpiScale,
		H: float32(b.physH) / b.dpiScale, Opacity: 1}
	var textureID uint32
	for _, color := range [][4]byte{{255, 0, 0, 255}, {0, 0, 255, 255}} {
		if err = surface.Update(color[:], 4); err != nil {
			t.Fatal(err)
		}
		pixels := target.render(b, func() { b.drawImage(&cmd) })
		got := pixelAt(b, pixels, int(b.physW)/2, int(b.physH)/2)
		if got != color {
			t.Fatalf("readback = %v, want %v", got, color)
		}
		tex, ok := b.surfaces.Resolve(surface, b.uploadSurfaceTexture)
		if !ok || tex.id == 0 {
			t.Fatal("surface has no texture")
		}
		if textureID != 0 && textureID != tex.id {
			t.Error("fixed-size update replaced GPU texture")
		}
		textureID = tex.id
	}
	if err = surface.Resize(2, 1); err != nil {
		t.Fatal(err)
	}
	if err = surface.Update([]byte{0, 255, 0, 255, 0, 255, 0, 255}, 8); err != nil {
		t.Fatal(err)
	}
	pixels := target.render(b, func() { b.drawImage(&cmd) })
	if got := pixelAt(b, pixels, int(b.physW)/2, int(b.physH)/2); got != [4]byte{0, 255, 0, 255} {
		t.Fatalf("resized readback = %v", got)
	}
	surface.Close()
	b.surfaces.Prune([]gui.RenderCmd{cmd})
	pixels = target.render(b, func() { b.drawImage(&cmd) })
	if got := pixelAt(b, pixels, int(b.physW)/2, int(b.physH)/2); got != [4]byte{0, 0, 0, 255} {
		t.Fatalf("closed surface readback = %v", got)
	}
}

// Upload-only baseline. Reading one texel from the uploaded texture forces the
// transfer to finish without including a draw; results include readback latency.
func BenchmarkPixelSurfaceUpload(b *testing.B) {
	for _, size := range [][2]int{{1280, 720}, {1920, 1080}, {3840, 2160}} {
		b.Run(fmt.Sprintf("%dx%d", size[0], size[1]), func(b *testing.B) {
			window := gui.NewWindow(gui.WindowCfg{Width: 64, Height: 64})
			native, err := New(window)
			if err != nil {
				b.Skipf("OpenGL unavailable: %v", err)
			}
			b.Cleanup(native.Destroy)
			pix := make([]byte, size[0]*size[1]*4)
			tex := native.uploadSurfaceTexture(glTexture{}, size[0], size[1], pix)
			b.Cleanup(func() { native.deleteSurfaceTexture(tex) })
			var framebuffer uint32
			gogl.GenFramebuffers(1, &framebuffer)
			gogl.BindFramebuffer(gogl.FRAMEBUFFER, framebuffer)
			gogl.FramebufferTexture2D(gogl.FRAMEBUFFER, gogl.COLOR_ATTACHMENT0,
				gogl.TEXTURE_2D, tex.id, 0)
			if status := gogl.CheckFramebufferStatus(gogl.FRAMEBUFFER); status != gogl.FRAMEBUFFER_COMPLETE {
				b.Fatalf("readback framebuffer incomplete: 0x%x", status)
			}
			b.Cleanup(func() { gogl.DeleteFramebuffers(1, &framebuffer) })
			var pixel [4]byte
			b.SetBytes(int64(len(pix)))
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				native.uploadSurfaceTexture(tex, size[0], size[1], pix)
				gogl.ReadPixels(0, 0, 1, 1, gogl.RGBA, gogl.UNSIGNED_BYTE, unsafe.Pointer(&pixel[0]))
			}
		})
	}
}
