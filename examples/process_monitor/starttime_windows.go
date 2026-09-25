//go:build windows

package main

import (
	"syscall"
	"time"
	"unsafe"
)

// Process start times on Windows come from the process handle itself: no
// extra subprocess, no WMI, no new dependency. OpenProcess can fail for
// protected processes; callers treat a zero time as PID-only for that row.

var (
	modKernel32         = syscall.NewLazyDLL("kernel32.dll")
	procOpenProcess     = modKernel32.NewProc("OpenProcess")
	procCloseHandle     = modKernel32.NewProc("CloseHandle")
	procGetProcessTimes = modKernel32.NewProc("GetProcessTimes")
)

// processQueryLimitedInformation is enough to read the times.
const processQueryLimitedInformation = 0x1000

// processStartTime returns the creation time of pid, or the zero time when
// the process cannot be opened or queried.
func processStartTime(pid int) time.Time {
	handle, _, _ := procOpenProcess.Call(
		uintptr(processQueryLimitedInformation), 0, uintptr(pid))
	if handle == 0 {
		return time.Time{}
	}
	defer func() { _, _, _ = procCloseHandle.Call(handle) }()

	var creation, exit, kernel, user syscall.Filetime
	ret, _, _ := procGetProcessTimes.Call(handle,
		uintptr(unsafe.Pointer(&creation)),
		uintptr(unsafe.Pointer(&exit)),
		uintptr(unsafe.Pointer(&kernel)),
		uintptr(unsafe.Pointer(&user)))
	if ret == 0 {
		return time.Time{}
	}
	return time.Unix(0, creation.Nanoseconds())
}
