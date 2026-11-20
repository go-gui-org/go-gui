//go:build darwin && arm64 && !ios

package metal

// activateApp sets the NSApplication activation policy to Regular
// and brings the app to the foreground. Must be called before
// creating any windows for CLI-launched (non-bundled) binaries.
func activateApp() {
	metalAppInit()
}

// setAppIcon sets the macOS Dock icon from PNG data.
func setAppIcon(png []byte) {
	if len(png) == 0 {
		return
	}
	metalSetDockIcon(png)
}
