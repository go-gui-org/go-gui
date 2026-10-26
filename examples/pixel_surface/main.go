// Pixel_surface animates owned pixels from a producer goroutine.
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

type appState struct{ surface *gui.PixelSurface }

func main() {
	screenshot := flag.String("screenshot", "", "write a static screenshot and exit")
	flag.Parse()
	gui.SetTheme(gui.ThemeDark)
	surface, err := gui.NewPixelSurface(gui.PixelSurfaceCfg{Width: frameWidth, Height: frameHeight})
	if err != nil {
		log.Print(err)
		return
	}
	defer surface.Close()
	pix := make([]byte, frameWidth*frameHeight*4)
	paintFrame(pix, 0)
	if err = surface.Update(pix, frameWidth*4); err != nil {
		log.Print(err)
		return
	}
	w := gui.NewWindow(gui.WindowCfg{
		Title: "Pixel Surface", Width: 720, Height: 500,
		State:  &appState{surface: surface},
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
	go animateSurface(surface, pix, stop, done)
	backend.Run(w)
	close(stop)
	<-done
}

func mainView(w *gui.Window) gui.View {
	state := gui.State[appState](w)
	return gui.Column(gui.ContainerCfg{
		Sizing: gui.FillFill, Padding: gui.PaddingLarge,
		Spacing: gui.SpacingMedium, HAlign: gui.HAlignCenter,
		Content: []gui.View{
			gui.Text(gui.TextCfg{Text: "Pixel Surface", TextStyle: gui.CurrentTheme().TextStyleTitle}),
			gui.Text(gui.TextCfg{Text: "Synthetic frames, published at 60 Hz"}),
			gui.Image(gui.ImageCfg{
				Surface: state.surface, Width: frameWidth, Height: frameHeight,
				A11YCfg: gui.A11YCfg{A11YLabel: "Animated color bars with a moving white marker"},
			}),
		},
	})
}

func animateSurface(surface *gui.PixelSurface, pix []byte, stop <-chan struct{}, done chan<- struct{}) {
	defer close(done)
	ticker := time.NewTicker(time.Second / 60)
	defer ticker.Stop()
	frame := 0
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			frame++
			paintFrame(pix, frame)
			if err := surface.Update(pix, frameWidth*4); err != nil {
				return
			}
		}
	}
}

func paintFrame(pix []byte, frame int) {
	bars := [8][3]byte{{255, 255, 255}, {255, 255, 0}, {0, 255, 255},
		{0, 255, 0}, {255, 0, 255}, {255, 0, 0}, {0, 0, 255}, {0, 0, 0}}
	marker := (frame * 4) % frameWidth
	for y := range frameHeight {
		for x := range frameWidth {
			color := bars[x*len(bars)/frameWidth]
			if y > frameHeight*3/4 {
				shade := byte(x * 255 / (frameWidth - 1))
				color = [3]byte{shade, shade, shade}
			}
			if x >= marker && x < marker+4 {
				color = [3]byte{255, 255, 255}
			}
			i := (y*frameWidth + x) * 4
			pix[i], pix[i+1], pix[i+2], pix[i+3] = color[0], color[1], color[2], 255
		}
	}
}
