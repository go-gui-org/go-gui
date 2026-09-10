package main

import (
	"os"
	"strings"
	"testing"
)

// Guides quote Go sources so a reader can paste the block and have
// it compile. Nothing enforces that on its own: a guide is markdown
// and the source is Go, and an edit to either is invisible to the
// other. This test is the enforcement. It fails when a marked region
// and its quoted block stop being byte-identical, in whichever
// direction the drift happened.

// snippetEntry pins one marked region to every guide quoting it.
// The repo keeps mirrors of some guides (showcase embedded copies
// plus docs/), so each entry lists all of them; a fix that lands in
// one and not the other is exactly the drift this catches.
type snippetEntry struct {
	sourceFile string
	beginMark  string
	endMark    string
	// anchor is a substring the marked region must contain. It
	// catches the markers drifting off the code they pin.
	anchor string
	// heading keys the quoted fence in each guide, so adding a
	// snippet earlier in a guide does not silently retarget this.
	heading string
	guides  []string
}

var snippetEntries = []snippetEntry{
	{
		sourceFile: "sound_player.go",
		beginMark:  "// doc:snippet-begin player",
		endMark:    "// doc:snippet-end player",
		anchor:     "func (p cueSoundPlayer) PlaySound(",
		heading:    "## A real player",
		guides: []string{
			"docs/widget_sound.md",
			"../../docs/widget-sound.md",
		},
	},
	{
		sourceFile: "../../gui/window_stream_example_test.go",
		beginMark:  "// doc:snippet-begin stream",
		endMark:    "// doc:snippet-end stream",
		anchor:     "func ExampleStream()",
		heading:    "## Streaming background data into a window",
		guides: []string{
			"../../docs/dx-cheat-sheet.md",
		},
	},
}

// markedRegion returns the source between the snippet markers, minus
// the marker's own explanatory comment block (which ends at the
// first blank line) and minus surrounding blank lines.
func markedRegion(
	t *testing.T,
	src, file, beginMark, endMark string,
) string {
	t.Helper()
	if strings.Count(src, beginMark) != 1 ||
		strings.Count(src, endMark) != 1 {
		t.Fatalf("%s must contain exactly one %q and one %q",
			file, beginMark, endMark)
	}
	_, rest, _ := strings.Cut(src, beginMark)
	body, _, _ := strings.Cut(rest, endMark)
	_, code, found := strings.Cut(body, "\n\n")
	if !found {
		t.Fatalf("%s: no blank line after the snippet marker comment",
			file)
	}
	return strings.Trim(code, "\n")
}

// guideSnippet returns the first fenced Go block that follows the
// entry's heading. Keyed on the heading rather than on block order.
func guideSnippet(
	t *testing.T,
	guide, doc, heading string,
) string {
	t.Helper()
	_, rest, found := strings.Cut(doc, heading)
	if !found {
		t.Fatalf("%s: no %q heading; the guide was restructured and "+
			"this test needs retargeting", guide, heading)
	}
	_, rest, found = strings.Cut(rest, "```go\n")
	if !found {
		t.Fatalf("%s: no Go code fence under %q", guide, heading)
	}
	block, _, found := strings.Cut(rest, "\n```")
	if !found {
		t.Fatalf("%s: unterminated Go code fence under %q", guide, heading)
	}
	return strings.Trim(block, "\n")
}

func TestGuideSnippetsMatchSource(t *testing.T) {
	for _, e := range snippetEntries {
		raw, err := os.ReadFile(e.sourceFile)
		if err != nil {
			t.Fatalf("read %s: %v", e.sourceFile, err)
		}
		want := markedRegion(t, string(raw), e.sourceFile,
			e.beginMark, e.endMark)
		if !strings.Contains(want, e.anchor) {
			t.Fatalf("%s: marked region no longer contains %q; the "+
				"markers have drifted off the code they pin",
				e.sourceFile, e.anchor)
		}

		for _, guide := range e.guides {
			doc, errRead := os.ReadFile(guide)
			if errRead != nil {
				t.Errorf("read %s: %v", guide, errRead)
				continue
			}
			got := guideSnippet(t, guide, string(doc), e.heading)
			if got == want {
				continue
			}
			// Report the first differing line rather than dumping
			// both blocks twice: the fix is always to copy one
			// side over the other.
			gotLines := strings.Split(got, "\n")
			wantLines := strings.Split(want, "\n")
			for i := 0; i < len(gotLines) || i < len(wantLines); i++ {
				g, w := "<missing>", "<missing>"
				if i < len(gotLines) {
					g = gotLines[i]
				}
				if i < len(wantLines) {
					w = wantLines[i]
				}
				if g == w {
					continue
				}
				t.Errorf("%s: snippet drifted from %s at block line %d\n"+
					"  guide:  %s\n  source: %s\n"+
					"fix: copy the region between the %q markers into "+
					"the guide's Go fence verbatim",
					guide, e.sourceFile, i+1, g, w, e.beginMark)
				break
			}
		}
	}
}
