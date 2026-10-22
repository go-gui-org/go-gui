// Ported by shaderport from rings.frag.
// Review it before use: see docs/shader-porting.md for what the
// translation covers.

package main

import "github.com/go-gui-org/go-gui/gui"

// ringsShader draws the ported shader. Set it as a container's Shader.
var ringsShader = &gui.Shader{
	Metal: `
float4 frag_color = float4(0.0);
mainImage(frag_color, float2((in.uv.x * 0.5 + 0.5) * iResolution.x, (0.5 - in.uv.y * 0.5) * iResolution.y), iResolution, iTime);
frag_color.a = 1.0;
`,
	MetalDecls: "// Concentric rings that drift outward, written as a Shadertoy shader.\n// rings_shader.go is generated from this file by cmd/shaderport:\n// run `go generate` after editing it.\n#define PI 3.14159265\n\nfloat3 palette(float t)\n{\n    return 0.5 + 0.5 * cos(2.0 * PI * (t + float3(0.0, 0.33, 0.67)));\n}\n\nfloat rings(float2 p, float iTime)\n{\n    float r = length(p);\n    return 0.5 + 0.5 * sin(24.0 * r - 3.0 * iTime);\n}\n\nvoid mainImage(thread float4& fragColor, float2 fragCoord, float3 iResolution, float iTime)\n{\n    float2 p = (2.0 * fragCoord - iResolution.xy) / iResolution.y;\n    float a = atan2(p.y, p.x) / (2.0 * PI);\n    float3 col = palette(a + 0.1 * iTime) * rings(p, iTime);\n    fragColor = float4(col, 1.0);\n}\n",
	GLSL: `
vec4 frag_color = vec4(0.0);
mainImage(frag_color, vec2((uv.x * 0.5 + 0.5) * iResolution.x, (0.5 - uv.y * 0.5) * iResolution.y));
frag_color.a = 1.0;
`,
	GLSLDecls: "// Concentric rings that drift outward, written as a Shadertoy shader.\n// rings_shader.go is generated from this file by cmd/shaderport:\n// run `go generate` after editing it.\n#define PI 3.14159265\n\nvec3 palette(float t)\n{\n    return 0.5 + 0.5 * cos(2.0 * PI * (t + vec3(0.0, 0.33, 0.67)));\n}\n\nfloat rings(vec2 p)\n{\n    float r = length(p);\n    return 0.5 + 0.5 * sin(24.0 * r - 3.0 * iTime);\n}\n\nvoid mainImage(out vec4 fragColor, in vec2 fragCoord)\n{\n    vec2 p = (2.0 * fragCoord - iResolution.xy) / iResolution.y;\n    float a = atan(p.y, p.x) / (2.0 * PI);\n    vec3 col = palette(a + 0.1 * iTime) * rings(p);\n    fragColor = vec4(col, 1.0);\n}\n",
	Animate:   true,
}
