//go:build linux && !js && !android

package gl

import (
	"slices"
	"strings"
	"unsafe"

	"github.com/ebitengine/purego"

	"github.com/go-gui-org/go-gui/gui"
	gogl "github.com/go-gui-org/go-gui/gui/backend/internal/glbind"
)

// EGL_EXT_image_dma_buf_import and EGL_EXT_image_dma_buf_import_modifiers
// (#1326).
const (
	eglExtensions         = 0x3055
	eglImagePreserved     = 0x30D2 // EGL_IMAGE_PRESERVED_KHR
	eglTrue               = 1
	eglHeight             = 0x3056
	eglWidth              = 0x3057
	eglLinuxDMABuf        = 0x3270
	eglLinuxDRMFourCC     = 0x3271
	eglDMABufPlane0FD     = 0x3272
	eglDMABufPlane0Offset = 0x3273
	eglDMABufPlane0Pitch  = 0x3274
)

// The attributes of memory planes 0 to 3: FD, offset, pitch, and the
// modifier's low and high halves. Planes 1 to 3 carry a layout's metadata.
var eglDMABufPlaneAttribs = [4][5]int32{
	{eglDMABufPlane0FD, eglDMABufPlane0Offset, eglDMABufPlane0Pitch, 0x3443, 0x3444},
	{0x3275, 0x3276, 0x3277, 0x3445, 0x3446},
	{0x3278, 0x3279, 0x327A, 0x3447, 0x3448},
	{0x3440, 0x3441, 0x3442, 0x3449, 0x344A},
}

// DRM format codes of the plane formats, and the modifier that leaves the
// layout to the driver.
const (
	drmFormatR8         = 0x20203852 // 'R8  '
	drmFormatGR88       = 0x38385247 // 'GR88': R in the low byte, as RG8
	drmFormatABGR8888   = 0x34324241 // 'AB24': bytes R, G, B, A, as RGBA8
	drmFormatModInvalid = 0x00ffffffffffffff
)

// eglDMABuf imports DMA-BUF planes as EGLImages on one EGL display. Its
// entry points are called through purego.SyscallN, which allocates far
// less than a RegisterFunc wrapper on a path that runs for every plane of
// every frame.
type eglDMABuf struct {
	dpy             uintptr
	create, destroy uintptr // eglCreateImageKHR, eglDestroyImageKHR
	// externalOnly holds the layouts the display imports only for an
	// external texture (GL_TEXTURE_EXTERNAL_OES), which a channel shader
	// cannot sample: createImage refuses them.
	externalOnly map[formatModifier]bool
	// attribs is the attribute list, filled per call: the size, format and
	// EGL_IMAGE_PRESERVED_KHR, then up to four memory planes of five
	// attributes, then EGL_NONE. It lives here so that handing it to the
	// driver allocates nothing.
	attribs [8 + 4*10 + 1]int32
}

// dmabufImporter returns an importer for this window's EGL display, or
// nil when the display or the context cannot import DMA-BUFs: then the
// window does not report gui.Window.ImportsDMABuf. Explicit modifiers are
// required, because hardware decoders export tiled buffers. The context
// must be current.
func (p *platformState) dmabufImporter() dmabufImporter {
	if p.eglDpy == 0 || eglQueryString == nil {
		return nil
	}
	exts := strings.Fields(cGoStringN(eglQueryString(p.eglDpy, eglExtensions), 1<<16))
	if !slices.Contains(exts, "EGL_EXT_image_dma_buf_import") ||
		!slices.Contains(exts, "EGL_EXT_image_dma_buf_import_modifiers") ||
		!glHasExtension("GL_OES_EGL_image") {
		return nil
	}
	create, destroy := eglProc("eglCreateImageKHR"), eglProc("eglDestroyImageKHR")
	if create == 0 || destroy == 0 || !gogl.InitEGLImage(eglProc) {
		return nil
	}
	e := &eglDMABuf{dpy: p.eglDpy, create: create, destroy: destroy, externalOnly: map[formatModifier]bool{}}
	for _, fourcc := range []uint32{drmFormatR8, drmFormatGR88, drmFormatABGR8888} {
		mods, external := queryModifiers(p.eglDpy, fourcc)
		for i, m := range mods {
			if external[i] {
				e.externalOnly[formatModifier{fourcc, m}] = true
			}
		}
	}
	return e
}

// formatModifier is a DRM format in one layout.
type formatModifier struct {
	fourcc   uint32
	modifier uint64
}

