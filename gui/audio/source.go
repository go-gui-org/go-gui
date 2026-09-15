//go:build !js && !android && !ios

package audio

import "errors"

// Source is a live audio source: a callback that fills a sample buffer.
//
// Fill is called on the audio thread at buffer rate.  It must not
// allocate or block, and it must write stereo samples into samples
// (one entry per output frame, channel 0 = left, channel 1 = right).
// Returning n < len(samples) is allowed; returning ok = false ends the
// source and halts its channel (a note-off voice that has finished its
// release envelope returns (0, false)).
//
// Fill must not call any other function in this package except
// [SampleRate].  The audio thread runs Fill while it holds the mixer
// lock, and most other calls take the init lock and then the mixer lock,
// so a call from Fill can deadlock against a Play on another goroutine,
// or hang quit, which waits for the audio thread to stop.
//
// The signature matches beep.Streamer exactly, so the beep backend is a
// zero-cost adapter; the interface keeps beep out of the public API and
// leaves other backends room to diverge.
// exportaudit:keep — app code implements it; nothing in-repo names the
// type (the showcase passes its voices to PlaySource).
type Source interface {
	Fill(samples [][2]float64) (n int, ok bool)
}

// PlaySource starts a live [Source] on the given channel (-1 selects
// the first free channel).  The channel is yours: [HaltChannel] stops
// the source early, and the source itself ends playback when Fill
// returns ok = false.
//
// Pass channel -1 to auto-select the first free channel.  Returns an
// error when audio is not [Init]ialized, the source is nil, or no
// channel is available.
func PlaySource(channel int, s Source) error {
	// Hold initMu through the backend call so a concurrent quit cannot tear
	// the mixer down between the check and the channel write.
	initMu.Lock()
	defer initMu.Unlock()
	if !initialized {
		return errNotInitialized
	}
	if s == nil {
		return errors.New("audio: nil source")
	}
	return backend.PlaySource(channel, s)
}

// SampleRate returns the output sample rate configured at [Init],
// in Hz.  Synthesis code reads it once when building a source to
// compute phase increments.  Returns 0 before [Init].
func SampleRate() int {
	return backend.SampleRate()
}
