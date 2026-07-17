//go:build !darwin && !linux && !windows

package main

import "errors"

// collectProcesses is a stub for platforms without a supported collector. The
// UI surfaces the error the same way it surfaces any sample failure.
func collectProcesses() ([]ProcInfo, error) {
	return nil, errors.New("process listing is not supported on this platform")
}
