package main

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// demoStackCards is a copy of stage() in examples/stack/main.go. Both
// are main packages, so they cannot share one builder. This test keeps
// the copies in step: each line below sets the card geometry, and both
// files must contain it unchanged. A change to one copy alone fails
// here, so the fix lands in both (and in this list). Runs of spaces
// are collapsed before the compare, so a gofmt realignment of a Cfg
// literal is not a false failure.
func TestStackCardsMatchStackExample(t *testing.T) {
	lines := []string{
		"Padding:    gui.NewPadding(step*50, 0, 0, step*80),",
		"Width:       170,",
		"Height:      110,",
		"ColorBorder: gui.White,",
		"Radius:      gui.RadiusMedium,",
		"Padding:     gui.PaddingMedium,",
		"Spacing:     gui.SpacingSmall,",
		"Width:   330,",
		"Height:  210,",
	}
	spaces := regexp.MustCompile(` +`)
	for _, file := range []string{"demo_stack.go", "../stack/main.go"} {
		src, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		flat := spaces.ReplaceAllString(string(src), " ")
		for _, line := range lines {
			if !strings.Contains(flat, spaces.ReplaceAllString(line, " ")) {
				t.Errorf("%s: missing %q; change both Stack card copies together", file, line)
			}
		}
	}
}
