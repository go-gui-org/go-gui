package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/tools/cover"
)

const testProfile = `mode: atomic
github.com/go-gui-org/go-gui/gui/a.go:10.20,15.30 4 3
github.com/go-gui-org/go-gui/gui/a.go:17.20,20.30 2 0
github.com/go-gui-org/go-gui/gui/sub/b.go:5.10,8.20 3 1
github.com/go-gui-org/go-gui/gui/empty/c.go:1.1,2.2 0 5
other.org/x/y.go:1.1,9.9 10 10
`

func parseProfiles(t *testing.T, text string) []*cover.Profile {
	t.Helper()
	profiles, err := cover.ParseProfilesFromReader(strings.NewReader(text))
	if err != nil {
		t.Fatal(err)
	}
	return profiles
}

func TestTotalCoverage(t *testing.T) {
	covered, stmts, pct := totalCoverage(parseProfiles(t, testProfile))
	// stmts: 4+2+3+0+10 = 19; covered: 4+0+3+0+10 = 17.
	if covered != 17 || stmts != 19 {
		t.Errorf("totalCoverage() = (%d, %d), want (17, 19)", covered, stmts)
	}
	if want := 100 * 17.0 / 19.0; pct != want {
		t.Errorf("totalCoverage() pct = %v, want %v", pct, want)
	}
}

func TestTotalCoverageEmpty(t *testing.T) {
	_, stmts, pct := totalCoverage(parseProfiles(t, "mode: atomic\n"))
	if stmts != 0 || pct != 0 {
		t.Errorf("totalCoverage() on empty profile = (%d, %v), want (0, 0)", stmts, pct)
	}
}

func TestPkgCoverage(t *testing.T) {
	got := pkgCoverage(parseProfiles(t, testProfile))
	// Grouped by exact directory: gui holds a.go's blocks (4 of 6
	// statements covered); sub/b.go is its own package at 100%.
	// The zero-statement block contributes nothing.
	if got["github.com/go-gui-org/go-gui/gui"] != 100*4.0/6.0 {
		t.Errorf("gui pkg = %v, want %v", got["github.com/go-gui-org/go-gui/gui"], 100*4.0/6.0)
	}
	if got["github.com/go-gui-org/go-gui/gui/sub"] != 100 {
		t.Errorf("sub pkg = %v, want 100", got["github.com/go-gui-org/go-gui/gui/sub"])
	}
	if got["other.org/x"] != 100 {
		t.Errorf("x pkg = %v, want 100", got["other.org/x"])
	}
	// A file without a slash keeps the whole name as the key.
	single := parseProfiles(t, "mode: set\nplain.go:1.1,5.5 2 1\nplain.go:6.6,9.9 2 0\n")
	if pkgCoverage(single)["plain.go"] != 50 {
		t.Errorf("slashless file pkg = %v, want 50", pkgCoverage(single)["plain.go"])
	}
}

func TestPkgPrefixCoverage(t *testing.T) {
	profiles := parseProfiles(t, testProfile)
	// Subpackages roll into the parent, like the script's "^pkg/" match:
	// gui + sub = 9 stmts, 7 covered.
	stmts, covered := pkgPrefixCoverage(profiles, "github.com/go-gui-org/go-gui/gui")
	if stmts != 9 || covered != 7 {
		t.Errorf("pkgPrefixCoverage(gui) = (%d, %d), want (9, 7)", stmts, covered)
	}
	// A sibling prefix must not match.
	stmts, _ = pkgPrefixCoverage(profiles, "github.com/go-gui-org/go-gui/guix")
	if stmts != 0 {
		t.Errorf("pkgPrefixCoverage(sibling) stmts = %d, want 0", stmts)
	}
}

func TestOneDecimal(t *testing.T) {
	// Delta is computed from the rounded table values, as in awk. The
	// tool re-rounds the difference before comparing or printing, so
	// assert on the rendering, not the raw float.
	if got := fmt.Sprintf("%.1f", oneDecimal(72.34)-oneDecimal(74.74)); got != "-2.4" {
		t.Errorf("rounded delta = %v, want -2.4", got)
	}
	// A raw -2.04 rounds to -2.0 and is not a regression.
	if d := oneDecimal(-2.04); !(d == -2.0 && d >= -regressionDelta) {
		t.Errorf("oneDecimal(-2.04) = %v, want -2.0 (no regression)", d)
	}
}

// captureStdout runs fn with os.Stdout redirected to a buffer.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	fn()
	w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}

