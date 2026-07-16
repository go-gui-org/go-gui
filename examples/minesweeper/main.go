// Minesweeper example — a complete game with no-guess mode, CSP
// solver, hints, board check, training mode, and garden theme.
package main

import (
	"fmt"
	"math/rand/v2"
	"time"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend"
)

// Screen identifies the active view.
type Screen uint8

const (
	ScreenLanding Screen = iota
	ScreenPlaying
)

// App holds all mutable application state.
type App struct {
	TimerStart time.Time
	Game       *Game
	HintCell   *Point
	BadChecks  []Point

	// Logo mini-game state
	LogoDots     []logoDot // one per pixel in "MINES"
	ElapsedSecs  int
	LandingFrame int

	LogoClicked  int // how many dots clicked so far
	LogoPause    int // frames to pause after boom before reset
	Screen       Screen
	Difficulty   Difficulty
	NoGuessMode  bool
	TrainingMode bool
	GardenTheme  bool
	TimerActive  bool
	LogoBoom     bool // true = bomb hit, showing all bombs
}

// logoDot tracks a single pixel in the title animation.
type logoDot struct {
	Mine     bool
	Revealed bool // true = clicked
}

const timerAnim = "minesweeper-timer"

func main() {
	gui.SetTheme(gui.ThemeDark.WithBorders(true))
	rows, cols, mines := DiffBeginner.Config()

	w := gui.NewWindow(gui.WindowCfg{
		State: &App{
			Game:       NewGame(rows, cols, mines, false, nil),
			Difficulty: DiffBeginner,
		},
		Title:     "Minesweeper",
		Width:     740,
		Height:    660,
		FixedSize: true,
		OnInit: func(w *gui.Window) {
			w.UpdateView(mainView)
			w.AnimationAdd(&gui.Animate{
				AnimID: timerAnim,
				Delay:  time.Second,
				Repeat: true,
				Callback: func(_ *gui.Animate, w *gui.Window) {
					app := gui.State[App](w)
					app.LandingFrame++
					if app.Screen == ScreenLanding {
						logoTick(app)
					}
					if app.TimerActive &&
						app.Game.State == GamePlaying {
						app.ElapsedSecs = int(
							time.Since(app.TimerStart).Seconds())
					}
				},
			})
		},
		OnEvent: handleEvent,
	})

	backend.Run(w)
}

// --- Event handling ---

func handleEvent(e *gui.Event, w *gui.Window) {
	if e.Type != gui.EventKeyDown {
		return
	}
	app := gui.State[App](w)
	if app.Screen == ScreenLanding {
		return
	}
	switch e.KeyCode {
	case gui.KeyR:
		resetGame(app)
		e.IsHandled = true
	case gui.KeyH:
		showHint(app)
		e.IsHandled = true
	case gui.KeyC:
		app.BadChecks = app.Game.CheckBoard()
		e.IsHandled = true
	case gui.KeyEscape:
		app.Screen = ScreenLanding
		app.TimerActive = false
		e.IsHandled = true
	}
}

func resetGame(app *App) {
	app.Game.Reset()
	app.ElapsedSecs = 0
	app.TimerActive = false
	app.HintCell = nil
	app.BadChecks = nil
}

func showHint(app *App) {
	if app.Game.State != GamePlaying || app.Game.FirstClick {
		return
	}
	app.HintCell = FindHint(app.Game)
}

func startNewGame(app *App, diff Difficulty) {
	rows, cols, mines := diff.Config()
	app.Difficulty = diff
	app.Game = NewGame(rows, cols, mines, app.NoGuessMode, nil)
	app.Screen = ScreenPlaying
	app.ElapsedSecs = 0
	app.TimerActive = false
	app.HintCell = nil
	app.BadChecks = nil
}

// --- View routing ---

func mainView(w *gui.Window) gui.View {
	app := gui.State[App](w)
	ww, wh := w.WindowSize()
	if app.Screen == ScreenPlaying {
		return gameView(w, float32(ww), float32(wh))
	}
	return landingView(w, float32(ww), float32(wh))
}

// --- Color palette ---

var (
	colorBG         = gui.RGB(6, 10, 20)
	colorNeonGreen  = gui.RGB(57, 255, 20)
	colorNeonPink   = gui.RGB(255, 16, 240)
	colorNeonCyan   = gui.RGB(0, 255, 255)
	colorNeonYellow = gui.RGB(255, 255, 0)
	colorArcadeRed  = gui.RGB(255, 60, 60)
	colorDimText    = gui.RGB(178, 191, 222)
)

