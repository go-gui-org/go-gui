//go:build darwin && arm64

package mtldraw

// The draw path of the renderer: the cached state setters, the quad
// batch and the textures. See renderer_darwin.go.

import (
	"unsafe"

	"github.com/ebitengine/purego/objc"
)

// ─── Cached state setters ─────────────────────────────────────

// setPipelineObj binds pso, unless it is bound.
func (c *metalCtx) setPipelineObj(pso objc.ID) {
	if c.enc == 0 || pso == 0 || c.st.pipe == pso {
		return
	}
	c.flush()
	msg(c.enc, rsel.setRenderPipelineState, uintptr(pso))
	c.st.pipe = pso
}

// setPipeline binds the built-in pipeline id.
func (c *metalCtx) setPipeline(id int) {
	if id < 0 || id >= pipeCount {
		return
	}
	c.setPipelineObj(c.pipelines[id])
}

// setMVP binds m at vertex index 1, unless it is bound.
func (c *metalCtx) setMVP(m *[16]float32) {
	if c.enc == 0 || (c.st.mvpOK && c.mvp == *m) {
		return
	}
	c.flush()
	c.mvp = *m
	msg(c.enc, rsel.setVertexBytes, ptr(&c.mvp), 64, 1)
	c.st.mvpOK = true
}

// setTM binds the per-draw uniform matrix m at vertex index 2, unless
// it is bound.
func (c *metalCtx) setTM(m *[16]float32) {
	if c.enc == 0 || (c.st.tmOK && c.tm == *m) {
		return
	}
	c.flush()
	c.tm = *m
	msg(c.enc, rsel.setVertexBytes, ptr(&c.tm), 64, 2)
	c.st.tmOK = true
}

// setFragment0 binds m at fragment index 0, unless it is bound. The
// gradient pipeline reads stops 4-7 there, and a custom shader its
// built-in block (#573). Each pipeline binds nothing else at that index.
func (c *metalCtx) setFragment0(m *[16]float32) {
	if c.enc == 0 || (c.st.frag0OK && c.frag0 == *m) {
		return
	}
	c.flush()
	c.frag0 = *m
	msg(c.enc, rsel.setFragmentBytes, ptr(&c.frag0), 64, 0)
	c.st.frag0OK = true
}

// setScissorRect binds the scissor rect, unless it is bound.
func (c *metalCtx) setScissorRect(x, y, w, h uint) {
	r := [4]uint{x, y, w, h}
	if c.st.scissOK && c.st.scissor == r {
		return
	}
	c.flush()
	c.scissor = r
	// MTLScissorRect is 32 bytes of integers: it goes by pointer.
	msg(c.enc, rsel.setScissorRect, ptr(&c.scissor))
	c.st.scissor, c.st.scissOK = r, true
}

// setScissor clips to the rect in pixels, clamped to the viewport.
// An empty clip paints nothing: rather than park a 1x1 scissor outside
// the viewport (which Metal validation rejects as out of bounds), it
// draws the open batch and suppresses every draw until the next
// nonempty clip.
func (c *metalCtx) setScissor(x, y, w, h int) {
	if c.enc == 0 {
		return
	}
	if x < 0 {
		w += x
		x = 0
	}
	if y < 0 {
		h += y
		y = 0
	}
	if w > 0 && h > 0 {
		w = min(w, c.viewW-x)
		h = min(h, c.viewH-y)
	}
	if w <= 0 || h <= 0 {
		c.flush()
		c.clipEmpty = true
		return
	}
	c.clipEmpty = false
	c.setScissorRect(uint(x), uint(y), uint(w), uint(h))
}

// setFragmentTex binds t at fragment texture index i, unless it is
// bound there. The cache compares objects, which is safe: the command
// buffer retains a bound texture, so its address is not reused while
// the encoder lives.
func (c *metalCtx) setFragmentTex(i int, t objc.ID, bound *objc.ID) {
	if *bound == t {
		return
	}
	c.flush()
	msg(c.enc, rsel.setFragmentTexture, uintptr(t), uintptr(i))
	*bound = t
}

