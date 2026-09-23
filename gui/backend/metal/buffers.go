//go:build darwin && cgo && !ios

// The cgo tag is load-bearing, not drift: vertex is consumed only
// by the cgo files (draw.go, text.go, rotation.go), which the go
// tool drops from cgo-disabled builds. Without it this file would
// be the sole survivor under GOOS=darwin CGO_ENABLED=0 and lint
// would flag the alias unused.

package metal

import "github.com/go-gui-org/go-gui/gui/backend/internal/gpu"

// vertex is a local alias for gpu.Vertex.
type vertex = gpu.Vertex
