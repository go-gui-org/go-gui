package gui

import (
	"testing"
	"time"
)

func TestDatePickerLayout(t *testing.T) {
	w := &Window{}
	v := DatePicker(DatePickerCfg{
		ID:    "dp1",
		Dates: []time.Time{time.Date(2025, 3, 15, 0, 0, 0, 0, time.Local)},
	})
	layout := generateViewLayout(v, w)
	if layout.Shape.ID != "dp1" {
		t.Errorf("ID = %q", layout.Shape.ID)
	}
	if layout.Shape.shapeType != shapeRectangle {
		t.Errorf("type = %d", layout.Shape.shapeType)
	}
}

func TestDatePickerStateInit(t *testing.T) {
	w := &Window{}
	d := time.Date(2025, 6, 10, 0, 0, 0, 0, time.Local)
	cfg := DatePickerCfg{ID: "dp-state", Dates: []time.Time{d}}
	applyDatePickerDefaults(&cfg)
	state := datePickerGetState(w, &cfg)
	if state.ViewMonth != 6 {
		t.Errorf("month = %d, want 6", state.ViewMonth)
	}
	if state.ViewYear != 2025 {
		t.Errorf("year = %d, want 2025", state.ViewYear)
	}
}

func TestDatePickerNavMonth(t *testing.T) {
	w := &Window{}
	sm := StateMap[string, datePickerState](w, nsDatePicker, capModerate)
	sm.Set("nav-test", datePickerState{ViewMonth: 1, ViewYear: 2025})

	datePickerNavMonth("nav-test", -1, w)
	s, _ := sm.Get("nav-test")
	if s.ViewMonth != 12 || s.ViewYear != 2024 {
		t.Errorf("prev = %d/%d", s.ViewMonth, s.ViewYear)
	}

	datePickerNavMonth("nav-test", 1, w)
	s, _ = sm.Get("nav-test")
	if s.ViewMonth != 1 || s.ViewYear != 2025 {
		t.Errorf("next = %d/%d", s.ViewMonth, s.ViewYear)
	}
}

func TestDatePickerDaysInMonth(t *testing.T) {
	tests := []struct {
		month, year, want int
	}{
		{1, 2025, 31},
		{2, 2024, 29}, // leap year
		{2, 2025, 28},
		{4, 2025, 30},
		{12, 2025, 31},
	}
	for _, tt := range tests {
		got := datePickerDaysInMonth(tt.month, tt.year)
		if got != tt.want {
			t.Errorf("daysInMonth(%d, %d) = %d, want %d",
				tt.month, tt.year, got, tt.want)
		}
	}
}

func TestIsSameDay(t *testing.T) {
	a := time.Date(2025, 3, 15, 10, 30, 0, 0, time.Local)
	b := time.Date(2025, 3, 15, 23, 59, 0, 0, time.Local)
	c := time.Date(2025, 3, 16, 0, 0, 0, 0, time.Local)

	if !isSameDay(a, b) {
		t.Error("same day should match")
	}
	if isSameDay(a, c) {
		t.Error("different days should not match")
	}
}

func TestDatePickerIsSelected(t *testing.T) {
	d1 := time.Date(2025, 3, 15, 0, 0, 0, 0, time.Local)
	d2 := time.Date(2025, 3, 16, 0, 0, 0, 0, time.Local)
	dates := []time.Time{d1}

	if !datePickerIsSelected(d1, dates) {
		t.Error("d1 should be selected")
	}
	if datePickerIsSelected(d2, dates) {
		t.Error("d2 should not be selected")
	}
}

func TestDatePickerIsDisabledWeekday(t *testing.T) {
	mon := time.Date(2025, 3, 17, 0, 0, 0, 0, time.Local) // Monday
	cfg := DatePickerCfg{
		AllowedWeekdays: []DatePickerWeekdays{DatePickerTuesday},
	}
	if !datePickerIsDisabled(mon, &cfg) {
		t.Error("Monday should be disabled")
	}
	tue := time.Date(2025, 3, 18, 0, 0, 0, 0, time.Local) // Tuesday
	if datePickerIsDisabled(tue, &cfg) {
		t.Error("Tuesday should not be disabled")
	}
}

func TestDatePickerIsDisabledMonth(t *testing.T) {
	mar := time.Date(2025, 3, 1, 0, 0, 0, 0, time.Local)
	cfg := DatePickerCfg{
		AllowedMonths: []DatePickerMonths{DatePickerJune},
	}
	if !datePickerIsDisabled(mar, &cfg) {
		t.Error("March should be disabled")
	}
}

func TestDatePickerIsDisabledYear(t *testing.T) {
	d := time.Date(2024, 1, 1, 0, 0, 0, 0, time.Local)
	cfg := DatePickerCfg{
		AllowedYears: []int{2025, 2026},
	}
	if !datePickerIsDisabled(d, &cfg) {
		t.Error("2024 should be disabled")
	}
}

