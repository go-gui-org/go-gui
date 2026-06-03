// Package main implements a calculator example app.
package main

import (
	"math"
	"strconv"
	"strings"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend"
)

const (
	windowWidth  = 275
	windowHeight = 475

	shellWidth   = float32(220)
	shellHeight  = float32(420)
	buttonSize   = float32(42)
	buttonGap    = float32(6)
	displayFocus = 1
)

var (
	colorBackdrop     = gui.RGB(2, 2, 3)
	colorShell        = gui.ColorFromString("#17171b")
	colorShellBorder  = gui.ColorFromString("#3c3c40")
	colorScreenText   = gui.RGB(232, 232, 235)
	colorNumberButton = gui.ColorFromString("#46464a")
	colorTopButton    = gui.ColorFromString("#8e8e92")
	colorOperator     = gui.ColorFromString("#ff9f0a")
)

type calculatorState struct {
	Display          string
	StoredValue      float64
	PendingOperation string
	ReplaceDisplay   bool
	Err              bool
}

type calcButton struct {
	Label      string
	Background gui.Color
	Action     func(*calculatorState)
}

var keypadRows = [][]calcButton{
	{
		{Label: "DEL", Background: colorTopButton, Action: backspace},
		{Label: "AC", Background: colorTopButton, Action: clearState},
		{Label: "%", Background: colorTopButton, Action: applyPercent},
		{Label: "/", Background: colorOperator, Action: operatorAction("/")},
	},
	{
		{Label: "7", Background: colorNumberButton, Action: digitAction("7")},
		{Label: "8", Background: colorNumberButton, Action: digitAction("8")},
		{Label: "9", Background: colorNumberButton, Action: digitAction("9")},
		{Label: "x", Background: colorOperator, Action: operatorAction("*")},
	},
	{
		{Label: "4", Background: colorNumberButton, Action: digitAction("4")},
		{Label: "5", Background: colorNumberButton, Action: digitAction("5")},
		{Label: "6", Background: colorNumberButton, Action: digitAction("6")},
		{Label: "-", Background: colorOperator, Action: operatorAction("-")},
	},
	{
		{Label: "1", Background: colorNumberButton, Action: digitAction("1")},
		{Label: "2", Background: colorNumberButton, Action: digitAction("2")},
		{Label: "3", Background: colorNumberButton, Action: digitAction("3")},
		{Label: "+", Background: colorOperator, Action: operatorAction("+")},
	},
	{
		{Label: "+/-", Background: colorNumberButton, Action: toggleSign},
		{Label: "0", Background: colorNumberButton, Action: digitAction("0")},
		{Label: ".", Background: colorNumberButton, Action: appendDecimal},
		{Label: "=", Background: colorOperator, Action: evaluate},
	},
}

func main() {
	gui.SetTheme(gui.ThemeDarkNoPadding)

	w := gui.NewWindow(gui.WindowCfg{
		State:     newCalculatorState(),
		Title:     "calculator",
		Width:     windowWidth,
		Height:    windowHeight,
		FixedSize: true,
		BgColor:   colorBackdrop,
		OnInit: func(w *gui.Window) {
			w.UpdateView(mainView)
			w.SetIDFocus(displayFocus)
		},
		OnEvent: handleKeyEvent,
	})

	backend.Run(w)
}

func newCalculatorState() *calculatorState {
	return &calculatorState{
		Display: "0",
	}
}

func digitAction(digit string) func(*calculatorState) {
	return func(state *calculatorState) {
		appendDigit(state, digit)
	}
}

func operatorAction(op string) func(*calculatorState) {
	return func(state *calculatorState) {
		applyOperator(state, op)
	}
}

