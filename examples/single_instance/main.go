// This example demonstrates single-instance mode and open events: a second launch hands its files and URLs to the running copy, which lists them.
package main

import (
	_ "embed"
	"flag"
	"log"
	"os"
	"time"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/appinfo"
	"github.com/go-gui-org/go-gui/gui/backend"
	"github.com/go-gui-org/go-gui/gui/backend/soft"
	"github.com/go-gui-org/go-gui/gui/instance"
)

//go:embed appinfo.toml
var manifest []byte

var info = appinfo.MustParse(manifest)

// DemoApp is the window state: every open request so far, newest last.
type DemoApp struct {
	Opened []string
}

func newWindow() *gui.Window {
	return gui.NewWindow(gui.WindowCfg{
		AppInfo: info,
		State:   &DemoApp{},
		Width:   560,
		Height:  360,
		OnInit: func(w *gui.Window) {
			w.SetView(mainView)
		},
	})
}

// onOpen runs on the main thread for each open request: this
// launch's own arguments, a forwarded second launch, or (macOS) a
// file or URL the OS sends to the bundle.
func onOpen(a *gui.App, ev gui.OpenEvent) {
	ws := a.Windows()
	if len(ws) == 0 {
		return
	}
	s := gui.State[DemoApp](ws[0])
	stamp := time.Now().Format("15:04:05 ")
	for _, f := range ev.Files {
		s.Opened = append(s.Opened, stamp+"file "+f)
	}
	for _, u := range ev.URLs {
		s.Opened = append(s.Opened, stamp+"url  "+u)
	}
}

func main() {
	screenshot := flag.String("screenshot", "", "write screenshot and exit")
	flag.Parse()

	if *screenshot != "" {
		w := newWindow()
		gui.State[DemoApp](w).Opened = []string{
			"12:00:01 file /Users/me/notes.txt",
			"12:00:07 url  single-instance://open?doc=42",
		}
		if err := soft.RenderToPNG(w, 2, *screenshot); err != nil {
			log.Fatalf("screenshot: %v", err)
		}
		os.Exit(0)
	}

	run()
}

// doc:snippet-begin single-instance
// The quoted block below is pinned by
// examples/showcase/sound_snippet_test.go against README.md.

// run elects this process or forwards its arguments, then runs the app.
func run() {
	lock, primary, err := instance.Acquire(info.ID)
	if err != nil {
		log.Printf("single instance: %v", err)
	}
	if !primary {
		return // the running copy has this launch's arguments
	}
	defer func() { _ = lock.Close() }()

	app := gui.NewApp()
	app.OnOpen = onOpen
	lock.Attach(app)
	backend.RunApp(app, newWindow())
}

// doc:snippet-end single-instance

func mainView(w *gui.Window) gui.View {
	theme := gui.CurrentTheme()
	s := gui.State[DemoApp](w)
	rows := make([]gui.View, 0, len(s.Opened)+3)
	rows = append(rows,
		gui.Text(gui.TextCfg{Text: info.Name, TextStyle: theme.TextStyleDisplay}),
		gui.Text(gui.TextCfg{
			Text:      "Launch the app again with file or URL arguments; they appear here.",
			TextStyle: theme.TextStyleSecondary,
		}),
	)
	if len(s.Opened) == 0 {
		rows = append(rows, gui.Text(gui.TextCfg{
			Text:      "Nothing opened yet.",
			TextStyle: theme.TextStylePlaceholder,
		}))
	}
	for _, line := range s.Opened {
		rows = append(rows, gui.Text(gui.TextCfg{Text: line, TextStyle: theme.TextStyleCode}))
	}
	return gui.Column(gui.ContainerCfg{
		Sizing:     gui.FillFill,
		Padding:    theme.PaddingLarge,
		SizeBorder: gui.NoBorder,
		Content:    rows,
	})
}