// bindTexture binds texture id at fragment index 0.
func (c *metalCtx) bindTexture(id int) {
	if c.enc == 0 || id <= 0 || id >= maxTex || c.textures[id] == 0 {
		return
	}
	c.setFragmentTex(0, c.textures[id], &c.st.tex0)
	c.noteTextureUse(id)
}

// setImageFilter binds the nearest sampler when nearest is true, and
// the linear one otherwise.
func (c *metalCtx) setImageFilter(nearest bool) {
	if c.enc == 0 {
		return
	}
	s := c.sampler
	if nearest {
		s = c.samplerNearest
	}
	// A new encoder binds the linear sampler (newEncoder).
	bound := c.st.sampler
	if bound == 0 {
		bound = c.sampler
	}
	if bound == s {
		return
	}
	c.flush()
	msg(c.enc, rsel.setFragmentSamplerStateAtIndex, uintptr(s), 0)
	c.st.sampler = s
}

// ─── Batched drawing ──────────────────────────────────────────

// reserve returns the address and buffer offset of size free bytes in
// the vertex buffers of this frame, aligned to 16. It takes a new chunk
// when the current one is full. ok is false when the frame has used
// every chunk, or when size is more than a chunk.
func (c *metalCtx) reserve(size uintptr) (ch *vchunk, off uintptr, ok bool) {
	if size > vchunkBytes {
		return nil, 0, false
	}
	list := &c.chunks[c.slot]
	for {
		if c.chunk < len(*list) {
			off = (c.chunkOff + 15) &^ 15
			if off+size <= vchunkBytes {
				c.chunkOff = off + size
				return &(*list)[c.chunk], off, true
			}
			c.chunk++
			c.chunkOff = 0
			continue
		}
		if len(*list) >= maxChunksPerFrame {
			return nil, 0, false
		}
		buf := objc.ID(msg(c.device, rsel.newBufferWithLength, vchunkBytes, mtlResourceStorageModeShared))
		if buf == 0 {
			return nil, 0, false
		}
		*list = append(*list, vchunk{buf: buf, addr: msg(buf, rsel.contents)})
	}
}

// drawQuad adds one quad of stride-byte vertices at v to the batch.
func (c *metalCtx) drawQuad(v unsafe.Pointer, stride uintptr) {
	if c.enc == 0 || c.clipEmpty {
		return
	}
	size := 4 * stride
	if c.batchN > 0 && (c.batchStride != stride || c.batchN == maxBatchQuads) {
		c.flush()
	}
	ch, off, ok := c.reserve(size)
	if !ok {
		// Out of vertex buffers for this frame: draw this quad alone.
		c.flush()
		c.drawInline(uintptr(v), size)
		return
	}
	if c.batchN > 0 && (ch.buf != c.batchBuf || off != c.batchStart+uintptr(c.batchN)*size) {
		c.flush() // the quad starts a new chunk
	}
	if c.batchN == 0 {
		c.batchBuf, c.batchStart, c.batchStride = ch.buf, off, stride
	}
	copy(bytesAt(ch.addr+off, int(size)), unsafe.Slice((*byte)(v), size))
	c.batchN++
}

// drawInline draws one quad from size bytes at v with setVertexBytes.
func (c *metalCtx) drawInline(v, size uintptr) {
	msg(c.enc, rsel.setVertexBytes, v, size, 0)
	c.st.vbuf = 0
	msg(c.enc, rsel.drawIndexedPrimitives, mtlPrimitiveTypeTriangle, 6,
		mtlIndexTypeUInt16, uintptr(c.quadIdx), 0)
}

// flush draws the open batch with one call.
func (c *metalCtx) flush() {
	if c.batchN == 0 {
		return
	}
	n := c.batchN
	c.batchN = 0
	if c.st.vbuf == c.batchBuf {
		msg(c.enc, rsel.setVertexBufferOffset, c.batchStart, 0)
	} else {
		msg(c.enc, rsel.setVertexBuffer, uintptr(c.batchBuf), c.batchStart, 0)
		c.st.vbuf = c.batchBuf
	}
	msg(c.enc, rsel.drawIndexedPrimitives, mtlPrimitiveTypeTriangle, uintptr(6*n),
		mtlIndexTypeUInt16, uintptr(c.quadIdx), 0)
}

