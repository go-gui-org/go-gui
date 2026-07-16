// Date Picker Options example demonstrates DatePicker configuration
// with toggleable options, weekday/month/year filters, and theme switching.
package main

import (
	"time"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend"
)

type App struct {
	WeekdaysLen string

	Dates              []time.Time
	HideTodayIndicator bool
	MondayFirst        bool
	ShowAdjacentMonths bool
	SelectMultiple     bool

	AllowMonday    bool
	AllowTuesday   bool
	AllowWednesday bool
	AllowThursday  bool
	AllowFriday    bool
	AllowSaturday  bool
	AllowSunday    bool

	AllowJanuary   bool
	AllowFebruary  bool
	AllowMarch     bool
	AllowApril     bool
	AllowMay       bool
	AllowJune      bool
	AllowJuly      bool
	AllowAugust    bool
	AllowSeptember bool
	AllowOctober   bool
	AllowNovember  bool
	AllowDecember  bool

	AllowYearNow  bool
	AllowYearLast bool
	AllowYearNext bool

	AllowToday        bool
	AllowYesterday    bool
	AllowFirstOfMonth bool

	LightTheme bool
}

func main() {
	gui.SetTheme(gui.ThemeDark.WithBorders(true))

	w := gui.NewWindow(gui.WindowCfg{
		State:  &App{WeekdaysLen: "one"},
		Title:  "Date Picker Options Demo",
		Width:  1200,
		Height: 950,
		OnInit: func(w *gui.Window) {
			w.UpdateView(mainView)
		},
	})

	backend.Run(w)
}

func mainView(w *gui.Window) gui.View {
	ww, wh := w.WindowSize()
	app := gui.State[App](w)

	return gui.Column(gui.ContainerCfg{
		Width:   float32(ww),
		Height:  float32(wh),
		Sizing:  gui.FixedFixed,
		HAlign:  gui.HAlignCenter,
		VAlign:  gui.VAlignMiddle,
		Spacing: gui.Some(gui.SpacingLarge),
		Content: []gui.View{
			borderedGroup("Calendar", []gui.View{datePicker(app, w)}),
			gui.Row(gui.ContainerCfg{
				Padding: gui.NoPadding,
				Spacing: gui.Some(gui.SpacingLarge * 2),
				Content: []gui.View{
					optionsGroup(app),
					weekdaysGroup(app),
					monthsGroup(app),
					yearsDatesGroup(app, w),
				},
			}),
		},
	})
}

func datePicker(app *App, _ *gui.Window) gui.View {
	var weekdaysLen gui.DatePickerWeekdayLen
	switch app.WeekdaysLen {
	case "three":
		weekdaysLen = gui.WeekdayThreeLetter
	case "full":
		weekdaysLen = gui.WeekdayFull
	default:
		weekdaysLen = gui.WeekdayOneLetter
	}

	var allowedWeekdays []gui.DatePickerWeekdays
	if app.AllowMonday {
		allowedWeekdays = append(allowedWeekdays, gui.DatePickerMonday)
	}
	if app.AllowTuesday {
		allowedWeekdays = append(allowedWeekdays, gui.DatePickerTuesday)
	}
	if app.AllowWednesday {
		allowedWeekdays = append(allowedWeekdays, gui.DatePickerWednesday)
	}
	if app.AllowThursday {
		allowedWeekdays = append(allowedWeekdays, gui.DatePickerThursday)
	}
	if app.AllowFriday {
		allowedWeekdays = append(allowedWeekdays, gui.DatePickerFriday)
	}
	if app.AllowSaturday {
		allowedWeekdays = append(allowedWeekdays, gui.DatePickerSaturday)
	}
	if app.AllowSunday {
		allowedWeekdays = append(allowedWeekdays, gui.DatePickerSunday)
	}

	var allowedMonths []gui.DatePickerMonths
	if app.AllowJanuary {
		allowedMonths = append(allowedMonths, gui.DatePickerJanuary)
	}
	if app.AllowFebruary {
		allowedMonths = append(allowedMonths, gui.DatePickerFebruary)
	}
	if app.AllowMarch {
		allowedMonths = append(allowedMonths, gui.DatePickerMarch)
	}
	if app.AllowApril {
		allowedMonths = append(allowedMonths, gui.DatePickerApril)
	}
	if app.AllowMay {
		allowedMonths = append(allowedMonths, gui.DatePickerMay)
	}
	if app.AllowJune {
		allowedMonths = append(allowedMonths, gui.DatePickerJune)
	}
	if app.AllowJuly {
		allowedMonths = append(allowedMonths, gui.DatePickerJuly)
	}
	if app.AllowAugust {
		allowedMonths = append(allowedMonths, gui.DatePickerAugust)
	}
	if app.AllowSeptember {
		allowedMonths = append(allowedMonths, gui.DatePickerSeptember)
	}
	if app.AllowOctober {
		allowedMonths = append(allowedMonths, gui.DatePickerOctober)
	}
	if app.AllowNovember {
		allowedMonths = append(allowedMonths, gui.DatePickerNovember)
	}
	if app.AllowDecember {
		allowedMonths = append(allowedMonths, gui.DatePickerDecember)
	}

	today := time.Now()
	var allowedYears []int
	if app.AllowYearNow {
		allowedYears = append(allowedYears, today.Year())
	}
	if app.AllowYearLast {
		allowedYears = append(allowedYears, today.Year()-1)
	}
	if app.AllowYearNext {
		allowedYears = append(allowedYears, today.Year()+1)
	}

	var allowedDates []time.Time
	if app.AllowToday {
		allowedDates = append(allowedDates, today)
	}
	if app.AllowYesterday {
		allowedDates = append(allowedDates, today.AddDate(0, 0, -1))
	}
	if app.AllowFirstOfMonth {
		allowedDates = append(allowedDates,
			time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, time.Local))
	}

	return gui.DatePicker(gui.DatePickerCfg{
		ID:                   "example",
		Dates:                app.Dates,
		HideTodayIndicator:   app.HideTodayIndicator,
		MondayFirstDayOfWeek: app.MondayFirst,
		ShowAdjacentMonths:   app.ShowAdjacentMonths,
		SelectMultiple:       app.SelectMultiple,
		WeekdaysLen:          weekdaysLen,
		AllowedWeekdays:      allowedWeekdays,
		AllowedMonths:        allowedMonths,
		AllowedYears:         allowedYears,
		AllowedDates:         allowedDates,
		OnSelect: func(times []time.Time, e *gui.Event, w *gui.Window) {
			gui.State[App](w).Dates = times
			e.IsHandled = true
		},
	})
}

