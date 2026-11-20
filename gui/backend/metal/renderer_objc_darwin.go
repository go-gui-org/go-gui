//go:build darwin && arm64 && !ios

package metal

// Metal constants, selectors and send helpers of the renderer
// (renderer_darwin.go).

import (
	"sync"
	"unsafe"

	"github.com/ebitengine/purego/objc"

	"github.com/go-gui-org/go-gui/gui/internal/darwinffi"
)

// Pipeline IDs, the index into metalCtx.pipelines.
const (
	pipeSolid = iota
	pipeShadow
	pipeBlur
	pipeGradient
	pipeImageClip
	pipeFilterBlurH
	pipeFilterBlurV
	pipeFilterTex
	pipeFilterColor
	pipeGlyphTex
	pipeGlyphColor
	pipeStencil
	pipeCount
)

const (
	maxTex = 8192
	// frameRing is the number of frames whose vertex data can be in use
	// at once. A CAMetalLayer has 3 drawables, so a fourth frame cannot
	// start before the first one is done. beginFrame also waits on the
	// command buffer of the slot it reuses, so the count is not a guess.
	frameRing = 3
	// vchunkBytes is the size of one shared vertex buffer. A frame takes
	// more chunks when it needs them, up to maxChunksPerFrame. 4 MiB
	// holds about 29,000 quads.
	vchunkBytes       = 4 << 20
	maxChunksPerFrame = 16
	// maxBatchQuads is the largest run of quads in one draw call. The
	// index buffer is uint16, so a run can address 65536 vertices.
	maxBatchQuads = 16384
	// Vertex sizes in bytes: gpu.Vertex (x, y, z, u, v, r, g, b, a) and
	// the glyph vertex (x, y, u, v, r, g, b, a).
	mainStride  = 36
	glyphStride = 32
	// setVertexBytes takes at most 4 KiB.
	maxInlineBytes = 4096
)

// Metal enum values, read from the macOS 27 SDK headers.
const (
	mtlPixelFormatR8Unorm    = 10
	mtlPixelFormatRG8Unorm   = 30
	mtlPixelFormatRGBA8Unorm = 70
	mtlPixelFormatStencil8   = 253

	mtlVertexFormatFloat2          = 29
	mtlVertexFormatFloat3          = 30
	mtlVertexFormatFloat4          = 31
	mtlVertexStepFunctionPerVertex = 1

	mtlBlendFactorSourceAlpha         = 4
	mtlBlendFactorOneMinusSourceAlpha = 5
	mtlColorWriteMaskNone             = 0

	mtlLoadActionLoad  = 1
	mtlLoadActionClear = 2

	mtlStoreActionDontCare                   = 0
	mtlStoreActionStore                      = 1
	mtlStoreActionMultisampleResolve         = 2
	mtlStoreActionStoreAndMultisampleResolve = 3
	mtlStoreActionUnknown                    = 4

	mtlCompareFunctionLessEqual       = 3
	mtlCompareFunctionAlways          = 7
	mtlStencilOperationKeep           = 0
	mtlStencilOperationIncrementClamp = 3
	mtlStencilOperationDecrementClamp = 4

	mtlSamplerMinMagFilterNearest    = 0
	mtlSamplerMinMagFilterLinear     = 1
	mtlSamplerAddressModeClampToEdge = 0

	mtlTextureUsageShaderRead    = 1
	mtlTextureUsageRenderTarget  = 4
	mtlTextureType2DMultisample  = 4
	mtlStorageModeShared         = 0
	mtlStorageModePrivate        = 2
	mtlResourceStorageModeShared = 0

	mtlPrimitiveTypeTriangle = 3
	mtlIndexTypeUInt16       = 0

	mtlLanguageVersion3_0           = 3 << 16
	mtlCommandBufferStatusCompleted = 4
)

