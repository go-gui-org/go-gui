// This example demonstrates custom text field looks wrapped around gui.Input
// (advanced: build-time interaction state of a child widget).
//
// It ports go-shirei's custom-textinputs demo. Two looks, each single-line and
// multiline:
//
//   - Material. A white field, a thin border and an underline that turns
//     into a thick accent line on focus.
//   - Windows XP. A square blue border, a white face and a light inset line
//     at the top.
//
// go-shirei splits a field into ProcessTextInput, which edits the text and
// returns HasFocus, and paint code. In go-gui the editing is gui.Input. The
// look is a container around an Input whose own chrome is turned off.
//
// What this costs (input for the design issue):
//
//   - The look needs the focus of the Input, not of the wrapper.
//     InteractionState.FocusWithin is true when the wrapper or the Input
//     inside it has focus, so the builder reads that.
//   - Turning the Input's chrome off takes four fields: a transparent
//     gui.Flat color set, no border, no radius, no padding; see plainInput.
//   - A press on the wrapper's padding does not reach the Input. The wrapper
//     has an OnMouseDown that moves focus to it; see focusField.
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
		State:  newApp(),
		Title:  "Custom Text Inputs",
		Width:  720,
		Height: 900,
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

// fieldID is the ID leaf of the Input inside every custom field. It joins the
// wrapper's scope, so each field's Input has its own effective ID.
const fieldID = "field"

// multilineHeight is the height of a multiline Input, about four lines.
const multilineHeight = 88

// App holds the text of every field, keyed by the field's ID leaf.
type App struct {
	text map[string]string

	// Change handlers are built once per field, so a frame does not
	// allocate new closures.
	changed map[string]func(string, gui.EventCtx)
}

func newApp() *App {
	app := &App{
		text: map[string]string{
			"name":     "Taro Yamada",
			"notes":    "Multiline default Input.\nSecond line.",
			"email":    "you@example.com",
			"matnotes": "Material multiline notes.\nLine two.",
			"user":     "Administrator",
			"xpnotes":  "XP multiline field.\nSecond line of text.",
		},
		changed: map[string]func(string, gui.EventCtx){},
	}
	for key := range app.text {
		app.changed[key] = func(s string, ctx gui.EventCtx) {
			app.text[key] = s
			ctx.Consume()
		}
	}
	return app
}

func mainView(w *gui.Window) gui.View {
	app := gui.State[App](w)
	title := gui.CurrentTheme().TextStyleDef
	title.Size = 18

	return gui.Column(gui.ContainerCfg{
		ID:         "page",
		Scrollable: true,
		Sizing:     gui.FillFill,
		Color:      pageBG,
		Padding:    gui.PadAll(28),
		Spacing:    gui.SomeF(14),
		Content: []gui.View{
			gui.Text(gui.TextCfg{Text: "Custom text inputs around gui.Input", TextStyle: title}),

			sectionTitle("Default gui.Input, for comparison"),
			fieldLabel("Single-line"),
			gui.Input(gui.InputCfg{ID: "name", Text: app.text["name"],
				Sizing: gui.FillFit, OnTextChanged: app.changed["name"]}),
			fieldLabel("Multiline"),
			gui.Input(gui.InputCfg{ID: "notes", Text: app.text["notes"],
				Mode: gui.InputMultiline, Height: multilineHeight, Sizing: gui.FillFixed,
				OnTextChanged: app.changed["notes"]}),

			sectionTitle("Material"),
			note("Flat fill, thin border, thick accent underline on focus."),
			group("material", gui.ContainerCfg{},
				fieldLabel("Single-line"),
				materialField(app, "email", false, materialBlue),
				fieldLabel("Multiline"),
				materialField(app, "matnotes", true, materialTeal),
			),

			sectionTitle("Windows XP Luna"),
			note("Blue border, white face, light top inset: a field, not a raised button."),
			group("xp", gui.ContainerCfg{
				Color:       xpPanel,
				ColorBorder: xpPanelEdge,
				SizeBorder:  gui.SomeF(1),
				Padding:     gui.PadAll(14),
			},
				fieldLabel("Single-line"),
				xpField(app, "user", false),
				fieldLabel("Multiline"),
				xpField(app, "xpnotes", true),
			),

			valuesPanel(app),
		},
	})
}

func sectionTitle(s string) gui.View {
	return gui.Text(gui.TextCfg{Text: s, TextStyle: gui.CurrentTheme().B3})
}

func fieldLabel(s string) gui.View {
	return gui.Text(gui.TextCfg{Text: s, TextStyle: gui.CurrentTheme().TextStyleLabel})
}

func note(s string) gui.View {
	return gui.Text(gui.TextCfg{Text: s, TextStyle: gui.CurrentTheme().TextStyleSecondary})
}

// group is one ID-bearing column. The ID scopes the fields inside it.
func group(id string, cfg gui.ContainerCfg, content ...gui.View) gui.View {
	cfg.ID = id
	cfg.Sizing = gui.FillFit
	cfg.Spacing = gui.SomeF(8)
	if !cfg.Padding.IsSet() {
		cfg.Padding = gui.PaddingNone
	}
	if !cfg.SizeBorder.IsSet() {
		cfg.SizeBorder = gui.NoBorder
	}
	cfg.Content = content
	return gui.Column(cfg)
}

