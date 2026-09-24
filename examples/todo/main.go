// This example demonstrates a small stateful app with a text input, action buttons, and a list.
// The todo example shows a small stateful GUI app with a text input,
// action buttons, and a list rendered from window state.
package main

import (
	"strings"

	"flag"
	"log"
	"os"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend"
	"github.com/go-gui-org/go-gui/gui/backend/soft"
)

var (
	colorPageBG      = gui.ColorFromString("#0f1d6b")
	colorCardBG      = gui.ColorFromString("#f7f7f8")
	colorInputBG     = gui.ColorFromString("#ececed")
	colorAccent      = gui.ColorFromString("#ff5b45")
	colorText        = gui.ColorFromString("#474747")
	colorMuted       = gui.ColorFromString("#9b9b9b")
	colorBorder      = gui.ColorFromString("#d9d9dd")
	colorStrike      = gui.ColorFromString("#b0b0b0")
	colorDeleteHover = gui.ColorFromString("#dcdcdf")
)

const (
	todoInputFocusID = "todo-input"
	windowWidth      = 540
	windowHeight     = 640
)

type todoItem struct {
	Title     string
	ID        int
	Completed bool
}

type appState struct {
	Draft  string
	Items  []todoItem
	NextID int
}

func newAppState() *appState {
	return &appState{
		NextID: 6,
		Items: []todoItem{
			{ID: 1, Title: "Add a task with the input above"},
			{ID: 2, Title: "Press Enter to add it to the list"},
			{ID: 3, Title: "Click the circle to mark it done", Completed: true},
			{ID: 4, Title: "Click × to delete a task"},
			{ID: 5, Title: "Drafts wait until you press Enter"},
		},
	}
}

