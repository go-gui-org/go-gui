package gui

import "github.com/go-gui-org/go-glyph"

// renderTextBlock emits one glyph-layout draw per chunk of blk that
// reaches into clip, each at its chunk's offset within the shape. A
// chunk the block has not shaped yet is shaped here, so what comes into
// view is drawn on the frame it appears; the chunks outside the clip
// stay unshaped. A typewriter reveal paints the chunks up to its end,
// the last of them cut at the revealed byte.
func renderTextBlock(
	shape *Shape, baseX, baseY float32, blk *textBlock, style TextStyle,
	gradient *glyph.GradientConfig, reveal bool, clip drawClip, w *Window,
) {
	var anim *textAnimRender
	if shape.TC != nil {
		anim = shape.TC.anim
	}
	blk.settle()
	for i := 0; i < len(blk.chunks); i++ {
		c := &blk.chunks[i]
		if c.start == c.end || !chunkIntersects(c, baseX, baseY, clip) {
			continue
		}
		// Shaping may cut an open chunk, which inserts its remainder as
		// the next chunk and moves the chunks below: re-read and re-test.
		blk.ensure(i, w)
		blk.settle()
		c = &blk.chunks[i]
		if c.layout == nil || !chunkIntersects(c, baseX, baseY, clip) {
			continue
		}
		l := *c.layout
		if reveal && anim != nil {
			if anim.revealEnd <= c.start {
				break
			}
			if anim.revealEnd < c.end {
				l = revealLayoutGlyphs(l, anim.revealEnd-c.start, w)
			}
		}
		// The chunks were shaped in the block's colours; a focused,
		// disabled or fading text paints the same glyphs in others.
		if !textPaintEqual(blk.style, style) {
			l.Items = recolorLayoutItems(l.Items, style, w)
		}
		emitTextLayoutCmd(shape, blk.text[c.start:c.end],
			baseX+c.x, baseY+c.y, l, style, gradient, false, w)
	}
}

// chunkIntersects reports whether a chunk placed at the shape's origin
// reaches into clip. An unshaped chunk is tested on its estimate; a
// zero-width one still counts for the height of its row.
func chunkIntersects(c *textChunk, baseX, baseY float32, clip drawClip) bool {
	return rectsOverlap(drawClip{
		X: baseX + c.x, Y: baseY + c.y,
		Width: f32Max(c.width, 1), Height: f32Max(c.height, 1),
	}, clip)
}
