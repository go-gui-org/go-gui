package main

import (
	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/highlight"
)

func detailPanel(w *gui.Window) gui.View {
	app := appState(w)
	entries := filteredEntries(app)

	if len(entries) == 0 {
		return gui.Column(gui.ContainerCfg{
			ID:         scrollDetail,
			Scrollable: true,
			Sizing:     gui.FillFill,
			SizeBorder: gui.NoBorder,
			Padding:    detailPanelPadding(),

			Content: []gui.View{
				gui.Text(gui.TextCfg{
					Text:      "No component matches filter",
					TextStyle: gui.CurrentTheme().TextStyleTitle,
				}),
			},
		})
	}

	// Generation must not write state: resolve locally and defer
	// the repair so a filter miss heals next frame.
	selected := app.SelectedComponent
	if !hasEntry(entries, selected) {
		selected = preferredComponentForGroup(entries)
		w.QueueCommand(func(w *gui.Window) {
			fixed := appState(w)
			latest := filteredEntries(fixed)
			if !hasEntry(latest, fixed.SelectedComponent) {
				fixed.SelectedComponent = preferredComponentForGroup(latest)
				w.InvalidateLayout()
			}
		})
	}

	entry := selectedEntry(entries, selected)

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
	if entry.ID == "" {
		content = demoPlaceholder(gui.CurrentTheme(), "No demo configured")
	} else {
		content = componentDemo(w, entry.ID)
	}

	// A doc page is its own doc, so it gets no second copy. This keys
	// on the page, not its catalog group: groups move (#1025).
	doc := ""
	if !isDocPage(entry.ID) {
		doc = componentDoc(entry.ID)
	}

	body := []gui.View{
		viewTitleBar(entry, doc != ""),
		gui.Text(gui.TextCfg{Text: entry.Summary, TextStyle: gui.CurrentTheme().TextStyleBody, Mode: gui.TextModeWrap}),
		content,
	}
	// The docs go under the demo, not behind a toggle: a reader who
	// scrolls finds them, and the demo stays in view while they read.
	if doc != "" {
		body = append(body, docsSection(w, entry.ID, doc))
	}
	body = append(body, line(), relatedExamplesFooter(entry.ID))

	return gui.Column(gui.ContainerCfg{
		ID:         scrollDetail,
		Scrollable: true,
		Sizing:     gui.FillFill,
		Color:      gui.CurrentTheme().ColorBackground,
		SizeBorder: gui.NoBorder,
		Padding:    detailPanelPadding(),

		Spacing: gui.SpacingPx(gui.CurrentTheme().SpacingLarge),
		Content: body,
	})
}

// docsSection is the "Documentation" heading and the page's widget
// doc. Its ID is the target of the title bar's docs link.
func docsSection(w *gui.Window, id, doc string) gui.View {
	style := gui.DefaultMarkdownStyle()
	style.CodeHighlighter = highlight.Default()
	return gui.Column(gui.ContainerCfg{
		ID:         docsSectionID,
		Sizing:     gui.FillFit,
		Padding:    gui.NoPadding,
		SizeBorder: gui.NoBorder,
		Spacing:    gui.SpacingMedium,
		Content: []gui.View{
			line(),
			gui.Text(gui.TextCfg{Text: "Documentation", TextStyle: gui.CurrentTheme().TextStyleTitle}),
			w.Markdown(gui.MarkdownCfg{
				ID:      "doc-" + id,
				Source:  doc,
				Padding: gui.NoPadding,
				Style:   style,
			}),
		},
	})
}

func detailPanelPadding() gui.Padding {
	base := gui.CurrentTheme().PaddingLarge
	base.Right += gui.CurrentTheme().ScrollbarStyle.Size + 4
	return base
}

