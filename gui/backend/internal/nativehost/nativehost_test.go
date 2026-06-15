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

func TestSendNotificationLengthCapping(t *testing.T) {
	// title and body are capped at maxNotifyTitleLen / maxNotifyBodyLen
	// before the exec call. Verify no panic with oversized inputs.
	longTitle := strings.Repeat("T", maxNotifyTitleLen+100)
	longBody := strings.Repeat("B", maxNotifyBodyLen+100)
	result := SendNotification(longTitle, longBody)
	if result.Status != gui.NotificationOK &&
		result.Status != gui.NotificationError {
		t.Errorf("unexpected status: %v", result.Status)
	}
}

func TestSendNotificationEmptyBody(t *testing.T) {
	result := SendNotification("test", "")
	if result.Status != gui.NotificationOK &&
		result.Status != gui.NotificationError {
		t.Errorf("unexpected status: %v", result.Status)
	}
}

func TestSendNotificationBoundaries(t *testing.T) {
	title := strings.Repeat("T", maxNotifyTitleLen)
	body := strings.Repeat("B", maxNotifyBodyLen)
	result := SendNotification(title, body)
	if result.Status != gui.NotificationOK &&
		result.Status != gui.NotificationError {
		t.Errorf("unexpected status: %v", result.Status)
	}

	// One byte over should also not panic.
	titleOver := strings.Repeat("T", maxNotifyTitleLen+1)
	bodyOver := strings.Repeat("B", maxNotifyBodyLen+1)
	result = SendNotification(titleOver, bodyOver)
	if result.Status != gui.NotificationOK &&
		result.Status != gui.NotificationError {
		t.Errorf("unexpected status: %v", result.Status)
	}
}

func TestSpellForwarderLengthCaps(t *testing.T) {
	// SpellCheck caps text at maxSpellTextLen (64 KB).
	bigText := strings.Repeat("x", maxSpellTextLen+100)
	ranges := SpellCheck(bigText)
	_ = ranges // may be nil or not depending on platform

	// SpellLearn caps word at maxSpellWordLen.
	longWord := strings.Repeat("w", maxSpellWordLen+50)
	SpellLearn(longWord) // must not panic
}

func TestSpellSuggestClampToBounds(t *testing.T) {
	// startByte < 0 → clamped to 0.
	suggestions := SpellSuggest("hello", -1, 3)
	_ = suggestions

	// startByte >= len(text) → nil.
	suggestions = SpellSuggest("hello", 10, 1)
	if suggestions != nil {
		t.Errorf("expected nil for startByte beyond text length, got %v",
			suggestions)
	}

	// lenBytes overflows → clamped to remaining length.
	suggestions = SpellSuggest("hello", 2, 100)
	_ = suggestions // should not panic

	// lenBytes <= 0 → clamped to remaining length.
	suggestions = SpellSuggest("hello", 1, 0)
	_ = suggestions
	suggestions = SpellSuggest("hello", 1, -5)
	_ = suggestions
}

func TestDialogAndPrintForwardersNoPanic(t *testing.T) {
	// Dialog forwarders open native dialogs on macOS — skip the CGo path.
	// Print forwarder is safe on all platforms (returns error without UI).
	_ = ShowPrintDialog(gui.NativePrintParams{})
}
