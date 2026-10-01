package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// resolveBinary maps the command line onto the binary to package.
// With a -build-pkg package there is no positional binary: the package
// is compiled first. Without it, exactly one positional binary is
// required, as before.
func resolveBinary(buildPkg string, args []string) (string, error) {
	if buildPkg != "" {
		if len(args) != 0 {
			return "", errors.New("-build-pkg takes no binary argument: it compiles the package itself")
		}
		return "", nil
	}
	if len(args) != 1 {
		return "", errors.New("expected exactly one binary argument")
	}
	return args[0], nil
}

// compileInTempDir compiles o.BuildPkg for o.Platform/o.Arch and
// returns the staged binary path. The caller removes the binary's
// parent directory when done.
func compileInTempDir(o bundleOpts) (string, error) {
	stage, err := os.MkdirTemp("", "buildapp-build-*")
	if err != nil {
		return "", err
	}
	name := filepath.Base(filepath.Clean(o.BuildPkg))
	// A package path that cleans to a dot name ("..", ".", "/") would
	// escape the staging directory as a binary name; fall back to a
	// fixed one instead.
	if name == "." || name == ".." ||
		name == string(filepath.Separator) || name == "" {
		name = "app"
	}
	if platformOrHost(o.Platform) == "windows" {
		name += ".exe"
	}
	out := filepath.Join(stage, name)
	if err = compilePkg(o, out); err != nil {
		_ = os.RemoveAll(stage)
		return "", err
	}
	return out, nil
}

// compilePkg runs `go build` for the target platform and arch. Linux
// and Windows build cgo-free; macOS keeps the host C toolchain for the
// Metal backend. On Windows "-H windowsgui" marks the PE as a
// GUI-subsystem image, so no console window opens behind the app.
func compilePkg(o bundleOpts, out string) error {
	goos := platformOrHost(o.Platform)
	goarch := o.Arch
	if goarch == "" {
		goarch = runtime.GOARCH
	}
	cgo := "0"
	if goos == "darwin" {
		cgo = "1"
	}
	ldflags := o.Ldflags
	if goos == "windows" && !strings.Contains(ldflags, "windowsgui") {
		ldflags = strings.TrimSpace(ldflags + " -H windowsgui")
	}
	args := []string{"build"}
	if ldflags != "" {
		args = append(args, "-ldflags", ldflags)
	}
	args = append(args, "-o", out, o.BuildPkg)
	env := withEnv(os.Environ(),
		[]string{"GOOS=" + goos, "GOARCH=" + goarch, "CGO_ENABLED=" + cgo})
	combined, cerr := execOutput(env, "go", args...)
	if cerr != nil {
		return fmt.Errorf("go build %s: %v: %s", o.BuildPkg, cerr, combined)
	}
	return nil
}

func platformOrHost(p string) string {
	if p == "" {
		return runtime.GOOS
	}
	return p
}

// withEnv returns base with each override applied, replacing any entry
// for the same key. Appending without replacing would leave duplicates,
// and the tool would read whichever copy it finds first.
func withEnv(base, overrides []string) []string {
	drop := map[string]bool{}
	for _, kv := range overrides {
		key, _, _ := strings.Cut(kv, "=")
		drop[key] = true
	}
	env := make([]string, 0, len(base)+len(overrides))
	for _, kv := range base {
		key, _, _ := strings.Cut(kv, "=")
		if !drop[key] {
			env = append(env, kv)
		}
	}
	return append(env, overrides...)
}
