package svg

import (
	"testing"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/svg/css"
)

// Phase D end-to-end: @keyframes parsing, animation-* CSS props,
// compile to SvgAnimation records, color tween, alternate / fill-
// backwards flags, compound transform split.

func TestPhaseD_KeyframesParse_FromTo(t *testing.T) {
	sheet := css.ParseFull(`
		@keyframes spin {
			from { transform: rotate(0deg) }
			to   { transform: rotate(360deg) }
		}
	`, css.ParseOptions{})
	if len(sheet.Keyframes) != 1 {
		t.Fatalf("keyframes count: %d", len(sheet.Keyframes))
	}
	kf := sheet.Keyframes[0]
	if kf.Name != "spin" {
		t.Errorf("name: %q", kf.Name)
	}
	if len(kf.Stops) != 2 {
		t.Fatalf("stops: %d", len(kf.Stops))
	}
	if kf.Stops[0].Offset != 0 || kf.Stops[1].Offset != 1 {
		t.Errorf("offsets: %v / %v",
			kf.Stops[0].Offset, kf.Stops[1].Offset)
	}
}

func TestPhaseD_KeyframesParse_Percentages(t *testing.T) {
	sheet := css.ParseFull(`
		@keyframes pulse {
			0%   { opacity: 1 }
			50%  { opacity: 0 }
			100% { opacity: 1 }
		}
	`, css.ParseOptions{})
	if len(sheet.Keyframes) != 1 {
		t.Fatalf("keyframes count: %d", len(sheet.Keyframes))
	}
	kf := sheet.Keyframes[0]
	if len(kf.Stops) != 3 {
		t.Fatalf("stops: %d", len(kf.Stops))
	}
	wantOffsets := []float32{0, 0.5, 1}
	for i, s := range kf.Stops {
		if s.Offset != wantOffsets[i] {
			t.Errorf("stop %d offset: got %v want %v",
				i, s.Offset, wantOffsets[i])
		}
	}
}

func TestPhaseD_KeyframesParse_SharedSelector(t *testing.T) {
	sheet := css.ParseFull(`
		@keyframes blink {
			0%, 100% { opacity: 1 }
			50%      { opacity: 0 }
		}
	`, css.ParseOptions{})
	if len(sheet.Keyframes) != 1 {
		t.Fatalf("keyframes count: %d", len(sheet.Keyframes))
	}
	if len(sheet.Keyframes[0].Stops) != 3 {
		t.Fatalf("stops: %d", len(sheet.Keyframes[0].Stops))
	}
}

func TestPhaseD_AnimationShorthand(t *testing.T) {
	var spec cssAnimSpec
	applyCSSAnimProp("animation",
		"spin 2s ease-in-out 0.5s infinite alternate forwards", &spec)
	if spec.Name != "spin" {
		t.Errorf("name: %q", spec.Name)
	}
	if spec.DurationSec != 2 {
		t.Errorf("dur: %v", spec.DurationSec)
	}
	if spec.DelaySec != 0.5 {
		t.Errorf("delay: %v", spec.DelaySec)
	}
	if spec.IterCount != gui.SvgAnimIterInfinite {
		t.Errorf("iter: %v", spec.IterCount)
	}
	if spec.Direction != cssAnimDirAlternate {
		t.Errorf("dir: %v", spec.Direction)
	}
	if spec.FillMode != cssAnimFillForwards {
		t.Errorf("fillmode: %v", spec.FillMode)
	}
	if spec.TimingFn != cssAnimTimingCubic {
		t.Errorf("timing: %v", spec.TimingFn)
	}
}

func TestPhaseD_CompileColorTween(t *testing.T) {
	src := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10">
		<style>
		@keyframes flash {
			from { fill: rgb(255,0,0) }
			to   { fill: rgb(0,255,0) }
		}
		.x { animation: flash 1s }
		</style>
		<rect class="x" width="10" height="10"/>
	</svg>`
	vg := parseSvgT(t, src)
	if len(vg.Animations) != 1 {
		t.Fatalf("animations: %d", len(vg.Animations))
	}
	a := vg.Animations[0]
	if a.Kind != gui.SvgAnimColor {
		t.Errorf("kind: %v", a.Kind)
	}
	if a.DurSec != 1 {
		t.Errorf("dur: %v", a.DurSec)
	}
	if len(a.ColorValues) != 2 {
		t.Fatalf("color stops: %d", len(a.ColorValues))
	}
	want0 := uint32(255)<<24 | uint32(255)
	want1 := uint32(255)<<16 | uint32(255)
	if a.ColorValues[0] != want0 || a.ColorValues[1] != want1 {
		t.Errorf("colors: %x / %x", a.ColorValues[0], a.ColorValues[1])
	}
}

func TestPhaseD_InvalidKeyframeColorDropsTimeline(t *testing.T) {
	src := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10">
		<style>
		@keyframes bad { from { fill: red } to { fill: bogus } }
		.x { animation: bad 1s }
		</style>
		<rect class="x" width="10" height="10"/>
	</svg>`
	vg := parseSvgT(t, src)
	for i, a := range vg.Animations {
		if a.Kind == gui.SvgAnimColor {
			t.Fatalf("anim %d: emitted color timeline despite "+
				"unparseable keyframe stop; cols=%v", i, a.ColorValues)
		}
	}
}

