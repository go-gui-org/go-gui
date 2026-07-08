//go:build !windows && !linux && !js

package gl

import (
	"github.com/go-gui-org/go-gui/gui"
	"github.com/jezek/xgb"
)

type platformState struct{}

func (p *platformState) makeCurrent()                  {}
func (p *platformState) swap()                         {}
func (p *platformState) drawableSize() (int32, int32)  { return 0, 0 }
func (p *platformState) dpiScale() float32             { return 1 }
func (p *platformState) setCursor(_ gui.MouseCursor)   {}
func (p *platformState) wake()                         {}
func (p *platformState) destroy()                      {}
func (p *platformState) pumpEvents(_ chan<- xgb.Event) {}
func (n *nativePlatform) IMEStart()                    {}
func (n *nativePlatform) IMEStop()                     {}
func (n *nativePlatform) IMESetRect(_, _, _, _ int32)  {}

// New creates a GL backend.  This is a stub for unsupported platforms.
func New(w *gui.Window) (*Backend, error) { return nil, nil }

// Destroy releases backend resources.  This is a stub for unsupported
// platforms.
func (b *Backend) Destroy() {}

// Run starts the event loop.  This is a stub for unsupported platforms.
func (b *Backend) Run(w *gui.Window) {}
