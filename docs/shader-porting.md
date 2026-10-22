# Porting a Shadertoy shader

`cmd/shaderport` converts a Shadertoy shader into a Go file. The file declares a
`*gui.Shader` with both bodies: `Metal` for macOS and iOS, and `GLSL` for
desktop OpenGL, WebGL2 and Android. You do not write the shader twice.

The tool runs before you build, not in your app. Its output is Go source that
you read and commit. If the translation is wrong, you see it in the diff, not as
a blank container at run time.

## Port a shader

1. Copy the shader code from Shadertoy into a file, for example `rings.frag`.
2. Run the tool:

   ```sh
   go run github.com/go-gui-org/go-gui/cmd/shaderport@latest \
       -name ringsShader -o rings_shader.go rings.frag
   ```

3. Read `rings_shader.go`. Compare the `GLSLDecls` field with your source.
4. Set the variable as the `Shader` of a container.

If the tool cannot translate a construct, it writes no file. It stops with the
line, the column and the reason, for example:

```text
rings.frag:14:12: unsupported: iMouse is not supported yet: ...
```

To keep the Go file current, put a `go:generate` line next to the code that uses
it:

```go
//go:generate go run github.com/go-gui-org/go-gui/cmd/shaderport -name ringsShader -o rings_shader.go rings.frag
```

Flags:

| Flag    | Default  | Meaning                                      |
| ------- | -------- | -------------------------------------------- |
| `-name` | `shader` | Name of the `*gui.Shader` variable           |
| `-pkg`  | `main`   | Package of the generated file                |
| `-o`    | stdout   | Output file. With no input file, reads stdin |

## Use the generated shader

`examples/custom_shader` ports `rings.frag` this way. The tile only refers to
the generated variable:

```go
// ringsTile draws a Shadertoy shader ported by cmd/shaderport.
// rings.frag is the source; go generate writes ringsShader, with
// Animate already set because the shader reads iTime.
func ringsTile() gui.View {
	return gui.Column(gui.ContainerCfg{
		Width:   180,
		Height:  180,
		Sizing:  gui.FixedFixed,
		Radius:  gui.RadiusPx(16), // ergonomics-audit:spacing — decorative radius, not a control corner
		HAlign:  gui.HAlignCenter,
		VAlign:  gui.VAlignMiddle,
		Shader:  ringsShader,
		Content: []gui.View{gui.Text(gui.TextCfg{Text: "Rings"})},
	})
}
```

The tool sets `Animate` when the shader reads `iTime`, `iTimeDelta` or `iFrame`.
Then the container draws a new frame each display refresh. No view function runs
and no layout runs for these frames.

## What the tool supports

The input is GLSL ES 3.00, the language of Shadertoy. The tool accepts these
items:

- A `mainImage(out vec4, in vec2)` function. A go-gui body that declares
  `frag_color` is also accepted (see `gui.Shader`).
- Helper functions, prototypes and structs.
- `const` globals, including `const` arrays.
- `#define` macros, including function-like and multi-line macros, and
  `#if`/`#ifdef` blocks.
- Scalar, vector and matrix types, swizzles, `out` and `inout` parameters, and
  `if`, `for` and `while`.
- The built-in inputs `iResolution`, `iTime`, `iTimeDelta` and `iFrame`.

The MSL output changes these items:

| GLSL                                    | MSL                                        |
| --------------------------------------- | ------------------------------------------ |
| `vec3`, `ivec2`, `mat2`, `mat3x4`       | `float3`, `int2`, `float2x2`, `float3x4`   |
| `out vec3 n`, `inout vec2 p`            | `thread float3& n`, `thread float2& p`     |
| `const float A = ...` at program scope  | `constant float A = ...`                   |
| `float[3](a, b, c)`                     | `{a, b, c}`                                |
| `S(a, b)` for a struct `S`              | `S{a, b}`                                  |
| `atan(y, x)`                            | `atan2(y, x)`                              |
| `mod(x, y)`                             | `_gg_mod(x, y)`, which floors as GLSL does |
| `dFdx`, `dFdy`, `inversesqrt`           | `dfdx`, `dfdy`, `rsqrt`                    |
| `lessThan(a, b)` and the other compares | `(a < b)`                                  |
| `v.st`, `v.pq`                          | `v.xy`, `v.zw`                             |
| `discard`                               | `discard_fragment()`                       |

