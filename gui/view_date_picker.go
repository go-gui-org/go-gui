package gui

import (
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
	TextStyle       TextStyle
	OnSelect        func([]time.Time, *Event, *Window)
	ID              string `gui:"required"`
	A11YLabel       string
	A11YDescription string
	Dates           []time.Time
	AllowedWeekdays []DatePickerWeekdays
	AllowedMonths   []DatePickerMonths
	AllowedYears    []int
	AllowedDates    []time.Time
	Padding         Opt[Padding]
	SizeBorder      Opt[float32]
	CellSpacing     Opt[float32]
	Radius          Opt[float32]
	RadiusBorder    Opt[float32]
	// FocusDisabled opts out of the default-on focus. Focus also
	// requires a non-empty ID; without one the control is inert.
	FocusDisabled        bool
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
	RequireID("DatePicker", cfg.ID)
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
		Focusable:   !cfg.FocusDisabled,
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
			if w.IsFocus(cfg.ID) {
				lo.Shape.ColorBorder = cfg.ColorBorderFocus
			}
		},
		OnClick: func(_ *Layout, e *Event, w *Window) {
			if !cfg.Disabled {
				w.SetFocus(cfg.ID)
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
	return generateViewLayout(col, w)
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
		ActiveLocale.Date.MonthYear,
	)

	focusID := cfg.ID
	onToggle := func(_ *Layout, e *Event, w *Window) {
		sm := StateMap[string, datePickerState](w, nsDatePicker, capModerate)
		s, _ := sm.Get(cfgID)
		s.ShowYearMonthPicker = !s.ShowYearMonthPicker
		sm.Set(cfgID, s)
		if focusID != "" {
			w.SetFocus(focusID)
		}
		w.UpdateWindow()
		e.IsHandled = true
	}

	onPrev := func(_ *Layout, e *Event, w *Window) {
		if focusID != "" {
			w.SetFocus(focusID)
		}
		datePickerNavMonth(cfgID, -1, w)
		e.IsHandled = true
	}

	onNext := func(_ *Layout, e *Event, w *Window) {
		if focusID != "" {
			w.SetFocus(focusID)
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

// datePickerUpdateSelections toggles the selected day.
func datePickerUpdateSelections(
	day int, state datePickerState,
	current []time.Time, multi bool,
) []time.Time {
	sel := time.Date(state.ViewYear, time.Month(state.ViewMonth),
		day, 0, 0, 0, 0, time.Local) //nolint:gosmopolitan // calendar widget uses local timezone
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
