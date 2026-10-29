# Keys and shortcuts on every keyboard layout

Issue: #1188. Status: **planned**. Linux (X11 and Wayland) first; Windows, macOS
and web follow as separate issues.

## Problem

A key press reaches the app as two events. `EventChar` carries the text the key
types. `EventKeyDown` carries a `KeyCode` and the `Modifiers`, and shortcuts,
navigation and editing keys read only that. On Linux the two are built from
different data:

- `EventChar` (Wayland) comes from the key's symbol under the full state —
  Shift, AltGr, NumLock, active layout — so text is right on every layout.
- `KeyCode` comes from the key's first-level symbol in the active layout, sent
  through `x11key.MapKeySym`, which knows US ASCII only. Anything else is
  `KeyInvalid`.

`Shortcut.matches` then compares `KeyCode` and `Modifiers` exactly. The result,
measured on the reporter's KDE machine (`de nodeadkeys`) and reproduced in an
Xorg container on `de`, `fr`, `ru` and `us dvorak`:

| Symptom                                                    | Cause                                                        |
| ---------------------------------------------------------- | ------------------------------------------------------------ |
| `Shortcut{KeySlash}` never fires on German                 | `/` is Shift+7: the press is `Key7` + Shift                  |
| `#`, `+`, `<`, `^`, ä, ö, ü, ß give `KeyInvalid`           | first-level symbol is not in the US table                    |
| AltGr+8 (`[`) fires a `{Key8}` shortcut                    | AltGr is not a `Modifier`, so the press is plain `Key8`      |
| numpad keys give `KeyInvalid`; NumLock-off arrows dead     | keypad symbols are not in the table                          |
| `KeyNumLock`, `KeyMenu`, `KeyRightAlt`, F13–F25 never sent | not in the table (right Alt on German is `ISO_Level3_Shift`) |
| Ctrl+1 dead on French                                      | the AZERTY number row types `&é"'(` without Shift            |
| Ctrl+C/V/X/Z/A dead on Russian, in inputs too              | first-level symbol is Cyrillic                               |

The X11 path has three more defects, in the text it types:

| Symptom (X11 only)                         | Cause                                                        |
| ------------------------------------------ | ------------------------------------------------------------ |
| AltGr+Q types `q`, not `@`                 | the keysym column is chosen from Shift alone                 |
| numpad digits type nothing                 | the X11 NumLock rule is not applied                          |
| Cyrillic, Greek, Hebrew text types nothing | `x11key.KeysymToRune` knows Latin-1 and Unicode keysyms only |

## Goals

1. Every shortcut whose key a layout can type fires on that layout.
2. A press never fires a shortcut for a key it did not produce (AltGr+8 is not
   `8`).
3. Ctrl+letter and Ctrl+digit shortcuts work on non-Latin and AZERTY layouts.
4. The numpad has key codes in both NumLock states.
5. The X11 path types the same text as the Wayland path.
6. Backends that are not ported yet behave exactly as today.

## Non-goals

- Shortcuts bound to national letters (ä, ß, é). They have no `KeyCode` and keep
  none; a later issue can add a rune-based `Shortcut`.
- Physical-position shortcuts (games). A separate feature if wanted.
- Windows, macOS, web. Their ports reuse the rules below in later issues.

## Design

### 1. Two key identities per press

`EventKeyDown` and `EventKeyUp` keep `KeyCode` and `Modifiers` and gain two
fields:

- `TypedKey KeyCode` — the key the press produced with Shift, AltGr and NumLock
  applied. Shift+7 on German is `KeySlash`; AltGr+8 is `KeyLeftBracket`; Shift+A
  is still `KeyA`.
