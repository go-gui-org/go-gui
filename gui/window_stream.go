package gui

// Stream bridges a background producer to the window: every value
// received on ch is applied on the frame thread and followed by a
// full layout refresh, so the frame the value belongs to is painted
// promptly instead of whenever the next input event happens to
// arrive.
//
// The motivation is the streaming shape — a goroutine reading
// stdin, a socket, or a ticker channel and showing each line in the
// window. Without a scheduled frame the backend idles (issue #559):
// the producer's output piles up invisibly until a mouse move wakes
// the loop, and then all of it appears at once. Stream schedules one
// frame per value through the same QueueCommand + InvalidateLayout path
// the sampler examples already use by hand.
//
// Call it once per producer, typically from OnInit after
// UpdateView has registered the view. Safe to call from any
// window lock itself — the mutation runs inside a queued command on
// the frame thread. apply therefore reads window state the way a
// view function does, and must not block: a slow apply stalls every
// frame behind it.
//
// Delivery is per item, in channel order (the channel preserves it
// and the command queue is FIFO), with no coalescing: a producer
// that outruns the frame rate grows the command queue, so bound the
// cadence at the producer — a buffer of 1 plus a 100 ms tick, say —
// rather than here. Two streams writing the same state interleave
// nondeterministically, the same rule animations follow.
//
// The goroutine exits when ch is closed or the window's lifecycle
// context ends, whichever comes first, and then closes the returned
// channel. A nil window, nil channel, or nil apply spawns nothing
// and answers an already-closed channel.
func Stream[T any](
	w *Window,
	ch <-chan T,
	apply func(*Window, T),
) <-chan struct{} {
	done := make(chan struct{})
	if w == nil || ch == nil || apply == nil {
		close(done)
		return done
	}
	go func() {
		defer close(done)
		for {
			select {
			case <-w.Ctx().Done():
				return
			case v, ok := <-ch:
				if !ok {
					return
				}
				// Per-iteration binding: the queued command may
				// run frames later, after v has been reassigned.
				value := v
				w.QueueCommand(func(w *Window) {
					apply(w, value)
					w.InvalidateLayout()
				})
			}
		}
	}()
	return done
}