// rsel holds the selectors of the renderer. loadRenderer fills it once.
var rsel struct {
	// Device and queue.
	newCommandQueue, newLibraryWithSourceOptionsError       objc.SEL
	newRenderPipelineStateWithDescriptorError               objc.SEL
	newDepthStencilStateWithDescriptor, newSamplerStateDesc objc.SEL
	newBufferWithBytes, newBufferWithLength                 objc.SEL
	newTextureWithDescriptor, supportsTextureSampleCount    objc.SEL
	newFunctionWithName, commandBuffer                      objc.SEL
	// Layer and drawable.
	setDevice, setPixelFormat, setFramebufferOnly                        objc.SEL
	setPresentsWithTransaction, presentsWithTransaction, setDrawableSize objc.SEL
	nextDrawable, texture, present, width, height                        objc.SEL
	// Descriptors.
	setLanguageVersion, attributes, layouts, objectAtIndexedSubscript objc.SEL
	setFormat, setOffset, setBufferIndex, setStride, setStepFunction  objc.SEL
	setRasterSampleCount, setVertexFunction, setFragmentFunction      objc.SEL
	setVertexDescriptor, colorAttachments, setBlendingEnabled         objc.SEL
	setSourceRGBBlendFactor, setDestinationRGBBlendFactor             objc.SEL
	setSourceAlphaBlendFactor, setDestinationAlphaBlendFactor         objc.SEL
	setWriteMask, setStencilAttachmentPixelFormat                     objc.SEL
	frontFaceStencil, setBackFaceStencil, setStencilCompareFunction   objc.SEL
	setStencilFailureOperation, setDepthFailureOperation              objc.SEL
	setDepthStencilPassOperation                                      objc.SEL
	setMinFilter, setMagFilter, setSAddressMode, setTAddressMode      objc.SEL
	setTextureType, setWidth, setHeight, setUsage, setStorageMode     objc.SEL
	setSampleCount                                                    objc.SEL
	setTexture, setResolveTexture, setLoadAction, setStoreAction      objc.SEL
	setClearColor, stencilAttachment, setClearStencil                 objc.SEL
	// Command buffer.
	renderCommandEncoderWithDescriptor, presentDrawable, commit objc.SEL
	waitUntilScheduled, waitUntilCompleted, status, errorSel    objc.SEL
	// Encoder.
	setViewport, setFragmentSamplerStateAtIndex, setRenderPipelineState objc.SEL
	setVertexBytes, setFragmentBytes, setScissorRect                    objc.SEL
	drawIndexedPrimitives, drawPrimitives                               objc.SEL
	setVertexBuffer, setVertexBufferOffset, setFragmentTexture          objc.SEL
	setDepthStencilState, setStencilReferenceValue                      objc.SEL
	setColorStoreAction, setStencilStoreAction, endEncoding             objc.SEL
	// Buffers, textures, errors.
	contents, replaceRegion, getBytes, localizedDescription objc.SEL
}

// rcls holds the classes of the renderer.
var rcls struct {
	compileOptions, vertexDesc, pipelineDesc, depthStencilDesc objc.ID
	samplerDesc, textureDesc, renderPassDesc, metalLayer       objc.ID
}

var (
	rendererOnce sync.Once
	// mtlCreateDevice is MTLCreateSystemDefaultDevice.
	mtlCreateDevice uintptr
)

