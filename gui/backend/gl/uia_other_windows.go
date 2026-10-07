//go:build windows && !js && !amd64 && !arm64

package gl

import "github.com/go-gui-org/go-gui/gui"

// The UI Automation provider needs the assembly thunks of amd64 and
// arm64 (uia_*.s). Other Windows architectures get no accessibility.
type uiaProvider struct{}

func newUIAProvider(uintptr, func(action, index int)) *uiaProvider { return nil }

func (*uiaProvider) getObject(uintptr, uintptr) (uintptr, bool) { return 0, false }
func (*uiaProvider) sync([]gui.A11yNode, int, float32)          {}
func (*uiaProvider) destroy()                                   {}
func (*uiaProvider) announce(string)                            {}
