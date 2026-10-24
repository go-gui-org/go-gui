//go:build !js && !android && !ios

package audio

import (
	"os"
	"path/filepath"
	"testing"
)

// TestDecodeBytesStreamsAreSeekable guards LoadMusicBytes for every
// format it accepts. Music.Play and FadeIn seek the decoder to 0 before
// they start, so a stream that cannot seek never plays: Play returns
// "seek music: ... not enabled" and the track is silent. MP3, OGG and
// FLAC from bytes used to go through a reader with no Seek, which the
// decoders take as "seeking disabled". Only WAV worked.
//
// The fixtures are 0.25 s 440 Hz tones made with ffmpeg.
func TestDecodeBytesStreamsAreSeekable(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"tone.flac", "tone.ogg", "tone.mp3"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			data, err := os.ReadFile(filepath.Join("testdata", name))
			if err != nil {
				t.Fatal(err)
			}
			stream, _, err := decodeBytes(data)
			if err != nil {
				t.Fatalf("decodeBytes: %v", err)
			}
			defer stream.Close()

			// Read some samples first, as playback would, then rewind:
			// that is the loop path, not only the first Play.
			buf := make([][2]float64, 512)
			if n, _ := stream.Stream(buf); n == 0 {
				t.Fatalf("stream produced no samples")
			}
			if err := stream.Seek(0); err != nil {
				t.Fatalf("Seek(0): %v", err)
			}
			if p := stream.Position(); p != 0 {
				t.Fatalf("Position after Seek(0) = %d, want 0", p)
			}
		})
	}
}
