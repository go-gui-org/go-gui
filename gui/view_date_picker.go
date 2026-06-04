package gui

import (
	"slices"
	"strconv"
	"time"
)

// DatePickerWeekdays identifies days of the week (1=Monday..7=Sunday).
type DatePickerWeekdays uint8

// DatePickerWeekdays values.
const (
	DatePickerMonday    DatePickerWeekdays = 1
	DatePickerTuesday   DatePickerWeekdays = 2
	DatePickerWednesday DatePickerWeekdays = 3
	DatePickerThursday  DatePickerWeekdays = 4
	DatePickerFriday    DatePickerWeekdays = 5
	DatePickerSaturday  DatePickerWeekdays = 6
	DatePickerSunday    DatePickerWeekdays = 7
)

// DatePickerMonths identifies months (1=January..12=December).
type DatePickerMonths uint16

// DatePickerMonths values.
const (
	DatePickerJanuary   DatePickerMonths = 1
	DatePickerFebruary  DatePickerMonths = 2
	DatePickerMarch     DatePickerMonths = 3
	DatePickerApril     DatePickerMonths = 4
	DatePickerMay       DatePickerMonths = 5
	DatePickerJune      DatePickerMonths = 6
	DatePickerJuly      DatePickerMonths = 7
	DatePickerAugust    DatePickerMonths = 8
	DatePickerSeptember DatePickerMonths = 9
	DatePickerOctober   DatePickerMonths = 10
	DatePickerNovember  DatePickerMonths = 11
	DatePickerDecember  DatePickerMonths = 12
)

// DatePickerWeekdayLen controls weekday header label length.
type DatePickerWeekdayLen uint8

// DatePickerWeekdayLen values.
const (
	WeekdayOneLetter   DatePickerWeekdayLen = iota // "S"
	WeekdayThreeLetter                             // "Sun"
	WeekdayFull                                    // "Sunday"
)

// datePickerState holds per-instance state for the date picker.
type datePickerState struct {
	ViewMonth           int
	ViewYear            int
	FocusDay            int
	CalBodyHeight       float32
	ShowYearMonthPicker bool
}

// DatePickerCfg configures a date picker calendar view.
type DatePickerCfg struct {
	TextStyle            TextStyle
	OnSelect             func([]time.Time, *Event, *Window)
	ID                   string `gui:"required"`
	A11YLabel            string
	A11YDescription      string
	Dates                []time.Time
	AllowedWeekdays      []DatePickerWeekdays
	AllowedMonths        []DatePickerMonths
	AllowedYears         []int
	AllowedDates         []time.Time
	Padding              Opt[Padding]
	SizeBorder           Opt[float32]
	CellSpacing          Opt[float32]
	Radius               Opt[float32]
	RadiusBorder         Opt[float32]
	IDFocus              uint32
	Color                Color
	ColorHover           Color
	ColorFocus           Color
	ColorClick           Color
	ColorBorder          Color
	ColorBorderFocus     Color
	ColorSelect          Color
	WeekdaysLen          DatePickerWeekdayLen
	Disabled             bool
	Invisible            bool
	SelectMultiple       bool
	HideTodayIndicator   bool
	MondayFirstDayOfWeek bool
	ShowAdjacentMonths   bool
}

type datePickerView struct {
	cfg DatePickerCfg
}

// DatePicker creates a calendar date picker view.
func DatePicker(cfg DatePickerCfg) View {
	requireID("DatePicker", cfg.ID)
	applyDatePickerDefaults(&cfg)
	return &datePickerView{cfg: cfg}
}

func (dv *datePickerView) Content() []View { return nil }

