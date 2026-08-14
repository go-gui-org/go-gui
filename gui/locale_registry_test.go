package gui

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestLocaleRegistryInit(t *testing.T) {
	names := localeRegisteredNames()
	want := []string{
		"ar-SA", "de-DE", "en-US", "es-ES", "fr-FR",
		"he-IL", "ja-JP", "ko-KR", "pt-BR", "zh-CN",
	}
	for _, id := range want {
		if !slices.Contains(names, id) {
			t.Fatalf("missing registered locale: %s (have %v)",
				id, names)
		}
	}
}

func TestLocaleGetKnown(t *testing.T) {
	l, ok := LocaleGet("de-DE")
	if !ok {
		t.Fatal("LocaleGet(de-DE) returned false")
	}
	if l.ID != "de-DE" {
		t.Fatalf("ID = %q", l.ID)
	}
}

func TestLocaleGetUnknown(t *testing.T) {
	_, ok := LocaleGet("xx-XX")
	if ok {
		t.Fatal("LocaleGet(xx-XX) should return false")
	}
}

func TestLocaleRegisterOverwrite(t *testing.T) {
	custom := localeDefaults()
	custom.ID = "test-overwrite"
	custom.strOK = "first"
	LocaleRegister(custom)

	custom.strOK = "second"
	LocaleRegister(custom)

	l, ok := LocaleGet("test-overwrite")
	if !ok {
		t.Fatal("not found")
	}
	if l.strOK != "second" {
		t.Fatalf("StrOK = %q, want second", l.strOK)
	}
}

func TestLocaleRegisteredNamesSorted(t *testing.T) {
	names := localeRegisteredNames()
	for i := 1; i < len(names); i++ {
		if names[i] < names[i-1] {
			t.Fatalf("not sorted: %v", names)
		}
	}
}

func TestLocalePresetFields(t *testing.T) {
	tests := []struct {
		id      string
		dir     textDirection
		decSep  rune
		curCode string
	}{
		{"en-US", TextDirLTR, '.', "USD"},
		{"de-DE", textDirAuto, ',', "EUR"},
		{"ar-SA", TextDirRTL, '.', "SAR"},
		{"fr-FR", textDirAuto, ',', "EUR"},
		{"es-ES", textDirAuto, ',', "EUR"},
		{"pt-BR", textDirAuto, ',', "BRL"},
		{"ja-JP", textDirAuto, '.', "JPY"},
		{"zh-CN", textDirAuto, '.', "CNY"},
		{"ko-KR", textDirAuto, '.', "KRW"},
		{"he-IL", TextDirRTL, '.', "ILS"},
	}
	for _, tt := range tests {
		l, ok := LocaleGet(tt.id)
		if !ok {
			t.Errorf("%s: not registered", tt.id)
			continue
		}
		if l.TextDir != tt.dir {
			t.Errorf("%s: TextDir = %v, want %v",
				tt.id, l.TextDir, tt.dir)
		}
		if l.Number.DecimalSep != tt.decSep {
			t.Errorf("%s: DecimalSep = %c, want %c",
				tt.id, l.Number.DecimalSep, tt.decSep)
		}
		if l.Currency.Code != tt.curCode {
			t.Errorf("%s: Currency.Code = %s, want %s",
				tt.id, l.Currency.Code, tt.curCode)
		}
		if l.strOK == "" {
			t.Errorf("%s: StrOK empty", tt.id)
		}
		if l.StrCancel == "" {
			t.Errorf("%s: StrCancel empty", tt.id)
		}
		if l.WeekdaysFull[0] == "" {
			t.Errorf("%s: WeekdaysFull[0] empty", tt.id)
		}
		if l.MonthsFull[0] == "" {
			t.Errorf("%s: MonthsFull[0] empty", tt.id)
		}
	}
}

// --- LocaleLoadDir ---

const testLocaleDirJSON = `{
  "id": "test-dir-locale",
  "number": {"decimal_sep": ",", "group_sep": "."},
  "date": {"first_day_of_week": 1},
  "currency": {"symbol": "T", "code": "TST", "position": "suffix"},
  "strings": {"ok": "Ja", "cancel": "Nein"}
}`

func writeLocaleJSON(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name),
		[]byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

func TestLocaleLoadDirRegistersAll(t *testing.T) {
	dir := t.TempDir()
	writeLocaleJSON(t, dir, "aa.json",
		`{"id": "test-aa", "strings": {"ok": "A"}}`)
	writeLocaleJSON(t, dir, "bb.json",
		`{"id": "test-bb", "strings": {"ok": "B"}}`)

	if err := LocaleLoadDir(dir); err != nil {
		t.Fatalf("LocaleLoadDir() error = %v", err)
	}
	if _, ok := LocaleGet("test-aa"); !ok {
		t.Error("test-aa not registered after LocaleLoadDir")
	}
	if _, ok := LocaleGet("test-bb"); !ok {
		t.Error("test-bb not registered after LocaleLoadDir")
	}
}

func TestLocaleLoadDirBadJSON(t *testing.T) {
	dir := t.TempDir()
	// Glob order is sorted, so name the good file first: it must be
	// registered before the bad file aborts the directory load.
	writeLocaleJSON(t, dir, "a-good.json",
		`{"id": "test-good", "strings": {"ok": "G"}}`)
	writeLocaleJSON(t, dir, "z-bad.json", `{"id": "test-bad",`)

	err := LocaleLoadDir(dir)
	if err == nil {
		t.Fatal("LocaleLoadDir with a bad bundle must return an error")
	}
	// The good file must still have been registered.
	if _, ok := LocaleGet("test-good"); !ok {
		t.Error("a-good.json should be registered before the failure")
	}
	// The bad file's locale must not be registered.
	if _, ok := LocaleGet("test-bad"); ok {
		t.Error("z-bad.json must not be registered")
	}
}

func TestLocaleLoadDirEmptyDir(t *testing.T) {
	if err := LocaleLoadDir(t.TempDir()); err != nil {
		t.Fatalf("LocaleLoadDir(empty) error = %v", err)
	}
}

func TestLocaleLoadDirLoadsFields(t *testing.T) {
	dir := t.TempDir()
	writeLocaleJSON(t, dir, "tl.json", testLocaleDirJSON)

	if err := LocaleLoadDir(dir); err != nil {
		t.Fatalf("LocaleLoadDir() error = %v", err)
	}
	l, ok := LocaleGet("test-dir-locale")
	if !ok {
		t.Fatal("test-dir-locale not registered")
	}
	if l.ID != "test-dir-locale" {
		t.Fatalf("ID = %q", l.ID)
	}
	if l.Number.DecimalSep != ',' || l.Number.GroupSep != '.' {
		t.Fatalf("number = %c/%c, want ,/.", l.Number.DecimalSep, l.Number.GroupSep)
	}
	if l.Date.FirstDayOfWeek != 1 {
		t.Fatalf("FirstDayOfWeek = %d, want 1", l.Date.FirstDayOfWeek)
	}
	if l.Currency.Code != "TST" || l.Currency.Symbol != "T" {
		t.Fatalf("currency = %s/%s, want TST/T",
			l.Currency.Code, l.Currency.Symbol)
	}
	if l.strOK != "Ja" || l.StrCancel != "Nein" {
		t.Fatalf("strings = %q/%q, want Ja/Nein", l.strOK, l.StrCancel)
	}
}
