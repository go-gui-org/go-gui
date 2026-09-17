package gui

import (
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

// mockNotificationPlatform stubs NativePlatform with a configurable
// SendNotification result.
type mockNotificationPlatform struct {
	noopNativePlatform
	result NativeNotificationResult
}

func (m *mockNotificationPlatform) SendNotification(_, _ string) NativeNotificationResult {
	return m.result
}

func TestNativeNotificationEmptyTitle(t *testing.T) {
	w := &Window{}
	var result NativeNotificationResult
	cfg := NativeNotificationCfg{
		Title:  "",
		OnDone: func(r NativeNotificationResult, _ *Window) { result = r },
	}
	w.NativeNotification(cfg)
	if result.Status != NotificationError {
		t.Errorf("expected error, got %d", result.Status)
	}
	if result.ErrorCode != "invalid_cfg" {
		t.Errorf("expected 'invalid_cfg', got %q", result.ErrorCode)
	}
}

func TestNativeNotificationNoPlatform(t *testing.T) {
	w := &Window{}
	var result NativeNotificationResult
	cfg := NativeNotificationCfg{
		Title:  "Hello",
		Body:   "World",
		OnDone: func(r NativeNotificationResult, _ *Window) { result = r },
	}
	nativeNotificationImpl(w, cfg)
	if result.Status != NotificationError {
		t.Errorf("expected error, got %d", result.Status)
	}
	if result.ErrorCode != "unsupported" {
		t.Errorf("expected 'unsupported', got %q", result.ErrorCode)
	}
}

func TestNativeNotificationMockPlatform(t *testing.T) {
	w := &Window{}
	w.nativePlatform = &mockNotificationPlatform{
		result: NativeNotificationResult{Status: NotificationOK},
	}
	var result NativeNotificationResult
	cfg := NativeNotificationCfg{
		Title:  "Hello",
		Body:   "World",
		OnDone: func(r NativeNotificationResult, _ *Window) { result = r },
	}
	nativeNotificationImpl(w, cfg)
	// Wait for goroutine to queue the command.
	time.Sleep(5 * time.Millisecond)
	w.flushCommands()
	if result.Status != NotificationOK {
		t.Errorf("expected OK, got %d", result.Status)
	}
}

func TestNativeNotificationMockPlatformError(t *testing.T) {
	w := &Window{}
	w.nativePlatform = &mockNotificationPlatform{
		result: NativeNotificationResult{
			Status:       NotificationError,
			ErrorCode:    "exec_failed",
			ErrorMessage: "mock error",
		},
	}
	var result NativeNotificationResult
	cfg := NativeNotificationCfg{
		Title:  "Hello",
		Body:   "World",
		OnDone: func(r NativeNotificationResult, _ *Window) { result = r },
	}
	nativeNotificationImpl(w, cfg)
	time.Sleep(5 * time.Millisecond)
	w.flushCommands()
	if result.Status != NotificationError {
		t.Errorf("expected error, got %d", result.Status)
	}
	if result.ErrorCode != "exec_failed" {
		t.Errorf("expected 'exec_failed', got %q", result.ErrorCode)
	}
	if result.ErrorMessage != "mock error" {
		t.Errorf("expected 'mock error', got %q", result.ErrorMessage)
	}
}

func TestNativeNotificationNilOnDone(_ *testing.T) {
	w := &Window{}
	cfg := NativeNotificationCfg{Title: "", Body: "B"}
	// Must not panic with nil OnDone.
	w.NativeNotification(cfg)
}

func TestNativeNotificationStatusValues(t *testing.T) {
	if NotificationOK != 0 || NotificationDenied != 1 || NotificationError != 2 {
		t.Error("unexpected status enum values")
	}
}

// notificationRecorder captures the title/body the impl hands the
// backend, so the length caps below can assert on them.
type notificationRecorder struct {
	noopNativePlatform
	title  string
	body   string
	result NativeNotificationResult
}

func (m *notificationRecorder) SendNotification(title, body string) NativeNotificationResult {
	m.title = title
	m.body = body
	return m.result
}

func TestNativeNotificationTruncatesToBackendCaps(t *testing.T) {
	w := &Window{}
	rec := &notificationRecorder{
		result: NativeNotificationResult{Status: NotificationOK},
	}
	w.nativePlatform = rec
	cfg := NativeNotificationCfg{
		Title:  strings.Repeat("é", 200), // 400 bytes > 256
		Body:   strings.Repeat("中", 500), // 1500 bytes > 1024
		OnDone: func(NativeNotificationResult, *Window) {},
	}
	nativeNotificationImpl(w, cfg)
	time.Sleep(5 * time.Millisecond)
	w.flushCommands()
	if len(rec.title) > maxNotificationTitleLen {
		t.Errorf("title %d bytes, want <= %d", len(rec.title), maxNotificationTitleLen)
	}
	if len(rec.body) > maxNotificationBodyLen {
		t.Errorf("body %d bytes, want <= %d", len(rec.body), maxNotificationBodyLen)
	}
	if !utf8.ValidString(rec.title) || !utf8.ValidString(rec.body) {
		t.Error("truncated strings must stay valid UTF-8")
	}
}
