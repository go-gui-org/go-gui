package gui

import (
	"math"
	"testing"
	"time"
)

// Shader containers in these tests are sbW x sbH logical pixels.
const sbW, sbH = 100, 50

// --- packing ---------------------------------------------------------

// packTest packs a block in logical pixels, then converts it the way a
// backend reads it, at scale.
func packTest(f *shaderFrameState, scale float32) [16]float32 {
	var dst [16]float32
	packShaderBuiltins(&dst, f, sbW, sbH, false)
	r := RenderCmd{shaderBuiltins: &dst}
	return r.ShaderBuiltinsAt(scale)
}

func TestShaderBuiltinsResolution(t *testing.T) {
	var f shaderFrameState
	f.advance(time.Unix(0, 0), 1)
	got := packTest(&f, 2)
	if got[shaderBuiltinResX] != 200 || got[shaderBuiltinResY] != 100 ||
		got[shaderBuiltinResZ] != 1 {
		t.Errorf("iResolution = %v, want (200,100,1)",
			got[shaderBuiltinResX:shaderBuiltinResZ+1])
	}
	for i := 10; i < 16; i++ {
		if got[i] != 0 {
			t.Errorf("reserved slot %d = %v, want 0", i, got[i])
		}
	}
}

func TestShaderBuiltinsClock(t *testing.T) {
	var f shaderFrameState
	t0 := time.Unix(100, 0)
	f.advance(t0, 1)
	got := packTest(&f, 1)
	if got[shaderBuiltinTime] != 0 || got[shaderBuiltinTimeDelta] != 0 ||
		got[shaderBuiltinFrame] != 0 {
		t.Errorf("first frame time/delta/frame = %v/%v/%v, want 0/0/0",
			got[shaderBuiltinTime], got[shaderBuiltinTimeDelta],
			got[shaderBuiltinFrame])
	}
	f.advance(t0.Add(100*time.Millisecond), 2)
	f.advance(t0.Add(150*time.Millisecond), 3)
	got = packTest(&f, 1)
	if got[shaderBuiltinTime] != 0.15 || got[shaderBuiltinTimeDelta] != 0.05 ||
		got[shaderBuiltinFrame] != 2 {
		t.Errorf("third frame time/delta/frame = %v/%v/%v, want 0.15/0.05/2",
			got[shaderBuiltinTime], got[shaderBuiltinTimeDelta],
			got[shaderBuiltinFrame])
	}
}

// After a pause, iTimeDelta is capped so a delta-driven animation does
// not jump by the whole pause; iTime still reports the real time.
func TestShaderBuiltinsTimeDeltaCapped(t *testing.T) {
	var f shaderFrameState
	t0 := time.Unix(100, 0)
	f.advance(t0, 1)
	f.advance(t0.Add(3*time.Second), 2)
	got := packTest(&f, 1)
	if got[shaderBuiltinTimeDelta] != shaderMaxTimeDelta {
		t.Errorf("iTimeDelta after a 3 s pause = %v, want %v",
			got[shaderBuiltinTimeDelta], shaderMaxTimeDelta)
	}
	if got[shaderBuiltinTime] != 3 {
		t.Errorf("iTime after a 3 s pause = %v, want 3 (not capped)",
			got[shaderBuiltinTime])
	}
}

func TestShaderBuiltinsHeadlessZeroesClock(t *testing.T) {
	var f shaderFrameState
	f.advance(time.Unix(0, 0), 1)
	f.advance(time.Unix(3, 0), 2)
	var got [16]float32
	packShaderBuiltins(&got, &f, sbW, sbH, true)
	if got[shaderBuiltinTime] != 0 || got[shaderBuiltinTimeDelta] != 0 ||
		got[shaderBuiltinFrame] != 0 {
		t.Error("headless pack should zero time, delta and frame")
	}
	if got[shaderBuiltinResX] != sbW {
		t.Error("headless pack should keep the resolution")
	}
}

