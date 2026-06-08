package gui

// rectsOverlap checks if two rectangles overlap (strict <).
func rectsOverlap(r1, r2 drawClip) bool {
	return r1.X < (r2.X+r2.Width) && r2.X < (r1.X+r1.Width) &&
		r1.Y < (r2.Y+r2.Height) && r2.Y < (r1.Y+r1.Height)
}

// dimAlpha halves the alpha for visually indicating disabled state.
func dimAlpha(c Color) Color {
	return Color{R: c.R, G: c.G, B: c.B, A: c.A / 2, set: c.set}
}

// resolveClipRadius computes the effective rounded clip radius for
// nested clipping containers.
func resolveClipRadius(parentRadius float32, shape *Shape) float32 {
	if !shape.Clip {
		return parentRadius
	}
	var baseRadius float32
	if shape.shapeType == shapeCircle {
		baseRadius = f32Min(shape.Width, shape.Height) / 2
	} else {
		baseRadius = shape.Radius
	}
	if !f32IsFinite(baseRadius) || baseRadius <= 0 {
		return parentRadius
	}
	leftInset := shape.Padding.Left + shape.SizeBorder
	rightInset := shape.Padding.Right + shape.SizeBorder
	topInset := shape.Padding.Top + shape.SizeBorder
	bottomInset := shape.Padding.Bottom + shape.SizeBorder
	inset := max(leftInset, rightInset, topInset, bottomInset)
	localRadius := f32Max(0, baseRadius-inset)
	if localRadius <= 0 {
		return parentRadius
	}
	if !f32IsFinite(parentRadius) || parentRadius <= 0 {
		return localRadius
	}
	return f32Min(parentRadius, localRadius)
}

// roundedImageClip holds clipped image draw parameters including
// mapped UV coordinates for SDF rounded clipping.
type roundedImageClip struct {
	X, Y   float32
	W, H   float32
	U0, V0 float32
	U1, V1 float32
}

// roundedImageClipParams computes the intersection of an image rect
// and a clip rect, mapping UV coordinates for the visible portion.
// Returns ok=false if there is no overlap.
func roundedImageClipParams(imgX, imgY, imgW, imgH float32, clip drawClip) (roundedImageClip, bool) {
	if imgW <= 0 || imgH <= 0 || clip.Width <= 0 || clip.Height <= 0 {
		return roundedImageClip{}, false
	}
	imgRight := imgX + imgW
	imgBottom := imgY + imgH
	clipRight := clip.X + clip.Width
	clipBottom := clip.Y + clip.Height
	x := f32Max(imgX, clip.X)
	y := f32Max(imgY, clip.Y)
	right := f32Min(imgRight, clipRight)
	bottom := f32Min(imgBottom, clipBottom)
	w := right - x
	h := bottom - y
	if w <= 0 || h <= 0 {
		return roundedImageClip{}, false
	}
	// Shrink into content box instead of cropping edge pixels.
	isInside := x >= imgX && y >= imgY && right <= imgRight && bottom <= imgBottom
	clipsSize := w < imgW || h < imgH
	anchorTopLeft := x == imgX && y == imgY
	if isInside && clipsSize && anchorTopLeft {
		return roundedImageClip{
			X: x, Y: y, W: w, H: h,
			U0: -1, V0: -1, U1: 1, V1: 1,
		}, true
	}
	invW := float32(2.0) / imgW
	invH := float32(2.0) / imgH
	return roundedImageClip{
		X: x, Y: y, W: w, H: h,
		U0: -1 + (x-imgX)*invW,
		V0: -1 + (y-imgY)*invH,
		U1: -1 + (right-imgX)*invW,
		V1: -1 + (bottom-imgY)*invH,
	}, true
}

// shapeBounds returns the shape's bounding rectangle as a
// drawClip.
func shapeBounds(shape *Shape) drawClip {
	return drawClip{
		X: shape.X, Y: shape.Y,
		Width: shape.Width, Height: shape.Height,
	}
}

// emitClipCmd emits a RenderClip command for the given clip rect.
func emitClipCmd(clip drawClip, w *Window) {
	emitRenderer(RenderCmd{
		Kind: RenderClip,
		X:    clip.X,
		Y:    clip.Y,
		W:    clip.Width,
		H:    clip.Height,
	}, w)
}

// quantizedScissorClip truncates clip coordinates to integer
// multiples of scale, matching sokol's scissor rect behavior.
func quantizedScissorClip(clip drawClip, scale float32) drawClip {
	if scale <= 0 {
		return clip
	}
	sx := int(clip.X * scale)
	sy := int(clip.Y * scale)
	sw := int(clip.Width * scale)
	sh := int(clip.Height * scale)
	return drawClip{
		X:      float32(sx) / scale,
		Y:      float32(sy) / scale,
		Width:  float32(sw) / scale,
		Height: float32(sh) / scale,
	}
}
