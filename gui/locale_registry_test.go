package gui

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestLocaleRegistryInit(t *testing.T) {
	names := LocaleRegisteredNames()
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
	custom.StrOK = "first"
	LocaleRegister(custom)

	custom.StrOK = "second"
	LocaleRegister(custom)

	l, ok := LocaleGet("test-overwrite")
	if !ok {
		t.Fatal("not found")
	}
	if l.StrOK != "second" {
		t.Fatalf("StrOK = %q, want second", l.StrOK)
	}
}

func TestLocaleRegisteredNamesSorted(t *testing.T) {
	names := LocaleRegisteredNames()
	for i := 1; i < len(names); i++ {
		if names[i] < names[i-1] {
			t.Fatalf("not sorted: %v", names)
		}
	}
}

func TestLocalePresetFields(t *testing.T) {
	tests := []struct {
		id      string
		dir     TextDirection
		decSep  rune
		curCode string
	}{
		{"en-US", TextDirLTR, '.', "USD"},
		{"de-DE", TextDirLTR, ',', "EUR"},
		{"ar-SA", TextDirRTL, '.', "SAR"},
		{"fr-FR", TextDirLTR, ',', "EUR"},
		{"es-ES", TextDirLTR, ',', "EUR"},
		{"pt-BR", TextDirLTR, ',', "BRL"},
		{"ja-JP", TextDirLTR, '.', "JPY"},
		{"zh-CN", TextDirLTR, '.', "CNY"},
		{"ko-KR", TextDirLTR, '.', "KRW"},
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
		if l.StrOK == "" {
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
	writeLocaleJSON(t, dir, "a-good.json",
		`{"id": "test-good", "strings": {"ok": "G"}}`)
	writeLocaleJSON(t, dir, "z-bad.json", `{"id": "test-bad",`)

	err := LocaleLoadDir(dir)
	if err == nil {
		t.Fatal("LocaleLoadDir with a bad bundle must return an error")
	}
	// Loading is two-phase: the good file must NOT be registered
	// when a sibling fails.
	if _, ok := LocaleGet("test-good"); ok {
		t.Error("a-good.json must not be registered after the failure")
	}
	// The bad file's locale must not be registered.
	if _, ok := LocaleGet("test-bad"); ok {
		t.Error("z-bad.json must not be registered")
	}
}

func TestLocaleLoadDirMissingDir(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "no-such-dir")
	if err := LocaleLoadDir(missing); err == nil {
		t.Fatal("LocaleLoadDir(missing) must return an error")
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
	if l.StrOK != "Ja" || l.StrCancel != "Nein" {
		t.Fatalf("strings = %q/%q, want Ja/Nein", l.StrOK, l.StrCancel)
	}
}

// TestLocaleLoadDirGlobMetaInPath checks a directory whose name holds
// glob metacharacters still loads: the path is data, not a pattern.
func TestLocaleLoadDirGlobMetaInPath(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "loc[1]")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	writeLocaleJSON(t, dir, "m.json", `{"id": "test-glob-meta"}`)
	if err := LocaleLoadDir(dir); err != nil {
		t.Fatalf("LocaleLoadDir() error = %v", err)
	}
	if _, ok := LocaleGet("test-glob-meta"); !ok {
		t.Fatal("test-glob-meta not registered")
	}
}

func TestLocaleLoadDirNotADirectory(t *testing.T) {
	dir := t.TempDir()
	writeLocaleJSON(t, dir, "file.json", `{"id": "test-not-dir"}`)
	if err := LocaleLoadDir(filepath.Join(dir, "file.json")); err == nil {
		t.Fatal("LocaleLoadDir(file) must return an error")
	}
}
