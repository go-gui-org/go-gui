package gpu

// backingShrinkRatio bounds how much spare backing store
// RetainBackingSize keeps before snapping back to the need.
// Past it the kept store costs more memory than the realloc it
// saves.
const backingShrinkRatio = 8

// RetainBackingSize returns the backing-store size to keep for a
// draw needing needW x needH pixels, given the kept curW x curH
// store. It grows to cover the need and only shrinks when the
// kept store is more than backingShrinkRatio times the need, so
// alternating draws of different sizes do not reallocate every
// frame. Resizing a GL canvas drops its backing store, so an
// exact-fit policy reallocates once per widget per frame when
// two custom shaders differ in size.
//
// The web backend sizes its offscreen custom-shader canvas with
// this and blits the drawn sub-rectangle out of the kept store.
func RetainBackingSize(needW, needH, curW, curH int) (int, int) {
	needW = max(needW, 1)
	needH = max(needH, 1)
	if curW < 1 || curH < 1 {
		return needW, needH
	}
	if int64(curW)*int64(curH) >
		backingShrinkRatio*int64(needW)*int64(needH) {
		return needW, needH
	}
	return max(curW, needW), max(curH, needH)
}