func writeProfile(t *testing.T, text string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "cover.out")
	if err := os.WriteFile(p, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestGatePass(t *testing.T) {
	// 17/19 statements = 89.5%, over the threshold; floors are all 0.
	if code := gate([]string{writeProfile(t, testProfile)}); code != 0 {
		t.Errorf("gate() = %d, want 0", code)
	}
}

func TestGateTotalFail(t *testing.T) {
	// 1 of 10 statements covered = 10%, under the threshold.
	low := "mode: set\n" +
		"github.com/go-gui-org/go-gui/gui/a.go:1.1,5.5 9 0\n" +
		"github.com/go-gui-org/go-gui/gui/a.go:6.6,9.9 1 1\n"
	if code := gate([]string{writeProfile(t, low)}); code != 1 {
		t.Errorf("gate() = %d, want 1", code)
	}
}

func TestGateMissingProfile(t *testing.T) {
	if code := gate([]string{filepath.Join(t.TempDir(), "nope.out")}); code != 1 {
		t.Errorf("gate() = %d, want 1", code)
	}
}

func TestDiffTable(t *testing.T) {
	base := "mode: set\n" +
		"github.com/go-gui-org/go-gui/gui/a.go:1.1,5.5 10 1\n" + // 100.0
		"github.com/go-gui-org/go-gui/gui/old/b.go:1.1,5.5 5 1\n" + // 50.0... continued below
		"github.com/go-gui-org/go-gui/gui/old/b.go:6.6,9.9 5 0\n" + // ...gone in cur
		"github.com/go-gui-org/go-gui/gui/drop/c.go:1.1,5.5 20 1\n" // 100.0 -> 90.0
	cur := "mode: set\n" +
		"github.com/go-gui-org/go-gui/gui/a.go:1.1,5.5 7 1\n" + // 70.0... continued below
		"github.com/go-gui-org/go-gui/gui/a.go:6.6,9.9 3 0\n" + // ...drop of 30
		"github.com/go-gui-org/go-gui/gui/drop/c.go:1.1,5.5 18 1\n" + // 90.0... continued below
		"github.com/go-gui-org/go-gui/gui/drop/c.go:6.6,9.9 2 0\n" +
		"github.com/go-gui-org/go-gui/gui/new/d.go:1.1,5.5 10 1\n" // new
	oldStdout := captureStdout(t, func() {
		if code := diff([]string{writeProfile(t, base), writeProfile(t, cur)}); code != 0 {
			t.Errorf("diff() = %d, want 0", code)
		}
	})
	for _, want := range []string{
		"## Coverage diff",
		"| Package | Baseline | PR | Δ |",
		"| github.com/go-gui-org/go-gui/gui | 100.0% | 70.0% | -30.0% |",
		"| github.com/go-gui-org/go-gui/gui/drop | 100.0% | 90.0% | -10.0% |",
		"| github.com/go-gui-org/go-gui/gui/new | — | 100.0% | new |",
		"| github.com/go-gui-org/go-gui/gui/old | 50.0% | — | removed |",
		"::warning::github.com/go-gui-org/go-gui/gui coverage dropped -30.0% (100.0% → 70.0%)",
		"2 package(s) with coverage drop > 2%.",
	} {
		if !strings.Contains(oldStdout, want) {
			t.Errorf("diff output missing %q\nfull output:\n%s", want, oldStdout)
		}
	}
	if strings.Contains(oldStdout, "drop | 100.0% | 90.0% | -10.0% |\"\n::warning::github.com/go-gui-org/go-gui/gui/drop") {
		t.Error("90% drop of 10 points must not warn (only the 30-point drop warns)")
	}
}

func TestDiffMissingBaseline(t *testing.T) {
	cur := writeProfile(t, testProfile)
	oldStdout := captureStdout(t, func() {
		if code := diff([]string{filepath.Join(t.TempDir(), "nope.out"), cur}); code != 0 {
			t.Errorf("diff() = %d, want 0", code)
		}
	})
	if !strings.Contains(oldStdout, "::warning::No baseline coverage file at ") {
		t.Errorf("diff output missing baseline warning:\n%s", oldStdout)
	}
}

func TestDiffSummaryFile(t *testing.T) {
	sum := filepath.Join(t.TempDir(), "summary.md")
	t.Setenv("GITHUB_STEP_SUMMARY", sum)
	base := writeProfile(t, testProfile)
	cur := writeProfile(t, testProfile)
	captureStdout(t, func() {
		if code := diff([]string{"--github-summary", base, cur}); code != 0 {
			t.Errorf("diff() = %d, want 0", code)
		}
	})
	b, err := os.ReadFile(sum)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "0 package(s) with coverage drop > 2%.") {
		t.Errorf("summary file missing footer:\n%s", b)
	}

	// Without the flag nothing is written even when the var is set.
	os.Remove(sum)
	captureStdout(t, func() { diff([]string{base, cur}) })
	if _, err := os.Stat(sum); !os.IsNotExist(err) {
		t.Error("summary file written without --github-summary")
	}
}
