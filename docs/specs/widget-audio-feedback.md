# Spec: opt-in widget audio feedback

Status: **phase 1 implemented** — `SoundCue`, `SoundPlayer`, `Theme.Sounds`,
window volume, and the `Button` / `Toggle` proof. Phases 2–4 are follow-up
issues.

Issue: #446 — "Widgets have no audio feedback for interactions".

## Motivation

No widget in `gui/` emitted sound. Feedback was visual only. Two audio
mechanisms existed and neither reached a widget:

- `Window.Beep` (`gui/native_sound.go`) — the system alert, with no non-test
  caller anywhere in the repo.
- `gui/audio` — sampled and synthesized audio, a leaf subpackage. `gui` must not
  import it: that would pull `beep`, `oto` and `pulse` into every consumer and
  invert the subpackage rule.

An app that wanted a click sound had to wrap `OnClick` on every widget and
handle `BeepAvailable` plus `audio.Init` errors itself.

## Design

### A cue is semantic, not a sample

`SoundCue` names the interaction; the injected player decides what it sounds
like. That indirection is what keeps `gui` free of an audio dependency, and it
lets one app map a cue to a WAV while another synthesizes it.

`SoundNone` is the zero value, so an unset field is silent and needs no `set`
flag — the same reasoning as `ColorSet` (`gui/color_set.go`).

The enum is open. New cues will be added, so no code in `gui/` switches
exhaustively over it, and the `SoundPlayer` contract requires a player to ignore
a cue it does not recognize.

### The seam is injected, like the other three

`SoundPlayer` is stored per window in `windowBackend`, beside `TextMeasurer`,
`SvgParser` and `NativePlatform`, and every call site nil-guards. One
difference, stated in the field comment: the backend installs the other three,
but installs **no** sound player. The app opts in with `SetSoundPlayer`, so
silence is the default and headless tests need no setup.

`NewBeepSoundPlayer` is the zero-dependency fallback: it plays the system alert
for `SoundError` and nothing else, because an alert is the wrong sound for a
click. It ignores gain — `NSBeep` and `MessageBeep` honor the system _alert_
volume and expose no app-level gain.

### Volume is window state, not theme state

`(*Window).SetSoundVolume` / `SoundVolume`, clamped to `0..1`, default `1`.
There is no separate mute flag: `0` is mute, and the gate is one comparison
ahead of the player call, so a muted window costs nothing per click.

Volume was deliberately **not** put on `Theme`. A theme is per-window visual
identity keyed by `Theme.id`, and `Themed(t, build)` scopes a theme to a
_subtree_; a loudness that changed when one panel was restyled would be
incoherent. Volume is a user preference.

It is also not expressed as a percentage of the system level, because it already
is one: the OS mixer applies the user's output volume downstream of the app, so
an app-level gain is inherently relative. An app cannot portably read the system
output level and must not try.

### Theme carries the role → cue map

`Theme.Sounds` is a `SoundSet` — one cue per interaction role. The zero set is
silent and is the intended default, so `ThemeDark` and `ThemeLight` make no
sound. `ThemeMaker` copies it from `ThemeCfg` and adds no default. It is not a
`default*Style` mirror, so `TestDefaultStylesMirrorThemeDark` needs no row and
`applyTheme` is untouched.

An app opts the whole widget set in by rebuilding through `ThemeMaker` with
`cfg.Sounds = gui.SoundsDefault()`. Mutating an installed theme value in place
is wrong here: `needsInstall` compares `Theme.id`, so a mutated copy carrying
the old id can be skipped in a second window. `ThemeMaker` stamps a fresh id.

A struct rather than a fixed array, so a role can be added without breaking an
app that builds one by field name.

### The cue rides the shape, not a closure

The real chokepoint is `eventHandlers`, not `ContainerCfg`: the unexported
`soundCue` field lives there, exactly like `clickOnSpace` and `clickOnEnter`,
which exist to avoid a per-frame closure allocation. Putting it there also
reaches the raw-`Shape` widgets (Image, Svg, Text, RTF, DrawCanvas, termgrid)
that build `eventHandlers` directly and never touch `ContainerCfg`.

