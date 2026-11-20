package sysbeep

import (
	"sync"
	"testing"
)

// TestPlayEventConcurrent pins that PlayEvent is safe from any goroutine.
// On darwin the sound cache is shared state; two first calls
// for the same cue must not race. A mutex guards it.
func TestPlayEventConcurrent(t *testing.T) {
	const workers = 8
	var start, done sync.WaitGroup
	start.Add(1)
	for range workers {
		done.Go(func() {
			start.Wait()
			for e := range int(eventCount) {
				PlayEvent(Event(e))
			}
			PlayEvent(eventCount)
		})
	}
	start.Done()
	done.Wait()
}
