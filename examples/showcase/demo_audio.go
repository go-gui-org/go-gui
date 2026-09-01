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
							if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
								v = 0
							}
							if v < 0 {
								v = 0
							} else if v > 100 {
								v = 100
							}
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

			sectionLabel(t, "Widget Sound Feedback"),
			widgetSoundPanel(t, app),

			gui.Text(gui.TextCfg{
				Text:      app.AudioStatus,
				TextStyle: t.N4,
				Mode:      gui.TextModeWrap,
			}),
		},
	})
}

// widgetSoundControls is the same panel, reached from the "Sound
// Feedback" page so the guide and its switch sit together.
func widgetSoundControls(w *gui.Window) gui.View {
	return widgetSoundPanel(gui.CurrentTheme(), appState(w))
}

// widgetSoundPanel is the interactive half of the "Sound Feedback"
// doc page: the opt-in switch, the window gain, and two widgets whose
// cues differ, so the click and the two toggle cues can be compared
// side by side.
func widgetSoundPanel(t gui.Theme, app *ShowcaseApp) gui.View {
	return gui.Column(gui.ContainerCfg{
		Sizing:  gui.FillFit,
		Spacing: gui.SomeF(8),
		Padding: gui.NoPadding,
		Content: []gui.View{
			gui.Text(gui.TextCfg{
				Text: "Silent until you opt in twice: a theme that " +
					"names a cue per role, and an installed " +
					"SoundPlayer. See the Sound Feedback page.",
				TextStyle: t.N4,
				Mode:      gui.TextModeWrap,
			}),
			gui.Switch(gui.SwitchCfg{
				ID:       "widget-sound-on",
				Label:    "Widget sounds",
				Selected: app.WidgetSoundOn,
				OnClick: func(ctx gui.EventCtx) {
					a := appState(ctx.Window)
					a.WidgetSoundOn = !a.WidgetSoundOn
					if a.WidgetSoundOn {
						installWidgetSounds(ctx.Window, a.WidgetSoundPlayer)
						ctx.Window.SetSoundVolume(a.WidgetSoundVolume)
					} else {
						removeWidgetSounds(ctx.Window)
					}
					ctx.Consume()
				},
			}),
			// Which player renders the cues. The synthesized one needs
			// gui/audio; the other two need neither assets nor an
			// audio library.
			gui.Select(gui.SelectCfg{
				ID:       "widget-sound-player",
				Label:    "Player",
				Options:  soundPlayerLabels,
				Selected: []string{soundPlayerValue(app.WidgetSoundPlayer)},
				OnSelect: func(selected []string, ctx gui.EventCtx) {
					if len(selected) == 0 {
						return
					}
					a := appState(ctx.Window)
					a.WidgetSoundPlayer = soundPlayerKindFor(selected[0])
					if a.WidgetSoundOn {
						installWidgetSounds(ctx.Window, a.WidgetSoundPlayer)
					}
					ctx.Consume()
				},
			}),
			gui.Row(gui.ContainerCfg{
				Sizing:  gui.FillFit,
				Spacing: gui.SomeF(8),
				Padding: gui.NoPadding,
				VAlign:  gui.VAlignMiddle,
				Content: []gui.View{
					gui.Text(gui.TextCfg{
						Text:      "Cue volume",
						TextStyle: t.N4,
						MinWidth:  90,
					}),
					gui.Slider(gui.SliderCfg{
						ID:     "widget-sound-vol",
						Value:  app.WidgetSoundVolume * 100,
						Min:    0,
						Max:    100,
						Sizing: gui.FillFit,
						OnChange: func(v float32, ctx gui.EventCtx) {
							if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
								v = 0
							}
							if v < 0 {
								v = 0
							} else if v > 100 {
								v = 100
							}
							a := appState(ctx.Window)
							a.WidgetSoundVolume = v / 100
							// 0 is mute; there is no separate flag.
							ctx.Window.SetSoundVolume(a.WidgetSoundVolume)
						},
					}),
					gui.Text(gui.TextCfg{
						Text: fmt.Sprintf("%.0f%%",
							app.WidgetSoundVolume*100),
						TextStyle: t.N4,
						MinWidth:  40,
					}),
				},
			}),
			gui.Row(gui.ContainerCfg{
				Sizing:  gui.FillFit,
				Spacing: gui.SomeF(12),
				Padding: gui.NoPadding,
				VAlign:  gui.VAlignMiddle,
				Content: []gui.View{
					gui.Button(gui.ButtonCfg{
						ID:      "widget-sound-click",
						Label:   "Click me",
						Padding: gui.NewPadding(8, 16, 8, 16),
						OnClick: func(ctx gui.EventCtx) {
							ctx.Consume()
						},
					}),
					gui.Toggle(gui.ToggleCfg{
						ID:       "widget-sound-toggle",
						Label:    "Toggle me",
						Selected: app.WidgetSoundDemoOn,
						OnClick: func(ctx gui.EventCtx) {
							a := appState(ctx.Window)
							a.WidgetSoundDemoOn = !a.WidgetSoundDemoOn
							ctx.Consume()
						},
					}),
					// SoundDisabled opts one instance out, whatever the
					// theme says.
					gui.Button(gui.ButtonCfg{
						ID:            "widget-sound-muted",
						Label:         "Silent button",
						Padding:       gui.NewPadding(8, 16, 8, 16),
						SoundDisabled: true,
						OnClick: func(ctx gui.EventCtx) {
							ctx.Consume()
						},
					}),
				},
			}),
			widgetSoundNonClickRow(),
		},
	})
}

