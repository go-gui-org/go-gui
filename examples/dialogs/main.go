// Dialogs demonstrates custom dialogs and native file dialogs.
package main

import (
	"strings"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend"
)

type App struct {
	LightTheme bool
}

func main() {
	gui.SetTheme(gui.ThemeDark.WithBorders(true))

	w := gui.NewWindow(gui.WindowCfg{
		State:  &App{},
		Title:  "dialogs",
		Width:  640,
		Height: 550,
		OnInit: func(w *gui.Window) {
			w.UpdateView(mainView)
		},
	})

	backend.Run(w)
}

func mainView(w *gui.Window) gui.View {
	ww, wh := w.WindowSize()
	app := gui.State[App](w)
	theme := gui.CurrentTheme()

	return gui.Column(gui.ContainerCfg{
		Width:  float32(ww),
		Height: float32(wh),
		Sizing: gui.FixedFixed,
		HAlign: gui.HAlignCenter,
		Content: []gui.View{
			toggleTheme(app, theme),
			gui.Column(gui.ContainerCfg{
				Title:       "Custom Dialogs",
				ColorBorder: theme.ColorActive,
				Padding:     gui.Some(gui.PaddingLarge),
				Content: []gui.View{
					messageButton(),
					confirmButton(),
					promptButton(),
					customButton(),
				},
			}),
			gui.Column(gui.ContainerCfg{
				Title:       "Native Dialogs",
				ColorBorder: theme.ColorActive,
				Padding:     gui.Some(gui.PaddingLarge),
				Content: []gui.View{
					nativeOpenButton(),
					nativeSaveButton(),
					nativeFolderButton(),
					nativeMessageButton(),
					nativeConfirmButton(),
				},
			}),
		},
	})
}

func messageButton() gui.View {
	return gui.Button(gui.ButtonCfg{
		IDFocus: 1,
		Sizing:  gui.FillFit,
		Content: []gui.View{gui.Text(gui.TextCfg{
			Text: "DialogMessage",
		})},
		OnClick: func(_ *gui.Layout, _ *gui.Event, w *gui.Window) {
			w.Dialog(gui.DialogCfg{
				AlignButtons: gui.HAlignEnd,
				DialogType:   gui.DialogMessage,
				Title:        "Title Displays Here",
				Body: "body text displays here...\n\n" +
					"Multi-line text supported.\n" +
					"See DialogCfg for other parameters\n\n" +
					"Buttons can be left/center/right aligned",
			})
		},
	})
}

func confirmButton() gui.View {
	return gui.Button(gui.ButtonCfg{
		IDFocus: 2,
		Sizing:  gui.FillFit,
		Content: []gui.View{gui.Text(gui.TextCfg{
			Text: "DialogConfirm",
		})},
		OnClick: func(_ *gui.Layout, _ *gui.Event, w *gui.Window) {
			w.Dialog(gui.DialogCfg{
				DialogType: gui.DialogConfirm,
				Title:      "Destroy All Data?",
				Body:       "Are you sure?",
				OnOkYes: func(w *gui.Window) {
					w.Dialog(gui.DialogCfg{Title: "Clicked Yes"})
				},
				OnCancelNo: func(w *gui.Window) {
					w.Dialog(gui.DialogCfg{Title: "Clicked No"})
				},
			})
		},
	})
}

func promptButton() gui.View {
	return gui.Button(gui.ButtonCfg{
		IDFocus: 3,
		Sizing:  gui.FillFit,
		Content: []gui.View{gui.Text(gui.TextCfg{
			Text: "DialogPrompt",
		})},
		OnClick: func(_ *gui.Layout, _ *gui.Event, w *gui.Window) {
			w.Dialog(gui.DialogCfg{
				DialogType: gui.DialogPrompt,
				Title:      "Monty Python Quiz",
				Body:       "What is your quest?",
				OnReply: func(reply string, w *gui.Window) {
					w.Dialog(gui.DialogCfg{
						Title: "Replied",
						Body:  reply,
					})
				},
				OnCancelNo: func(w *gui.Window) {
					w.Dialog(gui.DialogCfg{Title: "Canceled"})
				},
			})
		},
	})
}

func customButton() gui.View {
	return gui.Button(gui.ButtonCfg{
		IDFocus: 4,
		Sizing:  gui.FillFit,
		Content: []gui.View{gui.Text(gui.TextCfg{
			Text: "DialogCustom",
		})},
		OnClick: func(_ *gui.Layout, _ *gui.Event, w *gui.Window) {
			w.Dialog(gui.DialogCfg{
				DialogType: gui.DialogCustom,
				CustomContent: []gui.View{
					gui.Column(gui.ContainerCfg{
						HAlign: gui.HAlignCenter,
						VAlign: gui.VAlignMiddle,
						Content: []gui.View{
							gui.Text(gui.TextCfg{
								Text: "Custom Content",
							}),
							gui.Button(gui.ButtonCfg{
								IDFocus: 7568971, // dialogBaseIDFocus
								Content: []gui.View{gui.Text(gui.TextCfg{
									Text: "Close Me",
								})},
								OnClick: func(_ *gui.Layout, _ *gui.Event, w *gui.Window) {
									w.DialogDismiss()
								},
							}),
						},
					}),
				},
			})
		},
	})
}

