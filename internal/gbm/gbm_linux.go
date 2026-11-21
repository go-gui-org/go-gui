//go:build linux && !android

// Package gbm allocates DMA-BUFs with Mesa's GBM, through purego, to feed
// gui.PixelSurface.UpdateDMABuf in the GL backend's import test and in
// examples/dmabuf_surface (#1326). It stands in for a hardware video
// decoder. Nothing in gui depends on it.
package gbm

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"unsafe"

	"github.com/ebitengine/purego"

	"github.com/go-gui-org/go-gui/gui"
)

// Formats GBM allocates for the gui plane formats.
const (
	FormatR8   = 0x20203852 // DRM_FORMAT_R8, a PixelFormatR8 plane
	FormatGR88 = 0x38385247 // DRM_FORMAT_GR88, a PixelFormatRG8 plane
)

const (
	useRendering  = 1 << 2 // GBM_BO_USE_RENDERING
	useLinear     = 1 << 4 // GBM_BO_USE_LINEAR
	transferWrite = 1 << 1 // GBM_BO_TRANSFER_WRITE
)

var lib struct {
	once                                   sync.Once
	err                                    error
	createDevice, deviceDestroy            uintptr
	boCreate, boDestroy, boGetFD, boGetMod uintptr
	boGetOffset, boMap, boUnmap            uintptr
	boPlaneCount, boStrideForPlane         uintptr
	// boFDForPlane is gbm_bo_get_fd_for_plane (Mesa 21.1), or 0: then
	// every memory plane is taken to live in the buffer's one descriptor.
	boFDForPlane uintptr
	// boCreateModifiers is gbm_bo_create_with_modifiers2 (Mesa 21.3), or
	// 0 in an older libgbm; modifierPlanes is
	// gbm_device_get_format_modifier_plane_count, or 0.
	boCreateModifiers, modifierPlanes uintptr
}

func load() error {
	lib.once.Do(func() {
		h, err := purego.Dlopen("libgbm.so.1", purego.RTLD_NOW|purego.RTLD_GLOBAL)
		if err != nil {
			lib.err = fmt.Errorf("gbm: %w", err)
			return
		}
		for _, s := range []struct {
			addr *uintptr
			name string
		}{
			{&lib.createDevice, "gbm_create_device"},
			{&lib.deviceDestroy, "gbm_device_destroy"},
			{&lib.boCreate, "gbm_bo_create"},
			{&lib.boDestroy, "gbm_bo_destroy"},
			{&lib.boGetFD, "gbm_bo_get_fd"},
			{&lib.boGetMod, "gbm_bo_get_modifier"},
			{&lib.boGetOffset, "gbm_bo_get_offset"},
			{&lib.boMap, "gbm_bo_map"},
			{&lib.boUnmap, "gbm_bo_unmap"},
			{&lib.boPlaneCount, "gbm_bo_get_plane_count"},
			{&lib.boStrideForPlane, "gbm_bo_get_stride_for_plane"},
		} {
			if *s.addr, err = purego.Dlsym(h, s.name); err != nil {
				lib.err = fmt.Errorf("gbm: %w", err)
				return
			}
		}
		lib.boCreateModifiers, _ = purego.Dlsym(h, "gbm_bo_create_with_modifiers2")
		lib.boFDForPlane, _ = purego.Dlsym(h, "gbm_bo_get_fd_for_plane")
		lib.modifierPlanes, _ = purego.Dlsym(h, "gbm_device_get_format_modifier_plane_count")
	})
	return lib.err
}

// Device is a GBM device on a DRM render node.
type Device struct {
	file *os.File
	dev  uintptr
}

// Open opens GBM on the first DRM render node. It fails where there is no
// GPU, no render node or no libgbm.
func Open() (*Device, error) {
	if err := load(); err != nil {
		return nil, err
	}
	nodes, _ := filepath.Glob("/dev/dri/renderD*")
	if len(nodes) == 0 {
		return nil, errors.New("gbm: no DRM render node")
	}
	f, err := os.OpenFile(nodes[0], os.O_RDWR, 0)
	if err != nil {
		return nil, fmt.Errorf("gbm: %w", err)
	}
	dev, _, _ := purego.SyscallN(lib.createDevice, f.Fd())
	if dev == 0 {
		_ = f.Close()
		return nil, fmt.Errorf("gbm: gbm_create_device on %s failed", nodes[0])
	}
	return &Device{file: f, dev: dev}, nil
}

// Close destroys the device. Close its buffers first.
func (d *Device) Close() {
	purego.SyscallN(lib.deviceDestroy, d.dev)
	_ = d.file.Close()
}

// Buffer is one GBM buffer object and the DMA-BUF descriptors it exports:
// fds[0] for its first memory plane and one for each further plane, such
// as compression metadata.
type Buffer struct {
	bo            uintptr
	fds           []int
	Width, Height int
}