MSL `fmod` truncates toward zero. GLSL `mod` floors. For a negative value the
two give different results, so the tool adds `_gg_mod` and does not use `fmod`.

In MSL the built-in inputs are local variables of the fragment function. A
helper function cannot read them. Thus the tool adds each input that a helper
reads, directly or through a call or a macro, as a parameter. It also adds the
input to each call.

Some names are free in GLSL but reserved in MSL (`kernel`, `device`, `float2`)
or declared by the go-gui wrapper (`color`, `uv`, `tex`). The tool adds `_` to
these names. A function that has the name of an MSL library function, such as
`saturate` or `rotate`, also gets the `_`.

## What the tool rejects

| Construct                                      | Reason                                          |
| ---------------------------------------------- | ----------------------------------------------- |
| `iMouse`                                       | go-gui does not send the pointer to shaders yet |
| `iChannel0`-`iChannel3`, `texture()`, samplers | go-gui binds no textures to a custom shader     |
| `iDate`, `iSampleRate`, `iFrameRate`           | go-gui does not supply them                     |
| A global variable that is not `const`          | MSL has no mutable program-scope variables      |
| `uniform`, `in` and `out` globals              | Use the built-in inputs or `Shader.Params`      |
| `gl_FragCoord`, `gl_FragColor`, `void main()`  | Use `mainImage` and its parameters              |
| `inverse`, `outerProduct`, `matrixCompMult`    | MSL has no built-in function for them           |
| `floatBitsToInt` and the other bit casts       | Not in the subset yet                           |
| `^^`                                           | Write `a != b`                                  |
| `T[N] name` array declarations                 | Write `T name[N]`                               |
| An `out` or `inout` array parameter            | Not in the subset                               |
| `#include`                                     | Shadertoy has no includes                       |

A shader with a mutable global usually sets the global in `mainImage` and reads
it in helpers. Make it a local variable of `mainImage`. Then add it as a
parameter to the helpers that read it.

## Differences from Shadertoy

- **Coordinates.** Shadertoy gives `fragCoord` in pixels, with y up from the
  bottom edge. The tool computes `fragCoord` from go-gui's `uv`, so the shader
  sees the same values. If you write a go-gui body by hand, remember that `uv`
  spans -1 to 1 across the container, with y down, not 0 to 1.
- **Alpha.** Shadertoy ignores the alpha of `fragColor`. go-gui blends with it.
  The generated body sets alpha to 1, so a shader that leaves alpha at 0 still
  shows.
- **Corners.** The wrapper clips the shader to the rounded corners of the
  container.
- **Headless capture.** The `soft` backend has no shader compiler. A screenshot
  from `soft.RenderToPNG` shows the container without the shader. In a headless
  capture, and for a user who asked for reduced motion, the time inputs read
  zero.

## How the tool works

The tool is a token rewriter, not a compiler. It keeps your layout, comments and
macros. GLSL ES 3.00 has no implicit type conversions, and MSL accepts the mixed
vector and scalar calls of GLSL (`mix(vec3, vec3, float)`, `max(vec3, float)`).
Thus the tool does not need type information. It uses the declared type of a
variable in one case only: to tell a struct field named `s` or `t` from a
swizzle.

The tests compile every fixture in `internal/shaderport/testdata`. The MSL
compiles with the Metal compiler (`TestShaderportCompiles`). The GLSL compiles
with `glslangValidator` as GLSL 3.30 and as GLSL ES 3.00
(`TestPortGLSLCompiles`).