func nativeOpenButton() gui.View {
	return gui.Button(gui.ButtonCfg{
		IDFocus: 5,
		Sizing:  gui.FillFit,
		Content: []gui.View{gui.Text(gui.TextCfg{
			Text: "NativeOpenDialog",
		})},
		OnClick: func(_ *gui.Layout, _ *gui.Event, w *gui.Window) {
			w.NativeOpenDialog(gui.NativeOpenDialogCfg{
				Title:         "Open Files",
				AllowMultiple: true,
				Filters: []gui.NativeFileFilter{
					{Name: "Images", Extensions: []string{
						"png", "jpg", "jpeg",
					}},
					{Name: "Docs", Extensions: []string{
						"txt", "md",
					}},
				},
				OnDone: func(r gui.NativeDialogResult, w *gui.Window) {
					showNativeResult("NativeOpenDialog", r, w)
				},
			})
		},
	})
}

func nativeSaveButton() gui.View {
	return gui.Button(gui.ButtonCfg{
		IDFocus: 6,
		Sizing:  gui.FillFit,
		Content: []gui.View{gui.Text(gui.TextCfg{
			Text: "NativeSaveDialog",
		})},
		OnClick: func(_ *gui.Layout, _ *gui.Event, w *gui.Window) {
			w.NativeSaveDialog(gui.NativeSaveDialogCfg{
				Title:            "Save As",
				DefaultName:      "untitled",
				DefaultExtension: "txt",
				Filters: []gui.NativeFileFilter{
					{Name: "Text", Extensions: []string{"txt"}},
				},
				OnDone: func(r gui.NativeDialogResult, w *gui.Window) {
					showNativeResult("NativeSaveDialog", r, w)
				},
			})
		},
	})
}

func nativeFolderButton() gui.View {
	return gui.Button(gui.ButtonCfg{
		IDFocus: 7,
		Sizing:  gui.FillFit,
		Content: []gui.View{gui.Text(gui.TextCfg{
			Text: "NativeFolderDialog",
		})},
		OnClick: func(_ *gui.Layout, _ *gui.Event, w *gui.Window) {
			w.NativeFolderDialog(gui.NativeFolderDialogCfg{
				Title: "Choose Folder",
				OnDone: func(r gui.NativeDialogResult, w *gui.Window) {
					showNativeResult("NativeFolderDialog", r, w)
				},
			})
		},
	})
}

func nativeMessageButton() gui.View {
	return gui.Button(gui.ButtonCfg{
		IDFocus: 8,
		Sizing:  gui.FillFit,
		Content: []gui.View{gui.Text(gui.TextCfg{
			Text: "NativeMessageDialog",
		})},
		OnClick: func(_ *gui.Layout, _ *gui.Event, w *gui.Window) {
			w.NativeMessageDialog(gui.NativeMessageDialogCfg{
				Title: "Native Message",
				Body:  "This is a native OS message dialog.",
				Level: gui.AlertInfo,
				OnDone: func(r gui.NativeAlertResult, w *gui.Window) {
					showAlertResult("NativeMessageDialog", r, w)
				},
			})
		},
	})
}

func nativeConfirmButton() gui.View {
	return gui.Button(gui.ButtonCfg{
		IDFocus: 9,
		Sizing:  gui.FillFit,
		Content: []gui.View{gui.Text(gui.TextCfg{
			Text: "NativeConfirmDialog",
		})},
		OnClick: func(_ *gui.Layout, _ *gui.Event, w *gui.Window) {
			w.NativeConfirmDialog(gui.NativeConfirmDialogCfg{
				Title: "Native Confirm",
				Body:  "Do you want to proceed?",
				Level: gui.AlertWarning,
				OnDone: func(r gui.NativeAlertResult, w *gui.Window) {
					showAlertResult("NativeConfirmDialog", r, w)
				},
			})
		},
	})
}

func showAlertResult(kind string, r gui.NativeAlertResult,
	w *gui.Window) {
	var body string
	switch r.Status {
	case gui.DialogOK:
		body = "OK / Yes"
	case gui.DialogCancel:
		body = "Canceled / No"
	case gui.DialogError:
		switch {
		case r.ErrorCode != "" && r.ErrorMessage != "":
			body = r.ErrorCode + ": " + r.ErrorMessage
		case r.ErrorMessage != "":
			body = r.ErrorMessage
		default:
			body = "Unknown error."
		}
	}
	w.Dialog(gui.DialogCfg{Title: kind, Body: body})
}

func showNativeResult(kind string, r gui.NativeDialogResult,
	w *gui.Window) {
	var body string
	switch r.Status {
	case gui.DialogOK:
		paths := r.PathStrings()
		if len(paths) == 0 {
			body = "No paths returned."
		} else {
			body = strings.Join(paths, "\n")
		}
	case gui.DialogCancel:
		body = "Canceled."
	case gui.DialogError:
		switch {
		case r.ErrorCode != "" && r.ErrorMessage != "":
			body = r.ErrorCode + ": " + r.ErrorMessage
		case r.ErrorMessage != "":
			body = r.ErrorMessage
		default:
			body = "Unknown error."
		}
	}
	w.Dialog(gui.DialogCfg{Title: kind, Body: body})
}

func toggleTheme(app *App, theme gui.Theme) gui.View {
	return gui.Row(gui.ContainerCfg{
		HAlign:  gui.HAlignEnd,
		Sizing:  gui.FillFit,
		Padding: gui.NoPadding,
		Content: []gui.View{
			gui.Toggle(gui.ToggleCfg{
				TextSelect:   gui.IconMoon,
				TextUnselect: gui.IconSunnyO,
				TextStyle:    theme.Icon3,
				Padding:      gui.Some(gui.PaddingSmall),
				Selected:     app.LightTheme,
				OnClick: func(_ *gui.Layout, _ *gui.Event, w *gui.Window) {
					a := gui.State[App](w)
					a.LightTheme = !a.LightTheme
					if a.LightTheme {
						w.SetTheme(gui.ThemeLight.WithBorders(true))
					} else {
						w.SetTheme(gui.ThemeDark.WithBorders(true))
					}
				},
			}),
		},
	})
}
