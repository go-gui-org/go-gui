package gui

import "testing"

// pixAt returns the RGBA bytes of one pixel in a w-wide buffer.
func pixAt(pix []byte, w, x, y int) (r, g, b, a uint8) {
	i := (y*w + x) * 4
	return pix[i], pix[i+1], pix[i+2], pix[i+3]
}

// nearGray reports whether the channels are within tol of each other.
// Generators sample pixel *centres*, so the leftmost column of a plane
// is S≈0.008 rather than exactly 0 — near-gray, never bit-identical.
func nearGray(r, g, b uint8, tol int) bool {
	mx := max(int(r), max(int(g), int(b)))
	mn := min(int(r), min(int(g), int(b)))
	return mx-mn <= tol
}

// The plane's corners are what tell a user what the control does:
// saturation right, lightness up.
func TestPlanePixelsCorners(t *testing.T) {
	const size = 64
	pix := planePixels(0, size) // red hue

	// Top-left: S=0, L=1 → white. Top-right: S=1, L=1 → white too,
	// since full lightness washes out any hue.
	if r, g, b, _ := pixAt(pix, size, 0, 0); r < 250 || g < 250 || b < 250 {
		t.Errorf("top-left = %d,%d,%d, want ~white", r, g, b)
	}
	// Bottom row is L≈0 → black regardless of saturation.
	if r, g, b, _ := pixAt(pix, size, size-1, size-1); r > 8 || g > 8 || b > 8 {
		t.Errorf("bottom-right = %d,%d,%d, want ~black", r, g, b)
	}
	// Middle-left is unsaturated mid-lightness → gray.
	r, g, b, _ := pixAt(pix, size, 0, size/2)
	if !nearGray(r, g, b, 4) {
		t.Errorf("middle-left = %d,%d,%d, want ~gray", r, g, b)
	}
	// Middle-right is fully saturated red at mid lightness.
	r, g, b, _ = pixAt(pix, size, size-1, size/2)
	if r < 240 || g > 20 || b > 20 {
		t.Errorf("middle-right = %d,%d,%d, want ~red", r, g, b)
	}
}

// The wheel must be a disc: opaque at the centre, transparent at the
// corners, with hue following the angle.
func TestWheelPixelsDiscAndAngle(t *testing.T) {
	const size = 64
	pix := wheelPixels(0.5, size)

	if _, _, _, a := pixAt(pix, size, 0, 0); a != 0 {
		t.Errorf("corner alpha = %d, want 0 (outside the disc)", a)
	}
	if _, _, _, a := pixAt(pix, size, size/2, size/2); a != 255 {
		t.Errorf("centre alpha = %d, want 255", a)
	}
	// Centre is radius 0 → saturation 0 → gray at L=0.5.
	if r, g, b, _ := pixAt(pix, size, size/2, size/2); !nearGray(r, g, b, 8) {
		t.Errorf("centre = %d,%d,%d, want ~gray", r, g, b)
	}
	// Hue 0 (red) sits at +x, i.e. the right edge of the middle row.
	if r, g, b, _ := pixAt(pix, size, size-2, size/2); r < 200 || b > 80 {
		t.Errorf("right edge = %d,%d,%d, want ~red", r, g, b)
	}
	// Hue 180 (cyan) is opposite it.
	if r, g, b, _ := pixAt(pix, size, 1, size/2); r > 80 || b < 200 {
		t.Errorf("left edge = %d,%d,%d, want ~cyan", r, g, b)
	}
}

