Apply custom fragment shaders (Metal + GLSL) to any container.
Write only the color-computation body — the framework wraps it with
struct definitions, SDF round-rect clipping, and pipeline caching
via `BuildGLSLFragment`.

## Static Shader

```go
gui.Column(gui.ContainerCfg{
    Width: 200, Height: 200,
    Sizing: gui.FixedFixed,
    Radius: gui.SomeF(8),
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

Pass time or other values via Params. Each float maps to
p0.x, p0.y, p0.z, p0.w, p1.x, ... (up to 16 floats).

```go
elapsed := float32(time.Since(startTime).Milliseconds()) / 1000.0

gui.Column(gui.ContainerCfg{
    Width: 200, Height: 200,
    Sizing: gui.FixedFixed,
    Radius: gui.SomeF(16),
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
```

## Shader Properties

| Property | Type      | Description                          |
|----------|-----------|--------------------------------------|
| Metal    | string    | MSL fragment body                    |
| GLSL     | string    | GLSL fragment body (desktop GL 3.3 / WebGL2-compatible) |
| Params   | []float32 | Up to 16 custom floats               |

## Available Inputs

| Metal        | GLSL         | Type        | Description            |
|--------------|--------------|-------------|------------------------|
| in.uv        | uv           | float2/vec2 | -1..1 centered coords  |
| in.color     | color        | float4/vec4 | Vertex color           |
| in.p0..in.p3 | p0..p3       | float4/vec4 | Custom params          |
| in.position  | gl_FragCoord | float4/vec4 | Screen position        |

## Output

Declare a local `float4 frag_color` (Metal) or `vec4 frag_color`
(GLSL). The framework applies SDF clipping automatically.

## Notes

- Must provide both Metal and GLSL bodies for cross-platform
- Pipeline is compiled once per unique source and cached
  (`ShaderHash` computes the cache key)
- Shader fill takes priority over Gradient and solid Color
- Add a repeating animation to keep the frame loop hot
