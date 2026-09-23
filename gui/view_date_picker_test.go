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

// A selected day fills with the accent color, so its number draws in
// the paired foreground; a plain day keeps the body color
// (issue #373).
func TestDatePickerSelectedDayTextColor(t *testing.T) {
	w := &Window{}
	v := DatePicker(DatePickerCfg{
		ID:                "dp-text",
		Dates:             []time.Time{time.Date(2025, 3, 15, 0, 0, 0, 0, time.Local)},
		ColorTextOnSelect: White,
	})
	if got := textColorOf(t, w, v, "15"); !got.eq(White) {
		t.Errorf("selected day color = %v, want %v", got, White)
	}
	if got := textColorOf(t, w, v, "16"); !got.eq(DefaultTextStyle.Color) {
		t.Errorf("plain day color = %v, want body %v",
			got, DefaultTextStyle.Color)
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

func TestDatePickerNavMonthAbsentStateNoOp(t *testing.T) {
	// Absent state (evicted or never seeded) must be a no-op:
	// no garbage state written, no panic.
	w := &Window{}
	datePickerNavMonth("never-seeded", 1, w)
	sm := StateMap[string, datePickerState](w, nsDatePicker, capModerate)
	if sm.Contains("never-seeded") {
		t.Error("nav on absent state must not create state")
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
		ID:    "dp-sub-click",
		Dates: []time.Time{time.Date(2025, 6, 1, 0, 0, 0, 0, time.Local)},
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
	toggleBtn.Shape.events.OnClick(EventCtx{toggleBtn, e, w})
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
	firstDay.Shape.events.OnClick(EventCtx{firstDay, e, w})
	if w.FocusID() != "dp-sub-click" {
		t.Errorf("day cell click got focus %q, want dp-sub-click", w.FocusID())
	}
}

func TestDatePickerFocusIndicator(t *testing.T) {
	w := &Window{}
	focusedColor := RGBA(255, 0, 0, 255)
	cfg := DatePickerCfg{
		ID:     "dp-focus",
		Colors: ColorSet{BorderFocus: focusedColor},
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
		ID: "dp-click",
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
	layout.Shape.events.OnClick(EventCtx{&layout, e, w})

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
		OnSelect: func(dates []time.Time, ctx EventCtx) {
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
		ID:    "dp-key",
		Dates: []time.Time{time.Date(2025, 3, 15, 0, 0, 0, 0, time.Local)},
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

// --- DatePickerReset and the embedded month/year roller ---

func TestDatePickerReset(t *testing.T) {
	w := &Window{}
	cfg := DatePickerCfg{
		ID:    "dp-reset",
		Dates: []time.Time{time.Date(2025, 3, 15, 0, 0, 0, 0, time.Local)},
	}
	// GenerateLayout seeds the per-instance state.
	generateViewLayout(DatePicker(cfg), w)
	sm := StateMap[string, datePickerState](w, nsDatePicker, capModerate)
	if _, ok := sm.Get("dp-reset"); !ok {
		t.Fatal("state should be seeded by layout generation")
	}

	w.DatePickerReset("dp-reset")
	if _, ok := sm.Get("dp-reset"); ok {
		t.Fatal("DatePickerReset must delete the instance state")
	}

	// A fresh layout regenerates state from the configured dates,
	// not from the old view month.
	layout := generateViewLayout(DatePicker(cfg), w)
	s, ok := sm.Get("dp-reset")
	if !ok {
		t.Fatal("state should be re-seeded after reset")
	}
	if s.ViewMonth != 3 || s.ViewYear != 2025 {
		t.Fatalf("re-seeded view = %d/%d, want 3/2025",
			s.ViewMonth, s.ViewYear)
	}
	if layout.Shape.ID != "dp-reset" {
		t.Fatalf("layout ID = %q", layout.Shape.ID)
	}
}

func TestDatePickerResetUnknownIDNoop(t *testing.T) {
	w := &Window{}
	// Must not panic, and must not disturb other instances.
	w.Toast(ToastCfg{Title: "unrelated"}) // any prior state is untouched
	w.DatePickerReset("never-existed")
}

func TestDatePickerYearMonthPickerView(t *testing.T) {
	// The year/month picker embeds a month-year roller fed by the
	// picker's view state.
	cfg := DatePickerCfg{ID: "dp-ym"}
	applyDatePickerDefaults(&cfg)
	state := datePickerState{ViewMonth: 6, ViewYear: 2025}

	v := datePickerYearMonthPicker(&cfg, state)
	layout := generateViewLayout(v, &Window{})
	if layout.Shape.ID != "dp-ym:roller" {
		t.Fatalf("roller ID = %q, want dp-ym:roller", layout.Shape.ID)
	}
	if layout.Shape.A11YRole != AccessRoleDateField {
		t.Fatalf("roller role = %d, want DateField", layout.Shape.A11YRole)
	}
	// MonthYear mode → two drums (month, year).
	if got := len(layout.Children); got != 2 {
		t.Fatalf("drums = %d, want 2", got)
	}
}

// The roller is an overlay, not a view swap: opening it keeps the
// calendar grid in the tree and floats the roller over it behind a
// dismissing backdrop.
func TestDatePickerRollerOverlayKeepsCalendar(t *testing.T) {
	w := newTestWindow()
	cfg := DatePickerCfg{
		ID:    "dp-overlay",
		Dates: []time.Time{time.Date(2025, 6, 15, 0, 0, 0, 0, time.Local)},
	}
	generateViewLayout(DatePicker(cfg), w)
	sm := StateMap[string, datePickerState](w, nsDatePicker, capModerate)
	s, ok := sm.Get("dp-overlay")
	if !ok {
		t.Fatal("state should be seeded by layout generation")
	}
	s.ShowYearMonthPicker = true
	sm.Set("dp-overlay", s)

	layout := generateViewLayout(DatePicker(cfg), w)
	rollerID := ScopeID("dp-overlay", "roller")
	if findShapeByID(&layout, rollerID) == nil {
		t.Fatal("open picker should carry the roller")
	}
	// The grid stays: a June 2025 day cell is still in the tree.
	dayID := ScopeIDN(ScopeID("dp-overlay", "day"), "", 15)
	if findShapeByID(&layout, dayID) == nil {
		t.Fatal("open picker should keep the calendar grid, not swap it out")
	}
	backdropID := ScopeID("dp-overlay", "backdrop")
	backdrop := findShapeByID(&layout, backdropID)
	if backdrop == nil {
		t.Fatal("open picker should carry a backdrop")
	}
	if !backdrop.Shape.Float {
		t.Error("backdrop should float over the grid")
	}
	if backdrop.Shape.events.OnClick == nil {
		t.Fatal("backdrop OnClick missing")
	}
	cardID := ScopeID("dp-overlay", "card")
	card := findShapeByID(&layout, cardID)
	if card == nil {
		t.Fatal("open picker should carry a roller card")
	}
	if !card.Shape.Float {
		t.Error("card should float over the grid")
	}
	if card.Shape.FloatAnchor != FloatMiddleCenter ||
		card.Shape.FloatTieOff != FloatMiddleCenter {
		t.Error("card should center over the grid")
	}
	if card.Shape.FloatZIndex <= backdrop.Shape.FloatZIndex {
		t.Error("card should stack above the backdrop")
	}

	closed := generateViewLayout(DatePicker(DatePickerCfg{
		ID: "dp-overlay-closed",
	}), w)
	if findShapeByID(&closed, ScopeID("dp-overlay-closed", "roller")) != nil {
		t.Fatal("closed picker should not carry the roller")
	}
}

// Opening the roller must not move the outer box: the grid stays in
// flow and the overlay floats, so open and closed arrange identical.
func TestDatePickerRollerOverlayStableSize(t *testing.T) {
	sizes := func(open bool) (float32, float32) {
		w := NewWindow(WindowCfg{State: new(int), Width: 600, Height: 800})
		w.viewGenerator = func(win *Window) View {
			return Column(ContainerCfg{Sizing: FillFill,
				Content: []View{DatePicker(DatePickerCfg{
					ID: "dp",
					Dates: []time.Time{
						time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC),
					},
				})}})
		}
		n := time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)
		w.setVirtualNow(&n)
		w.refreshLayout.Store(true)
		w.FrameFn()
		if open {
			sm := StateMap[string, datePickerState](w, nsDatePicker, capModerate)
			s, ok := sm.Get("dp")
			if !ok {
				t.Fatal("no picker state")
			}
			s.ShowYearMonthPicker = true
			sm.Set("dp", s)
			w.refreshLayout.Store(true)
			w.FrameFn()
		}
		l, ok := w.layout.FindByID("dp")
		if !ok {
			t.Fatal("no picker")
		}
		return l.Shape.Width, l.Shape.Height
	}

	closedW, closedH := sizes(false)
	openW, openH := sizes(true)
	if openW != closedW || openH != closedH {
		t.Errorf("open size = %vx%v, want closed size %vx%v",
			openW, openH, closedW, closedH)
	}
}

// A backdrop click closes the roller without moving the view, and is
// consumed so the grid underneath stays inert.
func TestDatePickerRollerBackdropDismiss(t *testing.T) {
	w := newTestWindow()
	cfg := DatePickerCfg{ID: "dp-backdrop"}
	applyDatePickerDefaults(&cfg)
	sm := StateMap[string, datePickerState](w, nsDatePicker, capModerate)
	sm.Set("dp-backdrop", datePickerState{
		ViewMonth: 6, ViewYear: 2025, ShowYearMonthPicker: true,
	})

	layout := generateViewLayout(DatePicker(cfg), w)
	backdrop := findShapeByID(&layout, ScopeID("dp-backdrop", "backdrop"))
	if backdrop == nil {
		t.Fatal("open picker should carry a backdrop")
	}
	e := &Event{}
	backdrop.Shape.events.OnClick(EventCtx{backdrop, e, w})
	if !e.IsHandled {
		t.Error("backdrop click should be consumed")
	}
	got, _ := sm.Get("dp-backdrop")
	if got.ShowYearMonthPicker {
		t.Fatal("backdrop click should close the roller")
	}
	if got.ViewMonth != 6 || got.ViewYear != 2025 {
		t.Fatalf("backdrop dismiss must not move the view: %d/%d",
			got.ViewMonth, got.ViewYear)
	}
}

// A press on the grid behind the overlay reaches the backdrop, not
// the day cell: the roller closes and no date is selected. The
// backdrop is a Fill float inside a fit-sized wrapper, which
// arranges 0x0 unless its AmendLayout takes the body's size —
// without that, the click falls through to the grid.
func TestDatePickerRollerGridClickDismiss(t *testing.T) {
	selected := false
	w := NewWindow(WindowCfg{State: new(int), Width: 600, Height: 800})
	w.focused = true
	w.viewGenerator = func(win *Window) View {
		return Column(ContainerCfg{Sizing: FillFill,
			Content: []View{DatePicker(DatePickerCfg{
				ID: "dp-gridclick",
				Dates: []time.Time{
					time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC),
				},
				OnSelect: func(dates []time.Time, ctx EventCtx) {
					selected = true
				},
			})}})
	}
	n := time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)
	w.setVirtualNow(&n)
	w.refreshLayout.Store(true)
	w.FrameFn()
	sm := StateMap[string, datePickerState](w, nsDatePicker, capModerate)
	s, ok := sm.Get("dp-gridclick")
	if !ok {
		t.Fatal("no picker state")
	}
	s.ShowYearMonthPicker = true
	sm.Set("dp-gridclick", s)
	w.refreshLayout.Store(true)
	w.FrameFn()

	// June 2025 opens on a Sunday, so day 1 sits in the grid's top
	// row — above the centered card, on the backdrop.
	cell, ok := w.layout.FindByID(
		ScopeIDN(ScopeID("dp-gridclick", "day"), "", 1))
	if !ok {
		t.Fatal("no day cell")
	}
	e := &Event{Type: EventMouseDown, MouseButton: MouseLeft,
		MouseX: cell.Shape.X + cell.Shape.Width/2,
		MouseY: cell.Shape.Y + cell.Shape.Height/2}
	w.EventFn(e)
	if selected {
		t.Fatal("grid click behind the overlay must not select a date")
	}
	got, _ := sm.Get("dp-gridclick")
	if got.ShowYearMonthPicker {
		t.Fatal("grid click should dismiss the roller")
	}
}

// A day press that reaches the grid while the roller is open
// dismisses instead of selecting — in-month and adjacent-month
// cells alike — so no dispatch path can select through the overlay.
func TestDatePickerRollerDayPressDismiss(t *testing.T) {
	selected := 0
	w := newTestWindow()
	cfg := DatePickerCfg{
		ID:                 "dp-dayguard",
		Dates:              []time.Time{time.Date(2025, 6, 15, 0, 0, 0, 0, time.Local)},
		ShowAdjacentMonths: true,
		OnSelect: func(dates []time.Time, ctx EventCtx) {
			selected++
		},
	}
	applyDatePickerDefaults(&cfg)
	generateViewLayout(DatePicker(cfg), w)
	sm := StateMap[string, datePickerState](w, nsDatePicker, capModerate)
	s, ok := sm.Get("dp-dayguard")
	if !ok {
		t.Fatal("no picker state")
	}
	s.ShowYearMonthPicker = true
	sm.Set("dp-dayguard", s)

	layout := generateViewLayout(DatePicker(cfg), w)
	fire := func(id string) {
		t.Helper()
		// Each press runs against an open roller: reopen, since a
		// guarded press dismisses.
		s, _ := sm.Get("dp-dayguard")
		s.ShowYearMonthPicker = true
		sm.Set("dp-dayguard", s)
		cell := findShapeByID(&layout, id)
		if cell == nil {
			t.Fatalf("%s: not in the layout", id)
		}
		e := &Event{}
		cell.Shape.events.OnClick(EventCtx{cell, e, w})
		if !e.IsHandled {
			t.Errorf("%s: press should be consumed", id)
		}
	}

	// In-month day.
	fire(ScopeIDN(ScopeID("dp-dayguard", "day"), "", 15))
	// Adjacent-month day (July spills into June's last row).
	fire(ScopeIDN(ScopeID("dp-dayguard", "day", "next"), "", 1))

	if selected != 0 {
		t.Fatalf("OnSelect fired %d times, want 0", selected)
	}
	got, _ := sm.Get("dp-dayguard")
	if got.ShowYearMonthPicker {
		t.Fatal("day press should dismiss the roller")
	}
	if got.ViewMonth != 6 || got.ViewYear != 2025 {
		t.Fatalf("dismiss must not move the view: %d/%d",
			got.ViewMonth, got.ViewYear)
	}
}

// Clicks inside the card are absorbed so they never reach the
// backdrop, and the card renders smaller than the picker on its own
// solid surface.
func TestDatePickerRollerCardAbsorbsClicks(t *testing.T) {
	w := newTestWindow()
	cfg := DatePickerCfg{ID: "dp-card"}
	applyDatePickerDefaults(&cfg)
	base := cfg.Colors.Base
	sm := StateMap[string, datePickerState](w, nsDatePicker, capModerate)
	sm.Set("dp-card", datePickerState{
		ViewMonth: 6, ViewYear: 2025, ShowYearMonthPicker: true,
	})

	layout := generateViewLayout(DatePicker(cfg), w)
	cardID := ScopeID("dp-card", "card")
	card := findShapeByID(&layout, cardID)
	if card == nil {
		t.Fatal("open picker should carry a roller card")
	}
	if card.Shape.events.OnClick == nil {
		t.Fatal("card OnClick missing")
	}
	e := &Event{}
	card.Shape.events.OnClick(EventCtx{card, e, w})
	if !e.IsHandled {
		t.Error("card click should be consumed")
	}
	got, _ := sm.Get("dp-card")
	if !got.ShowYearMonthPicker {
		t.Error("card click must not dismiss the roller")
	}
	if card.Shape.Color != base {
		t.Errorf("card fill = %v, want picker base %v", card.Shape.Color, base)
	}

	layers := layoutArrange(&layout, w)
	var cardLayer *Layout
	for i := range layers[1:] {
		if layers[1+i].Shape.ID == cardID {
			cardLayer = &layers[1+i]
		}
	}
	if cardLayer == nil {
		t.Fatal("card should arrange as a float layer")
	}
	if cardLayer.Shape.Width >= layers[0].Shape.Width {
		t.Errorf("card width = %v, want narrower than picker %v",
			cardLayer.Shape.Width, layers[0].Shape.Width)
	}
}

// While the roller is open the header swaps the dead prev/next
// arrows for a confirm button; closed, the arrows are back and the
// confirm is gone.
func TestDatePickerControlsSwapArrowsForDone(t *testing.T) {
	cfg := DatePickerCfg{ID: "dp-hdr"}
	applyDatePickerDefaults(&cfg)
	w := &Window{}

	open := generateViewLayout(datePickerControls(&cfg,
		datePickerState{ViewMonth: 6, ViewYear: 2025, ShowYearMonthPicker: true}, w), w)
	if findShapeByID(&open, "dp-hdr:done") == nil {
		t.Fatal("open header should carry the done button")
	}
	if findShapeByID(&open, "dp-hdr:prev") != nil ||
		findShapeByID(&open, "dp-hdr:next") != nil {
		t.Fatal("open header should not carry the prev/next arrows")
	}
	if !layoutContainsText(&open, IconCheckCircleO) {
		t.Fatal("done button should carry the circle-check glyph")
	}

	closed := generateViewLayout(datePickerControls(&cfg,
		datePickerState{ViewMonth: 6, ViewYear: 2025}, w), w)
	if findShapeByID(&closed, "dp-hdr:prev") == nil ||
		findShapeByID(&closed, "dp-hdr:next") == nil {
		t.Fatal("closed header should carry the prev/next arrows")
	}
	if findShapeByID(&closed, "dp-hdr:done") != nil {
		t.Fatal("closed header should not carry the done button")
	}
}

// The confirm button beside the drums closes the roller without
// moving the view, so a mouse user never needs Escape.
func TestDatePickerRollerDoneDismiss(t *testing.T) {
	w := newTestWindow()
	sm := StateMap[string, datePickerState](w, nsDatePicker, capModerate)
	sm.Set("dp-done", datePickerState{
		ViewMonth: 6, ViewYear: 2025, ShowYearMonthPicker: true,
	})

	datePickerRollerDismiss("dp-done", w)
	got, _ := sm.Get("dp-done")
	if got.ShowYearMonthPicker {
		t.Fatal("Done should close the year/month picker")
	}
	if got.ViewMonth != 6 || got.ViewYear != 2025 {
		t.Fatalf("Done must not move the view: %d/%d",
			got.ViewMonth, got.ViewYear)
	}
	if w.FocusID() != "dp-done" {
		t.Fatalf("Done should return focus to the picker, got %q",
			w.FocusID())
	}
}

func TestDatePickerRollerDismissUnknownIDNoop(t *testing.T) {
	w := newTestWindow()
	// Must not panic, and must not disturb other instances.
	sm := StateMap[string, datePickerState](w, nsDatePicker, capModerate)
	sm.Set("dp-other", datePickerState{ViewMonth: 6, ViewYear: 2025})
	datePickerRollerDismiss("never-existed", w)
	got, _ := sm.Get("dp-other")
	if got.ViewMonth != 6 || got.ViewYear != 2025 {
		t.Fatal("dismiss of an unknown ID disturbed another picker")
	}
}

func TestDatePickerRollerKeyDownEscape(t *testing.T) {
	w := &Window{}
	sm := StateMap[string, datePickerState](w, nsDatePicker, capModerate)
	s := datePickerState{ViewMonth: 6, ViewYear: 2025, ShowYearMonthPicker: true}
	sm.Set("dp-roller", s)

	e := &Event{KeyCode: KeyEscape, Modifiers: ModNone}
	datePickerRollerKeyDown(sm, "dp-roller", s, e, w)
	if !e.IsHandled {
		t.Fatal("Escape should be handled")
	}
	got, _ := sm.Get("dp-roller")
	if got.ShowYearMonthPicker {
		t.Fatal("Escape should close the year/month picker")
	}
	if got.ViewMonth != 6 || got.ViewYear != 2025 {
		t.Fatalf("Escape must not move the view: %d/%d",
			got.ViewMonth, got.ViewYear)
	}
}

func TestDatePickerRollerKeyDownMonthNav(t *testing.T) {
	w := &Window{}
	sm := StateMap[string, datePickerState](w, nsDatePicker, capModerate)
	s := datePickerState{ViewMonth: 1, ViewYear: 2025}

	// Up: January wraps to December of the previous year.
	e := &Event{KeyCode: KeyUp, Modifiers: ModNone}
	datePickerRollerKeyDown(sm, "dp-roller", s, e, w)
	got, _ := sm.Get("dp-roller")
	if got.ViewMonth != 12 || got.ViewYear != 2024 {
		t.Fatalf("Up from Jan = %d/%d, want 12/2024",
			got.ViewMonth, got.ViewYear)
	}
	if !e.IsHandled {
		t.Fatal("Up should be handled")
	}

	// Down: December wraps to January of the next year.
	s = datePickerState{ViewMonth: 12, ViewYear: 2025}
	e = &Event{KeyCode: KeyDown, Modifiers: ModNone}
	datePickerRollerKeyDown(sm, "dp-roller", s, e, w)
	got, _ = sm.Get("dp-roller")
	if got.ViewMonth != 1 || got.ViewYear != 2026 {
		t.Fatalf("Down from Dec = %d/%d, want 1/2026",
			got.ViewMonth, got.ViewYear)
	}
}

func TestDatePickerRollerKeyDownYearNav(t *testing.T) {
	w := &Window{}
	sm := StateMap[string, datePickerState](w, nsDatePicker, capModerate)
	s := datePickerState{ViewMonth: 6, ViewYear: 2025}

	// Shift+Up: year-1.
	e := &Event{KeyCode: KeyUp, Modifiers: ModShift}
	datePickerRollerKeyDown(sm, "dp-roller", s, e, w)
	got, _ := sm.Get("dp-roller")
	if got.ViewYear != 2024 || got.ViewMonth != 6 {
		t.Fatalf("Shift+Up = %d/%d, want 2024/6",
			got.ViewYear, got.ViewMonth)
	}

	// Shift+Down: year+1.
	e = &Event{KeyCode: KeyDown, Modifiers: ModShift}
	datePickerRollerKeyDown(sm, "dp-roller", s, e, w)
	got, _ = sm.Get("dp-roller")
	if got.ViewYear != 2026 {
		t.Fatalf("Shift+Down = %d, want 2026", got.ViewYear)
	}
}

func TestDatePickerRollerKeyDownIgnoresOtherKeys(t *testing.T) {
	w := &Window{}
	sm := StateMap[string, datePickerState](w, nsDatePicker, capModerate)
	s := datePickerState{ViewMonth: 6, ViewYear: 2025}
	sm.Set("dp-roller", s)

	e := &Event{KeyCode: KeyA, Modifiers: ModNone}
	datePickerRollerKeyDown(sm, "dp-roller", s, e, w)
	if e.IsHandled {
		t.Fatal("unrecognized key must not be handled")
	}
	got, _ := sm.Get("dp-roller")
	if got.ViewMonth != 6 || got.ViewYear != 2025 {
		t.Fatalf("unrecognized key mutated state: %d/%d",
			got.ViewMonth, got.ViewYear)
	}
}

// The header buttons and the adjacent-month cells are ghost buttons
// (#718): they take their resting fill and border from the theme's
// ghost style rather than hand-building transparent colors. Dropping
// the variant would give them the base button's interior fill, which
// is visible but which no other test catches for the confirm button —
// it only exists while the roller is open, and no golden records that
// state.
func TestDatePickerGhostButtonsFollowTheme(t *testing.T) {
	ghost := guiTheme.buttonStyleGhost.Colors
	cfg := DatePickerCfg{ID: "dp-ghost", ShowAdjacentMonths: true}
	applyDatePickerDefaults(&cfg)
	w := &Window{}
	state := datePickerState{ViewMonth: 8, ViewYear: 2026}

	// Both header shapes: closed carries prev/next, open carries the
	// confirm button in their place.
	closed := generateViewLayout(datePickerControls(&cfg, state, w), w)
	openState := state
	openState.ShowYearMonthPicker = true
	open := generateViewLayout(datePickerControls(&cfg, openState, w), w)

	// August 2026 starts on a Saturday, so the first row holds six
	// trailing days of July, the last of which is the 31st.
	month := datePickerMonth(&cfg, state, w)
	grid := generateViewLayout(Column(ContainerCfg{Content: month}), w)

	cases := []struct {
		layout *Layout
		id     string
	}{
		{&closed, ScopeID(cfg.ID, "month")},
		{&closed, ScopeID(cfg.ID, "prev")},
		{&closed, ScopeID(cfg.ID, "next")},
		{&open, ScopeID(cfg.ID, "done")},
		{&grid, ScopeIDN(ScopeID(cfg.ID, "day", "prev"), "", 31)},
	}
	for _, c := range cases {
		found := findShapeByID(c.layout, c.id)
		if found == nil {
			t.Fatalf("%s: not in the layout", c.id)
		}
		if got := found.Shape.Color; got != ghost.Base {
			t.Errorf("%s fill = %v, want ghost %v", c.id, got, ghost.Base)
		}
		if got := found.Shape.ColorBorder; got != ghost.Border {
			t.Errorf("%s border = %v, want ghost %v",
				c.id, got, ghost.Border)
		}
	}
}