func (dv *datePickerView) GenerateLayout(w *Window) Layout {
	cfg := &dv.cfg
	dn := &DefaultDatePickerStyle
	cellSpacing := cfg.CellSpacing.Get(dn.CellSpacing)
	radiusBorder := cfg.RadiusBorder.Get(dn.RadiusBorder)

	// Get/init state.
	state := datePickerGetState(w, cfg)

	// Build view tree: controls + body.
	content := make([]View, 0, 2)
	content = append(content, datePickerControls(cfg, state, w))
	if state.ShowYearMonthPicker {
		// Wrap roller with calendar body height to prevent height
		// change when switching views.
		body := datePickerYearMonthPicker(cfg, state)
		if state.CalBodyHeight > 0 {
			body = Column(ContainerCfg{
				Sizing:     FillFit,
				MinHeight:  state.CalBodyHeight,
				HAlign:     HAlignCenter,
				VAlign:     VAlignMiddle,
				Padding:    NoPadding,
				SizeBorder: NoBorder,
				Content:    []View{body},
			})
		}
		content = append(content, body)
	} else {
		content = append(content, datePickerCalendar(cfg, state, w))
	}

	// Stable size: 7 columns wide, 6 day rows + gaps tall.
	// Include padding + border so min covers full outer box.
	cellSize := datePickerCellSize(cfg)
	pad := cfg.Padding.Get(dn.Padding)
	sizeBorder := cfg.SizeBorder.Get(dn.SizeBorder)
	padW := float32(pad.Left+pad.Right) + 2*sizeBorder
	padH := float32(pad.Top+pad.Bottom) + 2*sizeBorder
	minWidth := 7*cellSize + 6*cellSpacing + padW
	minHeight := 6*cellSize + 6*cellSpacing + padH

	cfgID := cfg.ID
	col := Column(ContainerCfg{
		ID:          cfg.ID,
		IDFocus:     cfg.IDFocus,
		A11YRole:    AccessRoleGrid,
		A11YLabel:   a11yLabel(cfg.A11YLabel, "Date Picker"),
		Color:       cfg.Color,
		ColorBorder: cfg.ColorBorder,
		SizeBorder:  cfg.SizeBorder,
		Radius:      Some(radiusBorder),
		Padding:     cfg.Padding,
		Spacing:     Some(cellSpacing),
		MinWidth:    minWidth,
		MinHeight:   minHeight,
		Disabled:    cfg.Disabled,
		Invisible:   cfg.Invisible,
		Content:     content,
		AmendLayout: func(lo *Layout, w *Window) {
			if w.IsFocus(cfg.IDFocus) {
				lo.Shape.ColorBorder = cfg.ColorBorderFocus
			}
		},
		OnClick: func(_ *Layout, e *Event, w *Window) {
			if cfg.IDFocus > 0 && !cfg.Disabled {
				w.SetIDFocus(cfg.IDFocus)
				e.IsHandled = true
			}
		},
		OnKeyDown: func(_ *Layout, e *Event, w *Window) {
			sm := StateMap[string, datePickerState](
				w, nsDatePicker, capModerate)
			s, _ := sm.Get(cfgID)
			if s.ShowYearMonthPicker {
				datePickerRollerKeyDown(
					sm, cfgID, s, e, w)
			} else {
				datePickerOnKeyDown(cfg, e, w)
			}
		},
	})
	return GenerateViewLayout(col, w)
}

// datePickerGetState retrieves or initializes per-instance state.
func datePickerGetState(w *Window, cfg *DatePickerCfg) datePickerState {
	sm := StateMap[string, datePickerState](w, nsDatePicker, capModerate)
	s, ok := sm.Get(cfg.ID)
	if !ok {
		now := time.Now()
		if len(cfg.Dates) > 0 {
			now = cfg.Dates[0]
		}
		s = datePickerState{
			ViewMonth: int(now.Month()),
			ViewYear:  now.Year(),
			FocusDay:  now.Day(),
		}
		sm.Set(cfg.ID, s)
	}
	return s
}

// DatePickerReset clears the state for a date picker instance.
func (w *Window) DatePickerReset(id string) {
	sm := StateMap[string, datePickerState](w, nsDatePicker, capModerate)
	sm.Delete(id)
	w.UpdateWindow()
}

