package gui

import "time"

// Slots of the custom-shader built-in uniform block (#573). The
// block is sixteen floats the GPU reads as one column-major 4x4
// matrix, so column c holds slots 4c..4c+3. The GLSL wrapper
// (shader.go) and the MSL wrapper (backend/internal/msl/custom.go)
// unpack these slots by index; change all three together.
//
// The names and units follow Shadertoy, so a shader copied from
// there reads them unedited: sizes are physical pixels. The block is
// packed in logical pixels while the frame's commands are built, and
// RenderCmd.ShaderBuiltinsAt converts it at upload with the scale of
// the frame the backend draws. Window.BackingScale cannot serve:
// backends set it while drawing, after the commands were built, so
// it lags a frame and is zero in the first.
const (
	shaderBuiltinResX      = 0 // iResolution.x: shape width
	shaderBuiltinResY      = 1 // iResolution.y: shape height
	shaderBuiltinResZ      = 2 // iResolution.z: pixel aspect, always 1
	shaderBuiltinTime      = 3 // iTime: seconds since the first shader frame
	shaderBuiltinMouse     = 4 // iMouse.xyzw in slots 4-7: not tracked, zero
	shaderBuiltinTimeDelta = 8 // iTimeDelta: seconds since the previous frame
	shaderBuiltinFrame     = 9 // iFrame: frames since the first shader frame
	// Slots 10-15 are reserved and stay zero. iMouse stays zero too:
	// Shadertoy's value for a canvas never clicked, so a shader that
	// reads it compiles and behaves, and tracking it later breaks
	// nothing.
)

// shaderMaxTimeDelta caps iTimeDelta in seconds, so the first frame
// after a pause (a hidden window, or a stretch with no shader drawn)
// does not step a delta-driven animation by the whole pause. iTime is
// not capped.
const shaderMaxTimeDelta = 0.25

// shaderFrameState is the window-wide half of the built-in block:
// the values every shader drawn in one frame shares. The per-shape
// half (resolution) is added by packShaderBuiltins.
//
// It is keyed on w.frameCount, not w.renderPass: FrameFn can run a
// second render pass in one frame (when the first moved the hover
// target), and only the last is presented. Keyed per pass, iFrame
// would count two frames and iTimeDelta would read near zero.
type shaderFrameState struct {
	started bool      // false until the first frame that drew a shader
	key     uint64    // w.frameCount the fields below describe
	start   time.Time // iTime origin
	last    time.Time // previous frame, for iTimeDelta
	time    float32
	delta   float32
	frame   uint32
}

// advance moves f to the frame numbered key.
func (f *shaderFrameState) advance(now time.Time, key uint64) {
	if !f.started {
		f.started = true
		f.start, f.last = now, now
		f.frame = 0
	} else {
		f.frame++
	}
	f.key = key
	f.time = float32(now.Sub(f.start).Seconds())
	f.delta = min(float32(now.Sub(f.last).Seconds()), shaderMaxTimeDelta)
	f.last = now
}

// shaderFrame returns the window-wide built-in state for the current
// frame. It reads the clock only on the first shader of a frame, so a
// window that draws no shader never does.
func (w *Window) shaderFrame() *shaderFrameState {
	f := &w.shaderBuiltinState
	if !f.started || f.key != w.frameCount {
		f.advance(time.Now(), w.frameCount)
	}
	return f
}

// packShaderShape writes the built-in block for the shader shape into
// shape.fx.shaderBuiltins.
func (w *Window) packShaderShape(shape *Shape) {
	f := w.shaderFrame()
	packShaderBuiltins(&shape.fx.shaderBuiltins, f,
		shape.Width, shape.Height, w.headlessRender)
}

// packShaderBuiltins writes the built-in block for one shader shape
// of width x height logical pixels into dst, in logical pixels; see
// RenderCmd.ShaderBuiltinsAt for the conversion. headless zeroes the
// clock-driven slots so a captured frame is reproducible.
func packShaderBuiltins(dst *[16]float32, f *shaderFrameState,
	width, height float32, headless bool) {
	*dst = [16]float32{}
	dst[shaderBuiltinResX] = width
	dst[shaderBuiltinResY] = height
	dst[shaderBuiltinResZ] = 1
	if !headless {
		dst[shaderBuiltinTime] = f.time
		dst[shaderBuiltinTimeDelta] = f.delta
		dst[shaderBuiltinFrame] = float32(f.frame)
	}
}

// ShaderBuiltinsAt returns the built-in uniform block of a
// RenderCustomShader, ready to upload beside the Params matrix as one
// column-major 4x4 matrix: iResolution, iTime, iTimeDelta and iFrame
// in Shadertoy's units, and iMouse as zero. scale is the backing scale
// of the frame being drawn, in physical pixels per logical pixel;
// sizes are converted with it, and a scale that is not a positive
// finite number counts as 1. A command without a block returns zeros.
//
// Upload the result on every draw, zeros included: a compiled program
// keeps its uniforms between draws, and containers sharing one shader
// share one program, so a skipped upload would show the previous
// container's values.
func (r *RenderCmd) ShaderBuiltinsAt(scale float32) [16]float32 {
	if r.shaderBuiltins == nil {
		return [16]float32{}
	}
	if !(scale > 0) || !f32IsFinite(scale) {
		scale = 1
	}
	b := *r.shaderBuiltins
	b[shaderBuiltinResX] *= scale
	b[shaderBuiltinResY] *= scale
	return b
}
