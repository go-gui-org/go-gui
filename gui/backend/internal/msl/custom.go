package msl

// BuildCustom produces a complete MSL source with vertex and
// fragment shaders for a custom shader body. The macOS and iOS
// backends share this so their wrappers cannot drift apart.
//
// The body must declare a local float4 frag_color; the wrapper
// adds the SDF round-rect clipping around it.
//
// NOTE: The custom VS uses standalone buffer(1)/buffer(2)
// bindings instead of the Uniforms struct at buffer(0) used by
// built-in shaders. The C layer (metalSetCustomPipeline) sets up
// matching bindings for custom pipelines.
func BuildCustom(body string) string {
	return `#include <metal_stdlib>
using namespace metal;

struct VertexIn {
    float3 position [[attribute(0)]];
    float2 texcoord [[attribute(1)]];
    float4 color    [[attribute(2)]];
};

struct VertexOut {
    float4 position [[position]];
    float2 uv;
    float4 color;
    float  params;
    float4 p0;
    float4 p1;
    float4 p2;
    float4 p3;
};

vertex VertexOut vs_main(
    VertexIn in [[stage_in]],
    constant float4x4 &mvp [[buffer(1)]],
    constant float4x4 &tm  [[buffer(2)]]
) {
    VertexOut out;
    out.position = mvp * float4(in.position.xy, 0.0, 1.0);
    out.uv       = in.texcoord;
    out.color    = in.color;
    out.params   = in.position.z;
    out.p0       = tm[0];
    out.p1       = tm[1];
    out.p2       = tm[2];
    out.p3       = tm[3];
    return out;
}

fragment float4 fs_main(
    VertexOut in [[stage_in]],
    texture2d<float> tex [[texture(0)]],
    sampler smp [[sampler(0)]]
) {
    float radius = floor(in.params / 4096.0) / 4.0;

    float2 width_inv = float2(fwidth(in.uv.x), fwidth(in.uv.y));
    float2 half_size = 1.0 / (width_inv + 1e-6);
    float2 pos = in.uv * half_size;

    float2 q = abs(pos) - half_size + float2(radius);
    float2 max_q = max(q, float2(0.0));
    float d = length(max_q) + min(max(q.x, q.y), 0.0) - radius;

    float grad_len = length(float2(dfdx(d), dfdy(d)));
    d = d / max(grad_len, 0.001);
    float sdf_alpha = 1.0 - smoothstep(-0.59, 0.59, d);

    // --- user body ---
    ` + body + `
    // --- end user body ---

    frag_color = float4(frag_color.rgb, frag_color.a * sdf_alpha);

    if (frag_color.a < 0.0) {
        frag_color += tex.sample(smp, in.uv);
    }
    return frag_color;
}
`
}
