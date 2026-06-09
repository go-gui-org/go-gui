package gui

import "sync/atomic"

// GridAbortSignal communicates cancellation via an atomic bool.
type GridAbortSignal struct {
	aborted atomic.Bool
}

// IsAborted reports cancellation status.
func (s *GridAbortSignal) IsAborted() bool {
	if s == nil {
		return false
	}
	return s.aborted.Load()
}

// GridAbortController manages an abort signal.
type GridAbortController struct {
	Signal *GridAbortSignal
}

// NewGridAbortController allocates a fresh abort controller.
func NewGridAbortController() *GridAbortController {
	return &GridAbortController{Signal: &GridAbortSignal{}}
}

// Abort marks the request as cancelled.
func (c *GridAbortController) Abort() {
	if c.Signal == nil {
		return
	}
	c.Signal.aborted.Store(true)
}