// widgetSoundNonClickRow demonstrates the cues that no click produces:
// a toast appearing and a dialog opening (issue #469). Both are
// imperative APIs, so each button just calls one and the cue rides the
// call, not the click.
func widgetSoundNonClickRow() gui.View {
	return gui.Row(gui.ContainerCfg{
		Sizing:  gui.FillFit,
		Spacing: gui.SomeF(12),
		Padding: gui.NoPadding,
		VAlign:  gui.VAlignMiddle,
		Content: []gui.View{
			gui.Button(gui.ButtonCfg{
				ID:      "widget-sound-toast",
				Label:   "Toast",
				Padding: gui.NewPadding(8, 16, 8, 16),
				OnClick: func(ctx gui.EventCtx) {
					ctx.Window.Toast(gui.ToastCfg{
						Title: "Notify",
						Body:  "An info toast takes the Notify cue.",
					})
					ctx.Consume()
				},
			}),
			gui.Button(gui.ButtonCfg{
				ID:      "widget-sound-toast-error",
				Label:   "Error toast",
				Padding: gui.NewPadding(8, 16, 8, 16),
				OnClick: func(ctx gui.EventCtx) {
					ctx.Window.Toast(gui.ToastCfg{
						Title:    "Error",
						Body:     "An error toast takes the Error cue.",
						Severity: gui.ToastError,
					})
					ctx.Consume()
				},
			}),
			gui.Button(gui.ButtonCfg{
				ID:      "widget-sound-dialog",
				Label:   "Dialog",
				Padding: gui.NewPadding(8, 16, 8, 16),
				OnClick: func(ctx gui.EventCtx) {
					ctx.Window.Dialog(gui.DialogCfg{
						Title: "Open",
						Body:  "Opening a dialog takes the Open cue.",
					})
					ctx.Consume()
				},
			}),
		},
	})
}

// soundPlayerLabels are the Select's options, and soundPlayerValue /
// soundPlayerKindFor map them onto the player kind so the app state
// stays typed rather than holding a string.
var soundPlayerLabels = []string{
	"Synthesized (gui/audio)",
	"System event sounds",
	"System alert on errors only",
}

func soundPlayerValue(kind soundPlayerKind) string {
	switch kind {
	case soundPlayerBeep:
		return soundPlayerLabels[2]
	case soundPlayerSystem:
		return soundPlayerLabels[1]
	default:
		return soundPlayerLabels[0]
	}
}

