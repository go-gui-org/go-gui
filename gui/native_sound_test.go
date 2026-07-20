package gui

import "testing"

// beepSpy records Beep calls and reports a configurable availability.
type beepSpy struct {
	NoopNativePlatform
	calls     int
	available bool
}

func (b *beepSpy) Beep()               { b.calls++ }
func (b *beepSpy) BeepAvailable() bool { return b.available }

func TestWindowBeepNoNativePlatform(t *testing.T) {
	// No platform attached (tests, headless): must not panic, and must
	// report unavailable so callers fall back to a visual cue.
	w := NewWindow(WindowCfg{State: new(struct{})})
	w.Beep()
	if w.BeepAvailable() {
		t.Error("BeepAvailable = true with no native platform")
	}
}

func TestWindowBeepNoopPlatform(t *testing.T) {
	w := NewWindow(WindowCfg{State: new(struct{})})
	w.SetNativePlatform(&NoopNativePlatform{})
	w.Beep()
	if w.BeepAvailable() {
		t.Error("BeepAvailable = true for NoopNativePlatform")
	}
}

func TestWindowBeepForwardsToPlatform(t *testing.T) {
	spy := &beepSpy{available: true}
	w := NewWindow(WindowCfg{State: new(struct{})})
	w.SetNativePlatform(spy)

	w.Beep()
	w.Beep()
	if spy.calls != 2 {
		t.Errorf("Beep forwarded %d times, want 2", spy.calls)
	}
	if !w.BeepAvailable() {
		t.Error("BeepAvailable did not forward platform result")
	}
}
