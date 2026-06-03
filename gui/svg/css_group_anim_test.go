package svg

import (
	"slices"
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

// CSS animation declared on a <g> wrapper must fan out onto every
// descendant primitive path, not bind only to the synthesized
// group-id. Regression for the GroupParent-chain walk added to
// resolveAnimationTargets.
func TestParseSvg_CSSAnimOnGroupFansToChildren(t *testing.T) {
	src := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100">
		<style>
			@keyframes spin {
				from { transform: rotate(0deg) }
				to   { transform: rotate(360deg) }
			}
			.gr { animation: spin 1s linear }
		</style>
		<g class="gr">
			<rect x="0"  y="0" width="10" height="10"/>
			<rect x="20" y="0" width="10" height="10"/>
			<rect x="40" y="0" width="10" height="10"/>
		</g>
	</svg>`
	parsed, err := New().ParseSvg(src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if len(parsed.Paths) != 3 {
		t.Fatalf("paths=%d want 3", len(parsed.Paths))
	}
	wantIDs := []uint32{
		parsed.Paths[0].PathID, parsed.Paths[1].PathID, parsed.Paths[2].PathID,
	}
	for _, id := range wantIDs {
		if id == 0 {
			t.Fatalf("zero PathID in %v", wantIDs)
		}
	}

	var rot *gui.SvgAnimation
	for i := range parsed.Animations {
		if parsed.Animations[i].Kind == gui.SvgAnimRotate {
			rot = &parsed.Animations[i]
			break
		}
	}
	if rot == nil {
		t.Fatal("no rotate animation compiled from group-level CSS anim")
	}
	got := append([]uint32(nil), rot.TargetPathIDs...)
	slices.Sort(got)
	slices.Sort(wantIDs)
	if !slices.Equal(got, wantIDs) {
		t.Errorf("TargetPathIDs=%v want %v (every descendant rect)",
			got, wantIDs)
	}
}

// Nested groups: outer carries the CSS animation; inner is a passive
// wrapper. Descendant rect must still bind to outer's animation.
func TestParseSvg_CSSAnimOnOuterGroupReachesNestedDescendants(t *testing.T) {
	src := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100">
		<style>
			@keyframes spin {
				from { transform: rotate(0deg) }
				to   { transform: rotate(360deg) }
			}
			.outer { animation: spin 1s linear }
		</style>
		<g class="outer">
			<g>
				<rect x="0" y="0" width="10" height="10"/>
			</g>
		</g>
	</svg>`
	parsed, err := New().ParseSvg(src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if len(parsed.Paths) != 1 {
		t.Fatalf("paths=%d want 1", len(parsed.Paths))
	}
	want := parsed.Paths[0].PathID
	var rot *gui.SvgAnimation
	for i := range parsed.Animations {
		if parsed.Animations[i].Kind == gui.SvgAnimRotate {
			rot = &parsed.Animations[i]
			break
		}
	}
	if rot == nil {
		t.Fatal("no rotate animation")
	}
	if len(rot.TargetPathIDs) != 1 || rot.TargetPathIDs[0] != want {
		t.Errorf("TargetPathIDs=%v want [%d]", rot.TargetPathIDs, want)
	}
}

// CSS-anim group with no descendant primitives must not panic and
// must not emit phantom animations. Guards the empty-paths slice
// branch in parseSvgContent.
func TestParseSvg_CSSAnimOnEmptyGroupNoPanic(t *testing.T) {
	src := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100">
		<style>
			@keyframes spin { from {transform:rotate(0)} to {transform:rotate(360deg)} }
			.empty { animation: spin 1s linear }
		</style>
		<g class="empty"></g>
	</svg>`
	parsed, err := New().ParseSvg(src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	for _, a := range parsed.Animations {
		if a.Kind == gui.SvgAnimRotate {
			t.Errorf("unexpected rotate anim from empty group: %+v", a)
		}
	}
}
