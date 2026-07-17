package main

import (
	"fmt"
	"strconv"

	"github.com/go-gui-org/go-gui/gui"
)

// Column indices. app.Sort.Column indexes colDefs.
const (
	colPID = iota
	colCPU
	colRSS
	colMem
	colUser
	colState
	colThreads
	colName
)

const rowHeight float32 = 26

// Data-visualization accent colors. The theme supplies no chart/status colors,
// so these are fixed (and chosen to read on both light and dark panels). All
// other chrome comes from theme tokens.
var (
	accentCPU  = gui.RGB(70, 190, 120) // CPU bars: green
	accentRAM  = gui.RGB(230, 170, 70) // RAM bars: amber
	accentMem  = gui.RGB(90, 150, 230) // system memory bar: blue
	colorAlert = gui.RGB(230, 90, 90)  // stopped / error text: red
)

// column describes one table column: header label, fixed width (0 = flexible),
// default sort direction, comparator, and cell renderer.
type column struct {
	less  func(a, b *Process) bool
	cell  func(p *Process, app *App) gui.View
	label string
	width float32
	desc  bool
}

// colDefs is the process table's column set, in display order. Comparators use
// a PID tiebreak so equal-keyed rows keep a stable order across refreshes
// (store iteration order is random).
var colDefs = []column{
	{
		label: "PID", width: 64,
		less: func(a, b *Process) bool { return a.PID < b.PID },
		cell: func(p *Process, _ *App) gui.View {
			return textCell(64, strconv.Itoa(p.PID), gui.CurrentTheme().N5)
		},
	},
	{
		label: "CPU", width: 118, desc: true,
		less: func(a, b *Process) bool {
			if a.CPUPercent != b.CPUPercent {
				return a.CPUPercent < b.CPUPercent
			}
			return a.PID < b.PID
		},
		cell: cpuCell,
	},
	{
		label: "RSS", width: 80, desc: true,
		less: lessRSS,
		cell: func(p *Process, _ *App) gui.View {
			return textCell(80, p.RSSText(), gui.CurrentTheme().N5)
		},
	},
	{
		label: "MEM%", width: 74, desc: true,
		less: lessRSS,
		cell: func(p *Process, _ *App) gui.View {
			return textCell(74, p.MemText(), gui.CurrentTheme().N5)
		},
	},
	{
		label: "USER", width: 110,
		less: func(a, b *Process) bool {
			if a.User != b.User {
				return a.User < b.User
			}
			return a.PID < b.PID
		},
		cell: func(p *Process, _ *App) gui.View {
			return textCell(110, p.User, gui.CurrentTheme().N5)
		},
	},
	{
		label: "STATE", width: 56,
		less: func(a, b *Process) bool {
			if a.State != b.State {
				return a.State < b.State
			}
			return a.PID < b.PID
		},
		cell: func(p *Process, _ *App) gui.View {
			return textCell(56, p.State, gui.CurrentTheme().N5)
		},
	},
	{
		label: "THR", width: 54, desc: true,
		less: func(a, b *Process) bool {
			if a.Threads != b.Threads {
				return a.Threads < b.Threads
			}
			return a.PID < b.PID
		},
		cell: func(p *Process, _ *App) gui.View {
			return textCell(54, p.ThreadsText(), gui.CurrentTheme().N5)
		},
	},
	{
		label: "NAME", width: 0, // flexible
		less: func(a, b *Process) bool {
			if a.Name != b.Name {
				return a.Name < b.Name
			}
			return a.PID < b.PID
		},
		cell: nameCell,
	},
}

func lessRSS(a, b *Process) bool {
	if a.RSSBytes != b.RSSBytes {
		return a.RSSBytes < b.RSSBytes
	}
	return a.PID < b.PID
}

// rootView is registered once in OnInit; the sampler goroutine re-runs it each
// refresh via Window.UpdateWindow.
func rootView(w *gui.Window) gui.View {
	app := gui.State[App](w)
	ww, wh := w.WindowSize()
	theme := gui.CurrentTheme()

	return gui.Column(gui.ContainerCfg{
		Width:   float32(ww),
		Height:  float32(wh),
		Sizing:  gui.FixedFixed,
		Color:   theme.ColorBackground,
		Padding: gui.NoPadding,
		Content: []gui.View{
			headerView(app),
			toolbarView(app),
			processTable(app),
			detailView(app),
		},
	})
}

