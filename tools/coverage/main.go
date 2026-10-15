// coverage enforces the coverage gates (gate) and reports per-package
// regressions between two profiles (diff).
//
// It replaces scripts/coverage-gate.sh, scripts/cov-diff.sh and
// scripts/pkgcov.awk (issue #1026), matching the scripts' messages, the
// ::error:: and ::warning:: annotations, the markdown table in
// $GITHUB_STEP_SUMMARY, and the exit codes on real inputs, so the
// Makefile targets and the CI coverage job keep working from `go run` on
// every OS, including Windows runners without bash, awk or join.
//
// Profiles are read with golang.org/x/tools/cover, not with text parsing.
// Per-package percentages are block-level (covered statements over all
// statements), the same definition as the total and as pkgcov.awk.
//
// One latent bug is fixed, not ported: the script computed per-package
// coverage from `go tool cover -func`, whose $2 column is the function
// name, not a statement count — every package scored 0.0%, so any floor
// above 0 would have failed unconditionally. All floors are 0 today, so
// no output changes on real inputs.
//
// Usage:
//
//	go run ./tools/coverage gate <profile>
//	go run ./tools/coverage diff [--github-summary] <baseline> <current>
package main

import (
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"

	"golang.org/x/tools/cover"
)

const (
	// totalThreshold is the floor for total coverage over the profile.
	// CI generates the profile with `go test ./gui/...`.
	totalThreshold = 70.0
	// regressionDelta is the per-package drop, in percentage points,
	// that counts as a regression in diff.
	regressionDelta = 2.0
)

// floors holds the per-package coverage floors: packages listed here
// must not drop under their floor. Floors are advisory for backend
// internals that CI runs but the host display rarely exercises.
var floors = []struct {
	pkg string
	min float64
}{
	{"github.com/go-gui-org/go-gui/gui/backend/internal/glyphconv", 0},
	{"github.com/go-gui-org/go-gui/gui/backend/internal/imgpath", 0},
	{"github.com/go-gui-org/go-gui/gui/backend/internal/texcache", 0},
	{"github.com/go-gui-org/go-gui/gui/backend/internal/imgload", 0},
	{"github.com/go-gui-org/go-gui/gui/backend/internal/tempfont", 0},
	{"github.com/go-gui-org/go-gui/gui/backend/internal/nativehost", 0},
	{"github.com/go-gui-org/go-gui/gui/backend/filedialog", 0},
}

func main() {
	if len(os.Args) < 3 || (os.Args[1] != "gate" && os.Args[1] != "diff") {
		fmt.Fprintln(os.Stderr, "usage: coverage gate <profile>")
		fmt.Fprintln(os.Stderr, "       coverage diff [--github-summary] <baseline> <current>")
		os.Exit(2)
	}
	var code int
	if os.Args[1] == "gate" {
		code = gate(os.Args[2:])
	} else {
		code = diff(os.Args[2:])
	}
	os.Exit(code)
}

// pkgCoverage aggregates a profile to per-package coverage percent,
// keyed by the block's directory (the pkgcov.awk grouping). Packages
// with no statements are skipped.
func pkgCoverage(profiles []*cover.Profile) map[string]float64 {
	type sums struct{ stmts, covered int64 }
	byPkg := make(map[string]*sums)
	for _, p := range profiles {
		pkg := p.FileName
		if i := strings.LastIndex(pkg, "/"); i > 0 {
			pkg = pkg[:i]
		}
		s := byPkg[pkg]
		if s == nil {
			s = &sums{}
			byPkg[pkg] = s
		}
		for _, b := range p.Blocks {
			s.stmts += int64(b.NumStmt)
			if b.Count > 0 {
				s.covered += int64(b.NumStmt)
			}
		}
	}
	out := make(map[string]float64, len(byPkg))
	for pkg, s := range byPkg {
		if s.stmts > 0 {
			out[pkg] = 100 * float64(s.covered) / float64(s.stmts)
		}
	}
	return out
}

// totalCoverage returns the total covered and statement counts and the
// percentage over the whole profile.
func totalCoverage(profiles []*cover.Profile) (covered, stmts int64, pct float64) {
	for _, p := range profiles {
		for _, b := range p.Blocks {
			stmts += int64(b.NumStmt)
			if b.Count > 0 {
				covered += int64(b.NumStmt)
			}
		}
	}
	if stmts > 0 {
		pct = 100 * float64(covered) / float64(stmts)
	}
	return covered, stmts, pct
}

func parseFile(path string) ([]*cover.Profile, error) {
	return cover.ParseProfiles(path)
}

