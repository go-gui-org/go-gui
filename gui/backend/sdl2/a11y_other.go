//go:build !linux && !js

package sdl2

import "github.com/go-gui-org/go-gui/gui"

func (n *nativePlatform) A11yInit(_ func(action, index int))  {}
func (n *nativePlatform) A11ySync(_ []gui.A11yNode, _, _ int) {}
func (n *nativePlatform) A11yDestroy()                        {}
func (n *nativePlatform) A11yAnnounce(_ string)               {}
