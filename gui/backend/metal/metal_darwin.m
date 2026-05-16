#import <Metal/Metal.h>
#import <QuartzCore/CAMetalLayer.h>
#include "metal_darwin.h"
#include <string.h>
#include <objc/runtime.h>

// ARC-compatible per-frame pool management.
extern void *objc_autoreleasePoolPush(void);
extern void  objc_autoreleasePoolPop(void *token);

// ─── Constants ────────────────────────────────────────────────

#define MAX_TEX 8192
#define TRI_BUF_RING 3
#define TRI_BUF_MAX_PER_FRAME 256
#define MAX_CUSTOM_PIPELINES 32

// ─── MetalContext ─────────────────────────────────────────────

struct MetalContext {
    id<MTLDevice>       device;
    id<MTLCommandQueue> queue;
    CAMetalLayer        *layer;
    id<MTLSamplerState> sampler;
    id<MTLBuffer>       quadIdx;
    id<MTLRenderPipelineState> pipelines[PIPE_COUNT];
    // Per-frame
    id<CAMetalDrawable>          drawable;
    id<MTLCommandBuffer>         cmdBuf;
    id<MTLRenderCommandEncoder>  enc;
    // Viewport
    int viewW, viewH;
    // Textures
    id<MTLTexture> textures[MAX_TEX];
    int nextTexID;
    int freeTexIDs[MAX_TEX];
    int freeTexCount;
    // Triangle buffers
    id<MTLBuffer> triBufs[TRI_BUF_RING][TRI_BUF_MAX_PER_FRAME];
    int triBufCursor[TRI_BUF_RING];
    int triBufFrame;
    // Filter
    id<MTLTexture> filterTexA;
    id<MTLTexture> filterTexB;
    id<MTLTexture> filterStencilTex;
    int filterW, filterH;
    // Stencil
    id<MTLTexture> stencilTex;
    int stencilTexW, stencilTexH;
    id<MTLDepthStencilState> stencilIncr;
    id<MTLDepthStencilState> stencilTest;
    id<MTLDepthStencilState> stencilDecr;
    id<MTLDepthStencilState> stencilOff;
    // Custom pipelines
    id<MTLRenderPipelineState> customPipelines[MAX_CUSTOM_PIPELINES];
    int freeCustomPipelineIDs[MAX_CUSTOM_PIPELINES];
    int freeCustomPipelineCount;
    int nextCustomPipelineID;
    // Per-frame autorelease pool token — drains command buffer,
    // descriptors, encoders, and one-off buffers each frame.
    // Managed via objc_autoreleasePoolPush/Pop (ARC-compatible).
    void *framePool;
};

// Local typedef — the header uses void* for cgo compat.
typedef struct MetalContext MetalContext;

// Cast helper — MetalCtx is void* in the header for cgo.
#define MC(p) ((MetalContext*)(p))

// ─── MSL Shader Source ──────────────────────────────────────

static NSString *mslSource = @R"(
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

    float2 q = abs(pos) - half_size
             + float2(radius + 1.5 * blur);
    float d = length(max(q, float2(0.0)))
            + min(max(q.x, q.y), 0.0) - radius;

    float2 q_c = abs(pos + in.offset) - half_size
               + float2(radius + 1.5 * blur);
    float d_c = length(max(q_c, float2(0.0)))
              + min(max(q_c.x, q_c.y), 0.0) - radius;

    float alpha_falloff = 1.0 - smoothstep(0.0,
                          max(1.0, blur), d);
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

    // Helper lambda-like macros via inline.
    auto unpack = [](float val1, float val2,
                     thread float4 &c, thread float &p) {
        float r = fmod(val1, 256.0);
        float g = fmod(floor(val1 / 256.0), 256.0);
        float b = floor(val1 / 65536.0);
        float a = fmod(val2, 256.0);
        p = floor(val2 / 256.0) / 10000.0;
        c = float4(r/255.0, g/255.0, b/255.0, a/255.0);
    };

    unpack(in.stop12.x, in.stop12.y,
           stop_colors[0], stop_positions[0]);
    unpack(in.stop12.z, in.stop12.w,
           stop_colors[1], stop_positions[1]);
    unpack(in.stop34.x, in.stop34.y,
           stop_colors[2], stop_positions[2]);
    unpack(in.stop34.z, in.stop34.w,
           stop_colors[3], stop_positions[3]);
    unpack(in.stop56.x, in.stop56.y,
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
)";

// ─── Helpers ──────────────────────────────────────────────────

static MTLVertexDescriptor *mainVertexDesc(void) {
    MTLVertexDescriptor *d = [[MTLVertexDescriptor alloc] init];
    // float3 position
    d.attributes[0].format      = MTLVertexFormatFloat3;
    d.attributes[0].offset      = 0;
    d.attributes[0].bufferIndex = 0;
    // float2 texcoord
    d.attributes[1].format      = MTLVertexFormatFloat2;
    d.attributes[1].offset      = 12;
    d.attributes[1].bufferIndex = 0;
    // float4 color
    d.attributes[2].format      = MTLVertexFormatFloat4;
    d.attributes[2].offset      = 20;
    d.attributes[2].bufferIndex = 0;
    d.layouts[0].stride         = 36;
    d.layouts[0].stepFunction   = MTLVertexStepFunctionPerVertex;
    return d;
}

static MTLVertexDescriptor *glyphVertexDesc(void) {
    MTLVertexDescriptor *d = [[MTLVertexDescriptor alloc] init];
    // float2 position
    d.attributes[0].format      = MTLVertexFormatFloat2;
    d.attributes[0].offset      = 0;
    d.attributes[0].bufferIndex = 0;
    // float2 texcoord
    d.attributes[1].format      = MTLVertexFormatFloat2;
    d.attributes[1].offset      = 8;
    d.attributes[1].bufferIndex = 0;
    // float4 color
    d.attributes[2].format      = MTLVertexFormatFloat4;
    d.attributes[2].offset      = 16;
    d.attributes[2].bufferIndex = 0;
    d.layouts[0].stride         = 32;
    d.layouts[0].stepFunction   = MTLVertexStepFunctionPerVertex;
    return d;
}