func headerView(app *App) gui.View {
	theme := gui.CurrentTheme()
	content := []gui.View{
		gui.Row(gui.ContainerCfg{
			Sizing:  gui.FillFit,
			Padding: gui.NoPadding,
			VAlign:  gui.VAlignMiddle,
			Spacing: gui.SomeF(12),
			Content: []gui.View{
				gui.Text(gui.TextCfg{Text: "Process Monitor", TextStyle: theme.B2}),
			},
		}),
	}

	switch {
	case app.Err != nil:
		content = append(content, gui.Text(gui.TextCfg{
			Text:      "sample error: " + app.Err.Error(),
			TextStyle: styleColor(theme.N5, colorAlert),
		}))
	case app.Snapshot == nil:
		content = append(content, gui.Text(gui.TextCfg{
			Text: "Collecting process samples…", TextStyle: theme.N5,
		}))
	default:
		content = append(content, statsRow(app))
	}

	return gui.Column(gui.ContainerCfg{
		Sizing:  gui.FillFit,
		Color:   theme.ColorPanel,
		Padding: gui.SomeP(12, 16, 12, 16),
		Spacing: gui.SomeF(8),
		Content: content,
	})
}

func statsRow(app *App) gui.View {
	theme := gui.CurrentTheme()
	content := []gui.View{
		statPill("Processes", fmt.Sprintf("%s active / %s kept",
			formatCount(app.Store.ActiveCount()), formatCount(len(app.Store.ByKey)))),
		statPill("Updated", formatTime(app.LastRefresh)),
	}

	// Only show the memory bar when the platform gave us real totals.
	if app.Snapshot.TotalMemoryBytes > 0 {
		used, total := app.Snapshot.UsedMemoryBytes, app.Snapshot.TotalMemoryBytes
		ratio := float32(float64(used) / float64(total))
		content = append(content, gui.Row(gui.ContainerCfg{
			Sizing:  gui.FitFit,
			Padding: gui.NoPadding,
			VAlign:  gui.VAlignMiddle,
			Spacing: gui.SomeF(8),
			Content: []gui.View{
				gui.Text(gui.TextCfg{Text: "Memory", TextStyle: theme.N6}),
				usageBar(ratio, 160, 10, accentMem),
				gui.Text(gui.TextCfg{
					Text:      fmt.Sprintf("%s / %s", formatBytes(used), formatBytes(total)),
					TextStyle: theme.N6,
				}),
			},
		}))
	}

	return gui.Row(gui.ContainerCfg{
		Sizing:  gui.FillFit,
		Padding: gui.NoPadding,
		VAlign:  gui.VAlignMiddle,
		Spacing: gui.SomeF(18),
		Content: content,
	})
}

func statPill(label, value string) gui.View {
	theme := gui.CurrentTheme()
	return gui.Row(gui.ContainerCfg{
		Sizing:  gui.FitFit,
		Color:   theme.ColorInterior,
		Radius:  gui.SomeF(theme.RadiusSmall),
		Padding: gui.SomeP(5, 9, 5, 9),
		Spacing: gui.SomeF(6),
		VAlign:  gui.VAlignMiddle,
		Content: []gui.View{
			gui.Text(gui.TextCfg{Text: label, TextStyle: theme.N6}),
			gui.Text(gui.TextCfg{Text: value, TextStyle: theme.B5}),
		},
	})
}

func toolbarView(app *App) gui.View {
	theme := gui.CurrentTheme()
	return gui.Row(gui.ContainerCfg{
		Sizing:  gui.FillFit,
		Color:   theme.ColorPanel,
		Padding: gui.SomeP(8, 16, 8, 16),
		Spacing: gui.SomeF(12),
		VAlign:  gui.VAlignMiddle,
		Content: []gui.View{
			gui.Text(gui.TextCfg{Text: "Filter", TextStyle: theme.B5}),
			gui.Input(gui.InputCfg{
				ID:          "pm-filter",
				Sizing:      gui.FixedFit,
				Width:       300,
				Text:        app.Filter,
				Placeholder: "name, command, user, or PID",
				OnTextChanged: func(_ *gui.Layout, text string, w *gui.Window) {
					gui.State[App](w).Filter = text
				},
			}),
			spacer(),
			gui.Text(gui.TextCfg{Text: "View", TextStyle: theme.B5}),
			viewModeRadio(app),
			gui.Text(gui.TextCfg{Text: "Every", TextStyle: theme.B5}),
			intervalRadio(app),
		},
	})
}

