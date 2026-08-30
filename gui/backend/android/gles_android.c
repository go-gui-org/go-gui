#include "gles_android.h"

#include <GLES3/gl3.h>
#include <GLES3/gl3ext.h>
#include <string.h>
#include <stdlib.h>
#include <math.h>

#ifdef __ANDROID__
#include <android/log.h>
#define LOG_TAG "go-gui-gles"
#define LOGE(...) __android_log_print(ANDROID_LOG_ERROR, LOG_TAG, __VA_ARGS__)
#define LOGW(...) __android_log_print(ANDROID_LOG_WARN,  LOG_TAG, __VA_ARGS__)
#else
#include <stdio.h>
#define LOGE(...) fprintf(stderr, __VA_ARGS__)
#define LOGW(...) fprintf(stderr, __VA_ARGS__)
#endif

// ─── Global State ─────────────────────────────────────────────

static GLuint _programs[PIPE_COUNT];
static GLint  _mvpLocs[PIPE_COUNT];
static GLint  _tmLocs[PIPE_COUNT];
static GLint  _tm2Locs[PIPE_COUNT];

// Quad geometry (shared index buffer).
static GLuint _quadVAO, _quadVBO, _quadIBO;
// SVG triangles.
static GLuint _svgVAO, _svgVBO;
// Glyph quads.
static GLuint _glyphVAO, _glyphVBO;

// Viewport.
static int _viewW, _viewH;

// Current pipeline state.
static int _curPipeline = -1;

// Textures.
#define MAX_TEX 8192
static GLuint _textures[MAX_TEX];
static int _nextTexID = 1;
static int _freeTexIDs[MAX_TEX];
static int _freeTexCount = 0;

// Filter FBO state.
static GLuint _filterFBO;
static GLuint _filterTexA, _filterTexB;
static GLuint _filterStencilRB;
static int _filterW, _filterH;
static GLuint _filterQuadVAO, _filterQuadVBO;

// Main FBO stencil (0 = default framebuffer stencil).
// On Android GLSurfaceView provides stencil if requested.

// ─── Embedded GLSL ES 3.00 Shaders ───────────────────────────

// --- Solid ---
static const char* vs_solid_src =
    "#version 300 es\n"
    "precision highp float;\n"
    "precision highp int;\n"
    "layout(location=0) in vec3 position;\n"
    "layout(location=1) in vec2 texcoord0;\n"
    "layout(location=2) in vec4 color0;\n"
    "uniform mat4 mvp;\n"
    "out vec2 uv;\n"
    "out vec4 color;\n"
    "out float params;\n"
    "void main() {\n"
    "    gl_Position = mvp * vec4(position.xy, 0.0, 1.0);\n"
    "    uv = texcoord0;\n"
    "    color = color0;\n"
    "    params = position.z;\n"
    "}\n";

static const char* fs_solid_src =
    "#version 300 es\n"
    "precision highp float;\n"
    "precision highp int;\n"
    "uniform sampler2D tex;\n"
    "in vec2 uv;\n"
    "in vec4 color;\n"
    "in float params;\n"
    "out vec4 frag_color;\n"
    "void main() {\n"
    "    float radius = floor(params / 4096.0) / 4.0;\n"
    "    float thickness = mod(params, 4096.0) / 4.0;\n"
    "    vec2 uv_to_px = 1.0 / (vec2(fwidth(uv.x), fwidth(uv.y)) + 1e-6);\n"
    "    vec2 half_size = uv_to_px;\n"
    "    vec2 pos = uv * half_size;\n"
    "    vec2 q = abs(pos) - half_size + vec2(radius);\n"
    "    float d = length(max(q, 0.0)) + min(max(q.x, q.y), 0.0) - radius;\n"
    "    if (thickness > 0.0) {\n"
    "        d = abs(d + thickness * 0.5) - thickness * 0.5;\n"
    "    }\n"
    "    float grad_len = length(vec2(dFdx(d), dFdy(d)));\n"
    "    d = d / max(grad_len, 0.001);\n"
    "    float alpha = 1.0 - smoothstep(-0.59, 0.59, d);\n"
    "    frag_color = vec4(color.rgb, color.a * alpha);\n"
    "    if (frag_color.a < 0.0) {\n"
    "        frag_color += texture(tex, uv);\n"
    "    }\n"
    "}\n";

// --- Shadow ---
static const char* vs_shadow_src =
    "#version 300 es\n"
    "precision highp float;\n"
    "precision highp int;\n"
    "layout(location=0) in vec3 position;\n"
    "layout(location=1) in vec2 texcoord0;\n"
    "layout(location=2) in vec4 color0;\n"
    "uniform mat4 mvp;\n"
    "uniform mat4 tm;\n"
    "out vec2 uv;\n"
    "out vec4 color;\n"
    "out float params;\n"
    "out vec2 offset;\n"
    "out float spread;\n"
    "void main() {\n"
    "    gl_Position = mvp * vec4(position.xy, 0.0, 1.0);\n"
    "    uv = texcoord0;\n"
    "    color = color0;\n"
    "    params = position.z;\n"
    "    offset = (tm * vec4(0,0,0,1)).xy;\n"
    "    spread = (tm * vec4(0,0,0,1)).z;\n"
    "}\n";

static const char* fs_shadow_src =
    "#version 300 es\n"
    "precision highp float;\n"
    "precision highp int;\n"
    "uniform sampler2D tex;\n"
    "in vec2 uv;\n"
    "in vec4 color;\n"
    "in float params;\n"
    "in vec2 offset;\n"
    "in float spread;\n"
    "out vec4 frag_color;\n"
    "void main() {\n"
    "    float radius = floor(params / 4096.0) / 4.0;\n"
    "    float blur = mod(params, 4096.0) / 4.0;\n"
    "    vec2 uv_to_px = 1.0 / (vec2(fwidth(uv.x), fwidth(uv.y)) + 1e-6);\n"
    "    vec2 half_size = uv_to_px;\n"
    "    vec2 pos = uv * half_size;\n"
    "    vec2 q = abs(pos) - half_size + vec2(radius + 1.5 * blur);\n"
    "    float d = length(max(q, 0.0)) + min(max(q.x, q.y), 0.0) - radius;\n"
    "    float radius_caster = radius - spread;\n"
    "    vec2 q_c = abs(pos + offset) - (half_size - spread) + vec2(radius_caster + 1.5 * blur);\n"
    "    float d_c = length(max(q_c, 0.0)) + min(max(q_c.x, q_c.y), 0.0) - radius_caster;\n"
    /* Centred falloff: ~50% at the shadow's own edge, decaying over
       +/- one blur radius. Matches a Gaussian of sigma = blur/2, which
       is what the soft and web backends produce. */
    "    float b_half = max(1.0, blur);\n"
    "    float alpha_falloff = 1.0 - smoothstep(-b_half, b_half, d);\n"
    "    float alpha_clip = smoothstep(-1.0, 0.0, d_c);\n"
    "    float alpha = alpha_falloff * alpha_clip;\n"
    "    frag_color = vec4(color.rgb, color.a * alpha);\n"
    "    if (frag_color.a < 0.0) {\n"
    "        frag_color += texture(tex, uv);\n"
    "    }\n"
    "}\n";

