// The snake example is a small game built on go-gui state,
// input events, and animations.
package main

import (
	"fmt"
	"time"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend"
)

const (
	gridWidth      = 20
	gridHeight     = 20
	cellSize       = 18
	tickMs         = 120
	tickAnimation  = "snake-tick"
	paddingOuter   = 16
	controlsIDBase = 100
	startButtonID  = 1
)

type App struct {
	Game         *Game
	Started      bool
	LandingFrame int
}

func main() {
	gui.SetTheme(gui.ThemeDark.WithBorders(true))

	w := gui.NewWindow(gui.WindowCfg{
		State:     &App{Game: NewGame(gridWidth, gridHeight, nil)},
		Title:     "Snake",
		Width:     560,
		Height:    640,
		FixedSize: true,
		OnInit: func(w *gui.Window) {
			w.UpdateView(mainView)
			// Advance the game on a repeating animation timer.
			startGameLoop(w)
		},
		OnEvent: handleKeyEvent,
	})

	backend.Run(w)
}

func handleKeyEvent(e *gui.Event, w *gui.Window) {
	if e.Type != gui.EventKeyDown {
		return
	}
	app := gui.State[App](w)
	g := app.Game

	if !app.Started {
		switch e.KeyCode {
		case gui.KeySpace, gui.KeyEnter:
			// Landing screen uses the same controls as the running game.
			startGame(w)
			e.IsHandled = true
		}
		return
	}

	switch e.KeyCode {
	case gui.KeyUp, gui.KeyW:
		g.SetDirection(DirUp)
		e.IsHandled = true
	case gui.KeyRight, gui.KeyD:
		g.SetDirection(DirRight)
		e.IsHandled = true
	case gui.KeyDown, gui.KeyS:
		g.SetDirection(DirDown)
		e.IsHandled = true
	case gui.KeyLeft, gui.KeyA:
		g.SetDirection(DirLeft)
		e.IsHandled = true
	case gui.KeySpace, gui.KeyP:
		g.TogglePause()
		e.IsHandled = true
	case gui.KeyR, gui.KeyEnter:
		g.Reset()
		e.IsHandled = true
	}
}

func startGameLoop(w *gui.Window) {
	w.AnimationAdd(&gui.Animate{
		AnimID: tickAnimation,
		Delay:  tickMs * time.Millisecond,
		Repeat: true,
		Callback: func(_ *gui.Animate, w *gui.Window) {
			app := gui.State[App](w)
			app.LandingFrame++
			if app.Started {
				app.Game.Tick()
			}
		},
	})
}

func startGame(w *gui.Window) {
	app := gui.State[App](w)
	app.Game.Reset()
	app.Started = true
}

func mainView(w *gui.Window) gui.View {
	ww, wh := w.WindowSize()
	app := gui.State[App](w)
	if !app.Started {
		return landingView(w, float32(ww), float32(wh))
	}
	g := app.Game
	theme := gui.CurrentTheme()

	status := "Running"
	if g.Paused {
		status = "Paused"
	}
	if g.Won {
		status = "You won - press R or Restart"
	} else if g.GameOver {
		status = "Game over - press R or Restart"
	}

	return gui.Column(gui.ContainerCfg{
		Width:   float32(ww),
		Height:  float32(wh),
		Sizing:  gui.FixedFixed,
		HAlign:  gui.HAlignCenter,
		VAlign:  gui.VAlignMiddle,
		Spacing: gui.Some[float32](10),
		Padding: gui.SomeP(paddingOuter, paddingOuter, paddingOuter, paddingOuter),
		Content: []gui.View{
			gui.Text(gui.TextCfg{Text: "Snake", TextStyle: theme.B2}),
			gui.Text(gui.TextCfg{Text: fmt.Sprintf("Score: %d", g.Score), TextStyle: theme.B3}),
			gui.Text(gui.TextCfg{Text: status, TextStyle: theme.M3}),
			renderGrid(g),
			renderControls(g),
			gui.Text(gui.TextCfg{Text: "Controls: Arrow keys or WASD. Space/P pauses. R restarts.", TextStyle: theme.M4}),
		},
	})
}

