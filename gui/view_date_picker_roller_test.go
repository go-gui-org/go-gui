package gui

import (
	"testing"
	"time"
)

func TestDatePickerRollerLayout(t *testing.T) {
	w := &Window{}
	v := DatePickerRoller(DatePickerRollerCfg{
		ID:           "roller1",
		SelectedDate: time.Date(2025, 6, 15, 0, 0, 0, 0, time.Local),
	})
	layout := generateViewLayout(v, w)
	if layout.Shape.ID != "roller1" {
		t.Errorf("ID = %q", layout.Shape.ID)
	}
}

func TestRollerDefaults(t *testing.T) {
	cfg := DatePickerRollerCfg{}
	applyRollerDefaults(&cfg)
	if cfg.minYear != 1900 {
		t.Errorf("MinYear = %d", cfg.minYear)
	}
	if cfg.maxYear != 2100 {
		t.Errorf("MaxYear = %d", cfg.maxYear)
	}
	if cfg.itemHeight != 24 {
		t.Errorf("ItemHeight = %f", cfg.itemHeight)
	}
	if cfg.visibleItems != 3 {
		t.Errorf("VisibleItems = %d", cfg.visibleItems)
	}
	if !cfg.ColorBorderFocus.IsSet() {
		t.Error("ColorBorderFocus should have a default value")
	}
	if cfg.SelectedDate.IsZero() {
		t.Error("SelectedDate should default to now")
	}
}

func TestRollerDefaultsEvenVisible(t *testing.T) {
	cfg := DatePickerRollerCfg{visibleItems: 4}
	applyRollerDefaults(&cfg)
	if cfg.visibleItems != 5 {
		t.Errorf("VisibleItems = %d, want 5 (rounded up)", cfg.visibleItems)
	}
}

func TestRollerDisplayModes(t *testing.T) {
	w := &Window{}
	modes := []datePickerRollerDisplayMode{
		rollerDayMonthYear,
		rollerMonthDayYear,
		rollerMonthYear,
		rollerYearOnly,
	}
	for _, m := range modes {
		v := DatePickerRoller(DatePickerRollerCfg{
			ID:           "rm",
			SelectedDate: time.Date(2025, 3, 15, 0, 0, 0, 0, time.Local),
			displayMode:  m,
		})
		layout := generateViewLayout(v, w)
		if layout.Shape.ID != "rm" {
			t.Errorf("mode %d: ID = %q", m, layout.Shape.ID)
		}
	}
}

func TestRollerDayFormat(t *testing.T) {
	if rollerDayFormat(1) != "01" {
		t.Errorf("day 1 = %q", rollerDayFormat(1))
	}
	if rollerDayFormat(15) != "15" {
		t.Errorf("day 15 = %q", rollerDayFormat(15))
	}
}

func TestRollerYearFormat(t *testing.T) {
	if rollerYearFormat(2025) != "2025" {
		t.Errorf("year = %q", rollerYearFormat(2025))
	}
}

func TestRollerMonthFormatShort(t *testing.T) {
	fn := rollerMonthFormat(false)
	if fn(1) != ActiveLocale.MonthsShort[0] {
		t.Errorf("short month 1 = %q", fn(1))
	}
	if fn(12) != ActiveLocale.MonthsShort[11] {
		t.Errorf("short month 12 = %q", fn(12))
	}
}

func TestRollerMonthFormatLong(t *testing.T) {
	fn := rollerMonthFormat(true)
	if fn(1) != ActiveLocale.MonthsFull[0] {
		t.Errorf("long month 1 = %q", fn(1))
	}
}

func TestRollerMonthFormatOutOfRange(t *testing.T) {
	fn := rollerMonthFormat(false)
	if fn(0) != "" {
		t.Errorf("month 0 = %q", fn(0))
	}
	if fn(13) != "" {
		t.Errorf("month 13 = %q", fn(13))
	}
}

func TestRollerAdjustDay(t *testing.T) {
	sel := time.Date(2025, 3, 15, 0, 0, 0, 0, time.Local)
	var got time.Time
	onChange := func(d time.Time, ctx EventCtx) { got = d }
	w := &Window{}

	rollerAdjustDay(1, sel, 1900, 2100, onChange, w)
	if got.Day() != 16 {
		t.Errorf("day+1 = %d", got.Day())
	}

	rollerAdjustDay(-1, sel, 1900, 2100, onChange, w)
	if got.Day() != 14 {
		t.Errorf("day-1 = %d", got.Day())
	}
}