var digitStr = [9]string{"0", "1", "2", "3", "4", "5", "6", "7", "8"}

var numberColors = [9]gui.Color{
	{},                     // 0: unused
	gui.RGB(55, 120, 255),  // 1: blue
	gui.RGB(40, 170, 40),   // 2: green
	gui.RGB(230, 40, 40),   // 3: red
	gui.RGB(30, 30, 160),   // 4: dark blue
	gui.RGB(160, 30, 30),   // 5: maroon
	gui.RGB(0, 160, 160),   // 6: teal
	gui.RGB(80, 80, 80),    // 7: dark gray
	gui.RGB(150, 150, 150), // 8: gray
}

// --- Landing page ---

func landingView(w *gui.Window, ww, wh float32) gui.View {
	app := gui.State[App](w)
	theme := gui.CurrentTheme()
	return gui.Column(gui.ContainerCfg{
		Width:   ww,
		Height:  wh,
		Sizing:  gui.FixedFixed,
		Color:   colorBG,
		HAlign:  gui.HAlignCenter,
		VAlign:  gui.VAlignMiddle,
		Spacing: gui.SomeF(14),
		Padding: gui.SomeP(24, 24, 24, 24),
		Content: landingContent(w, app, theme),
	})
}

func landingContent(w *gui.Window, app *App, theme gui.Theme) []gui.View {
	return []gui.View{
		tileTitleView(app),
		gui.Text(gui.TextCfg{
			Text:      "SWEEPER",
			TextStyle: ts(theme.B2, 28, colorNeonPink),
		}),
		gui.Text(gui.TextCfg{
			Text:      "ARCADE SECTOR 1983",
			TextStyle: ts(theme.M3, 16, colorNeonCyan),
		}),
		gui.Row(gui.ContainerCfg{
			HAlign: gui.HAlignCenter, Spacing: gui.SomeF(10),
			SizeBorder: gui.NoBorder,
			Content: []gui.View{
				diffButton(w, "BEGINNER", "9\u00d79", DiffBeginner, colorNeonGreen),
				diffButton(w, "INTER", "16\u00d716", DiffIntermediate, colorNeonYellow),
				diffButton(w, "EXPERT", "30\u00d716", DiffExpert, colorArcadeRed),
			},
		}),
		gui.Row(gui.ContainerCfg{
			HAlign: gui.HAlignCenter, Spacing: gui.SomeF(20),
			SizeBorder: gui.NoBorder,
			Content: []gui.View{
				gui.Switch(gui.SwitchCfg{
					Selected:  app.NoGuessMode,
					Label:     "No-Guess",
					TextStyle: ts(theme.M4, 14, colorDimText),
					OnClick: func(_ *gui.Layout, e *gui.Event, w *gui.Window) {
						gui.State[App](w).NoGuessMode =
							!gui.State[App](w).NoGuessMode
						e.IsHandled = true
					},
				}),
				gui.Switch(gui.SwitchCfg{
					Selected:  app.TrainingMode,
					Label:     "Training",
					TextStyle: ts(theme.M4, 14, colorDimText),
					OnClick: func(_ *gui.Layout, e *gui.Event, w *gui.Window) {
						gui.State[App](w).TrainingMode =
							!gui.State[App](w).TrainingMode
						e.IsHandled = true
					},
				}),
				gui.Switch(gui.SwitchCfg{
					Selected:  app.GardenTheme,
					Label:     "Garden",
					TextStyle: ts(theme.M4, 14, colorDimText),
					OnClick: func(_ *gui.Layout, e *gui.Event, w *gui.Window) {
						gui.State[App](w).GardenTheme =
							!gui.State[App](w).GardenTheme
						e.IsHandled = true
					},
				}),
			},
		}),
		gui.Text(gui.TextCfg{
			Text: "Left-click: reveal  Right-click: flag  " +
				"H: hint  C: check",
			TextStyle: ts(theme.M4, 12, colorDimText),
		}),
		gui.Text(gui.TextCfg{
			Text:      "R: reset  Esc: menu",
			TextStyle: ts(theme.M4, 12, colorDimText),
		}),
	}
}