// datePickerControls builds the header row: month/year + prev/next.
func datePickerControls(
	cfg *DatePickerCfg, state datePickerState, _ *Window,
) View {
	cfgID := cfg.ID
	monthLabel := LocaleFormatDate(
		datePickerViewTime(state),
		guiLocale.Date.MonthYear,
	)

	idFocus := cfg.IDFocus
	onToggle := func(_ *Layout, e *Event, w *Window) {
		sm := StateMap[string, datePickerState](w, nsDatePicker, capModerate)
		s, _ := sm.Get(cfgID)
		s.ShowYearMonthPicker = !s.ShowYearMonthPicker
		sm.Set(cfgID, s)
		if idFocus > 0 {
			w.SetIDFocus(idFocus)
		}
		w.UpdateWindow()
		e.IsHandled = true
	}

	onPrev := func(_ *Layout, e *Event, w *Window) {
		if idFocus > 0 {
			w.SetIDFocus(idFocus)
		}
		datePickerNavMonth(cfgID, -1, w)
		e.IsHandled = true
	}

	onNext := func(_ *Layout, e *Event, w *Window) {
		if idFocus > 0 {
			w.SetIDFocus(idFocus)
		}
		datePickerNavMonth(cfgID, 1, w)
		e.IsHandled = true
	}

	return Row(ContainerCfg{
		VAlign:     VAlignMiddle,
		Padding:    NoPadding,
		SizeBorder: NoBorder,
		Sizing:     FillFit,
		Content: []View{
			Button(ButtonCfg{
				Color:       ColorTransparent,
				ColorBorder: ColorTransparent,
				OnClick:     onToggle,
				Content: []View{Text(TextCfg{
					Text: monthLabel, TextStyle: cfg.TextStyle,
				})},
			}),
			Rectangle(RectangleCfg{Sizing: FillFit}),
			Button(ButtonCfg{
				Disabled:    state.ShowYearMonthPicker,
				Color:       ColorTransparent,
				ColorBorder: ColorTransparent,
				OnClick:     onPrev,
				Content: []View{Text(TextCfg{
					Text:      IconArrowLeft,
					TextStyle: CurrentTheme().Icon3,
				})},
			}),
			Button(ButtonCfg{
				Disabled:    state.ShowYearMonthPicker,
				Color:       ColorTransparent,
				ColorBorder: ColorTransparent,
				OnClick:     onNext,
				Content: []View{Text(TextCfg{
					Text:      IconArrowRight,
					TextStyle: CurrentTheme().Icon3,
				})},
			}),
		},
	})
}

// datePickerCalendar builds the weekday headers and the day grid.
func datePickerCalendar(
	cfg *DatePickerCfg, state datePickerState, w *Window,
) View {
	dn := &DefaultDatePickerStyle
	cellSpacing := cfg.CellSpacing.Get(dn.CellSpacing)
	content := make([]View, 0, 7)
	content = append(content, datePickerWeekdays(cfg))
	content = append(content, datePickerMonth(cfg, state, w)...)
	cfgID := cfg.ID
	return Column(ContainerCfg{
		Spacing:    Some(cellSpacing),
		Padding:    NoPadding,
		SizeBorder: NoBorder,
		Content:    content,
		AmendLayout: func(lo *Layout, w *Window) {
			sm := StateMap[string, datePickerState](
				w, nsDatePicker, capModerate)
			s, _ := sm.Get(cfgID)
			if s.CalBodyHeight != lo.Shape.Height {
				s.CalBodyHeight = lo.Shape.Height
				sm.Set(cfgID, s)
			}
		},
	})
}

// datePickerWeekdays builds the weekday header row (e.g., "Mon", "Tue").
func datePickerWeekdays(cfg *DatePickerCfg) View {
	dn := &DefaultDatePickerStyle
	cellSpacing := cfg.CellSpacing.Get(dn.CellSpacing)
	cellSize := datePickerCellSize(cfg)
	wdTS := cfg.TextStyle
	wdTS.Color = RGBA(wdTS.Color.R, wdTS.Color.G, wdTS.Color.B, 160)
	labels := make([]View, 0, 7)
	for i := range 7 {
		dow := datePickerWeekdayIndex(i, cfg.MondayFirstDayOfWeek)
		label := datePickerWeekdayLabel(dow, cfg.WeekdaysLen)
		labels = append(labels, Column(ContainerCfg{
			MinWidth:   cellSize,
			MaxWidth:   cellSize,
			HAlign:     HAlignCenter,
			SizeBorder: NoBorder,
			Padding:    Some(PaddingThree),
			Content:    []View{Text(TextCfg{Text: label, TextStyle: wdTS})},
		}))
	}
	return Row(ContainerCfg{
		Spacing:    Some(cellSpacing),
		Padding:    NoPadding,
		SizeBorder: NoBorder,
		Content:    labels,
	})
}