// For a user who asked for reduced motion the shader clock holds
// still, as in a headless capture: iTime, iTimeDelta and iFrame read
// zero on every frame, while the resolution is still filled in.
func TestShaderBuiltinsReducedMotionHoldsClock(t *testing.T) {
	for _, reduced := range []bool{false, true} {
		w := NewTestWindow(t, WindowCfg{Width: 400, Height: 300})
		w.nativePlatform = reducedMotionPlatform{pref: reduced}
		w.SetView(shaderRoot(func() []View { return []View{shaderBox("sh")} }))
		for range 3 {
			shaderFrameStep(w)
		}
		b := mustShape(t, w, "sh").fx.shaderBuiltins
		if b[shaderBuiltinResX] != sbW {
			t.Errorf("reduced=%v: iResolution.x = %v, want %v",
				reduced, b[shaderBuiltinResX], sbW)
		}
		frame, held := b[shaderBuiltinFrame], b[shaderBuiltinTime] == 0 &&
			b[shaderBuiltinTimeDelta] == 0 && b[shaderBuiltinFrame] == 0
		if reduced && !held {
			t.Errorf("reduced motion: time/delta/frame = %v/%v/%v, want 0/0/0",
				b[shaderBuiltinTime], b[shaderBuiltinTimeDelta], frame)
		}
		// Control: without the preference the clock runs.
		if !reduced && frame != 2 {
			t.Errorf("no preference: iFrame = %v after 3 frames, want 2", frame)
		}
	}
}

// ShaderBuiltinsAt scales the resolution and leaves time and frame
// alone.
func TestShaderBuiltinsAtScales(t *testing.T) {
	block := [16]float32{
		shaderBuiltinResX: 100, shaderBuiltinResY: 50, shaderBuiltinResZ: 1,
		shaderBuiltinTime: 3, shaderBuiltinTimeDelta: 0.125,
		shaderBuiltinFrame: 7,
	}
	r := RenderCmd{shaderBuiltins: &block}
	got := r.ShaderBuiltinsAt(1.25)
	want := block
	want[shaderBuiltinResX], want[shaderBuiltinResY] = 125, 62.5
	if got != want {
		t.Errorf("at 1.25:\n got %v\nwant %v", got, want)
	}
	if block[shaderBuiltinResX] != 100 {
		t.Error("ShaderBuiltinsAt must not change the stored block")
	}
}

// A scale that is not a positive finite number counts as 1, and a
// command without a block reads zeros.
func TestShaderBuiltinsAtEdgeCases(t *testing.T) {
	block := [16]float32{shaderBuiltinResX: 100}
	r := RenderCmd{shaderBuiltins: &block}
	nan := float32(math.NaN())
	inf := float32(math.Inf(1))
	for _, s := range []float32{0, -2, nan, inf} {
		if got := r.ShaderBuiltinsAt(s); got[shaderBuiltinResX] != 100 {
			t.Errorf("scale %v: res %v, want 100", s, got[shaderBuiltinResX])
		}
	}
	var none RenderCmd
	if got := none.ShaderBuiltinsAt(2); got != [16]float32{} {
		t.Errorf("no block: %v, want zeros", got)
	}
}

// --- emission and frames ----------------------------------------------

// shaderBox is a fixed-size shader container.
func shaderBox(id string, content ...View) View {
	return Column(ContainerCfg{
		ID: id, Width: sbW, Height: sbH, Sizing: FixedFixed,
		Padding: PaddingNone, SizeBorder: NoBorder,
		Shader:  &Shader{Metal: "m", GLSL: "g"},
		Content: content,
	})
}

// shaderRoot lays the views from content out in a column at the
// window's top-left corner. content runs every frame.
func shaderRoot(content func() []View) func(*Window) View {
	return func(*Window) View {
		return Column(ContainerCfg{
			Sizing: FillFill, Padding: PaddingNone, Spacing: SpacingPx(0),
			Content: content(),
		})
	}
}