func diffButton(w *gui.Window, title, subtitle string, diff Difficulty, color gui.Color) gui.View {
	theme := gui.CurrentTheme()
	return gui.Button(gui.ButtonCfg{
		// title is a distinct literal at every call site, so it yields a
		// unique, frame-stable focus ID without widening the signature.
		ID:          "mine_diff_" + title,
		MinWidth:    130,
		Color:       color.WithOpacity(0.15),
		ColorHover:  color.WithOpacity(0.3),
		ColorClick:  color.WithOpacity(0.5),
		ColorBorder: color,
		SizeBorder:  gui.SomeF(2),
		Padding:     gui.SomeP(12, 18, 12, 18),
		Content: []gui.View{
			gui.Text(gui.TextCfg{
				Text:      title,
				TextStyle: ts(theme.B3, 16, color),
			}),
			gui.Text(gui.TextCfg{
				Text:      subtitle,
				TextStyle: ts(theme.M4, 14, color),
			}),
		},
		OnClick: func(_ *gui.Layout, e *gui.Event, w *gui.Window) {
			startNewGame(gui.State[App](w), diff)
			e.IsHandled = true
		},
	})
}

// --- Pixel art title "MINES" with minesweeper animation ---

// logoPixels is the cached set of pixel positions for the "MINES" title.
var logoPixels = buildLogoPixels()

func buildLogoPixels() []Point {
	type letter struct {
		xOff   int
		pixels []Point
	}
	letters := []letter{
		{0, letterM()},
		{7, letterI()},
		{12, letterN()},
		{18, letterE()},
		{24, letterS()},
	}
	var out []Point
	for _, l := range letters {
		for _, p := range l.pixels {
			out = append(out, Point{p.Row, l.xOff + p.Col})
		}
	}
	return out
}

const logoBombs = 5

// logoReset randomises the bomb layout for the title animation.
func logoReset(app *App) {
	pixels := logoPixels
	n := len(pixels)
	app.LogoDots = make([]logoDot, n)
	app.LogoClicked = 0
	app.LogoBoom = false
	app.LogoPause = 0

	// Place bombs randomly.
	rng := rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64()))
	placed := 0
	for placed < logoBombs && placed < n {
		i := rng.IntN(n)
		if app.LogoDots[i].Mine {
			continue
		}
		app.LogoDots[i].Mine = true
		placed++
	}
}

// logoTick advances the title animation by one step (called each
// second via the timer animation).
func logoTick(app *App) {
	if app.LogoDots == nil {
		logoReset(app)
		return
	}

	// Pulsate after boom for 5 seconds, then reset.
	if app.LogoBoom {
		app.LogoPause++
		if app.LogoPause >= 5 {
			logoReset(app)
		}
		return
	}

	// Pick the next unrevealed dot to "click".
	n := len(app.LogoDots)
	unrevealed := make([]int, 0, n-app.LogoClicked)
	for i := range app.LogoDots {
		if !app.LogoDots[i].Revealed {
			unrevealed = append(unrevealed, i)
		}
	}
	if len(unrevealed) == 0 {
		logoReset(app)
		return
	}
	pick := unrevealed[rand.IntN(len(unrevealed))]
	app.LogoDots[pick].Revealed = true
	app.LogoClicked++

	if app.LogoDots[pick].Mine {
		// Boom — reveal all bombs.
		app.LogoBoom = true
		for i := range app.LogoDots {
			if app.LogoDots[i].Mine {
				app.LogoDots[i].Revealed = true
			}
		}
	}
}

func tileTitleView(app *App) gui.View {
	if app.LogoDots == nil {
		logoReset(app)
	}
	pixels := logoPixels

	blocks := make([]gui.View, 0, len(pixels))
	// During boom, pulsate bombs on/off each second.
	bombVisible := !app.LogoBoom || app.LogoPause%2 == 0
	for i, p := range pixels {
		dot := app.LogoDots[i]
		color := colorNeonGreen
		if dot.Revealed {
			if dot.Mine {
				if bombVisible {
					color = colorArcadeRed
				} else {
					color = gui.RGB(60, 10, 10)
				}
			} else {
				color = colorBG
			}
		}
		blocks = append(blocks, gui.Column(gui.ContainerCfg{
			X: float32(p.Col) * 10, Y: float32(p.Row) * 10,
			Width: 8, Height: 8,
			Sizing:     gui.FixedFixed,
			Color:      color,
			Padding:    gui.NoPadding,
			SizeBorder: gui.NoBorder,
		}))
	}
	return gui.Canvas(gui.ContainerCfg{
		Width: 280, Height: 54, Sizing: gui.FixedFixed,
		Padding: gui.NoPadding, SizeBorder: gui.NoBorder,
		Content: blocks,
	})
}