func optionsGroup(app *App) gui.View {
	return gui.Column(gui.ContainerCfg{
		Padding: gui.NoPadding,
		Content: []gui.View{
			borderedGroup("Options", []gui.View{
				gui.Toggle(gui.ToggleCfg{
					Label:    "Monday first day of week",
					Selected: app.MondayFirst,
					OnClick: func(_ *gui.Layout, _ *gui.Event, w *gui.Window) {
						gui.State[App](w).MondayFirst =
							!gui.State[App](w).MondayFirst
					},
				}),
				gui.Toggle(gui.ToggleCfg{
					Label:    "Show adjacent months",
					Selected: app.ShowAdjacentMonths,
					OnClick: func(_ *gui.Layout, _ *gui.Event, w *gui.Window) {
						gui.State[App](w).ShowAdjacentMonths =
							!gui.State[App](w).ShowAdjacentMonths
					},
				}),
				gui.Toggle(gui.ToggleCfg{
					Label:    "Hide today indicator",
					Selected: app.HideTodayIndicator,
					OnClick: func(_ *gui.Layout, _ *gui.Event, w *gui.Window) {
						gui.State[App](w).HideTodayIndicator =
							!gui.State[App](w).HideTodayIndicator
					},
				}),
				gui.Toggle(gui.ToggleCfg{
					Label:    "Multiple select",
					Selected: app.SelectMultiple,
					OnClick: func(_ *gui.Layout, _ *gui.Event, w *gui.Window) {
						gui.State[App](w).SelectMultiple =
							!gui.State[App](w).SelectMultiple
					},
				}),
			}),
		},
	})
}

func weekdaysGroup(app *App) gui.View {
	return gui.Column(gui.ContainerCfg{
		Padding: gui.NoPadding,
		Content: []gui.View{
			weekdaysLenGroup(app),
			gui.Rectangle(gui.RectangleCfg{Color: gui.ColorTransparent}),
			allowedWeekdaysGroup(app),
		},
	})
}

func weekdaysLenGroup(app *App) gui.View {
	return borderedGroup("Weekdays", []gui.View{
		gui.RadioButtonGroupColumn(gui.RadioButtonGroupCfg{
			Sizing:  gui.FillFit,
			Padding: gui.NoPadding,
			Value:   app.WeekdaysLen,
			Options: []gui.RadioOption{
				gui.NewRadioOption("One letter", "one"),
				gui.NewRadioOption("Three letter", "three"),
				gui.NewRadioOption("Full", "full"),
			},
			OnSelect: func(value string, w *gui.Window) {
				gui.State[App](w).WeekdaysLen = value
			},
		}),
	})
}