func shaderWindow(t *testing.T, view func(*Window) View) *Window {
	t.Helper()
	w := NewTestWindow(t, WindowCfg{Width: 400, Height: 300})
	w.SetView(view)
	shaderFrameStep(w)
	return w
}

// shaderFrameStep draws one frame the way the backend loop does:
// FrameFn advances the frame counter, then rebuilds the view, the
// layout and the render commands.
func shaderFrameStep(w *Window) {
	w.markLayoutRefresh(refreshTest)
	w.FrameFn()
}

// A shader container's command carries the built-in block, and it
// points at the shape's own storage. The block is in logical pixels
// whatever BackingScale says: that field lags a frame behind.
func TestRenderLayoutShaderEmitsBuiltins(t *testing.T) {
	w := makeWindow()
	w.BackingScale = 2
	shape := &Shape{
		shapeType: shapeRectangle,
		Color:     ColorTransparent,
		X:         10, Y: 20, Width: sbW, Height: sbH,
		fx: &shapeEffects{Shader: &Shader{Metal: "m", GLSL: "g"}},
	}
	renderLayout(&Layout{Shape: shape}, ColorTransparent,
		makeClip(0, 0, 200, 200), w)

	for _, r := range w.renderers {
		if r.Kind != RenderCustomShader {
			continue
		}
		if r.shaderBuiltins != &shape.fx.shaderBuiltins {
			t.Fatal("shaderBuiltins should point at the shape's storage")
		}
		if r.shaderBuiltins[shaderBuiltinResX] != sbW ||
			r.shaderBuiltins[shaderBuiltinResY] != sbH {
			t.Errorf("stored iResolution = %v,%v, want logical %v,%v",
				r.shaderBuiltins[shaderBuiltinResX],
				r.shaderBuiltins[shaderBuiltinResY], sbW, sbH)
		}
		return
	}
	t.Fatal("missing RenderCustomShader")
}

// iFrame counts frames, not render passes: a frame FrameFn rebuilds
// twice advances it once.
func TestShaderBuiltinsFrameSpansRenderPasses(t *testing.T) {
	w := shaderWindow(t, shaderRoot(func() []View {
		return []View{shaderBox("sh")}
	}))
	frameOf := func() float32 {
		return mustShape(t, w, "sh").fx.shaderBuiltins[shaderBuiltinFrame]
	}
	start := frameOf()
	w.frameCount++
	for pass := 1; pass <= 2; pass++ {
		w.markLayoutRefresh(refreshTest)
		w.Update()
		if got := frameOf(); got != start+1 {
			t.Errorf("pass %d: iFrame = %v, want %v", pass, got, start+1)
		}
	}
	shaderFrameStep(w)
	if got := frameOf(); got != start+2 {
		t.Errorf("next frame: iFrame = %v, want %v", got, start+2)
	}
}

// iMouse is not tracked: its slots stay zero, Shadertoy's value for a
// canvas never clicked, even under a press and drag on the shader.
func TestShaderBuiltinsMouseStaysZero(t *testing.T) {
	w := shaderWindow(t, shaderRoot(func() []View {
		return []View{shaderBox("sh")}
	}))
	s := mustShape(t, w, "sh")
	x, y := s.X+30, s.Y+20
	for _, e := range []Event{
		{Type: EventMouseDown, MouseButton: MouseLeft, MouseX: x, MouseY: y},
		{Type: EventMouseMove, MouseButton: MouseLeft, MouseX: x + 10, MouseY: y},
	} {
		w.EventFn(&e)
	}
	shaderFrameStep(w)
	b := mustShape(t, w, "sh").fx.shaderBuiltins
	if got := [4]float32(b[shaderBuiltinMouse : shaderBuiltinMouse+4]); got != [4]float32{} {
		t.Errorf("iMouse during a drag = %v, want zero", got)
	}
}

// --- Shader.Animate ---------------------------------------------------