func letterM() []Point {
	return []Point{
		{0, 0}, {0, 4},
		{1, 0}, {1, 1}, {1, 3}, {1, 4},
		{2, 0}, {2, 2}, {2, 4},
		{3, 0}, {3, 4},
		{4, 0}, {4, 4},
	}
}

func letterI() []Point {
	return []Point{
		{0, 0}, {0, 1}, {0, 2},
		{1, 1},
		{2, 1},
		{3, 1},
		{4, 0}, {4, 1}, {4, 2},
	}
}

func letterN() []Point {
	return []Point{
		{0, 0}, {0, 3},
		{1, 0}, {1, 1}, {1, 3},
		{2, 0}, {2, 2}, {2, 3},
		{3, 0}, {3, 3},
		{4, 0}, {4, 3},
	}
}

func letterE() []Point {
	return []Point{
		{0, 0}, {0, 1}, {0, 2}, {0, 3},
		{1, 0},
		{2, 0}, {2, 1}, {2, 2},
		{3, 0},
		{4, 0}, {4, 1}, {4, 2}, {4, 3},
	}
}

func letterS() []Point {
	return []Point{
		{0, 0}, {0, 1}, {0, 2}, {0, 3},
		{1, 0},
		{2, 0}, {2, 1}, {2, 2}, {2, 3},
		{3, 3},
		{4, 0}, {4, 1}, {4, 2}, {4, 3},
	}
}

// --- Game view ---

func cellPxSize(d Difficulty) float32 {
	switch d {
	case DiffIntermediate:
		return 28
	case DiffExpert:
		return 22
	default:
		return 36
	}
}

func gameView(w *gui.Window, ww, wh float32) gui.View {
	app := gui.State[App](w)
	g := app.Game
	theme := gui.CurrentTheme()
	cellPx := cellPxSize(app.Difficulty)
	boardW := float32(g.Cols) * cellPx
	boardH := float32(g.Rows) * cellPx

	var training *SolverResult
	if app.TrainingMode && g.State == GamePlaying && !g.FirstClick {
		// Training uses ground truth to highlight every frontier
		// cell as safe (green) or mine (red).
		training = &SolverResult{
			Safe:  frontierCells(g, false),
			Mines: frontierCells(g, true),
		}
	}

	return gui.Column(gui.ContainerCfg{
		Width: ww, Height: wh, Sizing: gui.FixedFixed,
		HAlign: gui.HAlignCenter, VAlign: gui.VAlignMiddle,
		Spacing: gui.SomeF(8), SizeBorder: gui.NoBorder,
		Padding: gui.SomeP(12, 16, 12, 16),
		Content: []gui.View{
			headerView(app, theme, boardW),
			boardView(app, g, theme, cellPx, boardW, boardH, training),
			footerView(app, g, theme),
		},
	})
}

// --- Header: mine counter, smiley, timer ---

func headerView(app *App, theme gui.Theme, boardW float32) gui.View {
	g := app.Game
	remaining := g.MineCount - g.FlagsUsed

	smileyIcon := gui.IconSmile
	smileyColor := gui.RGB(255, 200, 0)
	switch g.State {
	case GameLost:
		smileyIcon = gui.IconTarget
		smileyColor = gui.RGB(255, 60, 60)
	case GameWon:
		smileyIcon = gui.IconSmileHeart
		smileyColor = gui.RGB(100, 255, 100)
	}

	return gui.Canvas(gui.ContainerCfg{
		Width: boardW, Height: 44, Sizing: gui.FixedFixed,
		SizeBorder: gui.NoBorder, Padding: gui.NoPadding,
		Content: []gui.View{
			ledDisplay(0, 0, fmt.Sprintf("%03d", remaining), theme),
			gui.Column(gui.ContainerCfg{
				X: boardW/2 - 22, Y: 0,
				Width: 44, Height: 44, Sizing: gui.FixedFixed,
				SizeBorder: gui.NoBorder, Padding: gui.NoPadding,
				HAlign: gui.HAlignCenter, VAlign: gui.VAlignMiddle,
				Content: []gui.View{
					gui.Button(gui.ButtonCfg{
						ID:          "mine_reset",
						Color:       gui.RGB(40, 44, 52),
						ColorHover:  gui.RGB(55, 60, 68),
						ColorClick:  gui.RGB(30, 34, 40),
						ColorBorder: gui.RGB(100, 105, 110),
						SizeBorder:  gui.SomeF(2),
						Padding:     gui.SomeP(4, 4, 4, 4),
						Radius:      gui.SomeF(6),
						Content: []gui.View{
							gui.Text(gui.TextCfg{
								Text:      smileyIcon,
								TextStyle: ts(theme.Icon2, 24, smileyColor),
							}),
						},
						OnClick: func(_ *gui.Layout, e *gui.Event, w *gui.Window) {
							resetGame(gui.State[App](w))
							e.IsHandled = true
						},
					}),
				},
			}),
			ledDisplay(boardW-80, 0,
				fmt.Sprintf("%03d", min(app.ElapsedSecs, 999)), theme),
		},
	})
}