static id<MTLRenderPipelineState> makePipeline(
    id<MTLDevice> device,
    id<MTLLibrary> lib,
    NSString *vsName,
    NSString *fsName,
    MTLVertexDescriptor *vd,
    MTLPixelFormat pixFmt
) {
    MTLRenderPipelineDescriptor *desc =
        [[MTLRenderPipelineDescriptor alloc] init];
    desc.vertexFunction   = [lib newFunctionWithName:vsName];
    desc.fragmentFunction = [lib newFunctionWithName:fsName];
    desc.vertexDescriptor = vd;

    desc.colorAttachments[0].pixelFormat = pixFmt;
    desc.colorAttachments[0].blendingEnabled = YES;
    desc.colorAttachments[0].sourceRGBBlendFactor =
        MTLBlendFactorSourceAlpha;
    desc.colorAttachments[0].destinationRGBBlendFactor =
        MTLBlendFactorOneMinusSourceAlpha;
    desc.colorAttachments[0].sourceAlphaBlendFactor =
        MTLBlendFactorSourceAlpha;
    desc.colorAttachments[0].destinationAlphaBlendFactor =
        MTLBlendFactorOneMinusSourceAlpha;

    desc.stencilAttachmentPixelFormat = MTLPixelFormatStencil8;

    if (!desc.vertexFunction) {
        NSLog(@"metal: vertex function %@ not found", vsName);
        return nil;
    }
    if (!desc.fragmentFunction) {
        NSLog(@"metal: fragment function %@ not found", fsName);
        return nil;
    }

    NSError *err = nil;
    id<MTLRenderPipelineState> pso =
        [device newRenderPipelineStateWithDescriptor:desc
                                                error:&err];
    if (!pso) {
        NSLog(@"metal: pipeline %@/%@: %@", vsName, fsName, err);
    }
    return pso;
}

// makePipelineReplace creates a pipeline with no blending
// (write-through). Used for full-screen filter passes that render
// onto cleared targets where srcAlpha blending would corrupt the
// output by double-applying alpha.
static id<MTLRenderPipelineState> makePipelineReplace(
    id<MTLDevice> device,
    id<MTLLibrary> lib,
    NSString *vsName,
    NSString *fsName,
    MTLVertexDescriptor *vd,
    MTLPixelFormat pixFmt
) {
    MTLRenderPipelineDescriptor *desc =
        [[MTLRenderPipelineDescriptor alloc] init];
    desc.vertexFunction   = [lib newFunctionWithName:vsName];
    desc.fragmentFunction = [lib newFunctionWithName:fsName];
    desc.vertexDescriptor = vd;

    desc.colorAttachments[0].pixelFormat = pixFmt;
    desc.colorAttachments[0].blendingEnabled = NO;

    desc.stencilAttachmentPixelFormat = MTLPixelFormatStencil8;

    if (!desc.vertexFunction) {
        NSLog(@"metal: vertex function %@ not found", vsName);
        return nil;
    }
    if (!desc.fragmentFunction) {
        NSLog(@"metal: fragment function %@ not found", fsName);
        return nil;
    }

    NSError *err = nil;
    id<MTLRenderPipelineState> pso =
        [device newRenderPipelineStateWithDescriptor:desc
                                                error:&err];
    if (!pso) {
        NSLog(@"metal: pipeline %@/%@: %@", vsName, fsName, err);
    }
    return pso;
}

// makePipelineStencilMask creates a pipeline with color writes
// disabled (colorWriteMask = None), used for stencil mask passes.
static id<MTLRenderPipelineState> makePipelineStencilMask(
    id<MTLDevice> device,
    id<MTLLibrary> lib,
    NSString *vsName,
    NSString *fsName,
    MTLVertexDescriptor *vd,
    MTLPixelFormat pixFmt
) {
    MTLRenderPipelineDescriptor *desc =
        [[MTLRenderPipelineDescriptor alloc] init];
    desc.vertexFunction   = [lib newFunctionWithName:vsName];
    desc.fragmentFunction = [lib newFunctionWithName:fsName];
    desc.vertexDescriptor = vd;

    desc.colorAttachments[0].pixelFormat = pixFmt;
    desc.colorAttachments[0].writeMask   = MTLColorWriteMaskNone;
    desc.colorAttachments[0].blendingEnabled = NO;

    desc.stencilAttachmentPixelFormat = MTLPixelFormatStencil8;

    if (!desc.vertexFunction || !desc.fragmentFunction) {
        NSLog(@"metal: stencil pipeline: function not found");
        return nil;
    }

    NSError *err = nil;
    id<MTLRenderPipelineState> pso =
        [device newRenderPipelineStateWithDescriptor:desc
                                                error:&err];
    if (!pso) {
        NSLog(@"metal: stencil pipeline: %@", err);
    }
    return pso;
}

static id<MTLTexture> makeTexture(id<MTLDevice> device,
    int w, int h, MTLPixelFormat fmt) {
    MTLTextureDescriptor *td =
        [MTLTextureDescriptor texture2DDescriptorWithPixelFormat:fmt
                              width:w height:h mipmapped:NO];
    td.usage = MTLTextureUsageShaderRead;
    td.storageMode = MTLStorageModeShared;
    return [device newTextureWithDescriptor:td];
}

static id<MTLTexture> makeRenderTarget(id<MTLDevice> device,
    int w, int h, MTLPixelFormat fmt) {
    MTLTextureDescriptor *td =
        [MTLTextureDescriptor texture2DDescriptorWithPixelFormat:fmt
                              width:w height:h mipmapped:NO];
    td.usage = MTLTextureUsageShaderRead |
               MTLTextureUsageRenderTarget;
    td.storageMode = MTLStorageModePrivate;
    return [device newTextureWithDescriptor:td];
}

static void ensureStencilTexture(MetalContext* ctx, int w,
    int h) {
    if (ctx->stencilTex && ctx->stencilTexW == w &&
        ctx->stencilTexH == h)
        return;
    MTLTextureDescriptor *td = [MTLTextureDescriptor
        texture2DDescriptorWithPixelFormat:MTLPixelFormatStencil8
                                     width:w height:h
                                  mipmapped:NO];
    td.usage = MTLTextureUsageRenderTarget;
    td.storageMode = MTLStorageModePrivate;
    ctx->stencilTex = [ctx->device newTextureWithDescriptor:td];
    ctx->stencilTexW = w;
    ctx->stencilTexH = h;
}

