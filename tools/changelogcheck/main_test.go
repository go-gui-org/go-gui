package main

import (
	"os"
	"strings"
	"testing"
)

func TestCheck(t *testing.T) {
	cases := []struct {
		name string
		text string
		want []string // substrings, one per expected error
	}{
		{
			name: "clean",
			text: "# Changelog\n\n## [Unreleased]\n\n## [v1.0.0] - 2026-01-01\n\n" +
				"### Added\n\n- **Thing.**\n\n### Changed\n\n- **BREAKING: Foo (#1)**\n",
		},
		{
			name: "breaking under added",
			text: "## [v1.0.0] - 2026-01-01\n\n### Added\n\n- Breaking for apps.\n",
			want: []string{"v1.0.0: 'breaking' found under ### Added"},
		},
		{
			name: "marker under fixed",
			text: "## [v1.0.0]\n\n### Fixed\n\n- **BREAKING: Foo**\n",
			want: []string{"v1.0.0: '- **BREAKING:' found under ### Fixed"},
		},
		{
			name: "opt change without breaking line",
			text: "## [v1.0.0]\n\n### Changed\n\n- Gap is now `Opt[float32]`.\n",
			want: []string{"v1.0.0: body mentions Opt[ change"},
		},
		{
			name: "are now opt without breaking line",
			text: "## [v1.0.0]\n\n### Changed\n\n- Fields are now optional Opt values.\n",
			want: []string{"v1.0.0: body mentions Opt[ change"},
		},
		{
			name: "opt change with breaking line",
			text: "## [v1.0.0]\n\n### Changed\n\n- **BREAKING: Gap is now `Opt[float32]`.**\n",
		},
		{
			name: "breaking line in another section does not satisfy opt rule",
			text: "## [v1.0.0]\n\n### Added\n\n- Opt[int] field.\n\n### Changed\n\n" +
				"- Plain.\n\n### Fixed\n\n- **BREAKING: x**\n",
			want: []string{
				"v1.0.0: body mentions Opt[ change",
				"v1.0.0: '- **BREAKING:' found under ### Fixed",
			},
		},
		{
			name: "unreleased not gated",
			text: "## [Unreleased]\n\n### Added\n\n- Breaking.\n\n## [v1.0.0]\n\n### Fixed\n\n- ok\n",
		},
		{
			name: "older release not gated",
			text: "## [v2.0.0]\n\n### Fixed\n\n- ok\n\n## [v1.0.0]\n\n### Added\n\n- Breaking.\n",
		},
		{
			name: "only first matching section read",
			text: "## [v1.0.0]\n\n### Added\n\n- ok\n\n### Fixed\n\n- ok\n\n### Added\n\n- Breaking.\n",
		},
		{
			name: "section heading needs exact name",
			text: "## [v1.0.0]\n\n### Added things\n\n- Breaking.\n",
		},
		{
			name: "no release blocks",
			text: "# Changelog\n\n## [Unreleased]\n\n### Added\n\n- Breaking.\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := check(tc.text)
			if len(got) != len(tc.want) {
				t.Fatalf("check() = %q, want %d errors", got, len(tc.want))
			}
			for i, w := range tc.want {
				if !strings.Contains(got[i], w) {
					t.Errorf("error %d = %q, want it to contain %q", i, got[i], w)
				}
			}
		})
	}
}

func TestRepoChangelogPasses(t *testing.T) {
	b, err := os.ReadFile("../../CHANGELOG.md")
	if err != nil {
		t.Fatal(err)
	}
	if errs := check(string(b)); len(errs) != 0 {
		t.Errorf("CHANGELOG.md: %q", errs)
	}
}
