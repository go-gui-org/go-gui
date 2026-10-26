package gui

import (
	"strings"
	"testing"
)

func TestGLSLChannelBuildersAddOnlyTheChannelParts(t *testing.T) {
	const decls = "float f() { return 1.0; }"
	const body = "vec4 frag_color = texture(iChannel0, uv * 0.5 + 0.5);"
	for _, tc := range []struct {
		name            string
		plain, channels string
	}{
		{"desktop", BuildGLSLFragmentDecls(decls, body), BuildGLSLFragmentChannels(decls, body)},
		{"es", BuildGLSLESFragmentDecls(decls, body), BuildGLSLESFragmentChannels(decls, body)},
	} {
		for _, want := range []string{
			"uniform sampler2D iChannel0;", "uniform sampler2D iChannel3;",
			"vec3 iChannelResolution[4];",
			"iChannelResolution[0] = vec3(vec2(textureSize(iChannel0, 0)), 1.0);",
		} {
			if !strings.Contains(tc.channels, want) {
				t.Errorf("%s: channel source lacks %q", tc.name, want)
			}
		}
		if strings.Contains(tc.plain, "sampler2D iChannel") || strings.Contains(tc.plain, "iChannelResolution") {
			t.Errorf("%s: plain source declares channels", tc.name)
		}
		// The channel variant is the plain source plus the two channel
		// parts and nothing else, so a shader without a Surface compiles
		// exactly what it did before channels existed.
		stripped := strings.Replace(tc.channels, glslChannelDecls, "", 1)
		stripped = strings.Replace(stripped, glslChannelInit, "", 1)
		if stripped != tc.plain {
			t.Errorf("%s: channel source differs from the plain one beyond the channel parts", tc.name)
		}
		// Declarations come before decls, so a helper can read them, and
		// the init runs in main before the body.
		ch := tc.channels
		ordered := strings.Index(ch, "uniform sampler2D iChannel0;") < strings.Index(ch, decls) &&
			strings.Index(ch, "void main()") < strings.Index(ch, "iChannelResolution[0] =") &&
			strings.Index(ch, "iChannelResolution[0] =") < strings.Index(ch, body)
		if !ordered {
			t.Errorf("%s: channel parts out of order", tc.name)
		}
	}
}

func TestShaderHashSeparatesSurface(t *testing.T) {
	plain := &Shader{Metal: "m", GLSL: "g"}
	first := &Shader{Metal: "m", GLSL: "g", Surface: testPixelSurface(t, 1, 1)}
	second := &Shader{Metal: "m", GLSL: "g", Surface: testPixelSurface(t, 2, 2)}
	if ShaderHash(plain) == ShaderHash(first) {
		t.Error("a shader with a Surface shares the pipeline of the same source without one")
	}
	if ShaderHash(first) != ShaderHash(second) {
		t.Error("the surface itself changed the hash; only its presence builds a different wrapper")
	}
	// The flag must not be reproducible from source: decls ending in the
	// byte a byte-serial flag would add hash differently from the same
	// shader with a Surface.
	crafted := &Shader{Metal: "m", GLSL: "g", GLSLDecls: "x\x01"}
	withSurface := &Shader{Metal: "m", GLSL: "g", GLSLDecls: "x", Surface: first.Surface}
	if ShaderHash(crafted) == ShaderHash(withSurface) {
		t.Error("source bytes reproduce the Surface flag")
	}
	// A plain shader keeps the hash it had before channels existed.
	if ShaderHash(plain) != Fnv64Str(Fnv64Byte(Fnv64Str(Fnv64Byte(Fnv64Str(Fnv64Byte(
		Fnv64Str(Fnv64Offset, "m"), 0), "g"), 0), ""), 0), "") {
		t.Error("a plain shader's hash changed")
	}
}