// datePickerMonth builds 6 rows of 7 day cells for the current view month.
func datePickerMonth(
	cfg *DatePickerCfg, state datePickerState, w *Window,
) []View {

	dn := &DefaultDatePickerStyle
	radius := cfg.Radius.Get(dn.Radius)
	cellSpacing := cfg.CellSpacing.Get(dn.CellSpacing)
	cellSize := datePickerCellSize(cfg)
	viewTime := datePickerViewTime(state)
	year, month := viewTime.Year(), viewTime.Month()
	firstDay := time.Date(year, month, 1, 0, 0, 0, 0, time.Local)
	daysInMonth := datePickerDaysInMonth(int(month), year)
	startDOW := int(firstDay.Weekday())
	if cfg.MondayFirstDayOfWeek {
		startDOW = (startDOW + 6) % 7 // shift Sunday from 0 to 6
	}

	today := time.Now()
	onSelect := cfg.OnSelect
	selectMultiple := cfg.SelectMultiple

	rows := make([]View, 0, 6)
	day := 1 - startDOW
	for row := range 6 {
		cells := make([]View, 0, 7)
		for col := range 7 {
			d := day + row*7 + col
			if d < 1 || d > daysInMonth {
				if cfg.ShowAdjacentMonths {
					cells = append(cells, datePickerAdjacentCell(
						cfg, state, d, daysInMonth, cellSize))
				} else {
					cells = append(cells, Button(ButtonCfg{
						Color:       ColorTransparent,
						ColorBorder: ColorTransparent,
						Disabled:    true,
						MinWidth:    cellSize,
						MaxWidth:    cellSize,
						MaxHeight:   cellSize,
						Padding:     Some(PaddingThree),
						Content:     []View{Text(TextCfg{Text: " "})},
					}))
				}
				continue
			}
			cellDate := time.Date(year, month, d, 0, 0, 0, 0, time.Local)
			isToday := isSameDay(cellDate, today)
			selected := datePickerIsSelected(cellDate, cfg.Dates)
			disabled := datePickerIsDisabled(cellDate, cfg)
			isFocused := d == state.FocusDay
			dayStr := strconv.Itoa(d)

			cellColor := ColorTransparent
			colorHover := cfg.ColorHover
			if selected {
				cellColor = cfg.ColorSelect
				colorHover = cfg.ColorSelect
			}
			borderColor := ColorTransparent
			if isToday && !cfg.HideTodayIndicator {
				borderColor = cfg.TextStyle.Color
			}
			if isFocused && w.IsFocus(cfg.IDFocus) {
				borderColor = cfg.ColorBorderFocus
			}

			ts := cfg.TextStyle
			if disabled {
				ts.Color = RGBA(ts.Color.R, ts.Color.G, ts.Color.B, 100)
			}

			dayVal := d
			cfgID := cfg.ID
			cells = append(cells, Button(ButtonCfg{
				ID:          cfg.ID + ".day." + strconv.Itoa(d),
				MinWidth:    cellSize,
				MaxWidth:    cellSize,
				MaxHeight:   cellSize,
				Color:       cellColor,
				ColorBorder: borderColor,
				ColorClick:  cfg.ColorSelect,
				ColorHover:  colorHover,
				SizeBorder:  SomeF(2),
				Radius:      Some(radius),
				Padding:     Some(PaddingThree),
				Disabled:    disabled,
				Content: []View{Text(TextCfg{
					Text: dayStr, TextStyle: ts,
				})},
				OnClick: func(_ *Layout, e *Event, w *Window) {
					sm := StateMap[string, datePickerState](w, nsDatePicker, capModerate)
					s, _ := sm.Get(cfgID)
					s.FocusDay = dayVal
					sm.Set(cfgID, s)

					if cfg.IDFocus > 0 {
						w.SetIDFocus(cfg.IDFocus)
					}

					dates := datePickerUpdateSelections(
						dayVal, s, cfg.Dates,
						selectMultiple)
					if onSelect != nil {
						onSelect(dates, e, w)
					}
					e.IsHandled = true
				},
			}))
		}
		rows = append(rows, Row(ContainerCfg{
			Spacing:    Some(cellSpacing),
			Padding:    NoPadding,
			SizeBorder: NoBorder,
			Content:    cells,
		}))
	}
	return rows
}

