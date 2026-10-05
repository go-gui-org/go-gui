//go:build windows

package gui

import (
	"fmt"
	"io"
	"syscall"
	"unsafe"
)

// loadedModule is one DLL or EXE mapped into the test process.
type loadedModule struct {
	name string
	base uintptr
}

var (
	modKernel32            = syscall.NewLazyDLL("kernel32.dll")
	procEnumProcessModules = modKernel32.NewProc("K32EnumProcessModules")
	procGetModuleBaseNameW = modKernel32.NewProc("K32GetModuleBaseNameW")
)

// loadedModules lists the modules mapped into this process with their load
// bases. On Windows an HMODULE is the module's base address, so no extra call
// is needed to get the base.
func loadedModules() ([]loadedModule, error) {
	proc, err := syscall.GetCurrentProcess()
	if err != nil {
		return nil, err
	}
	// The module count can grow between calls, so retry with the size
	// the call reports until the buffer is big enough.
	handles := make([]syscall.Handle, 256)
	for {
		var needed uint32
		size := uint32(len(handles)) * uint32(unsafe.Sizeof(handles[0]))
		r, _, callErr := procEnumProcessModules.Call(
			uintptr(proc),
			uintptr(unsafe.Pointer(&handles[0])),
			uintptr(size),
			uintptr(unsafe.Pointer(&needed)),
		)
		if r == 0 {
			return nil, fmt.Errorf("K32EnumProcessModules: %w", callErr)
		}
		if needed <= size {
			handles = handles[:needed/uint32(unsafe.Sizeof(handles[0]))]
			break
		}
		handles = make([]syscall.Handle, needed/uint32(unsafe.Sizeof(handles[0]))+16)
	}

	mods := make([]loadedModule, 0, len(handles))
	nameBuf := make([]uint16, syscall.MAX_PATH)
	for _, h := range handles {
		n, _, _ := procGetModuleBaseNameW.Call(
			uintptr(proc),
			uintptr(h),
			uintptr(unsafe.Pointer(&nameBuf[0])),
			uintptr(len(nameBuf)),
		)
		name := "?"
		if n > 0 {
			name = syscall.UTF16ToString(nameBuf[:n])
		}
		mods = append(mods, loadedModule{name: name, base: uintptr(h)})
	}
	return mods, nil
}

// logLoadedModules writes one line per loaded module. A Windows CI crash
// (#886) reports a bogus return PC that points into a system DLL whose base
// is randomized on each boot. This log, printed in the same job output as the
// crash, maps that PC to a module and an offset.
//
// The list is a snapshot taken before any test runs. A DLL a test loads later
// is not in it. Logging again at exit would not help: the crash is a runtime
// fatal error, which skips deferred calls and never reaches os.Exit. The Go
// runtime loads its system DLLs (ntdll, kernel32, kernelbase, ...) before
// TestMain, so the snapshot covers the likely owners of the bad PC.
func logLoadedModules(w io.Writer) {
	mods, err := loadedModules()
	if err != nil {
		_, _ = fmt.Fprintf(w, "gui test: module list failed: %v\n", err)
		return
	}
	_, _ = fmt.Fprintf(w,
		"gui test: %d modules at startup (DLLs loaded later are not listed)\n",
		len(mods))
	for _, m := range mods {
		_, _ = fmt.Fprintf(w, "gui test: module %#x %s\n", m.base, m.name)
	}
}