func TestRollerAdjustDayNilOnChange(_ *testing.T) {
	sel := time.Date(2025, 3, 15, 0, 0, 0, 0, time.Local)
	// Should not panic.
	rollerAdjustDay(1, sel, 1900, 2100, nil, &Window{})
}

// rollerDrumAdjust is the dispatcher every roller interaction goes
// through — clicks, scrolls, and the embedded date-picker roller.
func TestRollerDrumAdjustDay(t *testing.T) {
	sel := time.Date(2025, 3, 15, 0, 0, 0, 0, time.Local)
	var got time.Time
	onChange := func(d time.Time, ctx EventCtx) { got = d }
	w := &Window{}

	rollerDrumAdjust("day", 1, sel, 1900, 2100, onChange, w, false)
	if got.Day() != 16 {
		t.Errorf("day drum +1 = %d, want 16", got.Day())
	}
	// Month boundary: March 31 + 1 day → April 1.
	sel = time.Date(2025, 3, 31, 0, 0, 0, 0, time.Local)
	rollerDrumAdjust("day", 1, sel, 1900, 2100, onChange, w, false)
	if got.Month() != 4 || got.Day() != 1 {
		t.Errorf("day drum month rollover = %v, want Apr 1", got)
	}
}

func TestRollerDrumAdjustMonth(t *testing.T) {
	sel := time.Date(2025, 1, 15, 0, 0, 0, 0, time.Local)
	var got time.Time
	onChange := func(d time.Time, ctx EventCtx) { got = d }
	w := &Window{}

	rollerDrumAdjust("month", 1, sel, 1900, 2100, onChange, w, false)
	if got.Month() != 2 {
		t.Errorf("month drum +1 = %v, want February", got)
	}
	// December + 1 wraps to January next year.
	sel = time.Date(2025, 12, 10, 0, 0, 0, 0, time.Local)
	rollerDrumAdjust("month", 1, sel, 1900, 2100, onChange, w, false)
	if got.Month() != 1 || got.Year() != 2026 {
		t.Errorf("month drum year wrap = %v, want Jan 2026", got)
	}
	// Month-end clamping: Jan 31 + 1 month → Feb 28.
	sel = time.Date(2025, 1, 31, 0, 0, 0, 0, time.Local)
	rollerDrumAdjust("month", 1, sel, 1900, 2100, onChange, w, false)
	if got.Month() != 2 || got.Day() != 28 {
		t.Errorf("month drum clamp = %v, want Feb 28", got)
	}
}

func TestRollerDrumAdjustYear(t *testing.T) {
	sel := time.Date(2025, 3, 15, 0, 0, 0, 0, time.Local)
	var got time.Time
	onChange := func(d time.Time, ctx EventCtx) { got = d }
	w := &Window{}

	rollerDrumAdjust("year", 1, sel, 1900, 2100, onChange, w, false)
	if got.Year() != 2026 {
		t.Errorf("year drum +1 = %d, want 2026", got.Year())
	}
	// Out of bounds without wrap → no change.
	rollerDrumAdjust("year", 1, time.Date(2100, 3, 15, 0, 0, 0, 0, time.Local),
		1900, 2100, onChange, w, false)
	if got.Year() != 2026 {
		t.Errorf("year drum past max changed to %d, want unchanged", got.Year())
	}
	// Wrapping year: max + 1 wraps to min.
	rollerDrumAdjust("year", 1, time.Date(2100, 3, 15, 0, 0, 0, 0, time.Local),
		1900, 2100, onChange, w, true)
	if got.Year() != 1900 {
		t.Errorf("year drum wrap = %d, want 1900", got.Year())
	}
}

func TestRollerDrumAdjustUnknownDrumNoop(t *testing.T) {
	sel := time.Date(2025, 3, 15, 0, 0, 0, 0, time.Local)
	fired := false
	onChange := func(time.Time, EventCtx) { fired = true }
	rollerDrumAdjust("unknown", 1, sel, 1900, 2100, onChange, &Window{}, false)
	if fired {
		t.Error("unknown drum name must not fire onChange")
	}
}