// drawMainQuad batches one quad of gpu.Vertex values.
func (c *metalCtx) drawMainQuad(v unsafe.Pointer) { c.drawQuad(v, mainStride) }

// drawGlyphQuad batches one quad of glyph vertices.
func (c *metalCtx) drawGlyphQuad(v unsafe.Pointer) { c.drawQuad(v, glyphStride) }

// drawTriangles draws numVerts gpu.Vertex values at v as a triangle list.
func (c *metalCtx) drawTriangles(v unsafe.Pointer, numVerts int) {
	if c.enc == 0 || c.clipEmpty || numVerts <= 0 {
		return
	}
	c.flush()
	size := uintptr(numVerts) * mainStride
	switch {
	case size <= maxInlineBytes:
		msg(c.enc, rsel.setVertexBytes, uintptr(v), size, 0)
		c.st.vbuf = 0
	default:
		if ch, off, ok := c.reserve(size); ok {
			copy(bytesAt(ch.addr+off, int(size)), unsafe.Slice((*byte)(v), size))
			msg(c.enc, rsel.setVertexBuffer, uintptr(ch.buf), off, 0)
			c.st.vbuf = ch.buf
			break
		}
		// More than a chunk, or no chunk left: a buffer of its own. The
		// command buffer retains it, so it can be released at once.
		buf := objc.ID(msg(c.device, rsel.newBufferWithBytes, uintptr(v), size,
			mtlResourceStorageModeShared))
		if buf == 0 {
			return
		}
		msg(c.enc, rsel.setVertexBuffer, uintptr(buf), 0, 0)
		release(buf)
		c.st.vbuf = 0
	}
	msg(c.enc, rsel.drawPrimitives, mtlPrimitiveTypeTriangle, 0, uintptr(numVerts))
}

// ─── Textures ─────────────────────────────────────────────────

// takeTexID returns a free texture ID, or 0 when all are in use.
func (c *metalCtx) takeTexID() int {
	if c.freeTexCount > 0 {
		c.freeTexCount--
		return int(c.freeTexIDs[c.freeTexCount])
	}
	if c.nextTexID >= maxTex {
		return 0
	}
	c.nextTexID++
	return c.nextTexID - 1
}

// giveTexID puts id back on the free list.
func (c *metalCtx) giveTexID(id int) {
	if c.freeTexCount < maxTex {
		c.freeTexIDs[c.freeTexCount] = int32(id)
		c.freeTexCount++
	}
}

// upload writes w x h pixels of bpp bytes from pix to tex at x, y.
func (c *metalCtx) upload(tex objc.ID, x, y, w, h, bpp int, pix unsafe.Pointer) {
	// MTLRegion is 48 bytes of integers: it goes by pointer.
	c.region = [6]uint{uint(x), uint(y), 0, uint(w), uint(h), 1}
	msg(tex, rsel.replaceRegion, ptr(&c.region), 0, uintptr(pix), uintptr(w*bpp))
}

// createTexture makes an RGBA8 texture and uploads pixels when not nil.
// It returns the texture ID, or 0.
func (c *metalCtx) createTexture(w, h int, pixels unsafe.Pointer) int {
	if c == nil || c.device == 0 || w <= 0 || h <= 0 {
		return 0
	}
	return c.createFormatTexture(w, h, mtlPixelFormatRGBA8Unorm, 4, pixels)
}

// createFormatTexture makes a texture of the given format and uploads
// pixels when not nil. It returns the texture ID, or 0.
func (c *metalCtx) createFormatTexture(w, h int, format uintptr, bpp int, pixels unsafe.Pointer) int {
	id := c.takeTexID()
	if id == 0 {
		return 0
	}
	tex := c.sampledTexture(w, h, format)
	if tex == 0 {
		c.giveTexID(id)
		return 0
	}
	if pixels != nil {
		c.upload(tex, 0, 0, w, h, bpp, pixels)
	}
	c.textures[id] = tex
	return id
}

// updateTexture writes a w x h RGBA8 region at x, y of texture id.
func (c *metalCtx) updateTexture(id, x, y, w, h int, data unsafe.Pointer) {
	if c == nil || data == nil || id <= 0 || id >= maxTex || c.textures[id] == 0 {
		return
	}
	if w <= 0 || h <= 0 || x < 0 || y < 0 {
		return
	}
	c.upload(c.textures[id], x, y, w, h, 4, data)
}

