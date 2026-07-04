// depsdoc generates docs/dependencies.md from go.mod, keeping version
// numbers current while preserving human-written purpose and provenance
// annotations.
//
// Usage:
//
//	go run ./tools/depsdoc/ [-w]
//
// Without -w, output is written to stdout.  With -w, docs/dependencies.md
// is updated in place.
package main

import (
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"

	"golang.org/x/mod/modfile"
)

var write = flag.Bool("w", false, "write docs/dependencies.md in place")

func main() {
	flag.Parse()
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "depsdoc: %v\n", err)
		os.Exit(1)
	}
}

// directPurpose maps direct-dependency modules to a short purpose string.
var directPurpose = map[string]string{
	"github.com/go-gui-org/go-glyph":              "Text shaping + glyph rasterization. Required by every backend.",
	"github.com/go-gui-org/go-glyph/backend/sdl2": "SDL2-specific glyph rasterisation backend used by `gui/backend/sdl2`.",
	"github.com/veandco/go-sdl2":      "SDL2 backend (`gui/backend/sdl2`). Window, input, GL/Metal context glue.",
	"github.com/go-gl/gl":             "OpenGL bindings for `gui/backend/gl`.",
	"github.com/tdewolff/parse/v2":    "CSS tokenizer for the SVG `<style>` / `style=\"\"` cascade pipeline.",
	"github.com/alecthomas/chroma/v2": "Syntax highlighting in the markdown widget.",
	"github.com/yuin/goldmark":        "Markdown parser (markdown widget + showcase docs).",
	"github.com/yuin/goldmark-emoji":  "Goldmark extension: `:emoji:` shortcodes.",
	"github.com/go-pdf/fpdf":          "PDF generation for the print-dialog backend.",
	"github.com/godbus/dbus/v5":       "Linux native platform: notifications, portals.",
	"golang.org/x/mod":                "Module version parsing; imported by `requiredid` analyzer.",
	"golang.org/x/tools":              "`go/analysis` framework for the `requiredid` analyzer (`tools/`).",
}

// indirectPulledBy maps indirect-dependency modules to what pulls them in.
var indirectPulledBy = map[string]string{
	"github.com/dlclark/regexp2":    "chroma",
	"github.com/dlclark/regexp2/v2": "chroma",
	"github.com/rivo/uniseg":        "grapheme segmentation (chroma/glyph)",
	"golang.org/x/mobile":           "gobind tool (Android AAR builds)",
	"golang.org/x/sync":             "x/tools",
	"golang.org/x/sys":              "sdl2 / godbus",
	"golang.org/x/text":             "misc. text processing (transitive)",
}

