package gui

import (
	"encoding/json"
	"os/exec"
	"strings"
	"testing"
)

// defaultFontFamily and defaultMonoFontFamily are declared once per
// platform, in a theme_defaults_<goos>.go file. Only darwin, linux,
// windows and js/wasm had one, so a build for any other GOOS failed on
// two missing constants — and nothing noticed, because the repo's
// cross-compile gate builds linux and windows only, and go-glyph
// (upstream) has no freebsd build to compile against even if it did.
//
// This is the gate instead: ask the toolchain which files it would
// select for each GOOS and assert exactly one of them declares the
// constants. It answers the question the compiler would, without
// needing the package to link on that platform.
func TestDefaultFontConstantsCoverEveryGOOS(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to go list")
	}
	for _, target := range distTargets(t) {
		t.Run(target.goos, func(t *testing.T) {
			files := goFilesFor(t, target.goos, target.goarch)
			var picked []string
			for _, f := range files {
				if strings.HasPrefix(f, "theme_defaults_") &&
					!strings.HasSuffix(f, "_test.go") {
					picked = append(picked, f)
				}
			}
			if len(picked) != 1 {
				t.Errorf("GOOS=%s selects %v, want exactly one "+
					"theme_defaults_<goos>.go", target.goos, picked)
			}
		})
	}
}

// target is one GOOS with an architecture it actually supports.
type target struct{ goos, goarch string }

// distTargets returns one buildable GOOS/GOARCH pair per GOOS, taken
// from the toolchain rather than hardcoded, so a GOOS added by a future
// Go release is covered the day the repo upgrades.
func distTargets(t *testing.T) []target {
	t.Helper()
	out, err := exec.Command("go", "tool", "dist", "list").Output()
	if err != nil {
		t.Skipf("go tool dist list: %v", err)
	}
	seen := map[string]bool{}
	var targets []target
	for line := range strings.SplitSeq(strings.TrimSpace(string(out)), "\n") {
		goos, goarch, ok := strings.Cut(strings.TrimSpace(line), "/")
		if !ok || seen[goos] {
			continue
		}
		seen[goos] = true
		targets = append(targets, target{goos, goarch})
	}
	return targets
}

// goFilesFor asks the toolchain which files it compiles for a target.
// -e keeps a package that will not build on that platform listable,
// which is the whole point: the constants must be declared even where
// a dependency has no port.
func goFilesFor(t *testing.T, goos, goarch string) []string {
	t.Helper()
	cmd := exec.Command("go", "list", "-e", "-json", ".")
	cmd.Env = append(cmd.Environ(),
		"GOOS="+goos, "GOARCH="+goarch, "CGO_ENABLED=0")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("go list for %s/%s: %v", goos, goarch, err)
	}
	var pkg struct {
		GoFiles        []string
		IgnoredGoFiles []string
	}
	if err := json.Unmarshal(out, &pkg); err != nil {
		t.Fatalf("decoding go list output for %s: %v", goos, err)
	}
	return pkg.GoFiles
}
