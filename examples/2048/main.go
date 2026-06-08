package main

import (
	"fmt"
	"math"
	"time"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend"
)

type Screen int

const (
	ScreenLanding Screen = iota
	ScreenPlaying
)

type App struct {
	Game         *Game
	Screen       Screen
	BestScore    int
	LandingFrame int
}

const (
	gridPx      = float32(400)
	tileMargin  = float32(12)
	tilePx      = (gridPx - (5 * tileMargin)) / 4
	radiusBoard = float32(8)
	radiusTile  = float32(4)
)

func main() {
	gui.SetTheme(gui.ThemeDark.WithBorders(true))

	w := gui.NewWindow(gui.WindowCfg{
		State: &App{
			Game:   NewGame(),
			Screen: ScreenLanding,
		},
		Title:     "2048",
		Width:     600,
		Height:    700,
		FixedSize: true,
		OnInit: func(w *gui.Window) {
			w.UpdateView(mainView)
			w.AnimationAdd(&gui.Animate{
				AnimID: "landing-pulsate",
				Delay:  16 * time.Millisecond,
				Repeat: true,
				Callback: func(_ *gui.Animate, w *gui.Window) {
					app := gui.State[App](w)
					app.LandingFrame++
				},
			})
		},
		OnEvent: handleEvent,
	})

	backend.Run(w)
}

func mainView(w *gui.Window) gui.View {
	app := gui.State[App](w)
	if app.Screen == ScreenLanding {
		return landingView(w)
	}
	return gameView(w)
}

func handleEvent(e *gui.Event, w *gui.Window) {
	app := gui.State[App](w)

	if app.Screen == ScreenLanding {
		if e.Type == gui.EventKeyDown {
			switch e.KeyCode {
			case gui.KeySpace, gui.KeyEnter:
				startGame(w)
				e.IsHandled = true
			}
		}
		return
	}

	if e.Type == gui.EventKeyDown {
		moved := false
		switch e.KeyCode {
		case gui.KeyUp, gui.KeyW:
			moved = app.Game.Move(DirUp)
		case gui.KeyDown, gui.KeyS:
			moved = app.Game.Move(DirDown)
		case gui.KeyLeft, gui.KeyA:
			moved = app.Game.Move(DirLeft)
		case gui.KeyRight, gui.KeyD:
			moved = app.Game.Move(DirRight)
		case gui.KeyR:
			app.Game.Reset()
			moved = true
		case gui.KeyEscape:
			app.Screen = ScreenLanding
			e.IsHandled = true
			return
		}

		if moved {
			if app.Game.Score > app.BestScore {
				app.BestScore = app.Game.Score
			}
			e.IsHandled = true
		}
	}
}

func startGame(w *gui.Window) {
	app := gui.State[App](w)
	app.Game.Reset()
	app.Screen = ScreenPlaying
}

// --- Views ---

func landingView(w *gui.Window) gui.View {
	app := gui.State[App](w)
	ww, wh := w.WindowSize()
	theme := gui.CurrentTheme()

	return gui.Column(gui.ContainerCfg{
		Width:  float32(ww),
		Height: float32(wh),
		Sizing: gui.FixedFixed,
		Color:  gui.RGB(20, 20, 25),
		HAlign: gui.HAlignCenter,
		VAlign: gui.VAlignMiddle,
		Content: []gui.View{
			// Background "jazz": floating tiles
			landingBackdrop(float32(ww), float32(wh), app.LandingFrame),

			gui.Column(gui.ContainerCfg{
				Spacing: gui.Some[float32](40),
				HAlign:  gui.HAlignCenter,
				Content: []gui.View{
					gui.Column(gui.ContainerCfg{
						Spacing: gui.Some[float32](10),
						HAlign:  gui.HAlignCenter,
						Content: []gui.View{
							gui.Text(gui.TextCfg{
								Text:      "2048",
								TextStyle: textStyle(theme.B1, 120, gui.White),
							}),
							gui.Text(gui.TextCfg{
								Text:      "JOIN THE NUMBERS TO GET TO THE 2048 TILE!",
								TextStyle: textStyle(theme.N1, 16, gui.RGB(150, 150, 160)),
							}),
						},
					}),

					gui.Button(gui.ButtonCfg{
						IDFocus:     1,
						MinWidth:    200,
						Color:       gui.RGB(237, 194, 46),
						ColorHover:  gui.RGB(245, 210, 80),
						Padding:     gui.SomeP(16, 32, 16, 32),
						SizeBorder:  gui.Some[float32](2),
						ColorBorder: gui.White,
						Content: []gui.View{
							gui.Text(gui.TextCfg{
								Text:      "NEW GAME",
								TextStyle: textStyle(theme.B2, 24, gui.White),
							}),
						},
						OnClick: func(_ *gui.Layout, e *gui.Event, w *gui.Window) {
							startGame(w)
							e.IsHandled = true
						},
					}),

					gui.Text(gui.TextCfg{
						Text:      "PRESS SPACE TO BEGIN",
						TextStyle: textStyle(theme.N1, 14, gui.RGB(100, 100, 110)),
					}),
				},
			}),
		},
	})
}