func landingView(w *gui.Window, ww, wh float32) gui.View {
	app := gui.State[App](w)
	theme := gui.CurrentTheme()
	frame := app.LandingFrame

	return gui.Canvas(gui.ContainerCfg{
		Width:   ww,
		Height:  wh,
		Sizing:  gui.FixedFixed,
		Color:   gui.RGB(6, 10, 20),
		Padding: gui.NoPadding,
		Content: []gui.View{
			landingBackdrop(ww, wh, frame),
			gui.Column(gui.ContainerCfg{
				X:       0,
				Y:       0,
				Width:   ww,
				Height:  wh,
				Sizing:  gui.FixedFixed,
				HAlign:  gui.HAlignCenter,
				VAlign:  gui.VAlignMiddle,
				Spacing: gui.Some[float32](14),
				Padding: gui.SomeP(24, 24, 24, 24),
				Content: []gui.View{
					gui.Column(gui.ContainerCfg{
						Width:   420,
						Height:  360,
						Sizing:  gui.FixedFixed,
						HAlign:  gui.HAlignCenter,
						VAlign:  gui.VAlignMiddle,
						Spacing: gui.Some[float32](10),
						Padding: gui.SomeP(22, 24, 22, 24),
						Radius:  gui.Some[float32](14),
						Content: []gui.View{
							tileTitleView(frame),
							gui.Text(gui.TextCfg{
								Text:      "ARCADE SECTOR 1983",
								TextStyle: textStyle(theme.M3, 18, gui.RGB(72, 198, 255)),
							}),
							gui.Text(gui.TextCfg{
								Text:      "Dodge the void. Eat the cores. Chase the high score.",
								TextStyle: textStyle(theme.B4, 16, gui.RGB(235, 240, 255)),
							}),
							gui.Text(gui.TextCfg{
								Text:      "Arrow keys or WASD to steer",
								TextStyle: textStyle(theme.M4, 14, gui.RGB(178, 191, 222)),
							}),
							gui.Text(gui.TextCfg{
								Text:      "Space to P pause",
								TextStyle: textStyle(theme.M4, 14, gui.RGB(178, 191, 222)),
							}),
							gui.Button(gui.ButtonCfg{
								IDFocus:     startButtonID,
								MinWidth:    230,
								Color:       gui.RGB(255, 110, 61),
								ColorHover:  gui.RGB(255, 135, 90),
								ColorFocus:  gui.RGB(255, 135, 90),
								ColorClick:  gui.RGB(230, 90, 42),
								ColorBorder: gui.RGB(255, 211, 92),
								SizeBorder:  gui.Some[float32](2),
								Padding:     gui.SomeP(12, 18, 12, 18),
								Content: []gui.View{
									gui.Text(gui.TextCfg{
										Text:      gui.IconGamepad + "  PRESS TO BEGIN",
										TextStyle: textStyle(theme.B3, 18, gui.White),
									}),
								},
								OnClick: func(_ *gui.Layout, e *gui.Event, w *gui.Window) {
									startGame(w)
									e.IsHandled = true
								},
							}),
						},
					}),
				},
			}),
		},
	})
}