func TestPhaseD_CompileOpacityTween(t *testing.T) {
	src := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10">
		<style>
		@keyframes fade { from { opacity: 1 } to { opacity: 0 } }
		.x { animation: fade 2s }
		</style>
		<rect class="x" width="10" height="10"/>
	</svg>`
	vg := parseSvgT(t, src)
	if len(vg.Animations) != 1 {
		t.Fatalf("animations: %d", len(vg.Animations))
	}
	a := vg.Animations[0]
	if a.Kind != gui.SvgAnimOpacity {
		t.Errorf("kind: %v", a.Kind)
	}
	if len(a.Values) != 2 || a.Values[0] != 1 || a.Values[1] != 0 {
		t.Errorf("values: %v", a.Values)
	}
}

func TestPhaseD_CompileRotateTween(t *testing.T) {
	src := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10">
		<style>
		@keyframes spin {
			from { transform: rotate(0deg) }
			to   { transform: rotate(360deg) }
		}
		.x { animation: spin 1s linear infinite }
		</style>
		<rect class="x" width="10" height="10"/>
	</svg>`
	vg := parseSvgT(t, src)
	if len(vg.Animations) != 1 {
		t.Fatalf("animations: %d", len(vg.Animations))
	}
	a := vg.Animations[0]
	if a.Kind != gui.SvgAnimRotate {
		t.Errorf("kind: %v", a.Kind)
	}
	if a.Iterations != gui.SvgAnimIterInfinite {
		t.Errorf("iter: %v", a.Iterations)
	}
	if len(a.Values) != 2 || a.Values[0] != 0 || a.Values[1] != 360 {
		t.Errorf("angles: %v", a.Values)
	}
}

func TestPhaseD_CompileCompoundTransformSplit(t *testing.T) {
	src := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10">
		<style>
		@keyframes mix {
			from { transform: rotate(0) translate(0,0) }
			to   { transform: rotate(90) translate(10,5) }
		}
		.x { animation: mix 1s }
		</style>
		<rect class="x" width="10" height="10"/>
	</svg>`
	vg := parseSvgT(t, src)
	if len(vg.Animations) != 2 {
		t.Fatalf("animations (rotate+translate split): %d",
			len(vg.Animations))
	}
	kinds := []gui.SvgAnimKind{
		vg.Animations[0].Kind, vg.Animations[1].Kind,
	}
	hasRotate := kinds[0] == gui.SvgAnimRotate ||
		kinds[1] == gui.SvgAnimRotate
	hasTranslate := kinds[0] == gui.SvgAnimTranslate ||
		kinds[1] == gui.SvgAnimTranslate
	if !hasRotate || !hasTranslate {
		t.Errorf("kinds: %+v", kinds)
	}
}

func TestPhaseD_AlternateFlag(t *testing.T) {
	src := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10">
		<style>
		@keyframes pulse { from { opacity: 1 } to { opacity: 0 } }
		.x { animation: pulse 1s 2 alternate }
		</style>
		<rect class="x" width="10" height="10"/>
	</svg>`
	vg := parseSvgT(t, src)
	if len(vg.Animations) != 1 {
		t.Fatalf("animations: %d", len(vg.Animations))
	}
	a := vg.Animations[0]
	if !a.Alternate {
		t.Errorf("Alternate not set")
	}
	if a.Iterations != 2 {
		t.Errorf("iter: %v", a.Iterations)
	}
}

