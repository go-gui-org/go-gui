// The multiline input example is a playground for cursor movement,
// selection, clipboard, and scrolling behavior.
package main

// Multiline Input Demo
// =============================
// Use this program to test cursor movement and text selections.
//
// Keyboard shortcuts:
//   - left/right         move cursor one character
//   - ctrl+left          move to start of line (or up one line)
//   - ctrl+right         move to end of line (or down one line)
//   - alt+left           move to end of previous word
//   - alt+right          move to start of next word
//   - home               move cursor to start of text
//   - end                move cursor to end of text
//   - Add shift to above shortcuts to select text
//   - ctrl+a / cmd+a     select all
//   - ctrl+c / cmd+c     copy selection
//   - ctrl+v / cmd+v     paste
//   - ctrl+x / cmd+x     cut selection
//   - ctrl+z / cmd+z     undo
//   - shift+ctrl+z       redo
//   - escape             deselect
//   - delete/backspace   delete previous character
//
// Mouse selection and auto-scroll during drag-select are supported.

import (
	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend"
)

const inputFocusID = "input"
const inputIDScroll uint32 = 1

const loremText = `Lorem ipsum dolor sit amet, consectetur adipiscing elit. Sed do eiusmod ` +
	`tempor incididunt ut labore et dolore magna aliqua. Ut enim ad minim veniam, ` +
	`quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat.

Duis aute irure dolor in reprehenderit in voluptate velit esse cillum dolore eu ` +
	`fugiat nulla pariatur. Excepteur sint occaecat cupidatat non proident, sunt in ` +
	`culpa qui officia deserunt mollit anim id est laborum.

Sed ut perspiciatis unde omnis iste natus error sit voluptatem accusantium ` +
	`doloremque laudantium, totam rem aperiam, eaque ipsa quae ab illo inventore ` +
	`veritatis et quasi architecto beatae vitae dicta sunt explicabo.

Nemo enim ipsam voluptatem quia voluptas sit aspernatur aut odit aut fugit, sed ` +
	`quia consequuntur magni dolores eos qui ratione voluptatem sequi nesciunt. Neque ` +
	`porro quisquam est, qui dolorem ipsum quia dolor sit amet.

At vero eos et accusamus et iusto odio dignissimos ducimus qui blanditiis ` +
	`praesentium voluptatum deleniti atque corrupti quos dolores et quas molestias ` +
	`excepturi sint occaecati cupiditate non provident.

` + `Temporibus autem quibusdam et aut officiis debitis aut rerum necessitatibus saepe ` +
	`eveniet ut et voluptates repudiandae sint et molestiae non recusandae. Itaque earum ` +
	`rerum hic tenetur a sapiente delectus, ut aut reiciendis voluptatibus maiores.

` + `Nam libero tempore, cum soluta nobis est eligendi optio cumque nihil impedit quo ` +
	`minus id quod maxime placeat facere possimus, omnis voluptas assumenda est, omnis ` +
	`dolor repellendus. Temporibus autem quibusdam et aut officiis debitis.

` + `Et harum quidem rerum facilis est et expedita distinctio. Nam libero tempore, cum ` +
	`soluta nobis est eligendi optio cumque nihil impedit quo minus id quod maxime ` +
	`placeat facere possimus, omnis voluptas assumenda est.

` + `Quis autem vel eum iure reprehenderit qui in ea voluptate velit esse quam nihil ` +
	`molestiae consequatur, vel illum qui dolorem eum fugiat quo voluptas nulla pariatur? ` +
	`At vero eos et accusamus et iusto odio dignissimos ducimus.

` + `Similique sunt in culpa qui officia deserunt mollitia animi, id est laborum et ` +
	`dolorum fuga. Et harum quidem rerum facilis est et expedita distinctio quae ab illo ` +
	`inventore veritatis et quasi architecto beatae vitae dicta sunt explicabo.`

type App struct {
	Text string
}

func main() {
	gui.SetTheme(gui.ThemeDark.WithBorders(true))

	w := gui.NewWindow(gui.WindowCfg{
		State:  &App{Text: loremText},
		Title:  "Multiline Input Demo",
		Width:  400,
		Height: 300,
		OnInit: func(w *gui.Window) {
			w.UpdateView(mainView)
			w.SetFocus(inputFocusID)
		},
	})

	backend.Run(w)
}

func mainView(w *gui.Window) gui.View {
	ww, wh := w.WindowSize()
	app := gui.State[App](w)

	return gui.Column(gui.ContainerCfg{
		Width:  float32(ww),
		Height: float32(wh),
		Sizing: gui.FixedFixed,
		Content: []gui.View{
			gui.Input(gui.InputCfg{
				ID:        inputFocusID,
				Focusable: true,
				IDScroll:  inputIDScroll,
				Mode:      gui.InputMultiline,
				Sizing:    gui.FillFill,
				Text:      app.Text,
				OnTextChanged: func(_ *gui.Layout, s string, w *gui.Window) {
					gui.State[App](w).Text = s
				},
			}),
		},
	})
}