func handleKeyEvent(e *gui.Event, w *gui.Window) {
	if e.Type != gui.EventKeyDown {
		return
	}

	state := gui.State[calculatorState](w)
	if digit := keyDigit(e.KeyCode); digit != "" {
		appendDigit(state, digit)
		e.IsHandled = true
		return
	}

	switch {
	case e.KeyCode == gui.KeyPeriod || e.KeyCode == gui.KeyKPDecimal:
		appendDecimal(state)
	case e.KeyCode == gui.KeySlash || e.KeyCode == gui.KeyKPDivide:
		applyOperator(state, "/")
	case e.KeyCode == gui.KeyKPMultiply:
		applyOperator(state, "*")
	case e.KeyCode == gui.KeyMinus || e.KeyCode == gui.KeyKPSubtract:
		applyOperator(state, "-")
	case e.KeyCode == gui.KeyEqual && e.Modifiers.Has(gui.ModShift):
		applyOperator(state, "+")
	case e.KeyCode == gui.KeyEqual || e.KeyCode == gui.KeyEnter || e.KeyCode == gui.KeyKPEnter:
		evaluate(state)
	case e.KeyCode == gui.KeyKPAdd:
		applyOperator(state, "+")
	case e.KeyCode == gui.KeyBackspace || e.KeyCode == gui.KeyDelete:
		backspace(state)
	case e.KeyCode == gui.KeyEscape:
		clearState(state)
	case e.KeyCode == gui.KeyP:
		applyPercent(state)
	case e.KeyCode == gui.KeyN:
		toggleSign(state)
	default:
		return
	}

	e.IsHandled = true
}

func keyDigit(key gui.KeyCode) string {
	switch key {
	case gui.Key0, gui.KeyKP0:
		return "0"
	case gui.Key1, gui.KeyKP1:
		return "1"
	case gui.Key2, gui.KeyKP2:
		return "2"
	case gui.Key3, gui.KeyKP3:
		return "3"
	case gui.Key4, gui.KeyKP4:
		return "4"
	case gui.Key5, gui.KeyKP5:
		return "5"
	case gui.Key6, gui.KeyKP6:
		return "6"
	case gui.Key7, gui.KeyKP7:
		return "7"
	case gui.Key8, gui.KeyKP8:
		return "8"
	case gui.Key9, gui.KeyKP9:
		return "9"
	default:
		return ""
	}
}

func mainView(w *gui.Window) gui.View {
	ww, wh := w.WindowSize()

	return gui.Canvas(gui.ContainerCfg{
		Width:   float32(ww),
		Height:  float32(wh),
		Sizing:  gui.FixedFixed,
		Color:   colorBackdrop,
		Padding: gui.NoPadding,
		Content: []gui.View{
			gui.Column(gui.ContainerCfg{
				X:       0,
				Y:       0,
				Width:   float32(ww),
				Height:  float32(wh),
				Sizing:  gui.FixedFixed,
				HAlign:  gui.HAlignCenter,
				VAlign:  gui.VAlignMiddle,
				Padding: gui.SomeP(28, 28, 28, 28),
				Content: []gui.View{
					calculatorShell(w),
				},
			}),
		},
	})
}

func calculatorShell(w *gui.Window) gui.View {
	return gui.Column(gui.ContainerCfg{
		Width:       shellWidth,
		Height:      shellHeight,
		Sizing:      gui.FixedFixed,
		Color:       colorShell,
		ColorBorder: colorShellBorder,
		SizeBorder:  gui.SomeF(2),
		Radius:      gui.SomeF(22),
		Padding:     gui.SomeP(10, 10, 10, 10),
		Spacing:     gui.SomeF(8),
		Content: []gui.View{
			topChrome(),
			displayView(w),
			keypadView(w),
		},
	})
}

func topChrome() gui.View {
	return gui.Row(gui.ContainerCfg{
		Sizing:  gui.FillFit,
		VAlign:  gui.VAlignMiddle,
		Padding: gui.NoPadding,
		Content: []gui.View{
			gui.Row(gui.ContainerCfg{
				Sizing:  gui.FitFit,
				Spacing: gui.SomeF(8),
				Padding: gui.NoPadding,
				Content: []gui.View{
					chromeDot(gui.RGB(255, 95, 87)),
					chromeDot(gui.RGB(255, 189, 46)),
					chromeDot(gui.RGB(76, 76, 78)),
				},
			}),
			gui.Row(gui.ContainerCfg{
				Sizing:  gui.FillFit,
				HAlign:  gui.HAlignEnd,
				Padding: gui.NoPadding,
				Content: []gui.View{
					gui.Text(gui.TextCfg{
						Text: "CALC",
						TextStyle: gui.TextStyle{
							Color:    gui.RGB(236, 236, 238),
							Size:     12,
							Typeface: 1,
						},
					}),
				},
			}),
		},
	})
}