func allowedWeekdaysGroup(app *App) gui.View {
	type weekdayToggle struct {
		id       string
		label    string
		selected bool
	}
	days := []weekdayToggle{
		{"mon", "Monday", app.AllowMonday},
		{"tue", "Tuesday", app.AllowTuesday},
		{"wed", "Wednesday", app.AllowWednesday},
		{"thu", "Thursday", app.AllowThursday},
		{"fri", "Friday", app.AllowFriday},
		{"sat", "Saturday", app.AllowSaturday},
		{"sun", "Sunday", app.AllowSunday},
	}

	content := make([]gui.View, len(days))
	for i, d := range days {
		content[i] = gui.Toggle(gui.ToggleCfg{
			ID:       d.id,
			Label:    d.label,
			Selected: d.selected,
			OnClick:  clickAllowWeekday,
		})
	}

	return borderedGroup("Allowed weekdays", content)
}

func clickAllowWeekday(l *gui.Layout, e *gui.Event, w *gui.Window) {
	app := gui.State[App](w)
	switch l.Shape.ID {
	case "mon":
		app.AllowMonday = !app.AllowMonday
	case "tue":
		app.AllowTuesday = !app.AllowTuesday
	case "wed":
		app.AllowWednesday = !app.AllowWednesday
	case "thu":
		app.AllowThursday = !app.AllowThursday
	case "fri":
		app.AllowFriday = !app.AllowFriday
	case "sat":
		app.AllowSaturday = !app.AllowSaturday
	case "sun":
		app.AllowSunday = !app.AllowSunday
	}
	e.IsHandled = true
}

func monthsGroup(app *App) gui.View {
	type monthToggle struct {
		id       string
		label    string
		selected bool
	}
	months := []monthToggle{
		{"jan", "January", app.AllowJanuary},
		{"feb", "February", app.AllowFebruary},
		{"mar", "March", app.AllowMarch},
		{"apr", "April", app.AllowApril},
		{"may", "May", app.AllowMay},
		{"jun", "June", app.AllowJune},
		{"jul", "July", app.AllowJuly},
		{"aug", "August", app.AllowAugust},
		{"sep", "September", app.AllowSeptember},
		{"oct", "October", app.AllowOctober},
		{"nov", "November", app.AllowNovember},
		{"dec", "December", app.AllowDecember},
	}

	content := make([]gui.View, len(months))
	for i, m := range months {
		content[i] = gui.Toggle(gui.ToggleCfg{
			ID:       m.id,
			Label:    m.label,
			Selected: m.selected,
			OnClick:  clickAllowMonth,
		})
	}

	return borderedGroup("Allowed months", content)
}

func clickAllowMonth(l *gui.Layout, e *gui.Event, w *gui.Window) {
	app := gui.State[App](w)
	switch l.Shape.ID {
	case "jan":
		app.AllowJanuary = !app.AllowJanuary
	case "feb":
		app.AllowFebruary = !app.AllowFebruary
	case "mar":
		app.AllowMarch = !app.AllowMarch
	case "apr":
		app.AllowApril = !app.AllowApril
	case "may":
		app.AllowMay = !app.AllowMay
	case "jun":
		app.AllowJune = !app.AllowJune
	case "jul":
		app.AllowJuly = !app.AllowJuly
	case "aug":
		app.AllowAugust = !app.AllowAugust
	case "sep":
		app.AllowSeptember = !app.AllowSeptember
	case "oct":
		app.AllowOctober = !app.AllowOctober
	case "nov":
		app.AllowNovember = !app.AllowNovember
	case "dec":
		app.AllowDecember = !app.AllowDecember
	}
	e.IsHandled = true
}

func yearsDatesGroup(app *App, _ *gui.Window) gui.View {
	return gui.Column(gui.ContainerCfg{
		Padding: gui.NoPadding,
		Sizing:  gui.FitFill,
		Content: []gui.View{
			allowedYearsGroup(app),
			gui.Rectangle(gui.RectangleCfg{Color: gui.ColorTransparent}),
			allowedDatesGroup(app),
			gui.Rectangle(gui.RectangleCfg{Sizing: gui.FitFill}),
			gui.Row(gui.ContainerCfg{
				HAlign:  gui.HAlignRight,
				VAlign:  gui.VAlignMiddle,
				Padding: gui.NoPadding,
				Sizing:  gui.FillFit,
				Content: []gui.View{
					gui.Button(gui.ButtonCfg{
						Content: []gui.View{
							gui.Text(gui.TextCfg{Text: "Reset"}),
						},
						OnClick: func(_ *gui.Layout, e *gui.Event, w *gui.Window) {
							w.DatePickerReset("example")
							app := gui.State[App](w)
							app.Dates = []time.Time{time.Now()}
							app.WeekdaysLen = "one"
							app.MondayFirst = false
							app.ShowAdjacentMonths = false
							app.HideTodayIndicator = false
							app.SelectMultiple = false
							app.AllowMonday = false
							app.AllowTuesday = false
							app.AllowWednesday = false
							app.AllowThursday = false
							app.AllowFriday = false
							app.AllowSaturday = false
							app.AllowSunday = false
							app.AllowJanuary = false
							app.AllowFebruary = false
							app.AllowMarch = false
							app.AllowApril = false
							app.AllowMay = false
							app.AllowJune = false
							app.AllowJuly = false
							app.AllowAugust = false
							app.AllowSeptember = false
							app.AllowOctober = false
							app.AllowNovember = false
							app.AllowDecember = false
							app.AllowYearNow = false
							app.AllowYearLast = false
							app.AllowYearNext = false
							app.AllowToday = false
							app.AllowYesterday = false
							app.AllowFirstOfMonth = false
							e.IsHandled = true
						},
					}),
					toggleTheme(app),
				},
			}),
		},
	})
}