func TestRollerDrumAdjustNilOnChange(_ *testing.T) {
	sel := time.Date(2025, 3, 15, 0, 0, 0, 0, time.Local)
	// All drum names, nil callback: must not panic.
	rollerDrumAdjust("day", 1, sel, 1900, 2100, nil, &Window{}, false)
	rollerDrumAdjust("month", 1, sel, 1900, 2100, nil, &Window{}, false)
	rollerDrumAdjust("year", 1, sel, 1900, 2100, nil, &Window{}, true)
	rollerDrumAdjust("year", 1, sel, 1900, 2100, nil, &Window{}, false)
}

func TestRollerAdjustMonth(t *testing.T) {
	sel := time.Date(2025, 1, 15, 0, 0, 0, 0, time.Local)
	var got time.Time
	onChange := func(d time.Time, ctx EventCtx) { got = d }
	w := &Window{}

	rollerAdjustMonth(1, sel, 1900, 2100, onChange, w)
	if got.Day() != 15 || got.Month() != 2 {
		t.Errorf("month+1 from Jan 15 = %v", got)
	}

	rollerAdjustMonth(-1, sel, 1900, 2100, onChange, w)
	if got.Day() != 15 || got.Month() != 12 || got.Year() != 2024 {
		t.Errorf("month-1 from Jan 15 = %v", got)
	}
}

func TestRollerAdjustYear(t *testing.T) {
	sel := time.Date(2024, 2, 29, 0, 0, 0, 0, time.Local) // leap day
	var got time.Time
	onChange := func(d time.Time, ctx EventCtx) { got = d }
	w := &Window{}

	rollerAdjustYear(1, sel, 1900, 2100, onChange, w, false)
	// 2024 leap → 2025 non-leap, Feb 29 clamped to 28.
	if got.Day() != 28 || got.Month() != 2 || got.Year() != 2025 {
		t.Errorf("year+1 from leap = %v", got)
	}
}

func TestRollerAdjustYearBounds(t *testing.T) {
	sel := time.Date(2100, 6, 1, 0, 0, 0, 0, time.Local)
	called := false
	onChange := func(_ time.Time, ctx EventCtx) { called = true }
	w := &Window{}

	rollerAdjustYear(1, sel, 1900, 2100, onChange, w, false)
	if called {
		t.Error("should not call onChange beyond maxYear")
	}

	sel = time.Date(1900, 6, 1, 0, 0, 0, 0, time.Local)
	called = false
	rollerAdjustYear(-1, sel, 1900, 2100, onChange, w, false)
	if called {
		t.Error("should not call onChange below minYear")
	}
}

func TestRollerDefaultsMinMaxSwap(t *testing.T) {
	cfg := DatePickerRollerCfg{minYear: 2100, maxYear: 1900}
	applyRollerDefaults(&cfg)
	if cfg.minYear != 1900 || cfg.maxYear != 2100 {
		t.Errorf("MinYear=%d MaxYear=%d, want 1900/2100",
			cfg.minYear, cfg.maxYear)
	}
}

func TestRollerAdjustYearWrap(t *testing.T) {
	var got time.Time
	onChange := func(d time.Time, ctx EventCtx) { got = d }
	w := &Window{}

	// Wrap forward past max.
	sel := time.Date(2030, 6, 15, 0, 0, 0, 0, time.Local)
	rollerAdjustYear(1, sel, 2020, 2030, onChange, w, true)
	if got.Year() != 2020 {
		t.Errorf("wrap forward: year = %d, want 2020", got.Year())
	}

	// Wrap backward past min.
	sel = time.Date(2020, 6, 15, 0, 0, 0, 0, time.Local)
	rollerAdjustYear(-1, sel, 2020, 2030, onChange, w, true)
	if got.Year() != 2030 {
		t.Errorf("wrap backward: year = %d, want 2030", got.Year())
	}
}

