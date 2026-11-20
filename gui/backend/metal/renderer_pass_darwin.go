//go:build darwin && arm64 && !ios

package metal

// Custom shader pipelines, the filter and stencil passes, and the
// test probes of the renderer. See renderer_darwin.go.

import (
	"log"
	"unsafe"

	"github.com/ebitengine/purego/objc"

	"github.com/go-gui-org/go-gui/gui/internal/darwinffi"
)

// ─── Custom shader pipelines ──────────────────────────────────

// buildCustomPipeline compiles a custom shader and returns its pipeline
// index, or -1.
func (c *metalCtx) buildCustomPipeline(src string) int32 {
	if c == nil || c.device == 0 {
		return -1
	}
	pool := msgCall(darwinffi.PoolPushAddr())
	defer msgCall(darwinffi.PoolPopAddr(), pool)
	lib := c.compileLibrary(src, false)
	if lib == 0 {
		return -1
	}
	defer release(lib)
	mvd := mainVertexDesc()
	defer release(mvd)
	// Custom shaders draw in the main pass, so they match its sample
	// count and its stencil attachment (#1299).
	pso := c.makePipeline(lib, "vs_main", "fs_main", mvd, c.sampleCount, 0)
	if pso == 0 {
		return -1
	}
	var idx int
	switch {
	case c.freeCustomCount > 0:
		c.freeCustomCount--
		idx = int(c.freeCustom[c.freeCustomCount])
	case c.nextCustomID < maxCustomPipelines:
		idx = c.nextCustomID
		c.nextCustomID++
	default:
		log.Print("metal: custom pipeline cache is full")
		release(pso)
		return -1
	}
	c.customPipelines[idx] = pso
	return int32(idx)
}

// deleteCustomPipeline releases custom pipeline idx.
func (c *metalCtx) deleteCustomPipeline(idx int32) {
	if c == nil || idx < 0 || idx >= maxCustomPipelines || c.customPipelines[idx] == 0 {
		return
	}
	if c.st.pipe == c.customPipelines[idx] {
		c.flush()
	}
	setTex(&c.customPipelines[idx], 0)
	if c.freeCustomCount < maxCustomPipelines {
		c.freeCustom[c.freeCustomCount] = idx
		c.freeCustomCount++
	}
}

// setCustomPipeline binds custom pipeline idx.
func (c *metalCtx) setCustomPipeline(idx int32) {
	if idx < 0 || idx >= maxCustomPipelines {
		return
	}
	c.setPipelineObj(c.customPipelines[idx])
}

// ─── Filter (glow) ────────────────────────────────────────────

// ensureFilterTextures sizes the filter textures to w x h.
func (c *metalCtx) ensureFilterTextures(w, h int) {
	if c.filterTexA != 0 && c.filterW == w && c.filterH == h {
		return
	}
	const rt = mtlTextureUsageShaderRead | mtlTextureUsageRenderTarget
	setTex(&c.filterTexA, c.newTexture(w, h, mtlPixelFormatBGRA8Unorm, rt, mtlStorageModePrivate, 1))
	setTex(&c.filterTexB, c.newTexture(w, h, mtlPixelFormatBGRA8Unorm, rt, mtlStorageModePrivate, 1))
	// The filter content pass draws with the main pipelines, so it is
	// multisampled like the main pass and resolves into filterTexA. The
	// blur and color passes read filterTexA.
	setTex(&c.filterMsaaTex, 0)
	if c.sampleCount > 1 {
		c.filterMsaaTex = c.newTexture(w, h, mtlPixelFormatBGRA8Unorm,
			mtlTextureUsageRenderTarget, mtlStorageModePrivate, c.sampleCount)
	}
	// A stencil attachment, so ClipContents works inside filters.
	setTex(&c.filterStencilTex, c.newTexture(w, h, mtlPixelFormatStencil8,
		mtlTextureUsageRenderTarget, mtlStorageModePrivate, c.sampleCount))
	c.filterW, c.filterH = w, h
}

