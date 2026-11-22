//go:build darwin && arm64 && !ios

package metal

// The Metal renderer, in Go through purego. It was metal_darwin.m until
// #STEP6. One metalCtx is the renderer of one window: its device, queue,
// pipelines, textures and the encoder of the frame being drawn.
//
// Every send goes through mfc (msg_darwin_arm64.go), so the draw path
// makes no heap allocation. A send through purego costs about 2.4 times
// a send inside C, so the draw path also sends less than the C code did:
//
//   - Quads are batched. drawQuad and drawGlyphQuad copy their four
//     vertices into a shared vertex buffer and send nothing. One
//     drawIndexedPrimitives call draws the whole run of quads when the
//     encoder state must change (flush).
//   - Encoder state is cached. The Go draw code sets the pipeline and the
//     MVP before each draw. A setter whose value is already bound sends
//     nothing and keeps the batch open, so a run of rects or glyphs is one
//     draw call.
//
// A batch holds quads that use the same encoder state, so batching does
// not change what is drawn. Each setter flushes the batch before it
// changes the state, and every draw that does not batch flushes first.
//
// Ownership: an object from alloc, new or copy (and from
// MTLCreateSystemDefaultDevice) is retained, and this code releases it.
// Other results are autoreleased, and the pool of the frame (or of the
// setup call) drains them. Pointer arguments point to fields of the
// metalCtx, which is on the heap, or to Go slices on the heap, never to a
// stack local: see darwinffi.Caller.

import (
	"log"
	"runtime"
	"time"
	"unsafe"

	"github.com/ebitengine/purego/objc"

	"github.com/go-gui-org/go-gui/gui/backend/internal/rstats"
	"github.com/go-gui-org/go-gui/gui/internal/darwinffi"
)

// ─── metalCtx ─────────────────────────────────────────────────

// vchunk is one shared vertex buffer and the CPU address of its bytes.
type vchunk struct {
	buf  objc.ID
	addr uintptr
}

// encState is what the encoder has bound. A new encoder starts with
// nothing known (the zero value), so every setter sends once.
type encState struct {
	pipe     objc.ID
	mvpOK    bool
	tmOK     bool
	frag0OK  bool
	scissor  [4]uint
	scissOK  bool
	tex0     objc.ID
	sampler  objc.ID
	channels [4]objc.ID
	// vbuf is the buffer bound at vertex index 0 by setVertexBuffer, or
	// 0 when setVertexBytes wrote there last.
	vbuf objc.ID
}

