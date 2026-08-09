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
	OnSelect        func([]time.Time, EventCtx)
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
	FocusDisabled bool
	Color         Color
	// Colors sets the per-state colors. Color above is the
	// shorthand for Colors.Base and wins over it.
	Colors               ColorSet
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

	// One resolved identity for every key below; see (*Window).EffID.
	cfg.ID = w.EffID(cfg.ID)
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
		Color:       cfg.Colors.Base,
		ColorBorder: cfg.Colors.Border,
		SizeBorder:  cfg.SizeBorder,
		Radius:      Some(radiusBorder),
		Padding:     cfg.Padding,
		Spacing:     Some(cellSpacing),
		MinWidth:    minWidth,
		MinHeight:   minHeight,
		Disabled:    cfg.Disabled,
		Invisible:   cfg.Invisible,
		Content:     content,
		AmendLayout: func(ctx EventCtx) {
			if ctx.Window.IsFocus(cfg.ID) {
				ctx.Layout.Shape.ColorBorder = cfg.Colors.BorderFocus
			}
		},
		OnClick: func(ctx EventCtx) {
			if !cfg.Disabled {
				ctx.Window.SetFocus(cfg.ID)
			}
		},
		OnKeyDown: func(ctx EventCtx) {
			sm := StateMap[string, datePickerState](
				ctx.Window, nsDatePicker, capModerate)
			s, ok := sm.Get(cfgID)
			if !ok {
				return
			}
			if s.ShowYearMonthPicker {
				datePickerRollerKeyDown(
					sm, cfgID, s, ctx.Event, ctx.Window)
			} else {
				datePickerOnKeyDown(cfg, ctx.Event, ctx.Window)
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
	onToggle := func(ctx EventCtx) {
		sm := StateMap[string, datePickerState](ctx.Window, nsDatePicker, capModerate)
		s, ok := sm.Get(cfgID)
		if !ok {
			return
		}
		s.ShowYearMonthPicker = !s.ShowYearMonthPicker
		sm.Set(cfgID, s)
		if focusID != "" {
			ctx.Window.SetFocus(focusID)
		}
		ctx.Window.UpdateWindow()
		ctx.Consume()
	}

	onPrev := func(ctx EventCtx) {
		if focusID != "" {
			ctx.Window.SetFocus(focusID)
		}
		datePickerNavMonth(cfgID, -1, ctx.Window)
		ctx.Consume()
	}

	onNext := func(ctx EventCtx) {
		if focusID != "" {
			ctx.Window.SetFocus(focusID)
		}
		datePickerNavMonth(cfgID, 1, ctx.Window)
		ctx.Consume()
	}

	return Row(ContainerCfg{
		VAlign:     VAlignMiddle,
		Padding:    NoPadding,
		SizeBorder: NoBorder,
		Sizing:     FillFit,
		Content: []View{
			Button(ButtonCfg{
				// Namespaced by the picker's ID so two date pickers in
				// one window keep separate focus and state identities.
				ID:      ScopeID(cfgID, "month"),
				Color:   ColorTransparent,
				Colors:  ColorSet{Border: ColorTransparent},
				OnClick: onToggle,
				Content: []View{Text(TextCfg{
					Text: monthLabel, TextStyle: cfg.TextStyle,
				})},
			}),
			Rectangle(RectangleCfg{Sizing: FillFit}),
			Button(ButtonCfg{
				ID:       ScopeID(cfgID, "prev"),
				Disabled: state.ShowYearMonthPicker,
				Color:    ColorTransparent,
				Colors:   ColorSet{Border: ColorTransparent},
				OnClick:  onPrev,
				Content: []View{Text(TextCfg{
					Text:      IconArrowLeft,
					TextStyle: CurrentTheme().Icon3,
				})},
			}),
			Button(ButtonCfg{
				ID:       ScopeID(cfgID, "next"),
				Disabled: state.ShowYearMonthPicker,
				Color:    ColorTransparent,
				Colors:   ColorSet{Border: ColorTransparent},
				OnClick:  onNext,
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
	s, ok := sm.Get(cfg.ID)
	if !ok {
		return
	}
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
			s, ok = sm.Get(cfg.ID)
			if !ok {
				return
			}
			s.FocusDay = datePickerDaysInMonth(s.ViewMonth, s.ViewYear)
		}
		update()
	case KeyRight:
		s.FocusDay++
		if s.FocusDay > days {
			datePickerNavMonth(cfg.ID, 1, w)
			s, ok = sm.Get(cfg.ID)
			if !ok {
				return
			}
			s.FocusDay = 1
		}
		update()
	case KeyUp:
		s.FocusDay -= 7
		if s.FocusDay < 1 {
			datePickerNavMonth(cfg.ID, -1, w)
			s, ok = sm.Get(cfg.ID)
			if !ok {
				return
			}
			prevDays := datePickerDaysInMonth(s.ViewMonth, s.ViewYear)
			s.FocusDay += prevDays
		}
		update()
	case KeyDown:
		s.FocusDay += 7
		if s.FocusDay > days {
			datePickerNavMonth(cfg.ID, 1, w)
			s, ok = sm.Get(cfg.ID)
			if !ok {
				return
			}
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
			cfg.OnSelect(dates, EventCtx{nil, e, w})
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
	cfg.Colors = cfg.Colors.resolved(cfg.Color, themeColorSet(
		d.Color, d.ColorHover, d.ColorClick,
		d.ColorFocus, d.ColorBorder, d.ColorBorderFocus,
	))
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