func viewModeRadio(app *App) gui.View {
	value := "Flat"
	if app.TreeMode {
		value = "Tree"
	}
	return gui.RadioButtonGroupRow(gui.RadioButtonGroupCfg{
		ID:    "pm-view",
		Items: []string{"Flat", "Tree"},
		Value: value,
		OnSelect: func(v string, w *gui.Window) {
			gui.State[App](w).TreeMode = v == "Tree"
		},
	})
}

func intervalRadio(app *App) gui.View {
	return gui.RadioButtonGroupRow(gui.RadioButtonGroupCfg{
		ID:    "pm-interval",
		Items: intervalLabels,
		Value: intervalLabel(app.Interval),
		OnSelect: func(v string, w *gui.Window) {
			gui.State[App](w).Interval = intervalFromLabel(v)
		},
	})
}

func processTable(app *App) gui.View {
	theme := gui.CurrentTheme()
	if app.Snapshot == nil {
		return centered("Waiting for first sample…")
	}

	// Rows are filtered and ordered here (not by a table widget): tree mode
	// orders siblings within the hierarchy and the filter keeps ancestors.
	less := colDefs[app.Sort.Column].less
	rows := visibleRows(app.Store.Processes(), app.Filter, less, app.Sort.Desc, app.TreeMode)

	var body gui.View
	if len(rows) == 0 {
		body = centered("No matching processes")
	} else {
		rowViews := make([]gui.View, 0, len(rows))
		for i, p := range rows {
			rowViews = append(rowViews, processRow(p, app, i))
		}
		body = gui.Column(gui.ContainerCfg{
			ID:            "pm-scroll",
			Scrollable:    true,
			ScrollbarCfgY: &gui.ScrollbarCfg{Overflow: gui.ScrollbarAuto},
			Sizing:        gui.FillFill,
			Padding:       gui.NoPadding,
			Content:       rowViews,
		})
	}

	return gui.Column(gui.ContainerCfg{
		Sizing:  gui.FillFill,
		Color:   theme.ColorBackground,
		Padding: gui.NoPadding,
		Content: []gui.View{headerRow(app), body},
	})
}

func headerRow(app *App) gui.View {
	theme := gui.CurrentTheme()
	cells := make([]gui.View, 0, len(colDefs))
	for i, col := range colDefs {
		cells = append(cells, headerCell(col, i, app))
	}
	return gui.Row(gui.ContainerCfg{
		Sizing:  gui.FillFixed,
		Height:  rowHeight,
		Color:   theme.ColorInterior,
		Padding: gui.NoPadding,
		VAlign:  gui.VAlignMiddle,
		Content: cells,
	})
}

func headerCell(col column, idx int, app *App) gui.View {
	theme := gui.CurrentTheme()
	label := col.label
	if app.Sort.Column == idx {
		if app.Sort.Desc {
			label += " ▼"
		} else {
			label += " ▲"
		}
	}

	cfg := gui.ContainerCfg{
		VAlign:  gui.VAlignMiddle,
		Clip:    true,
		Padding: gui.SomeP(0, 6, 0, 6),
		Content: []gui.View{gui.Text(gui.TextCfg{Text: label, TextStyle: theme.B6, Clip: true})},
		OnClick: func(_ *gui.Layout, e *gui.Event, w *gui.Window) {
			a := gui.State[App](w)
			if a.Sort.Column == idx {
				a.Sort.Desc = !a.Sort.Desc
			} else {
				a.Sort.Column = idx
				a.Sort.Desc = col.desc
			}
			e.IsHandled = true
		},
	}
	if col.width == 0 {
		cfg.Sizing = gui.FillFill
	} else {
		cfg.Width = col.width
		cfg.Sizing = gui.FixedFill
	}
	return gui.Row(cfg)
}