// Start or resume the main render pass.
static void beginMainEncoder(MetalContext* ctx, float r, float g,
    float b, float a, int clear) {
    MTLRenderPassDescriptor *rpd =
        [MTLRenderPassDescriptor renderPassDescriptor];
    rpd.colorAttachments[0].texture = ctx->drawable.texture;
    rpd.colorAttachments[0].storeAction = MTLStoreActionStore;
    if (clear) {
        rpd.colorAttachments[0].loadAction = MTLLoadActionClear;
        rpd.colorAttachments[0].clearColor =
            MTLClearColorMake(r, g, b, a);
    } else {
        rpd.colorAttachments[0].loadAction = MTLLoadActionLoad;
    }

    // Attach stencil buffer.
    ensureStencilTexture(ctx, ctx->viewW, ctx->viewH);
    if (ctx->stencilTex) {
        rpd.stencilAttachment.texture = ctx->stencilTex;
        rpd.stencilAttachment.storeAction = MTLStoreActionStore;
        if (clear) {
            rpd.stencilAttachment.loadAction =
                MTLLoadActionClear;
            rpd.stencilAttachment.clearStencil = 0;
        } else {
            rpd.stencilAttachment.loadAction = MTLLoadActionLoad;
        }
    }

    ctx->enc = [ctx->cmdBuf
        renderCommandEncoderWithDescriptor:rpd];
    [ctx->enc setViewport:(MTLViewport){
        0, 0, (double)ctx->viewW, (double)ctx->viewH, 0, 1}];
    [ctx->enc setFragmentSamplerState:ctx->sampler atIndex:0];
}

// ─── Public API ───────────────────────────────────────────────

MetalCtx metalCtxCreate(void* layerPtr) {
    MetalContext* ctx = calloc(1, sizeof(MetalContext));
    if (!ctx) return NULL;

    ctx->nextTexID = 1;
    ctx->triBufFrame = -1;

    ctx->layer = (__bridge CAMetalLayer*)layerPtr;
    ctx->device = MTLCreateSystemDefaultDevice();
    if (!ctx->device) {
        NSLog(@"metal: no Metal device");
        free(ctx);
        return NULL;
    }

    ctx->layer.device = ctx->device;
    ctx->layer.pixelFormat = MTLPixelFormatBGRA8Unorm;
    ctx->layer.framebufferOnly = YES;
    // Synchronize presentation with the compositor resize
    // transaction. Eliminates content shift during live resize.
    ctx->layer.presentsWithTransaction = YES;

    ctx->queue = [ctx->device newCommandQueue];

    // Compile MSL library.
    NSError *err = nil;
    id<MTLLibrary> lib =
        [ctx->device newLibraryWithSource:mslSource
                              options:nil error:&err];
    if (!lib) {
        NSLog(@"metal: compile shaders: %@", err);
        free(ctx);
        return NULL;
    }

    MTLPixelFormat pf = MTLPixelFormatBGRA8Unorm;
    MTLVertexDescriptor *mvd = mainVertexDesc();
    MTLVertexDescriptor *gvd = glyphVertexDesc();

    // Build pipeline states.
    ctx->pipelines[PIPE_SOLID] =
        makePipeline(ctx->device, lib, @"vs_solid", @"fs_solid",
                     mvd, pf);
    ctx->pipelines[PIPE_SHADOW] =
        makePipeline(ctx->device, lib, @"vs_shadow", @"fs_shadow",
                     mvd, pf);
    ctx->pipelines[PIPE_BLUR] =
        makePipeline(ctx->device, lib, @"vs_blur", @"fs_blur",
                     mvd, pf);
    ctx->pipelines[PIPE_GRADIENT] =
        makePipeline(ctx->device, lib, @"vs_gradient",
                     @"fs_gradient", mvd, pf);
    ctx->pipelines[PIPE_IMAGE_CLIP] =
        makePipeline(ctx->device, lib, @"vs_solid",
                     @"fs_image_clip", mvd, pf);
    ctx->pipelines[PIPE_FILTER_BLUR_H] =
        makePipelineReplace(ctx->device, lib, @"vs_filter",
                     @"fs_filter_blur_h", mvd, pf);
    ctx->pipelines[PIPE_FILTER_BLUR_V] =
        makePipelineReplace(ctx->device, lib, @"vs_filter",
                     @"fs_filter_blur_v", mvd, pf);
    ctx->pipelines[PIPE_FILTER_TEX] =
        makePipeline(ctx->device, lib, @"vs_filter",
                     @"fs_filter_tex", mvd, pf);
    ctx->pipelines[PIPE_FILTER_COLOR] =
        makePipelineReplace(ctx->device, lib, @"vs_filter",
                     @"fs_filter_color", mvd, pf);
    ctx->pipelines[PIPE_GLYPH_TEX] =
        makePipeline(ctx->device, lib, @"vs_glyph",
                     @"fs_glyph_tex", gvd, pf);
    ctx->pipelines[PIPE_GLYPH_COLOR] =
        makePipeline(ctx->device, lib, @"vs_glyph",
                     @"fs_glyph_color", gvd, pf);
    ctx->pipelines[PIPE_STENCIL] =
        makePipelineStencilMask(ctx->device, lib, @"vs_solid",
                                @"fs_stencil", mvd, pf);

    for (int i = 0; i < PIPE_COUNT; i++) {
        if (!ctx->pipelines[i]) {
            free(ctx);
            return NULL;
        }
    }

    // Build depth stencil states for stencil clipping.
    {
        MTLDepthStencilDescriptor *dsd;

        // Increment stencil where fragment passes.
        dsd = [[MTLDepthStencilDescriptor alloc] init];
        dsd.frontFaceStencil.stencilCompareFunction =
            MTLCompareFunctionAlways;
        dsd.frontFaceStencil.stencilFailureOperation =
            MTLStencilOperationKeep;
        dsd.frontFaceStencil.depthFailureOperation =
            MTLStencilOperationKeep;
        dsd.frontFaceStencil.depthStencilPassOperation =
            MTLStencilOperationIncrementClamp;
        dsd.backFaceStencil = dsd.frontFaceStencil;
        ctx->stencilIncr =
            [ctx->device
                newDepthStencilStateWithDescriptor:dsd];

        // Test stencil (children pass where >= ref).
        dsd = [[MTLDepthStencilDescriptor alloc] init];
        dsd.frontFaceStencil.stencilCompareFunction =
            MTLCompareFunctionLessEqual;
        dsd.frontFaceStencil.stencilFailureOperation =
            MTLStencilOperationKeep;
        dsd.frontFaceStencil.depthFailureOperation =
            MTLStencilOperationKeep;
        dsd.frontFaceStencil.depthStencilPassOperation =
            MTLStencilOperationKeep;
        dsd.backFaceStencil = dsd.frontFaceStencil;
        ctx->stencilTest =
            [ctx->device
                newDepthStencilStateWithDescriptor:dsd];

        // Decrement stencil where fragment passes.
        dsd = [[MTLDepthStencilDescriptor alloc] init];
        dsd.frontFaceStencil.stencilCompareFunction =
            MTLCompareFunctionAlways;
        dsd.frontFaceStencil.stencilFailureOperation =
            MTLStencilOperationKeep;
        dsd.frontFaceStencil.depthFailureOperation =
            MTLStencilOperationKeep;
        dsd.frontFaceStencil.depthStencilPassOperation =
            MTLStencilOperationDecrementClamp;
        dsd.backFaceStencil = dsd.frontFaceStencil;
        ctx->stencilDecr =
            [ctx->device
                newDepthStencilStateWithDescriptor:dsd];

        // Disable stencil test.
        dsd = [[MTLDepthStencilDescriptor alloc] init];
        ctx->stencilOff =
            [ctx->device
                newDepthStencilStateWithDescriptor:dsd];
    }

    // Quad index buffer: two triangles [0,1,2, 0,2,3].
    uint16_t idx[6] = {0, 1, 2, 0, 2, 3};
    ctx->quadIdx = [ctx->device newBufferWithBytes:idx
                                    length:sizeof(idx)
                                   options:MTLResourceStorageModeShared];

    // Shared sampler (linear + clamp-to-edge).
    MTLSamplerDescriptor *sd =
        [[MTLSamplerDescriptor alloc] init];
    sd.minFilter    = MTLSamplerMinMagFilterLinear;
    sd.magFilter    = MTLSamplerMinMagFilterLinear;
    sd.sAddressMode = MTLSamplerAddressModeClampToEdge;
    sd.tAddressMode = MTLSamplerAddressModeClampToEdge;
    ctx->sampler =
        [ctx->device newSamplerStateWithDescriptor:sd];

    return ctx;
}