// pkgPrefixCoverage sums statements and covered statements over every
// block whose file is pkg or under pkg/ — the script's "^pkg/" match on
// `go tool cover -func` lines, which rolls subpackages into their parent.
func pkgPrefixCoverage(profiles []*cover.Profile, pkg string) (stmts, covered int64) {
	prefix := pkg + "/"
	for _, p := range profiles {
		if p.FileName != pkg && !strings.HasPrefix(p.FileName, prefix) {
			continue
		}
		for _, b := range p.Blocks {
			stmts += int64(b.NumStmt)
			if b.Count > 0 {
				covered += int64(b.NumStmt)
			}
		}
	}
	return stmts, covered
}

// gate enforces the total threshold and the per-package floors on one
// profile. All output goes to stdout, like the script. Exit 0 when every
// gate passes, 1 otherwise.
func gate(args []string) int {
	profiles, err := parseFile(args[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "coverage gate: %v\n", err)
		return 1
	}
	_, _, pct := totalCoverage(profiles)
	if pct < totalThreshold {
		fmt.Printf("::error::Coverage %.1f%% below %g%% threshold\n", pct, totalThreshold)
		return 1
	}
	fmt.Printf("Coverage %.1f%% >= %g%% threshold\n", pct, totalThreshold)

	failures := 0
	for _, f := range floors {
		stmts, covered := pkgPrefixCoverage(profiles, f.pkg)
		var ppct float64
		if stmts > 0 {
			ppct = 100 * float64(covered) / float64(stmts)
		}
		if ppct < f.min {
			fmt.Printf("::error::%s coverage %.1f%% below %g%% floor\n", f.pkg, ppct, f.min)
			failures++
		}
	}
	if failures > 0 {
		fmt.Printf("::error::%d package(s) below coverage floor\n", failures)
		return 1
	}
	fmt.Println("All per-package coverage floors met.")
	return 0
}

// oneDecimal parses the %.1f rendering back to a float, mirroring how the
// script's awk computes the delta from the already-rounded table values.
func oneDecimal(f float64) float64 {
	v, _ := strconv.ParseFloat(fmt.Sprintf("%.1f", f), 64)
	return v
}

// diff compares two profiles package by package and reports regressions.
// With --github-summary the markdown table is also appended to
// $GITHUB_STEP_SUMMARY. Regressions are ::warning:: annotations only:
// diff always exits 0, like the script.
func diff(args []string) int {
	summary := false
	if len(args) > 0 && args[0] == "--github-summary" {
		summary = true
		args = args[1:]
	}
	if len(args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: coverage diff [--github-summary] <baseline> <current>")
		return 2
	}
	baseline, current := args[0], args[1]
	if _, err := os.Stat(baseline); err != nil {
		fmt.Printf("::warning::No baseline coverage file at %s\n", baseline)
		return 0
	}

	// An unreadable or unparsable profile reads as empty, the way the
	// script's awk-and-join pipeline treats it.
	baseCov := map[string]float64{}
	if profiles, err := parseFile(baseline); err == nil {
		baseCov = pkgCoverage(profiles)
	}
	curCov := map[string]float64{}
	if profiles, err := parseFile(current); err == nil {
		curCov = pkgCoverage(profiles)
	}

	emit := func(line string) {
		if summary {
			if sum := os.Getenv("GITHUB_STEP_SUMMARY"); sum != "" {
				// #nosec G304 — append target is the CI-provided step
				// summary path, not external input.
				f, err := os.OpenFile(sum, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
				if err == nil {
					_, _ = fmt.Fprintln(f, line)
					_ = f.Close()
				}
			}
		}
		fmt.Println(line)
	}

	keys := make([]string, 0, len(baseCov)+len(curCov))
	seen := make(map[string]bool)
	for pkg := range baseCov {
		keys = append(keys, pkg)
		seen[pkg] = true
	}
	for pkg := range curCov {
		if !seen[pkg] {
			keys = append(keys, pkg)
		}
	}
	slices.Sort(keys)

	emit("## Coverage diff")
	emit("")
	emit("| Package | Baseline | PR | Δ |")
	emit("|---------|----------|----|---|")

	regressions := 0
	for _, pkg := range keys {
		base, hasBase := baseCov[pkg]
		cur, hasCur := curCov[pkg]
		switch {
		case !hasBase:
			emit(fmt.Sprintf("| %s | — | %.1f%% | new |", pkg, cur))
		case !hasCur:
			emit(fmt.Sprintf("| %s | %.1f%% | — | removed |", pkg, base))
		default:
			b, c := oneDecimal(base), oneDecimal(cur)
			delta := oneDecimal(c - b)
			if delta < -regressionDelta {
				fmt.Printf("::warning::%s coverage dropped %.1f%% (%.1f%% → %.1f%%)\n",
					pkg, delta, b, c)
				regressions++
			}
			emit(fmt.Sprintf("| %s | %.1f%% | %.1f%% | %.1f%% |", pkg, b, c, delta))
		}
	}

	emit("")
	emit(fmt.Sprintf("%d package(s) with coverage drop > %g%%.", regressions, regressionDelta))
	return 0
}