// metalCtx is the renderer of one window. Main thread only.
type metalCtx struct {
	device, queue, layer objc.ID
	sampler              objc.ID
	// samplerNearest is bound only for one image draw that asks for
	// gui.ImageFilterNearest. sampler is bound otherwise.
	samplerNearest objc.ID
	quadIdx        objc.ID
	pipelines      [pipeCount]objc.ID

	// Per frame. target is the single-sample texture the main pass
	// resolves into: the drawable texture in a frame, an offscreen
	// texture in the edge coverage probe.
	drawable, target, cmdBuf, enc objc.ID
	// stats counts draws and batch causes when GOGUI_RENDER_STATS is set
	// (#1303). It is nil otherwise, and every call on it returns at once.
	stats     *rstats.Recorder
	framePool uintptr

	viewW, viewH int

	// Textures. The IDs start at 1, and 0 means "no texture".
	textures     [maxTex]objc.ID
	nextTexID    int
	freeTexIDs   [maxTex]int32
	freeTexCount int
	// A streamed texture is rewritten while earlier frames can still
	// sample it (gui.PixelSurface). texLastUse holds, retained, the
	// command buffer of the last frame that bound it. See
	// waitTextureIdle.
	texStreamed [maxTex]bool
	texLastUse  [maxTex]objc.ID
	// transparentTex is the 1x1 texture that a custom shader channel
	// reads when its surface has no plane for it.
	transparentTex objc.ID

	// Multisampling. sampleCount is 4 on every Mac GPU. SVG and other
	// triangle meshes have no antialiasing of their own, so without
	// samples their edges are hard pixel stairs (#823). msaaTex is the
	// multisampled color attachment of the main pass.
	sampleCount int
	msaaTex     objc.ID
	stencilTex  objc.ID
	stencilW    int
	stencilH    int

	// Filter. filterMsaaTex is the multisampled attachment of the filter
	// content pass. It resolves into filterTexA.
	filterMsaaTex, filterTexA, filterTexB, filterStencilTex objc.ID
	filterW, filterH                                        int
	// filterActive records a successful beginFilter. A failed begin
	// leaves the main encoder open, which endFilter must not end.
	filterActive bool
	// filterScissor saves the main-pass scissor across a filter.
	// Suspending the main pass drops the bound scissor, and the resumed
	// pass starts unclipped, so endFilter binds the saved rect again.
	filterScissor [4]uint
	filterScissOK bool

	stencilIncr, stencilTest, stencilDecr, stencilOff objc.ID

	customPipelines [maxCustomPipelines]objc.ID
	freeCustom      [maxCustomPipelines]int32
	freeCustomCount int
	nextCustomID    int

	// Vertex buffers: a list of chunks for each slot of the frame ring.
	// slot is the slot of the frame being drawn. chunk and chunkOff are
	// the write position in it. slotCmdBuf holds, retained, the last
	// command buffer that read the slot.
	chunks     [frameRing][]vchunk
	slot       int
	chunk      int
	chunkOff   uintptr
	slotCmdBuf [frameRing]objc.ID

	// The open batch: n quads of stride bytes, from byte start of buf.
	batchBuf    objc.ID
	batchStart  uintptr
	batchN      int
	batchStride uintptr

	// clipEmpty suppresses draws while the clip is empty. An empty clip
	// paints nothing, so setScissor binds no scissor and every draw below
	// is a no-op until the next nonempty clip. It is context state, not
	// encoder state: a fresh encoder binds no scissor either, so the two
	// agree across suspend and resume.
	clipEmpty bool

	st encState

	// Scratch for arguments that go by pointer. The bound values of the
	// cached setters are kept here too.
	mvp, tm, frag0, scratch [16]float32
	quadVerts               [4 * 9]float32
	// probeVerts holds the edge coverage probe triangle. Its address
	// reaches Metal as a uintptr, which does not keep a stack local in
	// place, so it lives on the heap with the rest of the context.
	probeVerts [27]float32
	viewport   [6]float64
	scissor    [4]uint
	region     [6]uint
	errOut     objc.ID
}