// --- Blur ---
static const char* vs_blur_src =
    "#version 300 es\n"
    "precision highp float;\n"
    "precision highp int;\n"
    "layout(location=0) in vec3 position;\n"
    "layout(location=1) in vec2 texcoord0;\n"
    "layout(location=2) in vec4 color0;\n"
    "uniform mat4 mvp;\n"
    "out vec2 uv;\n"
    "out vec4 color;\n"
    "out float params;\n"
    "void main() {\n"
    "    gl_Position = mvp * vec4(position.xy, 0.0, 1.0);\n"
    "    uv = texcoord0;\n"
    "    color = color0;\n"
    "    params = position.z;\n"
    "}\n";

static const char* fs_blur_src =
    "#version 300 es\n"
    "precision highp float;\n"
    "precision highp int;\n"
    "uniform sampler2D tex;\n"
    "in vec2 uv;\n"
    "in vec4 color;\n"
    "in float params;\n"
    "out vec4 frag_color;\n"
    "void main() {\n"
    "    float radius = floor(params / 4096.0) / 4.0;\n"
    "    float blur = mod(params, 4096.0) / 4.0;\n"
    "    vec2 uv_to_px = 1.0 / (vec2(fwidth(uv.x), fwidth(uv.y)) + 1e-6);\n"
    "    vec2 half_size = uv_to_px;\n"
    "    vec2 pos = uv * half_size;\n"
    "    vec2 q = abs(pos) - half_size + vec2(radius + 1.5 * blur);\n"
    "    float d = length(max(q, 0.0)) + min(max(q.x, q.y), 0.0) - radius;\n"
    "    float alpha = 1.0 - smoothstep(-blur, blur, d);\n"
    "    frag_color = vec4(color.rgb, color.a * alpha);\n"
    "    if (frag_color.a < 0.0) {\n"
    "        frag_color += texture(tex, uv);\n"
    "    }\n"
    "}\n";

// --- Gradient ---
static const char* vs_gradient_src =
    "#version 300 es\n"
    "precision highp float;\n"
    "precision highp int;\n"
    "layout(location=0) in vec3 position;\n"
    "layout(location=1) in vec2 texcoord0;\n"
    "layout(location=2) in vec4 color0;\n"
    "uniform mat4 mvp;\n"
    "uniform mat4 tm;\n"
    "out vec2 uv;\n"
    "out vec4 color;\n"
    "out float params;\n"
    "out vec4 stop12;\n"
    "out vec4 stop34;\n"
    "out vec4 axis;\n"
    "out vec4 meta;\n"
    "void main() {\n"
    "    gl_Position = mvp * vec4(position.xy, 0.0, 1.0);\n"
    "    uv = texcoord0;\n"
    "    color = color0;\n"
    "    params = position.z;\n"
    "    stop12 = tm[0];\n"
    "    stop34 = tm[1];\n"
    "    axis = tm[2];\n"
    "    meta = tm[3];\n"
    "}\n";

static const char* fs_gradient_src =
    "#version 300 es\n"
    "precision highp float;\n"
    "precision highp int;\n"
    "uniform sampler2D tex;\n"
    // Stops 4-7. Constant across the quad, so reading the uniform in
    // the fragment stage beats four more varyings carrying it there.
    "uniform mat4 tm2;\n"
    "in vec2 uv;\n"
    "in vec4 color;\n"
    "in float params;\n"
    "in vec4 stop12;\n"
    "in vec4 stop34;\n"
    "in vec4 axis;\n"
    "in vec4 meta;\n"
    "out vec4 frag_color;\n"
    "float random(vec2 coords) {\n"
    "    return fract(sin(dot(coords.xy, vec2(12.9898,78.233))) * 43758.5453);\n"
    "}\n"
    "void unpack_gradient_data(float val1, float val2, out vec4 c, out float p) {\n"
    "    float r = mod(val1, 256.0);\n"
    "    float g = mod(floor(val1 / 256.0), 256.0);\n"
    "    float b = floor(val1 / 65536.0);\n"
    "    float a = mod(val2, 256.0);\n"
    "    p = floor(val2 / 256.0) / 10000.0;\n"
    "    c = vec4(r/255.0, g/255.0, b/255.0, a/255.0);\n"
    "}\n"
    "void main() {\n"
    "    float radius = floor(params / 4096.0) / 4.0;\n"
    "    float hw = meta.x;\n"
    "    float hh = meta.y;\n"
    "    float grad_type = meta.z;\n"
    "    int stop_count = int(meta.w);\n"
    "    vec2 pos = uv * vec2(hw, hh);\n"
    "    vec2 q = abs(pos) - vec2(hw, hh) + vec2(radius);\n"
    "    float d = length(max(q, 0.0)) + min(max(q.x, q.y), 0.0) - radius;\n"
    "    float grad_len = length(vec2(dFdx(d), dFdy(d)));\n"
    "    d = d / max(grad_len, 0.001);\n"
    "    float sdf_alpha = 1.0 - smoothstep(-0.59, 0.59, d);\n"
    "    float t;\n"
    "    if (grad_type > 0.5) {\n"
    "        float target_radius = axis.w;\n"
    "        t = length(pos) / target_radius;\n"
    "    } else {\n"
    "        vec2 stop_dir = vec2(axis.z, axis.w);\n"
    "        t = dot(uv, stop_dir) * 0.5 + 0.5;\n"
    "    }\n"
    "    t = clamp(t, 0.0, 1.0);\n"
    "    vec4 stop_colors[8];\n"
    "    float stop_positions[8];\n"
    "    unpack_gradient_data(stop12.x, stop12.y, stop_colors[0], stop_positions[0]);\n"
    "    unpack_gradient_data(stop12.z, stop12.w, stop_colors[1], stop_positions[1]);\n"
    "    unpack_gradient_data(stop34.x, stop34.y, stop_colors[2], stop_positions[2]);\n"
    "    unpack_gradient_data(stop34.z, stop34.w, stop_colors[3], stop_positions[3]);\n"
    "    unpack_gradient_data(tm2[0].x, tm2[0].y, stop_colors[4], stop_positions[4]);\n"
    "    unpack_gradient_data(tm2[0].z, tm2[0].w, stop_colors[5], stop_positions[5]);\n"
    "    unpack_gradient_data(tm2[1].x, tm2[1].y, stop_colors[6], stop_positions[6]);\n"
    "    unpack_gradient_data(tm2[1].z, tm2[1].w, stop_colors[7], stop_positions[7]);\n"
    "    vec4 c1 = stop_colors[0];\n"
    "    vec4 c2 = c1;\n"
    "    float p1 = stop_positions[0];\n"
    "    float p2 = p1;\n"
    "    for (int i = 1; i < 8; i++) {\n"
    "        if (i >= stop_count) break;\n"
    "        if (t <= stop_positions[i]) {\n"
    "            c2 = stop_colors[i];\n"
    "            p2 = stop_positions[i];\n"
    "            c1 = stop_colors[i-1];\n"
    "            p1 = stop_positions[i-1];\n"
    "            break;\n"
    "        }\n"
    "        if (i == stop_count - 1) {\n"
    "            c1 = stop_colors[i];\n"
    "            c2 = c1;\n"
    "            p1 = stop_positions[i];\n"
    "            p2 = p1;\n"
    "        }\n"
    "    }\n"
    "    float local_t = (t - p1) / max(p2 - p1, 0.0001);\n"
    "    vec3 c1_pre = c1.rgb * c1.a;\n"
    "    vec3 c2_pre = c2.rgb * c2.a;\n"
    "    vec3 rgb_pre = mix(c1_pre, c2_pre, local_t);\n"
    "    float alpha = mix(c1.a, c2.a, local_t);\n"
    "    vec3 rgb = rgb_pre / max(alpha, 0.0001);\n"
    "    vec4 gradient_color = vec4(rgb, alpha);\n"
    "    float dither = (random(gl_FragCoord.xy) - 0.5) / 255.0;\n"
    "    gradient_color.rgb += vec3(dither);\n"
    "    frag_color = vec4(gradient_color.rgb, gradient_color.a * sdf_alpha * color.a);\n"
    "    if (frag_color.a < 0.0) {\n"
    "        frag_color += texture(tex, uv);\n"
    "    }\n"
    "}\n";

