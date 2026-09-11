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

// TestDatePickerRollerScopedFocusAndKeys is the regression test for
// issue #565: under an ID-bearing parent the roller resolved no
// effective ID, so click-to-focus parked focus in the void and key
// dispatch never reached it.
func TestDatePickerRollerScopedFocusAndKeys(t *testing.T) {
	w := &Window{}
	// EventFn drops key events on an unfocused window (eventAllowed).
	w.focused = true
	sel := time.Date(2025, 6, 15, 0, 0, 0, 0, time.Local)
	changes := 0
	v := Column(ContainerCfg{
		ID: "panel",
		Content: []View{
			DatePickerRoller(DatePickerRollerCfg{
				ID:           "roller",
				Focusable:    true,
				SelectedDate: sel,
				OnChange: func(_ time.Time, _ EventCtx) {
					changes++
				},
			}),
		},
	})
	w.layout = generateViewLayout(v, w)
	roller := &w.layout.Children[0]
	if roller.Shape.events.OnClick == nil {
		t.Fatal("OnClick handler missing")
	}
	roller.Shape.events.OnClick(EventCtx{roller, &Event{}, w})
	if w.FocusID() != "panel:roller" {
		t.Fatalf("focus = %q, want panel:roller", w.FocusID())
	}
	e := &Event{Type: EventKeyDown, KeyCode: KeyDown, Modifiers: ModNone}
	w.EventFn(e)
	if changes != 1 {
		t.Errorf("OnChange fired %d times, want 1", changes)
	}
	if !e.IsHandled {
		t.Error("KeyDown should be marked handled")
	}
}

// TestDatePickerRollerMouseScroll covers the wheel path: the scroll
// handler must hit-test the drum under the cursor. Dispatch hands it
// shape-relative coordinates while the drums carry absolute ones, so
// without the translate-back the wheel silently does nothing.
func TestDatePickerRollerMouseScroll(t *testing.T) {
	w := &Window{}
	// A sized window: the pipeline seeds hit-test clips from it.
	w.windowWidth = 800
	w.windowHeight = 600
	sel := time.Date(2025, 6, 15, 0, 0, 0, 0, time.Local)
	var got time.Time
	fired := 0
	v := Column(ContainerCfg{
		ID: "panel",
		Content: []View{
			// Push the roller away from the origin: near (0,0)
			// shape-relative and absolute coordinates coincide,
			// which would let a missing translate-back pass.
			Column(ContainerCfg{Height: 300}),
			DatePickerRoller(DatePickerRollerCfg{
				ID:           "roller",
				Focusable:    true,
				SelectedDate: sel,
				OnChange: func(d time.Time, _ EventCtx) {
					fired++
					got = d
				},
			}),
		},
	})
	w.layout = generateViewLayout(v, w)
	// layoutArrange runs AmendLayout, which installs OnMouseScroll;
	// clips are seeded from the window rect as layoutPipeline does.
	_ = layoutArrange(&w.layout, w)
	layoutSetShapeClips(&w.layout, w.windowRect())
	roller := &w.layout.Children[1]
	if roller.Shape.events.OnMouseScroll == nil {
		t.Fatal("OnMouseScroll missing: AmendLayout did not install it")
	}
	// Middle drum in day-month-year order is the month drum.
	if len(roller.Children) < 3 {
		t.Fatalf("drums = %d, want 3", len(roller.Children))
	}
	drum := &roller.Children[1]
	mx := drum.Shape.X + drum.Shape.Width/2
	my := drum.Shape.Y + drum.Shape.Height/2
	e := &Event{Type: EventMouseScroll, MouseX: mx, MouseY: my, ScrollY: -1}
	w.EventFn(e)
	if fired != 1 {
		t.Fatalf("OnChange fired %d times, want 1", fired)
	}
	if got.Month() != 7 {
		t.Errorf("month = %v, want July", got.Month())
	}
	if !e.IsHandled {
		t.Error("scroll should be marked handled")
	}
}