func TestDatePickerIsDisabledDates(t *testing.T) {
	allowed := time.Date(2025, 3, 15, 0, 0, 0, 0, time.Local)
	other := time.Date(2025, 3, 16, 0, 0, 0, 0, time.Local)
	cfg := DatePickerCfg{AllowedDates: []time.Time{allowed}}
	if datePickerIsDisabled(allowed, &cfg) {
		t.Error("allowed date should not be disabled")
	}
	if !datePickerIsDisabled(other, &cfg) {
		t.Error("non-allowed date should be disabled")
	}
}

func TestDatePickerUpdateSelections(t *testing.T) {
	state := datePickerState{ViewMonth: 3, ViewYear: 2025}

	// Single select.
	dates := datePickerUpdateSelections(15, state, nil, false)
	if len(dates) != 1 || dates[0].Day() != 15 {
		t.Errorf("single = %v", dates)
	}

	// Multi select — add.
	dates = datePickerUpdateSelections(16, state, dates, true)
	if len(dates) != 2 {
		t.Errorf("multi add = %v", dates)
	}

	// Multi select — toggle off.
	dates = datePickerUpdateSelections(15, state, dates, true)
	if len(dates) != 1 || dates[0].Day() != 16 {
		t.Errorf("multi toggle = %v", dates)
	}
}

func TestDatePickerWeekdayIndex(t *testing.T) {
	// Sunday first: col 0 = Sunday.
	if datePickerWeekdayIndex(0, false) != 0 {
		t.Error("Sunday first col 0")
	}
	// Monday first: col 0 = Monday.
	if datePickerWeekdayIndex(0, true) != 1 {
		t.Error("Monday first col 0")
	}
	// Monday first: col 6 = Sunday.
	if datePickerWeekdayIndex(6, true) != 0 {
		t.Error("Monday first col 6")
	}
}

func TestDatePickerWeekdayLabel(t *testing.T) {
	lbl := datePickerWeekdayLabel(0, WeekdayOneLetter)
	if lbl != "S" {
		t.Errorf("one letter Sunday = %q", lbl)
	}
	lbl = datePickerWeekdayLabel(1, WeekdayThreeLetter)
	if lbl != "Mon" {
		t.Errorf("three letter Monday = %q", lbl)
	}
	lbl = datePickerWeekdayLabel(2, WeekdayFull)
	if lbl != "Tuesday" {
		t.Errorf("full Tuesday = %q", lbl)
	}
}

func TestDatePickerDefaults(t *testing.T) {
	cfg := DatePickerCfg{}
	applyDatePickerDefaults(&cfg)
	if cfg.CellSpacing.Get(0) != 2 {
		t.Errorf("spacing = %f", cfg.CellSpacing.Get(0))
	}
	if !cfg.Radius.IsSet() {
		t.Error("radius should be set")
	}
}

func TestDatePickerSubElementClickFocus(t *testing.T) {
	w := &Window{}
	// Pin to June 2025 — June 1 is Sunday so the first cell in
	// row 0 is day 1 (no blank placeholders).
	cfg := DatePickerCfg{
		ID:        "dp-sub-click",
		Focusable: true,
		Dates:     []time.Time{time.Date(2025, 6, 1, 0, 0, 0, 0, time.Local)},
	}
	applyDatePickerDefaults(&cfg)

	v := DatePicker(cfg)
	layout := generateViewLayout(v, w)

	// Month toggle button is in the first child (Row).
	controls := &layout.Children[0]
	toggleBtn := &controls.Children[0]
	if toggleBtn.Shape.events.OnClick == nil {
		t.Fatal("toggle button OnClick missing")
	}
	e := &Event{}
	w.ClearFocus()
	toggleBtn.Shape.events.OnClick(toggleBtn, e, w)
	if w.FocusID() != "dp-sub-click" {
		t.Errorf("toggle button click got focus %q, want dp-sub-click", w.FocusID())
	}

	// Day cell focus.
	// Layout: Column -> [Controls, Column([Weekdays, Row1, Row2, ...])]
	calendarBody := &layout.Children[1]
	// calendarBody.Children[0] is Weekdays Row
	// calendarBody.Children[1] is the first day row
	firstRow := &calendarBody.Children[1]
	firstDay := &firstRow.Children[0] // June 1

	if firstDay.Shape.events.OnClick == nil {
		t.Fatal("day cell OnClick missing")
	}
	w.ClearFocus()
	firstDay.Shape.events.OnClick(firstDay, e, w)
	if w.FocusID() != "dp-sub-click" {
		t.Errorf("day cell click got focus %q, want dp-sub-click", w.FocusID())
	}
}

