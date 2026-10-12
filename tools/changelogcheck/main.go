// changelogcheck gates the Keep-a-Changelog shape of CHANGELOG.md. It fails
// if a breaking API change in the newest released block is documented outside
// ### Changed with a "- **BREAKING:" prefix.
//
// This is the fix for v0.68.0, where ScrollbarCfg.GapEdge/GapEnd changed from
// float32 to Opt[float32] under ### Added as "breaking for..." with no BREAKING
// label, so scanners and reviewers missed it.
//
// Usage, from the repo root:
//
//	go run ./tools/changelogcheck/
package main

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

const path = "CHANGELOG.md"

var (
	versionHeading = regexp.MustCompile(`(?m)^## \[`)
	breakingWord   = regexp.MustCompile(`(?i)breaking`)
	breakingLine   = regexp.MustCompile(`(?m)^- \*\*BREAKING:`)
	optChange      = regexp.MustCompile(`Opt\[|are now.*Opt`)
)

func main() {
	b, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "changelog-check: %s not found\n", path)
		return
	}
	errs := check(string(b))
	for _, e := range errs {
		fmt.Fprintf(os.Stderr, "::error::%s\n%s\n", e, e)
	}
	if len(errs) != 0 {
		os.Exit(1)
	}
	fmt.Println("changelog-check: ok")
}

// check returns the shape errors in the newest released version block.
// Unreleased and older blocks are not gated: historical entries are
// grandfathered, since the repo had several pre-gate breaking notes under
// Added (e.g., v0.59.0 DataGridCfg.Sizing).
func check(text string) []string {
	for _, raw := range versionHeading.Split(text, -1)[1:] {
		// raw starts with e.g. "v0.68.0] - 2026-09-05\n\n### Added\n...".
		end := strings.IndexByte(raw, ']')
		if end <= 0 {
			continue
		}
		fields := strings.Fields(raw[:end])
		if len(fields) == 0 {
			continue
		}
		if fields[0] == "Unreleased" {
			continue
		}
		return checkBlock(fields[0], raw[end+1:])
	}
	return nil
}

func checkBlock(ver, body string) []string {
	var errs []string

	// Rule 1: Added must not contain the word "breaking" (case-insensitive) as
	// a description of an API break. A breaking change belongs under Changed
	// with **BREAKING: . The one legitimate non-breaking use ("non-breaking")
	// is rare and can be reworded; flagging it is safer than missing a real
	// break.
	if breakingWord.MatchString(section(body, "Added")) {
		errs = append(errs, fmt.Sprintf("%s: 'breaking' found under ### Added "+
			"(move to ### Changed as '- **BREAKING: ...'): %s:%s", ver, path, ver))
	}

	// Rule 2: if the body mentions an Opt change (the v0.68.0 case, where
	// Added said "are now `Opt[float32]` — breaking for..."), Changed must
	// contain a BREAKING line.
	if optChange.MatchString(body) && !breakingLine.MatchString(section(body, "Changed")) {
		errs = append(errs, fmt.Sprintf("%s: body mentions Opt[ change but "+
			"### Changed has no '- **BREAKING:' line (%s)", ver, path))
	}

	// Rule 3: a BREAKING marker belongs only under Changed.
	for _, name := range []string{"Added", "Fixed", "Removed", "Deprecated", "Security"} {
		if breakingLine.MatchString(section(body, name)) {
			errs = append(errs, fmt.Sprintf("%s: '- **BREAKING:' found under "+
				"### %s (must be under ### Changed)", ver, name))
		}
	}
	return errs
}

// section returns the lines after the first "### <name>" heading up to the
// next "### " heading or the end of body. The heading line may carry trailing
// whitespace but nothing else.
func section(body, name string) string {
	heading := "### " + name
	rest := body
	for rest != "" {
		line, after, ok := strings.Cut(rest, "\n")
		if !ok {
			return "" // heading must end in a newline
		}
		rest = after
		if strings.TrimRight(line, " \t\r\f\v") != heading {
			continue
		}
		end := len(rest)
		if strings.HasPrefix(rest, "### ") {
			end = 0
		} else if i := strings.Index(rest, "\n### "); i >= 0 {
			end = i + 1
		}
		return rest[:end]
	}
	return ""
}
