package main

func componentDoc(id string) string {
	key := id
	switch key {
	case "native_notification":
		key = "notification"
	case "column_demo":
		key = "column"
	}
	file, ok := widgetDocFiles[key]
	if !ok {
		return ""
	}
	data, err := showcaseFS.ReadFile(file)
	if err != nil {
		return ""
	}
	return string(data)
}

var widgetDocFiles = map[string]string{
	"animations":         "docs/widget_animations.md",
	"audio":              "docs/widget_audio.md",
	"badge":              "docs/widget_badge.md",
	"blur":               "docs/widget_blur.md",
	"box_shadows":        "docs/widget_box_shadows.md",
	"breadcrumb":         "docs/widget_breadcrumb.md",
	"button":             "docs/widget_button.md",
	"color_filter":       "docs/widget_color_filter.md",
	"color_picker":       "docs/widget_color_picker.md",
	"column":             "docs/widget_column.md",
	"combobox":           "docs/widget_combobox.md",
	"command_button":     "docs/widget_command_button.md",
	"command_palette":    "docs/widget_command_palette.md",
	"context_menu":       "docs/widget_context_menu.md",
	"data_grid":          "docs/widget_data_grid.md",
	"data_source":        "docs/widget_data_source.md",
	"date_picker":        "docs/widget_date_picker.md",
	"date_picker_roller": "docs/widget_date_picker_roller.md",
	"dialog":             "docs/widget_dialog.md",
	"dock_layout":        "docs/widget_dock_layout.md",
	"drag_reorder":       "docs/widget_drag_reorder.md",
	"draw_canvas":        "docs/widget_draw_canvas.md",
	"expand_panel":       "docs/widget_expand_panel.md",
	"forms":              "docs/widget_forms.md",
	"gesture":            "docs/widget_gesture.md",
	"gradient":           "docs/widget_gradient.md",
	"icons":              "docs/widget_icons.md",
	"image":              "docs/widget_image.md",
	"input":              "docs/widget_input.md",
	"input_date":         "docs/widget_input_date.md",
	"listbox":            "docs/widget_listbox.md",
	"locale":             "docs/widget_locale.md",
	"markdown":           "docs/widget_markdown.md",
	"menus":              "docs/widget_menus.md",
	"multi_window":       "docs/widget_multi_window.md",
	"notification":       "docs/widget_notification.md",
	"numeric_input":      "docs/widget_numeric_input.md",
	"overflow_panel":     "docs/widget_overflow_panel.md",
	"printing":           "docs/widget_printing.md",
	"progress_bar":       "docs/widget_progress_bar.md",
	"pulsar":             "docs/widget_pulsar.md",
	"radio":              "docs/widget_radio.md",
	"radio_group":        "docs/widget_radio_group.md",
	"rectangle":          "docs/widget_rectangle.md",
	"rotated_box":        "docs/widget_rotated_box.md",
	"row":                "docs/widget_row.md",
	"rtf":                "docs/widget_rtf.md",
	"scrollbar":          "docs/widget_scrollbar.md",
	"select":             "docs/widget_select.md",
	"shader":             "docs/widget_shader.md",
	"sidebar":            "docs/widget_sidebar.md",
	"skeleton":           "docs/widget_skeleton.md",
	"slider":             "docs/widget_slider.md",
	"spinner":            "docs/widget_spinner.md",
	"splitter":           "docs/widget_splitter.md",
	"svg":                "docs/widget_svg.md",
	"svg_spinner":        "docs/widget_svg_spinner.md",
	"switch":             "docs/widget_switch.md",
	"tab_control":        "docs/widget_tab_control.md",
	"table":              "docs/widget_table.md",
	"text":               "docs/widget_text.md",
	"theme_gen":          "docs/widget_theme_gen.md",
	"theme_picker":       "docs/widget_theme_picker.md",
	"toast":              "docs/widget_toast.md",
	"toggle":             "docs/widget_toggle.md",
	"tooltip":            "docs/widget_tooltip.md",
	"tree":               "docs/widget_tree.md",
	"wrap_panel":         "docs/widget_wrap_panel.md",
}
