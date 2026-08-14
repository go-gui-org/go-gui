package gui

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLocaleParseFull(t *testing.T) {
	json := `{
		"id": "fr-FR",
		"text_dir": "ltr",
		"number": {
			"decimal_sep": ",",
			"group_sep": " ",
			"group_sizes": [3],
			"minus_sign": "-",
			"plus_sign": "+"
		},
		"date": {
			"short_date": "DD/MM/YYYY",
			"long_date": "D MMMM YYYY",
			"first_day_of_week": 1,
			"use_24h": true
		},
		"currency": {
			"symbol": "€",
			"code": "EUR",
			"position": "suffix",
			"spacing": true,
			"decimals": 2
		},
		"strings": {
			"ok": "D'accord",
			"yes": "Oui",
			"no": "Non",
			"cancel": "Annuler"
		},
		"weekdays_short": ["D","L","M","M","J","V","S"],
		"months_full": [
			"janvier","février","mars","avril","mai","juin",
			"juillet","août","septembre","octobre","novembre",
			"décembre"
		],
		"translations": {"hello": "bonjour"}
	}`
	l, err := LocaleParse(json)
	if err != nil {
		t.Fatalf("LocaleParse: %v", err)
	}
	if l.ID != "fr-FR" {
		t.Fatalf("ID = %q", l.ID)
	}
	if l.Number.DecimalSep != ',' {
		t.Fatalf("DecimalSep = %c", l.Number.DecimalSep)
	}
	if l.Number.GroupSep != ' ' {
		t.Fatalf("GroupSep = %c", l.Number.GroupSep)
	}
	if l.Date.ShortDate != "DD/MM/YYYY" {
		t.Fatalf("ShortDate = %q", l.Date.ShortDate)
	}
	if !l.Date.Use24H {
		t.Fatal("Use24H should be true")
	}
	if l.Date.FirstDayOfWeek != 1 {
		t.Fatalf("FirstDayOfWeek = %d", l.Date.FirstDayOfWeek)
	}
	if l.Currency.Position != affixSuffix {
		t.Fatalf("Position = %d", l.Currency.Position)
	}
	if l.strOK != "D'accord" {
		t.Fatalf("StrOK = %q", l.strOK)
	}
	if l.strYes != "Oui" {
		t.Fatalf("StrYes = %q", l.strYes)
	}
	if l.WeekdaysShort[0] != "D" {
		t.Fatalf("WeekdaysShort[0] = %q", l.WeekdaysShort[0])
	}
	if l.MonthsFull[0] != "janvier" {
		t.Fatalf("MonthsFull[0] = %q", l.MonthsFull[0])
	}
	if l.Translations["hello"] != "bonjour" {
		t.Fatalf("translations[hello] = %q",
			l.Translations["hello"])
	}
}

func TestLocaleParseMinimal(t *testing.T) {
	l, err := LocaleParse(`{}`)
	if err != nil {
		t.Fatalf("LocaleParse: %v", err)
	}
	// Falls back to en-US defaults.
	if l.ID != "en-US" {
		t.Fatalf("ID = %q, want en-US", l.ID)
	}
	if l.Number.DecimalSep != '.' {
		t.Fatalf("DecimalSep = %c, want '.'", l.Number.DecimalSep)
	}
	if l.StrCancel != "Cancel" {
		t.Fatalf("StrCancel = %q", l.StrCancel)
	}
	if l.WeekdaysFull[0] != "Sunday" {
		t.Fatalf("WeekdaysFull[0] = %q", l.WeekdaysFull[0])
	}
}

func TestLocaleParsePartialNumber(t *testing.T) {
	l, err := LocaleParse(`{"number":{"decimal_sep":";"}}`)
	if err != nil {
		t.Fatalf("LocaleParse: %v", err)
	}
	if l.Number.DecimalSep != ';' {
		t.Fatalf("DecimalSep = %c, want ';'", l.Number.DecimalSep)
	}
	// GroupSep falls back to default.
	if l.Number.GroupSep != ',' {
		t.Fatalf("GroupSep = %c, want ','", l.Number.GroupSep)
	}
}

