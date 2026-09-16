// This example demonstrates custom scrollbar looks drawn with
// ScrollbarCfg.Thumb and ScrollbarCfg.Track (advanced: views sized after
// layout).
//
// It ports go-shirei's custom-scrollbars demo. Four panels scroll the same
// rows:
//
//   - Default. The stock scrollbar, for comparison.
//   - Classic. A white track and a blue pill with a grip.
//   - Windows 98. A silver track and a raised 3D thumb with ridges.
//   - Cool blue. A pale trough and a powder-blue thumb with white ticks.
//
// go-shirei's ScrollBarExt takes a Thumb function that draws at a size. In
// go-gui the scrollbar keeps the scrolling: it sizes and moves the thumb,
// runs the drag, jumps on a gutter press and hides the thumb when nothing
// overflows. The hooks only draw. They get the hover and press state and the
// size of their part, and return a view that fills it.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend"
	"github.com/go-gui-org/go-gui/gui/backend/soft"
)

func main() {
	screenshot := flag.String("screenshot", "", "write screenshot and exit")
	flag.Parse()

	gui.SetTheme(gui.ThemeLight)

	w := gui.NewWindow(gui.WindowCfg{
		Title:  "Custom Scrollbars",
		Width:  960,
		Height: 640,
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

// rowCount is how many rows each panel holds: enough to scroll.
const rowCount = 40

// barSize is the thickness of every custom bar, as in go-shirei.
const barSize = 16

var (
	pageBG  = gui.Hex(0xf1f3f7)
	panelBG = gui.Hex(0xffffff)
	rowText = gui.Hex(0x1f2430)
	white   = gui.Hex(0xffffff)
)

func mainView(w *gui.Window) gui.View {
	title := gui.CurrentTheme().TextStyleDef
	title.Size = 18
	return gui.Column(gui.ContainerCfg{
		ID:      "page",
		Sizing:  gui.FillFill,
		Color:   pageBG,
		Padding: gui.PadAll(24),
		Spacing: gui.SomeF(16),
		Content: []gui.View{
			gui.Text(gui.TextCfg{Text: "Custom scrollbars with ScrollbarCfg.Thumb and Track", TextStyle: title}),
			panelRow(
				panel("stock", "Default", "gui's own scrollbar, for comparison", nil),
				panel("classic", "Classic", "White track, blue pill with a grip", classicBar()),
			),
			panelRow(
				panel("win98", "Windows 98", "Silver track, raised 3D thumb", win98Bar()),
				panel("blue", "Cool blue", "Pale trough, powder-blue thumb with white ticks", coolBlueBar()),
			),
		},
	})
}

func panelRow(content ...gui.View) gui.View {
	return gui.Row(gui.ContainerCfg{
		Sizing:     gui.FillFill,
		Padding:    gui.PaddingNone,
		SizeBorder: gui.NoBorder,
		Spacing:    gui.SomeF(14),
		Content:    content,
	})
}

// panel is a titled card whose body scrolls. bar is the vertical scrollbar
// config; nil keeps the stock one.
func panel(id, title, sub string, bar *gui.ScrollbarCfg) gui.View {
	rows := make([]gui.View, rowCount)
	for i := range rows {
		rows[i] = row(i)
	}
	return gui.Column(gui.ContainerCfg{
		Sizing:     gui.FillFill,
		Padding:    gui.PaddingNone,
		SizeBorder: gui.NoBorder,
		Spacing:    gui.SomeF(8),
		Content: []gui.View{
			gui.Column(gui.ContainerCfg{
				Padding:    gui.PaddingNone,
				SizeBorder: gui.NoBorder,
				Spacing:    gui.SomeF(2),
				Content: []gui.View{
					gui.Text(gui.TextCfg{Text: title, TextStyle: gui.CurrentTheme().B4}),
					gui.Text(gui.TextCfg{Text: sub, TextStyle: gui.CurrentTheme().TextStyleSecondary}),
				},
			}),
			gui.Column(gui.ContainerCfg{
				ID:            id,
				Sizing:        gui.FillFill,
				Scrollable:    true,
				ScrollbarCfgY: bar,
				Color:         panelBG,
				Radius:        gui.SomeF(6),
				ColorBorder:   gui.RGBA(0, 0, 0, 26),
				SizeBorder:    gui.SomeF(1),
				Padding:       gui.NewPadding(12, 12+barSize, 12, 12),
				Spacing:       gui.SomeF(6),
				Content:       rows,
			}),
		},
	})
}

// rowShades are the three alternating row backgrounds.
var rowShades = []gui.Color{gui.Hex(0xe9edf5), gui.Hex(0xedf0f6), gui.Hex(0xf1f3f8)}

func row(i int) gui.View {
	style := gui.CurrentTheme().TextStyleDef
	style.Color = rowText
	style.Size = 13
	return gui.Row(gui.ContainerCfg{
		Sizing:     gui.FillFit,
		Color:      rowShades[i%len(rowShades)],
		Radius:     gui.SomeF(3),
		SizeBorder: gui.NoBorder,
		Padding:    gui.NewPadding(6, 8, 6, 8),
		Content:    []gui.View{gui.Text(gui.TextCfg{Text: fmt.Sprintf("Row %02d — scroll me", i+1), TextStyle: style})},
	})
}

// fill is a container that fills its parent. Every layer of a thumb uses
// it, so the layers take the thumb's size.
func fill(c gui.Color, pad gui.Padding, radius float32, content ...gui.View) gui.ContainerCfg {
	return gui.ContainerCfg{
		Sizing:     gui.FillFill,
		Color:      c,
		Padding:    pad,
		Radius:     gui.SomeF(radius),
		SizeBorder: gui.NoBorder,
		Spacing:    gui.SomeF(0),
		HAlign:     gui.HAlignCenter,
		VAlign:     gui.VAlignMiddle,
		Content:    content,
	}
}

// ticks is a column of three short lines, as wide as frac of the thumb.
func ticks(s gui.ScrollbarState, frac, h float32, c gui.Color) gui.View {
	w := max(s.Width*frac, 4)
	lines := make([]gui.View, 3)
	for i := range lines {
		lines[i] = gui.Rectangle(gui.RectangleCfg{Width: w, Height: h, Sizing: gui.FixedFixed, Color: c})
	}
	return gui.Column(gui.ContainerCfg{
		Padding:    gui.PaddingNone,
		SizeBorder: gui.NoBorder,
		Spacing:    gui.SomeF(2),
		Content:    lines,
	})
}

// bar is the geometry every custom bar shares: go-shirei's 16 px track with
// no gap to the edge.
func bar(minThumb float32) *gui.ScrollbarCfg {
	return &gui.ScrollbarCfg{
		Size:         barSize,
		MinThumbSize: minThumb,
		GapEdge:      gui.SomeF(0),
		GapEnd:       gui.SomeF(0),
	}
}

// --- Classic ---------------------------------------------------------------

var (
	classicTrack  = gui.Hex(0xffffff)
	classicAccent = gui.Hex(0x3b82f6)
	classicHot    = gui.Hex(0x2563eb)
	classicEdge   = gui.Hex(0x1d4ed8)
)

// classicBar is a white track with a blue pill. The pill darkens under the
// pointer and while it is dragged.
func classicBar() *gui.ScrollbarCfg {
	cfg := bar(30)
	cfg.Track = func(gui.ScrollbarState) gui.View {
		return gui.Column(fill(classicTrack, gui.PaddingNone, 0))
	}
	cfg.Thumb = func(s gui.ScrollbarState) gui.View {
		face := classicAccent
		if s.Hovered || s.Pressed {
			face = classicHot
		}
		pill := fill(face, gui.PaddingNone, s.Width/2, ticks(s, 0.4, 1, gui.RGBA(255, 255, 255, 170)))
		pill.ColorBorder = classicEdge
		pill.SizeBorder = gui.SomeF(1)
		// One pixel of track shows around the pill.
		return gui.Column(fill(gui.ColorTransparent, gui.PadAll(1), 0, gui.Column(pill)))
	}
	return cfg
}

// --- Windows 98 ------------------------------------------------------------

var (
	win98Face  = gui.Hex(0xd1d1d1)
	win98Light = gui.Hex(0xffffff)
	win98Dark  = gui.Hex(0x666666)
	win98Ridge = gui.RGBA(0, 0, 0, 90)
)

// win98Bar is a silver track under a raised thumb: a dark layer at the
// bottom and right, a light layer at the top and left, and the face.
func win98Bar() *gui.ScrollbarCfg {
	cfg := bar(28)
	cfg.Track = func(gui.ScrollbarState) gui.View {
		return gui.Column(fill(win98Face, gui.PaddingNone, 0))
	}
	cfg.Thumb = func(s gui.ScrollbarState) gui.View {
		return gui.Column(fill(win98Dark, gui.NewPadding(0, 1, 1, 0), 0,
			gui.Column(fill(win98Light, gui.NewPadding(1, 0, 0, 1), 0,
				gui.Column(fill(win98Face, gui.PaddingNone, 0, ticks(s, 0.45, 1, win98Ridge)))))))
	}
	return cfg
}

// --- Cool blue -------------------------------------------------------------

var (
	blueTrough = gui.Hex(0xe8eef6)
	blueFace   = gui.Hex(0xa9c4ec)
	blueHot    = gui.Hex(0x92b3e6)
	blueRim    = gui.Hex(0x7b9fd6)
)

// coolBlueBar is a pale trough with a powder-blue thumb inset 2 px, a darker
// rim and white ticks.
func coolBlueBar() *gui.ScrollbarCfg {
	cfg := bar(28)
	cfg.Track = func(gui.ScrollbarState) gui.View {
		return gui.Column(fill(blueTrough, gui.PaddingNone, 0))
	}
	cfg.Thumb = func(s gui.ScrollbarState) gui.View {
		face := blueFace
		if s.Hovered || s.Pressed {
			face = blueHot
		}
		return gui.Column(fill(gui.ColorTransparent, gui.PadAll(2), 0,
			gui.Column(fill(blueRim, gui.PadAll(1), 3,
				gui.Column(fill(face, gui.PaddingNone, 2, ticks(s, 0.45, 1.5, white)))))))
	}
	return cfg
}