// ─── Custom Shader Pipelines ─────────────────────────────────

int metalBuildCustomPipeline(MetalCtx ctx_,
                             const char* mslSrc) {
    MetalContext* ctx = MC(ctx_);
    NSString *src = [NSString stringWithUTF8String:mslSrc];
    NSError *err = nil;
    id<MTLLibrary> lib =
        [ctx->device newLibraryWithSource:src
                              options:nil error:&err];
    if (!lib) {
        NSLog(@"metal: custom shader compile: %@", err);
        return -1;
    }

    MTLRenderPipelineDescriptor *desc =
        [[MTLRenderPipelineDescriptor alloc] init];
    desc.vertexFunction =
        [lib newFunctionWithName:@"vs_main"];
    desc.fragmentFunction =
        [lib newFunctionWithName:@"fs_main"];
    desc.vertexDescriptor = mainVertexDesc();
    desc.colorAttachments[0].pixelFormat =
        MTLPixelFormatBGRA8Unorm;
    desc.colorAttachments[0].blendingEnabled = YES;
    desc.colorAttachments[0].sourceRGBBlendFactor =
        MTLBlendFactorSourceAlpha;
    desc.colorAttachments[0].destinationRGBBlendFactor =
        MTLBlendFactorOneMinusSourceAlpha;
    desc.colorAttachments[0].sourceAlphaBlendFactor =
        MTLBlendFactorSourceAlpha;
    desc.colorAttachments[0].destinationAlphaBlendFactor =
        MTLBlendFactorOneMinusSourceAlpha;

    if (!desc.vertexFunction || !desc.fragmentFunction) {
        NSLog(@"metal: custom shader: function not found");
        return -1;
    }

    id<MTLRenderPipelineState> pso =
        [ctx->device newRenderPipelineStateWithDescriptor:desc
                                                error:&err];
    if (!pso) {
        NSLog(@"metal: custom pipeline: %@", err);
        return -1;
    }

    int idx = 0;
    if (ctx->freeCustomPipelineCount > 0) {
        idx = ctx->freeCustomPipelineIDs[
            --ctx->freeCustomPipelineCount];
    } else {
        if (ctx->nextCustomPipelineID >= MAX_CUSTOM_PIPELINES) {
            NSLog(@"metal: custom pipeline cache exhausted");
            return -1;
        }
        idx = ctx->nextCustomPipelineID++;
    }
    ctx->customPipelines[idx] = pso;
    return idx;
}

void metalDeleteCustomPipeline(MetalCtx ctx_, int idx) {
    MetalContext* ctx = MC(ctx_);
    if (idx < 0 || idx >= MAX_CUSTOM_PIPELINES) {
        return;
    }
    if (!ctx->customPipelines[idx]) {
        return;
    }
    ctx->customPipelines[idx] = nil;
    if (ctx->freeCustomPipelineCount < MAX_CUSTOM_PIPELINES) {
        ctx->freeCustomPipelineIDs[
            ctx->freeCustomPipelineCount++] = idx;
    }
}

void metalSetCustomPipeline(MetalCtx ctx_, int idx) {
    MetalContext* ctx = MC(ctx_);
    if (!ctx->enc || idx < 0 || idx >= MAX_CUSTOM_PIPELINES ||
        !ctx->customPipelines[idx])
        return;
    [ctx->enc setRenderPipelineState:ctx->customPipelines[idx]];
}