// --- Image clip ---
static const char* fs_image_clip_src =
    "#version 300 es\n"
    "precision highp float;\n"
    "precision highp int;\n"
    "uniform sampler2D tex;\n"
    "in vec2 uv;\n"
    "in vec4 color;\n"
    "in float params;\n"
    "out vec4 frag_color;\n"
    "void main() {\n"
    "    float radius = floor(params / 4096.0) / 4.0;\n"
    "    vec2 uv_to_px = 1.0 / (vec2(fwidth(uv.x), fwidth(uv.y)) + 1e-6);\n"
    "    vec2 half_size = uv_to_px;\n"
    "    vec2 pos = uv * half_size;\n"
    "    vec2 q = abs(pos) - half_size + vec2(radius);\n"
    "    float d = length(max(q, 0.0)) + min(max(q.x, q.y), 0.0) - radius;\n"
    "    float grad_len = length(vec2(dFdx(d), dFdy(d)));\n"
    "    d = d / max(grad_len, 0.001);\n"
    "    float alpha = 1.0 - smoothstep(-0.59, 0.59, d);\n"
    "    vec2 tex_uv = uv * 0.5 + 0.5;\n"
    "    vec4 tex_color = texture(tex, tex_uv);\n"
    "    frag_color = vec4(tex_color.rgb, tex_color.a * alpha);\n"
    "}\n";

// --- Filter blur H ---
static const char* vs_filter_src =
    "#version 300 es\n"
    "precision highp float;\n"
    "precision highp int;\n"
    "layout(location=0) in vec3 position;\n"
    "layout(location=1) in vec2 texcoord0;\n"
    "layout(location=2) in vec4 color0;\n"
    "uniform mat4 mvp;\n"
    "uniform mat4 tm;\n"
    "out vec2 uv;\n"
    "out vec4 color;\n"
    "out float std_dev;\n"
    "void main() {\n"
    "    gl_Position = mvp * vec4(position.xy, 0.0, 1.0);\n"
    "    uv = texcoord0;\n"
    "    color = color0;\n"
    "    std_dev = tm[0][0];\n"
    "}\n";

static const char* fs_filter_blur_h_src =
    "#version 300 es\n"
    "precision highp float;\n"
    "precision highp int;\n"
    "uniform sampler2D tex_smp;\n"
    "in vec2 uv;\n"
    "in vec4 color;\n"
    "in float std_dev;\n"
    "out vec4 frag_color;\n"
    "void main() {\n"
    "    float w[7];\n"
    "    w[0]=0.19947; w[1]=0.17603; w[2]=0.12098;\n"
    "    w[3]=0.06476; w[4]=0.02700; w[5]=0.00877; w[6]=0.00222;\n"
    "    vec2 tex_size = vec2(textureSize(tex_smp, 0));\n"
    "    float step_size = std_dev / tex_size.x;\n"
    "    frag_color = texture(tex_smp, uv) * w[0];\n"
    "    for (int i = 1; i < 7; i++) {\n"
    "        float off = float(i) * step_size;\n"
    "        frag_color += texture(tex_smp, uv + vec2(off, 0.0)) * w[i];\n"
    "        frag_color += texture(tex_smp, uv - vec2(off, 0.0)) * w[i];\n"
    "    }\n"
    "}\n";

// --- Filter blur V ---
static const char* fs_filter_blur_v_src =
    "#version 300 es\n"
    "precision highp float;\n"
    "precision highp int;\n"
    "uniform sampler2D tex_smp;\n"
    "in vec2 uv;\n"
    "in vec4 color;\n"
    "in float std_dev;\n"
    "out vec4 frag_color;\n"
    "void main() {\n"
    "    float w[7];\n"
    "    w[0]=0.19947; w[1]=0.17603; w[2]=0.12098;\n"
    "    w[3]=0.06476; w[4]=0.02700; w[5]=0.00877; w[6]=0.00222;\n"
    "    vec2 tex_size = vec2(textureSize(tex_smp, 0));\n"
    "    float step_size = std_dev / tex_size.y;\n"
    "    frag_color = texture(tex_smp, uv) * w[0];\n"
    "    for (int i = 1; i < 7; i++) {\n"
    "        float off = float(i) * step_size;\n"
    "        frag_color += texture(tex_smp, uv + vec2(0.0, off)) * w[i];\n"
    "        frag_color += texture(tex_smp, uv - vec2(0.0, off)) * w[i];\n"
    "    }\n"
    "}\n";

// --- Filter texture ---
static const char* fs_filter_tex_src =
    "#version 300 es\n"
    "precision highp float;\n"
    "precision highp int;\n"
    "uniform sampler2D tex_smp;\n"
    "in vec2 uv;\n"
    "in vec4 color;\n"
    "in float std_dev;\n"
    "out vec4 frag_color;\n"
    "void main() {\n"
    "    frag_color = texture(tex_smp, uv) * color;\n"
    "}\n";

// --- Filter color matrix ---
static const char* fs_filter_color_src =
    "#version 300 es\n"
    "precision highp float;\n"
    "precision highp int;\n"
    "uniform sampler2D tex_smp;\n"
    "uniform mat4 tm;\n"
    "in vec2 uv;\n"
    "in vec4 color;\n"
    "in float std_dev;\n"
    "out vec4 frag_color;\n"
    "void main() {\n"
    "    vec4 src = texture(tex_smp, uv);\n"
    "    frag_color = clamp(tm * src, 0.0, 1.0);\n"
    "}\n";