func chromeDot(color gui.Color) gui.View {
	return gui.Circle(gui.ContainerCfg{
		Width:   18,
		Height:  18,
		Sizing:  gui.FixedFixed,
		Color:   color,
		Padding: gui.NoPadding,
	})
}

func displayView(w *gui.Window) gui.View {
	state := gui.State[calculatorState](w)
	displayText := state.Display
	style := gui.TextStyle{
		Color:    colorScreenText,
		Size:     30,
		Typeface: 1,
	}
	if len(displayText) > 10 {
		style.Size = 22
	}
	if len(displayText) > 14 {
		style.Size = 18
	}

	return gui.Column(gui.ContainerCfg{
		Width:   shellWidth - 20,
		Height:  72,
		Sizing:  gui.FixedFixed,
		HAlign:  gui.HAlignEnd,
		VAlign:  gui.VAlignBottom,
		Padding: gui.SomeP(2, 4, 2, 4),
		Content: []gui.View{
			gui.Text(gui.TextCfg{
				IDFocus:   displayFocus,
				Text:      displayText,
				TextStyle: style,
			}),
		},
	})
}

func keypadView(w *gui.Window) gui.View {
	content := make([]gui.View, 0, len(keypadRows))
	for _, row := range keypadRows {
		content = append(content, keypadRow(w, row))
	}

	return gui.Column(gui.ContainerCfg{
		Width:   shellWidth - 20,
		Height:  234,
		Sizing:  gui.FixedFixed,
		Spacing: gui.Some(buttonGap),
		Padding: gui.NoPadding,
		Content: content,
	})
}

func keypadRow(w *gui.Window, buttons []calcButton) gui.View {
	content := make([]gui.View, 0, len(buttons))
	for _, button := range buttons {
		content = append(content, calcKey(w, button))
	}

	return gui.Row(gui.ContainerCfg{
		Width:   shellWidth - 20,
		Height:  buttonSize,
		Sizing:  gui.FixedFixed,
		Spacing: gui.Some(buttonGap),
		Padding: gui.NoPadding,
		Content: content,
	})
}

func calcKey(_ *gui.Window, button calcButton) gui.View {
	labelStyle := gui.TextStyle{
		Color: colorScreenText,
		Size:  16,
	}
	if button.Background == colorTopButton || button.Background == colorOperator {
		labelStyle.Color = gui.RGB(12, 12, 12)
	}
	if button.Label == "AC" || button.Label == "DEL" || button.Label == "+/-" {
		labelStyle.Size = 11
		labelStyle.Typeface = 1
	}

	return gui.Button(gui.ButtonCfg{
		ID:               "calc-" + strings.NewReplacer("/", "div", "*", "mul", "+", "add", "-", "sub", ".", "dot", "%", "pct", "=", "eq").Replace(button.Label),
		Color:            button.Background,
		ColorHover:       lighten(button.Background, 12),
		ColorClick:       lighten(button.Background, 24),
		ColorFocus:       lighten(button.Background, 12),
		ColorBorder:      lighten(button.Background, 18),
		ColorBorderFocus: lighten(button.Background, 28),
		SizeBorder:       gui.SomeF(2),
		Width:            buttonSize,
		Height:           buttonSize,
		Sizing:           gui.FixedFixed,
		Radius:           gui.Some(buttonSize / 2),
		Padding:          gui.NoPadding,
		Content: []gui.View{
			gui.Text(gui.TextCfg{
				Text:      button.Label,
				TextStyle: labelStyle,
			}),
		},
		OnClick: func(_ *gui.Layout, e *gui.Event, w *gui.Window) {
			button.Action(gui.State[calculatorState](w))
			e.IsHandled = true
		},
	})
}

