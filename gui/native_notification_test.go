package gui

import (
	"testing"
	"time"
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