// deleteTexture releases texture id and frees the ID.
func (c *metalCtx) deleteTexture(id int) {
	if c == nil || id <= 0 || id >= maxTex || c.textures[id] == 0 {
		return
	}
	setTex(&c.textures[id], 0)
	setTex(&c.texLastUse[id], 0)
	c.texStreamed[id] = false
	c.giveTexID(id)
}

// noteTextureUse records that the frame being drawn samples texture id,
// when the texture is streamed.
func (c *metalCtx) noteTextureUse(id int) {
	if c.texStreamed[id] && c.texLastUse[id] != c.cmdBuf {
		setTex(&c.texLastUse[id], retain(c.cmdBuf))
	}
}

// waitTextureIdle blocks until no submitted frame samples texture id.
// replaceRegion copies at once and does not wait for GPU work already
// committed, so a write to a texture that an earlier frame still reads
// changes what that frame shows. A use by the frame being drawn needs no
// wait: nothing is submitted yet, so the write lands before the GPU runs
// it. Every upload into a streamed texture goes through here.
func (c *metalCtx) waitTextureIdle(id int) {
	last := c.texLastUse[id]
	if last == 0 || last == c.cmdBuf {
		return
	}
	msg(last, rsel.waitUntilCompleted)
	setTex(&c.texLastUse[id], 0)
}

// streamTexture marks texture id as streamed (gui.PixelSurface).
func (c *metalCtx) streamTexture(id int) {
	if c == nil || id <= 0 || id >= maxTex || c.textures[id] == 0 {
		return
	}
	c.texStreamed[id] = true
}

// updateStreamTexture writes w x h pixels into streamed texture id,
// after the GPU is done with the frames that sample it.
func (c *metalCtx) updateStreamTexture(id, w, h, bpp int, data unsafe.Pointer) {
	if c == nil || id <= 0 || id >= maxTex || c.textures[id] == 0 || data == nil {
		return
	}
	if w <= 0 || h <= 0 || bpp <= 0 {
		return
	}
	c.waitTextureIdle(id)
	c.upload(c.textures[id], 0, 0, w, h, bpp, data)
}

// planeFormat maps a gui.PixelFormat (0 RGBA8, 1 R8, 2 RG8) to Metal.
func planeFormat(format int) uintptr {
	switch format {
	case 1:
		return mtlPixelFormatR8Unorm
	case 2:
		return mtlPixelFormatRG8Unorm
	default:
		return mtlPixelFormatRGBA8Unorm
	}
}

// createPlaneTexture makes a streamed texture for a shader channel
// plane and uploads pixels. It returns the texture ID, or 0.
func (c *metalCtx) createPlaneTexture(format, w, h, bpp int, pixels unsafe.Pointer) int {
	if c == nil || c.device == 0 || w <= 0 || h <= 0 || bpp <= 0 || pixels == nil {
		return 0
	}
	id := c.createFormatTexture(w, h, planeFormat(format), bpp, pixels)
	if id != 0 {
		c.texStreamed[id] = true
	}
	return id
}

// clearPixel is the pixel of transparentTex. It is global, so its
// address can go to replaceRegion.
var clearPixel [4]byte

// bindChannel binds texture tex at texture(channel+1) for a custom
// shader iChannel, or the transparent texture when tex is 0, and
// records the use so the next streamed upload waits for this frame.
func (c *metalCtx) bindChannel(channel, tex int) {
	if c == nil || c.enc == 0 || channel < 0 || channel > 3 {
		return
	}
	if c.transparentTex == 0 {
		c.transparentTex = c.sampledTexture(1, 1, mtlPixelFormatRGBA8Unorm)
		if c.transparentTex == 0 {
			return
		}
		c.upload(c.transparentTex, 0, 0, 1, 1, 4, unsafe.Pointer(&clearPixel))
	}
	t := c.transparentTex
	if tex > 0 && tex < maxTex && c.textures[tex] != 0 {
		t = c.textures[tex]
		c.noteTextureUse(tex)
	}
	c.setFragmentTex(channel+1, t, &c.st.channels[channel])
}
