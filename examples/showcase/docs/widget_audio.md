Opt-in audio playback via SDL_mixer. Supports sound effects on
multiple mixing channels and one music track. Desktop only
(macOS, Windows, Linux).

## Setup

```go
import "github.com/go-gui-org/go-gui/gui/audio"

// Initialize once (e.g. in main or OnInit).
if err := audio.Init(); err != nil {
    log.Fatal(err)
}
defer audio.Quit()
```

## Sound Effects

```go
// Load from file.
click, _ := audio.LoadSound("click.wav")
defer click.Free()
click.PlayOnce()

// Load from embedded bytes.
snd, _ := audio.LoadSoundBytes(wavData)
snd.Play(-1, 0) // channel -1 = first free, 0 = no loop
```

## Music

```go
bgm, _ := audio.LoadMusic("theme.ogg")
defer bgm.Free()
bgm.Play(-1)        // -1 = loop forever
bgm.FadeIn(-1, 2000) // fade in over 2 s

audio.PauseMusic()
audio.ResumeMusic()
audio.FadeOutMusic(1000)
```

## Volume

```go
audio.SetMasterVolume(0.8) // sound effects 0.0–1.0
audio.SetMusicVolume(0.5)  // music channel 0.0–1.0

snd.SetVolume(0.3)         // per-sound volume
```

## audio.Cfg

| Field          | Type   | Default                      |
|----------------|--------|------------------------------|
| Frequency      | int    | 44100                        |
| Format         | uint16 | mix.DEFAULT_FORMAT           |
| OutputChannels | int    | 2 (stereo)                   |
| ChunkSize      | int    | 2048                         |
| MixChannels    | int    | 16                           |
| Formats        | int    | INIT_OGG | INIT_MP3 |

## Sound API

| Function              | Description                              |
|-----------------------|------------------------------------------|
| LoadSound(path)       | Load from file (WAV, OGG, FLAC, …)      |
| LoadSoundBytes(data)  | Load from byte slice                     |
| Play(channel, loops)  | Play on channel (-1 = auto)              |
| PlayOnce()            | Shorthand for Play(-1, 0)                |
| FadeIn(ch, loops, ms) | Play with fade-in                        |
| SetVolume(v)          | Per-sound volume 0.0–1.0                 |
| Free()                | Release resources                        |

## Music API

| Function           | Description                                 |
|--------------------|---------------------------------------------|
| LoadMusic(path)    | Load music file (OGG, MP3, FLAC, MOD, …)   |
| Play(loops)        | Play (0 = once, -1 = forever)               |
| FadeIn(loops, ms)  | Play with fade-in                           |
| Free()             | Release resources                           |

## Global Controls

| Function            | Description                                |
|---------------------|--------------------------------------------|
| SetMasterVolume(v)  | All sound channels 0.0–1.0                 |
| SetMusicVolume(v)   | Music channel 0.0–1.0                      |
| HaltChannel(ch)     | Stop channel (-1 = all)                    |
| HaltMusic()         | Stop music immediately                     |
| FadeOutMusic(ms)    | Fade out music                             |
| PauseMusic()        | Pause music                                |
| ResumeMusic()       | Resume music                               |

## Notes

- Only **one music track** plays at a time (SDL_mixer limitation)
- Do not Free a Sound while it is still playing
- Call Init from the **main goroutine** (same thread as SDL event loop)
- Requires SDL_mixer system library (`brew install sdl2_mixer` on macOS)
