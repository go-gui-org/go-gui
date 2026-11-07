// The embedded_image example draws images that the binary carries with
// a go:embed directive (#1228). One scene is embedded three times, as PNG,
// JPEG and GIF. RegisterEncodedImage decodes each one once, at startup,
// and the window uses the returned sources like file paths: no temporary
// file.
package main

import (
	_ "embed"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend"
	"github.com/go-gui-org/go-gui/gui/backend/soft"
)

// sceneW and sceneH are the embedded images' size in pixels.
const (
	sceneW = 192
	sceneH = 128
)

// App holds the decoded pictures for the window's whole life.
type App struct {
	Pictures []picture
}

func main() {
	screenshot := flag.String("screenshot", "", "write screenshot and exit")
	flag.Parse()

	pictures, err := loadPictures()
	if err != nil {
		log.Fatalf("embedded images: %v", err)
	}

	gui.SetTheme(gui.ThemeDark)

	w := gui.NewWindow(gui.WindowCfg{
		State:  &App{Pictures: pictures},
		Title:  "Embedded Image",
		Width:  720,
		Height: 320,
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

// doc:snippet-begin embedded-image-register
//
// loadPictures registers each embedded encoding once, at startup. A
// source kept like this stays valid until DropImage: the in-memory image
// budget never evicts a RegisterEncodedImage image.

//go:embed sunset.png
var sunsetPNG []byte

//go:embed sunset.jpg
var sunsetJPEG []byte

//go:embed sunset.gif
var sunsetGIF []byte

// picture is one decoded image and the caption under it.
type picture struct {
	caption string
	src     string
}

func loadPictures() ([]picture, error) {
	encoded := []struct {
		format, key string
		data        []byte
	}{
		{"PNG", "embedded_image/sunset.png", sunsetPNG},
		{"JPEG", "embedded_image/sunset.jpg", sunsetJPEG},
		{"GIF", "embedded_image/sunset.gif", sunsetGIF},
	}
	pictures := make([]picture, 0, len(encoded))
	for _, e := range encoded {
		src, err := gui.RegisterEncodedImage(e.key, e.data)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", e.key, err)
		}
		caption := fmt.Sprintf("%s, %.1f KB", e.format, float64(len(e.data))/1024)
		pictures = append(pictures, picture{caption: caption, src: src})
	}
	return pictures, nil
}

// doc:snippet-end embedded-image-register

func mainView(w *gui.Window) gui.View {
	theme := gui.CurrentTheme()
	app := gui.State[App](w)
	cards := make([]gui.View, 0, len(app.Pictures))
	for _, p := range app.Pictures {
		cards = append(cards, card(p, theme))
	}
	return gui.Column(gui.ContainerCfg{
		Sizing:  gui.FillFill,
		Spacing: gui.SpacingLarge,
		Padding: gui.PaddingLarge,
		HAlign:  gui.HAlignCenter,
		Content: []gui.View{
			gui.Text(gui.TextCfg{
				Text:      "Embedded Image",
				TextStyle: theme.TextStyleTitle,
			}),
			gui.Text(gui.TextCfg{
				Text:      "One scene embedded as PNG, JPEG and GIF, each decoded once at startup.",
				TextStyle: theme.TextStyleSecondary,
			}),
			gui.Row(gui.ContainerCfg{
				Spacing:    gui.SpacingLarge,
				Padding:    gui.PaddingNone,
				SizeBorder: gui.NoBorder,
				Content:    cards,
			}),
		},
	})
}

// card shows one picture with its format and encoded size under it.
func card(p picture, theme gui.Theme) gui.View {
	return gui.Column(gui.ContainerCfg{
		Spacing:    gui.SpacingSmall,
		Padding:    gui.PaddingNone,
		SizeBorder: gui.NoBorder,
		HAlign:     gui.HAlignCenter,
		Content: []gui.View{
			gui.Image(gui.ImageCfg{
				Src:     p.src,
				Width:   sceneW,
				Height:  sceneH,
				A11YCfg: gui.A11YCfg{A11YLabel: "Sunset over hills, " + p.caption},
			}),
			gui.Text(gui.TextCfg{
				Text:      p.caption,
				TextStyle: theme.TextStyleSecondary,
			}),
		},
	})
}