void metalCtxDestroy(MetalCtx ctx_) {
    MetalContext* ctx = MC(ctx_);
    if (!ctx) return;
    if (ctx->framePool) {
        objc_autoreleasePoolPop(ctx->framePool);
        ctx->framePool = nil;
    }
    for (int i = 0; i < MAX_TEX; i++) {
        ctx->textures[i] = nil;
    }
    ctx->nextTexID = 1;
    ctx->freeTexCount = 0;
    for (int f = 0; f < TRI_BUF_RING; f++) {
        ctx->triBufCursor[f] = 0;
        for (int i = 0; i < TRI_BUF_MAX_PER_FRAME; i++) {
            ctx->triBufs[f][i] = nil;
        }
    }
    ctx->triBufFrame = -1;
    ctx->filterTexA = nil;
    ctx->filterTexB = nil;
    ctx->filterStencilTex = nil;
    ctx->stencilTex = nil;
    ctx->stencilTexW = 0;
    ctx->stencilTexH = 0;
    ctx->stencilIncr = nil;
    ctx->stencilTest = nil;
    ctx->stencilDecr = nil;
    ctx->stencilOff  = nil;
    for (int i = 0; i < PIPE_COUNT; i++) {
        ctx->pipelines[i] = nil;
    }
    for (int i = 0; i < MAX_CUSTOM_PIPELINES; i++) {
        ctx->customPipelines[i] = nil;
    }
    ctx->nextCustomPipelineID = 0;
    ctx->freeCustomPipelineCount = 0;
    ctx->quadIdx = nil;
    ctx->sampler = nil;
    ctx->queue   = nil;
    ctx->device  = nil;
    ctx->layer   = nil;
    free(ctx);
}

void metalResize(MetalCtx ctx_, int w, int h) {
    MetalContext* ctx = MC(ctx_);
    ctx->viewW = w;
    ctx->viewH = h;
    ctx->layer.drawableSize = CGSizeMake(w, h);
}

int metalBeginFrame(MetalCtx ctx_,
                    float r, float g, float b, float a) {
    MetalContext* ctx = MC(ctx_);
    ctx->framePool = objc_autoreleasePoolPush();

    ctx->drawable = [ctx->layer nextDrawable];
    if (!ctx->drawable) {
        objc_autoreleasePoolPop(ctx->framePool);
        ctx->framePool = nil;
        return -1;
    }

    ctx->triBufFrame =
        (ctx->triBufFrame + 1) % TRI_BUF_RING;
    ctx->triBufCursor[ctx->triBufFrame] = 0;

    ctx->cmdBuf = [ctx->queue commandBuffer];
    beginMainEncoder(ctx, r, g, b, a, 1);
    return 0;
}

void metalEndFrame(MetalCtx ctx_) {
    MetalContext* ctx = MC(ctx_);
    if (ctx->enc) {
        [ctx->enc endEncoding];
        ctx->enc = nil;
    }
    if (ctx->drawable && ctx->cmdBuf) {
        [ctx->cmdBuf commit];
        [ctx->cmdBuf waitUntilScheduled];
        [ctx->drawable present];
    }
    ctx->drawable = nil;
    ctx->cmdBuf   = nil;
    if (ctx->framePool) {
        objc_autoreleasePoolPop(ctx->framePool);
        ctx->framePool = nil;
    }
}

void metalSetPipeline(MetalCtx ctx_, int id) {
    MetalContext* ctx = MC(ctx_);
    if (id < 0 || id >= PIPE_COUNT || !ctx->enc) return;
    [ctx->enc setRenderPipelineState:ctx->pipelines[id]];
}

void metalSetMVP(MetalCtx ctx_, const float* m) {
    MetalContext* ctx = MC(ctx_);
    if (!ctx->enc) return;
    [ctx->enc setVertexBytes:m length:64 atIndex:1];
}

void metalSetTM(MetalCtx ctx_, const float* m) {
    MetalContext* ctx = MC(ctx_);
    if (!ctx->enc) return;
    [ctx->enc setVertexBytes:m length:64 atIndex:2];
}

void metalSetScissor(MetalCtx ctx_,
                     int x, int y, int w, int h, int viewH) {
    MetalContext* ctx = MC(ctx_);
    if (!ctx->enc) return;
    // Clamp to viewport.
    if (x < 0) { w += x; x = 0; }
    if (y < 0) { h += y; y = 0; }
    if (w <= 0 || h <= 0) {
        // Zero-area scissor: clip everything.
        [ctx->enc setScissorRect:(MTLScissorRect){0, 0, 1, 1}];
        return;
    }
    if (x + w > ctx->viewW) w = ctx->viewW - x;
    if (y + h > ctx->viewH) h = ctx->viewH - y;
    if (w <= 0 || h <= 0) {
        [ctx->enc setScissorRect:(MTLScissorRect){0, 0, 1, 1}];
        return;
    }
    [ctx->enc setScissorRect:(MTLScissorRect){
        (NSUInteger)x, (NSUInteger)y,
        (NSUInteger)w, (NSUInteger)h}];
}

void metalDisableScissor(MetalCtx ctx_) {
    MetalContext* ctx = MC(ctx_);
    if (!ctx->enc) return;
    [ctx->enc setScissorRect:(MTLScissorRect){
        0, 0, (NSUInteger)ctx->viewW,
        (NSUInteger)ctx->viewH}];
}

void metalDrawQuad(MetalCtx ctx_, const float* verts) {
    MetalContext* ctx = MC(ctx_);
    if (!ctx->enc) return;
    [ctx->enc setVertexBytes:verts length:4*36 atIndex:0];
    [ctx->enc drawIndexedPrimitives:MTLPrimitiveTypeTriangle
                     indexCount:6
                      indexType:MTLIndexTypeUInt16
                    indexBuffer:ctx->quadIdx
              indexBufferOffset:0];
}