// --- Glyph texture ---
static const char* vs_glyph_src =
    "#version 300 es\n"
    "precision highp float;\n"
    "precision highp int;\n"
    "layout(location=0) in vec2 position;\n"
    "layout(location=1) in vec2 texcoord0;\n"
    "layout(location=2) in vec4 color0;\n"
    "uniform mat4 mvp;\n"
    "out vec2 uv;\n"
    "out vec4 color;\n"
    "void main() {\n"
    "    gl_Position = mvp * vec4(position, 0.0, 1.0);\n"
    "    uv = texcoord0;\n"
    "    color = color0;\n"
    "}\n";

static const char* fs_glyph_tex_src =
    "#version 300 es\n"
    "precision highp float;\n"
    "precision highp int;\n"
    "uniform sampler2D tex;\n"
    "in vec2 uv;\n"
    "in vec4 color;\n"
    "out vec4 frag_color;\n"
    "void main() {\n"
    "    frag_color = texture(tex, uv) * color;\n"
    "}\n";

// --- Glyph color (no texture) ---
static const char* fs_glyph_color_src =
    "#version 300 es\n"
    "precision highp float;\n"
    "precision highp int;\n"
    "in vec2 uv;\n"
    "in vec4 color;\n"
    "out vec4 frag_color;\n"
    "void main() {\n"
    "    frag_color = color;\n"
    "}\n";

// --- Stencil ---
static const char* fs_stencil_src =
    "#version 300 es\n"
    "precision highp float;\n"
    "precision highp int;\n"
    "uniform sampler2D tex;\n"
    "in vec2 uv;\n"
    "in vec4 color;\n"
    "in float params;\n"
    "out vec4 frag_color;\n"
    "void main() {\n"
    "    float radius = floor(params / 4096.0) / 4.0;\n"
    "    vec2 uv_to_px = 1.0 / (vec2(fwidth(uv.x), fwidth(uv.y)) + 1e-6);\n"
    "    vec2 half_size = uv_to_px;\n"
    "    vec2 pos = uv * half_size;\n"
    "    vec2 q = abs(pos) - half_size + vec2(radius);\n"
    "    float d = length(max(q, 0.0)) + min(max(q.x, q.y), 0.0) - radius;\n"
    "    float grad_len = length(vec2(dFdx(d), dFdy(d)));\n"
    "    d = d / max(grad_len, 0.001);\n"
    "    float alpha = 1.0 - smoothstep(-0.59, 0.59, d);\n"
    "    if (alpha < 0.5) discard;\n"
    "    frag_color = vec4(1.0);\n"
    "    if (frag_color.a < 0.0) {\n"
    "        frag_color += texture(tex, uv);\n"
    "    }\n"
    "}\n";

// --- Custom shader vertex (same as VsCustomGLSL but ES 3.00) ---
static const char* vs_custom_src =
    "#version 300 es\n"
    "precision highp float;\n"
    "precision highp int;\n"
    "layout(location=0) in vec3 position;\n"
    "layout(location=1) in vec2 texcoord0;\n"
    "layout(location=2) in vec4 color0;\n"
    "uniform mat4 mvp;\n"
    "uniform mat4 tm;\n"
    "out vec2 uv;\n"
    "out vec4 color;\n"
    "out float params;\n"
    "out vec4 p0;\n"
    "out vec4 p1;\n"
    "out vec4 p2;\n"
    "out vec4 p3;\n"
    "void main() {\n"
    "    gl_Position = mvp * vec4(position.xy, 0.0, 1.0);\n"
    "    uv = texcoord0;\n"
    "    color = color0;\n"
    "    params = position.z;\n"
    "    p0 = tm[0];\n"
    "    p1 = tm[1];\n"
    "    p2 = tm[2];\n"
    "    p3 = tm[3];\n"
    "}\n";

// ─── Shader Compilation Helpers ───────────────────────────────

static GLuint compileShader(GLenum type, const char* src) {
    GLuint s = glCreateShader(type);
    glShaderSource(s, 1, &src, NULL);
    glCompileShader(s);
    GLint ok = 0;
    glGetShaderiv(s, GL_COMPILE_STATUS, &ok);
    if (!ok) {
        char log[1024];
        glGetShaderInfoLog(s, sizeof(log), NULL, log);
        LOGE("gles: shader compile: %s\n", log);
        glDeleteShader(s);
        return 0;
    }
    return s;
}

static GLuint buildProgram(const char* vsSrc,
                           const char* fsSrc) {
    GLuint vs = compileShader(GL_VERTEX_SHADER, vsSrc);
    if (!vs) return 0;
    GLuint fs = compileShader(GL_FRAGMENT_SHADER, fsSrc);
    if (!fs) { glDeleteShader(vs); return 0; }

    GLuint prog = glCreateProgram();
    glAttachShader(prog, vs);
    glAttachShader(prog, fs);
    glLinkProgram(prog);

    GLint ok = 0;
    glGetProgramiv(prog, GL_LINK_STATUS, &ok);
    if (!ok) {
        char log[1024];
        glGetProgramInfoLog(prog, sizeof(log), NULL, log);
        LOGE("gles: program link: %s\n", log);
        glDeleteProgram(prog);
        prog = 0;
    }

    glDeleteShader(vs);
    glDeleteShader(fs);
    return prog;
}

// ─── VAO/VBO Setup ───────────────────────────────────────────

// Main vertex: 9 floats (vec3 + vec2 + vec4), stride 36 bytes.
static void setupMainVAO(GLuint* vao, GLuint* vbo) {
    glGenVertexArrays(1, vao);
    glGenBuffers(1, vbo);
    glBindVertexArray(*vao);
    glBindBuffer(GL_ARRAY_BUFFER, *vbo);
    // position (location 0): vec3
    glEnableVertexAttribArray(0);
    glVertexAttribPointer(0, 3, GL_FLOAT, GL_FALSE, 36,
                          (void*)0);
    // texcoord (location 1): vec2
    glEnableVertexAttribArray(1);
    glVertexAttribPointer(1, 2, GL_FLOAT, GL_FALSE, 36,
                          (void*)12);
    // color (location 2): vec4
    glEnableVertexAttribArray(2);
    glVertexAttribPointer(2, 4, GL_FLOAT, GL_FALSE, 36,
                          (void*)20);
    glBindVertexArray(0);
}

// Glyph vertex: 8 floats (vec2 + vec2 + vec4), stride 32 bytes.
static void setupGlyphVAO(void) {
    glGenVertexArrays(1, &_glyphVAO);
    glGenBuffers(1, &_glyphVBO);
    glBindVertexArray(_glyphVAO);
    glBindBuffer(GL_ARRAY_BUFFER, _glyphVBO);
    // position (location 0): vec2
    glEnableVertexAttribArray(0);
    glVertexAttribPointer(0, 2, GL_FLOAT, GL_FALSE, 32,
                          (void*)0);
    // texcoord (location 1): vec2
    glEnableVertexAttribArray(1);
    glVertexAttribPointer(1, 2, GL_FLOAT, GL_FALSE, 32,
                          (void*)8);
    // color (location 2): vec4
    glEnableVertexAttribArray(2);
    glVertexAttribPointer(2, 4, GL_FLOAT, GL_FALSE, 32,
                          (void*)16);
    glBindVertexArray(0);
}

