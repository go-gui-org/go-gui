//go:build linux && !js && !android

package gl

import "syscall"

// monitorSysProcAttr has the kernel kill the gsettings monitor child
// when this process dies. WindowCleanup stops the monitor on a clean
// exit, but os.Exit, log.Fatal and a crash skip it, and the child
// then lives on: it blocks until the next color-scheme change and
// only then dies on the broken pipe.
//
// Pdeathsig fires when the OS thread that forked the child exits,
// not only the process. The Go runtime rarely ends a thread (only a
// goroutine that exits while locked to its thread). If it does, the
// monitor dies early, reads as an abnormal exit, and restarts after
// monitorRestartDelay, so the cost is one short gap in watching.
func monitorSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Pdeathsig: syscall.SIGKILL}
}
