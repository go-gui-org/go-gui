#ifndef METAL_DARWIN_H
#define METAL_DARWIN_H

#include <stdint.h>

// Pipeline IDs — must match Go constants.
enum {
    PIPE_SOLID = 0,
    PIPE_SHADOW,
    PIPE_BLUR,
    PIPE_GRADIENT,
    PIPE_IMAGE_CLIP,
    PIPE_FILTER_BLUR_H,
    PIPE_FILTER_BLUR_V,
    PIPE_FILTER_TEX,
    PIPE_FILTER_COLOR,
    PIPE_GLYPH_TEX,
    PIPE_GLYPH_COLOR,
    PIPE_STENCIL,
    PIPE_COUNT,
};

// Lifecycle
// mslSrc is the MSL shader source, owned by Go
// (gui/backend/internal/msl) and copied during this call.
int  metalInit(void* metalLayer, const char* mslSrc);
void metalDestroy(void);
void metalResize(int w, int h);

// Frame
int  metalBeginFrame(float r, float g, float b, float a);
void metalEndFrame(void);

// Pipeline and uniforms
void metalSetPipeline(int id);
void metalSetMVP(const float* m);
void metalSetTM(const float* m);

// Scissor
void metalSetScissor(int x, int y, int w, int h, int viewH);
void metalDisableScissor(void);

// Drawing (main vertex format: 9 floats/vertex)
void metalDrawQuad(const float* verts);
void metalDrawTriangles(const float* verts, int numVerts);

// Drawing (glyph vertex format: 8 floats/vertex)
void metalDrawGlyphQuad(const float* verts);

// Textures
int  metalCreateTexture(int w, int h, const void* pixels,
                        int hasData);
void metalUpdateTexture(int id, int x, int y, int w, int h,
                        const void* data);
void metalDeleteTexture(int id);
void metalBindTexture(int id);

// Custom shader pipelines
int  metalBuildCustomPipeline(const char* mslSrc);
void metalDeleteCustomPipeline(int idx);
void metalSetCustomPipeline(int idx);

// Filter (glow) system
int  metalBeginFilter(int w, int h);
void metalEndFilter(float blurRadius, int layers,
                    const float* colorMatrix);

// Stencil clip
void metalBeginStencilClip(const float* verts, int depth);
void metalEndStencilClip(const float* verts, int depth);

#endif
