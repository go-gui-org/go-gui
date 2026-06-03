package svg

import (
	"slices"
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

// TestResolveAnimationTargets_NestedGroupAnimReachesLeaf verifies that
// an animation bound to an outer group fans out to a leaf path whose
// own GroupID is a nested synth ID, via the GroupParent chain.
func TestResolveAnimationTargets_NestedGroupAnimReachesLeaf(t *testing.T) {
	vg := &VectorGraphic{
		Paths: []VectorPath{
			{PathID: 1, GroupID: "inner"},
			{PathID: 2, GroupID: "inner"},
		},
		GroupParent: map[string]string{
			"inner": "mid",
			"mid":   "outer",
		},
		Animations: []gui.SvgAnimation{
			{GroupID: "outer"},
			{GroupID: "mid"},
			{GroupID: "inner"},
		},
	}
	resolveAnimationTargets(vg)
	for i, a := range vg.Animations {
		got := append([]uint32(nil), a.TargetPathIDs...)
		slices.Sort(got)
		want := []uint32{1, 2}
		if !slices.Equal(got, want) {
			t.Errorf("anim[%d] GroupID=%s targets=%v want %v",
				i, a.GroupID, got, want)
		}
	}
}

// TestResolveAnimationTargets_GroupParentCycleDoesNotHang ensures the
// visited-set + depth-cap guards prevent infinite walks if the
// GroupParent map contains a cycle.
func TestResolveAnimationTargets_GroupParentCycleDoesNotHang(t *testing.T) {
	vg := &VectorGraphic{
		Paths: []VectorPath{
			{PathID: 7, GroupID: "a"},
		},
		GroupParent: map[string]string{
			"a": "b",
			"b": "c",
			"c": "a",
		},
		Animations: []gui.SvgAnimation{
			{GroupID: "a"}, {GroupID: "b"}, {GroupID: "c"},
		},
	}
	resolveAnimationTargets(vg)
	for i, a := range vg.Animations {
		if len(a.TargetPathIDs) != 1 || a.TargetPathIDs[0] != 7 {
			t.Errorf("anim[%d] gid=%s targets=%v want [7]",
				i, a.GroupID, a.TargetPathIDs)
		}
	}
}

// TestResolveAnimationTargets_SelfParentBreaks confirms a self-parent
// edge breaks immediately without duplication.
func TestResolveAnimationTargets_SelfParentBreaks(t *testing.T) {
	vg := &VectorGraphic{
		Paths: []VectorPath{
			{PathID: 5, GroupID: "loop"},
		},
		GroupParent: map[string]string{"loop": "loop"},
		Animations:  []gui.SvgAnimation{{GroupID: "loop"}},
	}
	resolveAnimationTargets(vg)
	got := vg.Animations[0].TargetPathIDs
	if len(got) != 1 || got[0] != 5 {
		t.Errorf("targets=%v want [5]", got)
	}
}

// TestResolveAnimationTargets_NilGroupParent ensures animations still
// bind to their direct GroupID when GroupParent is nil.
func TestResolveAnimationTargets_NilGroupParent(t *testing.T) {
	vg := &VectorGraphic{
		Paths: []VectorPath{
			{PathID: 11, GroupID: "g1"},
		},
		Animations: []gui.SvgAnimation{{GroupID: "g1"}},
	}
	resolveAnimationTargets(vg)
	got := vg.Animations[0].TargetPathIDs
	if len(got) != 1 || got[0] != 11 {
		t.Errorf("targets=%v want [11]", got)
	}
}

// TestResolveAnimationTargets_FilteredGroupPathParticipates verifies
// that paths inside FilteredGroups also contribute via the ancestor
// walk.
func TestResolveAnimationTargets_FilteredGroupPathParticipates(t *testing.T) {
	vg := &VectorGraphic{
		Paths: []VectorPath{
			{PathID: 1, GroupID: "leaf"},
		},
		FilteredGroups: []svgFilteredGroup{
			{Paths: []VectorPath{
				{PathID: 2, GroupID: "leaf"},
			}},
		},
		GroupParent: map[string]string{"leaf": "outer"},
		Animations:  []gui.SvgAnimation{{GroupID: "outer"}},
	}
	resolveAnimationTargets(vg)
	got := append([]uint32(nil), vg.Animations[0].TargetPathIDs...)
	slices.Sort(got)
	want := []uint32{1, 2}
	if !slices.Equal(got, want) {
		t.Errorf("targets=%v want %v", got, want)
	}
}

// TestResolveAnimationTargets_DepthCapStops constructs a chain longer
// than 64 and confirms the walk terminates without binding the path
// past the cap. The leaf must still bind to the immediate ancestors;
// groups beyond depth 64 must NOT contain the path.
func TestResolveAnimationTargets_DepthCapStops(t *testing.T) {
	parent := map[string]string{}
	prev := "g0"
	for i := 1; i < 80; i++ {
		cur := chainKey(i)
		parent[prev] = cur
		prev = cur
	}
	vg := &VectorGraphic{
		Paths: []VectorPath{
			{PathID: 9, GroupID: "g0"},
		},
		GroupParent: parent,
		Animations: []gui.SvgAnimation{
			{GroupID: "g0"},         // depth 0  — should bind
			{GroupID: chainKey(63)}, // depth 63 — should bind
			{GroupID: chainKey(70)}, // beyond cap — must NOT bind
		},
	}
	resolveAnimationTargets(vg)
	if len(vg.Animations[0].TargetPathIDs) != 1 {
		t.Errorf("depth-0 targets=%v want [9]", vg.Animations[0].TargetPathIDs)
	}
	if len(vg.Animations[1].TargetPathIDs) != 1 {
		t.Errorf("depth-63 targets=%v want [9]", vg.Animations[1].TargetPathIDs)
	}
	if len(vg.Animations[2].TargetPathIDs) != 0 {
		t.Errorf("beyond-cap targets=%v want empty",
			vg.Animations[2].TargetPathIDs)
	}
}

// TestResolveAnimationTargets_NoDuplicationUnderCycle ensures the
// visited set prevents one path being appended multiple times to the
// same group bucket when a cycle exists.
func TestResolveAnimationTargets_NoDuplicationUnderCycle(t *testing.T) {
	vg := &VectorGraphic{
		Paths: []VectorPath{
			{PathID: 4, GroupID: "x"},
		},
		GroupParent: map[string]string{
			"x": "y",
			"y": "x",
		},
		Animations: []gui.SvgAnimation{{GroupID: "x"}},
	}
	resolveAnimationTargets(vg)
	got := vg.Animations[0].TargetPathIDs
	if len(got) != 1 {
		t.Errorf("expected single PathID, got %v", got)
	}
}

func chainKey(i int) string {
	const digits = "0123456789"
	if i == 0 {
		return "g0"
	}
	out := []byte{'g'}
	var rev []byte
	for i > 0 {
		rev = append(rev, digits[i%10])
		i /= 10
	}
	for _, b := range slices.Backward(rev) {
		out = append(out, b)
	}
	return string(out)
}