func processRow(p *Process, app *App, i int) gui.View {
	theme := gui.CurrentTheme()
	bg := theme.ColorPanel
	if i%2 == 1 {
		bg = theme.ColorBackground
	}
	if app.Selected == p {
		bg = theme.ColorSelect
	}

	cells := make([]gui.View, 0, len(colDefs))
	for _, col := range colDefs {
		cells = append(cells, col.cell(p, app))
	}

	return gui.Row(gui.ContainerCfg{
		Sizing:  gui.FillFixed,
		Height:  rowHeight,
		Color:   bg,
		Padding: gui.NoPadding,
		VAlign:  gui.VAlignMiddle,
		Content: cells,
		OnClick: func(_ *gui.Layout, e *gui.Event, w *gui.Window) {
			gui.State[App](w).Selected = p
			e.IsHandled = true
		},
	})
}

func detailView(app *App) gui.View {
	theme := gui.CurrentTheme()
	if app.Snapshot == nil || app.Selected == nil {
		return gui.Column(gui.ContainerCfg{
			Sizing:  gui.FillFit,
			Color:   theme.ColorPanel,
			Padding: gui.SomeP(10, 16, 12, 16),
			Content: []gui.View{gui.Text(gui.TextCfg{Text: "Select a process", TextStyle: theme.N5})},
		})
	}

	p := app.Selected
	facts := []gui.View{
		gui.Text(gui.TextCfg{
			Text: fmt.Sprintf("%s  pid %d", p.Name, p.PID), TextStyle: theme.B4,
		}),
		gui.Text(gui.TextCfg{Text: fmt.Sprintf("ppid %d", p.PPID), TextStyle: theme.N6}),
		gui.Text(gui.TextCfg{Text: "cpu " + formatCPUPercent(p.CPUPercent), TextStyle: theme.N6}),
		gui.Text(gui.TextCfg{Text: "rss " + p.RSSText(), TextStyle: theme.N6}),
	}
	if !p.Running() {
		facts = append(facts, gui.Text(gui.TextCfg{
			Text: "stopped " + formatTime(p.StoppedAt), TextStyle: styleColor(theme.N6, colorAlert),
		}))
	}

	cmd := p.Cmdline
	if cmd == "" {
		cmd = p.Name
	}

	return gui.Column(gui.ContainerCfg{
		Sizing:  gui.FillFit,
		Color:   theme.ColorPanel,
		Padding: gui.SomeP(10, 16, 12, 16),
		Spacing: gui.SomeF(6),
		Content: []gui.View{
			gui.Row(gui.ContainerCfg{
				Sizing:  gui.FillFit,
				Padding: gui.NoPadding,
				VAlign:  gui.VAlignMiddle,
				Spacing: gui.SomeF(12),
				Content: facts,
			}),
			gui.Text(gui.TextCfg{Text: truncate(cmd, 160), TextStyle: theme.N6, Clip: true}),
			historyCharts(p),
		},
	})
}

func historyCharts(p *Process) gui.View {
	return gui.Row(gui.ContainerCfg{
		Sizing:  gui.FitFit,
		Padding: gui.NoPadding,
		Spacing: gui.SomeF(12),
		Content: []gui.View{
			usageChart(p, "CPU", 100, accentCPU,
				func(pt ProcessPoint) float64 { return pt.CPUPercent },
				func(v float64) string { return fmt.Sprintf("%.0f%%", v) }),
			usageChart(p, "RAM", ramHistoryScale(p.History), accentRAM,
				func(pt ProcessPoint) float64 { return float64(pt.RSSBytes) },
				func(v float64) string { return formatBytes(uint64(v)) }),
		},
	})
}

// --- small view helpers ---

// textCell renders a fixed-width, vertically-centered, clipped text cell.
func textCell(width float32, s string, style gui.TextStyle) gui.View {
	return gui.Row(gui.ContainerCfg{
		Width:   width,
		Sizing:  gui.FixedFill,
		VAlign:  gui.VAlignMiddle,
		Clip:    true,
		Padding: gui.SomeP(0, 6, 0, 6),
		Content: []gui.View{gui.Text(gui.TextCfg{Text: s, TextStyle: style, Clip: true})},
	})
}

