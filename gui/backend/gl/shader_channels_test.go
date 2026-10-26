//go:build !js && !darwin && !android && !cgo

package gl

import (
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

// channelSurface is a 2x2 R8 plane of 128 and a 1x1 RG8 plane of
// (64, 192): uniform values, so the readback is the same at any sample
// position.
func channelSurface(t *testing.T) *gui.PixelSurface {
	t.Helper()
	surface, err := gui.NewPixelSurface(gui.PixelSurfaceCfg{Planes: []gui.PixelPlaneCfg{
		{Format: gui.PixelFormatR8, Width: 2, Height: 2},
		{Format: gui.PixelFormatRG8, Width: 1, Height: 1},
	}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(surface.Close)
	if err = surface.UpdatePlanes(
		gui.PixelPlane{Pix: []byte{128, 128, 128, 128}, Stride: 2},
		gui.PixelPlane{Pix: []byte{64, 192}, Stride: 2},
	); err != nil {
		t.Fatal(err)
	}
	return surface
}

// near reports whether got matches want within one step per channel, the
// rounding of an 8-bit value through a float and back.
func near(got, want [4]byte) bool {
	for i := range got {
		d := int(got[i]) - int(want[i])
		if d < -1 || d > 1 {
			return false
		}
	}
	return true
}

func TestShaderChannelsReadback(t *testing.T) {
	b := newBatchProbeBackend(t)
	target := newReadbackTarget(t, b)
	surface := channelSurface(t)
	draw := func(glsl string) [4]byte {
		cmd := gui.RenderCmd{Kind: gui.RenderCustomShader,
			Shader: &gui.Shader{GLSL: glsl, Surface: surface},
			W:      float32(b.physW) / b.dpiScale, H: float32(b.physH) / b.dpiScale}
		pixels := target.render(b, func() { b.drawCustomShader(&cmd) })
		return pixelAt(b, pixels, int(b.physW)/2, int(b.physH)/2)
	}

	// Plane 0 at unit 1 as .r, plane 1 at unit 2 as .rg, and channel 2,
	// which has no plane, reads transparent.
	got := draw(`vec2 p = uv * 0.5 + 0.5;
vec4 frag_color = vec4(texture(iChannel0, p).r, texture(iChannel1, p).rg,
    1.0 - texture(iChannel2, p).a);`)
	if want := [4]byte{128, 64, 192, 255}; !near(got, want) {
		t.Errorf("channel readback = %v, want %v", got, want)
	}

	// iChannelResolution holds each plane's size; the transparent
	// stand-in is 1x1.
	got = draw(`vec4 frag_color = vec4(iChannelResolution[0].x / 255.0,
    iChannelResolution[1].x / 255.0, iChannelResolution[2].x / 255.0, 1.0);`)
	if want := [4]byte{2, 1, 1, 255}; !near(got, want) {
		t.Errorf("iChannelResolution readback = %v, want %v", got, want)
	}

	// A closed surface binds the transparent texture to every channel.
	surface.Close()
	b.channels.PruneShaders(nil)
	got = draw(`vec2 p = uv * 0.5 + 0.5;
vec4 frag_color = vec4(texture(iChannel0, p).r, texture(iChannel1, p).g, 0.0, 1.0);`)
	if want := [4]byte{0, 0, 0, 255}; !near(got, want) {
		t.Errorf("closed surface readback = %v, want %v", got, want)
	}
}

func TestShaderChannelTextureReuse(t *testing.T) {
	b := newBatchProbeBackend(t)
	surface := channelSurface(t)
	first, ok := b.channels.ResolvePlanes(surface, b.uploadChannels)
	if !ok || first.n != 2 || first.tex[0].id == 0 || first.tex[1].id == 0 {
		t.Fatalf("first upload = %+v, %v", first, ok)
	}
	if err := surface.UpdatePlanes(
		gui.PixelPlane{Pix: []byte{1, 2, 3, 4}, Stride: 2},
		gui.PixelPlane{Pix: []byte{5, 6}, Stride: 2},
	); err != nil {
		t.Fatal(err)
	}
	second, _ := b.channels.ResolvePlanes(surface, b.uploadChannels)
	if second.tex != first.tex {
		t.Error("a same-size frame replaced the plane textures")
	}
}

func TestBackendReportsShaders(t *testing.T) {
	w := gui.NewWindow(gui.WindowCfg{State: new(int), Width: 64, Height: 64})
	w.SetView(func(_ *gui.Window) gui.View { return gui.Column(gui.ContainerCfg{}) })
	if w.RendersShaders() {
		t.Fatal("a window reports shaders before a backend exists")
	}
	b, err := New(w)
	if err != nil || b == nil {
		t.Skipf("no GL backend: %v", err)
	}
	t.Cleanup(b.Destroy)
	if !w.RendersShaders() {
		t.Error("the GL backend did not report that it renders shaders")
	}
}
