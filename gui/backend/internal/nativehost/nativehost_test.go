package nativehost

import (
	"strings"
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

func TestValidateOpenURI_Valid(t *testing.T) {
	valid := []string{
		"http://example.com",
		"https://example.com/path?q=1",
		"mailto:user@example.com",
	}
	for _, raw := range valid {
		if err := ValidateOpenURI(raw); err != nil {
			t.Errorf("ValidateOpenURI(%q) = %v, want nil", raw, err)
		}
	}
}

func TestValidateOpenURI_Invalid(t *testing.T) {
	invalid := []string{
		"",
		"://",
		"ftp://example.com",
		"javascript:alert(1)",
		"file:///etc/passwd",
		"data:text/html,<script>alert(1)</script>",
	}
	for _, raw := range invalid {
		if err := ValidateOpenURI(raw); err == nil {
			t.Errorf("ValidateOpenURI(%q) = nil, want error", raw)
		}
	}
}

func TestValidateOpenURI_SchemeCaseInsensitive(t *testing.T) {
	if err := ValidateOpenURI("HTTPS://example.com"); err != nil {
		t.Errorf("scheme should be case-insensitive: %v", err)
	}
	if err := ValidateOpenURI("MAILTO:user@example.com"); err != nil {
		t.Errorf("scheme should be case-insensitive: %v", err)
	}
}

func TestSendNotificationErrorPaths(t *testing.T) {
	// Verify that empty title/body don't panic.
	result := SendNotification("", "")
	// Result may be OK or Error depending on platform; just verify
	// it's a valid enum value.
	if result.Status != gui.NotificationOK &&
		result.Status != gui.NotificationError {
		t.Errorf("unexpected status: %v", result.Status)
	}
}

func TestSpellCheckForwardersDoNotPanic(t *testing.T) {
	ranges := SpellCheck("hello")
	_ = ranges

	suggestions := SpellSuggest("helo", 0, 4)
	_ = suggestions

	SpellLearn("hello")
}

func TestValidateOpenURI_Unicode(t *testing.T) {
	// Unicode in path should be valid as long as scheme is ok.
	err := ValidateOpenURI(
		"https://example.com/中文")
	if err != nil {
		t.Errorf("unicode path should be valid: %v", err)
	}
}

func TestValidateOpenURI_Newlines(t *testing.T) {
	// Newlines in URI are a security risk (command injection).
	err := ValidateOpenURI("http://example.com\nrm -rf /")
	if err == nil {
		t.Error("URI with newline should be invalid")
	}
}

func TestValidateOpenURI_NullByte(t *testing.T) {
	err := ValidateOpenURI("http://example.com\x00")
	if err == nil {
		t.Error("URI with null byte should be invalid")
	}
}

func TestValidateOpenURI_LongURI(t *testing.T) {
	// URIs within the cap should validate fine.
	long := "https://example.com/" + strings.Repeat("a", 8000)
	err := ValidateOpenURI(long)
	if err != nil {
		t.Errorf("URI within cap should be valid: %v", err)
	}
}

func TestValidateOpenURI_TooLong(t *testing.T) {
	// URIs exceeding maxURILen should be rejected before parsing.
	tooLong := "https://example.com/" + strings.Repeat("a", maxURILen)
	err := ValidateOpenURI(tooLong)
	if err == nil {
		t.Error("URI exceeding maxURILen should be rejected")
	}
}
