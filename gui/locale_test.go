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
	nc := l.toNumericLocale()
	if nc.DecimalSep != ',' {
		t.Fatalf("DecimalSep = %c, want ','", nc.DecimalSep)
	}
	if nc.GroupSep != '.' {
		t.Fatalf("GroupSep = %c, want '.'", nc.GroupSep)
	}
}

func TestEffectiveTextDir(t *testing.T) {
	old := CurrentLocale()
	defer func() { SetLocale(old) }()

	SetLocale(LocaleArSA)
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
	old := CurrentLocale()
	defer func() { SetLocale(old) }()

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

// --- public locale-switching API ---

func TestWindowSetLocale(t *testing.T) {
	old := CurrentLocale()
	defer func() { SetLocale(old) }()

	w := &Window{}
	w.SetLocale(LocaleDeDE)
	if CurrentLocale().ID != "de-DE" {
		t.Fatalf("CurrentLocale().ID = %q, want de-DE", CurrentLocale().ID)
	}
	// The locale swap must have requested a window refresh.
	if !w.refreshLayout.Load() {
		t.Fatal("SetLocale should mark the window for a layout refresh")
	}
}

func TestWindowSetLocaleIDKnown(t *testing.T) {
	old := CurrentLocale()
	defer func() { SetLocale(old) }()

	// Use the built-in registry entry; do not mutate the registry.
	w := &Window{}
	if err := w.SetLocaleID("de-DE"); err != nil {
		t.Fatalf("SetLocaleID(de-DE) error = %v", err)
	}
	if CurrentLocale().ID != "de-DE" {
		t.Fatalf("CurrentLocale().ID = %q, want de-DE", CurrentLocale().ID)
	}
	if !w.refreshLayout.Load() {
		t.Fatal("SetLocaleID should mark the window for a layout refresh")
	}
}

func TestWindowSetLocaleIDUnknown(t *testing.T) {
	old := CurrentLocale()
	defer func() { SetLocale(old) }()

	w := &Window{}
	err := w.SetLocaleID("xx-XX")
	if err == nil {
		t.Fatal("SetLocaleID(unknown) must return an error")
	}
	if CurrentLocale().ID != old.ID {
		t.Fatalf("CurrentLocale() changed to %q on a failed lookup",
			CurrentLocale().ID)
	}
	if w.refreshLayout.Load() {
		t.Fatal("failed lookup must not request a window refresh")
	}
}

func TestLocaleCloneIsolation(t *testing.T) {
	old := CurrentLocale()
	defer func() { SetLocale(old) }()

	// Mutating a LocaleGet result must not corrupt the registry.
	first, ok := LocaleGet("de-DE")
	if !ok {
		t.Fatal("de-DE not registered")
	}
	first.StrOK = "MUTATED"
	first.Translations = map[string]string{"k": "v"}
	first.Number.GroupSizes[0] = 99
	second, ok := LocaleGet("de-DE")
	if !ok {
		t.Fatal("de-DE not registered")
	}
	if second.StrOK == "MUTATED" {
		t.Error("LocaleGet shares string state with the registry")
	}
	if _, found := second.Translations["k"]; found {
		t.Error("LocaleGet shares the translations map with the registry")
	}
	if second.Number.GroupSizes[0] == 99 {
		t.Error("LocaleGet shares GroupSizes with the registry")
	}

	// Mutating CurrentLocale's result must not corrupt the global.
	cur := CurrentLocale()
	cur.StrCancel = "MUTATED"
	if CurrentLocale().StrCancel == "MUTATED" {
		t.Error("CurrentLocale shares state with the global")
	}

	// SetLocale clones on the way in.
	src := LocaleEnUS
	SetLocale(src)
	src.StrOK = "MUTATED"
	if CurrentLocale().StrOK == "MUTATED" {
		t.Error("SetLocale aliases the caller's struct")
	}
}

func TestLocaleConcurrentAccess(t *testing.T) {
	old := CurrentLocale()
	defer func() { SetLocale(old) }()

	done := make(chan bool)
	for range 4 {
		go func() {
			for range 25 {
				SetLocale(LocaleDeDE)
				_ = CurrentLocale()
				_ = LocaleT("missing")
				_ = LocaleRegisteredNames()
			}
			done <- true
		}()
	}
	for range 4 {
		<-done
	}
}

// A hand-built global locale may carry TextDir Auto; readers resolve
// it to LTR so no caller handles a third case.
func TestActiveTextDirAutoResolvesLTR(t *testing.T) {
	saved := CurrentLocale()
	t.Cleanup(func() { SetLocale(saved) })
	SetLocale(Locale{TextDir: textDirAuto})
	if got := effectiveTextDir(&Shape{}); got != TextDirLTR {
		t.Fatalf("effectiveTextDir = %d, want LTR", got)
	}
}
