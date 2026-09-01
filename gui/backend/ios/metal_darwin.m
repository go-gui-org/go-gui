#import <Metal/Metal.h>
#import <QuartzCore/CAMetalLayer.h>
#include "metal_darwin.h"
#include <string.h>

// ─── Global State ─────────────────────────────────────────────

static id<MTLDevice>       _device;
static id<MTLCommandQueue> _queue;
static CAMetalLayer        *_layer;
static id<MTLSamplerState> _sampler;
static id<MTLBuffer>       _quadIdx;

static id<MTLRenderPipelineState> _pipelines[PIPE_COUNT];

// Per-frame state.
static id<CAMetalDrawable>          _drawable;
static id<MTLCommandBuffer>         _cmdBuf;
static id<MTLRenderCommandEncoder>  _enc;

// Viewport.
static int _viewW, _viewH;

// Textures.
#define MAX_TEX 8192
static id<MTLTexture> _textures[MAX_TEX];
static int _nextTexID = 1;
static int _freeTexIDs[MAX_TEX];
static int _freeTexCount = 0;

// Reusable large-triangle upload buffers (triple-buffered per frame
// to avoid per-draw allocations and CPU/GPU write hazards).
#define TRI_BUF_RING 3
#define TRI_BUF_MAX_PER_FRAME 256
static id<MTLBuffer> _triBufs[TRI_BUF_RING][TRI_BUF_MAX_PER_FRAME];
static int _triBufCursor[TRI_BUF_RING];
static int _triBufFrame = -1;

// Filter textures.
static id<MTLTexture> _filterTexA;
static id<MTLTexture> _filterTexB;
static id<MTLTexture> _filterStencilTex;
static int _filterW, _filterH;

// Stencil clip state.
static id<MTLTexture> _stencilTex;
static int _stencilTexW, _stencilTexH;
static id<MTLDepthStencilState> _stencilIncr;
static id<MTLDepthStencilState> _stencilTest;
static id<MTLDepthStencilState> _stencilDecr;
static id<MTLDepthStencilState> _stencilOff;

// ─── MSL Shader Compile Options ───────────────────────────────

// The MSL source itself is not here — it is shared with the macOS
// backend and lives in Go, at gui/backend/internal/msl. It arrives as
// a C string parameter.

// mslCompileOptions pins the MSL language version. See the macOS
// backend (gui/backend/metal/metal_darwin.m) for the full rationale:
// without a pin, the accepted feature set tracks the build machine's
// OS and newer constructs fail at runtime on older devices. Issue 126.
//
// 2.4, not the macOS backend's 3.0, because this target deploys to
// iOS 15 (-miphoneos-version-min=15.0) and MTLLanguageVersion3_0 is
// iOS 16+. Pinning above the deployment floor would reproduce issue
// 126 on iOS 15 devices and warns at build time. Raise both together
// only when the deployment target moves.
static MTLCompileOptions *mslCompileOptions(void) {
    MTLCompileOptions *opts = [[MTLCompileOptions alloc] init];
    opts.languageVersion = MTLLanguageVersion2_4;
    return opts;
}

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
        [_device newRenderPipelineStateWithDescriptor:desc
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
        [_device newRenderPipelineStateWithDescriptor:desc
                                                error:&err];
    if (!pso) {
        NSLog(@"metal: pipeline %@/%@: %@", vsName, fsName, err);
    }
    return pso;
}

// makePipelineStencilMask creates a pipeline with color writes
// disabled (colorWriteMask = None), used for stencil mask passes.
static id<MTLRenderPipelineState> makePipelineStencilMask(
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
        [_device newRenderPipelineStateWithDescriptor:desc
                                                error:&err];
    if (!pso) {
        NSLog(@"metal: stencil pipeline: %@", err);
    }
    return pso;
}

