# Custom shader backgrounds: built-in inputs

Issue #573. Status: **implemented** (Part A, the built-in inputs; Part B, the
GLSL-to-Go translator `cmd/shaderport`).

## Problem

Custom fragment shaders on containers shipped before this work
(`ContainerCfg.Shader`, `RectangleCfg.Shader`), but a shader copied from
Shadertoy did not run without rewriting:

- It reads `iTime` and `iResolution`, which did not exist. An app computed the
  time in its view function, passed it through `Params[0]`, and kept a repeating
  `Animate` with an empty callback alive to force frames. That rebuilt the view
  and the layout on every frame, and the app had to remove the animation again
  when the shader left the screen.
- Every shader is written twice, MSL and GLSL, and a copied shader is GLSL only
  and has the `mainImage` form. That is Part B.

## Design

### The built-in block

The inputs travel as one 4x4 float matrix, the same way `Params` does. Column c
holds slots 4c..4c+3:

| Slot  | Name              | Value                                            |
| ----- | ----------------- | ------------------------------------------------ |
| 0-2   | `iResolution.xyz` | container size; z is 1                           |
| 3     | `iTime`           | seconds since the first frame that drew a shader |
| 4-7   | `iMouse.xyzw`     | zero (not tracked; see below)                    |
| 8     | `iTimeDelta`      | seconds since the previous such frame, ≤ 0.25    |
| 9     | `iFrame`          | frames since the first such frame                |
| 10-15 | reserved, zero    |                                                  |

The slot constants live in `gui/shader_builtins.go`. The GLSL wrapper
(`gui/shader.go`) and the MSL wrapper (`gui/backend/internal/msl/custom.go`)
unpack by index. `TestGLSLWrapperBuiltinSlots` holds the GLSL unpacking to the
constants, and `TestCustomWrappersUnpackBuiltinsAlike` holds the MSL unpacking
to the GLSL one.

Units are Shadertoy's, so copied code reads them unchanged: sizes are physical
pixels. The existing `uv` spans -1..1 with y growing down; a ported shader
rebuilds `fragCoord` from it and `iResolution`.

### Where the values come from

The block is packed while the frame's render commands are built
(`packShaderShape`, called from `renderContainer`), not in the view function. A
render-only refresh rebuilds the commands from the existing layout, so the
built-ins advance without a view rebuild or a layout pass.

`RenderCmd` carries an unexported pointer to the block, which is stored in the
shape's effects (`shapeEffects.shaderBuiltins`) and rewritten in place on every
pass, so a long render-only animation does not allocate. The block is packed in
logical pixels. A backend reads it with `RenderCmd.ShaderBuiltinsAt(scale)`,
passing the scale of the frame it draws, and uploads the result on every draw,
zeros included: a cached program keeps its uniforms, and containers that share a
shader share one program.

Time, delta and frame are keyed on `Window.frameCount`, not on the render pass.
`FrameFn` can rebuild twice in one frame when the first pass moved the hover
target, and only the last pass is presented. `iTimeDelta` is capped at 0.25 s,
so the first frame after a pause (a hidden window, or no shader drawn for a
while) does not step a delta-driven animation by the whole pause; `iTime` is not
capped. A headless window (`SetHeadlessRender`) reads time, delta and frame as
zero, so captures are reproducible. So does a window whose user asked for
reduced motion (`prefersReducedMotion`, which also holds the thinking orb
still), so the shader holds still. Stopping the frames alone would not do: other
repaints (hover, typing) would still step `iTime` forward.

### Shader.Animate

`Shader.Animate` keeps frames coming while such a shader is drawn. The window
owns one render-only animation (`shaderAnimation`). `buildRenderers` clears a
flag before the pass, the shader emission sets it, and `syncShaderAnimation`
registers or removes the animation after the pass. Frames stop in the first
frame that draws no `Animate` shader. The lock order, `w.mu` then `animMu`, is
the caret blink's. A held clock (headless, or reduced motion; `shaderClockHeld`)
never starts it, and stops a running one.

### Wrappers and backends

The GLSL wrapper declares the built-ins as globals assigned at the top of
`main`; the MSL wrapper, which has no per-draw globals, declares them as locals
of `fs_main`. Both run the body in a nested block and copy its `frag_color` out,
so a body may declare a built-in's name or a wrapper variable's name (`d`, `q`,
`pos`, ...) and hide it.

| Backend    | Upload                                                      |
| ---------- | ----------------------------------------------------------- |
| Desktop GL | `_gg_builtins` uniform, `pipeline.uBuiltin`                 |
| WebGL2     | `_gg_builtins` uniform; WebGL returns null when dropped     |
| Android    | `glesSetBuiltins`, `_customBuiltinLocs` in `gles_android.c` |
| macOS, iOS | `metalSetCustomBuiltins`, fragment buffer 0                 |

On Metal, `fs_main` reads the buffer whatever the body uses, so every custom
draw must bind it; an unbound buffer is undefined behavior.
`TestCustomShaderBackendsUploadBuiltins` pins all five backends to an upload.

## iMouse (not implemented)

