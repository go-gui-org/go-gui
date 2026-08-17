//go:build !js && !android && !ios

package main

import (
	"fmt"
	"math"
	"sync/atomic"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/audio"
)

var musicTrack *audio.Music

func demoAudio(w *gui.Window) gui.View {
	t := gui.CurrentTheme()
	app := appState(w)

	return gui.Column(gui.ContainerCfg{
		Sizing:  gui.FillFit,
		Spacing: gui.SomeF(16),
		Padding: gui.NoPadding,
		Content: []gui.View{
			sectionLabel(t, "Live Synthesis"),
			gui.Column(gui.ContainerCfg{
				Sizing:  gui.FillFit,
				Spacing: gui.SomeF(8),
				Padding: gui.NoPadding,
				Content: []gui.View{
					synthPadGrid(t),
					gui.Button(gui.ButtonCfg{
						ID:      "btn-halt-all",
						Padding: gui.NewPadding(8, 16, 8, 16),
						Content: []gui.View{
							gui.Text(gui.TextCfg{
								Text:      gui.IconStop,
								TextStyle: t.Icon3,
							}),
							gui.Text(gui.TextCfg{
								Text:      "Halt All",
								TextStyle: t.N3,
							}),
						},
						OnClick: func(ctx gui.EventCtx) {
							haltAllSounds(ctx.Window)
						},
					}),
					gui.Text(gui.TextCfg{
						Text: "Press a pad to start a voice; release to let it decay. " +
							"Press and drag off without releasing and the voice keeps playing — " +
							"Halt All stops everything.",
						TextStyle: t.N4,
						Mode:      gui.TextModeWrap,
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
						Padding: gui.NewPadding(8, 16, 8, 16),
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
						OnClick: func(ctx gui.EventCtx) {
							loadMusicDemo(ctx.Window)
						},
					}),
					gui.Button(gui.ButtonCfg{
						ID:      "btn-play-music",
						Padding: gui.NewPadding(8, 16, 8, 16),
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
						OnClick: func(ctx gui.EventCtx) {
							playMusic(ctx.Window)
						},
					}),
					gui.Button(gui.ButtonCfg{
						ID:      "btn-fadeout-music",
						Padding: gui.NewPadding(8, 16, 8, 16),
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
						OnClick: func(ctx gui.EventCtx) {
							fadeOutMusic(ctx.Window)
						},
					}),
					gui.Button(gui.ButtonCfg{
						ID:      "btn-halt-music",
						Padding: gui.NewPadding(8, 16, 8, 16),
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
						OnClick: func(ctx gui.EventCtx) {
							stopMusic(ctx.Window)
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
						OnChange: func(v float32, ctx gui.EventCtx) {
							a := appState(ctx.Window)
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

// ---------------------------------------------------------------------------
// Live synthesis: a pad grid of streaming audio sources
//
// Each pad press creates a synthVoice and hands it to audio.PlaySource,
// which streams it on a free mixer channel; the release starts the
// voice's note-off envelope, and the voice ends itself by returning
// ok = false once the envelope completes.
// ---------------------------------------------------------------------------

// synthPadDef describes one pad: its note name and frequency.
type synthPadDef struct {
	id   string
	name string
	freq float64
}

// synthPadsPerRow is the pad grid width; synthPadRows holds the notes
// and synthActive one live voice per slot (row*synthPadsPerRow+col).
const synthPadsPerRow = 4

// synthPadRows is the pad grid, C4 major pentatonic in two rows of
// four.
var synthPadRows = [2][]synthPadDef{
	{
		{id: "pad-c4", name: "C4", freq: 261.63},
		{id: "pad-d4", name: "D4", freq: 293.66},
		{id: "pad-e4", name: "E4", freq: 329.63},
		{id: "pad-g4", name: "G4", freq: 392.00},
	},
	{
		{id: "pad-a4", name: "A4", freq: 440.00},
		{id: "pad-c5", name: "C5", freq: 523.25},
		{id: "pad-d5", name: "D5", freq: 587.33},
		{id: "pad-e5", name: "E5", freq: 659.25},
	},
}

// synthActive holds the live voice per pad slot. It is written from
// the UI thread and read only there; the voice's own state is
// audio-thread-exclusive apart from the release flag.
var synthActive [len(synthPadRows) * synthPadsPerRow]*synthVoice

// Envelope timings in seconds.
const (
	synthAttackS  = 0.01
	synthDecayS   = 0.25
	synthReleaseS = 0.30
	synthSustain  = 0.35 // sustain level as a fraction of the peak
)

// synthVoice is a live audio source: three harmonics of the pad's
// frequency shaped by an ADSR envelope. Fill runs on the audio thread,
// so the envelope state lives entirely there; the note-off arrives via
// an atomic flag set from the UI thread.
type synthVoice struct {
	sampleRate   float64
	freq         float64
	phase        float64
	level        float64
	elapsed      float64
	releaseLevel float64 // amplitude at note-off; release ramps from it
	releasing    atomic.Bool
	done         bool
}

func newSynthVoice(freq float64) *synthVoice {
	v := &synthVoice{
		sampleRate: float64(audio.SampleRate()),
		freq:       freq,
		level:      0.2,
	}
	if v.sampleRate == 0 {
		v.sampleRate = 44100
	}
	return v
}

// release starts the note-off envelope.
func (v *synthVoice) release() { v.releasing.Store(true) }

// Fill implements audio.Source. It must not allocate or block, and it
// writes stereo samples into the caller-owned buffer.
func (v *synthVoice) Fill(samples [][2]float64) (int, bool) {
	dt := 1 / v.sampleRate
	n := 0
	for i := range samples {
		if v.done {
			break
		}
		amp := v.envelope(dt)
		v.phase += 2 * math.Pi * v.freq * dt
		// Three harmonics sum to 1.75 peak; scale so the envelope
		// level is the peak amplitude.
		wave := (math.Sin(v.phase) + 0.5*math.Sin(2*v.phase) +
			0.25*math.Sin(3*v.phase)) / 1.75
		samples[i][0] = wave * amp
		samples[i][1] = wave * amp
		n++
	}
	// Returning ok = false ends the source and frees the channel; the
	// mixer reads only the n valid samples.
	return n, !v.done
}

// envelope advances the ADSR state by dt and returns the current
// amplitude. Called once per sample from Fill.
func (v *synthVoice) envelope(dt float64) float64 {
	if v.releasing.Load() {
		if v.elapsed >= synthReleaseS {
			v.done = true
			return 0
		}
		amp := v.releaseLevel * (1 - v.elapsed/synthReleaseS)
		v.elapsed += dt
		return amp
	}
	switch {
	case v.elapsed < synthAttackS:
		amp := v.level * (v.elapsed / synthAttackS)
		v.elapsed += dt
		v.releaseLevel = amp
		return amp
	case v.elapsed < synthAttackS+synthDecayS:
		t := (v.elapsed - synthAttackS) / synthDecayS
		amp := v.level * (1 - (1-synthSustain)*t)
		v.elapsed += dt
		v.releaseLevel = amp
		return amp
	default:
		amp := v.level * synthSustain
		v.releaseLevel = amp
		return amp
	}
}

// synthNoteOn starts a new voice for pad slot i on the first free
// mixer channel, releasing the previous voice of the same pad if it is
// still decaying.
func synthNoteOn(w *gui.Window, i int, freq float64) {
	if !ensureAudioInit(w) {
		return
	}
	app := appState(w)
	if prev := synthActive[i]; prev != nil {
		prev.release()
	}
	voice := newSynthVoice(freq)
	if err := audio.PlaySource(-1, voice); err != nil {
		app.AudioStatus = "Synth error: " + err.Error()
		return
	}
	synthActive[i] = voice
	app.AudioStatus = synthPadRows[i/synthPadsPerRow][i%synthPadsPerRow].name + " on"
}

// synthNoteOff starts the note-off envelope of the pad's voice. A
// release over a pad with no live voice (already released, or after
// Halt All) is a silent no-op.
func synthNoteOff(w *gui.Window, i int) {
	v := synthActive[i]
	if v == nil {
		return
	}
	v.release()
	synthActive[i] = nil
	appState(w).AudioStatus = synthPadRows[i/synthPadsPerRow][i%synthPadsPerRow].name + " off"
}

// haltAllSounds stops every voice and the music track: halts all
// mixer channels, forgets the pad voices (so a later release over a
// pad is a no-op), and halts music.
func haltAllSounds(w *gui.Window) {
	app := appState(w)
	// HaltMusic on an uninitialized backend derefs a nil ctrl, so
	// guard on the flag rather than letting the user panic the app
	// by clicking Halt All before anything ever played.
	if !app.AudioReady {
		app.AudioStatus = "No audio initialized"
		return
	}
	audio.HaltChannel(-1)
	audio.HaltMusic()
	for i := range synthActive {
		synthActive[i] = nil
	}
	app.AudioMusicPlaying = false
	app.AudioMusicPaused = false
	app.AudioStatus = "All sounds halted"
}

// synthPadView builds one pad. Press starts the voice (OnMouseDown);
// release starts its decay (OnMouseUp).
func synthPadView(t gui.Theme, i int, p synthPadDef) gui.View {
	return gui.Column(gui.ContainerCfg{
		ID:          p.id,
		Width:       88,
		Height:      56,
		Sizing:      gui.FixedFixed,
		Color:       t.ColorPanel,
		ColorBorder: t.ColorBorder,
		Radius:      gui.SomeF(10),
		HAlign:      gui.HAlignCenter,
		VAlign:      gui.VAlignMiddle,
		Content: []gui.View{
			gui.Text(gui.TextCfg{
				Text:      p.name,
				TextStyle: t.N3,
			}),
			gui.Text(gui.TextCfg{
				Text:      fmt.Sprintf("%.0f Hz", p.freq),
				TextStyle: t.N4,
			}),
		},
		OnMouseDown: func(ctx gui.EventCtx) {
			synthNoteOn(ctx.Window, i, p.freq)
			ctx.Consume()
		},
		OnMouseUp: func(ctx gui.EventCtx) {
			synthNoteOff(ctx.Window, i)
			ctx.Consume()
		},
	})
}

// synthPadGrid builds the pad rows.
func synthPadGrid(t gui.Theme) gui.View {
	var rows []gui.View
	for r := range synthPadRows {
		pads := make([]gui.View, 0, len(synthPadRows[r]))
		for c := range synthPadRows[r] {
			pads = append(pads, synthPadView(t, r*synthPadsPerRow+c, synthPadRows[r][c]))
		}
		rows = append(rows, gui.Row(gui.ContainerCfg{
			Sizing:  gui.FillFit,
			Spacing: gui.SomeF(8),
			Padding: gui.NoPadding,
			Content: pads,
		}))
	}
	return gui.Column(gui.ContainerCfg{
		Sizing:  gui.FillFit,
		Spacing: gui.SomeF(8),
		Padding: gui.NoPadding,
		Content: rows,
	})
}

func ensureAudioInit(w *gui.Window) bool {
	app := appState(w)
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

// --- music ---

// loadMusicDemo loads the embedded music clip (Mozart, Eine kleine
// Nachtmusik, K. 525, I. Allegro — public-domain Musopen recording,
// assets/music.ogg). LoadMusic is path-only, so the embedded asset is
// extracted to a temp file first, the same way the image demos do.
func loadMusicDemo(w *gui.Window) {
	if !ensureAudioInit(w) {
		return
	}
	app := appState(w)
	cleanupMusic(w)
	path := embeddedAssetPath("assets/music.ogg")
	if path == "" {
		app.AudioStatus = "Music asset missing"
		return
	}
	track, err := audio.LoadMusic(path)
	if err != nil {
		app.AudioStatus = "Load music error: " + err.Error()
		return
	}
	musicTrack = track
	app.AudioMusicLoaded = true
	app.AudioStatus = "Music loaded (Mozart K. 525)"
}

func playMusic(w *gui.Window) {
	app := appState(w)
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
	app := appState(w)
	if !app.AudioMusicPlaying {
		app.AudioStatus = "No music to fade out"
		return
	}
	audio.FadeOutMusic(1000)
	app.AudioMusicPlaying = false
	app.AudioStatus = "Music fading out (1s)"
}

func stopMusic(w *gui.Window) {
	app := appState(w)
	if !app.AudioReady {
		app.AudioStatus = "No audio initialized"
		return
	}
	audio.HaltMusic()
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
	app := appState(w)
	app.AudioMusicLoaded = false
	app.AudioMusicPlaying = false
}