static id<MTLTexture> makeTexture(int w, int h,
    MTLPixelFormat fmt) {
    MTLTextureDescriptor *td =
        [MTLTextureDescriptor texture2DDescriptorWithPixelFormat:fmt
                              width:w height:h mipmapped:NO];
    td.usage = MTLTextureUsageShaderRead;
    td.storageMode = MTLStorageModeShared;
    return [_device newTextureWithDescriptor:td];
}

static id<MTLTexture> makeRenderTarget(int w, int h,
    MTLPixelFormat fmt) {
    MTLTextureDescriptor *td =
        [MTLTextureDescriptor texture2DDescriptorWithPixelFormat:fmt
                              width:w height:h mipmapped:NO];
    td.usage = MTLTextureUsageShaderRead |
               MTLTextureUsageRenderTarget;
    td.storageMode = MTLStorageModePrivate;
    return [_device newTextureWithDescriptor:td];
}

static void ensureStencilTexture(int w, int h) {
    if (_stencilTex && _stencilTexW == w && _stencilTexH == h)
        return;
    MTLTextureDescriptor *td = [MTLTextureDescriptor
        texture2DDescriptorWithPixelFormat:MTLPixelFormatStencil8
                                     width:w height:h
                                  mipmapped:NO];
    td.usage = MTLTextureUsageRenderTarget;
    td.storageMode = MTLStorageModePrivate;
    _stencilTex = [_device newTextureWithDescriptor:td];
    _stencilTexW = w;
    _stencilTexH = h;
}

// Start or resume the main render pass.
static void beginMainEncoder(float r, float g, float b,
    float a, int clear) {
    MTLRenderPassDescriptor *rpd =
        [MTLRenderPassDescriptor renderPassDescriptor];
    rpd.colorAttachments[0].texture = _drawable.texture;
    rpd.colorAttachments[0].storeAction = MTLStoreActionStore;
    if (clear) {
        rpd.colorAttachments[0].loadAction = MTLLoadActionClear;
        rpd.colorAttachments[0].clearColor =
            MTLClearColorMake(r, g, b, a);
    } else {
        rpd.colorAttachments[0].loadAction = MTLLoadActionLoad;
    }

    // Attach stencil buffer.
    ensureStencilTexture(_viewW, _viewH);
    if (_stencilTex) {
        rpd.stencilAttachment.texture = _stencilTex;
        rpd.stencilAttachment.storeAction = MTLStoreActionStore;
        if (clear) {
            rpd.stencilAttachment.loadAction =
                MTLLoadActionClear;
            rpd.stencilAttachment.clearStencil = 0;
        } else {
            rpd.stencilAttachment.loadAction = MTLLoadActionLoad;
        }
    }

    _enc = [_cmdBuf renderCommandEncoderWithDescriptor:rpd];
    [_enc setViewport:(MTLViewport){
        0, 0, (double)_viewW, (double)_viewH, 0, 1}];
    [_enc setFragmentSamplerState:_sampler atIndex:0];
}

// ─── Public API ───────────────────────────────────────────────

