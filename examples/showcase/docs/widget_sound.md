# Widget Sound Feedback

Widgets can play a short sound when the user activates them. Nothing is audible
until you opt in twice: a theme that names a **cue** for each interaction role,
and a **sound player** installed on the window. Either one alone stays silent,
which is the default for every built-in theme.

This page is mirrored in the repo at `docs/widget-sound.md`.

## What a cue is

`gui.SoundCue` names a semantic event, not a file:

| Cue                  | Meaning                                       |
| -------------------- | --------------------------------------------- |
| `gui.SoundNone`      | Zero value. Silent                            |
| `gui.SoundClick`     | Momentary activation: button, menu item, link |
| `gui.SoundToggleOn`  | A state went off → on                         |
| `gui.SoundToggleOff` | A state went on → off                         |
| `gui.SoundError`     | Rejection: invalid input, refused commit      |
| `gui.SoundSelection` | One option picked out of several              |

The framework decides _which_ cue an interaction is. Your player decides what a
cue sounds like. That split is why `gui` never imports `gui/audio`, and why an
app can map the same cue to different assets on different platforms.

The list grows over time, so a player must ignore a cue it does not recognize
rather than treat it as an error. Do not write an exhaustive switch over
`SoundCue`.

## Turn it on in three lines

```go
cfg := w.Theme().Cfg
cfg.Sounds = gui.SoundsDefault()            // name a cue per role
w.SetTheme(gui.ThemeMaker(cfg))
w.SetSoundPlayer(gui.NewBeepSoundPlayer(w)) // render them
```

Rebuild through `ThemeMaker` rather than mutating the theme value in place. A
`Theme` carries an `id`, and theme installation skips a theme whose id is
already installed, so a mutated copy can silently lose the change in a second
window. `ThemeMaker` stamps a fresh id.

Set `Sounds` wherever you build the themes your app installs — switching to a
theme that does not name cues turns sound off again.

`NewBeepSoundPlayer` needs no assets and no audio library: it plays the system
alert for `SoundError` and stays silent for everything else, because an alert is
the wrong sound for a click. It is the right choice for an app that only wants
to signal rejection.

Volume is per window, `0.0` to `1.0`:

```go
w.SetSoundVolume(0.5)
```

`0` is mute — there is no separate mute flag. Values outside the range clamp.
The value is implicitly relative to the system output level, which the OS mixer
applies downstream; an app cannot portably read that level and should not try.
`NewBeepSoundPlayer` ignores gain entirely: `NSBeep` and `MessageBeep` honor the
system _alert_ volume and expose no app-level gain.

## A real player

For actual click sounds, implement `gui.SoundPlayer` over `gui/audio`. Below is
the showcase's player, quoted whole from `examples/showcase/sound_player.go`. It
synthesizes every cue rather than loading files, so it needs no assets. A test
compares this block against that file byte for byte: what you read here is what
compiles.

```go
// Cue envelope: percussive, one-shot, so each cue frees its own mixer
// channel. Short enough that fast clicking does not smear.
var cueEnv = voiceEnv{
	attackS: 0.008,
	decayS:  0.060,
	sustain: 0,
	oneShot: true,
}

// errorEnv is longer and lets the tone ring, so a rejection reads as
// different in kind rather than only in pitch.
var errorEnv = voiceEnv{
	attackS: 0.008,
	decayS:  0.220,
	sustain: 0,
	oneShot: true,
}

// cuePeakLevel is the voice amplitude before the window's gain. Well
// below the synth pads: a UI cue rides under whatever else is playing.
const cuePeakLevel = 0.22

// cueSoundPlayer maps gui.SoundCue onto synthesized blips. Nothing is
// loaded from disk — the showcase ships no sound assets — so the same
// code runs on every desktop platform with no licensing or size cost.
// An app with its own WAVs swaps the switch below for
// audio.LoadSoundBytes; see docs/widget-sound.md.
type cueSoundPlayer struct {
	w *gui.Window
}

var _ gui.SoundPlayer = cueSoundPlayer{}

// PlaySound implements gui.SoundPlayer. It runs on the event-dispatch
// goroutine, so it starts a voice and returns; the mixer does the work
// on the audio thread.
func (p cueSoundPlayer) PlaySound(cue gui.SoundCue, gain float32) {
	var freq float64
	env := cueEnv
	switch cue {
	case gui.SoundClick:
		freq = 880 // A5
	case gui.SoundToggleOn:
		freq = 880 * 4 / 3 // a fourth up
	case gui.SoundToggleOff:
		freq = 880 * 3 / 4 // a fourth down
	case gui.SoundSelection:
		freq = 880 * 3 / 2 // a fifth up: "picked", against click's neutral A5
	case gui.SoundError:
		freq = 220
		env = errorEnv
	default:
		// An unrecognised cue is ignored, not an error: gui adds cues
		// over time and a player must not break when it does.
		return
	}
	if !ensureAudioInit(p.w) {
		return
	}
	// The window's volume arrives as gain; scale the voice rather than
	// touching audio.SetMasterVolume, which the music demo owns.
	v := newVoice(freq, cuePeakLevel*float64(gain), env)
	// A failed start is silent on purpose: a click must never surface a
	// mixer error to the user.
	_ = audio.PlaySource(-1, v)
}

// SoundAvailable implements gui.SoundPlayer. gui/audio builds on every
// desktop target this file compiles for.
func (p cueSoundPlayer) SoundAvailable() bool { return true }
```