func landingBackdrop(ww, wh float32, frame int) gui.View {
	// Simple grid of tiles that fade in and out or move slightly
	content := []gui.View{}
	for i := range 8 {
		// Use frame to animate position
		offset := float32(math.Sin(float64(frame)/30.0+float64(i))) * 20.0
		x := float32((i*137)%int(ww)) - 40 + offset
		y := float32((i*251)%int(wh)) - 40 + offset

		size := float32(80 + (i*10)%40)
		alpha := uint8(20 + (i*5)%30)
		content = append(content, gui.Column(gui.ContainerCfg{
			X:      x,
			Y:      y,
			Width:  size,
			Height: size,
			Sizing: gui.FixedFixed,
			Color:  gui.RGBA(237, 194, 46, alpha),
			Radius: gui.Some[float32](12),
			Float:  true, // Make sure backdrop elements are floating
		}))
	}
	// Return a view that can be used as a child of a Column but behaves like a canvas
	return gui.Canvas(gui.ContainerCfg{
		Width: ww, Height: wh, Sizing: gui.FixedFixed,
		Content: content,
		Float:   true, // The backdrop itself should be floating so it doesn't push the UI Column
	})
}

func gameView(w *gui.Window) gui.View {
	app := gui.State[App](w)
	ww, wh := w.WindowSize()
	theme := gui.CurrentTheme()

	return gui.Column(gui.ContainerCfg{
		Width:   float32(ww),
		Height:  float32(wh),
		Sizing:  gui.FixedFixed,
		HAlign:  gui.HAlignCenter,
		Padding: gui.SomeP(40, 0, 0, 0),
		Spacing: gui.Some[float32](20),
		Content: []gui.View{
			// Header: Score and Best
			gui.Row(gui.ContainerCfg{
				Width:  gridPx,
				Sizing: gui.FixedFit,
				VAlign: gui.VAlignMiddle,
				Content: []gui.View{
					gui.Column(gui.ContainerCfg{
						HAlign: gui.HAlignLeft,
						Content: []gui.View{
							gui.Text(gui.TextCfg{
								Text:      "2048",
								TextStyle: textStyle(theme.B1, 64, gui.White),
							}),
						},
					}),
					gui.Row(gui.ContainerCfg{
						HAlign:  gui.HAlignRight,
						Spacing: gui.Some[float32](10),
						Content: []gui.View{
							scoreBox("SCORE", app.Game.Score),
							scoreBox("BEST", app.BestScore),
						},
					}),
				},
			}),

			// The Game Board
			gui.Canvas(gui.ContainerCfg{
				Width:   gridPx,
				Height:  gridPx,
				Sizing:  gui.FixedFixed,
				Color:   gui.RGB(187, 173, 160),
				Radius:  gui.Some[float32](radiusBoard),
				Padding: gui.NoPadding,
				Content: renderBoard(app.Game),
			}),

			// Footer: Instructions
			gui.Text(gui.TextCfg{
				Text:      "HOW TO PLAY: Use your arrow keys to move the tiles.\nWhen two tiles with the same number touch, they merge into one!",
				TextStyle: textStyle(theme.N1, 14, gui.RGB(150, 150, 160)),
			}),
		},
	})
}

func scoreBox(label string, value int) gui.View {
	theme := gui.CurrentTheme()
	labelStyle := textStyle(theme.B3, 14, gui.RGB(238, 228, 218))
	labelStyle.LetterSpacing = 1.0

	return gui.Column(gui.ContainerCfg{
		MinWidth: 90,
		Height:   65,
		Sizing:   gui.FixedFixed,
		Color:    gui.RGB(187, 173, 160),
		Radius:   gui.Some[float32](4),
		Padding:  gui.SomeP(8, 10, 8, 10),
		VAlign:   gui.VAlignMiddle,
		HAlign:   gui.HAlignCenter,
		Spacing:  gui.Some[float32](4),
		Content: []gui.View{
			gui.Text(gui.TextCfg{
				Text:      label,
				TextStyle: labelStyle,
			}),
			gui.Text(gui.TextCfg{
				Text:      fmt.Sprintf("%d", value),
				TextStyle: textStyle(theme.B2, 24, gui.White),
			}),
		},
	})
}
func renderBoard(g *Game) []gui.View {
	views := []gui.View{}

	// Background grid slots
	for y := range 4 {
		fy := float32(y)
		for x := range 4 {
			fx := float32(x)
			views = append(views, gui.Column(gui.ContainerCfg{
				X:       fx*tilePx + (fx+1)*tileMargin,
				Y:       fy*tilePx + (fy+1)*tileMargin,
				Width:   tilePx,
				Height:  tilePx,
				Sizing:  gui.FixedFixed,
				Color:   gui.RGBA(238, 228, 218, 89),
				Radius:  gui.Some[float32](radiusTile),
				Padding: gui.NoPadding,
			}))
		}
	}

	// Active tiles
	for y := range 4 {
		for x := range 4 {
			val := g.Grid[y][x]
			if val == 0 {
				continue
			}

			views = append(views, renderTile(x, y, val))
		}
	}

	// Overlays
	switch g.State {
	case StateGameOver:
		views = append(views, gameOverlay("GAME OVER!"))
	case StateWin:
		views = append(views, gameOverlay("YOU WIN!"))
	}

	return views
}

