Audio playback via beep. Supports sound effects on multiple mixing channels and
one music track. Desktop only (macOS, Windows, Linux).

## Setup

```go
import "github.com/go-gui-org/go-gui/gui/audio"

// Initialize once (e.g. in main or OnInit).
if err := audio.Init(); err != nil {
    log.Fatal(err)
}
```

Init is idempotent, so a lazy "initialize on first sound" helper is the usual
pattern. There is no exported shutdown: the mixer lives for the process.

## Sound Effects

```go
// Load from embedded bytes (WAV, OGG, MP3, FLAC).
snd, _ := audio.LoadSoundBytes(wavData)
defer snd.Free()

snd.PlayOnce()   // channel -1 = first free, no loop
snd.Play(-1, 0)  // the long form: channel, loop count
```

Loading is bytes-only: pair it with `//go:embed` and the sound ships inside the
binary with no path handling.

## Music

```go
bgm, _ := audio.LoadMusic("theme.ogg")
defer bgm.Free()
bgm.Play(-1)          // -1 = loop forever
bgm.FadeIn(-1, 2000)  // fade in over 2 s

audio.FadeOutMusic(1000)
audio.HaltMusic()
```

The showcase demo loads its music clip via `embeddedAssetPath`. The clip is the
embedded asset `examples/showcase/assets/music.ogg` (Mozart, Eine kleine
Nachtmusik K. 525, I. Allegro — a public-domain Musopen recording). `LoadMusic`
takes a file path.

## Live Synthesis

Stream generated samples instead of playing files back:

```go
// Fill is called on the audio thread; it must not allocate or block.
// Write stereo samples (left, right) and return the count written.
// Return ok = false to end the source; its channel is freed.
type Source interface {
    Fill(samples [][2]float64) (n int, ok bool)
}

// Start a source on a channel (-1 = first free).
if err := audio.PlaySource(-1, mySource); err != nil { ... }
audio.HaltChannel(ch) // stop early

// Synthesis code needs the configured rate for phase increments.
rate := audio.SampleRate() // 0 before Init
```

The `Source` signature matches `beep.Streamer` exactly, so the beep backend is a
zero-cost adapter — no allocations on the audio path.

A synth pad pairs `Source` with the container press/release callbacks:

```go
gui.Column(gui.ContainerCfg{
    ID: "pad",
    // OnMouseDown starts the voice; OnMouseUp starts its note-off.
    OnMouseDown: func(ctx gui.EventCtx) {
        active = newVoice(freq) // your Source
        audio.PlaySource(-1, active)
        ctx.Consume()
    },
    OnMouseUp: func(ctx gui.EventCtx) {
        active.release() // voice decays, then ends itself
        ctx.Consume()
    },
})
```

A voice plays a note-off by releasing internally and returning `(0, false)` once
its release envelope finishes. The channel frees itself, so no per-pad channel
bookkeeping is needed.

## Volume

```go
audio.SetMasterVolume(0.8) // all sound channels 0.0–1.0
v := audio.MasterVolume()  // read it back

vol := snd.Volume()        // per-sound volume, read-only today
```

Per-sound volume has a getter but no exported setter, so normalize assets before
embedding them, or scale a synthesized voice's own amplitude — see
[Widget Sound Feedback](widget_sound.md).

## audio.Cfg

| Field          | Type | Default | Notes                         |
| -------------- | ---- | ------- | ----------------------------- |
| Frequency      | int  | 44100   | Speaker sample rate (Hz)      |
| OutputChannels | int  | 2       | Ignored (beep is stereo)      |
| ChunkSize      | int  | 2048    | Speaker buffer size (samples) |
| MixChannels    | int  | 16      | Sound-effect channel count    |

## Sound API

| Function              | Description                                |
| --------------------- | ------------------------------------------ |
| LoadSoundBytes(data)  | Load from byte slice (WAV, OGG, MP3, FLAC) |
| Play(channel, loops)  | Play on channel (-1 = auto)                |
| PlayOnce()            | Shorthand for Play(-1, 0)                  |
| FadeIn(ch, loops, ms) | Play with fade-in                          |
| Volume()              | Per-sound volume 0.0–1.0                   |
| Free()                | Release resources                          |

## Music API

| Function          | Description                           |
| ----------------- | ------------------------------------- |
| LoadMusic(path)   | Load music file (WAV, OGG, MP3, FLAC) |
| Play(loops)       | Play (0 = once, -1 = forever)         |
| FadeIn(loops, ms) | Play with fade-in                     |
| Free()            | Release resources                     |

## Global Controls

| Function           | Description                    |
| ------------------ | ------------------------------ |
| SetMasterVolume(v) | All sound channels 0.0–1.0     |
| MasterVolume()     | Read the master volume         |
| HaltChannel(ch)    | Stop channel (-1 = all)        |
| IsPlaying(ch)      | Whether channel is playing     |
| HaltMusic()        | Stop music immediately         |
| FadeOutMusic(ms)   | Fade out music then halt       |
| PlaySource(ch, s)  | Stream a live Source           |
| SampleRate()       | Configured rate, 0 before Init |

For **widget interaction sounds** — a click on a Button, a Toggle flipping — see
[Widget Sound Feedback](widget_sound.md). That is a separate, higher-level
opt-in: `gui` emits semantic cues and your player, built on this package,
decides what they sound like.

## Notes

- Only **one music track** plays at a time (global music channel)
- Do not Free a Sound while it plays
- No external libraries required — beep is pure Go on macOS/Windows. Linux needs
  `-ldl` only