// datePickerAdjacentCell builds a faded cell for prev/next month.
func datePickerAdjacentCell(
	cfg *DatePickerCfg, state datePickerState,
	day, daysInMonth int, cellSize float32,
) View {
	var adjDay int
	var delta int
	if day < 1 {
		// Previous month.
		prevMonth := state.ViewMonth - 1
		prevYear := state.ViewYear
		if prevMonth < 1 {
			prevMonth = 12
			prevYear--
		}
		adjDay = datePickerDaysInMonth(prevMonth, prevYear) + day
		delta = -1
	} else {
		// Next month.
		adjDay = day - daysInMonth
		delta = 1
	}
	ts := cfg.TextStyle
	ts.Color = RGBA(ts.Color.R, ts.Color.G, ts.Color.B, 80)
	cfgID := cfg.ID
	onSelect := cfg.OnSelect
	selectMultiple := cfg.SelectMultiple

	var idSuffix string
	if delta < 0 {
		idSuffix = "prev"
	} else {
		idSuffix = "next"
	}

	return Button(ButtonCfg{
		ID:          cfg.ID + ".day." + idSuffix + "." + strconv.Itoa(adjDay),
		Color:       ColorTransparent,
		ColorBorder: ColorTransparent,
		MinWidth:    cellSize,
		MaxWidth:    cellSize,
		MaxHeight:   cellSize,
		Padding:     Some(PaddingThree),
		Content: []View{Text(TextCfg{
			Text:      strconv.Itoa(adjDay),
			TextStyle: ts,
		})},
		OnClick: func(_ *Layout, e *Event, w *Window) {
			if cfg.IDFocus > 0 {
				w.SetIDFocus(cfg.IDFocus)
			}
			datePickerNavMonth(cfgID, delta, w)
			// After navigation, select the day in the new month.
			// Retrieve updated state to get correct year/month.
			sm := StateMap[string, datePickerState](w, nsDatePicker, capModerate)
			s, _ := sm.Get(cfgID)
			dates := datePickerUpdateSelections(
				adjDay, s, cfg.Dates,
				selectMultiple)
			if onSelect != nil {
				onSelect(dates, e, w)
			}
			e.IsHandled = true
		},
	})
}

// datePickerYearMonthPicker builds a roller picker for fast month/year selection.
func datePickerYearMonthPicker(
	cfg *DatePickerCfg, state datePickerState,
) View {
	cfgID := cfg.ID
	return DatePickerRoller(DatePickerRollerCfg{
		ID:           cfg.ID + ".roller",
		SelectedDate: datePickerViewTime(state),
		DisplayMode:  RollerMonthYear,
		VisibleItems: 5,
		Color:        ColorTransparent,
		ColorBorder:  ColorTransparent,
		SizeBorder:   NoBorder,
		OnChange: func(t time.Time, w *Window) {
			if cfg.IDFocus > 0 {
				w.SetIDFocus(cfg.IDFocus)
			}
			sm := StateMap[string, datePickerState](
				w, nsDatePicker, capModerate)
			s, _ := sm.Get(cfgID)
			s.ViewMonth = int(t.Month())
			s.ViewYear = t.Year()
			sm.Set(cfgID, s)
			w.UpdateWindow()
		},
	})
}

