//go:build windows

package gui

import (
	"encoding/binary"
	"fmt"
	"io"
	"syscall"
	"unsafe"
)

// loadedModule is one DLL or EXE mapped into the test process.
type loadedModule struct {
	name string
	base uintptr
	// timeDateStamp and sizeOfImage come from the module's PE header. Together
	// they are the Microsoft symbol-server key for the exact build:
	// .../symbols/<name>/<TIMEDATESTAMP:%08X><SIZEOFIMAGE:%X>/<name>. Zero
	// when the header could not be read.
	timeDateStamp uint32
	sizeOfImage   uint32
}

var (
	modKernel32            = syscall.NewLazyDLL("kernel32.dll")
	procEnumProcessModules = modKernel32.NewProc("K32EnumProcessModules")
	procGetModuleBaseNameW = modKernel32.NewProc("K32GetModuleBaseNameW")
	procReadProcessMemory  = modKernel32.NewProc("ReadProcessMemory")
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
		stamp, size := readPEKey(proc, uintptr(h))
		mods = append(mods, loadedModule{
			name:          name,
			base:          uintptr(h),
			timeDateStamp: stamp,
			sizeOfImage:   size,
		})
	}
	return mods, nil
}

// readPEKey reads the PE header at a module base and returns its
// TimeDateStamp and SizeOfImage, or zeros when the header cannot be read.
// ReadProcessMemory copies the header into a Go buffer, so no uintptr is
// turned back into a pointer.
func readPEKey(proc syscall.Handle, base uintptr) (stamp, size uint32) {
	// The DOS header, PE signature, file header and the start of the
	// optional header all sit in the first page; 0x400 bytes covers them for
	// every image the linker produces.
	var hdr [0x400]byte
	var read uintptr
	r, _, _ := procReadProcessMemory.Call(
		uintptr(proc),
		base,
		uintptr(unsafe.Pointer(&hdr[0])),
		uintptr(len(hdr)),
		uintptr(unsafe.Pointer(&read)),
	)
	if r == 0 || read != uintptr(len(hdr)) || hdr[0] != 'M' || hdr[1] != 'Z' {
		return 0, 0
	}
	// e_lfanew (offset 0x3C) points at the "PE\0\0" signature. The file
	// header follows it (TimeDateStamp at +8), then the optional header
	// (SizeOfImage at +56 in both PE32 and PE32+).
	pe := int(binary.LittleEndian.Uint32(hdr[0x3C:]))
	const fileHdr, sizeOfImageOff = 4, 4 + 20 + 56
	if pe < 0 || pe+sizeOfImageOff+4 > len(hdr) ||
		string(hdr[pe:pe+4]) != "PE\x00\x00" {
		return 0, 0
	}
	stamp = binary.LittleEndian.Uint32(hdr[pe+fileHdr+4:])
	size = binary.LittleEndian.Uint32(hdr[pe+sizeOfImageOff:])
	return stamp, size
}

// logLoadedModules writes one line per loaded module. A Windows CI crash
// (#886) reports a bogus return PC that points into a system DLL whose base
// is randomized on each boot. This log, printed in the same job output as the
// crash, maps that PC to a module and an offset. The key column is the
// module's symbol-server key, which names the exact build so the offset can
// be symbolized against the right PDB.
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
		_, _ = fmt.Fprintf(w, "gui test: module %#x %s key %08X%X\n",
			m.base, m.name, m.timeDateStamp, m.sizeOfImage)
	}
}
