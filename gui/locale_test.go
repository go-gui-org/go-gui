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
	if l.StrOK != "OK" {
		t.Fatalf("StrOK = %q, want OK", l.StrOK)
	}
	if l.WeekdaysFull[0] != "Sunday" {
		t.Fatalf("WeekdaysFull[0] = %q, want Sunday", l.WeekdaysFull[0])
	}
	if l.MonthsFull[11] != "December" {
		t.Fatalf("MonthsFull[11] = %q, want December", l.MonthsFull[11])
	}
}

func TestLocaleToNumericLocale(t *testing.T) {
	l := LocaleDeDE
	nc := l.ToNumericLocale()
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

	ActiveLocale = LocaleArSA
	s := &Shape{TextDir: TextDirAuto}
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

	SetLocale(LocaleDeDE)
	cur := CurrentLocale()
	if cur.ID != "de-DE" {
		t.Fatalf("CurrentLocale().ID = %q, want de-DE", cur.ID)
	}
}

func TestLocalePresets(t *testing.T) {
	if LocaleEnUS.ID != "en-US" {
		t.Fatalf("LocaleEnUS.ID = %q", LocaleEnUS.ID)
	}
	if LocaleDeDE.ID != "de-DE" {
		t.Fatalf("LocaleDeDE.ID = %q", LocaleDeDE.ID)
	}
	if LocaleArSA.ID != "ar-SA" {
		t.Fatalf("LocaleArSA.ID = %q", LocaleArSA.ID)
	}
	if LocaleArSA.TextDir != TextDirRTL {
		t.Fatal("ar-SA should be RTL")
	}
	if LocaleDeDE.Currency.Symbol != "\u20AC" {
		t.Fatalf("de-DE symbol = %q, want \u20AC", LocaleDeDE.Currency.Symbol)
	}
	if LocaleDeDE.Date.FirstDayOfWeek != 1 {
		t.Fatalf("de-DE FirstDayOfWeek = %d, want 1",
			LocaleDeDE.Date.FirstDayOfWeek)
	}
}