// loadRenderer loads Metal and fills rsel and rcls.
func loadRenderer() {
	rendererOnce.Do(func() {
		loadObjc()
		metal := darwinffi.Open(darwinffi.PathMetal)
		darwinffi.Open(darwinffi.PathQuartzCore)
		mtlCreateDevice = darwinffi.Func(metal, "MTLCreateSystemDefaultDevice")
		for _, s := range []struct {
			p    *objc.SEL
			name string
		}{
			{&rsel.newCommandQueue, "newCommandQueue"},
			{&rsel.newLibraryWithSourceOptionsError, "newLibraryWithSource:options:error:"},
			{&rsel.newRenderPipelineStateWithDescriptorError, "newRenderPipelineStateWithDescriptor:error:"},
			{&rsel.newDepthStencilStateWithDescriptor, "newDepthStencilStateWithDescriptor:"},
			{&rsel.newSamplerStateDesc, "newSamplerStateWithDescriptor:"},
			{&rsel.newBufferWithBytes, "newBufferWithBytes:length:options:"},
			{&rsel.newBufferWithLength, "newBufferWithLength:options:"},
			{&rsel.newTextureWithDescriptor, "newTextureWithDescriptor:"},
			{&rsel.supportsTextureSampleCount, "supportsTextureSampleCount:"},
			{&rsel.newFunctionWithName, "newFunctionWithName:"},
			{&rsel.commandBuffer, "commandBuffer"},
			{&rsel.setDevice, "setDevice:"},
			{&rsel.setPixelFormat, "setPixelFormat:"},
			{&rsel.setFramebufferOnly, "setFramebufferOnly:"},
			{&rsel.setPresentsWithTransaction, "setPresentsWithTransaction:"},
			{&rsel.presentsWithTransaction, "presentsWithTransaction"},
			{&rsel.setDrawableSize, "setDrawableSize:"},
			{&rsel.nextDrawable, "nextDrawable"},
			{&rsel.texture, "texture"},
			{&rsel.present, "present"},
			{&rsel.width, "width"},
			{&rsel.height, "height"},
			{&rsel.setLanguageVersion, "setLanguageVersion:"},
			{&rsel.attributes, "attributes"},
			{&rsel.layouts, "layouts"},
			{&rsel.objectAtIndexedSubscript, "objectAtIndexedSubscript:"},
			{&rsel.setFormat, "setFormat:"},
			{&rsel.setOffset, "setOffset:"},
			{&rsel.setBufferIndex, "setBufferIndex:"},
			{&rsel.setStride, "setStride:"},
			{&rsel.setStepFunction, "setStepFunction:"},
			{&rsel.setRasterSampleCount, "setRasterSampleCount:"},
			{&rsel.setVertexFunction, "setVertexFunction:"},
			{&rsel.setFragmentFunction, "setFragmentFunction:"},
			{&rsel.setVertexDescriptor, "setVertexDescriptor:"},
			{&rsel.colorAttachments, "colorAttachments"},
			{&rsel.setBlendingEnabled, "setBlendingEnabled:"},
			{&rsel.setSourceRGBBlendFactor, "setSourceRGBBlendFactor:"},
			{&rsel.setDestinationRGBBlendFactor, "setDestinationRGBBlendFactor:"},
			{&rsel.setSourceAlphaBlendFactor, "setSourceAlphaBlendFactor:"},
			{&rsel.setDestinationAlphaBlendFactor, "setDestinationAlphaBlendFactor:"},
			{&rsel.setWriteMask, "setWriteMask:"},
			{&rsel.setStencilAttachmentPixelFormat, "setStencilAttachmentPixelFormat:"},
			{&rsel.frontFaceStencil, "frontFaceStencil"},
			{&rsel.setBackFaceStencil, "setBackFaceStencil:"},
			{&rsel.setStencilCompareFunction, "setStencilCompareFunction:"},
			{&rsel.setStencilFailureOperation, "setStencilFailureOperation:"},
			{&rsel.setDepthFailureOperation, "setDepthFailureOperation:"},
			{&rsel.setDepthStencilPassOperation, "setDepthStencilPassOperation:"},
			{&rsel.setMinFilter, "setMinFilter:"},
			{&rsel.setMagFilter, "setMagFilter:"},
			{&rsel.setSAddressMode, "setSAddressMode:"},
			{&rsel.setTAddressMode, "setTAddressMode:"},
			{&rsel.setTextureType, "setTextureType:"},
			{&rsel.setWidth, "setWidth:"},
			{&rsel.setHeight, "setHeight:"},
			{&rsel.setUsage, "setUsage:"},
			{&rsel.setStorageMode, "setStorageMode:"},
			{&rsel.setSampleCount, "setSampleCount:"},
			{&rsel.setTexture, "setTexture:"},
			{&rsel.setResolveTexture, "setResolveTexture:"},
			{&rsel.setLoadAction, "setLoadAction:"},
			{&rsel.setStoreAction, "setStoreAction:"},
			{&rsel.setClearColor, "setClearColor:"},
			{&rsel.stencilAttachment, "stencilAttachment"},
			{&rsel.setClearStencil, "setClearStencil:"},
			{&rsel.renderCommandEncoderWithDescriptor, "renderCommandEncoderWithDescriptor:"},
			{&rsel.presentDrawable, "presentDrawable:"},
			{&rsel.commit, "commit"},
			{&rsel.waitUntilScheduled, "waitUntilScheduled"},
			{&rsel.waitUntilCompleted, "waitUntilCompleted"},
			{&rsel.status, "status"},
			{&rsel.errorSel, "error"},
			{&rsel.setViewport, "setViewport:"},
			{&rsel.setFragmentSamplerStateAtIndex, "setFragmentSamplerState:atIndex:"},
			{&rsel.setRenderPipelineState, "setRenderPipelineState:"},
			{&rsel.setVertexBytes, "setVertexBytes:length:atIndex:"},
			{&rsel.setFragmentBytes, "setFragmentBytes:length:atIndex:"},
			{&rsel.setScissorRect, "setScissorRect:"},
			{&rsel.drawIndexedPrimitives, "drawIndexedPrimitives:indexCount:indexType:indexBuffer:indexBufferOffset:"},
			{&rsel.drawPrimitives, "drawPrimitives:vertexStart:vertexCount:"},
			{&rsel.setVertexBuffer, "setVertexBuffer:offset:atIndex:"},
			{&rsel.setVertexBufferOffset, "setVertexBufferOffset:atIndex:"},
			{&rsel.setFragmentTexture, "setFragmentTexture:atIndex:"},
			{&rsel.setDepthStencilState, "setDepthStencilState:"},
			{&rsel.setStencilReferenceValue, "setStencilReferenceValue:"},
			{&rsel.setColorStoreAction, "setColorStoreAction:atIndex:"},
			{&rsel.setStencilStoreAction, "setStencilStoreAction:"},
			{&rsel.endEncoding, "endEncoding"},
			{&rsel.contents, "contents"},
			{&rsel.replaceRegion, "replaceRegion:mipmapLevel:withBytes:bytesPerRow:"},
			{&rsel.getBytes, "getBytes:bytesPerRow:fromRegion:mipmapLevel:"},
			{&rsel.localizedDescription, "localizedDescription"},
		} {
			*s.p = darwinffi.Sel(s.name)
		}
		rcls.compileOptions = darwinffi.Class("MTLCompileOptions")
		rcls.vertexDesc = darwinffi.Class("MTLVertexDescriptor")
		rcls.pipelineDesc = darwinffi.Class("MTLRenderPipelineDescriptor")
		rcls.depthStencilDesc = darwinffi.Class("MTLDepthStencilDescriptor")
		rcls.samplerDesc = darwinffi.Class("MTLSamplerDescriptor")
		rcls.textureDesc = darwinffi.Class("MTLTextureDescriptor")
		rcls.renderPassDesc = darwinffi.Class("MTLRenderPassDescriptor")
		rcls.metalLayer = darwinffi.Class("CAMetalLayer")
	})
}

