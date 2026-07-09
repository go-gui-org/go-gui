//go:build darwin

package gl

// On macOS the GL backend is not used — platform_other.go provides
// a stub New that returns nil.  These minimal type definitions exist
// only so platform_other.go's method receivers and return types compile.
// The var block references all stub methods to suppress "unused" lint
// warnings — they are intentionally present for interface consistency
// across platforms.

// Backend is a stub for the GL rendering backend on macOS.
type Backend struct{}

type nativePlatform struct{}

var (
	_ = (*Backend)(nil)
	_ = (*nativePlatform)(nil)

	_ = (*platformState).makeCurrent
	_ = (*platformState).swap
	_ = (*platformState).drawableSize
	_ = (*platformState).dpiScale
	_ = (*platformState).setCursor
	_ = (*platformState).wake
	_ = (*platformState).destroy
	_ = (*platformState).pumpEvents

	_ = (*nativePlatform).IMEStart
	_ = (*nativePlatform).IMEStop
	_ = (*nativePlatform).IMESetRect
	_ = (*nativePlatform).CreateSystemTray
	_ = (*nativePlatform).UpdateSystemTray
	_ = (*nativePlatform).RemoveSystemTray
)
