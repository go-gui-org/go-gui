package gui

// VibrancyMaterial selects the macOS NSVisualEffectView material shown behind
// a vibrant window. macOS-only; other platforms treat every value as a no-op.
type VibrancyMaterial uint8

// VibrancyMaterial constants. VibrancyNone disables the effect and restores an
// opaque window.
const (
	VibrancyNone VibrancyMaterial = iota // no effect (opaque window)
	VibrancySidebar
	VibrancyMenu
	VibrancyHUD
	VibrancyUnderWindow
)

// SetWindowVibrancy places a translucent native backdrop (blur) behind the
// window content. macOS only; no-op on other platforms or when no native
// platform is set. Pair with a translucent WindowCfg.BgColor (alpha < 255) so
// the backdrop shows through the rendered content.
func (w *Window) SetWindowVibrancy(m VibrancyMaterial) {
	if w.nativePlatform != nil {
		w.nativePlatform.SetWindowVibrancy(m)
	}
}
