package main

import "testing"

func TestPackagingChanged(t *testing.T) {
	changed := []string{
		"Makefile",
		"gui/Makefile",
		".github/workflows/release.yml",
		".github/workflows/ci.yml",
		"scripts/install-deps.sh",
		"scripts/install-deps.sh.bak",
		"scripts/coverage-gate.sh",
	}
	got := packagingChanged(changed)
	want := []string{"Makefile", ".github/workflows/release.yml", "scripts/install-deps.sh"}
	if len(got) != len(want) {
		t.Fatalf("packagingChanged() = %q, want %q", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("packagingChanged()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestGoSources(t *testing.T) {
	got := goSources([]string{"a.go", "a_test.go", "b.md", "dir/c.go", "d.gox"})
	if len(got) != 2 || got[0] != "a.go" || got[1] != "dir/c.go" {
		t.Errorf("goSources() = %q, want [a.go dir/c.go]", got)
	}
}

func TestCountExported(t *testing.T) {
	cases := []struct {
		name string
		diff string
		want int
	}{
		{"func", "+func Foo() {}", 1},
		{"removed func", "-func Foo() {}", 1},
		{"method", "+func (w *Window) Foo() {}", 1},
		{"unexported method", "+func (w *Window) foo() {}", 0},
		{"type", "+type Foo struct{}", 1},
		{"const", "+const Foo = 1", 1},
		{"var", "+var Foo int", 1},
		{"field", "+\tFoo int", 1},
		{"field in block", "+\tFoo  = 1", 1},
		{"assignment", "+Foo =1", 0},
		{"unexported", "+func foo() {}", 0},
		{"comment", "+// Foo does things", 0},
		{"indented comment", "+\t// Foo does things", 0},
		{"context line", " func Foo() {}", 0},
		{"file header", "+++ b/gui/Foo.go", 0},
		{"old header", "--- a/gui/Foo.go", 0},
		{"plus-minus", "+-Foo bar", 0},
		{"bare plus", "+", 0},
		{"call", "+\tFoo(x)", 0},
		{"several", "+func Foo() {}\n-type Bar int\n+x := 1\n+\tBaz string", 3},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := countExported(tc.diff); got != tc.want {
				t.Errorf("countExported(%q) = %d, want %d", tc.diff, got, tc.want)
			}
		})
	}
}

func TestSkipRequested(t *testing.T) {
	cases := []struct {
		log  string
		want bool
	}{
		{"fix: thing\n\nchangelog: skip\n", true},
		{"fix: thing\n\n  Changelog:Skip\n", true},
		{"changelog: skipped tests\n", true},
		{"fix: thing\n\nno changelog: skip here\n", false},
		{"fix: thing\n", false},
	}
	for _, tc := range cases {
		if got := skipRequested(tc.log); got != tc.want {
			t.Errorf("skipRequested(%q) = %v, want %v", tc.log, got, tc.want)
		}
	}
}

func TestUnreleasedHasEntry(t *testing.T) {
	cases := []struct {
		name string
		text string
		want bool
	}{
		{"bullet", "## [Unreleased]\n\n### Fixed\n\n- **Fix.**\n\n## [v1.0.0]\n\n- old\n", true},
		{"indented bullet", "## [Unreleased]\n\n  - x\n", true},
		{"heading only", "## [Unreleased]\n\n### Added\n\n## [v1.0.0]\n\n- old\n", false},
		{"bare dash", "## [Unreleased]\n\n-\n- \n", false},
		{"dash without space", "## [Unreleased]\n\n-x\n", false},
		{"no unreleased", "## [v1.0.0]\n\n- old\n", false},
		{"bullet before unreleased", "- top\n\n## [Unreleased]\n\n## [v1.0.0]\n", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := unreleasedHasEntry(tc.text); got != tc.want {
				t.Errorf("unreleasedHasEntry() = %v, want %v", got, tc.want)
			}
		})
	}
}