func viewTitleBar(entry DemoEntry, hasDocs bool) gui.View {
	titleContent := []gui.View{
		gui.Text(gui.TextCfg{Text: entry.Label, TextStyle: gui.CurrentTheme().TextStyleDisplay}),
	}
	if hasDocs {
		titleContent = append(titleContent,
			gui.Row(gui.ContainerCfg{
				Sizing:     gui.FillFit,
				Padding:    gui.NoPadding,
				SizeBorder: gui.NoBorder,
			}),
			docsLink(),
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

// docsLink is a labeled button that scrolls to the docs section. The
// docs sit below the demo, so on a tall demo the reader cannot see
// them; the label says they exist without a scroll.
func docsLink() gui.View {
	return gui.Button(gui.ButtonCfg{
		ID:         docsLinkID,
		A11YCfg:    gui.A11YCfg{A11YLabel: "Scroll to documentation"},
		Color:      gui.ColorTransparent,
		SizeBorder: gui.NoBorder,
		Padding:    gui.NewPadding(gui.PadXSmall, gui.PadSmall, gui.PadXSmall, gui.PadSmall),
		Radius:     gui.RadiusSmall,
		Content: []gui.View{
			gui.Row(gui.ContainerCfg{
				Padding:    gui.NoPadding,
				SizeBorder: gui.NoBorder,
				Spacing:    gui.SpacingSmall,
				VAlign:     gui.VAlignMiddle,
				Content: []gui.View{
					gui.Text(gui.TextCfg{Text: gui.IconBook, TextStyle: gui.CurrentTheme().TextStyleIconSmall}),
					gui.Text(gui.TextCfg{Text: "Docs", TextStyle: gui.CurrentTheme().TextStyleBodySmall}),
				},
			}),
		},
		OnClick: func(ctx gui.EventCtx) {
			scrollToDocs(ctx)
			ctx.Consume()
		},
	})
}

// scrollToDocs eases the detail panel so the docs section's top meets
// the top of the panel's content. Positions come from the last
// arranged frame, reached through the clicked button's parents; the
// window has no exported "scroll this view into place" call.
func scrollToDocs(ctx gui.EventCtx) {
	if ctx.Layout == nil {
		return
	}
	root := ctx.Layout
	for root.Parent != nil {
		root = root.Parent
	}
	view, ok := root.FindByID(scrollDetail)
	if !ok {
		return
	}
	docs, ok := root.FindByID(gui.ScopeID(scrollDetail, docsSectionID))
	if !ok {
		return
	}
	viewTop := view.Shape.Y + view.Shape.Padding.Top
	current := ctx.Window.ScrollVerticalOffset(scrollDetail)
	ctx.Window.ScrollVerticalToSmooth(scrollDetail,
		docsScrollOffset(viewTop, docs.Shape.Y, current))
}

// docsScrollOffset is the scroll offset (<= 0, as ScrollVerticalTo
// takes it) that puts a view now drawn at docsY at viewTop. The view's
// unscrolled position is docsY - current; scroll by that, less viewTop.
func docsScrollOffset(viewTop, docsY, current float32) float32 {
	return viewTop - docsY + current
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
	"segmented_control":   demoSegmentedControl,
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
	"virtual_list":        demoVirtualList,
	"drag_reorder":        demoDragReorder,
	"printing":            demoPrinting,
	"text":                demoText,
	"typography":          demoTypography,
	"text_anim":           demoTextAnim,
	"rtf":                 demoRtf,
	"table":               demoTable,
	"data_grid":           demoDataGrid,
	"data_source":         demoDataSource,
	"date_picker":         demoDatePicker,
	"input_date":          demoInputDate,
	"numeric_input":       demoNumericInput,
	"input_group":         demoInputGroup,
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
	"window_opacity":      demoWindowOpacity,
	"spinner":             demoMathSpinner,
	"svg_spinner":         demoSvgSpinner,
	"thinking_orb":        demoThinkingOrb,
}

func componentDemo(w *gui.Window, id string) gui.View {
	if id == "commands" {
		return showcaseMarkdownPanel(w, "showcase-commands", docPageSource("commands"))
	}
	if id == "dx_cheat_sheet" {
		return showcaseMarkdownPanel(w, "showcase-dx-cheat-sheet", docPageSource("dx_cheat_sheet"))
	}
	if id == "sound" {
		// The controls go above the guide: a doc-only page cannot turn
		// the feature on, and the Audio page is the wrong place to look
		// for it.
		return gui.Column(gui.ContainerCfg{
			ID:         "sound-page",
			Sizing:     gui.FillFill,
			Spacing:    gui.SpacingMedium,
			Padding:    gui.NoPadding,
			SizeBorder: gui.NoBorder,
			Content: []gui.View{
				widgetSoundControls(w),
				showcaseMarkdownPanel(w, "showcase-sound",
					docPageSource("sound")),
			},
		})
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
		Padding: gui.PaddingLarge,
		Radius:  gui.RadiusMedium,
		Content: []gui.View{
			gui.Text(gui.TextCfg{Text: text, TextStyle: t.TextStyleBody, Mode: gui.TextModeWrap}),
		},
	})
}

func relatedExamplesFooter(id string) gui.View {
	return gui.Text(gui.TextCfg{
		Text:      "Related examples: " + relatedExamples(id),
		TextStyle: gui.CurrentTheme().TextStyleCaption,
		Mode:      gui.TextModeWrap,
	})
}