func ledDisplay(x, y float32, text string, theme gui.Theme) gui.View {
	return gui.Row(gui.ContainerCfg{
		X: x, Y: y, Width: 80, Height: 40,
		Sizing:      gui.FixedFixed,
		Color:       gui.RGB(20, 0, 0),
		Radius:      gui.SomeF(4),
		SizeBorder:  gui.SomeF(1),
		ColorBorder: gui.RGB(60, 60, 60),
		HAlign:      gui.HAlignCenter,
		VAlign:      gui.VAlignMiddle,
		Padding:     gui.SomeP(4, 8, 4, 8),
		Content: []gui.View{
			gui.Text(gui.TextCfg{
				Text:      text,
				TextStyle: ts(theme.M1, 24, gui.RGB(255, 0, 0)),
			}),
		},
	})
}

// --- Board grid ---

func boardView(app *App, g *Game, theme gui.Theme,
	cellPx, boardW, boardH float32, training *SolverResult) gui.View {

	trainSafe := make(map[Point]bool)
	trainMine := make(map[Point]bool)
	badCheck := make(map[Point]bool)
	if training != nil {
		for _, p := range training.Safe {
			trainSafe[p] = true
		}
		for _, p := range training.Mines {
			trainMine[p] = true
		}
	}
	for _, p := range app.BadChecks {
		badCheck[p] = true
	}

	cells := make([]gui.View, 0, g.Rows*g.Cols)
	for row := range g.Rows {
		for col := range g.Cols {
			cells = append(cells,
				cellView(row, col, g, app, cellPx, theme,
					trainSafe, trainMine, badCheck))
		}
	}

	canvasBg := gui.RGB(90, 95, 100)
	if app.GardenTheme {
		canvasBg = gui.RGB(70, 90, 50)
	}

	return gui.Canvas(gui.ContainerCfg{
		Width: boardW, Height: boardH, Sizing: gui.FixedFixed,
		Padding:     gui.NoPadding,
		Color:       canvasBg,
		ColorBorder: gui.RGB(60, 65, 70),
		SizeBorder:  gui.SomeF(2),
		Content:     cells,
	})
}

// --- Individual cell ---