// beginFilter suspends the main pass and starts the filter content
// pass, w x h. It returns 0, -1 with no frame, or -2 when the filter
// textures could not be made.
func (c *metalCtx) beginFilter(w, h int) int {
	if c == nil || c.enc == 0 || c.cmdBuf == 0 {
		return -1
	}
	c.ensureFilterTextures(w, h)
	if c.filterTexA == 0 || c.filterTexB == 0 || (c.sampleCount > 1 && c.filterMsaaTex == 0) {
		return -2
	}
	c.endMainEncoder(true) // endFilter resumes it

	// The pass is never resumed, so its samples resolve and are dropped.
	rpd := newObj(rcls.renderPassDesc)
	defer release(rpd)
	ca := at(objc.ID(msg(rpd, rsel.colorAttachments)), 0)
	if c.filterMsaaTex != 0 {
		msg(ca, rsel.setTexture, uintptr(c.filterMsaaTex))
		msg(ca, rsel.setResolveTexture, uintptr(c.filterTexA))
		msg(ca, rsel.setStoreAction, mtlStoreActionMultisampleResolve)
	} else {
		msg(ca, rsel.setTexture, uintptr(c.filterTexA))
		msg(ca, rsel.setStoreAction, mtlStoreActionStore)
	}
	msg(ca, rsel.setLoadAction, mtlLoadActionClear)
	msgClearColor(ca, 0, 0, 0, 0)
	// The stencil is cleared on each begin, so nothing is stored.
	if c.filterStencilTex != 0 {
		sa := objc.ID(msg(rpd, rsel.stencilAttachment))
		msg(sa, rsel.setTexture, uintptr(c.filterStencilTex))
		msg(sa, rsel.setLoadAction, mtlLoadActionClear)
		msg(sa, rsel.setStoreAction, mtlStoreActionDontCare)
		msg(sa, rsel.setClearStencil, 0)
	}
	c.enc = c.newEncoder(rpd, w, h)
	c.st = encState{}
	return 0
}

// orthoInto writes the pixel-space orthographic MVP for w x h to m.
func orthoInto(m *[16]float32, w, h int) {
	*m = [16]float32{}
	m[0] = 2 / float32(w)
	m[5] = -2 / float32(h)
	m[10] = -1
	m[12] = -1
	m[13] = 1
	m[15] = 1
}

// setFullQuad writes the full-target quad for w x h to quadVerts.
// flipV puts v=1 at the top: the blur passes flip V, and two of them
// cancel out.
func (c *metalCtx) setFullQuad(w, h int, flipV bool) {
	fw, fh := float32(w), float32(h)
	v0, v1 := float32(0), float32(1)
	if flipV {
		v0, v1 = 1, 0
	}
	c.quadVerts = [36]float32{
		0, 0, 0, 0, v0, 1, 1, 1, 1,
		fw, 0, 0, 1, v0, 1, 1, 1, 1,
		fw, fh, 0, 1, v1, 1, 1, 1, 1,
		0, fh, 0, 0, v1, 1, 1, 1, 1,
	}
}

// filterPass runs one full-screen pass of pipe from src into dst, w x h.
// tm0 goes in the first value of the uniform matrix (the blur standard
// deviation), or the matrix is the identity when tm0 is 0. cm is the
// color matrix at fragment index 0, or nil.
func (c *metalCtx) filterPass(pipe int, src, dst objc.ID, w, h int, tm0 float32, cm *[16]float32, flipV bool) {
	rpd := newObj(rcls.renderPassDesc)
	defer release(rpd)
	ca := at(objc.ID(msg(rpd, rsel.colorAttachments)), 0)
	msg(ca, rsel.setTexture, uintptr(dst))
	msg(ca, rsel.setLoadAction, mtlLoadActionClear)
	msg(ca, rsel.setStoreAction, mtlStoreActionStore)
	msgClearColor(ca, 0, 0, 0, 0)

	enc := c.newEncoder(rpd, w, h)
	if enc == 0 {
		return
	}
	msg(enc, rsel.setRenderPipelineState, uintptr(c.pipelines[pipe]))
	msg(enc, rsel.setFragmentTexture, uintptr(src), 0)
	if cm != nil {
		c.frag0 = *cm
		msg(enc, rsel.setFragmentBytes, ptr(&c.frag0), 64, 0)
	}
	c.scratch = [16]float32{}
	if tm0 != 0 {
		c.scratch[0] = tm0
	} else {
		c.scratch[0], c.scratch[5], c.scratch[10], c.scratch[15] = 1, 1, 1, 1
	}
	msg(enc, rsel.setVertexBytes, ptr(&c.scratch), 64, 2)
	orthoInto(&c.scratch, w, h)
	msg(enc, rsel.setVertexBytes, ptr(&c.scratch), 64, 1)
	c.setFullQuad(w, h, flipV)
	msg(enc, rsel.setVertexBytes, ptr(&c.quadVerts), uintptr(len(c.quadVerts)*4), 0)
	msg(enc, rsel.drawIndexedPrimitives, mtlPrimitiveTypeTriangle, 6,
		mtlIndexTypeUInt16, uintptr(c.quadIdx), 0)
	msg(enc, rsel.endEncoding)
}