// datePickerRollerKeyDown handles keyboard for the embedded
// month/year roller. Up/Down = month, Shift+Up/Down = year.
func datePickerRollerKeyDown(
	sm *BoundedMap[string, datePickerState],
	cfgID string, s datePickerState,
	e *Event, w *Window,
) {
	update := func(month, year int) {
		s.ViewMonth = month
		s.ViewYear = year
		sm.Set(cfgID, s)
		w.UpdateWindow()
		e.IsHandled = true
	}
	switch {
	case e.Modifiers == ModNone && e.KeyCode == KeyEscape:
		s.ShowYearMonthPicker = false
		sm.Set(cfgID, s)
		w.UpdateWindow()
		e.IsHandled = true
	case e.Modifiers == ModNone && e.KeyCode == KeyUp:
		m, y := s.ViewMonth-1, s.ViewYear
		if m < 1 {
			m, y = 12, y-1
		}
		update(m, y)
	case e.Modifiers == ModNone && e.KeyCode == KeyDown:
		m, y := s.ViewMonth+1, s.ViewYear
		if m > 12 {
			m, y = 1, y+1
		}
		update(m, y)
	case e.Modifiers == ModShift && e.KeyCode == KeyUp:
		update(s.ViewMonth, s.ViewYear-1)
	case e.Modifiers == ModShift && e.KeyCode == KeyDown:
		update(s.ViewMonth, s.ViewYear+1)
	}
}

// datePickerOnKeyDown handles arrow key navigation.
func datePickerOnKeyDown(cfg *DatePickerCfg, e *Event, w *Window) {
	sm := StateMap[string, datePickerState](w, nsDatePicker, capModerate)
	s, _ := sm.Get(cfg.ID)
	days := datePickerDaysInMonth(s.ViewMonth, s.ViewYear)

	update := func() {
		sm.Set(cfg.ID, s)
		w.UpdateWindow()
		e.IsHandled = true
	}

	switch e.KeyCode {
	case KeyLeft:
		s.FocusDay--
		if s.FocusDay < 1 {
			datePickerNavMonth(cfg.ID, -1, w)
			s, _ = sm.Get(cfg.ID)
			s.FocusDay = datePickerDaysInMonth(s.ViewMonth, s.ViewYear)
		}
		update()
	case KeyRight:
		s.FocusDay++
		if s.FocusDay > days {
			datePickerNavMonth(cfg.ID, 1, w)
			s, _ = sm.Get(cfg.ID)
			s.FocusDay = 1
		}
		update()
	case KeyUp:
		s.FocusDay -= 7
		if s.FocusDay < 1 {
			datePickerNavMonth(cfg.ID, -1, w)
			s, _ = sm.Get(cfg.ID)
			prevDays := datePickerDaysInMonth(s.ViewMonth, s.ViewYear)
			s.FocusDay += prevDays
		}
		update()
	case KeyDown:
		s.FocusDay += 7
		if s.FocusDay > days {
			datePickerNavMonth(cfg.ID, 1, w)
			s, _ = sm.Get(cfg.ID)
			s.FocusDay -= days
		}
		update()
	case KeyHome:
		s.FocusDay = 1
		update()
	case KeyEnd:
		s.FocusDay = days
		update()
	case KeyEnter, KeySpace:
		dates := datePickerUpdateSelections(
			s.FocusDay, s, cfg.Dates,
			cfg.SelectMultiple)
		if cfg.OnSelect != nil {
			cfg.OnSelect(dates, e, w)
		}
		e.IsHandled = true
	}
}

// datePickerNavMonth shifts the view month by delta.
func datePickerNavMonth(id string, delta int, w *Window) {
	sm := StateMap[string, datePickerState](w, nsDatePicker, capModerate)
	s, _ := sm.Get(id)
	s.ViewMonth += delta
	if s.ViewMonth > 12 {
		s.ViewMonth = 1
		s.ViewYear++
	} else if s.ViewMonth < 1 {
		s.ViewMonth = 12
		s.ViewYear--
	}
	sm.Set(id, s)
	w.UpdateWindow()
}

// datePickerUpdateSelections toggles the selected day.
func datePickerUpdateSelections(
	day int, state datePickerState,
	current []time.Time, multi bool,
) []time.Time {
	sel := time.Date(state.ViewYear, time.Month(state.ViewMonth),
		day, 0, 0, 0, 0, time.Local)
	if !multi {
		return []time.Time{sel}
	}
	// Toggle in multi-select mode.
	for i, d := range current {
		if isSameDay(d, sel) {
			result := make([]time.Time, 0, len(current)-1)
			result = append(result, current[:i]...)
			return append(result, current[i+1:]...)
		}
	}
	return append(current, sel)
}

// datePickerIsSelected checks if a date is in the selection.
func datePickerIsSelected(d time.Time, dates []time.Time) bool {
	for _, sel := range dates {
		if isSameDay(sel, d) {
			return true
		}
	}
	return false
}