func TestRollerAdjustMonthDayClamping(t *testing.T) {
	// Jan 31 + 1 month → Feb 28 (non-leap).
	sel := time.Date(2025, 1, 31, 0, 0, 0, 0, time.Local)
	var got time.Time
	onChange := func(d time.Time, ctx EventCtx) { got = d }
	w := &Window{}

	rollerAdjustMonth(1, sel, 1900, 2100, onChange, w)
	if got.Month() != 2 || got.Day() != 28 {
		t.Errorf("Jan 31 +1mo = %v, want Feb 28", got)
	}

	// Jan 31 + 1 month in leap year → Feb 29.
	sel = time.Date(2024, 1, 31, 0, 0, 0, 0, time.Local)
	rollerAdjustMonth(1, sel, 1900, 2100, onChange, w)
	if got.Month() != 2 || got.Day() != 29 {
		t.Errorf("Jan 31 +1mo leap = %v, want Feb 29", got)
	}
}

func TestRollerOnKeyDown(t *testing.T) {
	var got time.Time
	onChange := func(d time.Time, ctx EventCtx) { got = d }
	w := &Window{}
	sel := time.Date(2025, 6, 15, 0, 0, 0, 0, time.Local)

	// Up arrow → day-1 in default mode.
	e := &Event{KeyCode: KeyUp, Modifiers: ModNone}
	rollerOnKeyDown(onChange, sel, 1900, 2100, e, w,
		rollerDayMonthYear, false)
	if got.Day() != 14 {
		t.Errorf("Up day = %d, want 14", got.Day())
	}
	if !e.IsHandled {
		t.Error("Up should set IsHandled")
	}

	// Shift+Up → year-1.
	e = &Event{KeyCode: KeyUp, Modifiers: ModShift}
	rollerOnKeyDown(onChange, sel, 1900, 2100, e, w,
		rollerDayMonthYear, false)
	if got.Year() != 2024 {
		t.Errorf("Shift+Up year = %d, want 2024", got.Year())
	}

	// Alt+Down → month+1.
	e = &Event{KeyCode: KeyDown, Modifiers: ModAlt}
	rollerOnKeyDown(onChange, sel, 1900, 2100, e, w,
		rollerDayMonthYear, false)
	if got.Month() != 7 {
		t.Errorf("Alt+Down month = %d, want 7", got.Month())
	}

	// Up in YearOnly mode → year-1.
	e = &Event{KeyCode: KeyUp, Modifiers: ModNone}
	rollerOnKeyDown(onChange, sel, 1900, 2100, e, w,
		rollerYearOnly, false)
	if got.Year() != 2024 {
		t.Errorf("Up YearOnly = %d, want 2024", got.Year())
	}

	// Up in MonthYear mode → month-1.
	e = &Event{KeyCode: KeyUp, Modifiers: ModNone}
	rollerOnKeyDown(onChange, sel, 1900, 2100, e, w,
		rollerMonthYear, false)
	if got.Month() != 5 {
		t.Errorf("Up MonthYear = %d, want 5", got.Month())
	}
}

func TestRollerDisplayModeDrumCount(t *testing.T) {
	sel := time.Date(2025, 6, 15, 0, 0, 0, 0, time.Local)
	cfg := DatePickerRollerCfg{minYear: 1900, maxYear: 2100}
	applyRollerDefaults(&cfg)

	tests := []struct {
		mode datePickerRollerDisplayMode
		want int
	}{
		{rollerDayMonthYear, 3},
		{rollerMonthDayYear, 3},
		{rollerMonthYear, 2},
		{rollerYearOnly, 1},
	}
	for _, tt := range tests {
		cfg.displayMode = tt.mode
		specs := rollerDrumSpecs(&cfg, sel)
		if len(specs) != tt.want {
			t.Errorf("mode %d: drums = %d, want %d",
				tt.mode, len(specs), tt.want)
		}
	}
}

func TestWrapRange(t *testing.T) {
	if v := wrapRange(13, 1, 12); v != 1 {
		t.Errorf("wrapRange(13,1,12) = %d, want 1", v)
	}
	if v := wrapRange(0, 1, 12); v != 12 {
		t.Errorf("wrapRange(0,1,12) = %d, want 12", v)
	}
	if v := wrapRange(6, 1, 12); v != 6 {
		t.Errorf("wrapRange(6,1,12) = %d, want 6", v)
	}
}