// endFilter ends the filter content pass, runs the blur and color
// matrix passes, resumes the main pass and draws the result layers
// times onto it. cm is the color matrix, or nil.
func (c *metalCtx) endFilter(blurRadius float32, layers int, cm *[16]float32) {
	if c == nil || c.enc == 0 || c.cmdBuf == 0 {
		return
	}
	layers = max(layers, 1)
	w, h := c.filterW, c.filterH
	c.flush()
	msg(c.enc, rsel.endEncoding)
	c.enc = 0

	// src is the texture that holds the result so far.
	src := c.filterTexA
	if blurRadius >= 1 {
		// Horizontal A to B, then vertical B to A.
		c.filterPass(pipeFilterBlurH, c.filterTexA, c.filterTexB, w, h, blurRadius, nil, true)
		c.filterPass(pipeFilterBlurV, c.filterTexB, c.filterTexA, w, h, blurRadius, nil, true)
	}
	if cm != nil {
		// Non-flipped UVs, so the composite reads an upright image
		// whether or not the blur ran.
		c.filterPass(pipeFilterColor, c.filterTexA, c.filterTexB, w, h, 0, cm, false)
		src = c.filterTexB
	}

	// Resume the main pass (load, not clear) and composite.
	c.beginMainEncoder(0, 0, 0, 0, false)
	if c.enc == 0 {
		return
	}
	c.setPipeline(pipeFilterTex)
	c.setFragmentTex(0, src, &c.st.tex0)
	var m [16]float32
	orthoInto(&m, c.viewW, c.viewH)
	c.setMVP(&m)
	m = [16]float32{0: 1, 5: 1, 10: 1, 15: 1}
	c.setTM(&m)
	// The composite uses non-flipped UVs: v=0 at the top of the screen
	// reads the top of the texture.
	c.setFullQuad(c.viewW, c.viewH, false)
	msg(c.enc, rsel.setVertexBytes, ptr(&c.quadVerts), uintptr(len(c.quadVerts)*4), 0)
	c.st.vbuf = 0
	for range layers {
		msg(c.enc, rsel.drawIndexedPrimitives, mtlPrimitiveTypeTriangle, 6,
			mtlIndexTypeUInt16, uintptr(c.quadIdx), 0)
	}
}

// ─── Stencil clip ─────────────────────────────────────────────

// stencilQuad draws the quad at v into the stencil with state ds.
func (c *metalCtx) stencilQuad(v unsafe.Pointer, ds objc.ID) {
	c.flush()
	msg(c.enc, rsel.setDepthStencilState, uintptr(ds))
	c.setPipeline(pipeStencil)
	c.drawInline(uintptr(v), 4*mainStride)
}

// beginStencilClip adds the SDF quad at v to the stencil (no color) and
// clips the following draws to stencil depth.
func (c *metalCtx) beginStencilClip(v unsafe.Pointer, depth int) {
	if c == nil || c.enc == 0 || v == nil {
		return
	}
	// Clamp to the stencil range. A depth below 1 would wrap to a large
	// reference value (children draw unclipped); above 255 is more than
	// IncrementClamp reaches.
	depth = min(max(depth, 1), 255)
	c.stencilQuad(v, c.stencilIncr)
	msg(c.enc, rsel.setDepthStencilState, uintptr(c.stencilTest))
	msg(c.enc, rsel.setStencilReferenceValue, uintptr(depth))
}

// endStencilClip takes the SDF quad at v out of the stencil and clips
// to the parent depth, or turns the stencil test off at depth 1.
func (c *metalCtx) endStencilClip(v unsafe.Pointer, depth int) {
	if c == nil || c.enc == 0 || v == nil {
		return
	}
	depth = min(max(depth, 1), 255)
	c.stencilQuad(v, c.stencilDecr)
	if depth <= 1 {
		msg(c.enc, rsel.setDepthStencilState, uintptr(c.stencilOff))
		return
	}
	msg(c.enc, rsel.setDepthStencilState, uintptr(c.stencilTest))
	msg(c.enc, rsel.setStencilReferenceValue, uintptr(depth-1))
}

