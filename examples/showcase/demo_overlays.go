package main

import (
	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/highlight"
)

func demoDialog(w *gui.Window) gui.View {
	t := gui.CurrentTheme()
	app := gui.State[ShowcaseApp](w)
	return gui.Column(gui.ContainerCfg{
		Sizing:  gui.FillFit,
		Spacing: gui.SomeF(12),
		Padding: gui.NoPadding,
		Content: []gui.View{
			gui.Row(gui.ContainerCfg{
				Sizing:  gui.FillFit,
				Spacing: gui.SomeF(8),
				Padding: gui.NoPadding,
				Content: []gui.View{
					gui.Button(gui.ButtonCfg{
						ID:      "btn-dialog-msg",
						Padding: gui.SomeP(8, 16, 8, 16),
						Content: []gui.View{
							gui.Text(gui.TextCfg{Text: "Message", TextStyle: t.N3}),
						},
						OnClick: func(_ *gui.Layout, e *gui.Event, w *gui.Window) {
							w.Dialog(gui.DialogCfg{
								Title:      "Information",
								Body:       "This is a message dialog.",
								DialogType: gui.DialogMessage,
								OnOkYes: func(w *gui.Window) {
									gui.State[ShowcaseApp](w).DialogResult = "OK"
								},
							})
							e.IsHandled = true
						},
					}),
					gui.Button(gui.ButtonCfg{
						ID:      "btn-dialog-confirm",
						Padding: gui.SomeP(8, 16, 8, 16),
						Content: []gui.View{
							gui.Text(gui.TextCfg{Text: "Confirm", TextStyle: t.N3}),
						},
						OnClick: func(_ *gui.Layout, e *gui.Event, w *gui.Window) {
							w.Dialog(gui.DialogCfg{
								Title:      "Confirm Action",
								Body:       "Are you sure you want to proceed?",
								DialogType: gui.DialogConfirm,
								OnOkYes: func(w *gui.Window) {
									gui.State[ShowcaseApp](w).DialogResult = "Yes"
								},
								OnCancelNo: func(w *gui.Window) {
									gui.State[ShowcaseApp](w).DialogResult = "No"
								},
							})
							e.IsHandled = true
						},
					}),
					gui.Button(gui.ButtonCfg{
						ID:      "btn-dialog-prompt",
						Padding: gui.SomeP(8, 16, 8, 16),
						Content: []gui.View{
							gui.Text(gui.TextCfg{Text: "Prompt", TextStyle: t.N3}),
						},
						OnClick: func(_ *gui.Layout, e *gui.Event, w *gui.Window) {
							w.Dialog(gui.DialogCfg{
								Title:      "Input Required",
								Body:       "Enter your name:",
								DialogType: gui.DialogPrompt,
								OnReply: func(reply string, w *gui.Window) {
									gui.State[ShowcaseApp](w).DialogResult = "Reply: " + reply
								},
								OnCancelNo: func(w *gui.Window) {
									gui.State[ShowcaseApp](w).DialogResult = "Cancelled"
								},
							})
							e.IsHandled = true
						},
					}),
					gui.Button(gui.ButtonCfg{
						ID:      "btn-dialog-custom",
						Padding: gui.SomeP(8, 16, 8, 16),
						Content: []gui.View{
							gui.Text(gui.TextCfg{Text: "Custom", TextStyle: t.N3}),
						},
						OnClick: func(_ *gui.Layout, e *gui.Event, w *gui.Window) {
							w.Dialog(gui.DialogCfg{
								Title:      "Custom Dialog",
								DialogType: gui.DialogCustom,
								CustomContent: []gui.View{
									gui.Column(gui.ContainerCfg{
										Sizing:  gui.FillFit,
										Spacing: gui.SomeF(8),
										Padding: gui.SomeP(8, 8, 8, 8),
										Content: []gui.View{
											gui.Text(gui.TextCfg{
												Text:      "This dialog has custom content.",
												TextStyle: t.N3,
											}),
											gui.ProgressBar(gui.ProgressBarCfg{
												ID:       "showcase-dialog-pb",
												Percent:  0.65,
												TextShow: true,
												Sizing:   gui.FillFit,
											}),
										},
									}),
								},
								OnOkYes: func(w *gui.Window) {
									gui.State[ShowcaseApp](w).DialogResult = "Custom OK"
								},
							})
							e.IsHandled = true
						},
					}),
				},
			}),
			line(),
			sectionLabel(t, "Native File Dialogs"),
			gui.Row(gui.ContainerCfg{
				Sizing:  gui.FillFit,
				Spacing: gui.SomeF(8),
				Padding: gui.NoPadding,
				Content: []gui.View{
					gui.Button(gui.ButtonCfg{
						ID:      "btn-open-file",
						Padding: gui.SomeP(8, 16, 8, 16),
						Content: []gui.View{
							gui.Text(gui.TextCfg{Text: gui.IconFolder, TextStyle: t.N3}),
							gui.Text(gui.TextCfg{Text: "Open", TextStyle: t.N3}),
						},
						OnClick: func(_ *gui.Layout, e *gui.Event, w *gui.Window) {
							np := w.NativePlatformBackend()
							if np == nil {
								gui.State[ShowcaseApp](w).DialogResult = "No native platform"
								e.IsHandled = true
								return
							}
							r := np.ShowOpenDialog("Open File", "", nil, false)
							a := gui.State[ShowcaseApp](w)
							if len(r.Paths) > 0 {
								a.DialogResult = "Opened: " + r.Paths[0].Path
							} else {
								a.DialogResult = "Open cancelled"
							}
							e.IsHandled = true
						},
					}),
					gui.Button(gui.ButtonCfg{
						ID:      "btn-save-file",
						Padding: gui.SomeP(8, 16, 8, 16),
						Content: []gui.View{
							gui.Text(gui.TextCfg{Text: gui.IconDownload, TextStyle: t.N3}),
							gui.Text(gui.TextCfg{Text: "Save", TextStyle: t.N3}),
						},
						OnClick: func(_ *gui.Layout, e *gui.Event, w *gui.Window) {
							np := w.NativePlatformBackend()
							if np == nil {
								gui.State[ShowcaseApp](w).DialogResult = "No native platform"
								e.IsHandled = true
								return
							}
							r := np.ShowSaveDialog("Save File", "", "untitled.txt", ".txt", nil, true)
							a := gui.State[ShowcaseApp](w)
							if len(r.Paths) > 0 {
								a.DialogResult = "Save: " + r.Paths[0].Path
							} else {
								a.DialogResult = "Save cancelled"
							}
							e.IsHandled = true
						},
					}),
					gui.Button(gui.ButtonCfg{
						ID:      "btn-folder",
						Padding: gui.SomeP(8, 16, 8, 16),
						Content: []gui.View{
							gui.Text(gui.TextCfg{Text: gui.IconFolder, TextStyle: t.N3}),
							gui.Text(gui.TextCfg{Text: "Folder", TextStyle: t.N3}),
						},
						OnClick: func(_ *gui.Layout, e *gui.Event, w *gui.Window) {
							np := w.NativePlatformBackend()
							if np == nil {
								gui.State[ShowcaseApp](w).DialogResult = "No native platform"
								e.IsHandled = true
								return
							}
							r := np.ShowFolderDialog("Select Folder", "")
							a := gui.State[ShowcaseApp](w)
							if len(r.Paths) > 0 {
								a.DialogResult = "Folder: " + r.Paths[0].Path
							} else {
								a.DialogResult = "Folder cancelled"
							}
							e.IsHandled = true
						},
					}),
				},
			}),
			gui.Text(gui.TextCfg{
				Text:      "Result: " + app.DialogResult,
				TextStyle: t.N3,
			}),
		},
	})
}