// animatedBox is a shader container whose shader sets Animate.
func animatedBox(id string, animate bool) View {
	return Column(ContainerCfg{
		ID: id, Width: sbW, Height: sbH, Sizing: FixedFixed,
		Shader: &Shader{Metal: "m", GLSL: "g", Animate: animate},
	})
}

func TestShaderAnimationTicksRenderOnly(t *testing.T) {
	var a shaderAnimation
	if a.RefreshKind() != AnimationRefreshRenderOnly {
		t.Errorf("RefreshKind = %v, want AnimationRefreshRenderOnly", a.RefreshKind())
	}
	if !a.Update(nil, 0.016, nil) || a.IsStopped() {
		t.Error("the shader animation should ask for a frame on every tick and never stop")
	}
}

// The animation runs while an Animate shader is drawn and stops in the
// first frame that draws none.
func TestShaderAnimateStartsAndStops(t *testing.T) {
	show := true
	w := shaderWindow(t, shaderRoot(func() []View {
		if show {
			return []View{animatedBox("sh", true)}
		}
		return []View{Text(TextCfg{Text: "no shader"})}
	}))
	if !w.HasAnimation(shaderAnimationID) {
		t.Fatal("drawing an Animate shader should start the shader animation")
	}
	show = false
	shaderFrameStep(w)
	if w.HasAnimation(shaderAnimationID) {
		t.Error("a frame without an Animate shader should stop the shader animation")
	}
	show = true
	shaderFrameStep(w)
	if !w.HasAnimation(shaderAnimationID) {
		t.Error("an Animate shader drawn again should restart the animation")
	}
}

func TestShaderAnimateOffByDefault(t *testing.T) {
	w := shaderWindow(t, shaderRoot(func() []View {
		return []View{animatedBox("sh", false)}
	}))
	if w.HasAnimation(shaderAnimationID) {
		t.Error("a shader without Animate should not start the shader animation")
	}
}

// A command rejected by the render guard is not a drawn shader. It
// must not start the shader clock or a render-only loop that can never
// produce a custom-shader draw.
func TestShaderAnimateRejectedStartsNothing(t *testing.T) {
	tests := []struct {
		name   string
		width  float32
		shader *Shader
	}{
		{
			name:  "empty bodies",
			width: sbW,
			shader: &Shader{
				Animate: true,
			},
		},
		{
			name:  "zero width",
			width: 0,
			shader: &Shader{
				Metal: "m", GLSL: "g", Animate: true,
			},
		},
		{
			name:  "non-finite params",
			width: sbW,
			shader: &Shader{
				Metal: "m", GLSL: "g", Animate: true,
				Params: []float32{float32(math.NaN())},
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w := shaderWindow(t, shaderRoot(func() []View {
				return []View{Column(ContainerCfg{
					ID: "sh", Width: tc.width, Height: sbH,
					Sizing: FixedFixed, Shader: tc.shader,
				})}
			}))
			if w.HasAnimation(shaderAnimationID) {
				t.Error("a rejected Animate shader started the animation")
			}
			if w.shaderBuiltinState.started {
				t.Error("a rejected shader started the shader clock")
			}
			for _, r := range w.renderers {
				if r.Kind == RenderCustomShader {
					t.Error("render guard kept the rejected shader command")
				}
			}
		})
	}
}

// If a previously valid animated shader becomes invalid, the rejected
// draw must retire the animation in that same frame rather than leave
// an idle window rendering forever.
func TestShaderAnimateRejectedStopsRunningAnimation(t *testing.T) {
	valid := true
	w := shaderWindow(t, shaderRoot(func() []View {
		shader := &Shader{Animate: true}
		if valid {
			shader.Metal, shader.GLSL = "m", "g"
		}
		return []View{Column(ContainerCfg{
			ID: "sh", Width: sbW, Height: sbH,
			Sizing: FixedFixed, Shader: shader,
		})}
	}))
	if !w.HasAnimation(shaderAnimationID) {
		t.Fatal("the valid Animate shader did not start the animation")
	}

	valid = false
	shaderFrameStep(w)
	if w.HasAnimation(shaderAnimationID) {
		t.Error("a rejected shader left the animation running")
	}
	for _, r := range w.renderers {
		if r.Kind == RenderCustomShader {
			t.Error("render guard kept the rejected shader command")
		}
	}
}

