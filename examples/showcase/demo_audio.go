//go:build audio && !js && !android && !ios

package main

import (
	"encoding/binary"
	"fmt"
	"math"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/audio"
)

// Cached sounds — allocated once, reused across clicks.
// Freed implicitly when the process exits (SDL_mixer cleanup).
var (
	beepSound *audio.Sound
	highSound *audio.Sound
)

func demoAudio(w *gui.Window) gui.View {
	t := gui.CurrentTheme()
	app := gui.State[ShowcaseApp](w)

	return gui.Column(gui.ContainerCfg{
		Sizing:  gui.FillFit,
		Spacing: gui.SomeF(16),
		Padding: gui.NoPadding,
		Content: []gui.View{
			sectionLabel(t, "Sound Effects"),
			gui.Row(gui.ContainerCfg{
				Sizing:  gui.FillFit,
				Spacing: gui.SomeF(8),
				Padding: gui.NoPadding,
				Content: []gui.View{
					gui.Button(gui.ButtonCfg{
						ID:      "btn-beep",
						Padding: gui.SomeP(8, 16, 8, 16),
						Content: []gui.View{
							gui.Text(gui.TextCfg{
								Text:      gui.IconPlay,
								TextStyle: t.Icon3,
							}),
							gui.Text(gui.TextCfg{
								Text:      "Play Beep",
								TextStyle: t.N3,
							}),
						},
						OnClick: func(_ *gui.Layout, e *gui.Event,
							w *gui.Window) {
							playBeep(w)
							e.IsHandled = true
						},
					}),
					gui.Button(gui.ButtonCfg{
						ID:      "btn-beep-high",
						Padding: gui.SomeP(8, 16, 8, 16),
						Content: []gui.View{
							gui.Text(gui.TextCfg{
								Text:      gui.IconPlay,
								TextStyle: t.Icon3,
							}),
							gui.Text(gui.TextCfg{
								Text:      "Play High Tone",
								TextStyle: t.N3,
							}),
						},
						OnClick: func(_ *gui.Layout, e *gui.Event,
							w *gui.Window) {
							playHighTone(w)
							e.IsHandled = true
						},
					}),
				},
			}),

			sectionLabel(t, "Volume"),
			gui.Row(gui.ContainerCfg{
				Sizing:  gui.FillFit,
				Spacing: gui.SomeF(8),
				Padding: gui.NoPadding,
				VAlign:  gui.VAlignMiddle,
				Content: []gui.View{
					gui.Slider(gui.SliderCfg{
						ID:     "audio-vol",
						Value:  app.AudioVolume * 100,
						Min:    0,
						Max:    100,
						Sizing: gui.FillFit,
						OnChange: func(v float32, _ *gui.Event,
							w *gui.Window) {
							a := gui.State[ShowcaseApp](w)
							a.AudioVolume = v / 100
							audio.SetMasterVolume(a.AudioVolume)
						},
					}),
					gui.Text(gui.TextCfg{
						Text: fmt.Sprintf("%.0f%%",
							app.AudioVolume*100),
						TextStyle: t.N4,
						MinWidth:  40,
					}),
				},
			}),

			gui.Text(gui.TextCfg{
				Text:      app.AudioStatus,
				TextStyle: t.N4,
			}),
		},
	})
}

// ensureAudioInit lazily initializes the audio subsystem.
func ensureAudioInit(w *gui.Window) bool {
	app := gui.State[ShowcaseApp](w)
	if app.AudioReady {
		return true
	}
	if err := audio.Init(); err != nil {
		app.AudioStatus = "Error: " + err.Error()
		return false
	}
	audio.SetMasterVolume(app.AudioVolume)
	app.AudioReady = true
	app.AudioStatus = "Audio initialized"
	return true
}

func playBeep(w *gui.Window) {
	if !ensureAudioInit(w) {
		return
	}
	app := gui.State[ShowcaseApp](w)
	if beepSound == nil {
		wav := generateWAV(440, 0.25, 44100)
		snd, err := audio.LoadSoundBytes(wav)
		if err != nil {
			app.AudioStatus = "Load error: " + err.Error()
			return
		}
		beepSound = snd
	}
	if _, err := beepSound.PlayOnce(); err != nil {
		app.AudioStatus = "Play error: " + err.Error()
		return
	}
	app.AudioStatus = "Playing 440 Hz beep"
}

func playHighTone(w *gui.Window) {
	if !ensureAudioInit(w) {
		return
	}
	app := gui.State[ShowcaseApp](w)
	if highSound == nil {
		wav := generateWAV(880, 0.25, 44100)
		snd, err := audio.LoadSoundBytes(wav)
		if err != nil {
			app.AudioStatus = "Load error: " + err.Error()
			return
		}
		highSound = snd
	}
	if _, err := highSound.PlayOnce(); err != nil {
		app.AudioStatus = "Play error: " + err.Error()
		return
	}
	app.AudioStatus = "Playing 880 Hz tone"
}

// generateWAV creates a mono 16-bit PCM WAV with a sine tone.
func generateWAV(freq, seconds float64, sampleRate int) []byte {
	n := int(seconds * float64(sampleRate))
	dataSize := n * 2
	buf := make([]byte, 44+dataSize)

	// RIFF header
	copy(buf[0:4], "RIFF")
	binary.LittleEndian.PutUint32(buf[4:8], uint32(36+dataSize))
	copy(buf[8:12], "WAVE")

	// fmt sub-chunk
	copy(buf[12:16], "fmt ")
	binary.LittleEndian.PutUint32(buf[16:20], 16)
	binary.LittleEndian.PutUint16(buf[20:22], 1) // PCM
	binary.LittleEndian.PutUint16(buf[22:24], 1) // mono
	binary.LittleEndian.PutUint32(buf[24:28], uint32(sampleRate))
	binary.LittleEndian.PutUint32(buf[28:32], uint32(sampleRate*2))
	binary.LittleEndian.PutUint16(buf[32:34], 2)  // block align
	binary.LittleEndian.PutUint16(buf[34:36], 16) // bits/sample

	// data sub-chunk
	copy(buf[36:40], "data")
	binary.LittleEndian.PutUint32(buf[40:44], uint32(dataSize))

	omega := 2 * math.Pi * freq / float64(sampleRate)
	for i := range n {
		sample := int16(math.Sin(omega*float64(i)) * 0.5 * 32767)
		binary.LittleEndian.PutUint16(
			buf[44+i*2:46+i*2], uint16(sample))
	}
	return buf
}
