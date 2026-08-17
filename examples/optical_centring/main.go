// Optical centring probe (issue #346).
//
// Short, descender-free text sits high in a vertically-centred control:
// the shape is sized to the font's line box, but a run with no descender
// paints only in the upper part of it. Badge and ProgressBar take the
// correction, because the widget owns their text and its alphabet cannot
// descend. So do InputDate and NumericInput, which are editable but
// masked. A plain Input does not: arbitrary typed text does paint into
// the descent, so correcting it pushes the ink low.
//
// This app is the evidence for the calls that are still open. Each row
// shows a widget at three sizes, in a cap-only spelling and a
// descender-bearing one. Read it, do not reason about it:
//
//   - Do the corrected widgets (badge, progress readout) read centred
//     against the uncorrected ones beside them at 48pt?
//   - Do the corrected controls agree with each other? Badge, Button and
//     the progress readout all take it; the plain Text beside them does
//     not, and is the uncorrected reference.
//   - Type into the input: nothing may move vertically. The correction
//     is a property of the face, never of the string, and this is what
//     that guarantee looks like when it holds.
package main

import (
	"time"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend"
)

// App holds the typed text, so the input row is a live jitter check
// rather than a static picture.
type App struct {
	Typed string
}

// probeSizes are the sizes the correction is judged at. It scales with
// em, so a fault invisible at 16 is unmistakable at 48.
var probeSizes = []float32{16, 24, 48}

func main() {
	w := gui.SimpleWindow("Optical Centring", 900, 760,
		&App{Typed: "128"}, func(w *gui.Window) {
			w.UpdateView(mainView)
		})
	backend.Run(w)
}

func mainView(w *gui.Window) gui.View {
	app := gui.State[App](w)
	th := gui.CurrentTheme()

	rows := []gui.View{
		gui.Label("Optical centring probe — issue #346", th.B2),
		gui.Label("corrected: badge and progress readout on their own "+
			"ink; button, tab, select and menu on the face's cap band; "+
			"colour fields and masked inputs on its figures. "+
			"uncorrected: plain text, input. a badge that descends is "+
			"left alone, while a button that descends is not — the "+
			"badge is a value, the button is a label.",
			th.TextStyleSecondary),
		gui.Separator(gui.SeparatorCfg{}),
		// Select re-labels itself as the selection changes, so it takes
		// the content-free cap-band form: both of these must sit at the
		// same height, and the descender in "language" must hang below
		// the middle rather than pull the whole label up. It leads the
		// page because a control's label is what the correction is most
		// often judged by.
		gui.Label("select — cap band, whatever the label says",
			th.TextStyleLabel),
		gui.Row(gui.ContainerCfg{
			Spacing:    gui.Some(gui.SpacingMedium),
			VAlign:     gui.VAlignMiddle,
			SizeBorder: gui.NoBorder,
			Padding:    gui.NoPadding,
			Content: []gui.View{
				gui.Select(gui.SelectCfg{
					ID:          "probe_sel_desc",
					Placeholder: "Pick a language",
					Options:     []string{"Go", "Rust"},
				}),
				gui.Select(gui.SelectCfg{
					ID:          "probe_sel_caps",
					Placeholder: "PICK",
					Options:     []string{"Go", "Rust"},
				}),
			},
		}),
		gui.Separator(gui.SeparatorCfg{}),
	}

	for _, size := range probeSizes {
		rows = append(rows,
			gui.Label(sizeLabel(size), th.TextStyleLabel),
			sizeRow(size, "128"),
			sizeRow(size, "gypsy"),
			gui.Separator(gui.SeparatorCfg{}),
		)
	}

	rows = append(rows,
		gui.Label("type here — nothing may move vertically", th.TextStyleLabel),
		gui.Input(gui.InputCfg{
			ID:   "probe_input",
			Text: app.Typed,
			OnTextChanged: func(txt string, ctx gui.EventCtx) {
				gui.State[App](ctx.Window).Typed = txt
			},
		}),
		// The constrained-alphabet fields: editable, and corrected
		// anyway, because their masks cannot admit a descender. Type
		// digits into both — the value must not move vertically, and
		// each must sit level with the plain Input above, which is the
		// uncorrected control they have to keep matching in height.
		gui.Label("masked fields — corrected, and still may not move",
			th.TextStyleLabel),
		gui.Row(gui.ContainerCfg{
			Spacing:    gui.Some(gui.SpacingMedium),
			VAlign:     gui.VAlignMiddle,
			SizeBorder: gui.NoBorder,
			Padding:    gui.NoPadding,
			Content: []gui.View{
				gui.NumericInput(gui.NumericInputCfg{
					ID: "probe_num", Text: "128",
				}),
				gui.InputDate(gui.InputDateCfg{
					ID: "probe_date", Date: time.Now(),
				}),
			},
		}),
	)

	return gui.Column(gui.ContainerCfg{
		Sizing:     gui.FillFill,
		Scrollable: true,
		ID:         "probe_scroll",
		Spacing:    gui.Some(gui.SpacingMedium),
		Content:    rows,
	})
}

// sizeRow puts the corrected and uncorrected widgets side by side at one
// size, carrying one spelling. Side by side is the whole method: the
// error is a fraction of an em and is only obvious against a neighbour
// that does not share it.
func sizeRow(size float32, text string) gui.View {
	ts := gui.CurrentTheme().N4
	ts.Size = size

	return gui.Row(gui.ContainerCfg{
		VAlign:  gui.VAlignMiddle,
		Spacing: gui.Some(gui.SpacingMedium),
		Content: []gui.View{
			gui.Badge(gui.BadgeCfg{Label: text, TextStyle: ts}),
			gui.Button(gui.ButtonCfg{
				ID:      gui.ScopeIDN("probe", "btn_"+text, int(size)),
				Content: []gui.View{gui.Text(gui.TextCfg{Text: text, TextStyle: ts})},
				OnClick: func(gui.EventCtx) {},
			}),
			// A cap-only button beside the row's own spelling. Buttons
			// take the cap band, so this one and the "gypsy" button a
			// row down must sit at the same height in their boxes —
			// that is the property, and it is only visible as a pair.
			gui.Button(gui.ButtonCfg{
				ID:      gui.ScopeIDN("probe", "btncap_"+text, int(size)),
				Content: []gui.View{gui.Text(gui.TextCfg{Text: "PICK", TextStyle: ts})},
				OnClick: func(gui.EventCtx) {},
			}),
			gui.Text(gui.TextCfg{Text: text, TextStyle: ts}),
		},
	})
}

func sizeLabel(size float32) string {
	switch size {
	case 16:
		return "16pt — the default text size"
	case 24:
		return "24pt"
	default:
		return "48pt — where an uncorrected control is unmistakable"
	}
}
