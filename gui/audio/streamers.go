//go:build !js && !android && !ios

package audio

import (
	"sync"

	"github.com/gopxl/beep/v2"
)

// ---------------------------------------------------------------------------
// volumeStreamer
// ---------------------------------------------------------------------------

// volumeStreamer wraps a [beep.Streamer] and multiplies each sample by
// a volume factor returned by getVolume, clamped to [0, 1].
type volumeStreamer struct {
	streamer  beep.Streamer
	getVolume func() float64
}

func (v *volumeStreamer) Stream(samples [][2]float64) (n int, ok bool) {
	n, ok = v.streamer.Stream(samples)
	vol := clamp01(v.getVolume())
	if vol < 1 {
		for i := range n {
			samples[i][0] *= vol
			samples[i][1] *= vol
		}
	}
	return
}

func (v *volumeStreamer) Err() error { return v.streamer.Err() }

// ---------------------------------------------------------------------------
// fadeStreamer
// ---------------------------------------------------------------------------

// fadeStreamer wraps a [beep.Streamer] and ramps the volume from
// startVol to targetVol over a duration.  After the ramp completes:
//   - for fade-in (targetVol > 0), the streamer becomes transparent.
//   - for fade-out (targetVol == 0), the streamer drains and calls
//     onComplete (which must not acquire speaker or channel locks).
type fadeStreamer struct {
	streamer   beep.Streamer
	sampleRate beep.SampleRate
	startVol   float64
	targetVol  float64
	endSamples int
	elapsed    int
	done       bool
	onComplete func()
}

func (f *fadeStreamer) Stream(samples [][2]float64) (n int, ok bool) {
	if f.done || f.endSamples <= 0 {
		if f.targetVol == 0 {
			if f.onComplete != nil {
				f.onComplete()
			}
			return 0, false
		}
		return f.streamer.Stream(samples)
	}
	n, ok = f.streamer.Stream(samples)
	if n == 0 && !ok {
		return 0, false
	}
	for i := range n {
		t := min(float64(f.elapsed)/float64(f.endSamples), 1)
		vol := f.startVol + (f.targetVol-f.startVol)*t
		samples[i][0] *= vol
		samples[i][1] *= vol
		f.elapsed++
	}
	if f.elapsed >= f.endSamples {
		f.done = true
		if f.targetVol == 0 {
			if f.onComplete != nil {
				f.onComplete()
			}
			return n, false
		}
	}
	return n, ok
}

func (f *fadeStreamer) Err() error { return f.streamer.Err() }

// ---------------------------------------------------------------------------
// channelMixer
// ---------------------------------------------------------------------------

// channelMixer provides N numbered sound-effect channels backed by a
// single custom [beep.Streamer].  Each channel wraps a [beep.Ctrl] so
// it can be paused/resumed/replaced without allocations.
type channelMixer struct {
	chans        []*beep.Ctrl
	playing      []bool
	masterVolume *float64
	mu           sync.Mutex
}

func newChannelMixer(n int, master *float64) *channelMixer {
	chans := make([]*beep.Ctrl, n)
	for i := range chans {
		chans[i] = &beep.Ctrl{}
	}
	return &channelMixer{
		chans:        chans,
		playing:      make([]bool, n),
		masterVolume: master,
	}
}

func (cm *channelMixer) Stream(samples [][2]float64) (n int, ok bool) {
	var tmp [512][2]float64
	for len(samples) > 0 {
		toStream := min(len(tmp), len(samples))
		clear(samples[:toStream])
		active := false
		cm.mu.Lock()
		for i := range cm.chans {
			if cm.playing[i] {
				sn, sok := cm.chans[i].Stream(tmp[:toStream])
				if sn == 0 && !sok {
					cm.chans[i].Streamer = nil
					cm.playing[i] = false
				} else {
					for j := range sn {
						samples[j][0] += tmp[j][0]
						samples[j][1] += tmp[j][1]
					}
					active = true
				}
			}
		}
		cm.mu.Unlock()
		if !active {
			for i := range toStream {
				samples[i][0] = 0
				samples[i][1] = 0
			}
		} else if *cm.masterVolume < 1 {
			for i := range toStream {
				samples[i][0] *= *cm.masterVolume
				samples[i][1] *= *cm.masterVolume
			}
		}
		samples = samples[toStream:]
		n += toStream
	}
	return n, true
}

func (cm *channelMixer) Err() error { return nil }

func (cm *channelMixer) firstFree() int {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	for i := range cm.chans {
		if !cm.playing[i] {
			return i
		}
	}
	return -1
}

func (cm *channelMixer) set(ch int, s beep.Streamer) {
	cm.mu.Lock()
	cm.chans[ch].Streamer = s
	cm.chans[ch].Paused = false
	cm.playing[ch] = true
	cm.mu.Unlock()
}

func (cm *channelMixer) halt(channel int) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	if channel < 0 {
		for i := range cm.chans {
			cm.chans[i].Streamer = nil
			cm.playing[i] = false
		}
		return
	}
	if channel < len(cm.chans) {
		cm.chans[channel].Streamer = nil
		cm.playing[channel] = false
	}
}

func (cm *channelMixer) pause(channel int) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	if channel < 0 {
		for i := range cm.chans {
			cm.chans[i].Paused = true
		}
		return
	}
	if channel < len(cm.chans) {
		cm.chans[channel].Paused = true
	}
}

func (cm *channelMixer) resume(channel int) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	if channel < 0 {
		for i := range cm.chans {
			cm.chans[i].Paused = false
		}
		return
	}
	if channel < len(cm.chans) {
		cm.chans[channel].Paused = false
	}
}

func (cm *channelMixer) isPlaying(channel int) bool {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	if channel < 0 || channel >= len(cm.chans) {
		return false
	}
	return cm.playing[channel]
}

func (cm *channelMixer) numChannels() int { return len(cm.chans) }

// ---------------------------------------------------------------------------
// sourceStreamer
// ---------------------------------------------------------------------------

// sourceStreamer adapts a [Source] to beep.Streamer.  It holds the
// source by reference and forwards Fill verbatim — the beep backend
// path is exactly one interface hop and zero allocations per call.
type sourceStreamer struct {
	src Source
}

func (s *sourceStreamer) Stream(samples [][2]float64) (int, bool) {
	return s.src.Fill(samples)
}

func (s *sourceStreamer) Err() error { return nil }

// ---------------------------------------------------------------------------
// neverDrain
// ---------------------------------------------------------------------------

// neverDrain wraps a [beep.Streamer] so it never appears drained.
// When the inner streamer returns 0,false, the buffer is filled with
// silence and ok=true is returned.  This prevents the speaker mixer
// from auto-removing the streamer.
type neverDrain struct {
	streamer beep.Streamer
}

func (n *neverDrain) Stream(samples [][2]float64) (int, bool) {
	if n.streamer == nil {
		clear(samples)
		return len(samples), true
	}
	nn, ok := n.streamer.Stream(samples)
	if !ok {
		clear(samples[nn:])
		return len(samples), true
	}
	return nn, ok
}

func (n *neverDrain) Err() error {
	if n.streamer == nil {
		return nil
	}
	return n.streamer.Err()
}