// ─── Public API ───────────────────────────────────────────────

int glesInit(void) {
    // Build all shader programs.
    _programs[PIPE_SOLID] =
        buildProgram(vs_solid_src, fs_solid_src);
    _programs[PIPE_SHADOW] =
        buildProgram(vs_shadow_src, fs_shadow_src);
    _programs[PIPE_BLUR] =
        buildProgram(vs_blur_src, fs_blur_src);
    _programs[PIPE_GRADIENT] =
        buildProgram(vs_gradient_src, fs_gradient_src);
    _programs[PIPE_IMAGE_CLIP] =
        buildProgram(vs_solid_src, fs_image_clip_src);
    _programs[PIPE_FILTER_BLUR_H] =
        buildProgram(vs_filter_src, fs_filter_blur_h_src);
    _programs[PIPE_FILTER_BLUR_V] =
        buildProgram(vs_filter_src, fs_filter_blur_v_src);
    _programs[PIPE_FILTER_TEX] =
        buildProgram(vs_filter_src, fs_filter_tex_src);
    _programs[PIPE_FILTER_COLOR] =
        buildProgram(vs_filter_src, fs_filter_color_src);
    _programs[PIPE_GLYPH_TEX] =
        buildProgram(vs_glyph_src, fs_glyph_tex_src);
    _programs[PIPE_GLYPH_COLOR] =
        buildProgram(vs_glyph_src, fs_glyph_color_src);
    _programs[PIPE_STENCIL] =
        buildProgram(vs_solid_src, fs_stencil_src);

    for (int i = 0; i < PIPE_COUNT; i++) {
        if (!_programs[i]) return -1;
        _mvpLocs[i] =
            glGetUniformLocation(_programs[i], "mvp");
        _tmLocs[i] =
            glGetUniformLocation(_programs[i], "tm");
        // Only the gradient program declares tm2; the rest resolve
        // to -1 and glesSetTM2 skips them.
        _tm2Locs[i] =
            glGetUniformLocation(_programs[i], "tm2");
    }

    // Create quad VAO/VBO/IBO.
    setupMainVAO(&_quadVAO, &_quadVBO);
    glBindVertexArray(_quadVAO);
    glGenBuffers(1, &_quadIBO);
    glBindBuffer(GL_ELEMENT_ARRAY_BUFFER, _quadIBO);
    uint16_t idx[6] = {0, 1, 2, 0, 2, 3};
    glBufferData(GL_ELEMENT_ARRAY_BUFFER, sizeof(idx), idx,
                 GL_STATIC_DRAW);
    glBindVertexArray(0);

    // SVG VAO/VBO.
    setupMainVAO(&_svgVAO, &_svgVBO);

    // Glyph VAO/VBO.
    setupGlyphVAO();

    // Enable blending.
    glEnable(GL_BLEND);
    glBlendFunc(GL_SRC_ALPHA, GL_ONE_MINUS_SRC_ALPHA);

    // Disable depth test.
    glDisable(GL_DEPTH_TEST);

    // Generate filter FBO (created lazily on first use).
    glGenFramebuffers(1, &_filterFBO);

    // Filter quad VAO/VBO.
    setupMainVAO(&_filterQuadVAO, &_filterQuadVBO);

    _curPipeline = -1;
    return 0;
}

void glesDestroy(void) {
    for (int i = 0; i < PIPE_COUNT; i++) {
        if (_programs[i]) glDeleteProgram(_programs[i]);
        _programs[i] = 0;
    }

    if (_quadVAO) { glDeleteVertexArrays(1, &_quadVAO); _quadVAO = 0; }
    if (_quadVBO) { glDeleteBuffers(1, &_quadVBO); _quadVBO = 0; }
    if (_quadIBO) { glDeleteBuffers(1, &_quadIBO); _quadIBO = 0; }
    if (_svgVAO) { glDeleteVertexArrays(1, &_svgVAO); _svgVAO = 0; }
    if (_svgVBO) { glDeleteBuffers(1, &_svgVBO); _svgVBO = 0; }
    if (_glyphVAO) { glDeleteVertexArrays(1, &_glyphVAO); _glyphVAO = 0; }
    if (_glyphVBO) { glDeleteBuffers(1, &_glyphVBO); _glyphVBO = 0; }
    if (_filterQuadVAO) { glDeleteVertexArrays(1, &_filterQuadVAO); _filterQuadVAO = 0; }
    if (_filterQuadVBO) { glDeleteBuffers(1, &_filterQuadVBO); _filterQuadVBO = 0; }

    for (int i = 1; i < MAX_TEX; i++) {
        if (_textures[i]) {
            glDeleteTextures(1, &_textures[i]);
            _textures[i] = 0;
        }
    }
    _nextTexID = 1;
    _freeTexCount = 0;

    if (_filterTexA) { glDeleteTextures(1, &_filterTexA); _filterTexA = 0; }
    if (_filterTexB) { glDeleteTextures(1, &_filterTexB); _filterTexB = 0; }
    if (_filterStencilRB) { glDeleteRenderbuffers(1, &_filterStencilRB); _filterStencilRB = 0; }
    if (_filterFBO) { glDeleteFramebuffers(1, &_filterFBO); _filterFBO = 0; }
    _filterW = 0;
    _filterH = 0;

    _curPipeline = -1;
}

void glesResize(int w, int h) {
    _viewW = w;
    _viewH = h;
    glViewport(0, 0, w, h);
}

void glesBeginFrame(float r, float g, float b, float a) {
    glBindFramebuffer(GL_FRAMEBUFFER, 0);
    glViewport(0, 0, _viewW, _viewH);
    glClearColor(r, g, b, a);
    glClear(GL_COLOR_BUFFER_BIT | GL_STENCIL_BUFFER_BIT);
    glDisable(GL_SCISSOR_TEST);
    glEnable(GL_BLEND);
    glBlendFunc(GL_SRC_ALPHA, GL_ONE_MINUS_SRC_ALPHA);
    glDisable(GL_STENCIL_TEST);
    _curPipeline = -1;
}

void glesEndFrame(void) {
    glFlush();
}

void glesSetPipeline(int id) {
    if (id < 0 || id >= PIPE_COUNT || !_programs[id]) return;
    glUseProgram(_programs[id]);
    _curPipeline = id;
}

void glesSetMVP(const float* m) {
    if (_curPipeline < 0 || _curPipeline >= PIPE_COUNT) return;
    GLint loc = _mvpLocs[_curPipeline];
    if (loc >= 0) {
        glUniformMatrix4fv(loc, 1, GL_FALSE, m);
    }
}

void glesSetTM(const float* m) {
    if (_curPipeline < 0 || _curPipeline >= PIPE_COUNT) return;
    GLint loc = _tmLocs[_curPipeline];
    if (loc >= 0) {
        glUniformMatrix4fv(loc, 1, GL_FALSE, m);
    }
}

