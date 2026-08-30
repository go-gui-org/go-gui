#ifndef GLES_ANDROID_H
#define GLES_ANDROID_H

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
int  glesInit(void);
void glesDestroy(void);
void glesResize(int w, int h);

// Frame
void glesBeginFrame(float r, float g, float b, float a);
void glesEndFrame(void);

// Pipeline and uniforms
void glesSetPipeline(int id);
void glesSetMVP(const float* m);
void glesSetTM(const float* m);
void glesSetTM2(const float* m);

// Scissor
void glesSetScissor(int x, int y, int w, int h, int viewH);
void glesDisableScissor(void);

// Drawing (main vertex format: 9 floats/vertex)
void glesDrawQuad(const float* verts);
void glesDrawTriangles(const float* verts, int numVerts);

// Drawing (glyph vertex format: 8 floats/vertex)
void glesDrawGlyphQuad(const float* verts);

// Textures
int  glesCreateTexture(int w, int h, const void* pixels,
                       int hasData);
void glesUpdateTexture(int id, int x, int y, int w, int h,
                       const void* data);
void glesDeleteTexture(int id);
void glesBindTexture(int id);

// Custom shader pipelines
int  glesBuildCustomPipeline(const char* fragSrc);
void glesDeleteCustomPipeline(int idx);
void glesSetCustomPipeline(int idx);

// Filter (glow) system
int  glesBeginFilter(int w, int h);
void glesEndFilter(float blurRadius, int layers,
                   const float* colorMatrix);

// Stencil clip
void glesBeginStencilClip(const float* verts, int depth);
void glesEndStencilClip(const float* verts, int depth);

#endif