// ─── Test probes ──────────────────────────────────────────────

// compileShadersProbe compiles src at the pinned language version, as
// newMetalCtx does, with no window. 0 = compiled, -1 = compile failed,
// 1 = no Metal device (a headless runner: the caller skips).
//
// It guards issue 126: a Metal 3.2-only construct compiled on a current
// Mac and failed on every older one. The real compile runs only when a
// window exists, which no CI job has, so this is where it is caught.
func compileShadersProbe(src string) int {
	loadRenderer()
	pool := darwinffi.PoolPush()
	defer darwinffi.PoolPop(pool)
	c := &metalCtx{}
	c.device = objc.ID(msgCall(mtlCreateDevice))
	if c.device == 0 {
		return 1
	}
	defer c.destroy()
	lib := c.compileLibrary(src, true)
	if lib == 0 {
		return -1
	}
	release(lib)
	return 0
}

// edgeCoverageProbe draws one white right triangle, its legs on the top
// and left edges of a black 16x16 target, through the context, pipeline
// and main pass that a frame uses. It counts the pixels on the
// hypotenuse that are neither black nor white. A triangle mesh (an SVG
// fill) has no antialiasing in the shader, so a partial pixel can only
// come from multisampling: 0 means the edge is a hard stair (#823).
//
// It returns the count, -1 with no Metal device, or -2 when setup fails.
func edgeCoverageProbe(src string) int {
	loadRenderer()
	pool := darwinffi.PoolPush()
	defer darwinffi.PoolPop(pool)
	if dev := objc.ID(msgCall(mtlCreateDevice)); dev == 0 {
		return -1
	} else {
		release(dev)
	}
	layer := objc.ID(msg(objc.ID(msg(rcls.metalLayer, sels.alloc)), sels.init))
	defer release(layer)
	c := newMetalCtx(layer, src)
	if c == nil {
		return -2
	}
	defer c.destroy()

	const n = 16
	// Shared storage, so the CPU reads the pixels back with getBytes:.
	// Apple GPUs have unified memory, and the probe waits for the GPU.
	tex := c.newTexture(n, n, mtlPixelFormatBGRA8Unorm, mtlTextureUsageRenderTarget, mtlStorageModeShared, 1)
	if tex == 0 {
		return -2
	}
	defer release(tex)
	c.viewW, c.viewH = n, n
	c.target = tex
	c.cmdBuf = objc.ID(msg(c.queue, rsel.commandBuffer))
	defer func() { c.target, c.cmdBuf = 0, 0 }()
	c.beginMainEncoder(0, 0, 0, 1, true)
	if c.enc == 0 {
		// With no encoder every draw is a no-op and the readback is all
		// black: report a setup failure, not "0 partial pixels".
		return -2
	}

	var mvp [16]float32
	orthoInto(&mvp, n, n)
	// x, y, params, u, v, r, g, b, a: the SVG vertex layout. On the heap,
	// because its address goes to Metal.
	verts := new([27]float32)
	*verts = [27]float32{
		0, 0, 0, 0, 0, 1, 1, 1, 1,
		n, 0, 0, 0, 0, 1, 1, 1, 1,
		0, n, 0, 0, 0, 1, 1, 1, 1,
	}
	c.setPipeline(pipeSolid)
	c.setMVP(&mvp)
	c.drawTriangles(unsafe.Pointer(verts), 3)
	c.endMainEncoder(false)
	msg(c.cmdBuf, rsel.commit)
	msg(c.cmdBuf, rsel.waitUntilCompleted)
	if st := msg(c.cmdBuf, rsel.status); st != mtlCommandBufferStatusCompleted {
		// A failed command buffer leaves the readback unwritten, which
		// would also read as "0 partial pixels".
		log.Printf("metal: edge probe: %s", nsErrorText(objc.ID(msg(c.cmdBuf, rsel.errorSel))))
		return -2
	}

	px := make([]byte, n*n*4)
	c.region = [6]uint{0, 0, 0, n, n, 1}
	msg(tex, rsel.getBytes, uintptr(unsafe.Pointer(&px[0])), n*4, ptr(&c.region), 0)
	partial := 0
	for i := range n * n {
		if g := px[i*4+1]; g > 0 && g < 255 {
			partial++
		}
	}
	return partial
}