func TestLocaleParseInvalidJSON(t *testing.T) {
	_, err := LocaleParse(`{bad}`)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestLocaleParseTextDirRTL(t *testing.T) {
	l, err := LocaleParse(`{"text_dir":"rtl"}`)
	if err != nil {
		t.Fatalf("LocaleParse: %v", err)
	}
	if l.TextDir != TextDirRTL {
		t.Fatalf("TextDir = %d, want RTL", l.TextDir)
	}
}

func TestFirstRune(t *testing.T) {
	tests := []struct {
		input    string
		fallback rune
		want     rune
	}{
		{".", 'x', '.'},
		{"", 'x', 'x'},
		{"€", 'x', '€'},
		{"\U0001F600", 'x', '\U0001F600'}, // 4-byte emoji
	}
	for _, tt := range tests {
		got := firstRune(tt.input, tt.fallback)
		if got != tt.want {
			t.Errorf("firstRune(%q, %c) = %c, want %c",
				tt.input, tt.fallback, got, tt.want)
		}
	}
}

func TestLocaleParseWeekdaysWrongLength(t *testing.T) {
	l, err := LocaleParse(`{"weekdays_short":["a","b"]}`)
	if err != nil {
		t.Fatalf("LocaleParse: %v", err)
	}
	// Falls back to en-US defaults.
	if l.WeekdaysShort[0] != "S" {
		t.Fatalf("WeekdaysShort[0] = %q, want S",
			l.WeekdaysShort[0])
	}
}

func TestLocaleParseDateFirstDayZero(t *testing.T) {
	l, err := LocaleParse(`{"date":{"first_day_of_week":0}}`)
	if err != nil {
		t.Fatalf("LocaleParse: %v", err)
	}
	if l.Date.FirstDayOfWeek != 0 {
		t.Fatalf("FirstDayOfWeek = %d, want 0",
			l.Date.FirstDayOfWeek)
	}
}

func TestLocaleParseCurrencyDecimals(t *testing.T) {
	l, err := LocaleParse(`{"currency":{"decimals":0}}`)
	if err != nil {
		t.Fatalf("LocaleParse: %v", err)
	}
	if l.Currency.Decimals != 0 {
		t.Fatalf("Decimals = %d, want 0", l.Currency.Decimals)
	}
}

// --- LocaleLoad (file-based) ---

func TestLocaleLoadFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "fr.json")
	content := `{
		"id": "fr-FR",
		"number": {"decimal_sep": ",", "group_sep": " "},
		"date": {"short_date": "DD/MM/YYYY", "first_day_of_week": 1},
		"currency": {"symbol": "€", "code": "EUR", "position": "suffix"},
		"strings": {"ok": "D'accord", "cancel": "Annuler"}
	}`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	l, err := LocaleLoad(path)
	if err != nil {
		t.Fatalf("LocaleLoad() error = %v", err)
	}
	if l.ID != "fr-FR" {
		t.Fatalf("ID = %q, want fr-FR", l.ID)
	}
	if l.Number.DecimalSep != ',' {
		t.Fatalf("DecimalSep = %c, want ','", l.Number.DecimalSep)
	}
	if l.Date.ShortDate != "DD/MM/YYYY" {
		t.Fatalf("ShortDate = %q", l.Date.ShortDate)
	}
	if l.Currency.Code != "EUR" || l.Currency.Symbol != "\u20AC" {
		t.Fatalf("currency = %s/%s, want EUR/€",
			l.Currency.Code, l.Currency.Symbol)
	}
	if l.strOK != "D'accord" || l.StrCancel != "Annuler" {
		t.Fatalf("strings = %q/%q", l.strOK, l.StrCancel)
	}
}

func TestLocaleLoadMissingFile(t *testing.T) {
	if _, err := LocaleLoad(filepath.Join(t.TempDir(), "nope.json")); err == nil {
		t.Fatal("LocaleLoad(missing) must return an error")
	}
}

func TestLocaleLoadInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(path, []byte(`{"id": "x",`), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := LocaleLoad(path); err == nil {
		t.Fatal("LocaleLoad(invalid JSON) must return an error")
	}
}