void glesSetTM2(const float* m) {
    if (_curPipeline < 0 || _curPipeline >= PIPE_COUNT) return;
    GLint loc = _tm2Locs[_curPipeline];
    if (loc >= 0) {
        glUniformMatrix4fv(loc, 1, GL_FALSE, m);
    }
}

void glesSetScissor(int x, int y, int w, int h, int viewH) {
    if (x < 0) { w += x; x = 0; }
    if (y < 0) { h += y; y = 0; }
    if (w <= 0 || h <= 0) {
        glEnable(GL_SCISSOR_TEST);
        glScissor(0, 0, 1, 1);
        return;
    }
    if (x + w > _viewW) w = _viewW - x;
    if (y + h > _viewH) h = _viewH - y;
    if (w <= 0 || h <= 0) {
        glEnable(GL_SCISSOR_TEST);
        glScissor(0, 0, 1, 1);
        return;
    }
    glEnable(GL_SCISSOR_TEST);
    // Flip Y for OpenGL (origin bottom-left).
    glScissor(x, viewH - y - h, w, h);
}

void glesDisableScissor(void) {
    glDisable(GL_SCISSOR_TEST);
}

void glesDrawQuad(const float* verts) {
    glBindVertexArray(_quadVAO);
    glBindBuffer(GL_ARRAY_BUFFER, _quadVBO);
    glBufferData(GL_ARRAY_BUFFER, 4 * 36, verts,
                 GL_DYNAMIC_DRAW);
    glBindBuffer(GL_ELEMENT_ARRAY_BUFFER, _quadIBO);
    glDrawElements(GL_TRIANGLES, 6, GL_UNSIGNED_SHORT, 0);
    glBindVertexArray(0);
}

void glesDrawTriangles(const float* verts, int numVerts) {
    if (numVerts <= 0) return;
    glBindVertexArray(_svgVAO);
    glBindBuffer(GL_ARRAY_BUFFER, _svgVBO);
    glBufferData(GL_ARRAY_BUFFER, numVerts * 36, verts,
                 GL_DYNAMIC_DRAW);
    glDrawArrays(GL_TRIANGLES, 0, numVerts);
    glBindVertexArray(0);
}

void glesDrawGlyphQuad(const float* verts) {
    glBindVertexArray(_glyphVAO);
    glBindBuffer(GL_ARRAY_BUFFER, _glyphVBO);
    glBufferData(GL_ARRAY_BUFFER, 4 * 32, verts,
                 GL_DYNAMIC_DRAW);
    glDrawArrays(GL_TRIANGLE_FAN, 0, 4);
    glBindVertexArray(0);
}

// ─── Textures ─────────────────────────────────────────────────

int glesCreateTexture(int w, int h, const void* pixels,
                      int hasData) {
    int tid = 0;
    if (_freeTexCount > 0) {
        tid = _freeTexIDs[--_freeTexCount];
    } else {
        if (_nextTexID >= MAX_TEX) return 0;
        tid = _nextTexID++;
    }

    GLuint tex;
    glGenTextures(1, &tex);
    glBindTexture(GL_TEXTURE_2D, tex);
    glTexParameteri(GL_TEXTURE_2D, GL_TEXTURE_MIN_FILTER,
                    GL_LINEAR);
    glTexParameteri(GL_TEXTURE_2D, GL_TEXTURE_MAG_FILTER,
                    GL_LINEAR);
    glTexParameteri(GL_TEXTURE_2D, GL_TEXTURE_WRAP_S,
                    GL_CLAMP_TO_EDGE);
    glTexParameteri(GL_TEXTURE_2D, GL_TEXTURE_WRAP_T,
                    GL_CLAMP_TO_EDGE);

    if (hasData && pixels) {
        glTexImage2D(GL_TEXTURE_2D, 0, GL_RGBA8, w, h, 0,
                     GL_RGBA, GL_UNSIGNED_BYTE, pixels);
    } else {
        glTexImage2D(GL_TEXTURE_2D, 0, GL_RGBA8, w, h, 0,
                     GL_RGBA, GL_UNSIGNED_BYTE, NULL);
    }

    _textures[tid] = tex;
    return tid;
}

void glesUpdateTexture(int id, int x, int y, int w, int h,
                       const void* data) {
    if (id <= 0 || id >= MAX_TEX || !_textures[id]) return;
    glBindTexture(GL_TEXTURE_2D, _textures[id]);
    glTexSubImage2D(GL_TEXTURE_2D, 0, x, y, w, h,
                    GL_RGBA, GL_UNSIGNED_BYTE, data);
}

void glesDeleteTexture(int id) {
    if (id <= 0 || id >= MAX_TEX) return;
    if (!_textures[id]) return;
    glDeleteTextures(1, &_textures[id]);
    _textures[id] = 0;
    if (_freeTexCount < MAX_TEX) {
        _freeTexIDs[_freeTexCount++] = id;
    }
}

void glesBindTexture(int id) {
    if (id > 0 && id < MAX_TEX && _textures[id]) {
        glActiveTexture(GL_TEXTURE0);
        glBindTexture(GL_TEXTURE_2D, _textures[id]);
    }
}

// ─── Custom Shader Pipelines ─────────────────────────────────

#define MAX_CUSTOM_PIPELINES 32
static GLuint _customPrograms[MAX_CUSTOM_PIPELINES];
static GLint  _customMVPLocs[MAX_CUSTOM_PIPELINES];
static GLint  _customTMLocs[MAX_CUSTOM_PIPELINES];
static int _freeCustomIDs[MAX_CUSTOM_PIPELINES];
static int _freeCustomCount = 0;
static int _nextCustomID = 0;

int glesBuildCustomPipeline(const char* fragSrc) {
    GLuint prog = buildProgram(vs_custom_src, fragSrc);
    if (!prog) return -1;

    int idx = 0;
    if (_freeCustomCount > 0) {
        idx = _freeCustomIDs[--_freeCustomCount];
    } else {
        if (_nextCustomID >= MAX_CUSTOM_PIPELINES) {
            LOGE("gles: custom pipeline cache exhausted\n");
            glDeleteProgram(prog);
            return -1;
        }
        idx = _nextCustomID++;
    }
    _customPrograms[idx] = prog;
    _customMVPLocs[idx] =
        glGetUniformLocation(prog, "mvp");
    _customTMLocs[idx] =
        glGetUniformLocation(prog, "tm");
    return idx;
}

void glesDeleteCustomPipeline(int idx) {
    if (idx < 0 || idx >= MAX_CUSTOM_PIPELINES) return;
    if (!_customPrograms[idx]) return;
    glDeleteProgram(_customPrograms[idx]);
    _customPrograms[idx] = 0;
    if (_freeCustomCount < MAX_CUSTOM_PIPELINES) {
        _freeCustomIDs[_freeCustomCount++] = idx;
    }
}

void glesSetCustomPipeline(int idx) {
    if (idx < 0 || idx >= MAX_CUSTOM_PIPELINES ||
        !_customPrograms[idx])
        return;
    glUseProgram(_customPrograms[idx]);
    _curPipeline = -1; // Mark as custom.
}