func TestPhaseD_FillBackwardsFlag(t *testing.T) {
	src := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10">
		<style>
		@keyframes fade { from { opacity: 0 } to { opacity: 1 } }
		.x { animation: fade 1s 0.5s backwards }
		</style>
		<rect class="x" width="10" height="10"/>
	</svg>`
	vg := parseSvgT(t, src)
	if len(vg.Animations) != 1 {
		t.Fatalf("animations: %d", len(vg.Animations))
	}
	a := vg.Animations[0]
	if !a.FillBackwards {
		t.Errorf("FillBackwards not set")
	}
	if a.BeginSec != 0.5 {
		t.Errorf("begin: %v", a.BeginSec)
	}
}

func TestPhaseD_FillForwardsFreeze(t *testing.T) {
	src := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10">
		<style>
		@keyframes fade { from { opacity: 1 } to { opacity: 0 } }
		.x { animation: fade 1s forwards }
		</style>
		<rect class="x" width="10" height="10"/>
	</svg>`
	vg := parseSvgT(t, src)
	if len(vg.Animations) != 1 {
		t.Fatalf("animations: %d", len(vg.Animations))
	}
	if !vg.Animations[0].Freeze {
		t.Errorf("Freeze not set on forwards")
	}
}

func TestPhaseD_NoMatchingKeyframes(t *testing.T) {
	src := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10">
		<style>.x { animation: missing 1s }</style>
		<rect class="x" width="10" height="10"/>
	</svg>`
	vg := parseSvgT(t, src)
	if len(vg.Animations) != 0 {
		t.Errorf("expected 0 animations: %d", len(vg.Animations))
	}
}

func TestPhaseD_CompileDashOffsetTween(t *testing.T) {
	src := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10">
		<style>
		@keyframes dash { from { stroke-dashoffset: 100 } to { stroke-dashoffset: 0 } }
		.x { stroke: black; stroke-dasharray: 10; animation: dash 1s }
		</style>
		<rect class="x" width="10" height="10"/>
	</svg>`
	vg := parseSvgT(t, src)
	if len(vg.Animations) != 1 {
		t.Fatalf("animations: %d", len(vg.Animations))
	}
	a := vg.Animations[0]
	if a.Kind != gui.SvgAnimDashOffset {
		t.Errorf("kind: %v", a.Kind)
	}
	if len(a.Values) != 2 || a.Values[0] != 100 || a.Values[1] != 0 {
		t.Errorf("values: %v", a.Values)
	}
}

// Partial keyframes — only the `to` stop names the property. CSS spec
// implicitly synthesizes the missing 0% from the element's static
// stroke-dashoffset (here: 50). Without this, the timeline would not
// compile and heart-pulse-2 would never animate.
func TestPhaseD_CompileDashOffsetImplicitFrom(t *testing.T) {
	src := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10">
		<style>
		@keyframes dash { to { stroke-dashoffset: 0 } }
		.x { stroke: black; stroke-dasharray: 10; stroke-dashoffset: 50;
			animation: dash 1s }
		</style>
		<rect class="x" width="10" height="10"/>
	</svg>`
	vg := parseSvgT(t, src)
	if len(vg.Animations) != 1 {
		t.Fatalf("animations: %d", len(vg.Animations))
	}
	a := vg.Animations[0]
	if a.Kind != gui.SvgAnimDashOffset {
		t.Errorf("kind: %v", a.Kind)
	}
	if len(a.Values) != 2 || a.Values[0] != 50 || a.Values[1] != 0 {
		t.Errorf("values: %v", a.Values)
	}
}

func TestPhaseD_AnimationDelayShorthand(t *testing.T) {
	var spec cssAnimSpec
	applyCSSAnimProp("animation", "fade 200ms 100ms", &spec)
	if spec.DurationSec != 0.2 {
		t.Errorf("dur: %v", spec.DurationSec)
	}
	if spec.DelaySec != 0.1 {
		t.Errorf("delay: %v", spec.DelaySec)
	}
}

// CSS keyframe `opacity: 50%` must compile to 0.5, matching the
// static cascade. Earlier impl re-parsed with parseFloatTrimmed,
// turning 50% into 50 → clamped to 1 (fully opaque), so static and
// animated styling diverged.
func TestPhaseD_OpacityKeyframePercentage(t *testing.T) {
	src := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10">
		<style>
		@keyframes fade { from { opacity: 100% } to { opacity: 50% } }
		.x { animation: fade 1s }
		</style>
		<rect class="x" width="10" height="10"/>
	</svg>`
	vg := parseSvgT(t, src)
	if len(vg.Animations) != 1 {
		t.Fatalf("animations: %d", len(vg.Animations))
	}
	a := vg.Animations[0]
	if len(a.Values) < 2 {
		t.Fatalf("expected >=2 keyframe values, got %d", len(a.Values))
	}
	if a.Values[0] < 0.99 {
		t.Errorf("from opacity = %f, want ~1.0", a.Values[0])
	}
	if a.Values[len(a.Values)-1] > 0.51 || a.Values[len(a.Values)-1] < 0.49 {
		t.Errorf("to opacity = %f, want ~0.5", a.Values[len(a.Values)-1])
	}
}
