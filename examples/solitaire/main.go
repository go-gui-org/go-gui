// Solitaire example — Klondike solitaire with a 1980s arcade
// landing page, drag-and-drop card movement, scoring, and
// right-click auto-complete.
package main

import (
	"fmt"
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

// DragSource records the origin of a drag operation.
type DragSource struct {
	Type    SourceType
	ColIdx  int
	CardIdx int // index within tableau column (for stack drags)
}

// App holds all mutable application state.
type App struct {
	Game         *Game
	DragCards    []Card
	DragSource   DragSource
	LandingFrame int

	DragOffsetX float32
	DragOffsetY float32
	DragMouseX  float32
	DragMouseY  float32
	Screen      Screen

	// Drag state
	DragActive bool
}

const blinkAnim = "solitaire-blink"

func main() {
	gui.SetTheme(gui.ThemeDarkBordered)

	w := gui.NewWindow(gui.WindowCfg{
		State: &App{
			Game: NewGame(DrawOne, nil),
		},
		Title:     "Solitaire",
		Width:     640,
		Height:    740,
		FixedSize: true,
		OnInit: func(w *gui.Window) {
			w.UpdateView(mainView)
			w.AnimationAdd(&gui.Animate{
				AnimID: blinkAnim,
				Delay:  500 * time.Millisecond,
				Repeat: true,
				Callback: func(_ *gui.Animate, w *gui.Window) {
					app := gui.State[App](w)
					app.LandingFrame++
					// Auto-complete check.
					if app.Screen == ScreenPlaying &&
						app.Game.State == StatePlaying &&
						app.Game.AllFaceUp() {
						app.Game.AutoComplete()
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
	switch e.KeyCode {
	case gui.KeyN:
		if app.Screen == ScreenPlaying {
			app.Game.Reset(nil)
			e.IsHandled = true
		}
	case gui.KeyEscape:
		if app.Screen == ScreenPlaying {
			app.Screen = ScreenLanding
			e.IsHandled = true
		}
	}
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
	colorFelt       = gui.RGB(15, 60, 30)
	colorNeonGreen  = gui.RGB(57, 255, 20)
	colorNeonPink   = gui.RGB(255, 16, 240)
	colorNeonCyan   = gui.RGB(0, 255, 255)
	colorNeonYellow = gui.RGB(255, 255, 0)
	colorDimText    = gui.RGB(178, 191, 222)
	colorCardWhite  = gui.RGB(245, 245, 240)
	colorCardBorder = gui.RGB(140, 140, 130)
	colorCardBack   = gui.RGB(40, 12, 60)
	colorCardRed    = gui.RGB(210, 30, 30)
	colorCardBlack  = gui.RGB(30, 30, 30)
	colorSlotBorder = gui.RGB(50, 100, 55)
	colorGold       = gui.RGB(255, 215, 0)
)

// --- Card layout constants ---

const (
	cardW          float32 = 72
	cardH          float32 = 100
	colStride      float32 = 82 // cardW + 10 gap
	boardPadX      float32 = 20
	boardTopY      float32 = 15
	tableauTopY    float32 = 135
	overlapDown    float32 = 22
	overlapUp      float32 = 30
	wasteFanOffset float32 = 18
)

func colX(col int) float32 { return boardPadX + float32(col)*colStride }

// --- Landing page ---

func landingView(w *gui.Window, ww, wh float32) gui.View {
	app := gui.State[App](w)
	theme := gui.CurrentTheme()
	blink := app.LandingFrame%2 == 0

	insertCoinColor := colorNeonYellow
	if !blink {
		insertCoinColor = colorNeonYellow.WithOpacity(0.2)
	}

	return gui.Column(gui.ContainerCfg{
		Width:      ww,
		Height:     wh,
		Sizing:     gui.FixedFixed,
		Color:      colorBG,
		HAlign:     gui.HAlignCenter,
		VAlign:     gui.VAlignMiddle,
		Spacing:    gui.SomeF(16),
		SizeBorder: gui.NoBorder,
		Padding:    gui.SomeP(24, 24, 24, 24),
		Content: []gui.View{
			// Title
			gui.Text(gui.TextCfg{
				Text:      "SOLITAIRE",
				TextStyle: ts(theme.B1, 48, colorNeonCyan),
			}),
			gui.Text(gui.TextCfg{
				Text:      "KLONDIKE",
				TextStyle: ts(theme.B2, 28, colorNeonPink),
			}),
			gui.Text(gui.TextCfg{
				Text:      "ARCADE SECTOR 1983",
				TextStyle: ts(theme.M3, 14, colorNeonCyan.WithOpacity(0.6)),
			}),

			// Separator
			gui.Rectangle(gui.RectangleCfg{
				Width:  300,
				Height: 2,
				Sizing: gui.FixedFixed,
				Color:  colorNeonCyan.WithOpacity(0.3),
			}),

			// Mode buttons
			gui.Row(gui.ContainerCfg{
				HAlign:     gui.HAlignCenter,
				Spacing:    gui.SomeF(12),
				SizeBorder: gui.NoBorder,
				Content: []gui.View{
					modeButton(w, "DRAW 1", DrawOne, colorNeonGreen),
					modeButton(w, "DRAW 3", DrawThree, colorNeonYellow),
				},
			}),

			// Decorative card fan
			gui.Row(gui.ContainerCfg{
				HAlign:     gui.HAlignCenter,
				Spacing:    gui.SomeF(4),
				SizeBorder: gui.NoBorder,
				Content: []gui.View{
					miniCard("A", "♠", colorCardBlack),
					miniCard("K", "♥", colorCardRed),
					miniCard("Q", "♦", colorCardRed),
					miniCard("J", "♣", colorCardBlack),
				},
			}),

			// Insert coin
			gui.Text(gui.TextCfg{
				Text:      "INSERT COIN TO PLAY",
				TextStyle: ts(theme.B3, 18, insertCoinColor),
			}),

			// Controls
			gui.Text(gui.TextCfg{
				Text:      "DRAG CARDS \u2022 CLICK STOCK TO DRAW",
				TextStyle: ts(theme.M4, 12, colorDimText),
			}),
			gui.Text(gui.TextCfg{
				Text:      "RIGHT-CLICK: AUTO-MOVE \u2022 N: NEW \u2022 ESC: MENU",
				TextStyle: ts(theme.M4, 12, colorDimText),
			}),
		},
	})
}

func modeButton(w *gui.Window, title string, mode DrawMode, color gui.Color) gui.View {
	theme := gui.CurrentTheme()
	return gui.Button(gui.ButtonCfg{
		IDFocus:     uint32(mode) + 10,
		MinWidth:    140,
		Color:       color.WithOpacity(0.12),
		ColorHover:  color.WithOpacity(0.3),
		ColorClick:  color.WithOpacity(0.5),
		ColorBorder: color,
		SizeBorder:  gui.SomeF(2),
		Padding:     gui.SomeP(14, 22, 14, 22),
		Content: []gui.View{
			gui.Text(gui.TextCfg{
				Text:      title,
				TextStyle: ts(theme.B3, 18, color),
			}),
		},
		OnClick: func(_ *gui.Layout, e *gui.Event, w *gui.Window) {
			app := gui.State[App](w)
			app.Game = NewGame(mode, nil)
			app.Screen = ScreenPlaying
			e.IsHandled = true
		},
	})
}

// miniCard renders a small decorative card for the landing page.
func miniCard(rank, suit string, color gui.Color) gui.View {
	theme := gui.CurrentTheme()
	return gui.Column(gui.ContainerCfg{
		Width:       36,
		Height:      52,
		Sizing:      gui.FixedFixed,
		Color:       colorCardWhite,
		ColorBorder: colorCardBorder,
		SizeBorder:  gui.SomeF(1),
		Radius:      gui.SomeF(4),
		Padding:     gui.SomeP(2, 3, 2, 3),
		Content: []gui.View{
			gui.Text(gui.TextCfg{
				Text:      rank + suit,
				TextStyle: ts(theme.B4, 11, color),
			}),
		},
	})
}

// --- Game view ---

func gameView(w *gui.Window, ww, wh float32) gui.View {
	app := gui.State[App](w)
	game := app.Game
	theme := gui.CurrentTheme()

	views := make([]gui.View, 0, 64)

	// --- Top row: stock, waste, foundations ---
	views = append(views, stockView(game))
	views = append(views, wasteViews(app)...)
	for s := Spades; s <= Clubs; s++ {
		views = append(views, foundationView(game, s))
	}

	// --- Tableau columns ---
	for col := range 7 {
		views = append(views, tableauViews(app, col)...)
	}

	// --- Drag ghost ---
	if app.DragActive {
		views = append(views, dragGhostView(app))
	}

	// --- Status bar ---
	views = append(views, statusBar(app, theme, ww, wh))

	// --- Win overlay ---
	if game.State == StateWon {
		views = append(views, winOverlay(theme, ww, wh))
	}

	return gui.Canvas(gui.ContainerCfg{
		Width:      ww,
		Height:     wh,
		Sizing:     gui.FixedFixed,
		Color:      colorFelt,
		SizeBorder: gui.NoBorder,
		Content:    views,
	})
}

// --- Stock pile ---

func stockView(game *Game) gui.View {
	x := colX(0)
	if len(game.Stock) > 0 {
		return cardBackView(x, boardTopY, func(_ *gui.Layout, e *gui.Event, w *gui.Window) {
			if e.MouseButton != gui.MouseLeft {
				return
			}
			app := gui.State[App](w)
			app.Game.Draw()
			e.IsHandled = true
		})
	}
	// Empty stock — click to recycle.
	return emptySlot(x, boardTopY, func(_ *gui.Layout, e *gui.Event, w *gui.Window) {
		if e.MouseButton != gui.MouseLeft {
			return
		}
		app := gui.State[App](w)
		app.Game.Draw()
		e.IsHandled = true
	})
}

// --- Waste pile ---

func wasteViews(app *App) []gui.View {
	game := app.Game
	visible := game.VisibleWaste()
	if len(visible) == 0 {
		return []gui.View{emptySlot(colX(1), boardTopY, nil)}
	}

	views := make([]gui.View, 0, len(visible))
	for i, c := range visible {
		x := colX(1) + float32(i)*wasteFanOffset
		isTop := i == len(visible)-1

		// Skip rendering dragged waste card.
		if app.DragActive && app.DragSource.Type == SourceWaste && isTop {
			continue
		}

		if isTop {
			views = append(views, cardFaceUpView(c, x, boardTopY,
				makeWasteClickHandler()))
		} else {
			views = append(views, cardFaceUpViewNoClick(c, x, boardTopY))
		}
	}
	return views
}

func makeWasteClickHandler() func(*gui.Layout, *gui.Event, *gui.Window) {
	return func(layout *gui.Layout, e *gui.Event, w *gui.Window) {
		app := gui.State[App](w)
		game := app.Game
		tw := game.TopWaste()
		if tw == nil {
			return
		}

		// Right-click: auto-place to foundation or tableau.
		if e.MouseButton == gui.MouseRight {
			if game.AutoMoveToFoundation(SourceWaste, 0) ||
				game.AutoMoveToTableau(SourceWaste) {
				e.IsHandled = true
			}
			return
		}

		// Start drag.
		startDrag(app, layout, e, w, DragSource{Type: SourceWaste},
			[]Card{*tw})
		e.IsHandled = true
	}
}

// --- Foundation piles ---

func foundationView(game *Game, suit Suit) gui.View {
	col := int(suit) + 3 // foundations at columns 3–6
	x := colX(col)
	pile := game.Foundation[suit]
	if len(pile) == 0 {
		return foundationSlot(x, boardTopY, suit)
	}
	top := pile[len(pile)-1]
	return cardFaceUpViewNoClick(top, x, boardTopY)
}

// foundationSlot renders an empty foundation with a dim suit hint.
func foundationSlot(x, y float32, suit Suit) gui.View {
	theme := gui.CurrentTheme()
	hintColor := gui.RGB(180, 180, 180).WithOpacity(0.2)
	if suit.IsRed() {
		hintColor = colorCardRed.WithOpacity(0.25)
	}
	return gui.Column(gui.ContainerCfg{
		X:           x,
		Y:           y,
		Width:       cardW,
		Height:      cardH,
		Sizing:      gui.FixedFixed,
		Color:       colorFelt,
		ColorBorder: colorSlotBorder,
		SizeBorder:  gui.SomeF(1.5),
		Radius:      gui.SomeF(6),
		HAlign:      gui.HAlignCenter,
		VAlign:      gui.VAlignMiddle,
		Content: []gui.View{
			gui.Text(gui.TextCfg{
				Text:      suit.Symbol(),
				TextStyle: ts(theme.B1, 28, hintColor),
			}),
		},
	})
}

// --- Tableau columns ---

func tableauViews(app *App, col int) []gui.View {
	game := app.Game
	pile := game.Tableau[col]
	x := colX(col)

	if len(pile) == 0 {
		return []gui.View{emptySlot(x, tableauTopY, nil)}
	}

	views := make([]gui.View, 0, len(pile))
	y := tableauTopY
	for i, c := range pile {
		// Skip dragged cards.
		if app.DragActive && app.DragSource.Type == SourceTableau &&
			app.DragSource.ColIdx == col && i >= app.DragSource.CardIdx {
			break
		}

		if c.FaceUp {
			views = append(views, cardFaceUpView(c, x, y,
				makeTableauClickHandler(col, i)))
		} else {
			views = append(views, cardBackView(x, y, nil))
		}

		if c.FaceUp {
			y += overlapUp
		} else {
			y += overlapDown
		}
	}
	return views
}

func makeTableauClickHandler(col, cardIdx int) func(*gui.Layout, *gui.Event, *gui.Window) {
	return func(layout *gui.Layout, e *gui.Event, w *gui.Window) {
		app := gui.State[App](w)
		game := app.Game

		// Right-click: auto-place top card to foundation.
		if e.MouseButton == gui.MouseRight {
			isTopCard := cardIdx == len(game.Tableau[col])-1
			if isTopCard && game.AutoMoveToFoundation(SourceTableau, col) {
				e.IsHandled = true
			}
			return
		}

		// Build card stack from cardIdx onward.
		pile := game.Tableau[col]
		cards := make([]Card, len(pile)-cardIdx)
		copy(cards, pile[cardIdx:])

		startDrag(app, layout, e, w, DragSource{
			Type:    SourceTableau,
			ColIdx:  col,
			CardIdx: cardIdx,
		}, cards)
		e.IsHandled = true
	}
}

// --- Card views ---

func cardFaceUpView(c Card, x, y float32, onClick func(*gui.Layout, *gui.Event, *gui.Window)) gui.View {
	theme := gui.CurrentTheme()
	color := colorCardBlack
	if c.Suit.IsRed() {
		color = colorCardRed
	}
	label := c.RankString() + c.Suit.Symbol()
	return gui.Column(gui.ContainerCfg{
		X:           x,
		Y:           y,
		Width:       cardW,
		Height:      cardH,
		Sizing:      gui.FixedFixed,
		Color:       colorCardWhite,
		ColorBorder: colorCardBorder,
		SizeBorder:  gui.SomeF(1),
		Radius:      gui.SomeF(6),
		Padding:     gui.SomeP(4, 5, 4, 5),
		Clip:        true,
		OnAnyClick:  onClick,
		Content: []gui.View{
			gui.Text(gui.TextCfg{
				Text:      label,
				TextStyle: ts(theme.B3, 14, color),
			}),
			// Large centered suit.
			gui.Column(gui.ContainerCfg{
				Sizing:     gui.FillFill,
				HAlign:     gui.HAlignCenter,
				VAlign:     gui.VAlignMiddle,
				SizeBorder: gui.NoBorder,
				Content: []gui.View{
					gui.Text(gui.TextCfg{
						Text:      c.Suit.Symbol(),
						TextStyle: ts(theme.B1, 32, color),
					}),
				},
			}),
		},
	})
}

func cardFaceUpViewNoClick(c Card, x, y float32) gui.View {
	return cardFaceUpView(c, x, y, nil)
}

func cardBackView(x, y float32, onClick func(*gui.Layout, *gui.Event, *gui.Window)) gui.View {
	return gui.Column(gui.ContainerCfg{
		X:           x,
		Y:           y,
		Width:       cardW,
		Height:      cardH,
		Sizing:      gui.FixedFixed,
		Color:       colorCardBack,
		ColorBorder: colorNeonCyan,
		SizeBorder:  gui.SomeF(2),
		Radius:      gui.SomeF(6),
		OnAnyClick:  onClick,
	})
}

func emptySlot(x, y float32, onClick func(*gui.Layout, *gui.Event, *gui.Window)) gui.View {
	return gui.Column(gui.ContainerCfg{
		X:           x,
		Y:           y,
		Width:       cardW,
		Height:      cardH,
		Sizing:      gui.FixedFixed,
		Color:       colorFelt,
		ColorBorder: colorSlotBorder,
		SizeBorder:  gui.SomeF(1.5),
		Radius:      gui.SomeF(6),
		OnClick:     onClick,
	})
}

// --- Drag-and-drop ---

func startDrag(app *App, layout *gui.Layout, e *gui.Event, w *gui.Window, src DragSource, cards []Card) {
	app.DragActive = true
	app.DragCards = cards
	app.DragSource = src
	app.DragMouseX = e.MouseX + layout.Shape.X
	app.DragMouseY = e.MouseY + layout.Shape.Y
	app.DragOffsetX = e.MouseX
	app.DragOffsetY = e.MouseY

	w.MouseLock(gui.MouseLockCfg{
		MouseMove: func(_ *gui.Layout, e *gui.Event, w *gui.Window) {
			a := gui.State[App](w)
			a.DragMouseX = e.MouseX
			a.DragMouseY = e.MouseY
		},
		MouseUp: func(_ *gui.Layout, e *gui.Event, w *gui.Window) {
			a := gui.State[App](w)
			w.MouseUnlock()
			dropCards(a, e.MouseX, e.MouseY)
			a.DragActive = false
		},
	})
}

func dropCards(app *App, mx, my float32) {
	game := app.Game

	// Check foundation drop (top row, columns 3–6).
	if my < tableauTopY {
		for s := Spades; s <= Clubs; s++ {
			fx := colX(int(s) + 3)
			if mx >= fx && mx < fx+cardW {
				if tryFoundationDrop(app, game, s) {
					return
				}
			}
		}
	}

	// Check tableau drop.
	for col := range 7 {
		cx := colX(col)
		if mx >= cx && mx < cx+cardW {
			if tryTableauDrop(app, game, col) {
				return
			}
		}
	}
}

func tryFoundationDrop(app *App, game *Game, suit Suit) bool {
	if len(app.DragCards) != 1 {
		return false // Only single cards to foundation.
	}
	c := app.DragCards[0]
	if c.Suit != suit {
		return false
	}
	switch app.DragSource.Type {
	case SourceWaste:
		return game.MoveWasteToFoundation()
	case SourceTableau:
		return game.MoveTableauToFoundation(app.DragSource.ColIdx)
	}
	return false
}

func tryTableauDrop(app *App, game *Game, dstCol int) bool {
	switch app.DragSource.Type {
	case SourceWaste:
		return game.MoveWasteToTableau(dstCol)
	case SourceTableau:
		return game.MoveTableauToTableau(
			app.DragSource.ColIdx,
			app.DragSource.CardIdx,
			dstCol)
	case SourceFoundation:
		if len(app.DragCards) != 1 {
			return false
		}
		return game.MoveFoundationToTableau(
			app.DragCards[0].Suit, dstCol)
	}
	return false
}

func dragGhostView(app *App) gui.View {
	x := app.DragMouseX - app.DragOffsetX
	y := app.DragMouseY - app.DragOffsetY

	views := make([]gui.View, 0, len(app.DragCards))
	for i, c := range app.DragCards {
		views = append(views,
			cardFaceUpViewNoClick(c, 0, float32(i)*overlapUp))
	}
	return gui.Canvas(gui.ContainerCfg{
		X:          x,
		Y:          y,
		Width:      cardW,
		Height:     cardH + float32(len(app.DragCards)-1)*overlapUp,
		Sizing:     gui.FixedFixed,
		OverDraw:   true,
		SizeBorder: gui.NoBorder,
		Opacity:    gui.SomeF(0.85),
		Content:    views,
	})
}

// --- Status bar ---

func statusBar(app *App, theme gui.Theme, ww, wh float32) gui.View {
	game := app.Game
	scoreText := fmt.Sprintf("SCORE: %d", game.Score)
	movesText := fmt.Sprintf("MOVES: %d", game.Moves)

	return gui.Row(gui.ContainerCfg{
		X:          0,
		Y:          wh - 30,
		Width:      ww,
		Height:     30,
		Sizing:     gui.FixedFixed,
		Color:      gui.RGBA(0, 0, 0, 160),
		SizeBorder: gui.NoBorder,
		Padding:    gui.SomeP(4, 12, 4, 12),
		Content: []gui.View{
			gui.Text(gui.TextCfg{
				Text:      scoreText,
				TextStyle: ts(theme.B4, 13, colorNeonGreen),
			}),
			gui.Column(gui.ContainerCfg{
				Sizing:     gui.FillFit,
				SizeBorder: gui.NoBorder,
			}),
			gui.Text(gui.TextCfg{
				Text:      movesText,
				TextStyle: ts(theme.B4, 13, colorNeonCyan),
			}),
		},
	})
}

// --- Win overlay ---

func winOverlay(theme gui.Theme, ww, wh float32) gui.View {
	return gui.Column(gui.ContainerCfg{
		X:          0,
		Y:          0,
		Width:      ww,
		Height:     wh,
		Sizing:     gui.FixedFixed,
		Color:      gui.RGBA(0, 0, 0, 180),
		HAlign:     gui.HAlignCenter,
		VAlign:     gui.VAlignMiddle,
		Spacing:    gui.SomeF(12),
		SizeBorder: gui.NoBorder,
		OverDraw:   true,
		Content: []gui.View{
			gui.Text(gui.TextCfg{
				Text:      "YOU WIN!",
				TextStyle: ts(theme.B1, 52, colorNeonYellow),
			}),
			gui.Text(gui.TextCfg{
				Text:      "CONGRATULATIONS",
				TextStyle: ts(theme.B3, 20, colorGold),
			}),
			gui.Text(gui.TextCfg{
				Text:      "PRESS N FOR NEW GAME \u2022 ESC FOR MENU",
				TextStyle: ts(theme.M4, 14, colorDimText),
			}),
		},
	})
}

// --- Helpers ---

func ts(base gui.TextStyle, size float32, color gui.Color) gui.TextStyle {
	base.Size = size
	base.Color = color
	return base
}