// ─── Filter System ────────────────────────────────────────────

static void ensureFilterTextures(int w, int h) {
    if (_filterTexA && _filterW == w && _filterH == h) return;

    if (_filterTexA) glDeleteTextures(1, &_filterTexA);
    if (_filterTexB) glDeleteTextures(1, &_filterTexB);
    if (_filterStencilRB) glDeleteRenderbuffers(1, &_filterStencilRB);

    // Texture A.
    glGenTextures(1, &_filterTexA);
    glBindTexture(GL_TEXTURE_2D, _filterTexA);
    glTexImage2D(GL_TEXTURE_2D, 0, GL_RGBA8, w, h, 0,
                 GL_RGBA, GL_UNSIGNED_BYTE, NULL);
    glTexParameteri(GL_TEXTURE_2D, GL_TEXTURE_MIN_FILTER,
                    GL_LINEAR);
    glTexParameteri(GL_TEXTURE_2D, GL_TEXTURE_MAG_FILTER,
                    GL_LINEAR);
    glTexParameteri(GL_TEXTURE_2D, GL_TEXTURE_WRAP_S,
                    GL_CLAMP_TO_EDGE);
    glTexParameteri(GL_TEXTURE_2D, GL_TEXTURE_WRAP_T,
                    GL_CLAMP_TO_EDGE);

    // Texture B.
    glGenTextures(1, &_filterTexB);
    glBindTexture(GL_TEXTURE_2D, _filterTexB);
    glTexImage2D(GL_TEXTURE_2D, 0, GL_RGBA8, w, h, 0,
                 GL_RGBA, GL_UNSIGNED_BYTE, NULL);
    glTexParameteri(GL_TEXTURE_2D, GL_TEXTURE_MIN_FILTER,
                    GL_LINEAR);
    glTexParameteri(GL_TEXTURE_2D, GL_TEXTURE_MAG_FILTER,
                    GL_LINEAR);
    glTexParameteri(GL_TEXTURE_2D, GL_TEXTURE_WRAP_S,
                    GL_CLAMP_TO_EDGE);
    glTexParameteri(GL_TEXTURE_2D, GL_TEXTURE_WRAP_T,
                    GL_CLAMP_TO_EDGE);

    // Stencil renderbuffer.
    glGenRenderbuffers(1, &_filterStencilRB);
    glBindRenderbuffer(GL_RENDERBUFFER, _filterStencilRB);
    glRenderbufferStorage(GL_RENDERBUFFER, GL_STENCIL_INDEX8,
                          w, h);

    _filterW = w;
    _filterH = h;
}

static void bindFilterFBO(GLuint colorTex, int w, int h) {
    glBindFramebuffer(GL_FRAMEBUFFER, _filterFBO);
    glFramebufferTexture2D(GL_FRAMEBUFFER,
                           GL_COLOR_ATTACHMENT0,
                           GL_TEXTURE_2D, colorTex, 0);
    glFramebufferRenderbuffer(GL_FRAMEBUFFER,
                              GL_STENCIL_ATTACHMENT,
                              GL_RENDERBUFFER,
                              _filterStencilRB);
    glViewport(0, 0, w, h);
}

static void drawFilterQuad(int w, int h, int flipV) {
    float verts[36];
    // v0: 0,0
    verts[0] = 0; verts[1] = 0; verts[2] = 0;
    verts[3] = 0; verts[4] = flipV ? 1.0f : 0.0f;
    verts[5] = 1; verts[6] = 1; verts[7] = 1; verts[8] = 1;
    // v1: w,0
    verts[9] = (float)w; verts[10] = 0; verts[11] = 0;
    verts[12] = 1; verts[13] = flipV ? 1.0f : 0.0f;
    verts[14] = 1; verts[15] = 1; verts[16] = 1; verts[17] = 1;
    // v2: w,h
    verts[18] = (float)w; verts[19] = (float)h; verts[20] = 0;
    verts[21] = 1; verts[22] = flipV ? 0.0f : 1.0f;
    verts[23] = 1; verts[24] = 1; verts[25] = 1; verts[26] = 1;
    // v3: 0,h
    verts[27] = 0; verts[28] = (float)h; verts[29] = 0;
    verts[30] = 0; verts[31] = flipV ? 0.0f : 1.0f;
    verts[32] = 1; verts[33] = 1; verts[34] = 1; verts[35] = 1;

    glBindVertexArray(_filterQuadVAO);
    glBindBuffer(GL_ARRAY_BUFFER, _filterQuadVBO);
    glBufferData(GL_ARRAY_BUFFER, sizeof(verts), verts,
                 GL_DYNAMIC_DRAW);
    // Need IBO for indexed draw — use quad IBO.
    glBindBuffer(GL_ELEMENT_ARRAY_BUFFER, _quadIBO);
    glDrawElements(GL_TRIANGLES, 6, GL_UNSIGNED_SHORT, 0);
    glBindVertexArray(0);
}

static void setFilterMVP(int w, int h) {
    float mvp[16] = {0};
    mvp[0]  =  2.0f / w;
    mvp[5]  = -2.0f / h;
    mvp[10] = -1.0f;
    mvp[12] = -1.0f;
    mvp[13] =  1.0f;
    mvp[15] =  1.0f;
    GLint loc = _mvpLocs[_curPipeline >= 0 ? _curPipeline : 0];
    if (loc >= 0)
        glUniformMatrix4fv(loc, 1, GL_FALSE, mvp);
}

int glesBeginFilter(int w, int h) {
    ensureFilterTextures(w, h);
    if (!_filterTexA || !_filterTexB) return -1;

    // Bind FBO with filterTexA as color target.
    bindFilterFBO(_filterTexA, w, h);

    glClearColor(0, 0, 0, 0);
    glClear(GL_COLOR_BUFFER_BIT | GL_STENCIL_BUFFER_BIT);

    glEnable(GL_BLEND);
    glBlendFunc(GL_SRC_ALPHA, GL_ONE_MINUS_SRC_ALPHA);
    glDisable(GL_STENCIL_TEST);
    _curPipeline = -1;
    return 0;
}

