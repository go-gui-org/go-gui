package main

import (
	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/highlight"
)

func detailPanel(w *gui.Window) gui.View {
	app := gui.State[ShowcaseApp](w)
	entries := filteredEntries(app)

	if len(entries) == 0 {
		return gui.Column(gui.ContainerCfg{
			IDScroll:   scrollDetail,
			Sizing:     gui.FillFill,
			SizeBorder: gui.NoBorder,
			Padding:    gui.Some(detailPanelPadding()),
			ScrollbarCfgY: &gui.ScrollbarCfg{
				GapEdge: 4,
			},
			Content: []gui.View{
				gui.Text(gui.TextCfg{
					Text:      "No component matches filter",
					TextStyle: gui.CurrentTheme().B2,
				}),
			},
		})
	}

	if !hasEntry(entries, app.SelectedComponent) {
		app.SelectedComponent = preferredComponentForGroup(entries)
	}

	entry := selectedEntry(entries, app.SelectedComponent)

	// Stop shader animation when not viewing the shader demo.
	// QueueCommand defers to next frame (view functions hold w.mu).
	if entry.ID != "shader" {
		w.QueueCommand(func(w *gui.Window) {
			w.AnimationRemove("shader_tick")
		})
	}
	// Discard stale lazy-loaded tree nodes when navigating away.
	// Bump abort counter so in-flight goroutines discard their results.
	if entry.ID != "tree" {
		app.TreeLazyNodes = make(map[string][]gui.TreeNodeCfg)
		app.TreeLazyLoadAbort++
	}

	var content gui.View
	switch {
	case entry.ID == "":
		content = demoPlaceholder(gui.CurrentTheme(), "No demo configured")
	case app.ShowDocs && entry.Group != groupWelcome:
		style := gui.DefaultMarkdownStyle()
		style.H2 = gui.CurrentTheme().B3
		style.CodeHighlighter = highlight.Default()
		content = w.Markdown(gui.MarkdownCfg{
			ID:      "doc-" + entry.ID,
			Source:  componentDoc(entry.ID),
			Padding: gui.NoPadding,
			Style:   style,
		})
	default:
		content = componentDemo(w, entry.ID)
	}

	return gui.Column(gui.ContainerCfg{
		IDScroll:   scrollDetail,
		Sizing:     gui.FillFill,
		Color:      gui.CurrentTheme().ColorBackground,
		SizeBorder: gui.NoBorder,
		Padding:    gui.Some(detailPanelPadding()),
		Spacing:    gui.Some(gui.CurrentTheme().SpacingLarge),
		ScrollbarCfgY: &gui.ScrollbarCfg{
			GapEdge: 4,
		},
		Content: []gui.View{
			viewTitleBar(entry, app.ShowDocs),
			gui.Text(gui.TextCfg{Text: entry.Summary, TextStyle: gui.CurrentTheme().N3, Mode: gui.TextModeWrap}),
			content,
			line(),
			relatedExamplesFooter(entry.ID),
		},
	})
}

func detailPanelPadding() gui.Padding {
	base := gui.CurrentTheme().PaddingLarge
	base.Right += gui.CurrentTheme().ScrollbarStyle.Size + 4
	return base
}

func viewTitleBar(entry DemoEntry, showDocs bool) gui.View {
	titleContent := []gui.View{
		gui.Text(gui.TextCfg{Text: entry.Label, TextStyle: gui.CurrentTheme().B1}),
	}
	if entry.ID != "welcome" {
		titleContent = append(titleContent,
			gui.Row(gui.ContainerCfg{
				Sizing:     gui.FillFit,
				Padding:    gui.NoPadding,
				SizeBorder: gui.NoBorder,
			}),
			docButton(showDocs),
		)
	}
	return gui.Column(gui.ContainerCfg{
		Sizing:     gui.FillFit,
		Spacing:    gui.NoSpacing,
		Padding:    gui.NoPadding,
		SizeBorder: gui.NoBorder,
		Content: []gui.View{
			gui.Row(gui.ContainerCfg{
				Sizing:     gui.FillFit,
				Padding:    gui.NoPadding,
				SizeBorder: gui.NoBorder,
				VAlign:     gui.VAlignMiddle,
				Content:    titleContent,
			}),
			line(),
		},
	})
}