void metalDrawTriangles(MetalCtx ctx_,
                        const float* verts, int numVerts) {
    MetalContext* ctx = MC(ctx_);
    if (!ctx->enc || numVerts <= 0) return;
    int byteLen = numVerts * 36;
    if (byteLen <= 4096) {
        [ctx->enc setVertexBytes:verts length:byteLen atIndex:0];
    } else {
        id<MTLBuffer> buf = nil;
        if (ctx->triBufFrame >= 0 &&
            ctx->triBufFrame < TRI_BUF_RING) {
            int slot = ctx->triBufCursor[ctx->triBufFrame]++;
            if (slot < TRI_BUF_MAX_PER_FRAME) {
                buf = ctx->triBufs[ctx->triBufFrame][slot];
                if (!buf ||
                    [buf length] < (NSUInteger)byteLen) {
                    NSUInteger cap = (NSUInteger)byteLen;
                    NSUInteger page = 4096;
                    cap = ((cap + page - 1) / page) * page;
                    buf = [ctx->device
                        newBufferWithLength:cap
                                    options:MTLResourceStorageModeShared];
                    ctx->triBufs[ctx->triBufFrame][slot] = buf;
                }
                memcpy([buf contents], verts, (size_t)byteLen);
            }
        }
        if (!buf) {
            // Pool exhausted — allocate a one-off buffer.
            buf = [ctx->device
                newBufferWithBytes:verts
                            length:(NSUInteger)byteLen
                           options:MTLResourceStorageModeShared];
        }
        if (!buf) return;
        [ctx->enc setVertexBuffer:buf offset:0 atIndex:0];
    }
    [ctx->enc drawPrimitives:MTLPrimitiveTypeTriangle
             vertexStart:0 vertexCount:numVerts];
}

void metalDrawGlyphQuad(MetalCtx ctx_, const float* verts) {
    MetalContext* ctx = MC(ctx_);
    if (!ctx->enc) return;
    [ctx->enc setVertexBytes:verts length:4*32 atIndex:0];
    [ctx->enc drawIndexedPrimitives:MTLPrimitiveTypeTriangle
                     indexCount:6
                      indexType:MTLIndexTypeUInt16
                    indexBuffer:ctx->quadIdx
              indexBufferOffset:0];
}

// ─── Textures ─────────────────────────────────────────────────

int metalCreateTexture(MetalCtx ctx_,
                       int w, int h, const void* pixels,
                       int hasData) {
    MetalContext* ctx = MC(ctx_);
    int tid = 0;
    if (ctx->freeTexCount > 0) {
        tid = ctx->freeTexIDs[--ctx->freeTexCount];
    } else {
        if (ctx->nextTexID >= MAX_TEX) return 0;
        tid = ctx->nextTexID++;
    }

    id<MTLTexture> tex = makeTexture(ctx->device, w, h,
        MTLPixelFormatRGBA8Unorm);
    if (!tex) {
        if (ctx->freeTexCount < MAX_TEX) {
            ctx->freeTexIDs[ctx->freeTexCount++] = tid;
        }
        return 0;
    }
    if (hasData && pixels) {
        [tex replaceRegion:MTLRegionMake2D(0, 0, w, h)
               mipmapLevel:0
                 withBytes:pixels
               bytesPerRow:w * 4];
    }
    ctx->textures[tid] = tex;
    return tid;
}

void metalUpdateTexture(MetalCtx ctx_,
                        int id, int x, int y, int w, int h,
                        const void* data) {
    MetalContext* ctx = MC(ctx_);
    if (id <= 0 || id >= MAX_TEX || !ctx->textures[id]) return;
    [ctx->textures[id]
        replaceRegion:MTLRegionMake2D(x, y, w, h)
          mipmapLevel:0
            withBytes:data
          bytesPerRow:w * 4];
}

void metalDeleteTexture(MetalCtx ctx_, int id) {
    MetalContext* ctx = MC(ctx_);
    if (id <= 0 || id >= MAX_TEX) return;
    if (!ctx->textures[id]) return;
    ctx->textures[id] = nil;
    if (ctx->freeTexCount < MAX_TEX) {
        ctx->freeTexIDs[ctx->freeTexCount++] = id;
    }
}

void metalBindTexture(MetalCtx ctx_, int id) {
    MetalContext* ctx = MC(ctx_);
    if (!ctx->enc) return;
    if (id > 0 && id < MAX_TEX && ctx->textures[id]) {
        [ctx->enc setFragmentTexture:ctx->textures[id]
                             atIndex:0];
    }
}

// ─── Filter System ────────────────────────────────────────────

static void ensureFilterTextures(MetalContext* ctx,
    int w, int h) {
    if (ctx->filterTexA && ctx->filterW == w &&
        ctx->filterH == h) return;
    MTLPixelFormat pf = MTLPixelFormatBGRA8Unorm;
    ctx->filterTexA = makeRenderTarget(ctx->device, w, h, pf);
    ctx->filterTexB = makeRenderTarget(ctx->device, w, h, pf);
    // Stencil attachment so ClipContents works inside filters.
    MTLTextureDescriptor *std = [MTLTextureDescriptor
        texture2DDescriptorWithPixelFormat:MTLPixelFormatStencil8
                                     width:w height:h
                                  mipmapped:NO];
    std.usage = MTLTextureUsageRenderTarget;
    std.storageMode = MTLStorageModePrivate;
    ctx->filterStencilTex =
        [ctx->device newTextureWithDescriptor:std];
    ctx->filterW = w;
    ctx->filterH = h;
}

