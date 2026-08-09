package gui

import "testing"

// filteredAnimSvgParser hands LoadSvg a parsed graphic whose only
// Animated path lives inside a FilteredGroup. Used to verify that
// LoadSvg's hasAnimatedPaths probe also walks filtered groups.
type filteredAnimSvgParser struct {
	width, height float32
	includeAnim   bool
}

func (m *filteredAnimSvgParser) ParseSvg(_ string) (*SvgParsed, error) {
	return m.build(), nil
}

func (m *filteredAnimSvgParser) ParseSvgFile(_ string) (*SvgParsed, error) {
	return m.build(), nil
}

func (m *filteredAnimSvgParser) ParseSvgDimensions(_ string) (
	float32, float32, error,
) {
	return m.width, m.height, nil
}

func (m *filteredAnimSvgParser) Tessellate(_ *SvgParsed, _ float32) []TessellatedPath {
	return nil
}

func (m *filteredAnimSvgParser) build() *SvgParsed {
	parsed := &SvgParsed{Width: m.width, Height: m.height}
	if m.includeAnim {
		parsed.FilteredGroups = []SvgParsedFilteredGroup{{
			Paths: []TessellatedPath{{
				Triangles: []float32{0, 0, 1, 0, 0.5, 1},
				Color:     SvgColor{R: 255, A: 255},
				PathID:    1,
				Animated:  true,
			}},
		}}
	}
	return parsed
}

// LoadSvg must flip HasAnimatedPaths=true when the only Animated path
// is nested inside a FilteredGroup, so the render loop schedules a
// re-tessellation pass for it.
func TestLoadSvg_AnimatedPathInFilteredGroupTriggersAnimation(t *testing.T) {
	w := NewWindow(WindowCfg{})
	w.SetSvgParser(&filteredAnimSvgParser{
		width: 24, height: 24, includeAnim: true,
	})
	cached, err := w.LoadSvg("<svg/>", 24, 24)
	if err != nil {
		t.Fatalf("LoadSvg: %v", err)
	}
	if !cached.hasAnimatedPaths {
		t.Fatal("HasAnimatedPaths=false; expected true when only filtered " +
			"group carries an Animated path")
	}
}

// Without any Animated path (main or filtered), HasAnimatedPaths must
// stay false. Pins the negative case so the new branch can't false-
// positive when filtered groups exist but none animate.
func TestLoadSvg_NoAnimatedPathsLeavesFlagFalse(t *testing.T) {
	w := NewWindow(WindowCfg{})
	w.SetSvgParser(&filteredAnimSvgParser{
		width: 24, height: 24, includeAnim: false,
	})
	cached, err := w.LoadSvg("<svg/>", 24, 24)
	if err != nil {
		t.Fatalf("LoadSvg: %v", err)
	}
	if cached.hasAnimatedPaths {
		t.Fatal("HasAnimatedPaths=true on a non-animated graphic")
	}
}
