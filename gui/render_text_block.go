package gui

import "github.com/go-gui-org/go-glyph"

// renderTextBlock emits one glyph-layout draw per chunk of blk that
// reaches into clip, each at its chunk's offset within the shape, the
// whole block turned through xf about (baseX, baseY) when xfOn. A chunk
// the block has not shaped yet is shaped here, so what comes into view
// is drawn on the frame it appears; the chunks outside the clip stay
// unshaped. A typewriter reveal paints the chunks up to its end, the
// last of them cut at the revealed byte.
//
// glyph measures a gradient on the layout it draws and can be told no
// larger frame. A gradient text stays one layout up to what a layout can
// hold (textBlockOneLayout), so past that the block draws in its plain
// colour. The limit is the one main drew nothing past, and it covers a
// shimmer too, which stays in chunks below it: a shimmer can end while
// the text stays, and it runs across the lines, which wrap alike.
func renderTextBlock(
	shape *Shape, baseX, baseY float32, blk *textBlock, style TextStyle,
	gradient *glyph.GradientConfig, reveal bool,
	xf glyph.AffineTransform, xfOn bool, clip drawClip, w *Window,
) {
	if len(blk.text) > glyph.MaxTextLength {
		gradient, style.Gradient = nil, nil
	}
	var anim *textAnimRender
	if shape.TC != nil {
		anim = shape.TC.anim
	}
	blk.settle()
	for i := 0; i < len(blk.chunks); i++ {
		c := &blk.chunks[i]
		if c.start == c.end || !chunkVisible(c, baseX, baseY, xf, xfOn, clip) {
			continue
		}
		// Shaping may cut an open chunk, which inserts its remainder as
		// the next chunk and moves the chunks below: re-read and re-test.
		blk.ensure(i, w)
		blk.settle()
		c = &blk.chunks[i]
		if c.layout == nil || !chunkVisible(c, baseX, baseY, xf, xfOn, clip) {
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
		emitTextLayoutCmd(blk.text[c.start:c.end], baseX, baseY, c.x, c.y,
			l, style, gradient, xf, xfOn, w)
	}
}

// chunkVisible is chunkIntersects for a block drawn through xf when
// xfOn: it tests the bounds of the chunk's transformed box, which a
// rotation or a scale can bring into view from outside the untransformed
// one, and the other way round.
func chunkVisible(
	c *textChunk, baseX, baseY float32,
	xf glyph.AffineTransform, xfOn bool, clip drawClip,
) bool {
	if !xfOn {
		return chunkIntersects(c, baseX, baseY, clip)
	}
	x1, y1 := c.x+f32Max(c.width, 1), c.y+f32Max(c.height, 1)
	minX, minY := xf.Apply(c.x, c.y)
	maxX, maxY := minX, minY
	for _, p := range [3][2]float32{{x1, c.y}, {c.x, y1}, {x1, y1}} {
		px, py := xf.Apply(p[0], p[1])
		minX, maxX = f32Min(minX, px), f32Max(maxX, px)
		minY, maxY = f32Min(minY, py), f32Max(maxY, py)
	}
	if !f32IsFinite(minX) || !f32IsFinite(maxX) ||
		!f32IsFinite(minY) || !f32IsFinite(maxY) {
		return false // glyph draws nothing through a non-finite transform
	}
	return rectsOverlap(drawClip{
		X: baseX + minX, Y: baseY + minY,
		Width: maxX - minX, Height: maxY - minY,
	}, clip)
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