// NewBuffer allocates a width x height buffer of format. linear asks for
// rows one after another; otherwise the driver picks its layout, which may
// be tiled. Either way the layout is implicit: Plane reports
// DRM_FORMAT_MOD_INVALID, and an importer must leave the modifier out.
func (d *Device) NewBuffer(format uint32, width, height int, linear bool) (*Buffer, error) {
	flags := uintptr(useRendering)
	if linear {
		flags |= useLinear
	}
	bo, _, _ := purego.SyscallN(lib.boCreate, d.dev, uintptr(width), uintptr(height), uintptr(format), flags)
	if bo == 0 {
		return nil, fmt.Errorf("gbm: gbm_bo_create %dx%d format %#x failed", width, height, format)
	}
	return newBuffer(bo, width, height)
}

// newBuffer exports a DMA-BUF descriptor for each of bo's memory planes.
func newBuffer(bo uintptr, width, height int) (*Buffer, error) {
	b := &Buffer{bo: bo, Width: width, Height: height}
	n, _, _ := purego.SyscallN(lib.boPlaneCount, bo)
	for plane := range max(int(int32(n)), 1) {
		var fd uintptr
		switch {
		case plane == 0:
			fd, _, _ = purego.SyscallN(lib.boGetFD, bo)
		case lib.boFDForPlane != 0:
			fd, _, _ = purego.SyscallN(lib.boFDForPlane, bo, uintptr(plane))
		default:
			fd, _, _ = purego.SyscallN(lib.boGetFD, bo)
		}
		if int32(fd) < 0 {
			b.Close()
			return nil, fmt.Errorf("gbm: no descriptor for memory plane %d", plane)
		}
		b.fds = append(b.fds, int(int32(fd)))
	}
	return b, nil
}

// NewBufferModifiers allocates a width x height buffer of format in one of
// modifiers, the layouts the caller can import; the driver picks which.
// Plane reports the one it chose.
func (d *Device) NewBufferModifiers(format uint32, width, height int, modifiers []uint64) (*Buffer, error) {
	if lib.boCreateModifiers == 0 {
		return nil, errors.New("gbm: no gbm_bo_create_with_modifiers2 (Mesa 21.3 or later)")
	}
	if len(modifiers) == 0 {
		return nil, errors.New("gbm: no modifiers to choose from")
	}
	bo, _, _ := purego.SyscallN(lib.boCreateModifiers, d.dev, uintptr(width), uintptr(height), uintptr(format),
		uintptr(unsafe.Pointer(&modifiers[0])), uintptr(len(modifiers)), useRendering)
	if bo == 0 {
		return nil, fmt.Errorf("gbm: gbm_bo_create_with_modifiers2 %dx%d format %#x failed", width, height, format)
	}
	return newBuffer(bo, width, height)
}

// ModifierPlanes reports how many memory planes a buffer of format takes
// in modifier's layout: more than one when it carries metadata, such as
// AMD's DCC compression. It returns 0 when GBM cannot tell.
func (d *Device) ModifierPlanes(format uint32, modifier uint64) int {
	if lib.modifierPlanes == 0 {
		return 0
	}
	n, _, _ := purego.SyscallN(lib.modifierPlanes, d.dev, uintptr(format), uintptr(modifier))
	return max(int(int32(n)), 0)
}

// Write maps the buffer for writing, calls fill once per row with that
// row's bytes, and unmaps it. GBM copies through a staging buffer when the
// layout is tiled.
func (b *Buffer) Write(bytesPerPixel int, fill func(y int, row []byte)) error {
	var stride uint32
	var mapData uintptr
	p, _, _ := purego.SyscallN(lib.boMap, b.bo, 0, 0, uintptr(b.Width), uintptr(b.Height), transferWrite,
		uintptr(unsafe.Pointer(&stride)), uintptr(unsafe.Pointer(&mapData)))
	if p == 0 {
		return errors.New("gbm: gbm_bo_map failed")
	}
	base := *(*unsafe.Pointer)(unsafe.Pointer(&p)) // driver memory, not a Go pointer
	for y := range b.Height {
		fill(y, unsafe.Slice((*byte)(unsafe.Add(base, y*int(stride))), b.Width*bytesPerPixel))
	}
	purego.SyscallN(lib.boUnmap, b.bo, mapData)
	return nil
}

// Plane describes the buffer for UpdateDMABuf, its further memory planes
// as Metadata. The descriptors stay open until Close.
func (b *Buffer) Plane() gui.DMABufPlane {
	modifier, _, _ := purego.SyscallN(lib.boGetMod, b.bo)
	p := gui.DMABufPlane{Modifier: uint64(modifier)}
	for plane, fd := range b.fds {
		stride, _, _ := purego.SyscallN(lib.boStrideForPlane, b.bo, uintptr(plane))
		offset, _, _ := purego.SyscallN(lib.boGetOffset, b.bo, uintptr(plane))
		if plane == 0 {
			p.FD, p.Offset, p.Pitch = fd, uint32(offset), uint32(stride)
			continue
		}
		p.Metadata = append(p.Metadata, gui.DMABufMemory{FD: fd, Offset: uint32(offset), Pitch: uint32(stride)})
	}
	return p
}

// Close closes the descriptors and frees the buffer.
func (b *Buffer) Close() {
	for _, fd := range b.fds {
		_ = syscall.Close(fd)
	}
	purego.SyscallN(lib.boDestroy, b.bo)
}
