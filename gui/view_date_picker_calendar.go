package gui

import (
	"slices"
	"strconv"
	"time"
)

// datePickerCalendar builds the weekday headers and the day grid.
func datePickerCalendar(
	cfg *DatePickerCfg, state datePickerState, w *Window,
) View {
	dn := &defaultDatePickerStyle
	cellSpacing := cfg.CellSpacing.Get(dn.cellSpacing)
	content := make([]View, 0, 7)
	content = append(content, datePickerWeekdays(cfg))
	content = append(content, datePickerMonth(cfg, state, w)...)
	cfgID := cfg.ID
	return Column(ContainerCfg{
		Spacing:    Some(cellSpacing),
		Padding:    NoPadding,
		SizeBorder: NoBorder,
		Content:    content,
		AmendLayout: func(ctx EventCtx) {
			sm := StateMap[string, datePickerState](
				ctx.Window, nsDatePicker, capModerate)
			s, ok := sm.Get(cfgID)
			if !ok {
				return
			}
			if s.CalBodyHeight != ctx.Layout.Shape.Height {
				s.CalBodyHeight = ctx.Layout.Shape.Height
				sm.Set(cfgID, s)
			}
		},
	})
}

// datePickerWeekdays builds the weekday header row (e.g., "Mon", "Tue").
func datePickerWeekdays(cfg *DatePickerCfg) View {
	dn := &defaultDatePickerStyle
	cellSpacing := cfg.CellSpacing.Get(dn.cellSpacing)
	cellSize := datePickerCellSize(cfg)
	// A weekday header names the column; the dates are what is read.
	wdTS := withRoleAlpha(cfg.TextStyle, guiTheme.TextStyleSecondary)
	labels := make([]View, 0, 7)
	for i := range 7 {
		dow := datePickerWeekdayIndex(i, cfg.MondayFirstDayOfWeek)
		label := datePickerWeekdayLabel(dow, cfg.WeekdaysLen)
		labels = append(labels, Column(ContainerCfg{
			MinWidth:   cellSize,
			MaxWidth:   cellSize,
			HAlign:     HAlignCenter,
			SizeBorder: NoBorder,
			Padding:    paddingThree,
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

	dn := &defaultDatePickerStyle
	radius := cfg.Radius.Get(dn.Radius)
	cellSpacing := cfg.CellSpacing.Get(dn.cellSpacing)
	cellSize := datePickerCellSize(cfg)
	viewTime := datePickerViewTime(state)
	year, month := viewTime.Year(), viewTime.Month()
	firstDay := time.Date(year, month, 1, 0, 0, 0, 0, time.Local) //nolint:gosmopolitan // calendar widget uses local timezone
	daysInMonth := datePickerDaysInMonth(int(month), year)
	startDOW := int(firstDay.Weekday())
	if cfg.MondayFirstDayOfWeek {
		startDOW = (startDOW + 6) % 7 // shift Sunday from 0 to 6
	}

	// The window's clock, not the wall clock: a time-travel scrub pins a
	// virtual instant, and a calendar that read time.Now() would ring a
	// different cell than the snapshot it is rendering. w.Now() is
	// nil-safe and falls back to the wall clock.
	today := w.Now()
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
					// Blank spacer for a day outside the month: no
					// label, no handler, nothing to focus. This is the
					// decorative case FocusDisabled exists for, so opt
					// out rather than invent an ID for empty space.
					cells = append(cells, Button(ButtonCfg{
						// Same band as the cells it stands among, so
						// the whole grid is corrected once and the
						// recording does not carry an odd one out.
						opticalDigitLabel: true,
						FocusDisabled:     true,
						Color:             ColorTransparent,
						Colors:            ColorSet{Border: ColorTransparent}.resolved(ColorTransparent, themeButtonSet()),
						Disabled:          true,
						MinWidth:          cellSize,
						MaxWidth:          cellSize,
						MaxHeight:         cellSize,
						Padding:           paddingThree,
						Content:           []View{Text(TextCfg{Text: " "})},
					}))
				}
				continue
			}
			cellDate := time.Date(year, month, d, 0, 0, 0, 0, time.Local) //nolint:gosmopolitan // calendar widget uses local timezone
			isToday := isSameDay(cellDate, today)
			selected := datePickerIsSelected(cellDate, cfg.Dates)
			disabled := datePickerIsDisabled(cellDate, cfg)
			isFocused := d == state.FocusDay
			dayStr := strconv.Itoa(d)

			cellColor := ColorTransparent
			colorHover := cfg.Colors.Hover
			if selected {
				cellColor = cfg.ColorSelect
				colorHover = cfg.ColorSelect
			}
			borderColor := ColorTransparent
			if isToday && !cfg.HideTodayIndicator {
				borderColor = cfg.TextStyle.Color
			}
			if isFocused && w.IsFocus(cfg.ID) {
				borderColor = cfg.Colors.BorderFocus
			}

			// The selected fill and the day number on it travel
			// together (issue #373).
			ts := textOnFill(cfg.TextStyle, selected, cfg.ColorTextOnSelect)
			if disabled {
				ts = withRoleAlpha(ts, guiTheme.TextStyleDisabled)
			}

			dayVal := d
			cfgID := cfg.ID
			daySound := resolveSoundCue(guiTheme.Sounds.Selection,
				cfg.Sound, cfg.SoundDisabled)
			cells = append(cells, Button(ButtonCfg{
				ID: ScopeIDN(cfg.ID, "day", d),
				// Picking a day is choosing one of a month's worth,
				// so the selection role rather than Button's own
				// click default. SoundDisabled goes through too: a
				// resolved SoundNone reads as "unset" inside
				// ButtonCfg and would fall back to the theme
				// (issue #467).
				Sound:         daySound,
				SoundDisabled: daySound == SoundNone,
				// A day number is digits by construction, and figures
				// measure shorter than caps: on the cap band every cell
				// would land low (issue #346).
				opticalDigitLabel: true,
				MinWidth:          cellSize,
				MaxWidth:          cellSize,
				MaxHeight:         cellSize,
				Color:             cellColor,
				Colors:            ColorSet{Hover: colorHover, Click: cfg.ColorSelect, Border: borderColor}.resolved(cellColor, themeButtonSet()),
				SizeBorder:        SomeF(2),
				Radius:            Some(radius),
				Padding:           paddingThree,
				Disabled:          disabled,
				Content: []View{Text(TextCfg{
					Text: dayStr, TextStyle: ts,
				})},
				OnClick: func(ctx EventCtx) {
					sm := StateMap[string, datePickerState](ctx.Window, nsDatePicker, capModerate)
					s, ok := sm.Get(cfgID)
					if !ok {
						// No picker state: not ours
						return
					}
					s.FocusDay = dayVal
					sm.Set(cfgID, s)

					if !cfg.FocusDisabled {
						ctx.Window.SetFocus(cfg.ID)
					}

					dates := datePickerUpdateSelections(
						dayVal, s, cfg.Dates,
						selectMultiple)
					if onSelect != nil {
						onSelect(dates, EventCtx{nil, ctx.Event, ctx.Window})
					}
					// Picking a day is the click. The calendar sits
					// inside the picker and, when popped over a field,
					// inside that field too.
					ctx.Consume()
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
	// An adjacent-month day is context, not an unavailable date: it
	// takes the secondary role, not the disabled one.
	ts := withRoleAlpha(cfg.TextStyle, guiTheme.TextStyleSecondary)
	cfgID := cfg.ID
	onSelect := cfg.OnSelect
	selectMultiple := cfg.SelectMultiple

	var idSuffix string
	if delta < 0 {
		idSuffix = "prev"
	} else {
		idSuffix = "next"
	}

	adjSound := resolveSoundCue(guiTheme.Sounds.Selection,
		cfg.Sound, cfg.SoundDisabled)

	return Button(ButtonCfg{
		ID: ScopeIDN(ScopeID(cfg.ID, "day", idSuffix), "", adjDay),
		// Digits, like the in-month cells beside it, and it has to sit
		// level with them.
		opticalDigitLabel: true,
		Color:             ColorTransparent,
		Colors:            ColorSet{Border: ColorTransparent}.resolved(ColorTransparent, themeButtonSet()),
		// An adjacent-month cell selects a day too — it just navigates
		// first — so it takes the same cue as an in-month one.
		Sound:         adjSound,
		SoundDisabled: adjSound == SoundNone,
		MinWidth:      cellSize,
		MaxWidth:      cellSize,
		MaxHeight:     cellSize,
		Padding:       paddingThree,
		Content: []View{Text(TextCfg{
			Text:      strconv.Itoa(adjDay),
			TextStyle: ts,
		})},
		OnClick: func(ctx EventCtx) {
			if !cfg.FocusDisabled {
				ctx.Window.SetFocus(cfg.ID)
			}
			datePickerNavMonth(cfgID, delta, ctx.Window)
			// After navigation, select the day in the new month.
			// Retrieve updated state to get correct year/month.
			sm := StateMap[string, datePickerState](ctx.Window, nsDatePicker, capModerate)
			s, ok := sm.Get(cfgID)
			if !ok {
				// No picker state: not ours
				return
			}
			dates := datePickerUpdateSelections(
				adjDay, s, cfg.Dates,
				selectMultiple)
			if onSelect != nil {
				onSelect(dates, EventCtx{nil, ctx.Event, ctx.Window})
			}
			// As for an in-month day: navigating and selecting is the
			// whole click.
			ctx.Consume()
		},
	})
}

// datePickerYearMonthPicker builds a roller picker for fast month/year selection.
func datePickerYearMonthPicker(
	cfg *DatePickerCfg, state datePickerState,
) View {
	cfgID := cfg.ID
	return DatePickerRoller(DatePickerRollerCfg{
		ID:           ScopeID(cfg.ID, "roller"),
		SelectedDate: datePickerViewTime(state),
		DisplayMode:  RollerMonthYear,
		VisibleItems: 5,
		Color:        ColorTransparent,
		ColorBorder:  ColorTransparent,
		SizeBorder:   NoBorder,
		OnChange: func(t time.Time, ctx EventCtx) {
			if !cfg.FocusDisabled {
				ctx.Window.SetFocus(cfg.ID)
			}
			sm := StateMap[string, datePickerState](
				ctx.Window, nsDatePicker, capModerate)
			s, ok := sm.Get(cfgID)
			if !ok {
				return
			}
			s.ViewMonth = int(t.Month())
			s.ViewYear = t.Year()
			sm.Set(cfgID, s)
			ctx.Window.UpdateWindow()
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

// datePickerNavMonth shifts the view month by delta.
func datePickerNavMonth(id string, delta int, w *Window) {
	sm := StateMap[string, datePickerState](w, nsDatePicker, capModerate)
	s, ok := sm.Get(id)
	if !ok {
		return
	}
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
		1, 0, 0, 0, 0, time.Local) //nolint:gosmopolitan // calendar widget uses local timezone
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
		return ActiveLocale.WeekdaysMed[dow]
	case WeekdayFull:
		return ActiveLocale.WeekdaysFull[dow]
	default:
		return ActiveLocale.WeekdaysShort[dow]
	}
}
