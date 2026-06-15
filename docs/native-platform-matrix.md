# Native Platform Matrix

Feature support per backend and operating system. ✓ = functional, ✗ = stub/unavailable.

## Consolidated Feature Matrix

| Feature        | macOS (Metal) | Linux (GL) | Linux (SDL2) | Windows (GL) | Windows (SDL2) | Web (WASM) | Android | iOS |
| -------------- | :-----------: | :--------: | :----------: | :----------: | :------------: | :--------: | :-----: | :-: |
| Open file      |       ✓       |     ✓      |      ✓       |      ✓       |       ✓        |     ✓      |    ✗    |  ✗  |
| Save file      |       ✓       |     ✓      |      ✓       |      ✓       |       ✓        |     ✓¹     |    ✗    |  ✗  |
| Folder dialog  |       ✓       |     ✓      |      ✓       |      ✓       |       ✓        |     ✓²     |    ✗    |  ✗  |
| Message dialog |       ✓       |     ✓      |      ✓       |      ✓       |       ✓        |     ✓      |    ✗    |  ✗  |
| Confirm dialog |       ✓       |     ✓      |      ✓       |      ✓       |       ✓        |     ✓      |    ✗    |  ✗  |
| Save/discard   |       ✓       |     ✓      |      ✓       |      ✓       |       ✓        |     ✗      |    ✗    |  ✗  |
| Print dialog   |       ✓       |     ✓⁴     |      ✓⁴      |      ✓⁵      |       ✓⁵       |     ✓⁶     |    ✗    |  ✗  |
| Notifications  |      ✓⁷       |     ✓⁸     |      ✓⁸      |      ✓⁹      |       ✓⁹       |    ✓¹⁰     |    ✓    |  ✗  |
| A11y tree sync |      ✓¹¹      |    ✓¹²     |     ✓¹²      |      ✗       |       ✗        |    ✓¹³     |    ✓    |  ✗  |
| IME input      |       ✓       |     ✓      |      ✓       |      ✓       |       ✓        |     ✓      |    ✓    |  ✗  |
| Native menubar |      ✓¹⁴      |     ✗      |      ✗       |      ✗       |       ✗        |     ✗      |    ✗    |  ✗  |
| System tray    |      ✓¹⁵      |    ✓¹⁶     |     ✓¹⁶      |     ✓¹⁶      |      ✓¹⁶       |     ✗      |    ✗    |  ✗  |
| Spell check    |      ✓¹⁷      |    ✓¹⁸     |     ✓¹⁸      |      ✗       |       ✗        |     ✗      |    ✗    |  ✗  |
| Open URI       |       ✓       |     ✓      |      ✓       |      ✓       |       ✓        |     ✓      |    ✓    | ✗¹⁹ |
| Dark titlebar  |       ✗       |     ✗      |      ✗       |      ✗       |       ✗        |     ✗      |    ✗    |  ✗  |
| Bookmarks      |       ✗       |     ✗      |      ✗       |      ✗       |       ✗        |     ✗      |    ✗    |  ✗  |

¹ Web save uses File System Access API (`showSaveFilePicker`); falls back to suggested filename.  
² Web folder uses `showDirectoryPicker`.  
³ (removed — Save/Discard/Cancel now implemented on Linux and Windows)  
⁴ Linux: PDF rendered to temp file, opened via `lpr` or `xdg-open`.  
⁵ Windows: PDF rendered to temp file, opened via `ShellExecuteW "print"`.  
⁶ Web: renders canvas to PNG in hidden iframe, calls `window.print()`.  
⁷ macOS: `osascript` display notification.  
⁸ Linux: `notify-send` via D-Bus.  
⁹ Windows: PowerShell `System.Windows.Forms.NotifyIcon` balloon tip (GL and SDL2).  
¹⁰ Web: `Notification` API with permission request.  
¹¹ macOS: NSAccessibility protocol via C bridge (VoiceOver).  
¹² Linux: AT-SPI D-Bus via `atspi` bridge.  
¹³ Web: DOM ARIA attributes on canvas-adjacent elements. Windows lacks both UIA and AT-SPI bridges.  
¹⁴ macOS: `NSMenu`/`NSMenuItem` via C bridge. Native menubar is an AppKit-only concept.  
¹⁵ macOS: `NSStatusBar` via C bridge.  
¹⁶ Linux: StatusNotifierItem D-Bus (`sni` package). Windows: `Shell_NotifyIconW` (`sni` package).  
¹⁷ macOS: NSSpellChecker via C bridge.  
¹⁸ Linux requires `hunspell` build tag and `libhunspell-dev` at build time.  
¹⁹ iOS validates URI scheme but returns "not implemented".

