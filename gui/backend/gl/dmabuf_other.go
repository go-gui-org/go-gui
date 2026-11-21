//go:build !js && !darwin && !android && !linux

package gl

// dmabufImporter is nil: only Linux has DMA-BUFs (dmabuf_linux.go).
func (p *platformState) dmabufImporter() dmabufImporter { return nil }