- `TypedModifiers Modifier` — `Modifiers` without the ones the translation used
  up (libxkbcommon's _consumed modifiers_). For Shift+7 on German it is
  `ModNone`; for Ctrl+Shift+7 it is `ModCtrl`.

When the press produced the same key as `KeyCode` (letters, Shift+A), or a
symbol with no key code (Cyrillic с, ä), the backend sets `TypedKey = KeyCode`
and `TypedModifiers = Modifiers`. Case is not a different key: Ctrl+Shift+A must
not fire `Ctrl+A`.

A backend that leaves both fields zero gets today's matching.

### 2. `KeyCode`: one rule list for every layout

The first rule that applies wins. "Unmodified" means NumLock applied, nothing
else; "shifted" adds Shift.

1. **Modifier and named keys** by their own symbol: Shift, Control, Alt, Super,
   `ISO_Level3_Shift` on the right Alt key (`KeyRightAlt`), `Num_Lock`, `Menu`,
   `Scroll_Lock`, `Print`, `Pause`, F1–F25, Escape, Tab, Return, BackSpace,
   Insert, Delete, arrows, Home, End, Page Up/Down.
2. **Keypad.** NumLock on: `KP_0`–`KP_9` → `KeyKP0`–`KeyKP9`, `KP_Decimal` and
   `KP_Separator` → `KeyKPDecimal`. NumLock off: `KP_Home` → `KeyHome` and so on
   for every navigation symbol, `KP_Delete` → `KeyDelete`, `KP_Insert` →
   `KeyInsert`, `KP_Begin` → `KeyInvalid`. In both states the operators map to
   `KeyKPDivide`, `KeyKPMultiply`, `KeyKPSubtract`, `KeyKPAdd`, and `KP_Enter`
   stays `KeyEnter` (as on web).
3. **Unmodified ASCII letter or digit** → that key (`KeyA`…`KeyZ`,
   `Key0`…`Key9`).
4. **Shifted ASCII digit** → that digit. This is the AZERTY row.
5. **Non-Latin layout** (the active layout's A–D keys type no ASCII letter):
   - the unmodified letter or digit of the first configured Latin layout on the
     same physical key, before or after the active one, else
   - the US letter or digit at that physical position.

   On a non-Latin layout, letter shortcuts follow the first configured Latin
   layout; changing that order may change their logical shortcut identities.
   That is intended configuration semantics (D6): `KeyCode` is a layout-aware
   identity, so a Dvorak or Colemak user keeps their letters on Russian or
   Greek. Order matters only with two different Latin layouts plus a non-Latin
   one, and then the first Latin layout stands for the primary shortcut layout.

6. **Unmodified printable ASCII** → its key code (§ 3).
7. **Dead keys** by their spacing ASCII character: `dead_grave` → `` ` ``,
   `dead_circumflex` → `^`, `dead_tilde` → `~`. Others → `KeyInvalid`.
8. Otherwise `KeyInvalid`.

`EventKeyUp` repeats the `KeyCode` and `TypedKey` its press reported, even if
NumLock or the layout changed while the key was held.

### 3. Key codes for the rest of printable ASCII

`KeyCode` values for ASCII already equal the character code. The 21 printable
characters without a constant get one, with the character code as value:

`KeyExclamation` (!), `KeyQuotation` ("), `KeyNumberSign` (#), `KeyDollar` ($),
`KeyPercent` (%), `KeyAmpersand` (&), `KeyLeftParen` ((), `KeyRightParen` ()),
`KeyAsterisk` (\*), `KeyPlus` (+), `KeyColon` (:), `KeyLess` (<), `KeyGreater`
(>), `KeyQuestion` (?), `KeyAt` (@), `KeyCircumflex` (^), `KeyUnderscore` (\_),
`KeyLeftBrace` ({), `KeyBar` (|), `KeyRightBrace` (}), `KeyTilde` (~).

None collides: the named keys start at 256 and the letters are 65–90.

`keyScrollLock`, `keyPrintScreen`, `keyPause` and `keyF21`–`keyF24` are
exported, since rule 1 now emits them.

### 4. `ModAltGr`

A new modifier bit, `ModAltGr` (`0x10`), is set while ISO level 3 is active
(AltGr, or whatever key the layout binds to it). It takes part in exact
matching, so AltGr+8 (`Key8` + `ModAltGr`) no longer matches `{Key8}`. It is
consumed when it changed the symbol, so AltGr+8 matches `{KeyLeftBracket}`.

### 5. Matching

`Shortcut.matches` stays exact. Command dispatch makes two passes per key down:

1. **Exact:** `e.KeyCode == s.Key && e.Modifiers == s.Modifiers`.
2. **Typed**, only if no command matched in pass 1 and
   `e.TypedKey != KeyInvalid`:
   `e.TypedKey == s.Key && e.TypedModifiers == s.Modifiers`.

This is the rule libxkbcommon documents for shortcuts (see Sources). The pass
order settles a press that fits two commands: on US, Ctrl+Shift+= is exact
`{KeyEqual, ModCtrlShift}` and typed `{KeyPlus, ModCtrl}`; the exact one wins.

Widgets that compare `KeyCode` directly keep doing so and gain rules 2, 4 and 5
for free (numpad navigation in inputs, Ctrl+C on Russian).

The two passes run within each dispatch phase: `Global` commands before the
widgets, the others after. A `Global` command's typed match therefore still runs
before a non-global command's exact match, as `Global` already takes precedence.

"Used up" is libxkbcommon's `XKB_CONSUMED_MODE_GTK`, the mode made for
shortcuts: a modifier counts only if it changes what the key produces. The X11
default mode would also count Ctrl and Alt on keys that have a Ctrl+Alt level
(F1 → `XF86Switch_VT_1`).

### 6. Backends

- **Wayland:** libxkbcommon is already loaded. `TypedKey` comes from
  `xkb_state_key_get_one_sym`, `TypedModifiers` from
  `xkb_state_key_get_consumed_mods2`, `ModAltGr` from the `LevelThree` modifier.
  Missing libxkbcommon still falls back to X11, as today.
- **X11:** reads key symbols through xgb, with no new library
  (`gui/backend/gl/keys_x11.go`, `xkbmap_x11.go`). Measured on Xorg 21:
  - The active layout reaches a client that has not called `XkbUseExtension`
    only as a modifier: the compat map's `group 2 = AltGr` turns groups 2-4 into
    Mod5, the bit AltGr also sets, so every key after a layout switch read as
    AltGr held (found on the maintainer's Debian Xorg session). The backend
    therefore calls `XkbUseExtension`, as Xlib does for GTK and Qt; key events
    then carry the group in bits 13-14 and Mod5 only for AltGr. (`xdotool` locks
    the group of the key it sends and restores it afterwards; tests that switch
    layouts inject with `xte`.)
  - Key symbols come from `XkbGetMap` with the KeySyms part only: per key its
    groups × width array and its out-of-range group rule (wrap, clamp,
    redirect), so every group and level is exact. The reply is validated whole
    (length, keycode range, at most four groups, `nSyms` = groups × width, the
    total); one that fails leaves the core keymap in use, never a partial map.
    The core keymap is the fallback without the extension. It cannot be read for
    more: Xorg publishes per key the first group's two levels, the second
    group's two, then the first group's further levels and after them the
    second's, then groups 3 and 4, without saying how many levels each has; so
    it gives two groups and level 3 for the first only.
  - Key types are not read. On a key with one group the width is its type's
    level count, so an explicitly empty level types nothing, as in XKB; on a key
    with more groups the width is the widest group's, and a level is taken to
    exist where it lists a symbol.
  - The modifier map names the modifiers of `Num_Lock` (Mod2) and
    `ISO_Level3_Shift` (Mod5, through a virtual key, not RALT); they are read
    from it, not assumed, and `ModAltGr` follows the level-3 bit it names.
  - A keymap change (setxkbmap, a layout added in the desktop settings) reads
    the symbols and the modifier map again and drops the press records: on
    `XkbNewKeyboardNotify` and `XkbMapNotify`, which an XKB-aware client gets
    instead of the core `MappingNotify` (still handled without the extension).
  - A key the input method forwards (IBus `ForwardKeyEvent`) reports what the
    same press reports without one, when its keycode types the keysym forwarded;
    a keysym the engine made up keeps its own `KeyCode`.

  Where the core protocol and XKB disagree, the path follows XKB, which every
  Xlib and GTK application on the same desktop uses: on a keypad key (either of
  its first two levels a keypad keysym) Shift counts for nothing and only
  NumLock picks the keysym (the protocol would let Shift+keypad 7 type 7), and
  the used-up modifiers are counted as libxkbcommon's GTK mode counts them. Caps
  Lock acts as the key types XKB picks by itself make it act. A key is
  alphabetic when level 1 is a lowercase and level 2 an uppercase letter by
  libxkbcommon's case table, not necessarily each other's (ſ and ẞ on de's
  AltGr+S), and on levels 3 and 4 too when they are such a pair as well
  (FOUR_LEVEL_ALPHABETIC); there Caps Lock inverts Shift, so Shift+Caps Lock
  types the lowercase letter (the protocol would type the capital). Anywhere
  else it capitalizes the keysym typed, the XKB "Caps Lock transformation"
  (AltGr+S types Đ on `al`). Case comes from libxkbcommon's
  `xkb_keysym_to_upper` and `_to_lower` when it is loaded. Unicode's cases would
  be wrong both ways: Georgian puts a Latin capital beside a Mkhedruli letter
  (`ge`: პ/P, not alphabetic), and the same i/İ pair is alphabetic on `tr`
  (declared) but not on `az` (picked by XKB). Characters come from
  libxkbcommon's `xkb_keysym_to_utf32` when it is loaded, else from
  `x11key.KeysymToRune`: Latin-1, the keypad's printable keys, `EuroSign` (`€`)
  and direct-Unicode keysyms, each typed as libxkbcommon types it (a test
  compares the two tables). Without libxkbcommon, text in a script with legacy
  keysyms (Cyrillic, Greek, Hebrew) still types nothing on X11; its shortcuts
  work.

  What only key types say stays beyond the X11 path; the sweep counts these
  presses and compares everything else:
  - a type that orders its levels its own way: cm(azerty)'s digit row (Shift on
    level 4), the four-level keypad types of fr(bepo), dz, ml(fr-oss) (Shift
    picks the digit), Pause and Print (level 2 with Ctrl or Alt, not Shift), and
    which modifiers Shift+AltGr uses up on a four-level keypad key
    (fr(oss_latin9) and ir(pes_keypad) disagree); about 8,600 presses;
  - Caps Lock on a key whose layout names a type that gives Lock a meaning of
    its own: Shift+Caps Lock capitals on us(mac) and fr(ergol), Caps Lock on the
    digit row of fr(bepo) and us(chr), `ẞ` on level 5 for Caps Lock+ß on de,
    Lock used up by FOUR_LEVEL_PLUS_LOCK on fr(dvorak), Turkish i/İ and ı/I
    (`tr` declares them alphabetic, so Caps Lock types İ and I; X11 types I and
    ı); about 1,100 presses;
  - ISO level 5 (Mod3: de(T3), neo).

  On the core keymap fallback, besides: an explicitly empty level up to the
  fourth reads as absent (AltGr on a key whose levels 3 and 4 are both empty
  types the base symbol, ara), a one-level key with a cased letter types the
  capital with Shift as the protocol says (#948), and groups 3 and 4 read as the
  second.

  Reading key types too (`XkbGetMap` with the KeyTypes part) would close the
  first two items: level selection, used-up modifiers and empty levels from the
  types themselves. libxkbcommon-x11 is not used: Debian installs it only with
  Qt. Compiling `_XKB_RULES_NAMES` with libxkbcommon was rejected: it rebuilds
  the configuration the server was given, not the keymap it holds (xkbcomp
  uploads, xmodmap edits).

## Decisions taken

Confirmed by the maintainer on 2026-10-28, on condition that US is not made
worse (see § US regression guard).

| #   | Decision                                 | Taken                                  | Alternative                                   |
| --- | ---------------------------------------- | -------------------------------------- | --------------------------------------------- |
| D1  | Names of the two new `Event` fields      | `TypedKey`, `TypedModifiers`           | `ShortcutKey`, `ConsumedModifiers` (raw mask) |
| D2  | 21 ASCII key constants (§ 3)             | add them now                           | later; `#`, `+`, `<`, `^` stay `KeyInvalid`   |
| D3  | AltGr as `ModAltGr` in `Modifiers` (§ 4) | yes                                    | keep it out; only suppress the exact pass     |
| D4  | NumLock-off keypad keys                  | navigation codes (`KeyHome`)           | always `KeyKP7` (GLFW, physical)              |
| D5  | Dead keys (rule 7)                       | spacing ASCII character                | `KeyInvalid` as today                         |
| D6  | Non-Latin fallback order (rule 5)        | first configured Latin layout, then US | US position only (SDL), a preference setting  |

### US regression guard

With the `us` layout active nothing that works today may change, whether it is
configured alone or as the first, second, third or fourth layout beside others
(`us`, `us,de`, `de,us`, `ru,us`, `de,ru,us`, `de,ru,fr,us`):

- Every key with a `KeyCode` today keeps it, with the same `Modifiers`, for
  every combination of Shift, Ctrl, Alt and Super. Keys that are `KeyInvalid`
  today (numpad, the 105-key `<>` key, NumLock, Menu) gain a code.
- `ModAltGr` is never set: `us` has no level 3.
- The typed pass never fires a shortcut built from a constant that exists before
  this change. Every character US types with Shift
  (`! @ # $ % ^ & * ( ) _ + { } | : " < > ? ~`) maps to a constant from § 3, so
  only new shortcuts can match it.

The one intended change on US: numpad keys with NumLock off now navigate in text
fields.

## Golden tables

From xkeyboard-config 2.42 (libxkbcommon 1.7, Debian trixie). Key names are XKB
names (`AE07` is the key US labels 7). `—` is `KeyInvalid`/none. Modifier
columns list `Modifiers` → `TypedModifiers`.

### `de(nodeadkeys)` — the reporter's layout

| Press            | KeyCode                                | Char | TypedKey         | Modifiers → Typed | Fires                                        |
| ---------------- | -------------------------------------- | ---- | ---------------- | ----------------- | -------------------------------------------- |
| AE07             | `Key7`                                 | 7    | `Key7`           | – → –             | `{Key7}`                                     |
| Shift+AE07       | `Key7`                                 | /    | `KeySlash`       | Shift → –         | `{Key7, Shift}`, else `{KeySlash}`           |
| Ctrl+Shift+AE07  | `Key7`                                 | —    | `KeySlash`       | Ctrl+Shift → Ctrl | `{Key7, CtrlShift}`, else `{KeySlash, Ctrl}` |
| AltGr+AE08       | `Key8`                                 | [    | `KeyLeftBracket` | AltGr → –         | `{KeyLeftBracket}`; not `{Key8}`             |
| AltGr+AD01       | `KeyQ`                                 | @    | `KeyAt`          | AltGr → –         | `{KeyAt}`; not `{KeyQ}`                      |
| AltGr+AE11       | —                                      | \\   | `KeyBackslash`   | AltGr → –         | `{KeyBackslash}`                             |
| BKSL             | `KeyNumberSign`                        | #    | `KeyNumberSign`  | – → –             | `{KeyNumberSign}`                            |
| Shift+BKSL       | `KeyNumberSign`                        | '    | `KeyApostrophe`  | Shift → –         | `{KeyApostrophe}`                            |
| Shift+AB08       | `KeyComma`                             | ;    | `KeySemicolon`   | Shift → –         | `{KeySemicolon}`                             |
| Shift+AE10       | `Key0`                                 | =    | `KeyEqual`       | Shift → –         | `{KeyEqual}`                                 |
| Shift+AE12       | —                                      | \`   | `KeyGraveAccent` | Shift → –         | `{KeyGraveAccent}`                           |
| AD12             | `KeyPlus`                              | +    | `KeyPlus`        | – → –             | `{KeyPlus}`                                  |
| TLDE             | `KeyCircumflex`                        | ^    | `KeyCircumflex`  | – → –             | `{KeyCircumflex}`                            |
| LSGT             | `KeyLess`                              | <    | `KeyLess`        | – → –             | `{KeyLess}`                                  |
| AC11             | —                                      | ä    | —                | – → –             | nothing                                      |
| KP7, NumLock on  | `KeyKP7`                               | 7    | `KeyKP7`         | – → –             | `{KeyKP7}`                                   |
| KP7, NumLock off | `KeyHome`                              | —    | `KeyHome`        | – → –             | `{KeyHome}`; Input moves the caret           |
| KPDL, NumLock on | `KeyKPDecimal`                         | ,    | `KeyKPDecimal`   | – → –             | `{KeyKPDecimal}`                             |
| NMLK, RALT, COMP | `KeyNumLock`, `KeyRightAlt`, `KeyMenu` | —    | same             | – → –             | the named key                                |

`de` (with dead keys) differs only in TLDE (`dead_circumflex` → `KeyCircumflex`
by rule 7) and AE12 (`dead_acute` → `KeyInvalid`).

### `fr` — AZERTY digits

| Press      | KeyCode    | Char | TypedKey        | Modifiers → Typed | Fires                                       |
| ---------- | ---------- | ---- | --------------- | ----------------- | ------------------------------------------- |
| AE01       | `Key1`     | &    | `KeyAmpersand`  | – → –             | `{Key1}`, else `{KeyAmpersand}`             |
| Shift+AE01 | `Key1`     | 1    | `Key1`          | Shift → Shift     | `{Key1, Shift}`                             |
| Ctrl+AE01  | `Key1`     | —    | `KeyAmpersand`  | Ctrl → Ctrl       | `{Key1, Ctrl}`, else `{KeyAmpersand, Ctrl}` |
| AE08       | `Key8`     | \_   | `KeyUnderscore` | – → –             | `{Key8}`, else `{KeyUnderscore}`            |
| AltGr+AE10 | `Key0`     | @    | `KeyAt`         | AltGr → –         | `{KeyAt}`                                   |
| AD01       | `KeyA`     | a    | `KeyA`          | – → –             | `{KeyA}`                                    |
| Shift+AB09 | `KeyColon` | /    | `KeySlash`      | Shift → –         | `{KeySlash}`                                |

Shift+AE01 keeps Shift: the typed key equals `KeyCode` (§ 1).

### `ru` and `us,ru` — non-Latin

| Layout (active)      | Press      | KeyCode    | Char | TypedKey      | Fires           |
| -------------------- | ---------- | ---------- | ---- | ------------- | --------------- |
| `ru`                 | Ctrl+AB03  | `KeyC`     | —    | `KeyC`        | `{KeyC, Ctrl}`  |
| `ru`                 | AB03       | `KeyC`     | с    | `KeyC`        | `{KeyC}`        |
| `ru`                 | Shift+AE07 | `Key7`     | ?    | `KeyQuestion` | `{KeyQuestion}` |
| `ru`                 | LSGT       | `KeySlash` | /    | `KeySlash`    | `{KeySlash}`    |
| `us,ru` (ru)         | Ctrl+AB03  | `KeyC`     | —    | `KeyC`        | `{KeyC, Ctrl}`  |
| `ru,us` (ru)         | Ctrl+AB03  | `KeyC`     | —    | `KeyC`        | `{KeyC, Ctrl}`  |
| `us(dvorak),ru` (ru) | Ctrl+AB03  | `KeyJ`     | —    | `KeyJ`        | `{KeyJ, Ctrl}`  |

The last row is where D6 matters: the configured Latin layout (Dvorak) wins over
the US position. `ru,us` shows the Latin layout may come after the active one.
D6 was confirmed again on 2026-10-29 against a fixed US-position fallback, which
would be order-independent but would move Dvorak and Colemak users' letter
shortcuts on Russian or Greek; a preference API was judged disproportionate.

### Others

| Layout                   | Press            | KeyCode        | Char | TypedKey         | Note                                 |
| ------------------------ | ---------------- | -------------- | ---- | ---------------- | ------------------------------------ |
| `us(dvorak)`             | AB03             | `KeyJ`         | j    | `KeyJ`           | letters by layout, not position      |
| `ch`                     | Shift+AE01       | `Key1`         | +    | `KeyPlus`        |                                      |
| `ch`                     | AltGr+AD11       | —              | [    | `KeyLeftBracket` |                                      |
| `ch`                     | BKSL             | `KeyDollar`    | $    | `KeyDollar`      |                                      |
| `br`                     | AB11             | `KeySlash`     | /    | `KeySlash`       | ABNT2 extra key                      |
| `br`                     | KPDL, NumLock on | `KeyKPDecimal` | ,    | `KeyKPDecimal`   | `KP_Separator`                       |
| `jp`                     | AE13 and AB11    | `KeyBackslash` | \\   | `KeyBackslash`   | two keys, one code: the layout's own |
| `jp`                     | AD11             | `KeyAt`        | @    | `KeyAt`          |                                      |
| `ru`, `jp`, `us(dvorak)` | RALT             | `KeyRightAlt`  | —    | `KeyRightAlt`    | `Alt_R`, so `ModAlt`, not `ModAltGr` |
| `us,de,ru,fr`, fr active | AE01             | `Key1`         | &    | `KeyAmpersand`   | the fourth group: X11 reads XKB map  |
| `gr`                     | AB03             | `KeyC`         | ψ    | `KeyC`           |                                      |
| `pl`                     | AltGr+AC01       | `KeyA`         | ą    | `KeyA`           | ą has no KeyCode                     |
| `tr`, Caps Lock          | AC11             | `KeyI`         | İ    | `KeyI`           | declared type; X11: I (no key types) |

## Verification

1. **Golden tests** (`gui/backend/gl`, Linux): the tables above as key-sequence
   tests in the style of libxkbcommon's `test/keyseq.c`. Keymaps are compiled
   from layout names at test time; nothing from xkeyboard-config is checked in.
2. **US regression test:** every key of `us` × {none, Shift, Ctrl, Ctrl+Shift,
   Alt, Super} against today's `KeyCode`/`Modifiers`, as § US regression guard
   states, with US active in each group position of § US regression guard, on
   Wayland and through the X11 XKB map (the core keymap on `us` alone).
3. **All-layout sweep:** for every layout and variant in `evdev.xml`, every key,
   with none/Shift/AltGr/Shift+AltGr, each with Caps Lock off and on:
   - `EventChar` equals libxkbcommon's character, on both paths;
   - every key that types something or has a name gets a `KeyCode`, or is listed
     as an exception;
   - key up repeats key down's codes;
   - `ModAltGr` is set only with level 3;
   - Caps Lock and NumLock never appear in `Modifiers`.
4. **X11 core path against Wayland:** the tests build the core keymap Xorg
   derives for a layout from libxkbcommon's compile of it, in the column order
   measured on Xorg (§ 6), and press keys through the X11 path. The golden
   tables run on both paths; the sweep checks that X11 reports what Wayland does
   on every key the core keymap can describe.
5. **Backend check:** the Docker Xorg harness (Xorg with the dummy driver, keys
   injected by keycode with `xdotool`) runs the reporter's probe built from the
   branch: the de(nodeadkeys) and ru presses of the golden tables give the same
   results on real Xorg, and a `setxkbmap` switch while the window is open (us,
   de, ru) takes effect on the next press.
6. **Hardware:** the reporter (KDE, Wayland) and a Debian laptop.

CI installs `libxkbcommon0` and `xkb-data`. `GOGUI_REQUIRE_XKB=1` turns the
tests' skip into a failure there, as `GOGUI_REQUIRE_GL` does. Under `-race` the
sweep checks every tenth layout: it is one goroutine, so the race detector adds
nothing, and the full sweep takes ~100 s there instead of ~10 s.

## Phasing

| Phase | Content                                                                 | Status  |
| ----- | ----------------------------------------------------------------------- | ------- |
| 1     | This spec; decisions D1–D6                                              | done    |
| 2     | Test harness: libxkbcommon bindings for tests, golden tables, sweep     | done    |
| 3     | `KeyCode` rules (§ 2), new constants (§ 3), `ModAltGr`; Wayland         | done    |
| 4     | `TypedKey`/`TypedModifiers`, two-pass matching; Wayland                 | done    |
| 5     | X11 core path: NumLock, level 3, groups, consumed, characters           | done    |
| 6     | Docs: `docs/deployment.md`, CHANGELOG, a keyboard note in the guides    | pending |
| 7     | Hardware: reporter (KDE, Wayland), Debian laptop (X11 layout switching) | pending |

## Sources consulted

Read for understanding; no code, comments or tables were copied. Qt, GTK and
Firefox are under copyleft licenses and must not be ported.

- libxkbcommon, "Consumed Modifiers" in `include/xkbcommon/xkbcommon.h` — the
  matching rule of § 5.
- X Window System Protocol, § 5 Keyboards — the NumLock rule.
- The X Keyboard Extension: Protocol Specification — levels, groups.
- xkeyboard-config, `symbols/*` and `rules/evdev.xml` — the golden tables.
- W3C UI Events (legacy `keyCode`, `code` and `key` values).
- GTK 4 `gdk_key_event_matches`, Qt `QXkbCommon::possibleKeyCombinations` and
  `lookupLatinKeysym`, Firefox `KeymapWrapper::ComputeDOMKeyCode`, SDL3 keycode
  options, GLFW `createKeyTables`, Chromium `DifficultKeyboardCode`.

## Rejected Approaches

- **Physical key codes as the fix** (GLFW, W3C `code`): `KeyZ` would be the key
  labelled Y on QWERTZ, and none of the reported shortcuts would work.
- **Per-layout tables in go-gui:** fix one layout and miss the next.
- **Unicode code points as `KeyCode`** (SDL3): U+0100–U+017F collides with the
  named keys at 256–348.
- **Chromium's character-to-Windows-key tables:** large, carry their own
  copyright, and tie Linux to Windows semantics.
- **libxkbcommon-x11 for the X11 path:** Debian pulls it in only with Qt; GTK,
  XFCE and MATE machines often lack it.
- **A full XKB-extension client in Go:** re-implements libxkbcommon-x11. The X11
  path sends three XKB requests only (`UseExtension`, `SelectEvents`, `GetMap`
  with KeySyms), the smallest set that reports the group and reads every group's
  symbols.
- **Finding groups 3 and 4 in the core keymap** (counting its empty columns):
  the core mapping does not say how many levels each group has, so no reading of
  it finds where group 3 starts.
- **Compiling `_XKB_RULES_NAMES` with libxkbcommon on X11:** rebuilds the
  configuration the server was given, not the keymap it holds; wrong after an
  `xkbcomp` upload or an `xmodmap` edit, and still needs a fallback without
  libxkbcommon.
- **Bundling libxkbcommon:** needs cgo.
- **Porting GTK, Qt or Firefox code:** LGPL, GPL and MPL; go-gui is MIT.
