//go:build !linux

package ibus

// Kind classifies an event produced by the input method.
type kind int

const (
	// KindPreedit carries an updated (possibly empty) preedit string.
	KindPreedit kind = iota
	// KindCommit carries text the input method has committed.
	KindCommit
	// KindForwardKey carries a key the engine declined after all.
	KindForwardKey
)

// Event is one input-method result, queued for the main thread.
type Event struct {
	Text                   string
	Cursor, SelLen         int32
	Keyval, Keycode, State uint32
	Kind                   kind
}

// Client is an IBus input context. IBus is Linux-only, so this build
// has none: New always returns nil and every method is inert.
type Client struct{}

// New reports that no input method is available.
func New(_ string, _ func()) *Client { return nil }

// ProcessKey never consumes a key on this platform.
func (c *Client) ProcessKey(_, _, _ uint32) bool { return false }

// ProcessKeyRelease does nothing on this platform.
func (c *Client) ProcessKeyRelease(_, _, _ uint32) {}

// FocusIn does nothing on this platform.
func (c *Client) FocusIn() {}

// FocusOut does nothing on this platform.
func (c *Client) FocusOut() {}

// SetCursorLocation does nothing on this platform.
func (c *Client) SetCursorLocation(_, _, _, _ int32) {}

// Close does nothing on this platform.
func (c *Client) Close() {}

// Drain returns dst unchanged on this platform.
func (c *Client) Drain(dst []Event) []Event { return dst }
