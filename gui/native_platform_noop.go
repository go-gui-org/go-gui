package gui

import "sync/atomic"

// noopTrayIDs hands out unique positive tray IDs process-wide.
// NoopNativePlatform is a zero-value struct, so per-instance
// counters are impossible; the global keeps handles distinct.
var noopTrayIDs atomic.Int64

// NoopNativePlatform is a zero-value NativePlatform where every
// method is a no-op. Embed it in test mocks and override only
// the methods under test.
//
// Methods are documented by the interfaces they satisfy; individual
// method comments are omitted intentionally.
type noopNativePlatform struct{}

func (noopNativePlatform) ShowOpenDialog(_, _ string, _ []string, _ bool) PlatformDialogResult {
	// Cancelled, not OK: the zero PlatformDialogResult is DialogOK,
	// so a bare zero value would report a success with no paths.
	return PlatformDialogResult{Status: DialogCancel}
}
func (noopNativePlatform) ShowSaveDialog(_, _, _, _ string, _ []string, _ bool) PlatformDialogResult {
	return PlatformDialogResult{Status: DialogCancel}
}
func (noopNativePlatform) ShowFolderDialog(_, _ string) PlatformDialogResult {
	return PlatformDialogResult{Status: DialogCancel}
}
func (noopNativePlatform) ShowMessageDialog(_, _ string, _ NativeAlertLevel) NativeAlertResult {
	return NativeAlertResult{Status: DialogCancel}
}
func (noopNativePlatform) ShowConfirmDialog(_, _ string, _ NativeAlertLevel) NativeAlertResult {
	return NativeAlertResult{Status: DialogCancel}
}
func (noopNativePlatform) ShowSaveDiscardDialog(_, _ string, _ NativeAlertLevel) NativeAlertResult {
	return NativeAlertResult{Status: DialogCancel}
}
func (noopNativePlatform) SendNotification(_, _ string) NativeNotificationResult {
	// Error, not the zero NotificationOK: nothing was delivered.
	return NativeNotificationResult{
		Status:       NotificationError,
		ErrorCode:    "unsupported",
		ErrorMessage: "no native platform",
	}
}
func (noopNativePlatform) ShowPrintDialog(_ NativePrintParams) PrintRunResult {
	// Cancelled, not the zero PrintRunOK: nothing was printed.
	return PrintRunResult{Status: PrintRunCancel}
}
func (noopNativePlatform) BookmarkLoadAll(_ string) []BookmarkEntry            { return nil }
func (noopNativePlatform) BookmarkPersist(_, _ string, _ []byte)               {}
func (noopNativePlatform) BookmarkStopAccess(_ []byte)                         {}
func (noopNativePlatform) A11yInit(_ func(action, index int))                  {}
func (noopNativePlatform) A11ySync(_ []A11yNode, _, _ int)                     {}
func (noopNativePlatform) A11yDestroy()                                        {}
func (noopNativePlatform) A11yAnnounce(_ string)                               {}
func (noopNativePlatform) IMEStart()                                           {}
func (noopNativePlatform) IMEStop()                                            {}
func (noopNativePlatform) IMESetRect(_, _, _, _ int32)                         {}
func (noopNativePlatform) OpenURI(_ string) error                              { return nil }
func (noopNativePlatform) TitlebarDark(_ bool)                                 {}
func (noopNativePlatform) SystemAppearance() (Appearance, bool)                { return AppearanceLight, false }
func (noopNativePlatform) SetSystemAppearanceCallback(_ func(Appearance))      {}
func (noopNativePlatform) SetWindowVibrancy(_ VibrancyMaterial)                {}
func (noopNativePlatform) SetWindowOpacity(_ float32)                          {}
func (noopNativePlatform) StartWindowDrag()                                    {}
func (noopNativePlatform) StartWindowResize(_ WindowEdge)                      {}
func (noopNativePlatform) SpellCheck(_ string) []SpellRange                    { return nil }
func (noopNativePlatform) SpellSuggest(_ string, _, _ int) []string            { return nil }
func (noopNativePlatform) SpellLearn(_ string)                                 {}
func (noopNativePlatform) SetNativeMenubar(_ NativeMenubarCfg, _ func(string)) {}
func (noopNativePlatform) ClearNativeMenubar()                                 {}
func (noopNativePlatform) CreateSystemTray(_ SystemTrayCfg, _ func(string)) (int, error) {
	return int(noopTrayIDs.Add(1)), nil
}
func (noopNativePlatform) UpdateSystemTray(_ int, _ SystemTrayCfg) {}
func (noopNativePlatform) RemoveSystemTray(_ int)                  {}
func (noopNativePlatform) Beep()                                   {}
func (noopNativePlatform) BeepAvailable() bool                     { return false }
