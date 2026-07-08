//go:build !js && !android && !ios

package main

import (
	"encoding/binary"
	"fmt"
	"math"
	"os"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/audio"
)

var (
	beepSound  *audio.Sound
	highSound  *audio.Sound
	musicTrack *audio.Music
	musicPath  string
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
					gui.Button(gui.ButtonCfg{
						ID:      "btn-fade-in",
						Padding: gui.SomeP(8, 16, 8, 16),
						Content: []gui.View{
							gui.Text(gui.TextCfg{
								Text:      gui.IconPlay,
								TextStyle: t.Icon3,
							}),
							gui.Text(gui.TextCfg{
								Text:      "Fade In Beep (1s)",
								TextStyle: t.N3,
							}),
						},
						OnClick: func(_ *gui.Layout, e *gui.Event,
							w *gui.Window) {
							fadeInBeep(w)
							e.IsHandled = true
						},
					}),
				},
			}),

			sectionLabel(t, "Multi-Channel"),
			gui.Row(gui.ContainerCfg{
				Sizing:  gui.FillFit,
				Spacing: gui.SomeF(8),
				Padding: gui.NoPadding,
				Content: []gui.View{
					gui.Button(gui.ButtonCfg{
						ID:      "btn-ch0",
						Padding: gui.SomeP(8, 16, 8, 16),
						Content: []gui.View{
							gui.Text(gui.TextCfg{
								Text:      gui.IconPlay,
								TextStyle: t.Icon3,
							}),
							gui.Text(gui.TextCfg{
								Text:      "Play on Ch 0",
								TextStyle: t.N3,
							}),
						},
						OnClick: func(_ *gui.Layout, e *gui.Event,
							w *gui.Window) {
							playOnChannel(w, 0)
							e.IsHandled = true
						},
					}),
					gui.Button(gui.ButtonCfg{
						ID:      "btn-ch1",
						Padding: gui.SomeP(8, 16, 8, 16),
						Content: []gui.View{
							gui.Text(gui.TextCfg{
								Text:      gui.IconPlay,
								TextStyle: t.Icon3,
							}),
							gui.Text(gui.TextCfg{
								Text:      "Play on Ch 1",
								TextStyle: t.N3,
							}),
						},
						OnClick: func(_ *gui.Layout, e *gui.Event,
							w *gui.Window) {
							playOnChannel(w, 1)
							e.IsHandled = true
						},
					}),
					gui.Button(gui.ButtonCfg{
						ID:      "btn-halt-ch0",
						Padding: gui.SomeP(8, 16, 8, 16),
						Content: []gui.View{
							gui.Text(gui.TextCfg{
								Text:      gui.IconStop,
								TextStyle: t.Icon3,
							}),
							gui.Text(gui.TextCfg{
								Text:      "Halt Ch 0",
								TextStyle: t.N3,
							}),
						},
						OnClick: func(_ *gui.Layout, e *gui.Event,
							w *gui.Window) {
							audio.HaltChannel(0)
							app := gui.State[ShowcaseApp](w)
							app.AudioStatus = "Ch 0 halted"
							e.IsHandled = true
						},
					}),
				},
			}),

			sectionLabel(t, "Music"),
			gui.Row(gui.ContainerCfg{
				Sizing:  gui.FillFit,
				Spacing: gui.SomeF(8),
				Padding: gui.NoPadding,
				Content: []gui.View{
					gui.Button(gui.ButtonCfg{
						ID:      "btn-load-music",
						Padding: gui.SomeP(8, 16, 8, 16),
						Content: []gui.View{
							gui.Text(gui.TextCfg{
								Text:      gui.IconFolder,
								TextStyle: t.Icon3,
							}),
							gui.Text(gui.TextCfg{
								Text:      "Load Music",
								TextStyle: t.N3,
							}),
						},
						OnClick: func(_ *gui.Layout, e *gui.Event,
							w *gui.Window) {
							loadMusicDemo(w)
							e.IsHandled = true
						},
					}),
					gui.Button(gui.ButtonCfg{
						ID:      "btn-play-music",
						Padding: gui.SomeP(8, 16, 8, 16),
						Content: []gui.View{
							gui.Text(gui.TextCfg{
								Text:      gui.IconPlay,
								TextStyle: t.Icon3,
							}),
							gui.Text(gui.TextCfg{
								Text:      "Play Music",
								TextStyle: t.N3,
							}),
						},
						OnClick: func(_ *gui.Layout, e *gui.Event,
							w *gui.Window) {
							playMusic(w)
							e.IsHandled = true
						},
					}),
					gui.Button(gui.ButtonCfg{
						ID:      "btn-fadeout-music",
						Padding: gui.SomeP(8, 16, 8, 16),
						Content: []gui.View{
							gui.Text(gui.TextCfg{
								Text:      gui.IconStop,
								TextStyle: t.Icon3,
							}),
							gui.Text(gui.TextCfg{
								Text:      "Fade Out (1s)",
								TextStyle: t.N3,
							}),
						},
						OnClick: func(_ *gui.Layout, e *gui.Event,
							w *gui.Window) {
							fadeOutMusic(w)
							e.IsHandled = true
						},
					}),
					gui.Button(gui.ButtonCfg{
						ID:      "btn-halt-music",
						Padding: gui.SomeP(8, 16, 8, 16),
						Content: []gui.View{
							gui.Text(gui.TextCfg{
								Text:      gui.IconStop,
								TextStyle: t.Icon3,
							}),
							gui.Text(gui.TextCfg{
								Text:      "Stop",
								TextStyle: t.N3,
							}),
						},
						OnClick: func(_ *gui.Layout, e *gui.Event,
							w *gui.Window) {
							stopMusic(w)
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
						Value:  float32(app.AudioVolume * 100),
						Min:    0,
						Max:    100,
						Sizing: gui.FillFit,
						OnChange: func(v float32, _ *gui.Event,
							w *gui.Window) {
							a := gui.State[ShowcaseApp](w)
							a.AudioVolume = float64(v) / 100
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
				Mode:      gui.TextModeWrap,
			}),
		},
	})
}

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

// --- sound effects ---

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

func fadeInBeep(w *gui.Window) {
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
	if _, err := beepSound.FadeIn(-1, 0, 1000); err != nil {
		app.AudioStatus = "Fade error: " + err.Error()
		return
	}
	app.AudioStatus = "Beep fading in (1s)"
}

func playOnChannel(w *gui.Window, channel int) {
	if !ensureAudioInit(w) {
		return
	}
	app := gui.State[ShowcaseApp](w)
	if highSound == nil {
		wav := generateWAV(880, 0.5, 44100)
		snd, err := audio.LoadSoundBytes(wav)
		if err != nil {
			app.AudioStatus = "Load error: " + err.Error()
			return
		}
		highSound = snd
	}
	ch, err := highSound.Play(channel, 0)
	if err != nil {
		app.AudioStatus = "Channel play error: " + err.Error()
		return
	}
	app.AudioStatus = fmt.Sprintf("Playing on ch %d", ch)
}

// --- music ---

func loadMusicDemo(w *gui.Window) {
	if !ensureAudioInit(w) {
		return
	}
	app := gui.State[ShowcaseApp](w)
	cleanupMusic(w)
	wav := generateWAV(660, 3, 44100)
	tmp, err := os.CreateTemp("", "showcase-audio-*.wav")
	if err != nil {
		app.AudioStatus = "Temp file error: " + err.Error()
		return
	}
	if _, err := tmp.Write(wav); err != nil {
		_ = tmp.Close()
		app.AudioStatus = "Write error: " + err.Error()
		return
	}
	_ = tmp.Close()
	musicPath = tmp.Name()
	track, err := audio.LoadMusic(musicPath)
	if err != nil {
		app.AudioStatus = "Load music error: " + err.Error()
		return
	}
	musicTrack = track
	app.AudioMusicLoaded = true
	app.AudioStatus = "Music loaded (3s 660 Hz tone)"
}

func playMusic(w *gui.Window) {
	app := gui.State[ShowcaseApp](w)
	if musicTrack == nil {
		app.AudioStatus = "Load music first"
		return
	}
	if err := musicTrack.Play(-1); err != nil {
		app.AudioStatus = "Play music error: " + err.Error()
		return
	}
	app.AudioMusicPlaying = true
	app.AudioMusicPaused = false
	app.AudioStatus = "Music playing (loop)"
}

func fadeOutMusic(w *gui.Window) {
	app := gui.State[ShowcaseApp](w)
	if !app.AudioMusicPlaying {
		app.AudioStatus = "No music to fade out"
		return
	}
	audio.FadeOutMusic(1000)
	app.AudioMusicPlaying = false
	app.AudioStatus = "Music fading out (1s)"
}

func stopMusic(w *gui.Window) {
	audio.HaltMusic()
	app := gui.State[ShowcaseApp](w)
	app.AudioMusicPlaying = false
	app.AudioMusicPaused = false
	app.AudioStatus = "Music stopped"
}

func cleanupMusic(w *gui.Window) {
	audio.HaltMusic()
	if musicTrack != nil {
		musicTrack.Free()
		musicTrack = nil
	}
	if musicPath != "" {
		_ = os.Remove(musicPath)
		musicPath = ""
	}
	app := gui.State[ShowcaseApp](w)
	app.AudioMusicLoaded = false
	app.AudioMusicPlaying = false
}

// generateWAV creates a mono 16-bit PCM WAV with a sine tone.
// freq and seconds are clamped to safe ranges to prevent overflow.
func generateWAV(freq, seconds float64, sampleRate int) []byte {
	const maxSeconds = 300
	if sampleRate <= 0 {
		sampleRate = 44100
	}
	if sampleRate > 192000 {
		sampleRate = 192000
	}
	if math.IsNaN(seconds) || math.IsInf(seconds, 0) || seconds < 0 {
		seconds = 0
	}
	seconds = min(seconds, maxSeconds)

	n := int(seconds * float64(sampleRate))
	const maxSamples = 60 * 192000 // 60 seconds at max rate
	if n > maxSamples {
		n = maxSamples
	}
	dataSize := n * 2
	buf := make([]byte, 44+dataSize)

	copy(buf[0:4], "RIFF")
	binary.LittleEndian.PutUint32(buf[4:8], uint32(36+dataSize))
	copy(buf[8:12], "WAVE")

	copy(buf[12:16], "fmt ")
	binary.LittleEndian.PutUint32(buf[16:20], 16)
	binary.LittleEndian.PutUint16(buf[20:22], 1)
	binary.LittleEndian.PutUint16(buf[22:24], 1)
	binary.LittleEndian.PutUint32(buf[24:28], uint32(sampleRate))
	binary.LittleEndian.PutUint32(buf[28:32], uint32(sampleRate*2))
	binary.LittleEndian.PutUint16(buf[32:34], 2)
	binary.LittleEndian.PutUint16(buf[34:36], 16)

	copy(buf[36:40], "data")
	binary.LittleEndian.PutUint32(buf[40:44], uint32(dataSize))

	if math.IsNaN(freq) || math.IsInf(freq, 0) {
		freq = 0
	}

	omega := 2 * math.Pi * freq / float64(sampleRate)
	for i := range n {
		sample := int16(math.Sin(omega*float64(i)) * 0.5 * 32767)
		binary.LittleEndian.PutUint16(
			buf[44+i*2:46+i*2], uint16(sample))
	}
	return buf
}