// newMetalCtx makes the renderer for layer. src is the MSL source of
// the built-in shaders (gui/backend/internal/msl). It returns nil and
// logs the cause when Metal or the shaders fail.
func newMetalCtx(layer objc.ID, src string) *metalCtx {
	loadRenderer()
	pool := darwinffi.PoolPush()
	defer darwinffi.PoolPop(pool)

	c := &metalCtx{nextTexID: 1, layer: layer, stats: rstats.FromEnv("metal")}
	c.device = objc.ID(msgCall(mtlCreateDevice))
	if c.device == 0 {
		log.Print("metal: no Metal device")
		return nil
	}
	if layer != 0 {
		msg(layer, rsel.setDevice, uintptr(c.device))
		msg(layer, rsel.setPixelFormat, mtlPixelFormatBGRA8Unorm)
		msg(layer, rsel.setFramebufferOnly, 1)
		// Asynchronous presentation: the frame goes to Metal and the main
		// thread goes back to the event loop at once. With
		// presentsWithTransaction the thread waits in waitUntilScheduled
		// until the compositor takes the frame. Measured through go-term,
		// a repaint queued from another goroutine then waited a median
		// 6.2 ms before the loop could take it. Async presentation cut
		// that to 2.4 ms and keystroke latency by 23%.
		//
		// Live resize still needs transaction mode, which stops content
		// moving in the window during the drag. The live resize handlers
		// turn it on for the gesture, and endFrame reads the layer each
		// frame, so the two modes cannot disagree.
		msg(layer, rsel.setPresentsWithTransaction, 0)
	}
	c.queue = objc.ID(msg(c.device, rsel.newCommandQueue))

	lib := c.compileLibrary(src, true)
	if lib == 0 {
		c.destroy()
		return nil
	}
	defer release(lib)

	// 4x multisampling when the GPU has it, which every Mac GPU does.
	// The blur and color matrix passes run on single-sample filter
	// textures and stay at 1.
	c.sampleCount = 1
	if msg(c.device, rsel.supportsTextureSampleCount, 4)&0xff != 0 {
		c.sampleCount = 4
	}
	ms := c.sampleCount
	mvd := mainVertexDesc()
	defer release(mvd)
	gvd := glyphVertexDesc()
	defer release(gvd)

	type pdesc struct {
		vs, fs  string
		vd      objc.ID
		samples int
		kind    int
	}
	const (
		blend = iota
		replace
		stencilMask
		// replaceNoStencil writes through like replace, but its pass
		// has no stencil attachment: the blur and color filter passes.
		replaceNoStencil
	)
	for i, p := range [pipeCount]pdesc{
		pipeSolid:       {"vs_solid", "fs_solid", mvd, ms, blend},
		pipeShadow:      {"vs_shadow", "fs_shadow", mvd, ms, blend},
		pipeBlur:        {"vs_blur", "fs_blur", mvd, ms, blend},
		pipeGradient:    {"vs_gradient", "fs_gradient", mvd, ms, blend},
		pipeImageClip:   {"vs_solid", "fs_image_clip", mvd, ms, blend},
		pipeFilterBlurH: {"vs_filter", "fs_filter_blur_h", mvd, 1, replaceNoStencil},
		pipeFilterBlurV: {"vs_filter", "fs_filter_blur_v", mvd, 1, replaceNoStencil},
		pipeFilterTex:   {"vs_filter", "fs_filter_tex", mvd, ms, blend},
		pipeFilterColor: {"vs_filter", "fs_filter_color", mvd, 1, replaceNoStencil},
		pipeGlyphTex:    {"vs_glyph", "fs_glyph_tex", gvd, ms, blend},
		pipeGlyphColor:  {"vs_glyph", "fs_glyph_color", gvd, ms, blend},
		pipeStencil:     {"vs_solid", "fs_stencil", mvd, ms, stencilMask},
	} {
		c.pipelines[i] = c.makePipeline(lib, p.vs, p.fs, p.vd, p.samples, p.kind)
		if c.pipelines[i] == 0 {
			c.destroy()
			return nil
		}
	}

	// Depth stencil states for the stencil clip.
	c.stencilIncr = c.stencilState(mtlCompareFunctionAlways, mtlStencilOperationIncrementClamp)
	c.stencilTest = c.stencilState(mtlCompareFunctionLessEqual, mtlStencilOperationKeep)
	c.stencilDecr = c.stencilState(mtlCompareFunctionAlways, mtlStencilOperationDecrementClamp)
	dsd := newObj(rcls.depthStencilDesc)
	c.stencilOff = objc.ID(msg(c.device, rsel.newDepthStencilStateWithDescriptor, uintptr(dsd)))
	release(dsd)

	// Index buffer for a run of quads: two triangles {0,1,2, 0,2,3} per
	// quad, offset by 4 for each quad.
	idx := make([]uint16, maxBatchQuads*6)
	for q := range maxBatchQuads {
		b := uint16(q * 4)
		copy(idx[q*6:], []uint16{b, b + 1, b + 2, b, b + 2, b + 3})
	}
	c.quadIdx = objc.ID(msg(c.device, rsel.newBufferWithBytes,
		uintptr(unsafe.Pointer(&idx[0])), uintptr(len(idx)*2), mtlResourceStorageModeShared))
	// newBufferWithBytes copies the bytes synchronously, but the call
	// takes only a uintptr: keep the slice live until it returns.
	runtime.KeepAlive(idx)

	// Shared samplers: linear and nearest, both clamp-to-edge.
	sd := newObj(rcls.samplerDesc)
	msg(sd, rsel.setMinFilter, mtlSamplerMinMagFilterLinear)
	msg(sd, rsel.setMagFilter, mtlSamplerMinMagFilterLinear)
	msg(sd, rsel.setSAddressMode, mtlSamplerAddressModeClampToEdge)
	msg(sd, rsel.setTAddressMode, mtlSamplerAddressModeClampToEdge)
	c.sampler = objc.ID(msg(c.device, rsel.newSamplerStateDesc, uintptr(sd)))
	// Same clamp, nearest texel: pixel art keeps hard edges.
	msg(sd, rsel.setMinFilter, mtlSamplerMinMagFilterNearest)
	msg(sd, rsel.setMagFilter, mtlSamplerMinMagFilterNearest)
	c.samplerNearest = objc.ID(msg(c.device, rsel.newSamplerStateDesc, uintptr(sd)))
	release(sd)

	if c.stencilIncr == 0 || c.stencilTest == 0 || c.stencilDecr == 0 ||
		c.stencilOff == 0 || c.quadIdx == 0 || c.sampler == 0 || c.samplerNearest == 0 {
		log.Print("metal: could not make the renderer state objects")
		c.destroy()
		return nil
	}
	return c
}