// queryModifiers lists the layouts the display imports fourcc in, and for
// each whether only an external texture can sample it
// (eglQueryDmaBufModifiersEXT).
func queryModifiers(dpy uintptr, fourcc uint32) (mods []uint64, externalOnly []bool) {
	query := eglProc("eglQueryDmaBufModifiersEXT")
	if query == 0 {
		return nil, nil
	}
	var n int32
	purego.SyscallN(query, dpy, uintptr(fourcc), 0, 0, 0, uintptr(unsafe.Pointer(&n)))
	if n <= 0 {
		return nil, nil
	}
	mods = make([]uint64, n)
	external := make([]uint32, n) // EGLBoolean
	purego.SyscallN(query, dpy, uintptr(fourcc), uintptr(n),
		uintptr(unsafe.Pointer(&mods[0])), uintptr(unsafe.Pointer(&external[0])), uintptr(unsafe.Pointer(&n)))
	n = min(n, int32(len(mods)))
	externalOnly = make([]bool, n)
	for i := range externalOnly {
		externalOnly[i] = external[i] != 0
	}
	return mods[:n], externalOnly
}

// glHasExtension reports whether the current context lists name.
func glHasExtension(name string) bool {
	var n int32
	gogl.GetIntegerv(gogl.NUM_EXTENSIONS, &n)
	for i := range n {
		if cGoString(gogl.GetStringi(gogl.EXTENSIONS, uint32(i))) == name {
			return true
		}
	}
	return false
}

// createImage imports one plane, or returns 0 when the driver refuses it.
func (e *eglDMABuf) createImage(format gui.PixelFormat, width, height int, buf gui.DMABufPlane) uintptr {
	var fourcc uint32
	switch format {
	case gui.PixelFormatR8:
		fourcc = drmFormatR8
	case gui.PixelFormatRG8:
		fourcc = drmFormatGR88
	case gui.PixelFormatRGBA8:
		fourcc = drmFormatABGR8888
	default:
		return 0
	}
	if len(buf.Metadata) > len(eglDMABufPlaneAttribs)-1 || e.externalOnly[formatModifier{fourcc, buf.Modifier}] {
		return 0
	}
	a := &e.attribs
	a[0], a[1] = eglWidth, int32(width)
	a[2], a[3] = eglHeight, int32(height)
	a[4], a[5] = eglLinuxDRMFourCC, int32(fourcc)
	// The decoder's pixels must survive the import: without this,
	// EGL_KHR_image_base leaves an image's contents undefined after
	// creation.
	a[6], a[7] = eglImagePreserved, eglTrue
	n := 8
	put := func(plane int, fd int, offset, pitch uint32) {
		names := &eglDMABufPlaneAttribs[plane]
		a[n], a[n+1] = names[0], int32(fd)
		a[n+2], a[n+3] = names[1], int32(offset)
		a[n+4], a[n+5] = names[2], int32(pitch)
		n += 6
		// The modifier goes with every memory plane, as EGL asks; an
		// implicit layout (DRM_FORMAT_MOD_INVALID) names none.
		if buf.Modifier != drmFormatModInvalid {
			a[n], a[n+1] = names[3], int32(uint32(buf.Modifier))
			a[n+2], a[n+3] = names[4], int32(uint32(buf.Modifier>>32))
			n += 4
		}
	}
	put(0, buf.FD, buf.Offset, buf.Pitch)
	for i, m := range buf.Metadata {
		put(i+1, m.FD, m.Offset, m.Pitch)
	}
	a[n] = eglNone
	image, _, _ := purego.SyscallN(e.create, e.dpy, eglNoContext, eglLinuxDMABuf, 0,
		uintptr(unsafe.Pointer(&a[0])))
	return image
}

// bindImage gives the texture bound to GL_TEXTURE_2D the image's storage.
// GL reports a refusal, such as a layout only an external texture can
// sample, as an error, so errors left by earlier calls are cleared first;
// bounded, as a lost context may keep reporting one.
func (e *eglDMABuf) bindImage(image uintptr) bool {
	for range 8 {
		if gogl.GetError() == gogl.NO_ERROR {
			break
		}
	}
	gogl.EGLImageTargetTexture2DOES(gogl.TEXTURE_2D, image)
	return gogl.GetError() == gogl.NO_ERROR
}

func (e *eglDMABuf) destroyImage(image uintptr) {
	if image != 0 {
		_, _, _ = purego.SyscallN(e.destroy, e.dpy, image)
	}
}
