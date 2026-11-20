//go:build darwin && arm64 && !ios

// The arm64 tag is load-bearing, not drift: vertex is used only by the
// backend files (draw.go, text.go, rotation.go), which are darwin/arm64
// only. Without it this file would be the sole survivor under
// GOARCH=amd64, and lint would flag the alias unused.

package metal

import "github.com/go-gui-org/go-gui/gui/backend/internal/gpu"

// vertex is a local alias for gpu.Vertex.
type vertex = gpu.Vertex