`iMouse` is declared and reads zero, Shadertoy's value for a canvas never
clicked: a shader that reads it compiles and draws as if never clicked, and
tracking the mouse later changes no shader. A first implementation was built and
taken out in review (#1096): four of the five correctness findings were in it,
and it exposed #1098 (one held-button slot, since replaced by a set). A later
implementation has to handle what that one got wrong or found hard:

- a drag over an idle window must draw a frame, also without `Animate`;
- only the left button's own release may end the press (#1098);
- a long press, which the gesture recognizer handles apart from a tap, must
  still drag;
- Shadertoy encodes held and click-frame state in the signs of `z` and `w`, so a
  press on the edge pixel (coordinate 0) cannot carry the state; it needs an
  offset or another encoding;
- per-container state needs a stable key (the container ID), presses must be
  latched from events so a click between two frames is not lost, and points must
  be mapped through view transforms as hit testing maps them.

Polling the button once per render pass loses short clicks, one window-wide
state moves every shader on any click, and keying on tree position or on the
`*Shader` pointer resets the state on every view rebuild.

## Phasing

| Part | Content                                                         | Status      |
| ---- | --------------------------------------------------------------- | ----------- |
| —    | Shader-only container emits (#574); `ShaderHash` on both bodies | implemented |
| A    | Built-in inputs on every GPU backend; `Shader.Animate`          | implemented |
| B    | `cmd/shaderport`: Shadertoy GLSL to a `*gui.Shader`; guide      | implemented |

Out of scope, each its own issue if wanted: `iMouse`, a `gui/shaders` library,
and texture input (`iChannel0`).

## Part B: `cmd/shaderport`

### Program-scope declarations

A body is the inside of one function, so it cannot declare helper functions,
which most Shadertoy shaders do. `Shader.GLSLDecls` and `Shader.MetalDecls` hold
program-scope declarations. The wrappers put them before `main` / `fs_main`
(`gui.BuildGLSLFragmentDecls`, `gui.BuildGLSLESFragmentDecls`,
`msl.BuildCustomDecls`). In GLSL they come after the built-in globals, so a
helper reads `iTime` directly. In MSL the built-ins are locals of `fs_main`, so
the translator passes each one a helper reads as a parameter.

shaderport moves `mainImage` itself into the decls. Each body is then one call,
and an early `return` in `mainImage` works.

| Design                                         | Exported surface | Fails how                                                |
| ---------------------------------------------- | ---------------- | -------------------------------------------------------- |
| **`GLSLDecls` / `MetalDecls` fields (picked)** | 2 fields, 3 fns  | Compile error on that backend, logged once and cached    |
| Translator inlines every helper into the body  | none             | Out params, early returns and macros: wrong output       |
| Full-program `Source` fields                   | 2 fields         | Bypasses the rounded-corner clip and the output contract |

### The translator

`internal/shaderport` is a token rewriter, not a compiler: it keeps layout,
comments and macros, so the output reads as a reviewable diff. It needs no type
checker. GLSL ES 3.00 has no implicit conversions, and probing the Metal
compiler showed that MSL accepts GLSL's mixed vector and scalar calls
(`mix(vec3, vec3, float)`, `max(vec3, float)`, matrix constructors from scalars)
and non-trivial program-scope constants (`acos(-1.0)`). The rewrite table and
the rejected constructs are in `docs/shader-porting.md`.

The tests compile every fixture both ways: MSL through the Metal probe
(`TestShaderportCompiles`), GLSL through `glslangValidator` as 3.30 and ES 3.00
(`TestPortGLSLCompiles`). That caught one rule the probe of MSL alone missed:
desktop GLSL 3.30 rejects a `\` line continuation, which ES 3.00 accepts, so the
GLSL output joins continued `#define` lines.

### The hash cache

Backends key the compiled pipeline on `ShaderHash`, which walks every byte of
all four strings, on every draw of every frame. A ported shader is several KB
per language: 32 KB of source hashes in about 30 µs. `gui.ShaderHashCache`, one
per backend window, memoizes the hash per `*Shader` and revalidates it by each
string's data pointer and length. A hit costs about 4 ns and allocates nothing.
Go strings are immutable and the entry holds the pointers, so equal pointer and
length mean equal bytes.

## Rejected Approaches

- **Inlining helpers into the body** (no new `Shader` fields). Out parameters,
  early returns and macros make it a compiler pass, and its failure mode is
  wrong output rather than an error.
- **A full-program shader mode.** It lets an app drop the rounded-corner clip
  and the output contract, for no gain over decls.
- **Program-scope built-ins in MSL.** A program-scope MSL variable must be
  `constant` and initialized at compile time; it cannot change per draw.
- **Memoizing the hash in `Shader` fields.** One package-level shader drawn by
  two windows on two goroutines races on the fields, and an `atomic.Pointer`
  makes `Shader` uncopyable (vet copylocks). The cache lives in each backend
  window.

- **Runtime GLSL-to-MSL translation.** It moves a compile-time failure into the
  frame loop: a translator bug ships as a blank container on macOS while Linux
  looks fine. An offline tool emits reviewable Go source.
- **Vendoring naga or glslang/SPIRV-Cross.** The cgo-free Linux and Windows
  build is settled policy (`cgo-free-backend-feasibility.md`).
- **Reserving `Params` slots for the built-ins.** All 16 floats are app-owned;
  reserving any silently changes the meaning of existing shaders.
- **Copying the 16 floats into every `RenderCmd`.** It grows every command, of
  every kind, from 312 to 376 bytes for a feature most apps do not use. The
  block is stored once per shader shape and referenced.
- **One snapshot per frame, packed by each backend.** It adds a `Window` method
  and repeats the packing in five backends, and a backend that forgets the
  snapshot freezes the shader at time zero without an error.
- **Packing physical pixels from `Window.BackingScale`.** Backends set that
  field while drawing, after the commands were built, so it lags one frame and
  is zero in the first: a static shader on a HiDPI display could stay at half
  size. The block is packed in logical pixels and converted by the backend.
- **Passing the built-ins as varyings.** They are constant across the quad, and
  varyings would touch both vertex shaders, one of them a hand-edited C string
  in the Android backend.
- **Keeping the Animate animation alive from the view function** (a view-bound
  heartbeat, as spinners use). A render-only refresh runs no view function, so
  the heartbeat would go stale and the animation would be cancelled after two
  seconds.