func landingBackdrop(ww, wh float32, _ int) gui.View {
	content := []gui.View{
		gui.Column(gui.ContainerCfg{
			X:       32,
			Y:       44,
			Width:   ww - 64,
			Height:  wh - 88,
			Sizing:  gui.FixedFixed,
			Color:   gui.RGBA(13, 16, 31, 180),
			Padding: gui.NoPadding,
		}),
	}

	type star struct {
		x    float32
		y    float32
		size float32
	}
	stars := []star{
		{28, 30, 4}, {90, 72, 3}, {150, 40, 5}, {470, 50, 4}, {510, 100, 3},
		{60, 160, 3}, {455, 170, 4}, {520, 230, 3}, {38, 280, 5}, {500, 330, 3},
		{80, 410, 4}, {450, 460, 5}, {130, 520, 3}, {515, 560, 4},
	}
	for _, s := range stars {
		content = append(content, gui.Column(gui.ContainerCfg{
			X:       s.x,
			Y:       s.y,
			Width:   s.size,
			Height:  s.size,
			Sizing:  gui.FixedFixed,
			Color:   gui.RGB(255, 211, 92),
			Padding: gui.NoPadding,
		}))
	}

	type decoIcon struct {
		x     float32
		y     float32
		size  float32
		color gui.Color
		text  string
	}
	icons := []decoIcon{
		{44, 22, 40, gui.RGB(72, 198, 255), gui.IconRocket},
		{458, 22, 40, gui.RGB(255, 110, 61), gui.IconRocket},
		{62, 150, 24, gui.RGB(255, 110, 61), gui.IconStar},
		{480, 180, 24, gui.RGB(131, 255, 123), gui.IconStarO},
		{74, 526, 28, gui.RGB(72, 198, 255), gui.IconGamepad},
		{462, 526, 28, gui.RGB(255, 110, 61), gui.IconKeyboard},
	}
	for _, icon := range icons {
		content = append(content, gui.Column(gui.ContainerCfg{
			X:       icon.x,
			Y:       icon.y,
			Width:   40,
			Height:  40,
			Sizing:  gui.FixedFixed,
			Padding: gui.NoPadding,
			Content: []gui.View{
				gui.Text(gui.TextCfg{
					Text:      icon.text,
					TextStyle: textStyle(gui.CurrentTheme().Icon2, icon.size, icon.color),
				}),
			},
		}))
	}

	return gui.Canvas(gui.ContainerCfg{
		Width:   ww,
		Height:  wh,
		Sizing:  gui.FixedFixed,
		Padding: gui.NoPadding,
		Content: content,
	})
}

func renderGrid(g *Game) gui.View {
	boardWidth := float32(gridWidth * cellSize)
	boardHeight := float32(gridHeight * cellSize)

	return gui.Canvas(gui.ContainerCfg{
		Sizing:      gui.FixedFixed,
		Width:       boardWidth,
		Height:      boardHeight,
		Padding:     gui.NoPadding,
		Color:       gui.RGB(20, 24, 28),
		ColorBorder: gui.RGB(75, 80, 85),
		SizeBorder:  gui.Some[float32](1),
		Content:     gridCells(g),
	})
}

func gridCells(g *Game) []gui.View {
	cells := make([]gui.View, 0, gridWidth*gridHeight)
	body := make(map[Point]struct{}, len(g.Snake))
	for _, p := range g.Snake {
		body[p] = struct{}{}
	}
	head := g.Snake[0]

	for y := range gridHeight {
		for x := range gridWidth {
			p := Point{X: x, Y: y}
			color := gui.RGB(30, 35, 42)
			if p == g.Food {
				color = gui.RGB(216, 70, 70)
			} else if p == head {
				color = gui.RGB(120, 214, 101)
			} else if _, ok := body[p]; ok {
				color = gui.RGB(74, 171, 57)
			}

			cells = append(cells, gui.Column(gui.ContainerCfg{
				X:       float32(x*cellSize + 1),
				Y:       float32(y*cellSize + 1),
				Width:   float32(cellSize - 2),
				Height:  float32(cellSize - 2),
				Sizing:  gui.FixedFixed,
				Color:   color,
				Padding: gui.NoPadding,
			}))
		}
	}

	return cells
}

func renderControls(g *Game) gui.View {
	pauseLabel := "Pause"
	if g.Paused {
		pauseLabel = "Resume"
	}

	return gui.Column(gui.ContainerCfg{
		HAlign:  gui.HAlignCenter,
		Spacing: gui.Some[float32](8),
		Content: []gui.View{
			controlButton("Up", controlsIDBase+1, func(g *Game) { g.SetDirection(DirUp) }),
			gui.Row(gui.ContainerCfg{
				HAlign:  gui.HAlignCenter,
				Spacing: gui.Some[float32](8),
				Content: []gui.View{
					controlButton("Left", controlsIDBase+2, func(g *Game) { g.SetDirection(DirLeft) }),
					controlButton("Down", controlsIDBase+3, func(g *Game) { g.SetDirection(DirDown) }),
					controlButton("Right", controlsIDBase+4, func(g *Game) { g.SetDirection(DirRight) }),
				},
			}),
			gui.Row(gui.ContainerCfg{
				HAlign:  gui.HAlignCenter,
				Spacing: gui.Some[float32](8),
				Content: []gui.View{
					controlButton(pauseLabel, controlsIDBase+5, func(g *Game) { g.TogglePause() }),
					controlButton("Restart", controlsIDBase+6, func(g *Game) { g.Reset() }),
				},
			}),
		},
	})
}