// datePickerIsDisabled checks if a date is disallowed.
func datePickerIsDisabled(d time.Time, cfg *DatePickerCfg) bool {
	if len(cfg.AllowedDates) > 0 &&
		!slices.ContainsFunc(cfg.AllowedDates, func(ad time.Time) bool {
			return isSameDay(ad, d)
		}) {
		return true
	}
	if len(cfg.AllowedWeekdays) > 0 {
		dpDOW := DatePickerWeekdays((int(d.Weekday())+6)%7 + 1)
		if !slices.Contains(cfg.AllowedWeekdays, dpDOW) {
			return true
		}
	}
	if len(cfg.AllowedMonths) > 0 &&
		!slices.Contains(cfg.AllowedMonths, DatePickerMonths(d.Month())) {
		return true
	}
	if len(cfg.AllowedYears) > 0 &&
		!slices.Contains(cfg.AllowedYears, d.Year()) {
		return true
	}
	return false
}

// isSameDay compares two times ignoring time-of-day.
func isSameDay(a, b time.Time) bool {
	return a.Year() == b.Year() &&
		a.Month() == b.Month() &&
		a.Day() == b.Day()
}

// datePickerViewTime returns a time for the current view month/year.
func datePickerViewTime(state datePickerState) time.Time {
	return time.Date(state.ViewYear, time.Month(state.ViewMonth),
		1, 0, 0, 0, 0, time.Local)
}

// datePickerWeekdayIndex returns the weekday index for column i.
func datePickerWeekdayIndex(i int, mondayFirst bool) int {
	if mondayFirst {
		return (i + 1) % 7 // Mon=1,Tue=2,...,Sun=0
	}
	return i // Sun=0,Mon=1,...,Sat=6
}

// datePickerWeekdayLabel returns the locale weekday label.
func datePickerWeekdayLabel(dow int, wdLen DatePickerWeekdayLen) string {
	switch wdLen {
	case WeekdayThreeLetter:
		return guiLocale.WeekdaysMed[dow]
	case WeekdayFull:
		return guiLocale.WeekdaysFull[dow]
	default:
		return guiLocale.WeekdaysShort[dow]
	}
}

// datePickerDaysInMonth returns the number of days in a month.
func datePickerDaysInMonth(month, year int) int {
	t := time.Date(year, time.Month(month)+1, 0, 0, 0, 0, 0, time.UTC)
	return t.Day()
}

// datePickerCellSize returns the width/height for a single day cell.
// V calculates dynamically via text measurement; approximate here.
func datePickerCellSize(cfg *DatePickerCfg) float32 {
	switch cfg.WeekdaysLen {
	case WeekdayFull:
		return 76
	case WeekdayThreeLetter:
		return 44
	default:
		return 36
	}
}

func applyDatePickerDefaults(cfg *DatePickerCfg) {
	d := &DefaultDatePickerStyle
	if !cfg.Color.IsSet() {
		cfg.Color = d.Color
	}
	if !cfg.ColorHover.IsSet() {
		cfg.ColorHover = d.ColorHover
	}
	if !cfg.ColorFocus.IsSet() {
		cfg.ColorFocus = d.ColorFocus
	}
	if !cfg.ColorClick.IsSet() {
		cfg.ColorClick = d.ColorClick
	}
	if !cfg.ColorBorder.IsSet() {
		cfg.ColorBorder = d.ColorBorder
	}
	if !cfg.ColorBorderFocus.IsSet() {
		cfg.ColorBorderFocus = d.ColorBorderFocus
	}
	if !cfg.ColorSelect.IsSet() {
		cfg.ColorSelect = d.ColorSelect
	}
	if !cfg.Padding.IsSet() {
		cfg.Padding = Some(d.Padding)
	}
	if cfg.TextStyle == (TextStyle{}) {
		cfg.TextStyle = d.TextStyle
	}
	if !cfg.CellSpacing.IsSet() {
		cfg.CellSpacing = Some(d.CellSpacing)
	}
	if !cfg.Radius.IsSet() {
		cfg.Radius = Some(d.Radius)
	}
}