// compileLibrary compiles src into a library, retained, or returns 0
// and logs the error. pinned uses the pinned language version.
//
// The pin is the point for the built-in library. With no options the
// accepted MSL tracks the OS the build ran on, so a construct that needs
// a newer Metal compiles on a current Mac and fails on every older one,
// which is how issue 126 shipped. 3.0 is macOS 13 Ventura. Raising the
// pin is a decision to drop macOS versions: if a shader change trips it,
// change the shader.
//
// A custom shader is app source, so its OS floor is the app's decision:
// it compiles with no options. Call inside an autorelease pool.
func (c *metalCtx) compileLibrary(src string, pinned bool) objc.ID {
	nsSrc := darwinffi.NSString(src)
	if nsSrc == 0 {
		log.Print("metal: shader source is not valid UTF-8")
		return 0
	}
	var opts objc.ID
	if pinned {
		opts = newObj(rcls.compileOptions)
		msg(opts, rsel.setLanguageVersion, mtlLanguageVersion3_0)
		defer release(opts)
	}
	c.errOut = 0
	lib := objc.ID(msg(c.device, rsel.newLibraryWithSourceOptionsError,
		uintptr(nsSrc), uintptr(opts), ptr(&c.errOut)))
	if lib == 0 {
		log.Printf("metal: compile shaders: %s", nsErrorText(c.errOut))
	}
	return lib
}

// mainVertexDesc returns the layout of gpu.Vertex, retained: float3
// position, float2 texcoord, float4 color.
func mainVertexDesc() objc.ID {
	return vertexDesc(mainStride, [3][2]uintptr{
		{mtlVertexFormatFloat3, 0}, {mtlVertexFormatFloat2, 12}, {mtlVertexFormatFloat4, 20},
	})
}

// glyphVertexDesc returns the layout of the glyph vertex, retained:
// float2 position, float2 texcoord, float4 color.
func glyphVertexDesc() objc.ID {
	return vertexDesc(glyphStride, [3][2]uintptr{
		{mtlVertexFormatFloat2, 0}, {mtlVertexFormatFloat2, 8}, {mtlVertexFormatFloat4, 16},
	})
}

// vertexDesc returns a vertex descriptor with three attributes, each a
// {format, offset} in buffer 0, and the given stride.
func vertexDesc(stride uintptr, attrs [3][2]uintptr) objc.ID {
	d := newObj(rcls.vertexDesc)
	as := objc.ID(msg(d, rsel.attributes))
	for i, a := range attrs {
		attr := at(as, i)
		msg(attr, rsel.setFormat, a[0])
		msg(attr, rsel.setOffset, a[1])
		msg(attr, rsel.setBufferIndex, 0)
	}
	l := at(objc.ID(msg(d, rsel.layouts)), 0)
	msg(l, rsel.setStride, stride)
	msg(l, rsel.setStepFunction, mtlVertexStepFunctionPerVertex)
	return d
}