func demoNotification(w *gui.Window) gui.View {
	t := gui.CurrentTheme()
	app := gui.State[ShowcaseApp](w)
	return gui.Column(gui.ContainerCfg{
		Sizing:  gui.FillFit,
		Spacing: gui.SomeF(12),
		Padding: gui.NoPadding,
		Content: []gui.View{
			gui.Button(gui.ButtonCfg{
				ID:      "btn-notify",
				Padding: gui.SomeP(8, 16, 8, 16),
				Content: []gui.View{
					gui.Text(gui.TextCfg{Text: gui.IconBell, TextStyle: t.N3}),
					gui.Text(gui.TextCfg{Text: "Send Notification", TextStyle: t.N3}),
				},
				OnClick: func(_ *gui.Layout, e *gui.Event, w *gui.Window) {
					w.NativeNotification(gui.NativeNotificationCfg{
						Title: "showcase",
						Body:  "Hello from go-gui showcase!",
						OnDone: func(r gui.NativeNotificationResult, w *gui.Window) {
							a := gui.State[ShowcaseApp](w)
							switch r.Status {
							case gui.NotificationOK:
								a.NotifyResult = "Notification sent"
							case gui.NotificationDenied:
								a.NotifyResult = "Permission denied"
							default:
								a.NotifyResult = "Error: " + r.ErrorMessage
							}
						},
					})
					e.IsHandled = true
				},
			}),
			gui.Text(gui.TextCfg{
				Text:      app.NotifyResult,
				TextStyle: t.N3,
			}),
		},
	})
}