int metalInit(void* layerPtr, const char* mslSrc) {
    _layer = (__bridge CAMetalLayer*)layerPtr;
    _device = MTLCreateSystemDefaultDevice();
    if (!_device) {
        NSLog(@"metal: no Metal device");
        return -1;
    }

    _layer.device = _device;
    _layer.pixelFormat = MTLPixelFormatBGRA8Unorm;
    _layer.framebufferOnly = YES;
    // Synchronize presentation with the compositor resize
    // transaction. Eliminates content shift during live resize.
    _layer.presentsWithTransaction = YES;

    _queue = [_device newCommandQueue];

    // Compile MSL library. Source comes from Go — see
    // gui/backend/internal/msl.
    NSError *err = nil;
    NSString *src = [NSString stringWithUTF8String:mslSrc];
    id<MTLLibrary> lib =
        [_device newLibraryWithSource:src
                              options:mslCompileOptions()
                                error:&err];
    if (!lib) {
        NSLog(@"metal: compile shaders: %@", err);
        return -2;
    }

    MTLPixelFormat pf = MTLPixelFormatBGRA8Unorm;
    MTLVertexDescriptor *mvd = mainVertexDesc();
    MTLVertexDescriptor *gvd = glyphVertexDesc();

    // Build pipeline states.
    _pipelines[PIPE_SOLID] =
        makePipeline(lib, @"vs_solid", @"fs_solid", mvd, pf);
    _pipelines[PIPE_SHADOW] =
        makePipeline(lib, @"vs_shadow", @"fs_shadow", mvd, pf);
    _pipelines[PIPE_BLUR] =
        makePipeline(lib, @"vs_blur", @"fs_blur", mvd, pf);
    _pipelines[PIPE_GRADIENT] =
        makePipeline(lib, @"vs_gradient", @"fs_gradient",
                     mvd, pf);
    _pipelines[PIPE_IMAGE_CLIP] =
        makePipeline(lib, @"vs_solid", @"fs_image_clip",
                     mvd, pf);
    _pipelines[PIPE_FILTER_BLUR_H] =
        makePipelineReplace(lib, @"vs_filter",
                     @"fs_filter_blur_h", mvd, pf);
    _pipelines[PIPE_FILTER_BLUR_V] =
        makePipelineReplace(lib, @"vs_filter",
                     @"fs_filter_blur_v", mvd, pf);
    _pipelines[PIPE_FILTER_TEX] =
        makePipeline(lib, @"vs_filter", @"fs_filter_tex",
                     mvd, pf);
    _pipelines[PIPE_FILTER_COLOR] =
        makePipelineReplace(lib, @"vs_filter",
                     @"fs_filter_color", mvd, pf);
    _pipelines[PIPE_GLYPH_TEX] =
        makePipeline(lib, @"vs_glyph", @"fs_glyph_tex",
                     gvd, pf);
    _pipelines[PIPE_GLYPH_COLOR] =
        makePipeline(lib, @"vs_glyph", @"fs_glyph_color",
                     gvd, pf);
    _pipelines[PIPE_STENCIL] =
        makePipelineStencilMask(lib, @"vs_solid", @"fs_stencil",
                                mvd, pf);

    for (int i = 0; i < PIPE_COUNT; i++) {
        if (!_pipelines[i]) return -3;
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
        _stencilIncr =
            [_device newDepthStencilStateWithDescriptor:dsd];

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
        _stencilTest =
            [_device newDepthStencilStateWithDescriptor:dsd];

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
        _stencilDecr =
            [_device newDepthStencilStateWithDescriptor:dsd];

        // Disable stencil test.
        dsd = [[MTLDepthStencilDescriptor alloc] init];
        _stencilOff =
            [_device newDepthStencilStateWithDescriptor:dsd];
    }

    // Quad index buffer: two triangles [0,1,2, 0,2,3].
    uint16_t idx[6] = {0, 1, 2, 0, 2, 3};
    _quadIdx = [_device newBufferWithBytes:idx
                                    length:sizeof(idx)
                                   options:MTLResourceStorageModeShared];

    // Shared sampler (linear + clamp-to-edge).
    MTLSamplerDescriptor *sd = [[MTLSamplerDescriptor alloc] init];
    sd.minFilter    = MTLSamplerMinMagFilterLinear;
    sd.magFilter    = MTLSamplerMinMagFilterLinear;
    sd.sAddressMode = MTLSamplerAddressModeClampToEdge;
    sd.tAddressMode = MTLSamplerAddressModeClampToEdge;
    _sampler = [_device newSamplerStateWithDescriptor:sd];

    return 0;
}

// ─── Custom Shader Pipelines ─────────────────────────────────

#define MAX_CUSTOM_PIPELINES 32
static id<MTLRenderPipelineState>
    _customPipelines[MAX_CUSTOM_PIPELINES];