// A headless window reads shader time as zero; frames would show
// nothing new, so none are started.
func TestShaderAnimateHeadlessStartsNothing(t *testing.T) {
	w := NewTestWindow(t, WindowCfg{Width: 400, Height: 300})
	w.SetHeadlessRender(true)
	w.SetView(shaderRoot(func() []View { return []View{animatedBox("sh", true)} }))
	shaderFrameStep(w)
	if w.HasAnimation(shaderAnimationID) {
		t.Error("a headless window should not start the shader animation")
	}
}

// For a user who asked for reduced motion an Animate shader starts no
// frames, as in a headless window.
func TestShaderAnimateReducedMotionStartsNothing(t *testing.T) {
	w := NewTestWindow(t, WindowCfg{Width: 400, Height: 300})
	w.nativePlatform = reducedMotionPlatform{pref: true}
	w.SetView(shaderRoot(func() []View { return []View{animatedBox("sh", true)} }))
	shaderFrameStep(w)
	if w.HasAnimation(shaderAnimationID) {
		t.Error("reduced motion should not start the shader animation")
	}
}

// Turning on reduced motion while an Animate shader runs stops its
// frames in the next frame.
func TestShaderAnimateReducedMotionStopsRunning(t *testing.T) {
	w := shaderWindow(t, shaderRoot(func() []View {
		return []View{animatedBox("sh", true)}
	}))
	if !w.HasAnimation(shaderAnimationID) {
		t.Fatal("the Animate shader did not start the animation")
	}
	w.nativePlatform = reducedMotionPlatform{pref: true}
	shaderFrameStep(w)
	if w.HasAnimation(shaderAnimationID) {
		t.Error("turning on reduced motion should stop the shader animation")
	}
}

// Removing the animation by ID from outside does not leave a drawn
// Animate shader frozen: the next frame registers it again.
func TestShaderAnimateRestoredAfterRemove(t *testing.T) {
	w := shaderWindow(t, shaderRoot(func() []View {
		return []View{animatedBox("sh", true)}
	}))
	w.AnimationRemove(shaderAnimationID)
	shaderFrameStep(w)
	if !w.HasAnimation(shaderAnimationID) {
		t.Error("the shader animation should be registered again")
	}
}

// What the animation asks for each tick, a render-only refresh,
// advances the built-ins without running the view function: animated
// shaders cost no view rebuild and no layout pass (issue #573, D3).
func TestShaderAnimateFramesSkipTheView(t *testing.T) {
	views := 0
	w := shaderWindow(t, func(*Window) View {
		views++
		return Column(ContainerCfg{
			Sizing: FillFill, Padding: PaddingNone,
			Content: []View{animatedBox("sh", true)},
		})
	})
	frameOf := func() float32 {
		return mustShape(t, w, "sh").fx.shaderBuiltins[shaderBuiltinFrame]
	}
	start, viewsBefore := frameOf(), views
	for range 3 {
		commandMarkRenderOnlyRefresh(w)
		w.FrameFn()
	}
	if views != viewsBefore {
		t.Errorf("view function ran %d more times, want 0", views-viewsBefore)
	}
	if got := frameOf(); got != start+3 {
		t.Errorf("iFrame = %v after 3 render-only frames, want %v", got, start+3)
	}
}

// reducedMotionPlatform reports the reduced-motion preference. It
// embeds noopNativePlatform because a rendering window also calls the
// platform's other methods, such as A11yDestroy on cleanup.
type reducedMotionPlatform struct {
	noopNativePlatform
	pref bool
}

func (p reducedMotionPlatform) PrefersReducedMotion() bool { return p.pref }