void glesEndFilter(float blurRadius, int layers,
                   const float* colorMatrix) {
    if (layers < 1) layers = 1;
    int w = _filterW;
    int h = _filterH;

    GLuint compositeSrc = _filterTexA;

    // ── Blur passes ──
    if (blurRadius >= 1.0f) {
        float stdDev = blurRadius;

        // Horizontal blur: filterTexA → filterTexB
        bindFilterFBO(_filterTexB, w, h);
        glClearColor(0, 0, 0, 0);
        glClear(GL_COLOR_BUFFER_BIT);
        glDisable(GL_BLEND);

        glUseProgram(_programs[PIPE_FILTER_BLUR_H]);
        _curPipeline = PIPE_FILTER_BLUR_H;
        setFilterMVP(w, h);

        float tm[16] = {0};
        tm[0] = stdDev;
        GLint tmLoc = _tmLocs[PIPE_FILTER_BLUR_H];
        if (tmLoc >= 0)
            glUniformMatrix4fv(tmLoc, 1, GL_FALSE, tm);

        glActiveTexture(GL_TEXTURE0);
        glBindTexture(GL_TEXTURE_2D, _filterTexA);

        // Set tex_smp uniform.
        GLint texLoc = glGetUniformLocation(
            _programs[PIPE_FILTER_BLUR_H], "tex_smp");
        if (texLoc >= 0) glUniform1i(texLoc, 0);

        drawFilterQuad(w, h, 1);

        // Vertical blur: filterTexB → filterTexA
        bindFilterFBO(_filterTexA, w, h);
        glClearColor(0, 0, 0, 0);
        glClear(GL_COLOR_BUFFER_BIT);

        glUseProgram(_programs[PIPE_FILTER_BLUR_V]);
        _curPipeline = PIPE_FILTER_BLUR_V;
        setFilterMVP(w, h);

        tmLoc = _tmLocs[PIPE_FILTER_BLUR_V];
        if (tmLoc >= 0)
            glUniformMatrix4fv(tmLoc, 1, GL_FALSE, tm);

        glActiveTexture(GL_TEXTURE0);
        glBindTexture(GL_TEXTURE_2D, _filterTexB);

        texLoc = glGetUniformLocation(
            _programs[PIPE_FILTER_BLUR_V], "tex_smp");
        if (texLoc >= 0) glUniform1i(texLoc, 0);

        drawFilterQuad(w, h, 1);
    }

    // ── Color matrix pass ──
    if (colorMatrix != NULL) {
        bindFilterFBO(_filterTexB, w, h);
        glClearColor(0, 0, 0, 0);
        glClear(GL_COLOR_BUFFER_BIT);
        glDisable(GL_BLEND);

        glUseProgram(_programs[PIPE_FILTER_COLOR]);
        _curPipeline = PIPE_FILTER_COLOR;
        setFilterMVP(w, h);

        GLint tmLoc = _tmLocs[PIPE_FILTER_COLOR];
        if (tmLoc >= 0)
            glUniformMatrix4fv(tmLoc, 1, GL_FALSE,
                               colorMatrix);

        glActiveTexture(GL_TEXTURE0);
        glBindTexture(GL_TEXTURE_2D, _filterTexA);

        GLint texLoc = glGetUniformLocation(
            _programs[PIPE_FILTER_COLOR], "tex_smp");
        if (texLoc >= 0) glUniform1i(texLoc, 0);

        drawFilterQuad(w, h, 0);

        compositeSrc = _filterTexB;
    }

    // ── Resume main framebuffer ──
    glBindFramebuffer(GL_FRAMEBUFFER, 0);
    glViewport(0, 0, _viewW, _viewH);
    glEnable(GL_BLEND);
    glBlendFunc(GL_SRC_ALPHA, GL_ONE_MINUS_SRC_ALPHA);

    // ── Composite ──
    glUseProgram(_programs[PIPE_FILTER_TEX]);
    _curPipeline = PIPE_FILTER_TEX;

    float mvp[16] = {0};
    mvp[0]  =  2.0f / _viewW;
    mvp[5]  = -2.0f / _viewH;
    mvp[10] = -1.0f;
    mvp[12] = -1.0f;
    mvp[13] =  1.0f;
    mvp[15] =  1.0f;
    GLint loc = _mvpLocs[PIPE_FILTER_TEX];
    if (loc >= 0)
        glUniformMatrix4fv(loc, 1, GL_FALSE, mvp);

    float tm[16] = {0};
    tm[0] = 1; tm[5] = 1; tm[10] = 1; tm[15] = 1;
    GLint tmLoc = _tmLocs[PIPE_FILTER_TEX];
    if (tmLoc >= 0)
        glUniformMatrix4fv(tmLoc, 1, GL_FALSE, tm);

    glActiveTexture(GL_TEXTURE0);
    glBindTexture(GL_TEXTURE_2D, compositeSrc);

    GLint texLoc = glGetUniformLocation(
        _programs[PIPE_FILTER_TEX], "tex_smp");
    if (texLoc >= 0) glUniform1i(texLoc, 0);

    for (int i = 0; i < layers; i++) {
        drawFilterQuad(_viewW, _viewH, 0);
    }

    _curPipeline = -1;
}

// ─── Stencil Clip ─────────────────────────────────────────────

void glesBeginStencilClip(const float* verts, int depth) {
    glEnable(GL_STENCIL_TEST);

    // Increment stencil where SDF passes, no color writes.
    glStencilFunc(GL_ALWAYS, 0, 0xFF);
    glStencilOp(GL_KEEP, GL_KEEP, GL_INCR);
    glColorMask(GL_FALSE, GL_FALSE, GL_FALSE, GL_FALSE);

    glUseProgram(_programs[PIPE_STENCIL]);
    _curPipeline = PIPE_STENCIL;

    glBindVertexArray(_quadVAO);
    glBindBuffer(GL_ARRAY_BUFFER, _quadVBO);
    glBufferData(GL_ARRAY_BUFFER, 4 * 36, verts,
                 GL_DYNAMIC_DRAW);
    glBindBuffer(GL_ELEMENT_ARRAY_BUFFER, _quadIBO);
    glDrawElements(GL_TRIANGLES, 6, GL_UNSIGNED_SHORT, 0);
    glBindVertexArray(0);

    // Set test for children: pass where stencil >= depth.
    glColorMask(GL_TRUE, GL_TRUE, GL_TRUE, GL_TRUE);
    glStencilFunc(GL_LEQUAL, depth, 0xFF);
    glStencilOp(GL_KEEP, GL_KEEP, GL_KEEP);
}

void glesEndStencilClip(const float* verts, int depth) {
    // Decrement stencil where SDF passes, no color writes.
    glStencilFunc(GL_ALWAYS, 0, 0xFF);
    glStencilOp(GL_KEEP, GL_KEEP, GL_DECR);
    glColorMask(GL_FALSE, GL_FALSE, GL_FALSE, GL_FALSE);

    glUseProgram(_programs[PIPE_STENCIL]);
    _curPipeline = PIPE_STENCIL;

    glBindVertexArray(_quadVAO);
    glBindBuffer(GL_ARRAY_BUFFER, _quadVBO);
    glBufferData(GL_ARRAY_BUFFER, 4 * 36, verts,
                 GL_DYNAMIC_DRAW);
    glBindBuffer(GL_ELEMENT_ARRAY_BUFFER, _quadIBO);
    glDrawElements(GL_TRIANGLES, 6, GL_UNSIGNED_SHORT, 0);
    glBindVertexArray(0);

    glColorMask(GL_TRUE, GL_TRUE, GL_TRUE, GL_TRUE);

    if (depth <= 1) {
        glDisable(GL_STENCIL_TEST);
    } else {
        glStencilFunc(GL_LEQUAL, depth - 1, 0xFF);
        glStencilOp(GL_KEEP, GL_KEEP, GL_KEEP);
    }
}
