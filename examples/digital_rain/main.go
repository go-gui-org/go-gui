// Digital_rain recreates the falling-character effect from "The Matrix".
package main

import (
	"math/rand/v2"
	"time"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend"
)

const (
	fontSize       = 14
	charPadding    = 3
	lineHeight     = 1.3
	tickAnimation  = "rain-tick"
	renderInterval = 30 * time.Millisecond
	defaultDelay   = 200 * time.Millisecond
	minDelay       = 50 * time.Millisecond
	delayStep      = 50 * time.Millisecond
	minTrailLen    = 6
	minRows        = 8
	dropOffset     = 10 // secondary character offset for shimmer
	columnFraction = 3  // spawn up to cols*columnFraction/4
)

var rainDrops = []byte("0123456789!@#$%^&*()-=+[]{}|;:<>?~bdjpqtvz")

// Pre-built single-character strings to avoid per-frame allocations.
var dropStrings [256]string

func init() {
	for i := range 256 {
		dropStrings[i] = string(rune(i))
	}
}

type App struct {
	RainColumns  []RainColumn
	CharWidth    float32
	CharHeight   float32
	Cols         int
	Rows         int
	Delay        time.Duration
	TickProgress float32 // 0.0→1.0 within each tick cycle
}

type RainColumn struct {
	Drops []byte
	Col   int
	Len   int
	Head  int
}

func main() {
	gui.SetTheme(gui.ThemeDarkBordered)

	w := gui.NewWindow(gui.WindowCfg{
		State:  &App{Delay: defaultDelay},
		Title:  "Digital Rain",
		Width:  800,
		Height: 600,
		OnInit: func(w *gui.Window) {
			w.UpdateView(mainView)
			startTick(w)
		},
		OnEvent: handleEvent,
	})

	backend.Run(w)
}

func startTick(w *gui.Window) {
	app := gui.State[App](w)
	app.TickProgress = 0
	w.AnimationAdd(&gui.Animate{
		AnimID: tickAnimation,
		Delay:  renderInterval,
		Repeat: true,
		Callback: func(_ *gui.Animate, win *gui.Window) {
			a := gui.State[App](win)
			step := float32(renderInterval) / float32(a.Delay)
			a.TickProgress += step
			if a.TickProgress >= 1 {
				a.TickProgress -= 1
				advanceColumns(a)
			}
		},
	})
}

func advanceColumns(app *App) {
	maxCols := max(app.Cols*columnFraction/4, 1)
	if len(app.RainColumns) < maxCols {
		app.RainColumns = append(app.RainColumns,
			randomRainColumn(app.Cols, app.Rows))
	}
	for i := range app.RainColumns {
		rc := &app.RainColumns[i]
		rc.Head++
		if rc.Head > app.Rows+rc.Len {
			app.RainColumns[i] = randomRainColumn(app.Cols, app.Rows)
		}
	}
}

func handleEvent(e *gui.Event, w *gui.Window) {
	if e.Type != gui.EventKeyUp {
		return
	}
	app := gui.State[App](w)
	switch e.KeyCode {
	case gui.KeyUp:
		app.Delay += delayStep
		restartTick(w)
		e.IsHandled = true
	case gui.KeyDown:
		app.Delay = max(app.Delay-delayStep, minDelay)
		restartTick(w)
		e.IsHandled = true
	}
}

func restartTick(w *gui.Window) {
	w.AnimationRemove(tickAnimation)
	startTick(w)
}

func mainView(w *gui.Window) gui.View {
	ww, wh := w.WindowSize()
	app := gui.State[App](w)

	baseStyle := gui.CurrentTheme().M3
	baseStyle.Size = fontSize

	charW := w.TextWidth("M", baseStyle) + charPadding
	charH := float32(fontSize) * lineHeight

	app.CharWidth = charW
	app.CharHeight = charH
	app.Cols = int(float32(ww) / charW)
	app.Rows = int(float32(wh) / charH)

	content := make([]gui.View, 0, len(app.RainColumns)*20)

	for _, rc := range app.RainColumns {
		end := rc.Head - rc.Len
		x := float32(rc.Col) * charW
		start := max(0, end)
		stop := min(rc.Head, min(app.Rows, len(rc.Drops)))

		for row := start; row < stop; row++ {
			dist := row - end
			alpha := min(float32(75+dist*25), 255)

			atHead := row == rc.Head-1
			if atHead {
				alpha = app.TickProgress * 255
			}

			opacity := alpha / 255
			var color gui.Color
			if atHead {
				color = gui.White.WithOpacity(opacity)
			} else {
				color = gui.Green.WithOpacity(opacity)
			}

			style := baseStyle
			style.Color = color

			ch1 := dropStrings[rc.Drops[row]]
			ch2 := dropStrings[rc.Drops[(row+dropOffset)%len(rc.Drops)]]
			y := float32(row) * charH

			content = append(content, gui.Canvas(gui.ContainerCfg{
				X:          x,
				Y:          y,
				Width:      charW,
				Height:     charH,
				Sizing:     gui.FixedFixed,
				Padding:    gui.NoPadding,
				SizeBorder: gui.NoBorder,
				Content: []gui.View{
					gui.Text(gui.TextCfg{Text: ch1, TextStyle: style}),
					gui.Text(gui.TextCfg{Text: ch2, TextStyle: style}),
				},
			}))
		}
	}

	statusStyle := baseStyle
	statusStyle.Color = gui.Gray

	content = append(content, gui.Column(gui.ContainerCfg{
		X:          0,
		Y:          float32(wh) - charH - 4,
		Width:      float32(ww),
		Height:     charH + 4,
		Sizing:     gui.FixedFixed,
		HAlign:     gui.HAlignCenter,
		Padding:    gui.NoPadding,
		SizeBorder: gui.NoBorder,
		Content: []gui.View{
			gui.Text(gui.TextCfg{
				Text:      "Up/Down arrows change speed",
				TextStyle: statusStyle,
			}),
		},
	}))

	return gui.Canvas(gui.ContainerCfg{
		Width:      float32(ww),
		Height:     float32(wh),
		Sizing:     gui.FixedFixed,
		Color:      gui.Black,
		Padding:    gui.NoPadding,
		SizeBorder: gui.NoBorder,
		Content:    content,
	})
}

func randomRainColumn(maxCol, maxRow int) RainColumn {
	maxCol = max(maxCol, 1)
	maxRow = max(maxRow, minRows)
	maxLen := max(maxRow*3/4, minTrailLen+1)
	drops := make([]byte, maxRow)
	for i := range drops {
		drops[i] = rainDrops[rand.IntN(len(rainDrops))]
	}
	return RainColumn{
		Col:   rand.IntN(maxCol),
		Len:   minTrailLen + rand.IntN(maxLen-minTrailLen),
		Drops: drops,
	}
}