func run() error {
	data, err := os.ReadFile("go.mod")
	if err != nil {
		return fmt.Errorf("reading go.mod: %w", err)
	}

	f, err := modfile.Parse("go.mod", data, nil)
	if err != nil {
		return fmt.Errorf("parsing go.mod: %w", err)
	}

	goVersion := ""
	if f.Go != nil {
		goVersion = f.Go.Version
	}

	extract := func(reqs []*modfile.Require) []modver {
		var out []modver
		for _, r := range reqs {
			v := r.Mod.Version
			if v == "" {
				v = "(pseudo)"
			}
			out = append(out, modver{path: r.Mod.Path, version: v})
		}
		sort.Slice(out, func(i, j int) bool { return out[i].path < out[j].path })
		return out
	}

	// Separate direct from indirect based on the Indirect flag set by the
	// modfile parser from // indirect comments.
	var directRaw, indirectRaw []*modfile.Require
	for _, r := range f.Require {
		if r.Indirect {
			indirectRaw = append(indirectRaw, r)
		} else {
			directRaw = append(directRaw, r)
		}
	}
	direct := extract(directRaw)
	indirect := extract(indirectRaw)

	var b strings.Builder

	b.WriteString("# Dependencies\n\n")
	b.WriteString("Inventory of every Go module pulled in by go-gui. Source of truth:\n")
	b.WriteString("[`go.mod`](../go.mod) (declarations) and [`go.sum`](../go.sum)\n")
	b.WriteString("(content checksums).\n\n")
	fmt.Fprintf(&b, "Go toolchain pin: `go %s`.\n\n", goVersion)

	// Direct deps table.
	b.WriteString("## Direct Dependencies\n\n")
	b.WriteString("| Module | Version | Purpose |\n")
	b.WriteString("| ------ | ------- | ------- |\n")
	for _, m := range direct {
		purpose, ok := directPurpose[m.path]
		if !ok {
			purpose = "⚠ UNKNOWN — update directPurpose map in tools/depsdoc/main.go"
		}
		fmt.Fprintf(&b, "| `%s` | %s | %s |\n", m.path, m.version, purpose)
	}

	// Indirect deps table.
	b.WriteString("\n## Indirect Dependencies\n\n")
	b.WriteString("Pulled in transitively; listed for completeness.\n\n")
	b.WriteString("| Module | Version | Pulled in by |\n")
	b.WriteString("| ------ | ------- | ------------ |\n")
	for _, m := range indirect {
		pulledBy, ok := indirectPulledBy[m.path]
		if !ok {
			pulledBy = "⚠ UNKNOWN — update indirectPulledBy map in tools/depsdoc/main.go"
		}
		fmt.Fprintf(&b, "| `%s` | %s | %s |\n", m.path, m.version, pulledBy)
	}

	// Static sections.
	b.WriteString("\n## Updating\n\n")
	b.WriteString("```bash\n")
	b.WriteString("go get <module>@<version>\n")
	b.WriteString("go mod tidy\n")
	b.WriteString("```\n\n")
	b.WriteString("Example:\n\n")
	b.WriteString("```bash\n")
	b.WriteString("go get github.com/go-gui-org/go-glyph@v1.9.0\n")
	b.WriteString("go mod tidy\n")
	b.WriteString("```\n\n")
	b.WriteString("After updating dependencies, regenerate this file:\n\n")
	b.WriteString("```bash\n")
	b.WriteString("make deps-doc\n")
	b.WriteString("```\n\n")

	b.WriteString("## Local go-glyph Workflow\n\n")
	b.WriteString("Day-to-day text work often runs against a sibling checkout at\n")
	b.WriteString("`~/Documents/github/go-glyph`. Wire it in via a `replace` directive:\n\n")
	b.WriteString("```bash\n")
	b.WriteString("go mod edit -replace github.com/go-gui-org/go-glyph=../go-glyph\n")
	b.WriteString("```\n\n")
	b.WriteString("Drop the replace before tagging:\n\n")
	b.WriteString("```bash\n")
	b.WriteString("go mod edit -dropreplace github.com/go-gui-org/go-glyph\n")
	b.WriteString("go mod tidy\n")
	b.WriteString("```\n\n")
	b.WriteString("`go.mod` on `main` must not carry a local replace — CI fetches the\n")
	b.WriteString("tagged module.\n\n")

	b.WriteString("## Verification\n\n")
	b.WriteString("Pre-commit checks (PostToolUse hook runs lint-fix + tests on every\n")
	b.WriteString("`.go` edit; reproduce manually with):\n\n")
	b.WriteString("```bash\n")
	b.WriteString("go build ./...\n")
	b.WriteString("go vet ./...\n")
	b.WriteString("golangci-lint run ./...\n")
	b.WriteString("go test ./...\n")
	b.WriteString("```\n\n")
	b.WriteString("Commit `go.mod` and `go.sum` together whenever either changes.\n")

	// Check for unknown modules.
	unknown := false
	for _, m := range direct {
		if _, ok := directPurpose[m.path]; !ok {
			fmt.Fprintf(os.Stderr, "depsdoc: WARNING: direct module %q not in directPurpose map\n", m.path)
			unknown = true
		}
	}
	for _, m := range indirect {
		if _, ok := indirectPulledBy[m.path]; !ok {
			fmt.Fprintf(os.Stderr, "depsdoc: WARNING: indirect module %q not in indirectPulledBy map\n", m.path)
			unknown = true
		}
	}

	out := b.String()
	if *write {
		// #nosec G306 — standard 0644 for generated docs
		if err := os.WriteFile("docs/dependencies.md", []byte(out), 0o644); err != nil {
			return fmt.Errorf("writing docs/dependencies.md: %w", err)
		}
		fmt.Println("docs/dependencies.md updated.")
	} else {
		fmt.Print(out)
	}

	if unknown {
		fmt.Fprintln(os.Stderr, "\ndepsdoc: update the maps in tools/depsdoc/main.go to resolve warnings above.")
	}

	return nil
}

type modver struct {
	path, version string
}