// makePipeline builds a render pipeline, retained, or returns 0 and
// logs why. kind 0 blends source-over, 1 writes through with no blend
// (the full-screen filter passes, where blending would apply alpha
// twice), 2 writes no color (the stencil mask), 3 writes through with
// no stencil format (the blur and color filter passes, whose pass
// descriptor has no stencil attachment). A pipeline's sample count must
// equal the sample count of the pass it draws in, and its stencil
// format must equal the format of the pass's stencil attachment, or
// Metal rejects the draw with validation on.
func (c *metalCtx) makePipeline(lib objc.ID, vs, fs string, vd objc.ID, samples, kind int) objc.ID {
	vf := objc.ID(msg(lib, rsel.newFunctionWithName, uintptr(darwinffi.NSString(vs))))
	defer release(vf)
	ff := objc.ID(msg(lib, rsel.newFunctionWithName, uintptr(darwinffi.NSString(fs))))
	defer release(ff)
	if vf == 0 || ff == 0 {
		log.Printf("metal: shader function %s or %s not found", vs, fs)
		return 0
	}
	desc := newObj(rcls.pipelineDesc)
	defer release(desc)
	msg(desc, rsel.setRasterSampleCount, uintptr(samples))
	msg(desc, rsel.setVertexFunction, uintptr(vf))
	msg(desc, rsel.setFragmentFunction, uintptr(ff))
	msg(desc, rsel.setVertexDescriptor, uintptr(vd))
	ca := at(objc.ID(msg(desc, rsel.colorAttachments)), 0)
	msg(ca, rsel.setPixelFormat, mtlPixelFormatBGRA8Unorm)
	switch kind {
	case 0:
		setSourceOver(ca)
	case 2:
		msg(ca, rsel.setWriteMask, mtlColorWriteMaskNone)
	}
	// Kind 3 (replaceNoStencil) draws without stencil: its pass has no
	// stencil attachment, and a pipeline that declares a stencil format
	// the pass lacks is rejected with validation on.
	if kind != 3 {
		msg(desc, rsel.setStencilAttachmentPixelFormat, mtlPixelFormatStencil8)
	}
	c.errOut = 0
	pso := objc.ID(msg(c.device, rsel.newRenderPipelineStateWithDescriptorError,
		uintptr(desc), ptr(&c.errOut)))
	if pso == 0 {
		log.Printf("metal: pipeline %s/%s: %s", vs, fs, nsErrorText(c.errOut))
	}
	return pso
}

// setSourceOver turns on source-over blending for a color attachment.
func setSourceOver(ca objc.ID) {
	msg(ca, rsel.setBlendingEnabled, 1)
	msg(ca, rsel.setSourceRGBBlendFactor, mtlBlendFactorSourceAlpha)
	msg(ca, rsel.setDestinationRGBBlendFactor, mtlBlendFactorOneMinusSourceAlpha)
	msg(ca, rsel.setSourceAlphaBlendFactor, mtlBlendFactorSourceAlpha)
	msg(ca, rsel.setDestinationAlphaBlendFactor, mtlBlendFactorOneMinusSourceAlpha)
}

// stencilState returns a depth stencil state, retained, that compares
// with cmp and applies pass where the fragment passes, on both faces.
func (c *metalCtx) stencilState(cmp, pass uintptr) objc.ID {
	d := newObj(rcls.depthStencilDesc)
	defer release(d)
	s := objc.ID(msg(d, rsel.frontFaceStencil))
	msg(s, rsel.setStencilCompareFunction, cmp)
	msg(s, rsel.setStencilFailureOperation, mtlStencilOperationKeep)
	msg(s, rsel.setDepthFailureOperation, mtlStencilOperationKeep)
	msg(s, rsel.setDepthStencilPassOperation, pass)
	msg(d, rsel.setBackFaceStencil, uintptr(s))
	return objc.ID(msg(c.device, rsel.newDepthStencilStateWithDescriptor, uintptr(d)))
}

