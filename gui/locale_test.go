package gui

import "testing"

func TestLocaleDefaults(t *testing.T) {
	l := localeDefaults()
	if l.ID != "en-US" {
		t.Fatalf("ID = %q, want en-US", l.ID)
	}
	if l.TextDir != TextDirLTR {
		t.Fatalf("TextDir = %d, want LTR", l.TextDir)
	}
	if l.Number.DecimalSep != '.' {
		t.Fatalf("DecimalSep = %c, want '.'", l.Number.DecimalSep)
	}
	if l.Number.GroupSep != ',' {
		t.Fatalf("GroupSep = %c, want ','", l.Number.GroupSep)
	}
	if len(l.Number.GroupSizes) != 1 || l.Number.GroupSizes[0] != 3 {
		t.Fatalf("GroupSizes = %v, want [3]", l.Number.GroupSizes)
	}
	if l.Currency.Symbol != "$" {
		t.Fatalf("Symbol = %q, want $", l.Currency.Symbol)
	}
	if l.Currency.Decimals != 2 {
		t.Fatalf("Decimals = %d, want 2", l.Currency.Decimals)
	}
	if l.strOK != "OK" {
		t.Fatalf("StrOK = %q, want OK", l.strOK)
	}
	if l.WeekdaysFull[0] != "Sunday" {
		t.Fatalf("WeekdaysFull[0] = %q, want Sunday", l.WeekdaysFull[0])
	}
	if l.MonthsFull[11] != "December" {
		t.Fatalf("MonthsFull[11] = %q, want December", l.MonthsFull[11])
	}
}

func TestLocaleToNumericLocale(t *testing.T) {
	l := localeDeDE
	nc := l.toNumericLocale()
	if nc.DecimalSep != ',' {
		t.Fatalf("DecimalSep = %c, want ','", nc.DecimalSep)
	}
	if nc.GroupSep != '.' {
		t.Fatalf("GroupSep = %c, want '.'", nc.GroupSep)
	}
}

func TestEffectiveTextDir(t *testing.T) {
	old := ActiveLocale
	defer func() { ActiveLocale = old }()

	ActiveLocale = localeArSA
	s := &Shape{TextDir: textDirAuto}
	if effectiveTextDir(s) != TextDirRTL {
		t.Fatal("auto should fall back to RTL locale")
	}
	s.TextDir = TextDirLTR
	if effectiveTextDir(s) != TextDirLTR {
		t.Fatal("explicit LTR should override locale")
	}
}

func TestSetLocaleAndGet(t *testing.T) {
	old := ActiveLocale
	defer func() { ActiveLocale = old }()

	setLocale(localeDeDE)
	cur := CurrentLocale()
	if cur.ID != "de-DE" {
		t.Fatalf("CurrentLocale().ID = %q, want de-DE", cur.ID)
	}
}

func TestLocalePresets(t *testing.T) {
	if localeEnUS.ID != "en-US" {
		t.Fatalf("LocaleEnUS.ID = %q", localeEnUS.ID)
	}
	if localeDeDE.ID != "de-DE" {
		t.Fatalf("LocaleDeDE.ID = %q", localeDeDE.ID)
	}
	if localeArSA.ID != "ar-SA" {
		t.Fatalf("LocaleArSA.ID = %q", localeArSA.ID)
	}
	if localeArSA.TextDir != TextDirRTL {
		t.Fatal("ar-SA should be RTL")
	}
	if localeDeDE.Currency.Symbol != "\u20AC" {
		t.Fatalf("de-DE symbol = %q, want \u20AC", localeDeDE.Currency.Symbol)
	}
	if localeDeDE.Date.FirstDayOfWeek != 1 {
		t.Fatalf("de-DE FirstDayOfWeek = %d, want 1",
			localeDeDE.Date.FirstDayOfWeek)
	}
}

// --- public locale-switching API ---

func TestWindowSetLocale(t *testing.T) {
	old := ActiveLocale
	defer func() { ActiveLocale = old }()

	w := &Window{}
	w.SetLocale(localeDeDE)
	if ActiveLocale.ID != "de-DE" {
		t.Fatalf("ActiveLocale.ID = %q, want de-DE", ActiveLocale.ID)
	}
	// The locale swap must have requested a window refresh.
	if !w.refreshLayout {
		t.Fatal("SetLocale should mark the window for a layout refresh")
	}
}

func TestWindowSetLocaleIDKnown(t *testing.T) {
	old := ActiveLocale
	defer func() { ActiveLocale = old }()

	// Use the built-in registry entry; do not mutate the registry.
	w := &Window{}
	if err := w.SetLocaleID("de-DE"); err != nil {
		t.Fatalf("SetLocaleID(de-DE) error = %v", err)
	}
	if ActiveLocale.ID != "de-DE" {
		t.Fatalf("ActiveLocale.ID = %q, want de-DE", ActiveLocale.ID)
	}
	if !w.refreshLayout {
		t.Fatal("SetLocaleID should mark the window for a layout refresh")
	}
}

func TestWindowSetLocaleIDUnknown(t *testing.T) {
	old := ActiveLocale
	defer func() { ActiveLocale = old }()

	w := &Window{}
	err := w.SetLocaleID("xx-XX")
	if err == nil {
		t.Fatal("SetLocaleID(unknown) must return an error")
	}
	if ActiveLocale.ID != old.ID {
		t.Fatalf("ActiveLocale changed to %q on a failed lookup",
			ActiveLocale.ID)
	}
	if w.refreshLayout {
		t.Fatal("failed lookup must not request a window refresh")
	}
}