func cellView(row, col int, g *Game, app *App, cellPx float32,
	theme gui.Theme,
	trainSafe, trainMine, badCheck map[Point]bool) gui.View {

	cell := g.cell(row, col)
	pt := Point{row, col}

	// Theme-dependent icons
	mineIcon := gui.IconTarget
	flagIcon := gui.IconFlag
	flagColor := gui.RGB(220, 40, 40)
	if app.GardenTheme {
		mineIcon = gui.IconStar
		flagIcon = gui.IconHeart
		flagColor = gui.RGB(200, 60, 200)
	}

	// Base colors — classic vs garden
	hiddenBg := gui.RGB(170, 175, 180)
	hiddenBd := gui.RGB(200, 205, 210)
	revealedBg := gui.RGB(195, 200, 205)
	revealedBd := gui.RGB(175, 180, 185)
	if app.GardenTheme {
		hiddenBg = gui.RGB(140, 165, 90)
		hiddenBd = gui.RGB(165, 190, 115)
		revealedBg = gui.RGB(215, 210, 175)
		revealedBd = gui.RGB(195, 190, 155)
	}

	bgColor := hiddenBg
	borderColor := hiddenBd
	var content []gui.View

	if g.State == GameLost {
		bgColor, borderColor, content = cellGameOver(
			cell, mineIcon, flagIcon, flagColor, cellPx, theme,
			hiddenBg, hiddenBd, revealedBg, revealedBd)
	} else {
		switch cell.State {
		case CellRevealed:
			bgColor = revealedBg
			borderColor = revealedBd
			if badCheck[pt] {
				bgColor = gui.RGB(255, 180, 80)
				borderColor = gui.RGB(220, 150, 50)
			}
			if cell.Adjacent > 0 {
				content = numContent(cell.Adjacent, cellPx, theme)
			}
		case CellFlagged:
			content = iconContent(flagIcon, cellPx, flagColor, theme)
		case CellHidden:
			if app.HintCell != nil && *app.HintCell == pt {
				bgColor = gui.RGB(100, 220, 100)
				borderColor = gui.RGB(80, 190, 80)
			}
			if trainSafe[pt] {
				bgColor = gui.RGB(130, 220, 130)
				borderColor = gui.RGB(80, 180, 80)
			}
			if trainMine[pt] {
				bgColor = gui.RGB(220, 130, 130)
				borderColor = gui.RGB(180, 80, 80)
			}
		}
	}

	return gui.Column(gui.ContainerCfg{
		X: float32(col)*cellPx + 1, Y: float32(row)*cellPx + 1,
		Width: cellPx - 2, Height: cellPx - 2,
		Sizing:      gui.FixedFixed,
		Color:       bgColor,
		SizeBorder:  gui.SomeF(1),
		ColorBorder: borderColor,
		Padding:     gui.NoPadding,
		HAlign:      gui.HAlignCenter,
		VAlign:      gui.VAlignMiddle,
		OnAnyClick:  cellClickHandler(row, col),
		Content:     content,
	})
}

func cellGameOver(cell *Cell,
	mineIcon, flagIcon string, flagColor gui.Color,
	cellPx float32, theme gui.Theme,
	hiddenBg, hiddenBd, revealedBg, revealedBd gui.Color,
) (bg, bd gui.Color, content []gui.View) {

	switch {
	case cell.Mine && cell.State == CellRevealed:
		// The mine that ended the game.
		return gui.RGB(220, 50, 50), gui.RGB(180, 40, 40),
			iconContent(mineIcon, cellPx, gui.RGB(30, 30, 30), theme)

	case cell.Mine && cell.State == CellFlagged:
		// Correctly flagged mine.
		return gui.RGB(140, 190, 140), gui.RGB(120, 170, 120),
			iconContent(flagIcon, cellPx, flagColor, theme)

	case cell.Mine:
		// Unflagged mine — show it.
		return revealedBg, revealedBd,
			iconContent(mineIcon, cellPx, gui.RGB(80, 80, 80), theme)

	case !cell.Mine && cell.State == CellFlagged:
		// Incorrectly flagged — show X.
		return gui.RGB(230, 180, 100), gui.RGB(200, 150, 80),
			textContent("\u2717", cellPx, gui.RGB(200, 50, 50), theme)

	case cell.State == CellRevealed:
		bg = revealedBg
		bd = revealedBd
		if cell.Adjacent > 0 {
			content = numContent(cell.Adjacent, cellPx, theme)
		}
		return bg, bd, content

	default:
		return revealedBg, revealedBd, nil
	}
}

func iconContent(icon string, cellPx float32, color gui.Color,
	theme gui.Theme) []gui.View {
	return []gui.View{
		gui.Text(gui.TextCfg{
			Text:      icon,
			TextStyle: ts(theme.Icon3, cellPx*0.55, color),
		}),
	}
}

func textContent(text string, cellPx float32, color gui.Color,
	theme gui.Theme) []gui.View {
	return []gui.View{
		gui.Text(gui.TextCfg{
			Text:      text,
			TextStyle: ts(theme.B3, cellPx*0.65, color),
		}),
	}
}

func numContent(adj int, cellPx float32, theme gui.Theme) []gui.View {
	return []gui.View{
		gui.Text(gui.TextCfg{
			Text:      digitStr[adj],
			TextStyle: ts(theme.B3, cellPx*0.65, numberColors[adj]),
		}),
	}
}

// --- Cell click handler ---

