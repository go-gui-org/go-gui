# Custom GPU shader integration

How to write and integrate custom fragment shaders into go-gui widgets.

## 1. Overview

Custom shaders replace the default rectangle fill with a GPU fragment
shader. Write the shader once in GLSL and Metal Shading Language
(MSL), pass it to any `ContainerCfg` or `RectangleCfg`, and animate
it by updating `Params` each frame.

The vertex shader is pre-defined. The fragment shader receives
interpolated UV coordinates, vertex color, and up to 16 custom float
parameters. An SDF rounded-rect alpha mask is applied automatically
so the shader respects the widget's `Radius`.

### Backend support

| Backend                | Support | Notes                                                                                                                       |
| ---------------------- | :-----: | --------------------------------------------------------------------------------------------------------------------------- |
| Metal (macOS)          |    ✓    | MSL via CGo. Full support.                                                                                                  |
| Metal (iOS)            |    ✓    | Same MSL pipeline as macOS.                                                                                                 |
| OpenGL (Linux/Windows) |    ✓    | GLSL 3.30. Up to 32 cached programs.                                                                                        |
| OpenGL ES (Android)    |    ✓    | GLSL ES 3.00 via JNI/CGo.                                                                                                   |
| Web (WASM)             |    ✓    | WebGL2 offscreen canvas composited into Canvas2D. Up to 32 cached programs. Falls back to solid fill if WebGL2 unavailable. |
| SDL2                   |    ✗    | Skipped — no GPU pipeline. Falls back to solid fill.                                                                        |

## 2. The Shader type

```go
// File: gui/shader.go

type Shader struct {
    Metal  string    // MSL fragment body
    GLSL   string    // GLSL fragment body (desktop GL 3.3 and WebGL2 GLSL ES 3.00)
    Params []float32 // up to 16 custom floats, accessible as p0–p3 in the shader
}
```

### ShaderHash

`ShaderHash(s *Shader) uint64` computes an FNV-1a cache key from the
shader source. Uses `s.Metal` on macOS, `s.GLSL` otherwise. The
backends use this to cache compiled shader programs — no recompilation
on every frame.

### BuildGLSLFragment

`BuildGLSLFragment(body string) string` wraps a user-supplied GLSL
body with the standard preamble (version, uniforms, inputs, SDF
rounded-rect alpha) and epilogue. The built-in shaders use this; for
custom shaders set via `Shader.GLSL`, the backend wraps it internally
with the same preamble.

## 3. Writing a fragment shader

### Available inputs

The fragment shader receives these inputs from the vertex stage:

| GLSL name | Metal name      | Type              | Description                                                                                                                 |
| --------- | --------------- | ----------------- | --------------------------------------------------------------------------------------------------------------------------- |
| `uv`      | `in.uv`         | `vec2` / `float2` | Normalized texture coordinates (-1..1 across the quad).                                                                     |
| `color`   | `in.color`      | `vec4` / `float4` | Vertex color (pre-multiplied alpha).                                                                                        |
| `params`  | `in.params`     | `float`           | Packed radius/thickness. Use SDF alpha from the preamble instead.                                                           |
| `p0`–`p3` | `in.p0`–`in.p3` | `vec4` / `float4` | Your custom `Params`, packed 4 floats per vector. `Params[0]` → `p0.x`, `Params[1]` → `p0.y`, …, `Params[4]` → `p1.x`, etc. |
| `tex`     | (sampler)       | `sampler2D`       | A white 1×1 dummy texture. Use for `texture()` lookups if needed.                                                           |

### GLSL body

Keep GLSL syntax compatible with both desktop GL 3.30 and WebGL2
GLSL ES 3.00:

- Declare `vec4 frag_color;` in your body. The preamble's `main()`
  reads it.
- Avoid desktop-only built-ins like `gl_FragCoord`. Use `uv` instead.
- Use `vec2`/`vec3`/`vec4` (not `float2`/`float3`/`float4`).
- Don't redeclare uniforms or inputs — the preamble provides them.