Measured across `gui/` and `gui/datagrid/`: 86 click sites — 43 direct
`ContainerCfg`, 28 `ButtonCfg` (which funnels into Button's own container), 9
raw `eventHandlers`, 6 wrapper Cfgs. 71 of 86 inherit the plumbing from the
`ContainerCfg.Sound` field alone.

### Precedence, resolved at generation time

```
SoundDisabled  >  Cfg.Sound  >  Theme.Sounds.<role>
```

Resolution happens during `GenerateLayout`, where the widget already knows its
own state. That is why a state-dependent toggle needs **no** `SoundOn` /
`SoundOff` field pair: `Toggle` reads `cfg.Selected` and stamps `SoundToggleOff`
when the click will turn it off. No runtime branch, no allocation.

### Dispatch

One helper, `playShapeSound(*Layout, *Window)`, nil-safe on every seam. Four
call sites cover all click dispatch:

| Path     | Site                                        |
| -------- | ------------------------------------------- |
| Mouse    | `callRelative`, gated on `class == evClick` |
| Spacebar | `charHandler`, the `clickOnSpace` branch    |
| Enter    | `keydownHandler`, the `clickOnEnter` branch |
| A11y     | `a11yActionCallback`, `A11yActionPress`     |

The cue fires **before** `OnClick` and **independently of `ctx.Consume()`**:
sound confirms that the widget was activated, it does not participate in event
propagation. All four paths are asserted in `gui/sound_test.go`.

It is not routed through `deferCallback`. Dispatch runs from `EventFn` with no
frame lock held, and a deferred callback may not capture a `*Layout` — the arena
pools it.

## What stays silent

Pure click absorbers must never sound, and opt-in-by-default protects them with
no special case: the Toast scrim, the ContextMenu dismiss `OnAnyClick`,
scrollbar track/thumb/arrows, and Input's caret-placement click all consume
without user semantics. **Never add a blanket "any `ContainerCfg.OnClick`
sounds" rule.**

One known exclusion: `gui/view_text.go` uses a package-level shared
`textEventHandlers` singleton, which cannot carry a per-instance cue without
becoming per-view. Link-click sound is out of scope.

## Rollout

- **Phase 1 (this change)** — the seam, `Button`, `Toggle`, headless tests, both
  guides, the showcase page and player.
- **Phase 2** (#467) — the mechanical pass: every widget whose activation
  already lands on a `ContainerCfg` or `ButtonCfg` (Switch, Radio, ExpandPanel,
  Select, Combobox, ListBox, Tree, TabControl, Menu, Dialog, Toast, DatePicker,
  CommandPalette, DockLayout, Image, Svg, and `gui/datagrid`). Split by file
  count, not one 40-file diff.
- **Phase 3** (#468) — bespoke paths dispatch never sees: Table keyboard
  activation (calls `OnClick` with a nil `Layout`), Input commit vs reject,
  InputNumeric clamp, keyboard navigation (probably silent — decide it
  explicitly), drag-terminated changes, drag-reorder drop.
- **Phase 4** (#469) — non-click semantics (Toast appear, Dialog open, Form
  submit, DataGrid `OnCRUDError`) and a native system-sound player:
  `NSSound(named:)`, `PlaySound` with `SND_ALIAS`, freedesktop sound-naming IDs
  on Linux (the beep backend already shells to `canberra-gtk-play -i bell`, so
  `-i button-pressed` is a short step). That is cross-platform native work,
  including ObjC, and belongs in `gui/backend/sysbeep`.

## Showcase sounds are synthesized

The repo ships no sound assets and does not gain any. The showcase's player
reuses the ADSR voice already in `demo_audio.go` with a percussive, one-shot
envelope — `voiceEnv` was extracted from the pad's package constants so the pad
and the cue share `Fill` and `envelope` rather than forking the oscillator.

Click is A5; toggle-on and toggle-off are a fourth above and below it; error is
a low tone with a longer decay. The `audio.LoadSoundBytes` path for an app with
its own WAVs is documented in the guide but not shipped.

## Rejected

- **`gui` imports `gui/audio`** — an import cycle in spirit and a dependency in
  fact for every consumer.
- **Volume on `Theme`** — see above.
- **A separate mute flag** — `0` already means mute.
- **`SoundOn` / `SoundOff` field pairs on toggles** — unnecessary once the cue
  resolves at generation time.
- **Shipping WAV assets** — binaries, an attribution obligation, and a guide
  snippet nobody could copy without the files.

## Documentation

- `docs/widget-sound.md` and `examples/showcase/docs/widget_sound.md` — the
  app-author guide, mirrored. Every later phase must update the cue table in
  both.
- `examples/showcase/docs/widget_audio.md` — cross-linked, and corrected: it
  documented `audio.Quit`, `audio.LoadSound`, `Sound.SetVolume`,
  `SetMusicVolume`, `PauseMusic` and several other functions that are unexported
  in the package as it stands.
- The guide's player is quoted whole from `examples/showcase/sound_player.go`
  between `// doc:snippet-begin player` markers, not paraphrased, so a reader
  can paste it. `TestSoundGuideSnippetMatchesSource`
  (`examples/showcase/sound_snippet_test.go`) byte-compares the marked region
  against the Go fence under the "A real player" heading in both guides and
  names the first differing line. The block therefore compiles by construction —
  it is part of the showcase build — and cannot rot silently in either
  direction. Two names inside it (`ensureAudioInit`, `newVoice`) are showcase
  plumbing, explained in prose beneath the fence rather than inlined, which
  would have broken the byte comparison.
