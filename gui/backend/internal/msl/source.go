// Package msl holds the Metal Shading Language source shared by the
// macOS backend (gui/backend/metal) and the iOS backend
// (gui/backend/ios).
//
// These backends carried byte-identical 428-line copies of this
// shader until issue 126 — a Metal 3.2-only lambda that broke every
// app on macOS Sonoma — had to be fixed in both of them.
//
// It lives in Go rather than a shared .h on purpose. The Go build
// cache only tracks headers inside a package directory (see
// `go list -f '{{.HFiles}}'`), so a shared header outside both
// packages is invisible to it: editing the shader would silently
// reuse a stale binary. A Go constant is tracked correctly, and the
// backends pass it across the cgo boundary at context creation.
//
// Compatibility floor: this source must compile at MSL 3.0
// (macOS 13 Ventura), which is pinned via MTLCompileOptions at both
// call sites. Constructs from a newer Metal — lambdas being the one
// that bit us — compile clean on a current Mac and fail at runtime on
// every older one. TestBuiltinShadersCompile guards this.
package msl

// Source is the MSL for the built-in render pipelines. Kept as one
// string so both backends provably compile the same shader.
const Source = `
#include <metal_stdlib>
using namespace metal;

// ── Common Structs ──

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
};

struct ShadowOut {
    float4 position [[position]];
    float2 uv;
    float4 color;
    float  params;
    float2 offset;
    float  spread;
};

struct BlurOut {
    float4 position [[position]];
    float2 uv;
    float4 color;
    float  params;
};

struct GradientOut {
    float4 position [[position]];
    float2 uv;
    float4 color;
    float  params;
    float4 stop12;
    float4 stop34;
    float4 stop56;
    float4 meta;
};

struct FilterOut {
    float4 position [[position]];
    float2 uv;
    float4 color;
    float  std_dev;
};

struct GlyphIn {
    float2 position [[attribute(0)]];
    float2 texcoord [[attribute(1)]];
    float4 color    [[attribute(2)]];
};

struct GlyphOut {
    float4 position [[position]];
    float2 uv;
    float4 color;
};

// ── Vertex Shaders ──

vertex VertexOut vs_solid(
    VertexIn in [[stage_in]],
    constant float4x4 &mvp [[buffer(1)]]
) {
    VertexOut out;
    out.position = mvp * float4(in.position.xy, 0.0, 1.0);
    out.uv       = in.texcoord;
    out.color    = in.color;
    out.params   = in.position.z;
    return out;
}

vertex ShadowOut vs_shadow(
    VertexIn in [[stage_in]],
    constant float4x4 &mvp [[buffer(1)]],
    constant float4x4 &tm  [[buffer(2)]]
) {
    ShadowOut out;
    out.position = mvp * float4(in.position.xy, 0.0, 1.0);
    out.uv       = in.texcoord;
    out.color    = in.color;
    out.params   = in.position.z;
    out.offset   = (tm * float4(0, 0, 0, 1)).xy;
    out.spread   = (tm * float4(0, 0, 0, 1)).z;
    return out;
}

vertex GradientOut vs_gradient(
    VertexIn in [[stage_in]],
    constant float4x4 &mvp [[buffer(1)]],
    constant float4x4 &tm  [[buffer(2)]]
) {
    GradientOut out;
    out.position = mvp * float4(in.position.xy, 0.0, 1.0);
    out.uv       = in.texcoord;
    out.color    = in.color;
    out.params   = in.position.z;
    out.stop12   = tm[0];
    out.stop34   = tm[1];
    out.stop56   = tm[2];
    out.meta     = tm[3];
    return out;
}

vertex FilterOut vs_filter(
    VertexIn in [[stage_in]],
    constant float4x4 &mvp [[buffer(1)]],
    constant float4x4 &tm  [[buffer(2)]]
) {
    FilterOut out;
    out.position = mvp * float4(in.position.xy, 0.0, 1.0);
    out.uv       = in.texcoord;
    out.color    = in.color;
    out.std_dev  = tm[0][0];
    return out;
}

vertex GlyphOut vs_glyph(
    GlyphIn in [[stage_in]],
    constant float4x4 &mvp [[buffer(1)]]
) {
    GlyphOut out;
    out.position = mvp * float4(in.position, 0.0, 1.0);
    out.uv       = in.texcoord;
    out.color    = in.color;
    return out;
}

// ── Fragment Shaders ──

fragment float4 fs_solid(VertexOut in [[stage_in]]) {
    float radius    = floor(in.params / 4096.0) / 4.0;
    float thickness = fmod(in.params, 4096.0) / 4.0;

    float2 uv_to_px  = 1.0 / (float2(fwidth(in.uv.x),
                        fwidth(in.uv.y)) + 1e-6);
    float2 half_size  = uv_to_px;
    float2 pos        = in.uv * half_size;

    float2 q = abs(pos) - half_size + float2(radius);
    float d = length(max(q, float2(0.0)))
            + min(max(q.x, q.y), 0.0) - radius;

    if (thickness > 0.0) {
        d = abs(d + thickness * 0.5) - thickness * 0.5;
    }

    float grad_len = length(float2(dfdx(d), dfdy(d)));
    d = d / max(grad_len, 0.001);
    float alpha = 1.0 - smoothstep(-0.59, 0.59, d);
    return float4(in.color.rgb, in.color.a * alpha);
}

fragment float4 fs_shadow(ShadowOut in [[stage_in]]) {
    float radius = floor(in.params / 4096.0) / 4.0;
    float blur   = fmod(in.params, 4096.0) / 4.0;

    float2 uv_to_px = 1.0 / (float2(fwidth(in.uv.x),
                       fwidth(in.uv.y)) + 1e-6);
    float2 half_size = uv_to_px;
    float2 pos       = in.uv * half_size;

    // Shadow field: the quad and vertex radius are already inflated
    // by spread, so the shadow grows beyond the caster.
    float2 q = abs(pos) - half_size
             + float2(radius + 1.5 * blur);
    float d = length(max(q, float2(0.0)))
            + min(max(q.x, q.y), 0.0) - radius;

    // Caster cut-out: half_size and radius shrink by spread to trace
    // the caster's un-inflated extent.
    float  radius_caster = radius - in.spread;
    float2 q_c = abs(pos + in.offset) - (half_size - in.spread)
               + float2(radius_caster + 1.5 * blur);
    float d_c = length(max(q_c, float2(0.0)))
              + min(max(q_c.x, q_c.y), 0.0) - radius_caster;

    // Falloff, centred on the shadow's own edge. A Gaussian blur of a
    // hard-edged rect — which is what the soft backend and a canvas
    // shadowBlur both produce — is ~50% AT the edge and decays over
    // roughly +/- one blur radius with sigma = blur/2. Ramping from
    // full opacity at the edge instead (the old 0..blur form) is twice
    // the ink over twice the width, and reads as a grey cloud rather
    // than as depth. smoothstep over -blur..+blur tracks that Gaussian
    // CDF closely: at d = sigma it gives 0.16 against the Gaussian's
    // 0.159. The blur pipeline below already uses this same form.
    float b_half        = max(1.0, blur);
    float alpha_falloff = 1.0 - smoothstep(-b_half, b_half, d);
    float alpha_clip    = smoothstep(-1.0, 0.0, d_c);
    float alpha         = alpha_falloff * alpha_clip;

    return float4(in.color.rgb, in.color.a * alpha);
}

vertex BlurOut vs_blur(
    VertexIn in [[stage_in]],
    constant float4x4 &mvp [[buffer(1)]]
) {
    BlurOut out;
    out.position = mvp * float4(in.position.xy, 0.0, 1.0);
    out.uv       = in.texcoord;
    out.color    = in.color;
    out.params   = in.position.z;
    return out;
}

fragment float4 fs_blur(BlurOut in [[stage_in]]) {
    float radius = floor(in.params / 4096.0) / 4.0;
    float blur   = fmod(in.params, 4096.0) / 4.0;

    float2 uv_to_px = 1.0 / (float2(fwidth(in.uv.x),
                       fwidth(in.uv.y)) + 1e-6);
    float2 half_size = uv_to_px;
    float2 pos       = in.uv * half_size;

    float2 q = abs(pos) - half_size
             + float2(radius + 1.5 * blur);
    float d = length(max(q, float2(0.0)))
            + min(max(q.x, q.y), 0.0) - radius;

    float alpha = 1.0 - smoothstep(-blur, blur, d);
    return float4(in.color.rgb, in.color.a * alpha);
}

// Unpack one packed gradient stop. Two floats carry RGBA plus the
// stop position: val1 packs r/g/b as base-256 digits, val2 packs
// alpha in the low digit and position * 10000 above it.
//
// Deliberately a plain function, not a lambda. Lambda expressions are
// Metal 3.2+ (macOS 15 Sequoia); on Sonoma and earlier they fail to
// compile, and because this library is built once at context init,
// that broke *every* app — not just ones drawing gradients. Issue 126.
static void unpack_stop(float val1, float val2,
                        thread float4 &c, thread float &p) {
    float r = fmod(val1, 256.0);
    float g = fmod(floor(val1 / 256.0), 256.0);
    float b = floor(val1 / 65536.0);
    float a = fmod(val2, 256.0);
    p = floor(val2 / 256.0) / 10000.0;
    c = float4(r/255.0, g/255.0, b/255.0, a/255.0);
}

fragment float4 fs_gradient(GradientOut in [[stage_in]]) {
    float radius = floor(in.params / 4096.0) / 4.0;

    float hw = in.meta.x;
    float hh = in.meta.y;
    float grad_type = in.meta.z;
    int stop_count  = int(in.meta.w);

    float2 pos = in.uv * float2(hw, hh);

    float2 q = abs(pos) - float2(hw, hh) + float2(radius);
    float d = length(max(q, float2(0.0)))
            + min(max(q.x, q.y), 0.0) - radius;

    float grad_len = length(float2(dfdx(d), dfdy(d)));
    d = d / max(grad_len, 0.001);
    float sdf_alpha = 1.0 - smoothstep(-0.59, 0.59, d);

    float t;
    if (grad_type > 0.5) {
        float target_radius = in.stop56.w;
        t = length(pos) / target_radius;
    } else {
        float2 stop_dir = float2(in.stop56.z, in.stop56.w);
        t = dot(in.uv, stop_dir) * 0.5 + 0.5;
    }
    t = clamp(t, 0.0, 1.0);

    // Unpack gradient stops.
    float4 stop_colors[6];
    float  stop_positions[6];

    unpack_stop(in.stop12.x, in.stop12.y,
                stop_colors[0], stop_positions[0]);
    unpack_stop(in.stop12.z, in.stop12.w,
                stop_colors[1], stop_positions[1]);
    unpack_stop(in.stop34.x, in.stop34.y,
                stop_colors[2], stop_positions[2]);
    unpack_stop(in.stop34.z, in.stop34.w,
                stop_colors[3], stop_positions[3]);
    unpack_stop(in.stop56.x, in.stop56.y,
                stop_colors[4], stop_positions[4]);
    stop_colors[5]    = stop_colors[4];
    stop_positions[5] = stop_positions[4];

    float4 c1 = stop_colors[0];
    float4 c2 = c1;
    float  p1 = stop_positions[0];
    float  p2 = p1;

    for (int i = 1; i < 6; i++) {
        if (i >= stop_count) break;
        if (t <= stop_positions[i]) {
            c2 = stop_colors[i];
            p2 = stop_positions[i];
            c1 = stop_colors[i-1];
            p1 = stop_positions[i-1];
            break;
        }
        if (i == stop_count - 1) {
            c1 = stop_colors[i];
            c2 = c1;
            p1 = stop_positions[i];
            p2 = p1;
        }
    }

    float local_t = (t - p1) / max(p2 - p1, 0.0001);

    float3 c1_pre  = c1.rgb * c1.a;
    float3 c2_pre  = c2.rgb * c2.a;
    float3 rgb_pre = mix(c1_pre, c2_pre, local_t);
    float  alpha   = mix(c1.a, c2.a, local_t);
    float3 rgb     = rgb_pre / max(alpha, 0.0001);
    float4 gc      = float4(rgb, alpha);

    // Dithering to reduce banding.
    float2 fc = float2(in.position.xy);
    float dither = fract(sin(dot(fc, float2(12.9898, 78.233)))
                   * 43758.5453) - 0.5;
    gc.rgb += float3(dither / 255.0);

    return float4(gc.rgb, gc.a * sdf_alpha * in.color.a);
}

fragment float4 fs_image_clip(
    VertexOut in [[stage_in]],
    texture2d<float> tex [[texture(0)]],
    sampler smp [[sampler(0)]]
) {
    float radius = floor(in.params / 4096.0) / 4.0;

    float2 uv_to_px = 1.0 / (float2(fwidth(in.uv.x),
                       fwidth(in.uv.y)) + 1e-6);
    float2 half_size = uv_to_px;
    float2 pos       = in.uv * half_size;

    float2 q = abs(pos) - half_size + float2(radius);
    float d = length(max(q, float2(0.0)))
            + min(max(q.x, q.y), 0.0) - radius;

    float grad_len = length(float2(dfdx(d), dfdy(d)));
    d = d / max(grad_len, 0.001);
    float alpha = 1.0 - smoothstep(-0.59, 0.59, d);

    float2 tex_uv   = in.uv * 0.5 + 0.5;
    float4 tex_color = tex.sample(smp, tex_uv);
    return float4(tex_color.rgb, tex_color.a * alpha);
}

fragment float4 fs_filter_blur_h(
    FilterOut in [[stage_in]],
    texture2d<float> tex [[texture(0)]],
    sampler smp [[sampler(0)]]
) {
    constexpr float w[7] = {
        0.19947, 0.17603, 0.12098,
        0.06476, 0.02700, 0.00877, 0.00222
    };
    float tex_w    = tex.get_width();
    float step_sz  = in.std_dev / tex_w;

    float4 c = tex.sample(smp, in.uv) * w[0];
    for (int i = 1; i < 7; i++) {
        float off = float(i) * step_sz;
        c += tex.sample(smp, in.uv + float2(off, 0)) * w[i];
        c += tex.sample(smp, in.uv - float2(off, 0)) * w[i];
    }
    return c;
}

fragment float4 fs_filter_blur_v(
    FilterOut in [[stage_in]],
    texture2d<float> tex [[texture(0)]],
    sampler smp [[sampler(0)]]
) {
    constexpr float w[7] = {
        0.19947, 0.17603, 0.12098,
        0.06476, 0.02700, 0.00877, 0.00222
    };
    float tex_h    = tex.get_height();
    float step_sz  = in.std_dev / tex_h;

    float4 c = tex.sample(smp, in.uv) * w[0];
    for (int i = 1; i < 7; i++) {
        float off = float(i) * step_sz;
        c += tex.sample(smp, in.uv + float2(0, off)) * w[i];
        c += tex.sample(smp, in.uv - float2(0, off)) * w[i];
    }
    return c;
}

fragment float4 fs_filter_tex(
    FilterOut in [[stage_in]],
    texture2d<float> tex [[texture(0)]],
    sampler smp [[sampler(0)]]
) {
    return tex.sample(smp, in.uv) * in.color;
}

fragment float4 fs_filter_color(
    FilterOut in [[stage_in]],
    texture2d<float> tex [[texture(0)]],
    sampler smp [[sampler(0)]],
    constant float4x4 &cm [[buffer(0)]]
) {
    float4 src = tex.sample(smp, in.uv);
    return clamp(cm * src, 0.0, 1.0);
}

fragment float4 fs_glyph_tex(
    GlyphOut in [[stage_in]],
    texture2d<float> tex [[texture(0)]],
    sampler smp [[sampler(0)]]
) {
    return tex.sample(smp, in.uv) * in.color;
}

fragment float4 fs_glyph_color(GlyphOut in [[stage_in]]) {
    return in.color;
}

fragment float4 fs_stencil(VertexOut in [[stage_in]]) {
    float radius = floor(in.params / 4096.0) / 4.0;

    float2 uv_to_px = 1.0 / (float2(fwidth(in.uv.x),
                       fwidth(in.uv.y)) + 1e-6);
    float2 half_size = uv_to_px;
    float2 pos       = in.uv * half_size;

    float2 q = abs(pos) - half_size + float2(radius);
    float d = length(max(q, float2(0.0)))
            + min(max(q.x, q.y), 0.0) - radius;

    float grad_len = length(float2(dfdx(d), dfdy(d)));
    d = d / max(grad_len, 0.001);
    float alpha = 1.0 - smoothstep(-0.59, 0.59, d);

    if (alpha < 0.5) discard_fragment();
    return float4(1.0);
}
`
