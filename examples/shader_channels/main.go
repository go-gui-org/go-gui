// This example demonstrates a shader reading the planes of a PixelSurface (advanced: custom shaders).
// Shader_channels publishes synthetic NV12 video frames, a luma plane and a
// half-size chroma plane, and converts them to RGB in a fragment shader that
// reads them as iChannel0 and iChannel1. No video code is involved: the
// frames are color bars converted to YUV in Go.
package main

import (
	"flag"
	"log"
	"time"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend"
	"github.com/go-gui-org/go-gui/gui/backend/soft"
)

const frameWidth, frameHeight = 640, 360

type appState struct {
	surface *gui.PixelSurface
	shader  *gui.Shader
}

// nv12Planes describes an NV12 frame: full-size luma, and chroma at half
// size with Cb and Cr interleaved.
var nv12Planes = []gui.PixelPlaneCfg{
	{Format: gui.PixelFormatR8, Width: frameWidth, Height: frameHeight},
	{Format: gui.PixelFormatRG8, Width: frameWidth / 2, Height: frameHeight / 2},
}

// The shader converts BT.709 limited-range YUV to RGB. p = uv * 0.5 + 0.5
// puts (0,0) at the frame's top-left pixel.
const nv12GLSL = `vec2 p = uv * 0.5 + 0.5;
float y = (texture(iChannel0, p).r - 16.0 / 255.0) * (255.0 / 219.0);
vec2 c = (texture(iChannel1, p).rg - 128.0 / 255.0) * (255.0 / 224.0);
vec3 rgb = vec3(y + 1.5748 * c.y, y - 0.1873 * c.x - 0.4681 * c.y, y + 1.8556 * c.x);
vec4 frag_color = vec4(clamp(rgb, 0.0, 1.0), 1.0);`

const nv12Metal = `float2 p = in.uv * 0.5 + 0.5;
float y = (iChannel0.sample(iChannelSampler, p).r - 16.0 / 255.0) * (255.0 / 219.0);
float2 c = (iChannel1.sample(iChannelSampler, p).rg - 128.0 / 255.0) * (255.0 / 224.0);
float3 rgb = float3(y + 1.5748 * c.y, y - 0.1873 * c.x - 0.4681 * c.y, y + 1.8556 * c.x);
float4 frag_color = float4(clamp(rgb, 0.0, 1.0), 1.0);`

func main() {
	screenshot := flag.String("screenshot", "", "write a static screenshot and exit")
	flag.Parse()
	gui.SetTheme(gui.ThemeDark)
	surface, err := gui.NewPixelSurface(gui.PixelSurfaceCfg{Planes: nv12Planes})
	if err != nil {
		log.Print(err)
		return
	}
	defer surface.Close()
	frame := newFrame()
	frame.paint(0)
	if err = frame.publish(surface); err != nil {
		log.Print(err)
		return
	}
	state := &appState{surface: surface,
		shader: &gui.Shader{GLSL: nv12GLSL, Metal: nv12Metal, Surface: surface}}
	w := gui.NewWindow(gui.WindowCfg{
		Title: "Shader Channels", Width: 720, Height: 500, State: state,
		OnInit: func(window *gui.Window) { window.SetView(mainView) },
	})
	w.SetView(mainView)
	if *screenshot != "" {
		if err = soft.RenderToPNG(w, 1, *screenshot); err != nil {
			log.Print(err)
		}
		w.WindowCleanup()
		return
	}
	stop, done := make(chan struct{}), make(chan struct{})
	go animate(surface, frame, stop, done)
	backend.Run(w)
	close(stop)
	<-done
}