func TestHueOfQuadrants(t *testing.T) {
	cases := []struct {
		name   string
		dx, dy float32
		want   float32
	}{
		{"right", 1, 0, 0},
		{"up", 0, -1, 90},    // y grows downward on screen
		{"left", -1, 0, 180}, //
		{"down", 0, 1, 270},  //
		{"up-right", 1, -1, 45},
	}
	for _, tc := range cases {
		if got := hueOf(tc.dx, tc.dy); f32Abs(got-tc.want) > 0.5 {
			t.Errorf("%s: hueOf = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// The thumb must sit on the color it picks, which is why the strip
// insets its gradient by half a thumb and pins the end caps.
func TestChannelStripInsetPinsEndCaps(t *testing.T) {
	const w, h, inset = 64, 4, 8
	v := HSLA{H: 0, S: 1, L: 0.5, A: 1}
	pix := channelStripPixels(ChannelHue, v, w, h, inset, false)

	// Everything inside the left inset pins to hue 0 (red).
	for x := range inset {
		r, g, b, _ := pixAt(pix, w, x, 0)
		if r < 250 || g > 5 || b > 5 {
			t.Fatalf("x=%d = %d,%d,%d, want pinned red", x, r, g, b)
		}
	}
	// The right cap pins to hue 360, which is red again.
	r, g, b, _ := pixAt(pix, w, w-1, 0)
	if r < 250 || g > 5 || b > 5 {
		t.Errorf("right cap = %d,%d,%d, want pinned red", r, g, b)
	}
	// A third of the way along the track is around hue 120 (green).
	x := inset + (w-2*inset)/3
	r, g, b, _ = pixAt(pix, w, x, 0)
	if g < 200 || r > 60 {
		t.Errorf("x=%d = %d,%d,%d, want ~green", x, r, g, b)
	}
}

// The H/S/L tracks are drawn opaque; only the alpha track composites
// over the checkerboard, and it must actually reach transparency at the
// low end and the color at the high end.
func TestChannelStripAlphaCompositesOverChecker(t *testing.T) {
	const w, h, inset = 64, checkerCell * 2, 8
	v := HSLA{H: 0, S: 1, L: 0.5, A: 1} // red

	opaque := channelStripPixels(ChannelHue, v, w, h, inset, false)
	for y := range h {
		if _, _, _, a := pixAt(opaque, w, w/2, y); a != 255 {
			t.Fatalf("hue strip alpha = %d, want opaque", a)
		}
	}

	alpha := channelStripPixels(ChannelAlpha, v, w, h, inset, false)
	// Left cap is alpha 0 → pure checkerboard, so the two checker rows
	// must differ from each other.
	r0, g0, b0, _ := pixAt(alpha, w, 0, 0)
	r1, _, _, _ := pixAt(alpha, w, 0, checkerCell)
	if r0 != g0 || g0 != b0 {
		t.Errorf("alpha=0 pixel = %d,%d,%d, want neutral checker",
			r0, g0, b0)
	}
	if r0 == r1 {
		t.Error("checkerboard rows identical; pattern not visible")
	}
	// Right cap is alpha 1 → the color itself.
	r, g, b, _ := pixAt(alpha, w, w-1, 0)
	if r < 240 || g > 20 || b > 20 {
		t.Errorf("alpha=1 pixel = %d,%d,%d, want ~red", r, g, b)
	}
}

// Content keying is the whole caching design: a channel must not
// rebuild its own strip, and must rebuild the strips that depend on it.
func TestChannelStripKeyingIgnoresOwnChannel(t *testing.T) {
	resetMemImages(t)
	const w, h, inset = 32, 8, 4

	base := HSLA{H: 10, S: 0.5, L: 0.5, A: 1}
	hueSrc := channelStripSrc(ChannelHue, base, w, h, inset, false)
	satSrc := channelStripSrc(ChannelSaturation, base, w, h, inset, false)

	moved := base
	moved.H = 200
	if got := channelStripSrc(ChannelHue, moved, w, h, inset, false); got != hueSrc {
		t.Error("hue strip rebuilt when only hue moved")
	}
	if got := channelStripSrc(
		ChannelSaturation, moved, w, h, inset, false); got == satSrc {
		t.Error("saturation strip did not rebuild when hue moved")
	}

	// Alpha is in no strip's key at all.
	faded := base
	faded.A = 0.25
	if got := channelStripSrc(
		ChannelSaturation, faded, w, h, inset, false); got != satSrc {
		t.Error("saturation strip rebuilt when only alpha moved")
	}
}

func TestPlaneAndWheelKeying(t *testing.T) {
	resetMemImages(t)
	base := HSLA{H: 10, S: 0.5, L: 0.5, A: 1}

	plane := planeSrc(base, 16)
	wheel := wheelSrc(base, 16)

	// Saturation moves the markers, not the imagery.
	sat := base
	sat.S = 0.9
	if planeSrc(sat, 16) != plane {
		t.Error("plane rebuilt when only saturation moved")
	}
	if wheelSrc(sat, 16) != wheel {
		t.Error("wheel rebuilt when only saturation moved")
	}

	// Hue repaints the plane; lightness repaints the wheel.
	hue := base
	hue.H = 200
	if planeSrc(hue, 16) == plane {
		t.Error("plane did not rebuild when hue moved")
	}
	if wheelSrc(hue, 16) != wheel {
		t.Error("wheel rebuilt when only hue moved")
	}
	light := base
	light.L = 0.9
	if wheelSrc(light, 16) == wheel {
		t.Error("wheel did not rebuild when lightness moved")
	}
}

// Sub-quantum jitter must land on the same key, or a drag rebuilds
// every buffer every frame — the failure the quantization prevents.
func TestKeysQuantize(t *testing.T) {
	resetMemImages(t)
	a := HSLA{H: 200.1, S: 0.5001, L: 0.5, A: 1}
	b := HSLA{H: 200.2, S: 0.5002, L: 0.5, A: 1}
	if planeSrc(a, 16) != planeSrc(b, 16) {
		t.Error("plane key not quantized")
	}
	if wheelSrc(a, 16) != wheelSrc(b, 16) {
		t.Error("wheel key not quantized")
	}
}

// A steady frame must cost nothing: the key is built into a stack
// buffer and the hit returns the string stored at registration.
func TestBufferSrcCacheHitDoesNotAllocate(t *testing.T) {
	resetMemImages(t)
	v := HSLA{H: 210, S: 0.7, L: 0.55, A: 0.85}
	planeSrc(v, 16)
	wheelSrc(v, 16)
	channelStripSrc(ChannelHue, v, 32, 8, 4, false)
	checkerSrc(16, 16)

	if got := testing.AllocsPerRun(100, func() {
		planeSrc(v, 16)
		wheelSrc(v, 16)
		channelStripSrc(ChannelHue, v, 32, 8, 4, false)
		checkerSrc(16, 16)
	}); got != 0 {
		t.Errorf("allocs per frame = %v, want 0", got)
	}
}

func TestColorChannelAccessors(t *testing.T) {
	v := HSLA{H: 180, S: 0.25, L: 0.75, A: 0.5}
	cases := []struct {
		ch   ColorChannel
		want float32
		span float32
	}{
		{ChannelHue, 180, 360},
		{ChannelSaturation, 25, 100},
		{ChannelLightness, 75, 100},
		{ChannelAlpha, 0.5, 1},
	}
	for _, tc := range cases {
		if got := tc.ch.value(v); f32Abs(got-tc.want) > 0.001 {
			t.Errorf("ch %d value = %v, want %v", tc.ch, got, tc.want)
		}
		if got := tc.ch.span(); got != tc.span {
			t.Errorf("ch %d span = %v, want %v", tc.ch, got, tc.span)
		}
		// with/value must round-trip, since a drag writes through with
		// and the next frame reads back through value.
		got := tc.ch.value(tc.ch.with(v, tc.want))
		if f32Abs(got-tc.want) > 0.001 {
			t.Errorf("ch %d round-trip = %v, want %v", tc.ch, got, tc.want)
		}
	}
}