func main() {
	screenshot := flag.String("screenshot", "", "write screenshot and exit")
	flag.Parse()

	gui.SetTheme(gui.ThemeLight.WithPadding(false))

	w := gui.NewWindow(gui.WindowCfg{
		State:  newAppState(),
		Title:  "todo",
		Width:  windowWidth,
		Height: windowHeight,
		OnInit: func(w *gui.Window) {
			// Render once and put the caret in the input field.
			w.SetView(mainView)
			w.SetFocus(todoInputFocusID)
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

func mainView(w *gui.Window) gui.View {
	return gui.Column(gui.ContainerCfg{
		Sizing:  gui.FillFill,
		Color:   colorPageBG,
		Padding: gui.NewPadding(12, 12, 12, 12),
		Content: []gui.View{
			cardView(w),
		},
	})
}

// The card fills what the page's 12px padding leaves, so it no longer
// needs the window size threaded in as "minus 24".
func cardView(w *gui.Window) gui.View {
	return gui.Column(gui.ContainerCfg{
		Sizing:      gui.FillFill,
		Color:       colorCardBG,
		Radius:      gui.SomeF(18),
		Padding:     gui.NewPadding(34, 34, 34, 34),
		Spacing:     gui.SomeF(22),
		ColorBorder: colorCardBG,
		Content: []gui.View{
			headerView(),
			composerView(w),
			listView(w),
		},
	})
}

func headerView() gui.View {
	titleStyle := gui.CurrentTheme().TextStyleDisplay
	titleStyle.Color = gui.ColorFromString("#15326f")
	iconStyle := gui.CurrentTheme().TextStyleIconXLarge
	iconStyle.Color = colorAccent
	return gui.Row(gui.ContainerCfg{
		Sizing:  gui.FillFit,
		Padding: gui.NoPadding,
		Spacing: gui.SomeF(10),
		VAlign:  gui.VAlignMiddle,
		Content: []gui.View{
			gui.Text(gui.TextCfg{
				Text:      "To-Do List",
				TextStyle: titleStyle,
			}),
			gui.Text(gui.TextCfg{
				Text:      gui.IconListTask,
				TextStyle: iconStyle,
			}),
		},
	})
}

func composerView(w *gui.Window) gui.View {
	app := gui.State[appState](w)
	theme := gui.CurrentTheme()
	inputStyle := theme.TextStyleBodyLarge
	inputStyle.Color = colorText
	placeholderStyle := theme.TextStyleBodyLarge
	placeholderStyle.Color = colorMuted
	addStyle := theme.TextStyleTitle
	addStyle.Color = gui.RGB(255, 255, 255)

	return gui.Row(gui.ContainerCfg{
		Sizing:  gui.FillFit,
		Padding: gui.NoPadding,
		VAlign:  gui.VAlignMiddle,
		Content: []gui.View{
			gui.Input(gui.InputCfg{
				ID:               todoInputFocusID,
				Sizing:           gui.FillFit,
				Text:             app.Draft,
				Placeholder:      "Add your task",
				Color:            colorInputBG,
				Colors:           gui.ColorSet{Hover: colorInputBG, Border: colorInputBG, BorderFocus: colorAccent},
				Radius:           gui.SomeF(20),
				Padding:          gui.NewPadding(18, 20, 18, 20),
				TextStyle:        inputStyle,
				PlaceholderStyle: placeholderStyle,
				OnTextChanged: func(text string, ctx gui.EventCtx) {
					// Keep the input fully controlled by app state.
					gui.State[appState](ctx.Window).Draft = text
				},
				OnTextCommit: func(
					text string, reason gui.InputCommitReason,
					ctx gui.EventCtx,
				) {
					// Enter means "add this"; a commit fired by blur
					// only means focus moved on. Tabbing out of a
					// half-typed task should leave the draft alone,
					// not silently create a todo.
					if reason != gui.InputCommitEnter {
						return
					}
					addTodo(ctx.Window, text)
				},
			}),
			gui.Button(gui.ButtonCfg{
				ID: "add-todo",
				// The button keeps one appearance through hover,
				// press and focus; Flat says that in a line.
				Colors:   gui.Flat(colorAccent),
				Radius:   gui.SomeF(20),
				Padding:  gui.NewPadding(18, 28, 18, 28),
				MinWidth: 140,
				Content: []gui.View{
					gui.Text(gui.TextCfg{
						Text:      "ADD",
						TextStyle: addStyle,
					}),
				},
				OnClick: func(ctx gui.EventCtx) {
					addTodo(ctx.Window, gui.State[appState](ctx.Window).Draft)
				},
			}),
		},
	})
}

func listView(w *gui.Window) gui.View {
	app := gui.State[appState](w)
	content := make([]gui.View, 0, len(app.Items))
	for _, item := range app.Items {
		content = append(content, todoRowView(item))
	}

	return gui.Column(gui.ContainerCfg{
		Sizing:  gui.FillFill,
		Padding: gui.NoPadding,
		Spacing: gui.SomeF(14),
		Content: content,
	})
}

func todoRowView(item todoItem) gui.View {
	return gui.Row(gui.ContainerCfg{
		Sizing:  gui.FillFit,
		Padding: gui.NoPadding,
		Spacing: gui.SomeF(12),
		VAlign:  gui.VAlignMiddle,
		Content: []gui.View{
			completeButton(item),
			gui.Text(gui.TextCfg{
				Text:      item.Title,
				TextStyle: itemTextStyle(item.Completed),
				Sizing:    gui.FillFit,
			}),
			deleteButton(item.ID),
		},
	})
}

func completeButton(item todoItem) gui.View {
	checkStyle := gui.CurrentTheme().TextStyleTitle
	checkStyle.Color = gui.RGB(255, 255, 255)
	cfg := gui.ButtonCfg{
		ID:      gui.ScopeIDN("todo", "check", item.ID),
		Width:   32,
		Height:  32,
		Sizing:  gui.FixedFixed,
		Radius:  gui.SomeF(16),
		Padding: gui.NoPadding,
		OnClick: func(ctx gui.EventCtx) {
			toggleTodo(ctx.Window, item.ID)
		},
	}

	if item.Completed {
		// Completed items keep one appearance in every state.
		cfg.Colors = gui.Flat(colorAccent)
		cfg.Content = []gui.View{
			gui.Text(gui.TextCfg{
				Text:      "✓",
				TextStyle: checkStyle,
			}),
		}
		return gui.Button(cfg)
	}

	// Fill stays constant; only the border reacts to focus.
	cfg.Colors = gui.ColorSet{
		Base:        colorCardBG,
		Border:      colorBorder,
		BorderFocus: colorAccent,
	}
	cfg.SizeBorder = gui.SomeF(2)
	cfg.Content = []gui.View{gui.Text(gui.TextCfg{Text: ""})}
	return gui.Button(cfg)
}

func deleteButton(id int) gui.View {
	deleteStyle := gui.CurrentTheme().TextStyleDisplay
	deleteStyle.Color = colorMuted
	return gui.Button(gui.ButtonCfg{
		ID:      gui.ScopeIDN("todo", "delete", id),
		Width:   28,
		Height:  28,
		Sizing:  gui.FixedFixed,
		Color:   gui.ColorTransparent,
		Colors:  gui.ColorSet{Hover: colorDeleteHover, Click: colorDeleteHover, Focus: colorDeleteHover, Border: gui.ColorTransparent, BorderFocus: gui.ColorTransparent},
		Radius:  gui.SomeF(14),
		Padding: gui.NoPadding,
		Content: []gui.View{
			gui.Text(gui.TextCfg{
				Text:      "×",
				TextStyle: deleteStyle,
			}),
		},
		OnClick: func(ctx gui.EventCtx) {
			deleteTodo(ctx.Window, id)
		},
	})
}

func itemTextStyle(completed bool) gui.TextStyle {
	style := gui.CurrentTheme().TextStyleBodyLarge
	style.Color = colorText
	if completed {
		style.Color = colorStrike
		style.Strikethrough = true
	}
	return style
}

func addTodo(w *gui.Window, title string) {
	title = strings.TrimSpace(title)
	if title == "" {
		return
	}

	app := gui.State[appState](w)
	app.Items = append(app.Items, todoItem{
		ID:    app.NextID,
		Title: title,
	})
	app.NextID++
	app.Draft = ""
	// Re-focus the input so the next task can be entered immediately.
	w.SetFocus(todoInputFocusID)
}

func toggleTodo(w *gui.Window, id int) {
	app := gui.State[appState](w)
	for i := range app.Items {
		if app.Items[i].ID != id {
			continue
		}
		app.Items[i].Completed = !app.Items[i].Completed
		return
	}
}

func deleteTodo(w *gui.Window, id int) {
	app := gui.State[appState](w)
	items := app.Items[:0]
	// Reuse the existing backing array to keep the example simple and cheap.
	for _, item := range app.Items {
		if item.ID == id {
			continue
		}
		items = append(items, item)
	}
	app.Items = items
}
