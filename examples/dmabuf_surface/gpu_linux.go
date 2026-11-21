//go:build linux && !android

package main

import (
	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/internal/gbm"
)

// gpuFrames is a pool of NV12 frames in GBM buffers, standing in for a
// video decoder's picture pool. A frame is painted only while it is free:
// from the moment UpdateDMABuf lends it until its Release, the renderer
// may be reading it.
type gpuFrames struct {
	dev  *gbm.Device
	free chan *gpuFrame
	all  []*gpuFrame
}

// gpuFrame is one pooled frame, and the GPUFrame the surface is lent. A
// pointer, so the surface can tell frames apart; reused, so publishing
// allocates nothing.
type gpuFrame struct {
	pool         *gpuFrames
	luma, chroma *gbm.Buffer
	planes       [2]gui.DMABufPlane
}

// Release runs on the render thread once the GPU has finished with the
// frame. It must not block: the pool's channel has room for every frame.
func (f *gpuFrame) Release() { f.pool.free <- f }

func newGPUFrames(n int) (*gpuFrames, error) {
	dev, err := gbm.Open()
	if err != nil {
		return nil, err
	}
	g := &gpuFrames{dev: dev, free: make(chan *gpuFrame, n)}
	for range n {
		f := &gpuFrame{pool: g}
		// The driver picks the layout, tiled or not, as a decoder's
		// would; the import passes its modifier on.
		if f.luma, err = dev.NewBuffer(gbm.FormatR8, frameWidth, frameHeight, false); err == nil {
			f.chroma, err = dev.NewBuffer(gbm.FormatGR88, frameWidth/2, frameHeight/2, false)
		}
		if err != nil {
			if f.luma != nil {
				f.luma.Close()
			}
			g.close()
			return nil, err
		}
		f.planes = [2]gui.DMABufPlane{f.luma.Plane(), f.chroma.Plane()}
		g.all = append(g.all, f)
		g.free <- f
	}
	return g, nil
}

// publish paints frame n into a free frame and lends it to the surface.
// It reports false, and publishes nothing, when every frame is still lent.
func (g *gpuFrames) publish(surface *gui.PixelSurface, n int) (bool, error) {
	var f *gpuFrame
	select {
	case f = <-g.free:
	default:
		return false, nil
	}
	err := f.luma.Write(1, func(y int, row []byte) { lumaRow(n, y, row) })
	if err == nil {
		err = f.chroma.Write(2, func(y int, row []byte) { chromaRow(n, y, row) })
	}
	if err == nil {
		err = surface.UpdateDMABuf(f.planes[:], f)
	}
	if err != nil {
		g.free <- f // not taken: still ours
		return false, err
	}
	return true, nil
}

// close frees every buffer and the device, once the surface is closed and
// its renderers have released every frame.
func (g *gpuFrames) close() {
	for _, f := range g.all {
		f.luma.Close()
		f.chroma.Close()
	}
	g.dev.Close()
}
