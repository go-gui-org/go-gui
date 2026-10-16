// The pixel_art example draws one small sprite at 12x scale twice:
// smooth with ImageFilterLinear, and with hard pixel edges with
// ImageFilterNearest (#1065).
package main

import (
	"flag"
	"log"
	"os"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend"
	"github.com/go-gui-org/go-gui/gui/backend/soft"
)

const (
	// spriteSize is the sprite's width and height in texels.
	spriteSize = 8
	// zoom is how many screen pixels one texel covers.
	zoom    = 12
	drawn   = spriteSize * zoom
	gap     = 32
	canvasW = drawn*2 + gap
	canvasH = drawn
)

// spriteRows is an 8x8 heart. '#' is the outline, 'r' the fill, 'w' a
// highlight and '.' transparent.
var spriteRows = [spriteSize]string{
	".##..##.",
	"#rr##rr#",
	"#rwrrrr#",
	"#rrrrrr#",
	".#rrrr#.",
	"..#rr#..",
	"...##...",
	"........",
}

func main() {
	screenshot := flag.String("screenshot", "", "write screenshot and exit")
	flag.Parse()

	gui.SetTheme(gui.ThemeDark)

	w := gui.NewWindow(gui.WindowCfg{
		Title:  "Pixel Art",
		Width:  360,
		Height: 260,
		OnInit: func(w *gui.Window) {
			w.SetView(mainView)
		},
	})

	if *screenshot != "" {
		if err := soft.RenderToPNG(w, 2, *screenshot); err != nil {
			log.Fatalf("screenshot: %v", err)
		}
		os.Exit(0)
	}
	backend.Run(w)
}

func mainView(w *gui.Window) gui.View {
	theme := gui.CurrentTheme()
	src := spriteSrc()
	return gui.Column(gui.ContainerCfg{
		Sizing:  gui.FillFill,
		Spacing: gui.SpacingLarge,
		Padding: gui.PaddingLarge,
		HAlign:  gui.HAlignCenter,
		Content: []gui.View{
			gui.Text(gui.TextCfg{
				Text:      "Pixel Art",
				TextStyle: theme.TextStyleTitle,
			}),
			gui.DrawCanvas(gui.DrawCanvasCfg{
				ID:      "sprites",
				Width:   canvasW,
				Height:  canvasH,
				Sizing:  gui.FixedFixed,
				Version: 1,
				OnDraw: func(dc *gui.DrawContext) {
					drawSprites(dc, src)
				},
			}),
			// The captions line up under the two sprites: the same
			// widths and the same gap as the canvas draws them with.
			gui.Row(gui.ContainerCfg{
				Spacing:    gui.SpacingPx(gap),
				Padding:    gui.PaddingNone,
				SizeBorder: gui.NoBorder,
				Content: []gui.View{
					label("Linear", theme),
					label("Nearest", theme),
				},
			}),
		},
	})
}

// label is the caption under one of the two sprites, centred in a
// column as wide as the sprite.
func label(text string, theme gui.Theme) gui.View {
	return gui.Column(gui.ContainerCfg{
		Width:      drawn,
		Sizing:     gui.FixedFit,
		Padding:    gui.PaddingNone,
		SizeBorder: gui.NoBorder,
		HAlign:     gui.HAlignCenter,
		Content: []gui.View{
			gui.Text(gui.TextCfg{
				Text:      text,
				TextStyle: theme.TextStyleSecondary,
			}),
		},
	})
}

// doc:snippet-begin pixel-art-draw

// drawSprites draws the sprite twice at the same size. The left copy
// uses the default linear filter and looks blurred. The right copy
// sets the nearest filter inside Save/Restore, so each texel stays a
// sharp square and the filter goes back to linear after it.
func drawSprites(dc *gui.DrawContext, src string) {
	dc.Image(0, 0, drawn, drawn, src, gui.Opt[float32]{}, gui.Color{})

	dc.Save()
	dc.SetImageFilter(gui.ImageFilterNearest)
	dc.Image(drawn+gap, 0, drawn, drawn, src, gui.Opt[float32]{}, gui.Color{})
	dc.Restore()
}

// doc:snippet-end pixel-art-draw

// spriteKey names the sprite in gui's in-memory image registry.
const spriteKey = "pixel-art-heart"

// spritePix is the sprite as NRGBA bytes, built once. The registry
// keeps this buffer, so it is never changed after it is built.
var spritePix = buildSprite()

// spriteSrc returns the sprite's source string. UseImage stores the
// buffer on the first call; later calls with the same key and size only
// mark it recently used, so the backend keeps its uploaded texture.
func spriteSrc() string {
	return gui.UseImage(spriteKey, spriteSize, spriteSize, spritePix)
}

// buildSprite turns spriteRows into NRGBA bytes.
func buildSprite() []byte {
	pix := make([]byte, 0, spriteSize*spriteSize*4)
	for _, row := range spriteRows {
		for _, c := range []byte(row) {
			switch c {
			case '#':
				pix = append(pix, 130, 15, 35, 255)
			case 'r':
				pix = append(pix, 220, 40, 60, 255)
			case 'w':
				pix = append(pix, 255, 230, 230, 255)
			default:
				pix = append(pix, 0, 0, 0, 0)
			}
		}
	}
	return pix
}
