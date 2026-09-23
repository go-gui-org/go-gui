package sysbeep

import (
	"sync"
	"testing"
)

// TestPlayEventConcurrent pins that PlayEvent is safe from any goroutine.
// On darwin the C-string cache was a plain array write; two first calls
// for the same cue raced. sync.Once per event serializes the init.
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