func appendDigit(state *calculatorState, digit string) {
	if state.Err {
		clearState(state)
	}
	if state.ReplaceDisplay || state.Display == "0" {
		state.Display = digit
		state.ReplaceDisplay = false
		return
	}
	state.Display += digit
}

func appendDecimal(state *calculatorState) {
	if state.Err {
		clearState(state)
	}
	if state.ReplaceDisplay {
		state.Display = "0."
		state.ReplaceDisplay = false
		return
	}
	if strings.Contains(state.Display, ".") {
		return
	}
	state.Display += "."
}

func clearState(state *calculatorState) {
	state.Display = "0"
	state.StoredValue = 0
	state.PendingOperation = ""
	state.ReplaceDisplay = false
	state.Err = false
}

func backspace(state *calculatorState) {
	if state.Err {
		clearState(state)
		return
	}
	if state.ReplaceDisplay {
		state.Display = "0"
		state.ReplaceDisplay = false
		return
	}
	if len(state.Display) <= 1 || (len(state.Display) == 2 && strings.HasPrefix(state.Display, "-")) {
		state.Display = "0"
		return
	}
	state.Display = state.Display[:len(state.Display)-1]
}

func toggleSign(state *calculatorState) {
	if state.Err || state.Display == "0" {
		return
	}
	if rest, ok := strings.CutPrefix(state.Display, "-"); ok {
		state.Display = rest
		return
	}
	state.Display = "-" + state.Display
}

func applyPercent(state *calculatorState) {
	value, ok := parseDisplay(state)
	if !ok {
		return
	}
	state.Display = formatValue(value / 100)
	state.ReplaceDisplay = true
}

func applyOperator(state *calculatorState, operation string) {
	value, ok := parseDisplay(state)
	if !ok {
		return
	}
	if state.PendingOperation != "" && !state.ReplaceDisplay {
		value, ok = compute(state.StoredValue, value, state.PendingOperation)
		if !ok {
			setError(state)
			return
		}
		state.Display = formatValue(value)
	}
	state.StoredValue = value
	state.PendingOperation = operation
	state.ReplaceDisplay = true
}

func evaluate(state *calculatorState) {
	if state.PendingOperation == "" {
		return
	}

	value, ok := parseDisplay(state)
	if !ok {
		return
	}
	result, ok := compute(state.StoredValue, value, state.PendingOperation)
	if !ok {
		setError(state)
		return
	}

	state.Display = formatValue(result)
	state.StoredValue = result
	state.PendingOperation = ""
	state.ReplaceDisplay = true
}

func parseDisplay(state *calculatorState) (float64, bool) {
	value, err := strconv.ParseFloat(state.Display, 64)
	if err != nil {
		setError(state)
		return 0, false
	}
	return value, true
}

func compute(left, right float64, operation string) (float64, bool) {
	switch operation {
	case "+":
		return left + right, true
	case "-":
		return left - right, true
	case "*":
		return left * right, true
	case "/":
		if right == 0 {
			return 0, false
		}
		return left / right, true
	default:
		return right, true
	}
}

func setError(state *calculatorState) {
	state.Display = "Error"
	state.StoredValue = 0
	state.PendingOperation = ""
	state.ReplaceDisplay = true
	state.Err = true
}

func formatValue(value float64) string {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return "Error"
	}
	if value == 0 {
		return "0"
	}

	text := strconv.FormatFloat(value, 'f', -1, 64)
	if len(text) > 16 {
		text = strconv.FormatFloat(value, 'g', 10, 64)
	}
	if strings.Contains(text, ".") {
		text = strings.TrimRight(text, "0")
		text = strings.TrimRight(text, ".")
	}
	if text == "-0" {
		return "0"
	}
	return text
}

func lighten(color gui.Color, delta uint8) gui.Color {
	return gui.RGB(
		clampColor(int(color.R)+int(delta)),
		clampColor(int(color.G)+int(delta)),
		clampColor(int(color.B)+int(delta)),
	)
}

func clampColor(value int) uint8 {
	if value < 0 {
		return 0
	}
	if value > math.MaxUint8 {
		return math.MaxUint8
	}
	return uint8(value)
}
