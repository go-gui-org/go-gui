//go:build !linux || android

package main

import (
	"errors"

	"github.com/go-gui-org/go-gui/gui"
)

// gpuFrames has no GPU buffers here: DMA-BUFs are Linux's. The example
// copies its frames instead.
type gpuFrames struct{}

func newGPUFrames(int) (*gpuFrames, error) { return nil, errors.New("DMA-BUFs need Linux") }

func (*gpuFrames) publish(*gui.PixelSurface, int) (bool, error) { return false, nil }

func (*gpuFrames) close() {}
