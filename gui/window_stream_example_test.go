package gui_test

import (
	"fmt"

	"github.com/go-gui-org/go-gui/gui"
)

// doc:snippet-begin stream
//
// Feed a background producer into the window through Stream: each
// line is applied on the frame thread and followed by a refresh, so
// the window repaints as lines arrive instead of stalling until the
// next mouse move (issue #559). Copy this shape and change only the
// state type, the channel element, and the apply body.

type streamLines struct {
	Lines []string
}

func streamLinesView(w *gui.Window) gui.View {
	state := gui.State[streamLines](w)
	text := ""
	if len(state.Lines) > 0 {
		text = state.Lines[len(state.Lines)-1]
	}
	return gui.Column(gui.ContainerCfg{
		Sizing:  gui.FillFill,
		Content: []gui.View{gui.Label(text, gui.TextStyle{})},
	})
}

func ExampleStream() {
	w := gui.NewWindow(gui.WindowCfg{State: &streamLines{}})
	defer w.WindowCleanup()
	w.SetView(streamLinesView)

	lines := make(chan string, 4)
	done := gui.Stream(w, lines, func(w *gui.Window, line string) {
		st := gui.State[streamLines](w)
		st.Lines = append(st.Lines, line)
	})
	lines <- "hello"
	lines <- "world"
	close(lines)
	<-done
	w.FrameFn()

	fmt.Println(gui.State[streamLines](w).Lines)
	// Output: [hello world]
}

// doc:snippet-end stream