func allowedYearsGroup(app *App) gui.View {
	return borderedGroup("Allowed years", []gui.View{
		gui.Text(gui.TextCfg{Text: "Examples"}),
		gui.Toggle(gui.ToggleCfg{
			ID:       "year_now",
			Label:    "This year",
			Selected: app.AllowYearNow,
			OnClick:  clickAllowYear,
		}),
		gui.Toggle(gui.ToggleCfg{
			ID:       "year_last",
			Label:    "Last year",
			Selected: app.AllowYearLast,
			OnClick:  clickAllowYear,
		}),
		gui.Toggle(gui.ToggleCfg{
			ID:       "year_next",
			Label:    "Next year",
			Selected: app.AllowYearNext,
			OnClick:  clickAllowYear,
		}),
	})
}

func clickAllowYear(l *gui.Layout, e *gui.Event, w *gui.Window) {
	app := gui.State[App](w)
	switch l.Shape.ID {
	case "year_now":
		app.AllowYearNow = !app.AllowYearNow
	case "year_last":
		app.AllowYearLast = !app.AllowYearLast
	case "year_next":
		app.AllowYearNext = !app.AllowYearNext
	}
	e.IsHandled = true
}

func allowedDatesGroup(app *App) gui.View {
	return borderedGroup("Allowed dates", []gui.View{
		gui.Text(gui.TextCfg{Text: "Examples"}),
		gui.Toggle(gui.ToggleCfg{
			ID:       "tdy",
			Label:    "Today",
			Selected: app.AllowToday,
			OnClick:  clickAllowDate,
		}),
		gui.Toggle(gui.ToggleCfg{
			ID:       "ydy",
			Label:    "Yesterday",
			Selected: app.AllowYesterday,
			OnClick:  clickAllowDate,
		}),
		gui.Toggle(gui.ToggleCfg{
			ID:       "fdy",
			Label:    "First of month",
			Selected: app.AllowFirstOfMonth,
			OnClick:  clickAllowDate,
		}),
	})
}

func clickAllowDate(l *gui.Layout, e *gui.Event, w *gui.Window) {
	app := gui.State[App](w)
	switch l.Shape.ID {
	case "tdy":
		app.AllowToday = !app.AllowToday
	case "ydy":
		app.AllowYesterday = !app.AllowYesterday
	case "fdy":
		app.AllowFirstOfMonth = !app.AllowFirstOfMonth
	}
	e.IsHandled = true
}

func toggleTheme(app *App) gui.View {
	return gui.Toggle(gui.ToggleCfg{
		TextSelect:   gui.IconMoon,
		TextUnselect: gui.IconSunnyO,
		TextStyle:    gui.CurrentTheme().Icon3,
		Padding:      gui.Some(gui.PaddingSmall),
		Selected:     app.LightTheme,
		OnClick: func(_ *gui.Layout, _ *gui.Event, w *gui.Window) {
			app := gui.State[App](w)
			app.LightTheme = !app.LightTheme
			if app.LightTheme {
				w.SetTheme(gui.ThemeLight.WithBorders(true))
			} else {
				w.SetTheme(gui.ThemeDark.WithBorders(true))
			}
		},
	})
}

func borderedGroup(title string, content []gui.View) gui.View {
	theme := gui.CurrentTheme()
	return gui.Column(gui.ContainerCfg{
		Title:       title,
		TitleBG:     theme.ColorBackground,
		ColorBorder: theme.ColorBorder,
		SizeBorder:  gui.Some[float32](1),
		MinWidth:    200,
		Padding:     gui.Some(theme.Cfg.PaddingLarge),
		Content:     content,
	})
}
