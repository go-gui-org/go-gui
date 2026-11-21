// This example demonstrates frames that live on the GPU, drawn without a copy (advanced: custom shaders).
// Dmabuf_surface publishes NV12 frames the way a hardware video decoder
// would: each frame is a pair of DMA-BUFs, allocated with GBM and lent to
// a PixelSurface with UpdateDMABuf, then handed back through Release. A
// window that cannot import DMA-BUFs gets the same frames copied with
// UpdatePlanes instead.
package main

import (
	"flag"
	"log"
	"sync/atomic"
	"time"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend"
	"github.com/go-gui-org/go-gui/gui/backend/soft"
)

const frameWidth, frameHeight = 640, 360

type App struct {
	shader *gui.Shader
	// gpuErr says why frames cannot go to the GPU (no Linux, no GPU,
	// no libgbm), or is nil. lent is set once a frame went out as a
	// DMA-BUF.
	gpuErr error
	lent   *atomic.Bool
}

// nv12Planes describes an NV12 frame: full-size luma, and chroma at half
// size with Cb and Cr interleaved.
var nv12Planes = []gui.PixelPlaneCfg{
	{Format: gui.PixelFormatR8, Width: frameWidth, Height: frameHeight},
	{Format: gui.PixelFormatRG8, Width: frameWidth / 2, Height: frameHeight / 2},
}

// The shader converts BT.709 limited-range YUV to RGB, as in
// examples/shader_channels: it reads a DMA-BUF frame and a copied one
// alike.
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
	copied := newCPUFrame()
	copied.paint(0)
	if err = copied.publish(surface); err != nil {
		log.Print(err)
		return
	}
	gpu, gpuErr := newGPUFrames(3)
	state := &App{gpuErr: gpuErr, lent: new(atomic.Bool),
		shader: &gui.Shader{GLSL: nv12GLSL, Metal: nv12Metal, Surface: surface}}
	w := gui.SimpleWindow("DMA-BUF Surface", 720, 520, state, func(w *gui.Window) {
		w.SetView(mainView)
	})
	if *screenshot != "" {
		if err = soft.RenderToPNG(w, 1, *screenshot); err != nil {
			log.Print(err)
		}
		w.WindowCleanup()
		return
	}
	stop, done := make(chan struct{}), make(chan struct{})
	go produce(w, surface, gpu, copied, state.lent, stop, done)
	backend.Run(w)
	close(stop)
	<-done
	// The surface releases the frame it still shows when it closes; the
	// renderer released the rest when its window closed.
	surface.Close()
	if gpu != nil {
		gpu.close()
	}
}

func mainView(w *gui.Window) gui.View {
	state := gui.State[App](w)
	note := "Frames in DMA-BUFs, drawn without a copy"
	switch {
	case !w.RendersShaders():
		note = "This renderer runs no shaders, so the frame area stays empty"
	case !w.ImportsDMABuf():
		note = "This window cannot import DMA-BUFs: frames are copied with UpdatePlanes"
	case state.gpuErr != nil:
		note = "No GPU buffers (" + state.gpuErr.Error() + "): frames are copied with UpdatePlanes"
	case !state.lent.Load():
		note = "Waiting for the first DMA-BUF frame"
	}
	return gui.Column(gui.ContainerCfg{
		Sizing: gui.FillFill, Padding: gui.PaddingLarge,
		Spacing: gui.SpacingMedium, HAlign: gui.HAlignCenter,
		Content: []gui.View{
			gui.Text(gui.TextCfg{Text: "DMA-BUF Surface", TextStyle: gui.CurrentTheme().TextStyleTitle}),
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

// produce publishes 60 frames a second until stop: as DMA-BUFs while the
// window imports them and a pooled GPU frame is free, otherwise copied.
// It checks ImportsDMABuf every frame, because the backend sets it only
// once the window is up.
func produce(w *gui.Window, surface *gui.PixelSurface, gpu *gpuFrames, copied *cpuFrame,
	lent *atomic.Bool, stop <-chan struct{}, done chan<- struct{}) {
	defer close(done)
	ticker := time.NewTicker(time.Second / 60)
	defer ticker.Stop()
	for n := 1; ; n++ {
		select {
		case <-stop:
			return
		case <-ticker.C:
		}
		if gpu != nil && w.ImportsDMABuf() {
			// A frame is free once the renderer released it; with all
			// three still on their way back, this tick is skipped.
			published, err := gpu.publish(surface, n)
			if err == nil {
				if published && !lent.Swap(true) {
					w.InvalidateLayout() // the note changes
				}
				continue
			}
			// Copy from here on: a pool that fails once is not tried again.
			log.Printf("%v; copying frames from now on", err)
			gpu = nil
		}
		copied.paint(n)
		if copied.publish(surface) != nil {
			return
		}
	}
}

// cpuFrame holds one frame's planes in main memory, reused every frame.
type cpuFrame struct {
	luma, chroma []byte
}

func newCPUFrame() *cpuFrame {
	return &cpuFrame{
		luma:   make([]byte, frameWidth*frameHeight),
		chroma: make([]byte, frameWidth/2*frameHeight/2*2),
	}
}

func (f *cpuFrame) paint(n int) {
	for y := range frameHeight {
		lumaRow(n, y, f.luma[y*frameWidth:(y+1)*frameWidth])
	}
	for y := range frameHeight / 2 {
		chromaRow(n, y, f.chroma[y*frameWidth:(y+1)*frameWidth])
	}
}

// publish copies both planes to the surface as one frame.
func (f *cpuFrame) publish(surface *gui.PixelSurface) error {
	return surface.UpdatePlanes(
		gui.PixelPlane{Pix: f.luma, Stride: frameWidth},
		gui.PixelPlane{Pix: f.chroma, Stride: frameWidth},
	)
}

// picture is the frame without its marker, in NV12, converted once: the
// frames differ only in the marker, so a frame costs a copy of each row
// and four columns of white, and the CPU time the example takes is mostly
// the display path's.
var picture = paintPicture()

func paintPicture() (p struct{ luma, chroma []byte }) {
	p.luma = make([]byte, frameWidth*frameHeight)
	p.chroma = make([]byte, frameWidth*frameHeight/2)
	for y := range frameHeight {
		for x := range frameWidth {
			luma, cb, cr := rgbToYUV(pixelAt(x, y))
			p.luma[y*frameWidth+x] = luma
			// Chroma from the top-left pixel of each 2x2 block.
			if x%2 == 0 && y%2 == 0 {
				i := (y/2)*frameWidth + x
				p.chroma[i], p.chroma[i+1] = cb, cr
			}
		}
	}
	return p
}

// marker is the first column of frame n's white marker, which moves right.
func marker(n int) int { return (n * 4) % frameWidth }

// lumaRow paints row y of frame n's luma plane.
func lumaRow(n, y int, row []byte) {
	copy(row, picture.luma[y*frameWidth:(y+1)*frameWidth])
	white, _, _ := rgbToYUV(255, 255, 255)
	for x := marker(n); x < min(marker(n)+4, frameWidth); x++ {
		row[x] = white
	}
}

// chromaRow paints row y of frame n's chroma plane, Cb and Cr
// interleaved.
func chromaRow(n, y int, row []byte) {
	copy(row, picture.chroma[y*frameWidth:(y+1)*frameWidth])
	for x := marker(n) / 2; x < min((marker(n)+4)/2, frameWidth/2); x++ {
		row[2*x], row[2*x+1] = 128, 128 // white has no chroma
	}
}

// pixelAt is the RGB color of one pixel of the picture: color bars, and a
// gray ramp below them.
func pixelAt(x, y int) (r, g, b byte) {
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