func cellClickHandler(row, col int) func(*gui.Layout, *gui.Event, *gui.Window) {
	return func(_ *gui.Layout, e *gui.Event, w *gui.Window) {
		app := gui.State[App](w)
		g := app.Game
		if g.State != GamePlaying {
			return
		}
		switch e.MouseButton {
		case gui.MouseLeft:
			if !app.TimerActive {
				app.TimerStart = time.Now()
				app.TimerActive = true
			}
			c := g.cell(row, col)
			switch c.State {
			case CellRevealed:
				g.Chord(row, col)
			case CellHidden:
				g.Reveal(row, col)
			}
			switch g.State {
			case GameLost:
				app.TimerActive = false
			case GameWon:
				g.AutoFlagMines()
				app.TimerActive = false
			}
			app.HintCell = nil
			app.BadChecks = nil
		case gui.MouseRight:
			g.Flag(row, col)
			app.HintCell = nil
			app.BadChecks = nil
		}
		e.IsHandled = true
	}
}

// --- Footer: status and buttons ---

func footerView(app *App, g *Game, theme gui.Theme) gui.View {
	var items []gui.View

	switch g.State {
	case GameWon:
		items = append(items, gui.Text(gui.TextCfg{
			Text:      "YOU WIN!",
			TextStyle: ts(theme.B3, 16, gui.RGB(100, 255, 100)),
		}))
	case GameLost:
		items = append(items, gui.Text(gui.TextCfg{
			Text:      "GAME OVER",
			TextStyle: ts(theme.B3, 16, gui.RGB(255, 80, 80)),
		}))
	default:
		if app.NoGuessMode {
			items = append(items, gui.Text(gui.TextCfg{
				Text:      "No-Guess Mode",
				TextStyle: ts(theme.M4, 13, colorNeonCyan),
			}))
		}
		if app.TrainingMode {
			items = append(items, gui.Text(gui.TextCfg{
				Text:      "Training: green=safe  red=mine",
				TextStyle: ts(theme.M4, 12, colorDimText),
			}))
		}
	}

	var buttons []gui.View
	if g.State == GamePlaying {
		buttons = []gui.View{
			smallButton("Hint (H)", func(w *gui.Window) {
				showHint(gui.State[App](w))
			}),
			smallButton("Check (C)", func(w *gui.Window) {
				a := gui.State[App](w)
				a.BadChecks = a.Game.CheckBoard()
			}),
			smallButton("Menu (Esc)", func(w *gui.Window) {
				a := gui.State[App](w)
				a.Screen = ScreenLanding
				a.TimerActive = false
			}),
		}
	} else {
		buttons = []gui.View{
			smallButton("New Game (R)", func(w *gui.Window) {
				resetGame(gui.State[App](w))
			}),
			smallButton("Menu (Esc)", func(w *gui.Window) {
				a := gui.State[App](w)
				a.Screen = ScreenLanding
				a.TimerActive = false
			}),
		}
	}

	items = append(items, gui.Row(gui.ContainerCfg{
		HAlign: gui.HAlignCenter, Spacing: gui.SomeF(8),
		SizeBorder: gui.NoBorder, Content: buttons,
	}))

	return gui.Column(gui.ContainerCfg{
		HAlign: gui.HAlignCenter, Spacing: gui.SomeF(6),
		SizeBorder: gui.NoBorder, Content: items,
	})
}

func smallButton(label string, action func(*gui.Window)) gui.View {
	return gui.Button(gui.ButtonCfg{
		Color:       gui.RGB(45, 50, 58),
		ColorHover:  gui.RGB(60, 66, 74),
		ColorClick:  gui.RGB(35, 40, 48),
		ColorBorder: gui.RGB(90, 95, 100),
		SizeBorder:  gui.SomeF(1),
		Padding:     gui.SomeP(6, 12, 6, 12),
		Content: []gui.View{
			gui.Text(gui.TextCfg{
				Text: label,
				TextStyle: ts(gui.CurrentTheme().M4, 12,
					gui.RGB(200, 205, 210)),
			}),
		},
		OnClick: func(_ *gui.Layout, e *gui.Event, w *gui.Window) {
			action(w)
			e.IsHandled = true
		},
	})
}

// ts creates a TextStyle from a base with overridden size and color.
func ts(base gui.TextStyle, size float32, color gui.Color) gui.TextStyle {
	base.Size = size
	base.Color = color
	return base
}
