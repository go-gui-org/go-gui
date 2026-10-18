Apply custom fragment shaders (Metal + GLSL) to any container. Write only the
color-computation body — the framework wraps it with struct definitions,
Shadertoy's built-in inputs, SDF round-rect clipping, and pipeline caching via
`BuildGLSLFragment`.

## Static Shader

```go
gui.Column(gui.ContainerCfg{
    Width: 200, Height: 200,
    Sizing: gui.FixedFixed,
    Radius: gui.RadiusMedium,
    Shader: &gui.Shader{
        Metal: `
            float2 st = in.uv * 0.5 + 0.5;
            float4 frag_color = float4(st.x, st.y, 0.5, 1.0);
        `,
        GLSL: `
            vec2 st = uv * 0.5 + 0.5;
            vec4 frag_color = vec4(st.x, st.y, 0.5, 1.0);
        `,
    },
})
```

## Animated Shader

Read `iTime` and set `Animate`. The window then draws a new frame each display
refresh while the container is on screen. Each frame rebuilds only the render
commands: no view function runs and nothing is re-arranged. Frames stop in the
first frame that draws no `Animate` shader. For a user who asked for reduced
motion the shader holds still instead, as in a headless capture: its clock reads
zero and no frames start. This is from `examples/custom_shader`:

```go
// rainbowTile is an animated shader. iTime counts seconds from the
// first frame that drew a shader. Animate keeps frames coming while
// the tile is on screen, and each frame rebuilds only the render
// commands: no view function runs and nothing is re-arranged.
func rainbowTile() gui.View {
	return gui.Column(gui.ContainerCfg{
		Width:  180,
		Height: 180,
		Sizing: gui.FixedFixed,
		Radius: gui.RadiusPx(16), // ergonomics-audit:spacing — decorative radius, not a control corner
		HAlign: gui.HAlignCenter,
		VAlign: gui.VAlignMiddle,
		Shader: &gui.Shader{
			Metal: `
				float2 st = in.uv * 0.5 + 0.5;
				float3 c = 0.5 + 0.5 * cos(iTime + st.xyx + float3(0,2,4));
				float4 frag_color = float4(c, 1.0);
			`,
			GLSL: `
				vec2 st = uv * 0.5 + 0.5;
				vec3 c = 0.5 + 0.5 * cos(iTime + st.xyx + vec3(0,2,4));
				vec4 frag_color = vec4(c, 1.0);
			`,
			Animate: true,
		},
		Content: []gui.View{gui.Text(gui.TextCfg{Text: "Rainbow"})},
	})
}

```

## Built-in inputs

| Name          | Type          | Value                                            |
| ------------- | ------------- | ------------------------------------------------ |
| `iResolution` | `vec3/float3` | Container size in physical pixels; `z` is 1      |
| `iTime`       | `float`       | Seconds since the first frame that drew a shader |
| `iTimeDelta`  | `float`       | Seconds since the previous frame, at most 0.25   |
| `iFrame`      | `int`         | Frames since the first frame that drew a shader  |
| `iMouse`      | `vec4/float4` | Always zero: the mouse is not tracked            |

The names, types and units are Shadertoy's, so copied code reads them unchanged.
Sizes are physical pixels. `iMouse` reads zero, Shadertoy's value for a canvas
never clicked, so a shader that reads it compiles and draws as if never clicked.
Time and frame advance once per displayed frame, and read zero in headless
captures and for a user who asked for reduced motion. A body that declares its
own `iTime` hides the built-in and still compiles.

## Passing your own values

`Params` carries up to 16 floats of your own. Each float maps to p0.x, p0.y,
p0.z, p0.w, p1.x, ... Params are set in the view function, so a change reaches
the shader with the next view rebuild.

## Shader Properties

| Property | Type      | Description                                             |
| -------- | --------- | ------------------------------------------------------- |
| Metal    | string    | MSL fragment body                                       |
| GLSL     | string    | GLSL fragment body (desktop GL 3.3 / WebGL2-compatible) |
| Params   | []float32 | Up to 16 custom floats                                  |
| Animate  | bool      | Redraw every frame while the shader is drawn            |

## Available Inputs

Besides the built-ins above:

| Metal        | GLSL         | Type        | Description           |
| ------------ | ------------ | ----------- | --------------------- |
| in.uv        | uv           | float2/vec2 | -1..1 centered coords |
| in.color     | color        | float4/vec4 | Vertex color          |
| in.p0..in.p3 | p0..p3       | float4/vec4 | Custom params         |
| in.position  | gl_FragCoord | float4/vec4 | Screen position       |

## Output

Declare a local `float4 frag_color` (Metal) or `vec4 frag_color` (GLSL). The
framework applies SDF clipping automatically.

## Porting from Shadertoy

- Replace `mainImage(out vec4 fragColor, in vec2 fragCoord)` with a body that
  declares `frag_color`. Rebuild `fragCoord` from `uv`, which spans -1..1 with y
  growing down:
  `vec2 fragCoord = vec2(uv.x * 0.5 + 0.5, 0.5 - uv.y * 0.5) * iResolution.xy;`
- Write the Metal body as well: `vec2` becomes `float2`, `mat4` becomes
  `float4x4`, `mod` becomes `fmod`, `atan(y, x)` becomes `atan2(y, x)`, and `uv`
  becomes `in.uv`.
- A body is the inside of one function, so it cannot declare helper functions.
  Inline them.
- `iChannel0..3`, `iDate` and `iSampleRate` are not provided.
- Set `Animate` if the shader changes over time.

## Notes

- Must provide both Metal and GLSL bodies for cross-platform. A shader with
  neither body is dropped by render validation; each backend compiles only its
  own body and ignores the other.
- Bodies are trusted app source spliced into the shader as-is: no `#version` or
  `main`, and never built from untrusted input.
- Params past the first 16 are ignored; NaN or Inf params drop the draw.
- Pipeline is compiled once per unique source and cached (`ShaderHash` computes
  the cache key). A body that fails to compile compiles once, not once per
  frame, and then follows the fallback contract below.
- Shader fill takes priority over Gradient and solid Color
- Set `Animate` to keep the frame loop running; an app adds no animation of its
  own.

## Failure contract

What a missing GPU feature or a broken body draws, per backend:

| Backend            | No WebGL2 / GL context | Body fails to compile |
| ------------------ | ---------------------- | --------------------- |
| Web                | Solid fill             | Solid fill            |
| Desktop GL, Metal, | Nothing                | Nothing               |
| iOS, Android       |                        |                       |
| Soft (headless)    | Nothing                | Nothing               |

The web solid fill is the widget color, or the theme's active color when the
widget sets none. The soft backend has no GPU and always skips custom shaders,
so screenshots render them as empty.