func renderTile(x, y, val int) gui.View {
	theme := gui.CurrentTheme()
	bg, fg := tileColors(val)
	fontSize := float32(32)
	if val >= 1024 {
		fontSize = 24
	} else if val >= 100 {
		fontSize = 28
	}

	fx, fy := float32(x), float32(y)

	return gui.Column(gui.ContainerCfg{
		ID:      fmt.Sprintf("tile-%d-%d", x, y),
		Hero:    true,
		X:       fx*tilePx + (fx+1)*tileMargin,
		Y:       fy*tilePx + (fy+1)*tileMargin,
		Width:   tilePx,
		Height:  tilePx,
		Sizing:  gui.FixedFixed,
		Color:   bg,
		Radius:  gui.Some[float32](radiusTile),
		Padding: gui.NoPadding,
		VAlign:  gui.VAlignMiddle,
		HAlign:  gui.HAlignCenter,
		Content: []gui.View{
			gui.Text(gui.TextCfg{
				Text:      fmt.Sprintf("%d", val),
				TextStyle: textStyle(theme.B1, fontSize, fg),
			}),
		},
	})
}

func gameOverlay(msg string) gui.View {
	theme := gui.CurrentTheme()
	return gui.Column(gui.ContainerCfg{
		Width:   gridPx,
		Height:  gridPx,
		Sizing:  gui.FixedFixed,
		Color:   gui.RGBA(238, 228, 218, 150),
		Radius:  gui.Some[float32](radiusBoard),
		Padding: gui.NoPadding,
		VAlign:  gui.VAlignMiddle,
		HAlign:  gui.HAlignCenter,
		Spacing: gui.Some[float32](20),
		Content: []gui.View{
			gui.Text(gui.TextCfg{
				Text:      msg,
				TextStyle: textStyle(theme.B1, 48, gui.RGB(119, 110, 101)),
			}),
			gui.Button(gui.ButtonCfg{
				IDFocus: 10,
				Color:   gui.RGB(143, 122, 102),
				Padding: gui.SomeP(12, 24, 12, 24),
				Content: []gui.View{
					gui.Text(gui.TextCfg{
						Text:      "TRY AGAIN",
						TextStyle: textStyle(theme.B2, 18, gui.White),
					}),
				},
				OnClick: func(_ *gui.Layout, e *gui.Event, w *gui.Window) {
					gui.State[App](w).Game.Reset()
					e.IsHandled = true
				},
			}),
		},
	})
}

func tileColors(val int) (gui.Color, gui.Color) {
	fgLight := gui.RGB(249, 246, 242)
	fgDark := gui.RGB(119, 110, 101)

	switch val {
	case 2:
		return gui.RGB(238, 228, 218), fgDark
	case 4:
		return gui.RGB(237, 224, 200), fgDark
	case 8:
		return gui.RGB(242, 177, 121), fgLight
	case 16:
		return gui.RGB(245, 149, 99), fgLight
	case 32:
		return gui.RGB(246, 124, 95), fgLight
	case 64:
		return gui.RGB(246, 94, 59), fgLight
	case 128:
		return gui.RGB(237, 207, 114), fgLight
	case 256:
		return gui.RGB(237, 204, 97), fgLight
	case 512:
		return gui.RGB(237, 200, 80), fgLight
	case 1024:
		return gui.RGB(237, 197, 63), fgLight
	case 2048:
		return gui.RGB(237, 194, 46), fgLight
	default:
		return gui.RGB(60, 58, 50), fgLight
	}
}

func textStyle(base gui.TextStyle, size float32, color gui.Color) gui.TextStyle {
	s := base
	s.Size = size
	s.Color = color
	return s
}
