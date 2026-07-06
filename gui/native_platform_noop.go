package gui

// NoopNativePlatform is a zero-value NativePlatform where every
// method is a no-op. Embed it in test mocks and override only
// the methods under test.
//
// Methods are documented by the interfaces they satisfy; individual
// method comments are omitted intentionally.
type NoopNativePlatform struct{}

func (NoopNativePlatform) ShowOpenDialog(_, _ string, _ []string, _ bool) PlatformDialogResult {
	return PlatformDialogResult{}
}
func (NoopNativePlatform) ShowSaveDialog(_, _, _, _ string, _ []string, _ bool) PlatformDialogResult {
	return PlatformDialogResult{}
}
func (NoopNativePlatform) ShowFolderDialog(_, _ string) PlatformDialogResult {
	return PlatformDialogResult{}
}
func (NoopNativePlatform) ShowMessageDialog(_, _ string, _ NativeAlertLevel) NativeAlertResult {
	return NativeAlertResult{}
}
func (NoopNativePlatform) ShowConfirmDialog(_, _ string, _ NativeAlertLevel) NativeAlertResult {
	return NativeAlertResult{}
}
func (NoopNativePlatform) ShowSaveDiscardDialog(_, _ string, _ NativeAlertLevel) NativeAlertResult {
	return NativeAlertResult{}
}
func (NoopNativePlatform) SendNotification(_, _ string) NativeNotificationResult {
	return NativeNotificationResult{}
}
func (NoopNativePlatform) ShowPrintDialog(_ NativePrintParams) PrintRunResult {
	return PrintRunResult{}
}
func (NoopNativePlatform) BookmarkLoadAll(_ string) []BookmarkEntry            { return nil }
func (NoopNativePlatform) BookmarkPersist(_, _ string, _ []byte)               {}
func (NoopNativePlatform) BookmarkStopAccess(_ []byte)                         {}
func (NoopNativePlatform) A11yInit(_ func(action, index int))                  {}
func (NoopNativePlatform) A11ySync(_ []A11yNode, _, _ int)                     {}
func (NoopNativePlatform) A11yDestroy()                                        {}
func (NoopNativePlatform) A11yAnnounce(_ string)                               {}
func (NoopNativePlatform) IMEStart()                                           {}
func (NoopNativePlatform) IMEStop()                                            {}
func (NoopNativePlatform) IMESetRect(_, _, _, _ int32)                         {}
func (NoopNativePlatform) OpenURI(_ string) error                              { return nil }
func (NoopNativePlatform) TitlebarDark(_ bool)                                 {}
func (NoopNativePlatform) SetWindowVibrancy(_ VibrancyMaterial)                {}
func (NoopNativePlatform) SpellCheck(_ string) []SpellRange                    { return nil }
func (NoopNativePlatform) SpellSuggest(_ string, _, _ int) []string            { return nil }
func (NoopNativePlatform) SpellLearn(_ string)                                 {}
func (NoopNativePlatform) SetNativeMenubar(_ NativeMenubarCfg, _ func(string)) {}
func (NoopNativePlatform) ClearNativeMenubar()                                 {}
func (NoopNativePlatform) CreateSystemTray(_ SystemTrayCfg, _ func(string)) (int, error) {
	return 0, nil
}
func (NoopNativePlatform) UpdateSystemTray(_ int, _ SystemTrayCfg) {}
func (NoopNativePlatform) RemoveSystemTray(_ int)                  {}
