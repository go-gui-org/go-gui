//go:build !windows && !linux && !js

package gl

import (
	"errors"
	"fmt"
)

// SetKeepAwake does nothing here: the GL backend runs on Windows and
// Linux, which have their own.
func (n *nativePlatform) SetKeepAwake(_ bool, _, _ string) error {
	return fmt.Errorf("not supported on this platform: %w",
		errors.ErrUnsupported)
}
