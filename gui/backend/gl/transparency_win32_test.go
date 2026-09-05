//go:build windows && !js

package gl

import (
	"testing"
	"unsafe"
)

// DWM reads DWM_BLURBEHIND by offset, so a layout that drifts from
// dwmapi.h is silently wrong rather than a compile error: DWM would
// read the flags from one field and the region from another.
func TestDwmBlurBehindLayout(t *testing.T) {
	var bb dwmBlurBehind
	// DWORD + BOOL + HRGN + BOOL, with HRGN 8-byte aligned on 64-bit.
	want := unsafe.Sizeof(uintptr(0)) + 16
	if got := unsafe.Sizeof(bb); got != want {
		t.Errorf("sizeof(DWM_BLURBEHIND) = %d, want %d", got, want)
	}
	if off := unsafe.Offsetof(bb.dwFlags); off != 0 {
		t.Errorf("dwFlags at offset %d, want 0", off)
	}
	if off := unsafe.Offsetof(bb.fEnable); off != 4 {
		t.Errorf("fEnable at offset %d, want 4", off)
	}
}

// The empty-region trick is what makes this transparency rather than
// blur, so both flags must be set: DWM_BB_ENABLE alone blurs the whole
// client area.
func TestDwmBlurBehindFlags(t *testing.T) {
	if dwmBBEnable|dwmBBBlurRegion != 0x3 {
		t.Errorf("flags = %#x, want 0x3", dwmBBEnable|dwmBBBlurRegion)
	}
}