// destroy releases every object of the renderer. A nil c is a no-op.
func (c *metalCtx) destroy() {
	if c == nil {
		return
	}
	if c.framePool != 0 {
		msgCall(darwinffi.PoolPopAddr(), c.framePool)
		c.framePool = 0
	}
	for i := range c.textures {
		release(c.textures[i])
		release(c.texLastUse[i])
		c.textures[i], c.texLastUse[i], c.texStreamed[i] = 0, 0, false
	}
	for i := range c.chunks {
		for _, ch := range c.chunks[i] {
			release(ch.buf)
		}
		c.chunks[i] = nil
		release(c.slotCmdBuf[i])
		c.slotCmdBuf[i] = 0
	}
	for _, o := range []*objc.ID{
		&c.transparentTex, &c.msaaTex, &c.stencilTex, &c.filterMsaaTex,
		&c.filterTexA, &c.filterTexB, &c.filterStencilTex,
		&c.stencilIncr, &c.stencilTest, &c.stencilDecr, &c.stencilOff,
		&c.quadIdx, &c.sampler, &c.samplerNearest, &c.queue, &c.device,
	} {
		release(*o)
		*o = 0
	}
	for i := range c.pipelines {
		release(c.pipelines[i])
		c.pipelines[i] = 0
	}
	for i := range c.customPipelines {
		release(c.customPipelines[i])
		c.customPipelines[i] = 0
	}
	c.nextTexID, c.freeTexCount = 1, 0
	c.nextCustomID, c.freeCustomCount = 0, 0
	c.drawable, c.target, c.cmdBuf, c.enc, c.layer = 0, 0, 0, 0, 0
}

// resize sets the size of the view and the drawable in pixels.
func (c *metalCtx) resize(w, h int) {
	c.viewW, c.viewH = w, h
	if c.layer == 0 {
		return
	}
	// CGSize is two doubles, a float aggregate: D0 and D1.
	mfc.D[0], mfc.D[1] = float64(w), float64(h)
	msg(c.layer, rsel.setDrawableSize)
}

// ─── Textures for attachments ─────────────────────────────────

// newTexture returns a 2D texture, retained, or 0. A sample count above
// 1 makes a multisampled texture, which can only be drawn into and
// resolved, never sampled.
func (c *metalCtx) newTexture(w, h int, format, usage, storage uintptr, samples int) objc.ID {
	td := newObj(rcls.textureDesc)
	defer release(td)
	msg(td, rsel.setPixelFormat, format)
	msg(td, rsel.setWidth, uintptr(w))
	msg(td, rsel.setHeight, uintptr(h))
	if samples > 1 {
		msg(td, rsel.setTextureType, mtlTextureType2DMultisample)
		msg(td, rsel.setSampleCount, uintptr(samples))
	}
	msg(td, rsel.setUsage, usage)
	msg(td, rsel.setStorageMode, storage)
	return objc.ID(msg(c.device, rsel.newTextureWithDescriptor, uintptr(td)))
}

// sampledTexture returns a shared texture that shaders sample.
func (c *metalCtx) sampledTexture(w, h int, format uintptr) objc.ID {
	return c.newTexture(w, h, format, mtlTextureUsageShaderRead, mtlStorageModeShared, 1)
}

// setTex releases the texture in *slot and stores t there.
func setTex(slot *objc.ID, t objc.ID) {
	release(*slot)
	*slot = t
}