func TestRollerDefaults(t *testing.T) {
	cfg := DatePickerRollerCfg{}
	applyRollerDefaults(&cfg)
	if cfg.MinYear != 1900 {
		t.Errorf("MinYear = %d", cfg.MinYear)
	}
	if cfg.MaxYear != 2100 {
		t.Errorf("MaxYear = %d", cfg.MaxYear)
	}
	if cfg.ItemHeight != 24 {
		t.Errorf("ItemHeight = %f", cfg.ItemHeight)
	}
	if cfg.VisibleItems != 3 {
		t.Errorf("VisibleItems = %d", cfg.VisibleItems)
	}
	if !cfg.ColorBorderFocus.IsSet() {
		t.Error("ColorBorderFocus should have a default value")
	}
	if cfg.SelectedDate.IsZero() {
		t.Error("SelectedDate should default to now")
	}
}

func TestRollerDefaultsEvenVisible(t *testing.T) {
	cfg := DatePickerRollerCfg{VisibleItems: 4}
	applyRollerDefaults(&cfg)
	if cfg.VisibleItems != 5 {
		t.Errorf("VisibleItems = %d, want 5 (rounded up)", cfg.VisibleItems)
	}
}

func TestRollerDisplayModes(t *testing.T) {
	w := &Window{}
	modes := []DatePickerRollerDisplayMode{
		RollerDayMonthYear,
		RollerMonthDayYear,
		RollerMonthYear,
		RollerYearOnly,
	}
	for _, m := range modes {
		v := DatePickerRoller(DatePickerRollerCfg{
			ID:           "rm",
			SelectedDate: time.Date(2025, 3, 15, 0, 0, 0, 0, time.Local),
			DisplayMode:  m,
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
	cfg := DatePickerRollerCfg{MinYear: 2100, MaxYear: 1900}
	applyRollerDefaults(&cfg)
	if cfg.MinYear != 1900 || cfg.MaxYear != 2100 {
		t.Errorf("MinYear=%d MaxYear=%d, want 1900/2100",
			cfg.MinYear, cfg.MaxYear)
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
		RollerDayMonthYear, false)
	if got.Day() != 14 {
		t.Errorf("Up day = %d, want 14", got.Day())
	}
	if !e.IsHandled {
		t.Error("Up should set IsHandled")
	}

	// Shift+Up → year-1.
	e = &Event{KeyCode: KeyUp, Modifiers: ModShift}
	rollerOnKeyDown(onChange, sel, 1900, 2100, e, w,
		RollerDayMonthYear, false)
	if got.Year() != 2024 {
		t.Errorf("Shift+Up year = %d, want 2024", got.Year())
	}

	// Alt+Down → month+1.
	e = &Event{KeyCode: KeyDown, Modifiers: ModAlt}
	rollerOnKeyDown(onChange, sel, 1900, 2100, e, w,
		RollerDayMonthYear, false)
	if got.Month() != 7 {
		t.Errorf("Alt+Down month = %d, want 7", got.Month())
	}

	// Up in YearOnly mode → year-1.
	e = &Event{KeyCode: KeyUp, Modifiers: ModNone}
	rollerOnKeyDown(onChange, sel, 1900, 2100, e, w,
		RollerYearOnly, false)
	if got.Year() != 2024 {
		t.Errorf("Up YearOnly = %d, want 2024", got.Year())
	}

	// Up in MonthYear mode → month-1.
	e = &Event{KeyCode: KeyUp, Modifiers: ModNone}
	rollerOnKeyDown(onChange, sel, 1900, 2100, e, w,
		RollerMonthYear, false)
	if got.Month() != 5 {
		t.Errorf("Up MonthYear = %d, want 5", got.Month())
	}
}

func TestRollerDisplayModeDrumCount(t *testing.T) {
	sel := time.Date(2025, 6, 15, 0, 0, 0, 0, time.Local)
	cfg := DatePickerRollerCfg{MinYear: 1900, MaxYear: 2100}
	applyRollerDefaults(&cfg)

	tests := []struct {
		mode DatePickerRollerDisplayMode
		want int
	}{
		{RollerDayMonthYear, 3},
		{RollerMonthDayYear, 3},
		{RollerMonthYear, 2},
		{RollerYearOnly, 1},
	}
	for _, tt := range tests {
		cfg.DisplayMode = tt.mode
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

// The factory keys focus and state on cfg.ID, so an empty one is a
// programmer error caught at build time by the `gui:"required"` tag and
// at runtime here. The literal omits the ID on purpose, so it carries
// the directive that suppresses the analyzer for that one literal.
func TestDatePickerRollerRequiresID(t *testing.T) {
	assertPanicsRequiringID(t, "DatePickerRoller", func() {
		_ = DatePickerRoller(DatePickerRollerCfg{}) // requiredid:ignore
	})
}