// ─── Send helpers ─────────────────────────────────────────────

// newObj returns [[cls alloc] init], retained.
func newObj(cls objc.ID) objc.ID {
	return objc.ID(msg(objc.ID(msg(cls, sels.alloc)), sels.init))
}

// release sends release to obj when it is not nil.
func release(obj objc.ID) {
	if obj != 0 {
		msg(obj, sels.release)
	}
}

// retain sends retain to obj when it is not nil and returns it.
func retain(obj objc.ID) objc.ID {
	if obj != 0 {
		msg(obj, sels.retain)
	}
	return obj
}

// at returns [arr objectAtIndexedSubscript:i], for the attribute,
// layout and color attachment arrays of the descriptors.
func at(arr objc.ID, i int) objc.ID {
	return objc.ID(msg(arr, rsel.objectAtIndexedSubscript, uintptr(i)))
}

// msgClearColor sends a setClearColor: with an MTLClearColor. The struct
// is four doubles, a homogeneous float aggregate, so it goes in D0-D3.
func msgClearColor(obj objc.ID, r, g, b, a float64) {
	mfc.D[0], mfc.D[1], mfc.D[2], mfc.D[3] = r, g, b, a
	msg(obj, rsel.setClearColor)
}

// ptr returns the address of p as a uintptr argument. p must be on the
// heap or global: a metalCtx field or a heap slice.
func ptr[T any](p *T) uintptr { return uintptr(unsafe.Pointer(p)) }

// bytesAt returns the n bytes at the address a, which is memory outside
// the Go heap (the contents of an MTLBuffer). The bits go through a
// pointer variable because go vet flags every direct uintptr-to-pointer
// cast.
func bytesAt(a uintptr, n int) []byte {
	var p *byte
	*(*uintptr)(unsafe.Pointer(&p)) = a
	return unsafe.Slice(p, n)
}

// nsErrorText returns the localized description of the NSError e.
func nsErrorText(e objc.ID) string {
	if e == 0 {
		return "unknown error"
	}
	return darwinffi.GoString(msg(e, rsel.localizedDescription))
}
