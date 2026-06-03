package svg

import (
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

// makeAnimatedSquarePath builds a closed-rect VectorPath suitable for
// tessellation: opaque fill, identity transform, monotonic Animated.
func makeAnimatedSquarePath(id uint32) VectorPath {
	return VectorPath{
		PathID: id,
		Segments: []PathSegment{
			{Cmd: CmdMoveTo, Points: []float32{0, 0}},
			{Cmd: CmdLineTo, Points: []float32{10, 0}},
			{Cmd: CmdLineTo, Points: []float32{10, 10}},
			{Cmd: CmdLineTo, Points: []float32{0, 10}},
			{Cmd: CmdClose},
		},
		Transform: identityTransform,
		FillColor: gui.SvgColor{R: 255, A: 255},
		Animated:  true,
	}
}

// TessellateAnimated must include animated paths from FilteredGroups,
// not only from vg.Paths. Regression for the filtered-group sweep
// added in the CSS pipeline work.
func TestTessellateAnimated_FilteredGroupAnimatedPathIncluded(t *testing.T) {
	p := New()
	const fgPathID uint32 = 42
	vg := &VectorGraphic{
		Width: 100, Height: 100,
		FilteredGroups: []svgFilteredGroup{
			{Paths: []VectorPath{makeAnimatedSquarePath(fgPathID)}},
		},
	}
	parsed := p.buildParsed(99, "", vg, 1)
	overrides := map[uint32]gui.SvgAnimAttrOverride{
		fgPathID: {Mask: gui.SvgAnimMaskWidth, Width: 20},
	}
	got := p.TessellateAnimated(parsed, 1, overrides, nil)
	if len(got) == 0 {
		t.Fatal("expected tessellated triangles for animated filtered-group path")
	}
	found := false
	for _, tp := range got {
		if tp.PathID == fgPathID {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("PathID %d missing from tessellated output: %+v",
			fgPathID, idsOf(got))
	}
}

// Animated paths in BOTH main Paths and FilteredGroups must both
// surface in the result.
func TestTessellateAnimated_MainAndFilteredGroupBothIncluded(t *testing.T) {
	p := New()
	vg := &VectorGraphic{
		Width: 100, Height: 100,
		Paths: []VectorPath{makeAnimatedSquarePath(1)},
		FilteredGroups: []svgFilteredGroup{
			{Paths: []VectorPath{makeAnimatedSquarePath(2)}},
		},
	}
	parsed := p.buildParsed(101, "", vg, 1)
	overrides := map[uint32]gui.SvgAnimAttrOverride{
		1: {Mask: gui.SvgAnimMaskWidth, Width: 5},
		2: {Mask: gui.SvgAnimMaskWidth, Width: 5},
	}
	got := p.TessellateAnimated(parsed, 1, overrides, nil)
	ids := idsOf(got)
	hasOne, hasTwo := false, false
	for _, id := range ids {
		switch id {
		case 1:
			hasOne = true
		case 2:
			hasTwo = true
		}
	}
	if !hasOne || !hasTwo {
		t.Errorf("ids=%v want both 1 and 2 present", ids)
	}
}

// Clip-pathed animated paths must be skipped even when nested inside
// a filtered group; caller falls back to cached triangles for them.
func TestTessellateAnimated_FilteredGroupClipPathedSkipped(t *testing.T) {
	p := New()
	const id uint32 = 7
	clipped := makeAnimatedSquarePath(id)
	clipped.ClipPathID = "anyClip"
	vg := &VectorGraphic{
		Width: 100, Height: 100,
		FilteredGroups: []svgFilteredGroup{
			{Paths: []VectorPath{clipped}},
		},
	}
	parsed := p.buildParsed(102, "", vg, 1)
	overrides := map[uint32]gui.SvgAnimAttrOverride{
		id: {Mask: gui.SvgAnimMaskWidth, Width: 3},
	}
	got := p.TessellateAnimated(parsed, 1, overrides, nil)
	if got != nil {
		t.Errorf("expected nil result when only animated path is clip-pathed; got %v",
			idsOf(got))
	}
}

func idsOf(tps []gui.TessellatedPath) []uint32 {
	out := make([]uint32, 0, len(tps))
	for _, tp := range tps {
		out = append(out, tp.PathID)
	}
	return out
}