func TestShaderHashCacheSeesSurfaceChange(t *testing.T) {
	var c ShaderHashCache
	s := &Shader{Metal: "m", GLSL: "g"}
	before := c.Hash(s)
	s.Surface = testPixelSurface(t, 1, 1)
	if got := c.Hash(s); got != ShaderHash(s) || got == before {
		t.Errorf("cached hash after setting Surface = %x, want %x", got, ShaderHash(s))
	}
}

func TestShaderSurfaceRedrawsWithoutLayout(t *testing.T) {
	surface := testPlaneSurface(t)
	builds := 0
	w := NewTestWindow(t, WindowCfg{Width: 200, Height: 200})
	w.SetView(func(*Window) View {
		builds++
		return Column(ContainerCfg{
			Width: sbW, Height: sbH, Sizing: FixedFixed,
			Shader: &Shader{Metal: "m", GLSL: "g", Surface: surface},
		})
	})
	w.FrameFn()
	commandSurface := func() *PixelSurface {
		for _, cmd := range w.Renderers() {
			if cmd.Kind == RenderCustomShader {
				return cmd.DrawnSurface()
			}
		}
		return nil
	}
	if commandSurface() != surface {
		t.Fatal("shader command does not carry its surface")
	}
	before := builds
	if err := surface.UpdatePlanes(PixelPlane{Pix: make([]byte, 8), Stride: 4}, PixelPlane{Pix: make([]byte, 4), Stride: 4}); err != nil {
		t.Fatal(err)
	}
	if !w.FrameFn() {
		t.Error("a new frame did not request a redraw")
	}
	if builds != before {
		t.Error("a new frame rebuilt the layout")
	}
	if commandSurface() != surface {
		t.Error("surface lost during the render-only refresh")
	}
}

func TestShaderWithoutSurfaceHasNoSurface(t *testing.T) {
	w := shaderWindow(t, shaderRoot(func() []View { return []View{shaderBox("sh")} }))
	for _, cmd := range w.Renderers() {
		if cmd.Kind == RenderCustomShader && cmd.DrawnSurface() != nil {
			t.Error("a plain shader command carries a surface")
		}
	}
	if w.pixelSurfaces.drawn != nil || w.pixelSurfaces.drew {
		t.Error("a plain shader built surface subscriptions")
	}
}

func TestRendersShaders(t *testing.T) {
	w := NewTestWindow(t, WindowCfg{Width: 10, Height: 10})
	if w.RendersShaders() {
		t.Error("a window with no GPU backend reports shaders")
	}
	SetWindowRendersShaders(w, true)
	if !w.RendersShaders() {
		t.Error("RendersShaders ignored the backend")
	}
	var absent *Window
	SetWindowRendersShaders(absent, true)
	if absent.RendersShaders() {
		t.Error("a nil window reports shaders")
	}
}

func TestDrawnSurfaceHasOneSourcePerKind(t *testing.T) {
	image := testPixelSurface(t, 1, 1)
	planes := testPlaneSurface(t)
	for _, tc := range []struct {
		name string
		cmd  RenderCmd
		want *PixelSurface
	}{
		{"image", RenderCmd{Kind: RenderImage, Surface: image}, image},
		// A shader reads only Shader.Surface, even where RenderCmd.Surface
		// is nil or set to something else.
		{"shader", RenderCmd{Kind: RenderCustomShader, Shader: &Shader{Surface: planes}}, planes},
		{"shader ignores Surface", RenderCmd{Kind: RenderCustomShader, Surface: image, Shader: &Shader{Surface: planes}}, planes},
		{"plain shader", RenderCmd{Kind: RenderCustomShader, Shader: &Shader{}}, nil},
		{"shader without Shader", RenderCmd{Kind: RenderCustomShader}, nil},
		{"other kind", RenderCmd{Kind: RenderRect, Surface: image}, nil},
	} {
		if got := tc.cmd.DrawnSurface(); got != tc.want {
			t.Errorf("%s: DrawnSurface = %p, want %p", tc.name, got, tc.want)
		}
	}
}