## Dialogs

| Feature             | macOS (Metal) | Linux (GL/SDL2) | Windows (GL/SDL2) | Web (WASM) | Android | iOS |
| ------------------- | :-----------: | :-------------: | :---------------: | :--------: | :-----: | :-: |
| Open file           |       ✓       |        ✓        |         ✓         |     ✓      |    ✗    |  ✗  |
| Save file           |       ✓       |        ✓        |         ✓         |     ✓¹     |    ✗    |  ✗  |
| Open folder         |       ✓       |        ✓        |         ✓         |     ✓²     |    ✗    |  ✗  |
| Message (alert)     |       ✓       |        ✓        |         ✓         |     ✓      |    ✗    |  ✗  |
| Confirm (OK/Cancel) |       ✓       |        ✓        |         ✓         |     ✓      |    ✗    |  ✗  |
| Save/Discard/Cancel |       ✓       |        ✓        |         ✓         |     ✗      |    ✗    |  ✗  |

¹ Web save uses File System Access API (`showSaveFilePicker`); falls back to suggested filename.  
² Web folder uses `showDirectoryPicker`.  
³ Linux: zenity `--question --extra-button Discard` or kdialog `--warningyesnocancel`. Windows: `MessageBoxW` `MB_YESNOCANCEL`.

## Printing

| Feature      | macOS (Metal) | Linux (GL/SDL2) | Windows (GL/SDL2) | Web (WASM) | Android | iOS |
| ------------ | :-----------: | :-------------: | :---------------: | :--------: | :-----: | :-: |
| Print dialog |       ✓       |       ✓¹        |        ✓²         |     ✓³     |    ✗    |  ✗  |

¹ Linux: PDF rendered to temp file, opened via `lpr` or `xdg-open`.  
² Windows: PDF rendered to temp file, opened via `ShellExecuteW "print"`.  
³ Web: renders canvas to PNG in hidden iframe, calls `window.print()`.

## Notifications

| Feature         | macOS (Metal) | Linux (GL/SDL2) | Windows (GL/SDL2) | Web (WASM) | Android | iOS |
| --------------- | :-----------: | :-------------: | :---------------: | :--------: | :-----: | :-: |
| OS notification |      ✓¹       |       ✓²        |        ✓³         |     ✓⁴     |    ✓    |  ✗  |

¹ macOS: `osascript` display notification.  
² Linux: `notify-send` via D-Bus.  
³ Windows: PowerShell `System.Windows.Forms.NotifyIcon` balloon tip (GL and SDL2).  
⁴ Web: `Notification` API with permission request.

## Accessibility

| Feature        | macOS (Metal) | Linux (GL/SDL2) | Windows (GL/SDL2) | Web (WASM) | Android | iOS |
| -------------- | :-----------: | :-------------: | :---------------: | :--------: | :-----: | :-: |
| Init/Sync      |      ✓¹       |       ✓²        |         ✗         |     ✓³     |    ✓    |  ✗  |
| Announce       |       ✓       |        ✓        |         ✗         |     ✓      |    ✓    |  ✗  |
| Full tree sync |       ✓       |        ✓        |         ✗         |     ✓      |    ✓    |  ✗  |

¹ macOS: NSAccessibility protocol via C bridge (VoiceOver).  
² Linux: AT-SPI D-Bus via `atspi` bridge.  
³ Web: DOM ARIA attributes on canvas-adjacent elements.  
Windows lacks both UIA and AT-SPI bridges — a11y is not functional there.

## IME / Text Input

| Feature     | macOS (Metal) | Linux (GL/SDL2) | Windows (GL/SDL2) | Web (WASM) | Android | iOS |
| ----------- | :-----------: | :-------------: | :---------------: | :--------: | :-----: | :-: |
| Start/Stop  |       ✓       |        ✓        |         ✓         |     ✓      |    ✓    |  ✗  |
| Cursor rect |       ✓       |        ✓        |         ✓         |     ✓      |    ✓    |  ✗  |