func soundPlayerKindFor(label string) soundPlayerKind {
	switch label {
	case soundPlayerLabels[2]:
		return soundPlayerBeep
	case soundPlayerLabels[1]:
		return soundPlayerSystem
	default:
		return soundPlayerSynth
	}
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

// Envelope timings in seconds for the pad voice.
const (
	synthAttackS  = 0.01
	synthDecayS   = 0.25
	synthReleaseS = 0.30
	synthSustain  = 0.35 // sustain level as a fraction of the peak
)

// voiceEnv holds the timings one voice runs on, so the pad voice and
// the UI cue voice share Fill and envelope instead of forking the
// oscillator (issue #446).
type voiceEnv struct {
	attackS  float64
	decayS   float64
	releaseS float64
	// sustain is the level held after decay, as a fraction of the peak.
	sustain float64
	// oneShot ends the voice when the decay finishes, freeing its mixer
	// channel with no note-off. A pad sustains until release; a UI cue
	// has no key to lift.
	oneShot bool
}

// padEnv is the sustaining envelope the synth pads play.
var padEnv = voiceEnv{
	attackS:  synthAttackS,
	decayS:   synthDecayS,
	releaseS: synthReleaseS,
	sustain:  synthSustain,
}

// synthVoice is a live audio source: three harmonics of the pad's
// frequency shaped by an ADSR envelope. Fill runs on the audio thread,
// so the envelope state lives entirely there; the note-off arrives via
// an atomic flag set from the UI thread.
type synthVoice struct {
	env          voiceEnv
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
	return newVoice(freq, 0.2, padEnv)
}

// newVoice builds a voice at the given peak level and envelope.
func newVoice(freq, level float64, env voiceEnv) *synthVoice {
	if math.IsNaN(freq) || math.IsInf(freq, 0) || freq <= 0 {
		freq = 440
	}
	if math.IsNaN(level) || math.IsInf(level, 0) {
		level = 0
	}
	if level < 0 {
		level = 0
	} else if level > 1 {
		level = 1
	}
	v := &synthVoice{
		env:        env,
		sampleRate: float64(audio.SampleRate()),
		freq:       freq,
		level:      level,
	}
	if v.sampleRate == 0 || math.IsNaN(v.sampleRate) || math.IsInf(v.sampleRate, 0) || v.sampleRate <= 0 {
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
	e := &v.env
	// Defensive: a zero or negative timing would divide by zero and
	// propagate Inf/NaN into the mixer. Treat it as an instant stage.
	if e.attackS <= 0 {
		e.attackS = 1e-6
	}
	if e.decayS <= 0 {
		e.decayS = 1e-6
	}
	if e.releaseS <= 0 {
		e.releaseS = 1e-6
	}
	if math.IsNaN(e.attackS) || math.IsInf(e.attackS, 0) {
		e.attackS = 1e-6
	}
	if math.IsNaN(e.decayS) || math.IsInf(e.decayS, 0) {
		e.decayS = 1e-6
	}
	if math.IsNaN(e.releaseS) || math.IsInf(e.releaseS, 0) {
		e.releaseS = 1e-6
	}
	if v.releasing.Load() {
		if v.elapsed >= e.releaseS {
			v.done = true
			return 0
		}
		amp := v.releaseLevel * (1 - v.elapsed/e.releaseS)
		v.elapsed += dt
		return amp
	}
	switch {
	case v.elapsed < e.attackS:
		amp := v.level * (v.elapsed / e.attackS)
		v.elapsed += dt
		v.releaseLevel = amp
		return amp
	case v.elapsed < e.attackS+e.decayS:
		t := (v.elapsed - e.attackS) / e.decayS
		amp := v.level * (1 - (1-e.sustain)*t)
		v.elapsed += dt
		v.releaseLevel = amp
		return amp
	default:
		// A one-shot has nothing to sustain into: end it here so the
		// mixer channel is freed without a note-off.
		if e.oneShot {
			v.done = true
			return 0
		}
		amp := v.level * e.sustain
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