func mainView(w *gui.Window) gui.View {
	state := gui.State[appState](w)
	note := "NV12 frames, converted to RGB on the GPU at 60 Hz"
	if !w.RendersShaders() {
		note = "This renderer runs no shaders, so the frame area stays empty"
	}
	return gui.Column(gui.ContainerCfg{
		Sizing: gui.FillFill, Padding: gui.PaddingLarge,
		Spacing: gui.SpacingMedium, HAlign: gui.HAlignCenter,
		Content: []gui.View{
			gui.Text(gui.TextCfg{Text: "Shader Channels", TextStyle: gui.CurrentTheme().TextStyleTitle}),
			gui.Text(gui.TextCfg{Text: note}),
			gui.Column(gui.ContainerCfg{
				Width: frameWidth, Height: frameHeight, Sizing: gui.FixedFixed,
				Padding: gui.PaddingNone, SizeBorder: gui.NoBorder, Radius: gui.NoRadius,
				Color: gui.Black, Shader: state.shader,
				A11YCfg: gui.A11YCfg{A11YLabel: "Animated color bars with a moving white marker"},
			}),
		},
	})
}

func animate(surface *gui.PixelSurface, frame *nv12Frame, stop <-chan struct{}, done chan<- struct{}) {
	defer close(done)
	ticker := time.NewTicker(time.Second / 60)
	defer ticker.Stop()
	n := 0
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			n++
			frame.paint(n)
			if err := frame.publish(surface); err != nil {
				return
			}
		}
	}
}

// nv12Frame holds one frame's planes, reused for every frame.
type nv12Frame struct {
	luma, chroma []byte
}

func newFrame() *nv12Frame {
	return &nv12Frame{
		luma:   make([]byte, frameWidth*frameHeight),
		chroma: make([]byte, frameWidth/2*frameHeight/2*2),
	}
}

// publish hands both planes to the surface as one frame.
func (f *nv12Frame) publish(surface *gui.PixelSurface) error {
	return surface.UpdatePlanes(
		gui.PixelPlane{Pix: f.luma, Stride: frameWidth},
		gui.PixelPlane{Pix: f.chroma, Stride: frameWidth},
	)
}

// paint draws color bars with a moving white marker and a gray ramp, and
// converts them to NV12. Chroma is taken from the top-left pixel of each
// 2x2 block; the bars are wide enough that averaging changes nothing.
func (f *nv12Frame) paint(n int) {
	marker := (n * 4) % frameWidth
	for y := range frameHeight {
		for x := range frameWidth {
			r, g, b := pixelAt(x, y, marker)
			luma, cb, cr := rgbToYUV(r, g, b)
			f.luma[y*frameWidth+x] = luma
			if x%2 == 0 && y%2 == 0 {
				i := (y/2)*frameWidth + x
				f.chroma[i], f.chroma[i+1] = cb, cr
			}
		}
	}
}

// pixelAt is the RGB color of one pixel of frame n.
func pixelAt(x, y, marker int) (r, g, b byte) {
	if x >= marker && x < marker+4 {
		return 255, 255, 255
	}
	if y > frameHeight*3/4 {
		v := byte(x * 255 / (frameWidth - 1))
		return v, v, v
	}
	bars := [8][3]byte{{255, 255, 255}, {255, 255, 0}, {0, 255, 255},
		{0, 255, 0}, {255, 0, 255}, {255, 0, 0}, {0, 0, 255}, {0, 0, 0}}
	c := bars[x*len(bars)/frameWidth]
	return c[0], c[1], c[2]
}

// rgbToYUV converts to BT.709 limited range, the inverse of the shader.
func rgbToYUV(r, g, b byte) (y, cb, cr byte) {
	rf, gf, bf := float64(r)/255, float64(g)/255, float64(b)/255
	luma := 0.2126*rf + 0.7152*gf + 0.0722*bf
	return clampByte(16 + 219*luma),
		clampByte(128 + 224*(bf-luma)/1.8556),
		clampByte(128 + 224*(rf-luma)/1.5748)
}

func clampByte(v float64) byte {
	switch {
	case v <= 0:
		return 0
	case v >= 255:
		return 255
	}
	return byte(v + 0.5)
}