macOS/GL/SDL2: via SDL2 text input API. Android: native Kotlin bridge. Web: hidden `<input>` element.

## Native Menubar

| Feature           | macOS (Metal) | Linux (GL/SDL2) | Windows (GL/SDL2) | Web (WASM) | Android | iOS |
| ----------------- | :-----------: | :-------------: | :---------------: | :--------: | :-----: | :-: |
| Set/Clear menubar |      ✓¹       |        ✗        |         ✗         |     ✗      |    ✗    |  ✗  |

¹ macOS: `NSMenu`/`NSMenuItem` via C bridge. Native menubar is an AppKit-only concept.

## System Tray

| Feature              | macOS (Metal) | Linux (GL/SDL2) | Windows (GL/SDL2) | Web (WASM) | Android | iOS |
| -------------------- | :-----------: | :-------------: | :---------------: | :--------: | :-----: | :-: |
| Create/Update/Remove |      ✓¹       |       ✓²        |        ✓²         |     ✗      |    ✗    |  ✗  |

¹ macOS: `NSStatusBar` via C bridge.  
² Linux: StatusNotifierItem D-Bus (`sni` package). Windows: `Shell_NotifyIconW` (`sni` package).

## Spell Check

| Feature       | macOS (Metal) | Linux (GL/SDL2)¹ | Windows (GL/SDL2) | Web (WASM) | Android | iOS |
| ------------- | :-----------: | :--------------: | :---------------: | :--------: | :-----: | :-: |
| Check/Suggest |       ✓       |        ✓         |         ✗         |     ✗²     |    ✗    |  ✗  |
| Learn word    |       ✓       |        ✓         |         ✗         |     ✗      |    ✗    |  ✗  |

¹ Linux requires `hunspell` build tag and `libhunspell-dev` at build time. Without the tag, falls to `spellcheck_other.go` stub.  
² Web: "no browser JS API exposes spell results" — upstream limitation.

## Security-Scoped Bookmarks

| Feature           | All platforms |
| ----------------- | :-----------: |
| Load/Persist/Stop |       ✗       |

Bookmark support (macOS security-scoped URLs for sandboxed file access) is unimplemented on all backends. The macOS C dialog layer parses `bookmarkData` from `NSOpenPanel`, but the Go side does not consume it.

## Titlebar Dark

| Feature       | All platforms |
| ------------- | :-----------: |
| Dark titlebar |       ✗       |

No-op on all backends. Requires platform-specific window manager calls (macOS `NSWindow.appearance`, Windows `DwmSetWindowAttribute`, Linux `GTK_CSD`).

## URI Opening

| Feature | macOS (Metal) | Linux (GL/SDL2) | Windows (GL/SDL2) | Web (WASM) | Android | iOS |
| ------- | :-----------: | :-------------: | :---------------: | :--------: | :-----: | :-: |
| OpenURI |      ✓¹       |       ✓²        |        ✓²         |     ✓³     |    ✓    | ✗⁴  |

¹ macOS: `open` command with scheme validation (http, https, mailto).  
² Linux: `xdg-open`. Windows: `rundll32 url.dll,FileProtocolHandler`.  
³ Web: `window.open(uri, '_blank')`.  
⁴ iOS validates URI scheme but returns "not implemented".

## Backend Selection Summary

| OS      | Default Backend | Alt Backend |        Build Tag        |
| ------- | :-------------: | :---------: | :---------------------: |
| macOS   |      Metal      |     GL      |    `darwin && !ios`     |
| Linux   |      SDL2       |     GL      | `!darwin && !js && !gl` |
| Windows |      SDL2       |     GL      | `!darwin && !js && !gl` |
| Web     |   Web (WASM)    |      —      |      `js && wasm`       |
| Android |     Android     |      —      |        `android`        |
| iOS     |       iOS       |      —      |          `ios`          |

GL backend is selected with `-tags gl`. On macOS, GL can be forced with `-tags gl` (overrides the Metal default). SDL2 is the default on Linux and Windows when no `gl` tag is present.