Two names the block leans on. `ensureAudioInit(p.w)` is showcase plumbing that
initializes `gui/audio` once and reports a failure in the demo's status line; an
app without one writes `if err := audio.Init(); err != nil { return }` instead.
`newVoice` builds an `audio.Source` — see `examples/showcase/demo_audio.go` for
the oscillator and the envelope, and [the audio guide](widget_audio.md) for the
`Source` contract.

Install it on the window:

```go
w.SetSoundPlayer(cueSoundPlayer{w: w})
```

`PlaySound` runs on the event-dispatch goroutine, so it must be fire-and-forget:
start the sound and return. Never block, and never allocate on the audio thread.

## Playing your own files

An app that ships sound assets swaps the switch for loaded samples:

```go
type filePlayer struct {
    click, on, off, fail *audio.Sound
}

func newFilePlayer() (*filePlayer, error) {
    if err := audio.Init(); err != nil {
        return nil, err
    }
    p := &filePlayer{}
    var err error
    if p.click, err = audio.LoadSoundBytes(clickWav); err != nil {
        return nil, err
    }
    // ... one per cue
    return p, nil
}

func (p *filePlayer) PlaySound(cue gui.SoundCue, gain float32) {
    var s *audio.Sound
    switch cue {
    case gui.SoundClick:
        s = p.click
    case gui.SoundToggleOn:
        s = p.on
    case gui.SoundToggleOff:
        s = p.off
    case gui.SoundError:
        s = p.fail
    default:
        return
    }
    // gui/audio exports no per-sound volume setter today, so a
    // file-based player either ignores gain or scales the whole mixer
    // with audio.SetMasterVolume. Prefer pre-normalized assets.
    _, _ = s.PlayOnce()
}

func (p *filePlayer) SoundAvailable() bool { return true }

// Free every sound at teardown; never Free one while it plays.
func (p *filePlayer) Close() {
    p.click.Free()
    p.on.Free()
    p.off.Free()
    p.fail.Free()
}
```

Embed the WAVs with `//go:embed` and pass the bytes — `audio.LoadSoundBytes`
avoids the temp-file dance a path-based load needs. The repo ships no sound
assets of its own, so the showcase uses the synthesized player above.

## Per-widget control

Two fields on the widget's `Cfg`:

| Field                | Effect                                         |
| -------------------- | ---------------------------------------------- |
| `Sound SoundCue`     | Overrides the theme's cue for this instance    |
| `SoundDisabled bool` | Silences this instance whatever the theme says |

Precedence, resolved once at generation time:

```
SoundDisabled  >  Cfg.Sound  >  Theme.Sounds.<role>
```

```go
gui.Button(gui.ButtonCfg{ID: "del", Label: "Delete", Sound: gui.SoundError})
gui.Button(gui.ButtonCfg{ID: "tick", Label: "Tick", SoundDisabled: true})
```

A `Toggle` picks its own cue from its state: a selected toggle is about to turn
off, so it emits `SoundToggleOff`. You do not need an on/off field pair.

`SliderCfg` and `NumericInputCfg` take `SoundDisabled` alone. Neither has an
activation moment to sound at, so their only cue is the `Error` a refused step
or a refused arrow key emits, and a `Sound` field would name a cue that never
plays.

On an `Input`, `Sound` names the Enter-commit cue only. Rejection always takes
`Theme.Sounds.Error`; `SoundDisabled` silences both.

## Which widgets sound

Every widget below emits a cue once the app has opted in. A widget that toggles
names what the click will do, not what the state is, so an open panel about to
close emits `SoundToggleOff`.

