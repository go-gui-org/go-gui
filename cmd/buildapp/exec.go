package main

import (
	"os/exec"
)

// exec.go holds the single seam every external tool call goes through,
// so tests inject fakes instead of needing Apple's toolchain.
//
// execOutput runs name with args and extra environment entries (nil
// means inherit). It returns the combined output. execLookPath reports
// whether name resolves, without running anything.
var execOutput = func(env []string, name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...) // #nosec G204 — build tool, args from flags
	if env != nil {
		cmd.Env = env
	}
	return cmd.CombinedOutput()
}

// runTool is execOutput with an inherited environment, for the macOS
// packager calls that set no variables.
func runTool(name string, args ...string) ([]byte, error) {
	return execOutput(nil, name, args...)
}

var execLookPath = func(name string) error {
	_, err := exec.LookPath(name)
	return err
}