int metalBeginFilter(MetalCtx ctx_, int w, int h) {
    MetalContext* ctx = MC(ctx_);
    if (!ctx->enc || !ctx->cmdBuf) return -1;

    ensureFilterTextures(ctx, w, h);
    if (!ctx->filterTexA || !ctx->filterTexB) return -2;

    // End current main encoder.
    [ctx->enc endEncoding];
    ctx->enc = nil;

    // Start render pass targeting filterTexA.
    MTLRenderPassDescriptor *rpd =
        [MTLRenderPassDescriptor renderPassDescriptor];
    rpd.colorAttachments[0].texture     = ctx->filterTexA;
    rpd.colorAttachments[0].loadAction  = MTLLoadActionClear;
    rpd.colorAttachments[0].storeAction = MTLStoreActionStore;
    rpd.colorAttachments[0].clearColor  =
        MTLClearColorMake(0, 0, 0, 0);

    // Attach stencil so ClipContents works inside filters.
    if (ctx->filterStencilTex) {
        rpd.stencilAttachment.texture     =
            ctx->filterStencilTex;
        rpd.stencilAttachment.loadAction  = MTLLoadActionClear;
        rpd.stencilAttachment.storeAction = MTLStoreActionStore;
        rpd.stencilAttachment.clearStencil = 0;
    }

    ctx->enc = [ctx->cmdBuf
        renderCommandEncoderWithDescriptor:rpd];
    [ctx->enc setViewport:(MTLViewport){
        0, 0, (double)w, (double)h, 0, 1}];
    [ctx->enc setFragmentSamplerState:ctx->sampler atIndex:0];
    return 0;
}

void metalEndFilter(MetalCtx ctx_, float blurRadius,
                    int layers, const float* colorMatrix) {
    MetalContext* ctx = MC(ctx_);
    if (!ctx->enc || !ctx->cmdBuf) return;

    if (layers < 1) layers = 1;

    int w = ctx->filterW;
    int h = ctx->filterH;

    // End filter content encoder.
    [ctx->enc endEncoding];
    ctx->enc = nil;

    // compositeSrc tracks which texture holds the final result.
    id<MTLTexture> compositeSrc = ctx->filterTexA;

    // ── Blur passes (skip when blurRadius < 1) ──
    if (blurRadius >= 1) {
        float stdDev = blurRadius;

        // Horizontal blur: filterTexA → filterTexB
        {
            MTLRenderPassDescriptor *rpd =
                [MTLRenderPassDescriptor renderPassDescriptor];
            rpd.colorAttachments[0].texture     = ctx->filterTexB;
            rpd.colorAttachments[0].loadAction  = MTLLoadActionClear;
            rpd.colorAttachments[0].storeAction = MTLStoreActionStore;
            rpd.colorAttachments[0].clearColor  =
                MTLClearColorMake(0, 0, 0, 0);

            id<MTLRenderCommandEncoder> enc =
                [ctx->cmdBuf
                    renderCommandEncoderWithDescriptor:rpd];
            [enc setViewport:(MTLViewport){
                0, 0, (double)w, (double)h, 0, 1}];
            [enc setRenderPipelineState:
                ctx->pipelines[PIPE_FILTER_BLUR_H]];
            [enc setFragmentSamplerState:ctx->sampler atIndex:0];
            [enc setFragmentTexture:ctx->filterTexA atIndex:0];

            float tm[16] = {0};
            tm[0] = stdDev;
            [enc setVertexBytes:tm length:64 atIndex:2];

            float mvp[16] = {0};
            mvp[0]  =  2.0f / w;
            mvp[5]  = -2.0f / h;
            mvp[10] = -1.0f;
            mvp[12] = -1.0f;
            mvp[13] =  1.0f;
            mvp[15] =  1.0f;
            [enc setVertexBytes:mvp length:64 atIndex:1];

            float verts[] = {
                0,0,0, 0,1, 1,1,1,1,
                (float)w,0,0, 1,1, 1,1,1,1,
                (float)w,(float)h,0, 1,0, 1,1,1,1,
                0,(float)h,0, 0,0, 1,1,1,1,
            };
            [enc setVertexBytes:verts length:sizeof(verts)
                        atIndex:0];
            [enc drawIndexedPrimitives:MTLPrimitiveTypeTriangle
                            indexCount:6
                             indexType:MTLIndexTypeUInt16
                           indexBuffer:ctx->quadIdx
                     indexBufferOffset:0];
            [enc endEncoding];
        }

        // Vertical blur: filterTexB → filterTexA
        {
            MTLRenderPassDescriptor *rpd =
                [MTLRenderPassDescriptor renderPassDescriptor];
            rpd.colorAttachments[0].texture     = ctx->filterTexA;
            rpd.colorAttachments[0].loadAction  = MTLLoadActionClear;
            rpd.colorAttachments[0].storeAction = MTLStoreActionStore;
            rpd.colorAttachments[0].clearColor  =
                MTLClearColorMake(0, 0, 0, 0);

            id<MTLRenderCommandEncoder> enc =
                [ctx->cmdBuf
                    renderCommandEncoderWithDescriptor:rpd];
            [enc setViewport:(MTLViewport){
                0, 0, (double)w, (double)h, 0, 1}];
            [enc setRenderPipelineState:
                ctx->pipelines[PIPE_FILTER_BLUR_V]];
            [enc setFragmentSamplerState:ctx->sampler atIndex:0];
            [enc setFragmentTexture:ctx->filterTexB atIndex:0];

            float tm[16] = {0};
            tm[0] = stdDev;
            [enc setVertexBytes:tm length:64 atIndex:2];

            float mvp[16] = {0};
            mvp[0]  =  2.0f / w;
            mvp[5]  = -2.0f / h;
            mvp[10] = -1.0f;
            mvp[12] = -1.0f;
            mvp[13] =  1.0f;
            mvp[15] =  1.0f;
            [enc setVertexBytes:mvp length:64 atIndex:1];

            float verts[] = {
                0,0,0, 0,1, 1,1,1,1,
                (float)w,0,0, 1,1, 1,1,1,1,
                (float)w,(float)h,0, 1,0, 1,1,1,1,
                0,(float)h,0, 0,0, 1,1,1,1,
            };
            [enc setVertexBytes:verts length:sizeof(verts)
                        atIndex:0];
            [enc drawIndexedPrimitives:MTLPrimitiveTypeTriangle
                            indexCount:6
                             indexType:MTLIndexTypeUInt16
                           indexBuffer:ctx->quadIdx
                     indexBufferOffset:0];
            [enc endEncoding];
        }
        // After blur, result is in filterTexA.
    }

    // ── Color matrix pass: filterTexA → filterTexB ──
    // Uses non-flipped UVs so the composite always reads an
    // upright image regardless of whether blur ran first.
    if (colorMatrix != NULL) {
        MTLRenderPassDescriptor *rpd =
            [MTLRenderPassDescriptor renderPassDescriptor];
        rpd.colorAttachments[0].texture     = ctx->filterTexB;
        rpd.colorAttachments[0].loadAction  = MTLLoadActionClear;
        rpd.colorAttachments[0].storeAction = MTLStoreActionStore;
        rpd.colorAttachments[0].clearColor  =
            MTLClearColorMake(0, 0, 0, 0);

        id<MTLRenderCommandEncoder> enc =
            [ctx->cmdBuf
                renderCommandEncoderWithDescriptor:rpd];
        [enc setViewport:(MTLViewport){
            0, 0, (double)w, (double)h, 0, 1}];
        [enc setRenderPipelineState:
            ctx->pipelines[PIPE_FILTER_COLOR]];
        [enc setFragmentSamplerState:ctx->sampler atIndex:0];
        [enc setFragmentTexture:ctx->filterTexA atIndex:0];
        [enc setFragmentBytes:colorMatrix length:64 atIndex:0];

        float tm[16] = {0};
        tm[0] = 1; tm[5] = 1; tm[10] = 1; tm[15] = 1;
        [enc setVertexBytes:tm length:64 atIndex:2];

        float mvp[16] = {0};
        mvp[0]  =  2.0f / w;
        mvp[5]  = -2.0f / h;
        mvp[10] = -1.0f;
        mvp[12] = -1.0f;
        mvp[13] =  1.0f;
        mvp[15] =  1.0f;
        [enc setVertexBytes:mvp length:64 atIndex:1];

        // Non-flipped UVs: v=0 at top, v=1 at bottom.
        float verts[] = {
            0,0,0, 0,0, 1,1,1,1,
            (float)w,0,0, 1,0, 1,1,1,1,
            (float)w,(float)h,0, 1,1, 1,1,1,1,
            0,(float)h,0, 0,1, 1,1,1,1,
        };
        [enc setVertexBytes:verts length:sizeof(verts)
                    atIndex:0];
        [enc drawIndexedPrimitives:MTLPrimitiveTypeTriangle
                        indexCount:6
                         indexType:MTLIndexTypeUInt16
                       indexBuffer:ctx->quadIdx
                 indexBufferOffset:0];
        [enc endEncoding];

        compositeSrc = ctx->filterTexB;
    }

    // ── Resume main render pass (load, not clear) ──
    beginMainEncoder(ctx, 0, 0, 0, 0, 0);

    // ── Composite: draw result texture onto main drawable ──
    [ctx->enc setRenderPipelineState:
        ctx->pipelines[PIPE_FILTER_TEX]];
    [ctx->enc setFragmentTexture:compositeSrc atIndex:0];

    float mvp[16] = {0};
    mvp[0]  =  2.0f / ctx->viewW;
    mvp[5]  = -2.0f / ctx->viewH;
    mvp[10] = -1.0f;
    mvp[12] = -1.0f;
    mvp[13] =  1.0f;
    mvp[15] =  1.0f;
    [ctx->enc setVertexBytes:mvp length:64 atIndex:1];

    float tm[16] = {0};
    tm[0] = 1; tm[5] = 1; tm[10] = 1; tm[15] = 1;
    [ctx->enc setVertexBytes:tm length:64 atIndex:2];

    // H-blur and V-blur each flip V, cancelling out. The color
    // pass uses non-flipped UVs. Composite always uses non-flipped
    // UVs (v=0 at screen-top → texture-top).
    float verts[] = {
        0,0,0, 0,0, 1,1,1,1,
        (float)ctx->viewW,0,0, 1,0, 1,1,1,1,
        (float)ctx->viewW,(float)ctx->viewH,0, 1,1, 1,1,1,1,
        0,(float)ctx->viewH,0, 0,1, 1,1,1,1,
    };
    [ctx->enc setVertexBytes:verts length:sizeof(verts)
                     atIndex:0];
    for (int i = 0; i < layers; i++) {
        [ctx->enc drawIndexedPrimitives:MTLPrimitiveTypeTriangle
                         indexCount:6
                          indexType:MTLIndexTypeUInt16
                        indexBuffer:ctx->quadIdx
                  indexBufferOffset:0];
    }
}