// ensureMainAttachments sizes the multisampled color texture and the
// stencil texture to the resolve target. Every attachment of a pass must
// have one size, so they follow the target texture, not viewW and viewH,
// which can lag the drawable during a live resize.
func (c *metalCtx) ensureMainAttachments(w, h int) {
	// A missing texture (a failed allocation) is tried again on the next
	// pass, not cached. With msaaTex nil the pass would be single-sample
	// while every pipeline is multisampled, and Metal would reject each
	// draw for the rest of the window's life.
	if c.stencilTex != 0 && (c.sampleCount == 1 || c.msaaTex != 0) &&
		c.stencilW == w && c.stencilH == h {
		return
	}
	setTex(&c.stencilTex, c.newTexture(w, h, mtlPixelFormatStencil8,
		mtlTextureUsageRenderTarget, mtlStorageModePrivate, c.sampleCount))
	setTex(&c.msaaTex, 0)
	if c.sampleCount > 1 {
		c.msaaTex = c.newTexture(w, h, mtlPixelFormatBGRA8Unorm,
			mtlTextureUsageRenderTarget, mtlStorageModePrivate, c.sampleCount)
	}
	c.stencilW, c.stencilH = w, h
}

// ─── Encoders ─────────────────────────────────────────────────

// newEncoder starts a render pass from rpd, sets the viewport to w x h
// and binds the linear sampler. It forgets the bound state of the last
// encoder.
func (c *metalCtx) newEncoder(rpd objc.ID, w, h int) objc.ID {
	enc := objc.ID(msg(c.cmdBuf, rsel.renderCommandEncoderWithDescriptor, uintptr(rpd)))
	if enc == 0 {
		return 0
	}
	c.viewport = [6]float64{0, 0, float64(w), float64(h), 0, 1}
	// MTLViewport is six doubles: more than four, so not a float
	// aggregate. A struct over 16 bytes goes by pointer.
	msg(enc, rsel.setViewport, ptr(&c.viewport))
	msg(enc, rsel.setFragmentSamplerStateAtIndex, uintptr(c.sampler), 0)
	return enc
}

// beginMainEncoder starts or resumes the main render pass.
//
// The store actions stay open (MTLStoreActionUnknown) until
// endMainEncoder, because only the caller that ends the pass knows if it
// resumes. A pass suspended for a filter must keep its samples and its
// stencil, so the resumed pass can load them. The last pass of the frame
// needs only the resolved color, so it resolves and drops the samples,
// which on Apple GPUs are then never written to memory.
func (c *metalCtx) beginMainEncoder(r, g, b, a float64, clear bool) {
	rpd := newObj(rcls.renderPassDesc)
	defer release(rpd)
	w := int(msg(c.target, rsel.width))
	h := int(msg(c.target, rsel.height))
	c.ensureMainAttachments(w, h)
	ca := at(objc.ID(msg(rpd, rsel.colorAttachments)), 0)
	if c.msaaTex != 0 {
		msg(ca, rsel.setTexture, uintptr(c.msaaTex))
		msg(ca, rsel.setResolveTexture, uintptr(c.target))
	} else {
		msg(ca, rsel.setTexture, uintptr(c.target))
	}
	msg(ca, rsel.setStoreAction, mtlStoreActionUnknown)
	if clear {
		msg(ca, rsel.setLoadAction, mtlLoadActionClear)
		msgClearColor(ca, r, g, b, a)
	} else {
		msg(ca, rsel.setLoadAction, mtlLoadActionLoad)
	}
	if c.stencilTex != 0 {
		sa := objc.ID(msg(rpd, rsel.stencilAttachment))
		msg(sa, rsel.setTexture, uintptr(c.stencilTex))
		msg(sa, rsel.setStoreAction, mtlStoreActionUnknown)
		if clear {
			msg(sa, rsel.setLoadAction, mtlLoadActionClear)
			msg(sa, rsel.setClearStencil, 0)
		} else {
			msg(sa, rsel.setLoadAction, mtlLoadActionLoad)
		}
	}
	c.enc = c.newEncoder(rpd, c.viewW, c.viewH)
	c.st = encState{}
}

