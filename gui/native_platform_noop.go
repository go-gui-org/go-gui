package gui

// NoopNativePlatform is a zero-value NativePlatform where every
// method is a no-op. Embed it in test mocks and override only
// the methods under test.
//
// Methods are documented by the interfaces they satisfy; individual
// method comments are omitted intentionally.
type noopNativePlatform struct{}

func (noopNativePlatform) ShowOpenDialog(_, _ string, _ []string, _ bool) PlatformDialogResult {
	return PlatformDialogResult{}
}
func (noopNativePlatform) ShowSaveDialog(_, _, _, _ string, _ []string, _ bool) PlatformDialogResult {
	return PlatformDialogResult{}
}
func (noopNativePlatform) ShowFolderDialog(_, _ string) PlatformDialogResult {
	return PlatformDialogResult{}
}
func (noopNativePlatform) ShowMessageDialog(_, _ string, _ NativeAlertLevel) NativeAlertResult {
	return NativeAlertResult{}
}
func (noopNativePlatform) ShowConfirmDialog(_, _ string, _ NativeAlertLevel) NativeAlertResult {
	return NativeAlertResult{}
}
func (noopNativePlatform) ShowSaveDiscardDialog(_, _ string, _ NativeAlertLevel) NativeAlertResult {
	return NativeAlertResult{}
}
func (noopNativePlatform) SendNotification(_, _ string) NativeNotificationResult {
	return NativeNotificationResult{}
}
func (noopNativePlatform) ShowPrintDialog(_ NativePrintParams) PrintRunResult {
	return PrintRunResult{}
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
func (noopNativePlatform) SetWindowVibrancy(_ VibrancyMaterial)                {}
func (noopNativePlatform) StartWindowDrag()                                    {}
func (noopNativePlatform) StartWindowResize(_ WindowEdge)                      {}
func (noopNativePlatform) SpellCheck(_ string) []SpellRange                    { return nil }
func (noopNativePlatform) SpellSuggest(_ string, _, _ int) []string            { return nil }
func (noopNativePlatform) SpellLearn(_ string)                                 {}
func (noopNativePlatform) SetNativeMenubar(_ NativeMenubarCfg, _ func(string)) {}
func (noopNativePlatform) ClearNativeMenubar()                                 {}
func (noopNativePlatform) CreateSystemTray(_ SystemTrayCfg, _ func(string)) (int, error) {
	return 0, nil
}
func (noopNativePlatform) UpdateSystemTray(_ int, _ SystemTrayCfg) {}
func (noopNativePlatform) RemoveSystemTray(_ int)                  {}
func (noopNativePlatform) Beep()                                   {}
func (noopNativePlatform) BeepAvailable() bool                     { return false }
