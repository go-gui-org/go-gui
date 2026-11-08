package gui

import "testing"

func TestSetWindowVibrancyNilPlatform(_ *testing.T) {
	w := NewWindow(WindowCfg{})
	defer w.Close()
	// Should not panic with nil nativePlatform.
	w.SetWindowVibrancy(VibrancyUnderWindow)
	w.SetWindowVibrancy(VibrancyNone)
}