static int _freeCustomPipelineIDs[MAX_CUSTOM_PIPELINES];
static int _freeCustomPipelineCount = 0;
static int _nextCustomPipelineID = 0;

int metalBuildCustomPipeline(const char* mslSrc) {
    NSString *src = [NSString stringWithUTF8String:mslSrc];
    NSError *err = nil;
    // options:nil is deliberate here — unlike the built-in library,
    // this compiles user-supplied MSL, so the OS floor it targets is
    // the caller's decision, not ours. Do not apply the 3.0 pin.
    id<MTLLibrary> lib =
        [_device newLibraryWithSource:src
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
        [_device newRenderPipelineStateWithDescriptor:desc
                                                error:&err];
    if (!pso) {
        NSLog(@"metal: custom pipeline: %@", err);
        return -1;
    }

    int idx = 0;
    if (_freeCustomPipelineCount > 0) {
        idx = _freeCustomPipelineIDs[--_freeCustomPipelineCount];
    } else {
        if (_nextCustomPipelineID >= MAX_CUSTOM_PIPELINES) {
            NSLog(@"metal: custom pipeline cache exhausted");
            return -1;
        }
        idx = _nextCustomPipelineID++;
    }
    _customPipelines[idx] = pso;
    return idx;
}

void metalDeleteCustomPipeline(int idx) {
    if (idx < 0 || idx >= MAX_CUSTOM_PIPELINES) {
        return;
    }
    if (!_customPipelines[idx]) {
        return;
    }
    _customPipelines[idx] = nil;
    if (_freeCustomPipelineCount < MAX_CUSTOM_PIPELINES) {
        _freeCustomPipelineIDs[_freeCustomPipelineCount++] = idx;
    }
}

void metalSetCustomPipeline(int idx) {
    if (!_enc || idx < 0 || idx >= MAX_CUSTOM_PIPELINES ||
        !_customPipelines[idx])
        return;
    [_enc setRenderPipelineState:_customPipelines[idx]];
}

void metalDestroy(void) {
    for (int i = 0; i < MAX_TEX; i++) {
        _textures[i] = nil;
    }
    _nextTexID = 1;
    _freeTexCount = 0;
    for (int f = 0; f < TRI_BUF_RING; f++) {
        _triBufCursor[f] = 0;
        for (int i = 0; i < TRI_BUF_MAX_PER_FRAME; i++) {
            _triBufs[f][i] = nil;
        }
    }
    _triBufFrame = -1;
    _filterTexA = nil;
    _filterTexB = nil;
    _filterStencilTex = nil;
    _stencilTex = nil;
    _stencilTexW = 0;
    _stencilTexH = 0;
    _stencilIncr = nil;
    _stencilTest = nil;
    _stencilDecr = nil;
    _stencilOff  = nil;
    for (int i = 0; i < PIPE_COUNT; i++) {
        _pipelines[i] = nil;
    }
    for (int i = 0; i < MAX_CUSTOM_PIPELINES; i++) {
        _customPipelines[i] = nil;
    }
    _nextCustomPipelineID = 0;
    _freeCustomPipelineCount = 0;
    _quadIdx = nil;
    _sampler = nil;
    _queue   = nil;
    _device  = nil;
    _layer   = nil;
}

void metalResize(int w, int h) {
    _viewW = w;
    _viewH = h;
    _layer.drawableSize = CGSizeMake(w, h);
}

int metalBeginFrame(float r, float g, float b, float a) {
    @autoreleasepool {
        _drawable = [_layer nextDrawable];
    }
    if (!_drawable) return -1;

    _triBufFrame = (_triBufFrame + 1) % TRI_BUF_RING;
    _triBufCursor[_triBufFrame] = 0;

    _cmdBuf = [_queue commandBuffer];
    beginMainEncoder(r, g, b, a, 1);
    return 0;
}

void metalEndFrame(void) {
    if (_enc) {
        [_enc endEncoding];
        _enc = nil;
    }
    if (_drawable && _cmdBuf) {
        [_cmdBuf commit];
        [_cmdBuf waitUntilScheduled];
        [_drawable present];
    }
    _drawable = nil;
    _cmdBuf   = nil;
}

void metalSetPipeline(int id) {
    if (id < 0 || id >= PIPE_COUNT || !_enc) return;
    [_enc setRenderPipelineState:_pipelines[id]];
}

void metalSetMVP(const float* m) {
    if (!_enc) return;
    [_enc setVertexBytes:m length:64 atIndex:1];
}

void metalSetTM(const float* m) {
    if (!_enc) return;
    [_enc setVertexBytes:m length:64 atIndex:2];
}

// metalSetGradientTM2 hands the gradient fragment shader stops 4-7.
// The fragment stage has its own buffer index space, and the gradient
// pipeline binds nothing else there, so index 0 is free. Bound to the
// fragment stage rather than passed down as varyings because the
// values are constant across the quad.
void metalSetGradientTM2(const float* m) {
    if (!_enc) return;
    [_enc setFragmentBytes:m length:64 atIndex:0];
}

void metalSetScissor(int x, int y, int w, int h, int viewH) {
    if (!_enc) return;
    // Clamp to viewport.
    if (x < 0) { w += x; x = 0; }
    if (y < 0) { h += y; y = 0; }
    if (w <= 0 || h <= 0) {
        // Zero-area scissor: clip everything.
        [_enc setScissorRect:(MTLScissorRect){0, 0, 1, 1}];
        return;
    }
    if (x + w > _viewW) w = _viewW - x;
    if (y + h > _viewH) h = _viewH - y;
    if (w <= 0 || h <= 0) {
        [_enc setScissorRect:(MTLScissorRect){0, 0, 1, 1}];
        return;
    }
    [_enc setScissorRect:(MTLScissorRect){
        (NSUInteger)x, (NSUInteger)y,
        (NSUInteger)w, (NSUInteger)h}];
}

void metalDisableScissor(void) {
    if (!_enc) return;
    [_enc setScissorRect:(MTLScissorRect){
        0, 0, (NSUInteger)_viewW, (NSUInteger)_viewH}];
}

void metalDrawQuad(const float* verts) {
    if (!_enc) return;
    [_enc setVertexBytes:verts length:4*36 atIndex:0];
    [_enc drawIndexedPrimitives:MTLPrimitiveTypeTriangle
                     indexCount:6
                      indexType:MTLIndexTypeUInt16
                    indexBuffer:_quadIdx
              indexBufferOffset:0];
}

void metalDrawTriangles(const float* verts, int numVerts) {
    if (!_enc || numVerts <= 0) return;
    int byteLen = numVerts * 36;
    if (byteLen <= 4096) {
        [_enc setVertexBytes:verts length:byteLen atIndex:0];
    } else {
        id<MTLBuffer> buf = nil;
        if (_triBufFrame >= 0 && _triBufFrame < TRI_BUF_RING) {
            int slot = _triBufCursor[_triBufFrame]++;
            if (slot < TRI_BUF_MAX_PER_FRAME) {
                buf = _triBufs[_triBufFrame][slot];
                if (!buf || [buf length] < (NSUInteger)byteLen) {
                    NSUInteger cap = (NSUInteger)byteLen;
                    NSUInteger page = 4096;
                    cap = ((cap + page - 1) / page) * page;
                    buf = [_device newBufferWithLength:cap
                                               options:MTLResourceStorageModeShared];
                    _triBufs[_triBufFrame][slot] = buf;
                }
                memcpy([buf contents], verts, (size_t)byteLen);
            }
        }
        if (!buf) {
            // Pool exhausted — allocate a one-off buffer.
            buf = [_device newBufferWithBytes:verts
                                       length:(NSUInteger)byteLen
                                      options:MTLResourceStorageModeShared];
        }
        if (!buf) return;
        [_enc setVertexBuffer:buf offset:0 atIndex:0];
    }
    [_enc drawPrimitives:MTLPrimitiveTypeTriangle
             vertexStart:0 vertexCount:numVerts];
}

void metalDrawGlyphQuad(const float* verts) {
    if (!_enc) return;
    [_enc setVertexBytes:verts length:4*32 atIndex:0];
    [_enc drawIndexedPrimitives:MTLPrimitiveTypeTriangle
                     indexCount:6
                      indexType:MTLIndexTypeUInt16
                    indexBuffer:_quadIdx
              indexBufferOffset:0];
}

// ─── Textures ─────────────────────────────────────────────────

int metalCreateTexture(int w, int h, const void* pixels,
                       int hasData) {
    int tid = 0;
    if (_freeTexCount > 0) {
        tid = _freeTexIDs[--_freeTexCount];
    } else {
        if (_nextTexID >= MAX_TEX) return 0;
        tid = _nextTexID++;
    }

    id<MTLTexture> tex = makeTexture(w, h,
        MTLPixelFormatRGBA8Unorm);
    if (!tex) {
        if (_freeTexCount < MAX_TEX) {
            _freeTexIDs[_freeTexCount++] = tid;
        }
        return 0;
    }
    if (hasData && pixels) {
        [tex replaceRegion:MTLRegionMake2D(0, 0, w, h)
               mipmapLevel:0
                 withBytes:pixels
               bytesPerRow:w * 4];
    }
    _textures[tid] = tex;
    return tid;
}

void metalUpdateTexture(int id, int x, int y, int w, int h,
                        const void* data) {
    if (id <= 0 || id >= MAX_TEX || !_textures[id]) return;
    [_textures[id] replaceRegion:MTLRegionMake2D(x, y, w, h)
                     mipmapLevel:0
                       withBytes:data
                     bytesPerRow:w * 4];
}

void metalDeleteTexture(int id) {
    if (id <= 0 || id >= MAX_TEX) return;
    if (!_textures[id]) return;
    _textures[id] = nil;
    if (_freeTexCount < MAX_TEX) {
        _freeTexIDs[_freeTexCount++] = id;
    }
}

void metalBindTexture(int id) {
    if (!_enc) return;
    if (id > 0 && id < MAX_TEX && _textures[id]) {
        [_enc setFragmentTexture:_textures[id] atIndex:0];
    }
}

// ─── Filter System ────────────────────────────────────────────

static void ensureFilterTextures(int w, int h) {
    if (_filterTexA && _filterW == w && _filterH == h) return;
    MTLPixelFormat pf = MTLPixelFormatBGRA8Unorm;
    _filterTexA = makeRenderTarget(w, h, pf);
    _filterTexB = makeRenderTarget(w, h, pf);
    // Stencil attachment so ClipContents works inside filters.
    MTLTextureDescriptor *std = [MTLTextureDescriptor
        texture2DDescriptorWithPixelFormat:MTLPixelFormatStencil8
                                     width:w height:h
                                  mipmapped:NO];
    std.usage = MTLTextureUsageRenderTarget;
    std.storageMode = MTLStorageModePrivate;
    _filterStencilTex = [_device newTextureWithDescriptor:std];
    _filterW = w;
    _filterH = h;
}

int metalBeginFilter(int w, int h) {
    if (!_enc || !_cmdBuf) return -1;

    ensureFilterTextures(w, h);
    if (!_filterTexA || !_filterTexB) return -2;

    // End current main encoder.
    [_enc endEncoding];
    _enc = nil;

    // Start render pass targeting filterTexA.
    MTLRenderPassDescriptor *rpd =
        [MTLRenderPassDescriptor renderPassDescriptor];
    rpd.colorAttachments[0].texture     = _filterTexA;
    rpd.colorAttachments[0].loadAction  = MTLLoadActionClear;
    rpd.colorAttachments[0].storeAction = MTLStoreActionStore;
    rpd.colorAttachments[0].clearColor  =
        MTLClearColorMake(0, 0, 0, 0);

    // Attach stencil so ClipContents works inside filters.
    if (_filterStencilTex) {
        rpd.stencilAttachment.texture     = _filterStencilTex;
        rpd.stencilAttachment.loadAction  = MTLLoadActionClear;
        rpd.stencilAttachment.storeAction = MTLStoreActionStore;
        rpd.stencilAttachment.clearStencil = 0;
    }

    _enc = [_cmdBuf renderCommandEncoderWithDescriptor:rpd];
    [_enc setViewport:(MTLViewport){
        0, 0, (double)w, (double)h, 0, 1}];
    [_enc setFragmentSamplerState:_sampler atIndex:0];
    return 0;
}

void metalEndFilter(float blurRadius, int layers,
                    const float* colorMatrix) {
    if (!_enc || !_cmdBuf) return;

    if (layers < 1) layers = 1;

    int w = _filterW;
    int h = _filterH;

    // End filter content encoder.
    [_enc endEncoding];
    _enc = nil;

    // compositeSrc tracks which texture holds the final result.
    id<MTLTexture> compositeSrc = _filterTexA;

    // ── Blur passes (skip when blurRadius < 1) ──
    if (blurRadius >= 1) {
        float stdDev = blurRadius;

        // Horizontal blur: filterTexA → filterTexB
        {
            MTLRenderPassDescriptor *rpd =
                [MTLRenderPassDescriptor renderPassDescriptor];
            rpd.colorAttachments[0].texture     = _filterTexB;
            rpd.colorAttachments[0].loadAction  = MTLLoadActionClear;
            rpd.colorAttachments[0].storeAction = MTLStoreActionStore;
            rpd.colorAttachments[0].clearColor  =
                MTLClearColorMake(0, 0, 0, 0);

            id<MTLRenderCommandEncoder> enc =
                [_cmdBuf renderCommandEncoderWithDescriptor:rpd];
            [enc setViewport:(MTLViewport){
                0, 0, (double)w, (double)h, 0, 1}];
            [enc setRenderPipelineState:
                _pipelines[PIPE_FILTER_BLUR_H]];
            [enc setFragmentSamplerState:_sampler atIndex:0];
            [enc setFragmentTexture:_filterTexA atIndex:0];

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
                           indexBuffer:_quadIdx
                     indexBufferOffset:0];
            [enc endEncoding];
        }

        // Vertical blur: filterTexB → filterTexA
        {
            MTLRenderPassDescriptor *rpd =
                [MTLRenderPassDescriptor renderPassDescriptor];
            rpd.colorAttachments[0].texture     = _filterTexA;
            rpd.colorAttachments[0].loadAction  = MTLLoadActionClear;
            rpd.colorAttachments[0].storeAction = MTLStoreActionStore;
            rpd.colorAttachments[0].clearColor  =
                MTLClearColorMake(0, 0, 0, 0);

            id<MTLRenderCommandEncoder> enc =
                [_cmdBuf renderCommandEncoderWithDescriptor:rpd];
            [enc setViewport:(MTLViewport){
                0, 0, (double)w, (double)h, 0, 1}];
            [enc setRenderPipelineState:
                _pipelines[PIPE_FILTER_BLUR_V]];
            [enc setFragmentSamplerState:_sampler atIndex:0];
            [enc setFragmentTexture:_filterTexB atIndex:0];

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
                           indexBuffer:_quadIdx
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
        rpd.colorAttachments[0].texture     = _filterTexB;
        rpd.colorAttachments[0].loadAction  = MTLLoadActionClear;
        rpd.colorAttachments[0].storeAction = MTLStoreActionStore;
        rpd.colorAttachments[0].clearColor  =
            MTLClearColorMake(0, 0, 0, 0);

        id<MTLRenderCommandEncoder> enc =
            [_cmdBuf renderCommandEncoderWithDescriptor:rpd];
        [enc setViewport:(MTLViewport){
            0, 0, (double)w, (double)h, 0, 1}];
        [enc setRenderPipelineState:
            _pipelines[PIPE_FILTER_COLOR]];
        [enc setFragmentSamplerState:_sampler atIndex:0];
        [enc setFragmentTexture:_filterTexA atIndex:0];
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
                       indexBuffer:_quadIdx
                 indexBufferOffset:0];
        [enc endEncoding];

        compositeSrc = _filterTexB;
    }

    // ── Resume main render pass (load, not clear) ──
    beginMainEncoder(0, 0, 0, 0, 0);

    // ── Composite: draw result texture onto main drawable ──
    [_enc setRenderPipelineState:_pipelines[PIPE_FILTER_TEX]];
    [_enc setFragmentTexture:compositeSrc atIndex:0];

    float mvp[16] = {0};
    mvp[0]  =  2.0f / _viewW;
    mvp[5]  = -2.0f / _viewH;
    mvp[10] = -1.0f;
    mvp[12] = -1.0f;
    mvp[13] =  1.0f;
    mvp[15] =  1.0f;
    [_enc setVertexBytes:mvp length:64 atIndex:1];

    float tm[16] = {0};
    tm[0] = 1; tm[5] = 1; tm[10] = 1; tm[15] = 1;
    [_enc setVertexBytes:tm length:64 atIndex:2];

    // H-blur and V-blur each flip V, cancelling out. The color
    // pass uses non-flipped UVs. Composite always uses non-flipped
    // UVs (v=0 at screen-top → texture-top).
    float verts[] = {
        0,0,0, 0,0, 1,1,1,1,
        (float)_viewW,0,0, 1,0, 1,1,1,1,
        (float)_viewW,(float)_viewH,0, 1,1, 1,1,1,1,
        0,(float)_viewH,0, 0,1, 1,1,1,1,
    };
    [_enc setVertexBytes:verts length:sizeof(verts) atIndex:0];
    for (int i = 0; i < layers; i++) {
        [_enc drawIndexedPrimitives:MTLPrimitiveTypeTriangle
                         indexCount:6
                          indexType:MTLIndexTypeUInt16
                        indexBuffer:_quadIdx
                  indexBufferOffset:0];
    }
}

