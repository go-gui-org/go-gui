//go:build windows && !js

package gl

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

func setClipboard(hwnd uintptr, s string) {
	u := windows.StringToUTF16(s) // NUL-terminated
	if r, _, _ := pOpenClipboard.Call(hwnd); r == 0 {
		return
	}
	defer pCloseClipboard.Call()
	pEmptyClipboard.Call()
	n := uintptr(len(u) * 2)
	h, _, _ := pGlobalAlloc.Call(gmemMoveable, n)
	if h == 0 {
		return
	}
	dst, _, _ := pGlobalLock.Call(h)
	if dst == 0 {
		return
	}
	// Copy via RtlMoveMemory to avoid a uintptr→unsafe.Pointer
	// conversion on the locked handle (go vet unsafeptr).
	pRtlMoveMemory.Call(dst, uintptr(unsafe.Pointer(&u[0])), n)
	pGlobalUnlock.Call(h)
	pSetClipboardData.Call(cfUnicodeText, h)
}

func getClipboard(hwnd uintptr) string {
	if r, _, _ := pOpenClipboard.Call(hwnd); r == 0 {
		return ""
	}
	defer pCloseClipboard.Call()
	h, _, _ := pGetClipboardData.Call(cfUnicodeText)
	if h == 0 {
		return ""
	}
	src, _, _ := pGlobalLock.Call(h)
	if src == 0 {
		return ""
	}
	defer pGlobalUnlock.Call(h)
	n, _, _ := pLstrlenW.Call(src) // uint16 count, excluding NUL
	if n == 0 {
		return ""
	}
	if n > maxClipboardChars {
		n = maxClipboardChars // cap oversized clipboard content
	}
	buf := make([]uint16, n)
	pRtlMoveMemory.Call(uintptr(unsafe.Pointer(&buf[0])), src, n*2)
	return windows.UTF16ToString(buf)
}