func demoInspector(w *gui.Window) gui.View {
	t := gui.CurrentTheme()
	inspectorStyle := gui.DefaultMarkdownStyle()
	inspectorStyle.CodeHighlighter = highlight.Default()
	return gui.Column(gui.ContainerCfg{
		Sizing:  gui.FillFit,
		Spacing: gui.SomeF(16),
		Padding: gui.NoPadding,
		Content: []gui.View{
			w.Markdown(gui.MarkdownCfg{
				ID:      "inspector-info",
				Padding: gui.NoPadding,
				Style:   inspectorStyle,
				Source: `Press **F12** to toggle the inspector. ` +
					`Excluded in prod builds (` + "`-tags prod`" + `).

## Keyboard Shortcuts

| Key | Action |
|-----|--------|
| F12 | Toggle inspector on/off |
| Alt+Left / Alt+Right | Resize panel |
| Alt+Up | Move panel to opposite side |

## Features

- **Layout tree** — navigable tree of every node with type, size, and ID
- **Property detail** — select a node to see position, size, sizing, ` +
					`padding, spacing, color, radius, focus, scroll, alignment, ` +
					`float, clip, opacity, events, and child count
- **Wireframe highlight** — selected node outlined in cyan; ` +
					`padding area in green`,
			}),
			sectionLabel(t, "Try It"),
			gui.Text(gui.TextCfg{
				Text:      "Press F12 to open the inspector now.",
				TextStyle: t.N3,
			}),
		},
	})
}

func demoContextMenu(w *gui.Window) gui.View {
	t := gui.CurrentTheme()
	app := gui.State[ShowcaseApp](w)

	action := func(id string, e *gui.Event, w *gui.Window) {
		gui.State[ShowcaseApp](w).ContextMenuResult = id
		e.IsHandled = true
	}

	return gui.Column(gui.ContainerCfg{
		Sizing:  gui.FillFit,
		Spacing: gui.SomeF(12),
		Padding: gui.NoPadding,
		Content: []gui.View{
			sectionLabel(t, "Basic Context Menu"),
			gui.ContextMenu(w, gui.ContextMenuCfg{
				ID:     "ctx-basic",
				Sizing: gui.FillFit,
				Items: []gui.MenuItemCfg{
					{ID: "cut", Text: "Cut"},
					{ID: "copy", Text: "Copy"},
					{ID: "paste", Text: "Paste"},
					gui.MenuSeparator(),
					{ID: "delete", Text: "Delete"},
				},
				Action: action,
				Content: []gui.View{
					gui.Column(gui.ContainerCfg{
						Sizing:  gui.FillFit,
						Color:   t.ColorPanel,
						Padding: gui.SomeP(24, 24, 24, 24),
						Radius:  gui.SomeF(8),
						Content: []gui.View{
							gui.Text(gui.TextCfg{
								Text:      "Right-click here for a basic menu",
								TextStyle: t.N3,
							}),
						},
					}),
				},
			}),
			line(),
			sectionLabel(t, "With Submenus and Subtitles"),
			gui.ContextMenu(w, gui.ContextMenuCfg{
				ID:     "ctx-sub",
				Sizing: gui.FillFit,
				Items: []gui.MenuItemCfg{
					gui.MenuSubtitle("Edit"),
					{ID: "cut2", Text: "Cut"},
					{ID: "copy2", Text: "Copy"},
					{ID: "paste2", Text: "Paste"},
					gui.MenuSeparator(),
					gui.MenuSubmenu("format", "Format", []gui.MenuItemCfg{
						{ID: "bold", Text: "Bold"},
						{ID: "italic", Text: "Italic"},
						{ID: "underline", Text: "Underline"},
					}),
					gui.MenuSeparator(),
					gui.MenuSubtitle("View"),
					{ID: "zoom_in", Text: "Zoom In"},
					{ID: "zoom_out", Text: "Zoom Out"},
				},
				Action: action,
				Content: []gui.View{
					gui.Column(gui.ContainerCfg{
						Sizing:  gui.FillFit,
						Color:   t.ColorPanel,
						Padding: gui.SomeP(24, 24, 24, 24),
						Radius:  gui.SomeF(8),
						Content: []gui.View{
							gui.Text(gui.TextCfg{
								Text:      "Right-click here for submenus",
								TextStyle: t.N3,
							}),
						},
					}),
				},
			}),
			line(),
			gui.Text(gui.TextCfg{
				Text:      "Selected: " + app.ContextMenuResult,
				TextStyle: t.N3,
			}),
		},
	})
}

func demoTooltip(w *gui.Window) gui.View {
	t := gui.CurrentTheme()
	return gui.Row(gui.ContainerCfg{
		Sizing:  gui.FillFit,
		Spacing: gui.SomeF(16),
		Padding: gui.NoPadding,
		Content: []gui.View{
			gui.WithTooltip(w, gui.WithTooltipCfg{
				Text: "This is a tooltip!",
				Content: []gui.View{
					gui.Button(gui.ButtonCfg{
						ID:      "btn-tooltip",
						Padding: gui.SomeP(8, 16, 8, 16),
						Content: []gui.View{
							gui.Text(gui.TextCfg{Text: "Hover me", TextStyle: t.N3}),
						},
					}),
				},
			}),
			gui.WithTooltip(w, gui.WithTooltipCfg{
				Text: "Another tooltip with more text",
				Content: []gui.View{
					gui.Badge(gui.BadgeCfg{
						Label:   "Tip",
						Variant: gui.BadgeInfo,
					}),
				},
			}),
		},
	})
}