// valuesPanel shows the live text of the custom fields.
func valuesPanel(app *App) gui.View {
	return gui.Column(gui.ContainerCfg{
		ID:          "values",
		Sizing:      gui.FillFit,
		Color:       white,
		Radius:      gui.SomeF(4),
		ColorBorder: gui.RGBA(0, 0, 0, 20),
		SizeBorder:  gui.SomeF(1),
		Padding:     gui.NewPadding(10, 14, 10, 14),
		Spacing:     gui.SomeF(4),
		Content: []gui.View{
			gui.Text(gui.TextCfg{Text: "Live values", TextStyle: gui.CurrentTheme().TextStyleLabel}),
			gui.Text(gui.TextCfg{ID: "default-values",
				Text: fmt.Sprintf("default=%q", app.text["name"])}),
			gui.Text(gui.TextCfg{ID: "custom-values",
				Text: fmt.Sprintf("material=%q  xp=%q", app.text["email"], app.text["user"])}),
		},
	})
}

// plainInput is a gui.Input with its own background, border, radius and
// padding turned off, so the wrapper around it draws the whole look.
func plainInput(app *App, key string, multiline bool) gui.View {
	cfg := gui.InputCfg{
		ID:            fieldID,
		Text:          app.text[key],
		OnTextChanged: app.changed[key],
		Sizing:        gui.FillFit,
		// One transparent ColorSet turns off the fill, hover and both
		// border colors.
		Colors:     gui.Flat(gui.ColorTransparent),
		SizeBorder: gui.SomeF(0),
		Radius:     gui.SomeF(0),
		Padding:    gui.PaddingNone,
	}
	if multiline {
		cfg.Mode = gui.InputMultiline
		cfg.Height = multilineHeight
		cfg.Sizing = gui.FillFixed
	}
	return gui.Input(cfg)
}

// focusField moves focus to the Input when a press lands on the wrapper's
// padding or border. A press on the Input itself has already focused it.
// One handler serves every field: the wrapper is ctx.Layout, so its
// effective ID comes from the shape. OnMouseDown, not OnClick: gui.Debug
// expects an Interactive root with OnClick to be a button.
func focusField(ctx gui.EventCtx) {
	id := gui.ScopeID(ctx.EffID(ctx.Layout.Shape.ID), fieldID)
	if !ctx.Window.IsFocus(id) {
		ctx.Window.SetFocus(id)
	}
	ctx.Consume()
}

var (
	pageBG = gui.Hex(0xeeeff2)
	white  = gui.Hex(0xffffff)
)

// --- Material --------------------------------------------------------------

var (
	materialBlue      = gui.Hex(0x2272c3)
	materialTeal      = gui.Hex(0x30a393)
	materialBorder    = gui.Hex(0xcccccc)
	materialHover     = gui.Hex(0xa6a6a6)
	materialUnderline = gui.RGBA(204, 204, 204, 128)
)

// materialField is a white field with a thin border and an underline. On
// focus the border and underline take the accent color and the underline
// grows to 2 px. The underline strip is always 2 px tall, so the field does
// not change height; the line inside it grows.
func materialField(app *App, id string, multiline bool, accent gui.Color) gui.View {
	return gui.Interactive(id, func(s gui.InteractionState) gui.View {
		border, line, lineH := materialBorder, materialUnderline, float32(1)
		switch {
		case s.FocusWithin:
			border, line, lineH = accent, accent, 2
		case s.Hovered:
			border = materialHover
		}
		return gui.Column(gui.ContainerCfg{
			ID:          id,
			Sizing:      gui.FillFit,
			Color:       white,
			Radius:      gui.SomeF(4),
			ColorBorder: border,
			SizeBorder:  gui.SomeF(1),
			Padding:     gui.NewPadding(8, 8, 0, 8),
			Spacing:     gui.SomeF(6),
			Clip:        true,
			OnMouseDown: focusField,
			Content: []gui.View{
				plainInput(app, id, multiline),
				gui.Column(gui.ContainerCfg{
					ID:         "underline",
					Height:     2,
					Sizing:     gui.FillFixed,
					Padding:    gui.PaddingNone,
					SizeBorder: gui.NoBorder,
					VAlign:     gui.VAlignBottom,
					Content: []gui.View{
						gui.Rectangle(gui.RectangleCfg{Height: lineH, Sizing: gui.FillFixed, Color: line}),
					},
				}),
			},
		})
	})
}

// --- Windows XP ------------------------------------------------------------

var (
	xpPanel     = gui.Hex(0xece9d8)
	xpPanelEdge = gui.Hex(0xb5b2a3)
	xpBorder    = gui.Hex(0x7f9db9)
	xpFocus     = gui.Hex(0x3c6fb0)
	xpInset     = gui.RGBA(0, 0, 0, 30)
)

// xpField is a Luna edit control: a square blue border, a white face and a
// 1 px shade under the top border, so the face reads as sunken. The border
// darkens on focus.
func xpField(app *App, id string, multiline bool) gui.View {
	return gui.Interactive(id, func(s gui.InteractionState) gui.View {
		border := xpBorder
		if s.FocusWithin {
			border = xpFocus
		}
		return gui.Column(gui.ContainerCfg{
			ID:          id,
			Sizing:      gui.FillFit,
			Color:       white,
			Radius:      gui.SomeF(0),
			ColorBorder: border,
			SizeBorder:  gui.SomeF(1),
			Padding:     gui.PaddingNone,
			Spacing:     gui.SomeF(0),
			Clip:        true,
			OnMouseDown: focusField,
			Content: []gui.View{
				gui.Rectangle(gui.RectangleCfg{Height: 1, Sizing: gui.FillFixed, Color: xpInset}),
				gui.Column(gui.ContainerCfg{
					Sizing:     gui.FillFit,
					Padding:    gui.NewPadding(5, 7, 6, 7),
					SizeBorder: gui.NoBorder,
					Content:    []gui.View{plainInput(app, id, multiline)},
				}),
			},
		})
	})
}
