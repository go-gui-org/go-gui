package main

import (
	"os"
	"strings"
	"testing"
)

// The "Sound Feedback" guide quotes examples/showcase/sound_player.go whole
// rather than paraphrasing it, so a reader can paste the block and have it
// compile. Nothing enforces that on its own: the guide is markdown and the
// player is Go, and an edit to either is invisible to the other. This test is
// the enforcement. It fails when the marked region and the quoted block stop
// being byte-identical, in whichever direction the drift happened.

const (
	snippetSourceFile = "sound_player.go"
	snippetBeginMark  = "// doc:snippet-begin player"
	snippetEndMark    = "// doc:snippet-end player"
)

// soundGuides lists every copy of the guide. The repo keeps a mirror under
// docs/ as well as the showcase's embedded copy, so both are checked; a fix
// that lands in one and not the other is exactly the drift this catches.
var soundGuides = []string{
	"docs/widget_sound.md",
	"../../docs/widget-sound.md",
}

// markedRegion returns the source between the snippet markers, minus the
// marker's own explanatory comment block (which ends at the first blank
// line) and minus surrounding blank lines.
func markedRegion(t *testing.T, src string) string {
	t.Helper()
	if strings.Count(src, snippetBeginMark) != 1 ||
		strings.Count(src, snippetEndMark) != 1 {
		t.Fatalf("%s must contain exactly one %q and one %q",
			snippetSourceFile, snippetBeginMark, snippetEndMark)
	}
	_, rest, _ := strings.Cut(src, snippetBeginMark)
	body, _, _ := strings.Cut(rest, snippetEndMark)
	_, code, found := strings.Cut(body, "\n\n")
	if !found {
		t.Fatalf("%s: no blank line after the snippet marker comment",
			snippetSourceFile)
	}
	return strings.Trim(code, "\n")
}

// guideSnippet returns the first fenced Go block that follows the "A real
// player" heading. Keyed on the heading rather than on block order so that
// adding a snippet earlier in the guide does not silently retarget the test.
func guideSnippet(t *testing.T, guide, doc string) string {
	t.Helper()
	const heading = "## A real player"
	_, rest, found := strings.Cut(doc, heading)
	if !found {
		t.Fatalf("%s: no %q heading; the guide was restructured and this "+
			"test needs retargeting", guide, heading)
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

func TestSoundGuideSnippetMatchesSource(t *testing.T) {
	raw, err := os.ReadFile(snippetSourceFile)
	if err != nil {
		t.Fatalf("read %s: %v", snippetSourceFile, err)
	}
	want := markedRegion(t, string(raw))
	if !strings.Contains(want, "func (p cueSoundPlayer) PlaySound(") {
		t.Fatalf("%s: marked region no longer contains PlaySound; the "+
			"markers have drifted off the player", snippetSourceFile)
	}

	for _, guide := range soundGuides {
		doc, errRead := os.ReadFile(guide)
		if errRead != nil {
			t.Errorf("read %s: %v", guide, errRead)
			continue
		}
		got := guideSnippet(t, guide, string(doc))
		if got == want {
			continue
		}
		// Report the first differing line rather than dumping ~50 lines
		// twice: the fix is always to copy one side over the other.
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
				"fix: copy the region between the %q markers into the "+
				"guide's Go fence verbatim",
				guide, snippetSourceFile, i+1, g, w, snippetBeginMark)
			break
		}
	}
}