func docButton(showDocs bool) gui.View {
	color := gui.ColorTransparent
	if showDocs {
		color = gui.CurrentTheme().ColorActive
	}
	return gui.Button(gui.ButtonCfg{
		ID:         "btn-doc-toggle",
		Focusable:  true,
		A11YLabel:  "Toggle docs",
		Color:      color,
		SizeBorder: gui.NoBorder,
		Padding:    gui.SomeP(4, 8, 4, 8),
		Radius:     gui.SomeF(3),
		Content: []gui.View{
			gui.Text(gui.TextCfg{Text: gui.IconBook, TextStyle: gui.CurrentTheme().Icon4}),
		},
		OnClick: func(_ *gui.Layout, e *gui.Event, w *gui.Window) {
			gui.State[ShowcaseApp](w).ShowDocs = !gui.State[ShowcaseApp](w).ShowDocs
			e.IsHandled = true
		},
	})
}

var componentDemos = map[string]func(*gui.Window) gui.View{
	"audio":               demoAudio,
	"welcome":             demoWelcome,
	"button":              demoButton,
	"input":               demoInput,
	"toggle":              demoToggle,
	"switch":              demoSwitch,
	"radio":               demoRadio,
	"radio_group":         demoRadioGroup,
	"combobox":            demoCombobox,
	"select":              demoSelect,
	"listbox":             demoListBox,
	"slider":              demoSlider,
	"progress_bar":        demoProgressBar,
	"pulsar":              demoPulsar,
	"toast":               demoToast,
	"badge":               demoBadge,
	"skeleton":            demoSkeleton,
	"native_notification": demoNotification,
	"breadcrumb":          demoBreadcrumb,
	"menus":               demoMenus,
	"dialog":              demoDialog,
	"tree":                demoTree,
	"drag_reorder":        demoDragReorder,
	"printing":            demoPrinting,
	"text":                demoText,
	"rtf":                 demoRtf,
	"table":               demoTable,
	"data_grid":           demoDataGrid,
	"data_source":         demoDataSource,
	"date_picker":         demoDatePicker,
	"input_date":          demoInputDate,
	"numeric_input":       demoNumericInput,
	"forms":               demoForms,
	"date_picker_roller":  demoDatePickerRoller,
	"svg":                 demoSvg,
	"image":               demoImage,
	"expand_panel":        demoExpandPanel,
	"icons":               demoIcons,
	"blur":                demoBlur,
	"color_filter":        demoColorFilter,
	"gradient":            demoGradient,
	"box_shadows":         demoBoxShadows,
	"shader":              demoShader,
	"draw_canvas":         demoDrawCanvas,
	"gesture":             demoGesture,
	"animations":          demoAnimations,
	"color_picker":        demoColorPicker,
	"theme_gen":           demoThemeGen,
	"markdown":            demoMarkdown,
	"tab_control":         demoTabControl,
	"command_palette":     demoCommandPalette,
	"context_menu":        demoContextMenu,
	"tooltip":             demoTooltip,
	"inspector":           demoInspector,
	"rectangle":           demoRectangle,
	"scrollbar":           demoScrollbar,
	"splitter":            demoSplitter,
	"rotated_box":         demoRotatedBox,
	"row":                 demoRow,
	"column_demo":         demoColumn,
	"wrap_panel":          demoWrapPanel,
	"overflow_panel":      demoOverflowPanel,
	"sidebar":             demoSidebar,
	"dock_layout":         demoDockLayout,
	"command_button":      demoCommandButton,
	"theme_picker":        demoThemePicker,
	"multi_window":        demoMultiWindow,
	"spinner":             demoMathSpinner,
	"svg_spinner":         demoSvgSpinner,
}

func componentDemo(w *gui.Window, id string) gui.View {
	if id == "commands" {
		return showcaseMarkdownPanel(w, "showcase-commands", docPageSource("commands"))
	}
	if fn, ok := componentDemos[id]; ok {
		return fn(w)
	}
	return demoPlaceholder(gui.CurrentTheme(), "Demo: "+id)
}

func demoPlaceholder(t gui.Theme, text string) gui.View {
	return gui.Column(gui.ContainerCfg{
		Sizing:  gui.FillFit,
		Color:   t.ColorPanel,
		Padding: gui.SomeP(24, 24, 24, 24),
		Radius:  gui.SomeF(8),
		Content: []gui.View{
			gui.Text(gui.TextCfg{Text: text, TextStyle: t.N3, Mode: gui.TextModeWrap}),
		},
	})
}

func relatedExamplesFooter(id string) gui.View {
	return gui.Text(gui.TextCfg{
		Text:      "Related examples: " + relatedExamples(id),
		TextStyle: gui.CurrentTheme().N5,
		Mode:      gui.TextModeWrap,
	})
}