// cpuCell renders the CPU percentage next to a small usage bar.
func cpuCell(p *Process, _ *App) gui.View {
	theme := gui.CurrentTheme()
	ratio := float32(0)
	if p.CPUPercent > 0 {
		ratio = float32(p.CPUPercent / 100)
	}
	return gui.Row(gui.ContainerCfg{
		Width:   118,
		Sizing:  gui.FixedFill,
		VAlign:  gui.VAlignMiddle,
		Clip:    true,
		Padding: gui.SomeP(0, 6, 0, 6),
		Spacing: gui.SomeF(6),
		Content: []gui.View{
			gui.Text(gui.TextCfg{Text: p.CPUText(), TextStyle: theme.N5}),
			usageBar(ratio, 48, 8, accentCPU),
		},
	})
}

// nameCell renders the process name, prefixed in tree mode by an indent and a
// collapse chevron for processes that have children.
func nameCell(p *Process, app *App) gui.View {
	theme := gui.CurrentTheme()
	kids := make([]gui.View, 0, 3)

	if app.TreeMode {
		if depth := min(p.TreeDepth, 8); depth > 0 {
			kids = append(kids, gui.Rectangle(gui.RectangleCfg{
				Width: float32(depth) * 14, Sizing: gui.FixedFill, Color: gui.ColorTransparent,
			}))
		}
		if p.TreeChildCount > 0 {
			marker := "▾"
			if p.Collapsed {
				marker = "▸"
			}
			kids = append(kids, gui.Row(gui.ContainerCfg{
				Width:   16,
				Sizing:  gui.FixedFill,
				VAlign:  gui.VAlignMiddle,
				Content: []gui.View{gui.Text(gui.TextCfg{Text: marker, TextStyle: theme.N5})},
				OnClick: func(_ *gui.Layout, e *gui.Event, _ *gui.Window) {
					// p is a stable store pointer; toggling under event dispatch
					// (which holds the window lock) is safe.
					p.Collapsed = !p.Collapsed
					e.IsHandled = true
				},
			}))
		} else {
			kids = append(kids, gui.Rectangle(gui.RectangleCfg{
				Width: 16, Sizing: gui.FixedFill, Color: gui.ColorTransparent,
			}))
		}
	}

	style := theme.N5
	if !p.Running() {
		style = theme.N6 // dim stopped processes
	}
	kids = append(kids, gui.Text(gui.TextCfg{Text: p.Name, TextStyle: style, Clip: true}))

	return gui.Row(gui.ContainerCfg{
		Sizing:  gui.FillFill,
		VAlign:  gui.VAlignMiddle,
		Clip:    true,
		Padding: gui.SomeP(0, 6, 0, 6),
		Spacing: gui.SomeF(2),
		Content: kids,
	})
}

// usageBar renders a rounded background track with a proportional fill.
func usageBar(ratio, width, height float32, fill gui.Color) gui.View {
	if ratio < 0 {
		ratio = 0
	}
	if ratio > 1 {
		ratio = 1
	}
	theme := gui.CurrentTheme()
	return gui.Row(gui.ContainerCfg{
		Width:   width,
		Height:  height,
		Sizing:  gui.FixedFixed,
		Color:   theme.ColorBorder,
		Radius:  gui.SomeF(2),
		Padding: gui.NoPadding,
		Content: []gui.View{
			gui.Rectangle(gui.RectangleCfg{
				Width: width * ratio, Height: height, Sizing: gui.FixedFixed,
				Color: fill, Radius: 2,
			}),
		},
	})
}

func centered(msg string) gui.View {
	theme := gui.CurrentTheme()
	return gui.Column(gui.ContainerCfg{
		Sizing:  gui.FillFill,
		HAlign:  gui.HAlignCenter,
		VAlign:  gui.VAlignMiddle,
		Padding: gui.NoPadding,
		Content: []gui.View{gui.Text(gui.TextCfg{Text: msg, TextStyle: theme.N4})},
	})
}

func spacer() gui.View {
	return gui.Rectangle(gui.RectangleCfg{Sizing: gui.FillFit, Color: gui.ColorTransparent})
}

// styleColor returns a copy of style with its color overridden.
func styleColor(style gui.TextStyle, c gui.Color) gui.TextStyle {
	style.Color = c
	return style
}