// endMainEncoder flushes the batch, sets the store actions that
// beginMainEncoder left open, and ends the pass. resume is true when a
// later pass of this frame loads the attachments again (a filter
// suspends the main pass), false at the end of the frame.
func (c *metalCtx) endMainEncoder(resume bool) {
	if c.enc == 0 {
		return
	}
	c.flush(rstats.CausePass)
	var color uintptr = mtlStoreActionStore
	if c.msaaTex != 0 {
		// A pass with a resolve texture must resolve, even when it is
		// suspended. A suspended pass also keeps its samples for the
		// resumed pass. Only frames with a filter pay for that store.
		color = mtlStoreActionMultisampleResolve
		if resume {
			color = mtlStoreActionStoreAndMultisampleResolve
		}
	}
	msg(c.enc, rsel.setColorStoreAction, color, 0)
	if c.stencilTex != 0 {
		var st uintptr = mtlStoreActionDontCare
		if resume {
			st = mtlStoreActionStore
		}
		msg(c.enc, rsel.setStencilStoreAction, st)
	}
	msg(c.enc, rsel.endEncoding)
	c.enc = 0
	c.st = encState{}
}

// ─── Frame ────────────────────────────────────────────────────

// beginFrame opens the frame pool, takes the next drawable and starts
// the main pass cleared to the color. It returns false when there is no
// drawable (a hidden window), and then nothing is drawn.
func (c *metalCtx) beginFrame(r, g, b, a float32) bool {
	c.framePool = msgCall(darwinffi.PoolPushAddr())
	c.drawable = objc.ID(msg(c.layer, rsel.nextDrawable))
	if c.drawable == 0 {
		msgCall(darwinffi.PoolPopAddr(), c.framePool)
		c.framePool = 0
		return false
	}
	c.target = objc.ID(msg(c.drawable, rsel.texture))

	// Take the next slot of vertex buffers. Wait until the GPU is done
	// with the last frame that used it: with 3 drawables it always is, so
	// the wait returns at once.
	c.slot = (c.slot + 1) % frameRing
	if cb := c.slotCmdBuf[c.slot]; cb != 0 {
		msg(cb, rsel.waitUntilCompleted)
		if c.stats != nil {
			// The command buffer is complete, so its GPU times are final.
			// They are seconds on the host clock; the frame's GPU time is
			// their difference. Read them here, frameRing frames late,
			// so that measuring adds no wait.
			start := msgF64(cb, rsel.gpuStartTime)
			end := msgF64(cb, rsel.gpuEndTime)
			if end > start {
				c.stats.GPU(time.Duration((end - start) * float64(time.Second)))
			}
		}
		release(cb)
		c.slotCmdBuf[c.slot] = 0
	}
	c.chunk, c.chunkOff = 0, 0
	c.stats.BeginFrame()

	c.cmdBuf = objc.ID(msg(c.queue, rsel.commandBuffer))
	c.slotCmdBuf[c.slot] = retain(c.cmdBuf)
	c.beginMainEncoder(float64(r), float64(g), float64(b), float64(a), true)
	return true
}

// endFrame ends the main pass, presents the drawable and commits.
func (c *metalCtx) endFrame() {
	c.endMainEncoder(false)
	if c.drawable != 0 && c.cmdBuf != 0 {
		// presentsWithTransaction needs the app to present the drawable
		// itself, after the command buffer is scheduled, which is what
		// blocks the main thread. Without it, the command buffer presents
		// when the GPU is done. The layer is read each frame, so the
		// live resize switch takes effect on the next frame.
		if msg(c.layer, rsel.presentsWithTransaction)&0xff != 0 {
			msg(c.cmdBuf, rsel.commit)
			msg(c.cmdBuf, rsel.waitUntilScheduled)
			msg(c.drawable, rsel.present)
		} else {
			msg(c.cmdBuf, rsel.presentDrawable, uintptr(c.drawable))
			msg(c.cmdBuf, rsel.commit)
		}
	}
	c.drawable, c.target, c.cmdBuf = 0, 0, 0
	// Submit time ends after commit (and, with presentsWithTransaction,
	// after the wait until scheduled), so it is the CPU cost of the
	// whole frame's encode and hand-off.
	c.stats.EndFrame()
	if c.framePool != 0 {
		msgCall(darwinffi.PoolPopAddr(), c.framePool)
		c.framePool = 0
	}
}