| Widget                                     | Cue on activation                         |
| ------------------------------------------ | ----------------------------------------- |
| `Button`, `MenuItem`, `Dialog` buttons     | `Click`                                   |
| `Toast` buttons, `CommandPalette` backdrop | `Click`                                   |
| `Image`, `Svg`                             | `Click`                                   |
| `Toggle`, `Switch`, `ExpandPanel`          | `ToggleOn` / `ToggleOff`                  |
| `Select`, `Combobox`, `ThemePicker`        | `ToggleOn` / `ToggleOff` (the field)      |
| `Radio`, `RadioButtonGroup`                | `Selection`                               |
| `ListBox` rows, `Select` options           | `Selection`                               |
| `TabControl`, `DockLayout` tabs            | `Selection`                               |
| `Breadcrumb`, `ColorSwatch`                | `Selection`                               |
| `DatePicker` day cells                     | `Selection`                               |
| `Tree` rows                                | `Selection`, or toggle if it has children |
| `datagrid` rows, toolbar, pager            | `Click`                                   |
| `datagrid` sort, pin, column reorder       | `Selection`                               |

Keyboard and drag paths sound too, and refusal is its own cue:

| Interaction                                    | Cue                      |
| ---------------------------------------------- | ------------------------ |
| `Table`, `ListBox`, `Tree` keyboard activation | `Selection`              |
| `Menu` keyboard activation                     | `Click`                  |
| `Input` Enter commit                           | `Click`                  |
| `Input` rejection (mask, paste, delete, veto)  | `Error`                  |
| Arrow key already at a bound or list edge      | `Error`                  |
| `NumericInput` step at `Min` or `Max`          | `Error`                  |
| Drag-reorder drop that moves an item           | `Selection`              |
| Dock panel dropped into a zone                 | `Selection`              |
| `Splitter` collapse toggle                     | `ToggleOn` / `ToggleOff` |

Movement is silent, refusal is not. An arrow key that moves a selection makes no
sound — a held arrow key would machine-gun the cue — while an arrow key that
cannot move emits `Error`. Continuous drag is silent for the same reason: the
slider thumb, the splitter handle and the colour plane and wheel say nothing
while dragging, and only the splitter's collapse toggle and a drop that really
moves something are cues. A drag cancelled with Escape, a drop that lands where
the item already was, and an `Input` commit caused by blur are all silent by
decision.

Widgets that only absorb clicks stay silent by design: the toast scrim, the
context-menu dismiss layer, scrollbar tracks, the `CommandPalette` card, the
`DatePicker` field itself (its click only takes focus), and the caret-placement
click inside an `Input`. So do a disabled control, a `Breadcrumb` crumb marked
disabled, a `ListBox` subheading, and a menu separator.

Text is not covered: `gui.Text` shares one package-level handler record across
every text shape, so it cannot carry a per-instance cue without giving up that
zero-allocation sharing. Hover, focus and notification cues are still to come
(#469).

## Platform reality

- `gui/audio` does not build for `js`, `android`, or `ios`. Put your player
  behind a `//go:build !js && !android && !ios` tag and supply an empty stub for
  the others, the way the showcase does.
- `BeepAvailable()` is false on mobile and wasm, and on Linux without
  `canberra-gtk-play`.
- The failure mode is always silence, never a panic. A nil `SoundPlayer` is the
  default and every call site nil-guards, which is also why headless tests are
  silent with no setup.
- Call `SoundAvailable()` when the user genuinely needs to notice the event, and
  add a visual cue when it returns false.

## Ordering

The cue fires **before** `OnClick` runs, and **regardless of whether the handler
calls `ctx.Consume()`**. The sound confirms that the widget was activated; it
does not take part in event propagation.

Every activation path sounds, not just the mouse: click, Space, Enter, and the
accessibility press action all emit the same cue. Keyboard activation that never
reaches event dispatch — a `Table` row activated with Enter, an `Input`
rejection, a drag-reorder drop — raises its cue at its own call site, with the
same ordering.

## Testing

Widget sound is fully headless-testable. Inject a spy player and drive the
widget with the `Test*` helpers:

```go
type spy struct{ cues []gui.SoundCue }

func (s *spy) PlaySound(c gui.SoundCue, _ float32) {
    s.cues = append(s.cues, c)
}
func (s *spy) SoundAvailable() bool { return true }

sp := &spy{}
w.SetSoundPlayer(sp)
w.TestClick("save")
// sp.cues == []gui.SoundCue{gui.SoundClick}
```

## See also

- [Audio playback](widget_audio.md) — the `gui/audio` package: sounds, music,
  and live synthesis
- `docs/specs/widget-audio-feedback.md` — the design record and the alternatives
  that were rejected