func controlButton(label string, id uint32, action func(*Game)) gui.View {
	return gui.Button(gui.ButtonCfg{
		IDFocus:  id,
		MinWidth: 72,
		Content: []gui.View{
			gui.Text(gui.TextCfg{Text: label}),
		},
		OnClick: func(_ *gui.Layout, e *gui.Event, w *gui.Window) {
			action(gui.State[App](w).Game)
			e.IsHandled = true
		},
	})
}

func textStyle(base gui.TextStyle, size float32, color gui.Color) gui.TextStyle {
	base.Size = size
	base.Color = color
	return base
}

func tileTitleView(frame int) gui.View {
	letters := []struct {
		x      int
		pixels []Point
	}{
		{0, titleLetterS()},
		{6, titleLetterN()},
		{12, titleLetterA()},
		{18, titleLetterK()},
		{24, titleLetterE()},
	}

	blocks := make([]gui.View, 0, 96)
	offsetY := float32(0)
	switch frame % 6 {
	case 2, 5:
		offsetY = 1
	case 3, 4:
		offsetY = 2
	}
	for _, letter := range letters {
		for _, p := range letter.pixels {
			color := gui.RGB(74, 171, 57)
			if p.Y == 0 || p.X%2 == 0 {
				color = gui.RGB(120, 214, 101)
			}
			if (frame+p.X+letter.x)%10 < 2 {
				color = gui.RGB(138, 226, 118)
			}
			blocks = append(blocks, gui.Column(gui.ContainerCfg{
				X:       float32((letter.x + p.X) * 10),
				Y:       float32(p.Y*10) + offsetY,
				Width:   8,
				Height:  8,
				Sizing:  gui.FixedFixed,
				Color:   color,
				Padding: gui.NoPadding,
			}))
		}
	}

	return gui.Canvas(gui.ContainerCfg{
		Width:   290,
		Height:  54,
		Sizing:  gui.FixedFixed,
		Padding: gui.NoPadding,
		Content: blocks,
	})
}

func titleLetterS() []Point {
	return []Point{
		{0, 0}, {1, 0}, {2, 0}, {3, 0},
		{0, 1},
		{0, 2}, {1, 2}, {2, 2}, {3, 2},
		{3, 3},
		{0, 4}, {1, 4}, {2, 4}, {3, 4},
	}
}

func titleLetterN() []Point {
	return []Point{
		{0, 0}, {3, 0},
		{0, 1}, {1, 1}, {3, 1},
		{0, 2}, {2, 2}, {3, 2},
		{0, 3}, {3, 3},
		{0, 4}, {3, 4},
	}
}

func titleLetterA() []Point {
	return []Point{
		{1, 0}, {2, 0},
		{0, 1}, {3, 1},
		{0, 2}, {1, 2}, {2, 2}, {3, 2},
		{0, 3}, {3, 3},
		{0, 4}, {3, 4},
	}
}

func titleLetterK() []Point {
	return []Point{
		{0, 0}, {3, 0},
		{0, 1}, {2, 1},
		{0, 2}, {1, 2},
		{0, 3}, {2, 3},
		{0, 4}, {3, 4},
	}
}

func titleLetterE() []Point {
	return []Point{
		{0, 0}, {1, 0}, {2, 0}, {3, 0},
		{0, 1},
		{0, 2}, {1, 2}, {2, 2},
		{0, 3},
		{0, 4}, {1, 4}, {2, 4}, {3, 4},
	}
}
