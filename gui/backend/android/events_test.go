//go:build android

package android

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestIMESanitizeText(t *testing.T) {
	if got := imeSanitizeText("a�b"); got != "ab" {
		t.Errorf("imeSanitizeText stripped to %q, want %q", got, "ab")
	}
	if got := imeSanitizeText(""); got != "" {
		t.Errorf("imeSanitizeText(\"\") = %q, want empty", got)
	}
	if got := imeSanitizeText("�"); got != "" {
		t.Errorf("imeSanitizeText lone failure = %q, want empty", got)
	}
	big := strings.Repeat("あ", maxIMETextRunes+10)
	if got := imeSanitizeText(big); utf8.RuneCountInString(got) != maxIMETextRunes {
		t.Errorf("sanitized runes = %d, want %d",
			utf8.RuneCountInString(got), maxIMETextRunes)
	}
}