```glsl
// Simple tint shader — multiplies color by a custom factor.
float factor = p0.x;
vec3 tinted = color.rgb * factor;
vec4 frag_color = vec4(tinted, color.a);
```

### Metal (MSL) body

- Use `float2`/`float3`/`float4` Metal types.
- Access uniforms via `in.` prefix: `in.uv`, `in.color`, `in.p0`.
- Declare `float4 frag_color;` in your body.

```metal
// Same tint shader in MSL.
float factor = in.p0.x;
float3 tinted = in.color.rgb * factor;
float4 frag_color = float4(tinted, in.color.a);
```

### SDF rounded-rect alpha

The preamble computes an SDF-based rounded-rect alpha from `params`
(packed radius) and multiplies it with `frag_color.a`:

```glsl
_frag_out = vec4(frag_color.rgb, frag_color.a * sdf_alpha);
```

This means shaders automatically get anti-aliased rounded corners when
the widget has a non-zero `Radius`.

## 4. Applying a shader

Shaders can be set on `ContainerCfg` and `RectangleCfg`:

```go
// Static shader — renders once, no animation.
gui.Column(gui.ContainerCfg{
    Width:  200,
    Height: 200,
    Radius: gui.Some[float32](16),
    Shader: &gui.Shader{
        Metal: `
            float4 frag_color = float4(0.2, 0.4, 0.8, 1.0);
        `,
        GLSL: `
            vec4 frag_color = vec4(0.2, 0.4, 0.8, 1.0);
        `,
    },
    Content: []gui.View{
        gui.Text(gui.TextCfg{Text: "Solid blue"}),
    },
})
```

The shader replaces the container's background fill. Child content
(text, nested widgets) renders on top as normal.

### On ContainerCfg

`ContainerCfg.Shader` replaces the background of any container
(Column, Row, Stack, etc.). The shader fills the container's bounds
including padding.

### On RectangleCfg

`RectangleCfg.Shader` replaces the fill of a basic rectangle. Use when
you need a shader-only surface with no children.

```go
gui.Rectangle(gui.RectangleCfg{
    Width:  100,
    Height: 100,
    Shader: &gui.Shader{
        Metal: `...`,
        GLSL:  `...`,
    },
})
```

## 5. Animation with Params

Update `Params` each frame for time-based effects. Two approaches:

### Animation callback (recommended)

Use a repeating `Animate` to keep the frame loop hot, then compute the
elapsed time in the view function:

```go
type App struct {
    StartTime time.Time
}

func (w *gui.Window) init() {
    w.AnimationAdd(&gui.Animate{
        AnimID:   "shader_tick",
        Repeat:   true,
        Callback: func(_ *gui.Animate, _ *gui.Window) {},
    })
}

func mainView(w *gui.Window) gui.View {
    app := gui.State[App](w)
    elapsed := float32(time.Since(app.StartTime).Milliseconds()) / 1000.0

    return gui.Column(gui.ContainerCfg{
        Shader: &gui.Shader{
            Metal: `
                float t = in.p0.x;
                float2 st = in.uv * 0.5 + 0.5;
                float3 c = 0.5 + 0.5 * cos(t + st.xyx + float3(0,2,4));
                float4 frag_color = float4(c, 1.0);
            `,
            GLSL: `
                float t = p0.x;
                vec2 st = uv * 0.5 + 0.5;
                vec3 c = 0.5 + 0.5 * cos(t + st.xyx + vec3(0,2,4));
                vec4 frag_color = vec4(c, 1.0);
            `,
            Params: []float32{elapsed},
        },
    })
}
```

The repeating `Animate` with an empty callback is a pattern for "keep
the frame loop running." Without it, the view only re-renders on
events (mouse move, key press) and the animation stalls when idle.

### Mutable Params reference

To update `Params` without recreating the `Shader` struct each frame,
keep a reference:

```go
type App struct {
    ShaderParams []float32 // shared backing array
}

// In OnInit:
app.ShaderParams = make([]float32, 1)
// In the view:
app.ShaderParams[0] = elapsed
// Shader.Params: app.ShaderParams
```

The grid layout pipeline copies nothing from the Shader — the pointer
is stable across frames as long as you mutate the backing slice in
place.

### Timing

`time.Since()` in the view function is cheap (monotonic clock read).
For smoother effects, pass frame count or delta time instead:

```go
app.Frame++
app.ShaderParams[0] = float32(app.Frame) / 60.0 // approximate seconds at 60fps
```

## 6. Backend implementation details

### Metal (macOS/iOS)

- MSL source is compiled via CGo (`MTLCompileOptions`,
  `newLibraryWithSource`).
- Pipeline state objects cached by `ShaderHash`.
- `Params` uploaded as a 4×4 float matrix to the vertex shader, which
  passes them through to the fragment stage.

### OpenGL (Linux/Windows)

- `BuildGLSLFragment(s.GLSL)` wraps the user body with the standard
  preamble.
- Vertex shader: `shader.VsCustomGLSL`.
- Compiled program cached up to `maxCustomPipelines = 32`.
- `Params` uploaded as `glUniform4fv` for `p0`–`p3`.

### Web (WASM)

- Creates an offscreen WebGL2 canvas.
- GLSL body is wrapped with WebGL2 ES 3.00 preamble (`#version 300
es`, `out vec4 _frag_out`).
- Renders to offscreen canvas, then composites into the main Canvas2D
  via `drawImage`.
- Falls back to solid fill when WebGL2 is unavailable (older browsers,
  strict CSP).

### SDL2

Custom shaders are skipped. The widget renders with its solid
background color instead.

## 7. Pre-built shaders

The `gui/shader/` package provides GLSL and Metal source constants for
every built-in shader. Useful as reference or as a base for custom
shaders:

| Constant                                 | Purpose                                    |
| ---------------------------------------- | ------------------------------------------ |
| `VsGLSL`, `FsGLSL`                       | Default rect fill with rounded corners.    |
| `VsShadowGLSL`, `FsShadowGLSL`           | Box shadow with Gaussian blur.             |
| `VsBlurGLSL`, `FsBlurGLSL`               | Single-pass directional blur.              |
| `VsGradientGLSL`, `FsGradientGLSL`       | Linear gradient fill.                      |
| `VsCustomGLSL`                           | Vertex shader for custom fragment shaders. |
| `FsFilterBlurHGLSL`, `FsFilterBlurVGLSL` | Separable blur passes.                     |
| `FsFilterColorGLSL`                      | Color matrix filter.                       |
| `FsImageClipGLSL`                        | Image with clip region.                    |

All have corresponding Metal constants (`VsMetal`, `FsMetal`, etc. in
`gui/shader/metal.go`).

## 8. Reference example

Full working example: `examples/custom_shader/main.go` — two animated
shader squares (Rainbow and Plasma) with time-based `Params`, repeating
animation tick, and both Metal + GLSL shader bodies.

## Checklist

- [ ] Provide both `Metal` and `GLSL` shader bodies. The framework
      picks the right one at runtime.
- [ ] Keep GLSL syntax compatible with desktop GL 3.30 and WebGL2
      GLSL ES 3.00. No `gl_FragCoord`, no desktop-only built-ins.
- [ ] Declare `vec4 frag_color;` / `float4 frag_color;` in your
      shader body — the preamble reads it to produce the final output.
- [ ] `Params` length ≤ 16. The vertex shader packs 4 floats per
      vector (p0–p3).
- [ ] Set up a repeating `Animate` callback to keep the frame loop hot
      for time-based effects.
- [ ] Test on all target backends. The SDL2 backend skips custom
      shaders — provide a fallback background color for those users.
- [ ] Shader compilation failures are silent (logged to stderr). Test
      with a known-good shader first.