// ─── Stencil Clip ─────────────────────────────────────────────

void metalBeginStencilClip(const float* verts, int depth) {
    if (!_enc) return;

    // Increment stencil where SDF passes, no color output.
    [_enc setDepthStencilState:_stencilIncr];
    [_enc setRenderPipelineState:_pipelines[PIPE_STENCIL]];
    [_enc setVertexBytes:verts length:4*36 atIndex:0];
    [_enc drawIndexedPrimitives:MTLPrimitiveTypeTriangle
                     indexCount:6
                      indexType:MTLIndexTypeUInt16
                    indexBuffer:_quadIdx
              indexBufferOffset:0];

    // Set stencil test for children.
    [_enc setDepthStencilState:_stencilTest];
    [_enc setStencilReferenceValue:(uint32_t)depth];
}

void metalEndStencilClip(const float* verts, int depth) {
    if (!_enc) return;

    // Decrement stencil where SDF passes, no color output.
    [_enc setDepthStencilState:_stencilDecr];
    [_enc setRenderPipelineState:_pipelines[PIPE_STENCIL]];
    [_enc setVertexBytes:verts length:4*36 atIndex:0];
    [_enc drawIndexedPrimitives:MTLPrimitiveTypeTriangle
                     indexCount:6
                      indexType:MTLIndexTypeUInt16
                    indexBuffer:_quadIdx
              indexBufferOffset:0];

    if (depth <= 1) {
        [_enc setDepthStencilState:_stencilOff];
    } else {
        [_enc setDepthStencilState:_stencilTest];
        [_enc setStencilReferenceValue:(uint32_t)(depth - 1)];
    }
}