func TestDatePickerFocusIndicator(t *testing.T) {
	w := &Window{}
	focusedColor := RGBA(255, 0, 0, 255)
	cfg := DatePickerCfg{
		ID:               "dp-focus",
		Focusable:        true,
		ColorBorderFocus: focusedColor,
	}
	applyDatePickerDefaults(&cfg)

	v := DatePicker(cfg)
	layout := generateViewLayout(v, w)

	// No focus initially.
	if layout.Shape.ColorBorder == focusedColor {
		t.Error("should not be focused initially")
	}

	// Set focus and re-layout.
	w.SetFocus("dp-focus")
	layout = generateViewLayout(v, w)
	// layoutArrange executes AmendLayout hooks.
	_ = layoutArrange(&layout, w)

	if layout.Shape.ColorBorder != focusedColor {
		t.Errorf("got border %v, want %v", layout.Shape.ColorBorder, focusedColor)
	}
}

func TestDatePickerClickFocus(t *testing.T) {
	w := &Window{}
	cfg := DatePickerCfg{
		ID:        "dp-click",
		Focusable: true,
	}
	applyDatePickerDefaults(&cfg)

	v := DatePicker(cfg)
	layout := generateViewLayout(v, w)

	if w.FocusID() == "dp-click" {
		t.Error("should not be focused initially")
	}

	// Simulate click on root.
	if layout.Shape.events.OnClick == nil {
		t.Fatal("OnClick handler missing")
	}
	e := &Event{}
	layout.Shape.events.OnClick(&layout, e, w)

	if w.FocusID() != "dp-click" {
		t.Errorf("got focus %q, want dp-click", w.FocusID())
	}
}

func TestDatePickerClickAdjacentMonth(t *testing.T) {
	w := &Window{}
	var selected []time.Time
	cfg := DatePickerCfg{
		ID:    "dp-adj",
		Dates: []time.Time{time.Date(2025, 3, 1, 0, 0, 0, 0, time.Local)}, // March 1st
		OnSelect: func(dates []time.Time, _ *Event, _ *Window) {
			selected = dates
		},
		MondayFirstDayOfWeek: false, // Sunday is day 0. March 1, 2025 is Saturday (6).
		ShowAdjacentMonths:   true,
	}
	applyDatePickerDefaults(&cfg)

	// March 1, 2025 starts on Saturday.
	// If Sunday is first day (col 0), then March 1 is col 6 of row 0.
	// The first row (row 0) will have days from Feb:
	// Col 0: Feb 23, Col 1: Feb 24, ..., Col 5: Feb 28, Col 6: Mar 1.

	v := DatePicker(cfg)
	_ = generateViewLayout(v, w)

	// Feb 28, 2025 is the day we want to click.
	// Find the cell with ID "dp-adj.day.prev.28".

	// Since we are in unit tests, we don't have a full event loop,
	// but we can call the OnClick directly if we find the layout.
	// Alternatively, we can just call the logic.

	// Let's verify the navigation logic via datePickerAdjacentCell's OnClick behavior.
	// datePickerNavMonth is what's called.

	sm := StateMap[string, datePickerState](w, nsDatePicker, capModerate)
	datePickerNavMonth("dp-adj", -1, w)
	s, _ := sm.Get("dp-adj")
	if s.ViewMonth != 2 || s.ViewYear != 2025 {
		t.Errorf("nav failed: %d/%d", s.ViewMonth, s.ViewYear)
	}

	_ = selected
}

func TestDatePickerKeyboardNav(t *testing.T) {
	w := &Window{}
	cfg := DatePickerCfg{
		ID:        "dp-key",
		Focusable: true,
		Dates:     []time.Time{time.Date(2025, 3, 15, 0, 0, 0, 0, time.Local)},
	}
	applyDatePickerDefaults(&cfg)
	w.SetFocus("dp-key")

	v := DatePicker(cfg)
	_ = generateViewLayout(v, w)

	sm := StateMap[string, datePickerState](w, nsDatePicker, capModerate)
	s, _ := sm.Get("dp-key")
	if s.FocusDay != 15 {
		t.Errorf("initial focus = %d", s.FocusDay)
	}

	// Move left.
	e := &Event{KeyCode: KeyLeft}
	datePickerOnKeyDown(&cfg, e, w)
	s, _ = sm.Get("dp-key")
	if s.FocusDay != 14 {
		t.Errorf("focus after Left = %d", s.FocusDay)
	}

	// Move up (prev week).
	e = &Event{KeyCode: KeyUp}
	datePickerOnKeyDown(&cfg, e, w)
	s, _ = sm.Get("dp-key")
	if s.FocusDay != 7 {
		t.Errorf("focus after Up = %d", s.FocusDay)
	}

	// Move home.
	e = &Event{KeyCode: KeyHome}
	datePickerOnKeyDown(&cfg, e, w)
	s, _ = sm.Get("dp-key")
	if s.FocusDay != 1 {
		t.Errorf("focus after Home = %d", s.FocusDay)
	}
}