// ─── Stencil Clip ─────────────────────────────────────────────

void metalBeginStencilClip(MetalCtx ctx_,
                           const float* verts, int depth) {
    MetalContext* ctx = MC(ctx_);
    if (!ctx->enc) return;

    // Increment stencil where SDF passes, no color output.
    [ctx->enc setDepthStencilState:ctx->stencilIncr];
    [ctx->enc setRenderPipelineState:
        ctx->pipelines[PIPE_STENCIL]];
    [ctx->enc setVertexBytes:verts length:4*36 atIndex:0];
    [ctx->enc drawIndexedPrimitives:MTLPrimitiveTypeTriangle
                     indexCount:6
                      indexType:MTLIndexTypeUInt16
                    indexBuffer:ctx->quadIdx
              indexBufferOffset:0];

    // Set stencil test for children.
    [ctx->enc setDepthStencilState:ctx->stencilTest];
    [ctx->enc setStencilReferenceValue:(uint32_t)depth];
}

void metalEndStencilClip(MetalCtx ctx_,
                         const float* verts, int depth) {
    MetalContext* ctx = MC(ctx_);
    if (!ctx->enc) return;

    // Decrement stencil where SDF passes, no color output.
    [ctx->enc setDepthStencilState:ctx->stencilDecr];
    [ctx->enc setRenderPipelineState:
        ctx->pipelines[PIPE_STENCIL]];
    [ctx->enc setVertexBytes:verts length:4*36 atIndex:0];
    [ctx->enc drawIndexedPrimitives:MTLPrimitiveTypeTriangle
                     indexCount:6
                      indexType:MTLIndexTypeUInt16
                    indexBuffer:ctx->quadIdx
              indexBufferOffset:0];

    if (depth <= 1) {
        [ctx->enc setDepthStencilState:ctx->stencilOff];
    } else {
        [ctx->enc setDepthStencilState:ctx->stencilTest];
        [ctx->enc setStencilReferenceValue:
            (uint32_t)(depth - 1)];
    }
}
