# Deploying your app

`go build` produces a bare binary. `cmd/buildapp` turns it into something you
can hand to a user: a signed `.app` on macOS, an icon-embedded `.exe` in a
`.zip` on Windows, a menu-installable tarball on Linux. One binary in, one
artefact out.

| Platform | Output                               | What packaging adds                        |
| -------- | ------------------------------------ | ------------------------------------------ |
| macOS    | signed `.app` (+ `.dmg` with `-dmg`) | `Info.plist`, `.icns` icon, code signature |
| Windows  | `.zip` holding the `.exe`            | icon resource embedded in the PE image     |
| Linux    | `.tar.gz`                            | `.desktop` entry, icon, `install.sh`       |

The full flag reference lives in
[`cmd/buildapp/README.md`](../cmd/buildapp/README.md). This page covers the
standard path end to end.

## Step 1: compile for the target

A C toolchain is needed only for the mobile backends. macOS, Linux and Windows
(desktop) build fully cgo-free — unless the app opts into `-tags hunspell`,
which pulls in C Hunspell for spell checking and needs a C toolchain with
libhunspell-dev. macOS builds are for Apple silicon (darwin/arm64) only: a
darwin/amd64 (Intel) build stops at link.

```bash
# Windows (amd64). -H windowsgui marks the PE as a GUI-subsystem image:
# without it the loader allocates a console, so an empty terminal window
# opens behind the app window.
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 \
  go build -ldflags "-H windowsgui" -o build/myapp.exe ./myapp/

# Linux (amd64)
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
  go build -o build/myapp ./myapp/

# macOS (Apple silicon)
CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 \
  go build -o build/myapp ./myapp/
```

Or let buildapp compile for you with `-build-pkg` (same flags, one step —
`-H windowsgui` is added on Windows automatically). It takes no positional
binary then:

```bash
go run ./cmd/buildapp -platform windows -arch amd64 -build-pkg ./myapp/ \
  -o build -name "My App" -icon icon.png
```

The macOS binary cross-compiles from any OS, but only for darwin/arm64: Intel
Macs are not supported. Packaging still runs on a Mac: the signed `.app` needs
`sips`, `iconutil` and `codesign` (`hdiutil` for `-dmg`).

Stage the binary under a clean name before packaging: the executable's basename
becomes the installed program name (the `Exec=` line on Linux, the file inside
the `.zip` on Windows, `Contents/MacOS/<name>` on macOS). `myapp-linux` in means
`Exec=myapp-linux` out.

On Linux/X11, transparent and translucent windows need a running compositing
manager at runtime — without one the window renders black, which `gui.Debug`
reports. Either depend on one in the package or document it for the user. See
`docs/specs/transparent-windows.md`.

On a Wayland desktop (`WAYLAND_DISPLAY` set) the app uses the native Wayland
backend on amd64 and arm64. `GOGUI_WAYLAND=0` makes it run through XWayland
instead, and other CPUs always do. The backend opens these libraries at run
time, so the binary does not link them:

| Library                        | Debian / Ubuntu package   | Without it                                   |
| ------------------------------ | ------------------------- | -------------------------------------------- |
| `libwayland-client.so.0`       | `libwayland-client0`      | warning, falls back to X11                   |
| `libwayland-egl.so.1`          | `libwayland-egl1`         | warning, falls back to X11                   |
| `libxkbcommon.so.0`            | `libxkbcommon0`           | warning, falls back to X11                   |
| `libdecor-0.so.0` and a plugin | `libdecor-0-plugin-1-gtk` | falls back to X11 on GNOME, Cinnamon, weston |

KDE and wlroots compositors (sway) draw the frame themselves and need no
libdecor. See `docs/specs/wayland-backend.md`.

On a Linux machine without a GPU, EGL falls back to Mesa's llvmpipe, which draws
on the CPU at two to three times the CPU of go-gui's own software renderer while
animating, and about twice the memory (#1265). So when `GOGUI_RENDERER` is not
set, the backend checks at start which GL it would get. If `GL_RENDERER` names a
software rasterizer (llvmpipe, softpipe, lavapipe, SwiftShader), it draws with
go-gui's software renderer instead, and writes one log line that says so. On X11
an EGL display that cannot open also picks the software renderer. On native
Wayland it fails the Wayland setup instead, and the app falls back to X11, which
checks again. A Wayland desktop can hand clients llvmpipe on a machine whose GPU
works under X11 (Cinnamon does), so the check reads the GL that the Wayland
connection gets. The software renderer draws the full frame on each redraw and
draws custom shaders' children but not their shader fill. When the check picked
it, the GL driver it loaded stays in memory, about 70 MB with llvmpipe; set
`GOGUI_RENDERER=soft` to load no GL driver at all.

If EGL opens a display but the check cannot create a GL context, the backend
keeps GL: the window uses its own EGL config, which can still work. If the
window's own context then fails, startup returns `ErrNoGPUContext`; set
`GOGUI_RENDERER=soft` to use the software renderer.

`GOGUI_RENDERER=soft` uses the software renderer on any machine, without the
check and without loading EGL. On X11 it uploads frames through MIT-SHM, or with
`PutImage` when the X server cannot map the shared memory (a remote display); on
native Wayland it commits `wl_shm` buffers, so it needs no `libwayland-egl`.
`GOGUI_RENDERER=gl` keeps GL without the check, and then a machine with no GPU
context fails with `ErrNoGPUContext`. Other values keep GL and log a warning.

On Windows, `GOGUI_RENDERER=soft` draws with the same software renderer and
copies each frame to the window through GDI (#1310). The process then loads no
`opengl32.dll`, neither the system one nor a Mesa DLL beside the exe, so an app
starts on a machine whose OpenGL cannot run it: a VM or an RDP session without a
GPU driver, where Windows has only its generic OpenGL 1.1 and startup otherwise
fails with `ErrNoGPUContext`. Frames are paced to the monitor's refresh rate. A
`WindowCfg.Transparent` window is opaque on this path, and custom shaders draw
their children but not their shader fill.

With the variable unset, Windows tries GL for the first window. If that window
gets no WGL context, or cannot bind the GL functions the backend needs, as on
Microsoft's generic OpenGL 1.1, it opens on the software renderer instead, and
so does every later window of the process. One log line names the step that
failed and its cause, with `GL_VERSION` and `GL_RENDERER` when a context was
made, and says that custom shaders are not drawn. The `opengl32.dll` the GL
attempt loaded stays in memory. A machine whose GL comes up stays on GL, an app
that ships Mesa's `opengl32.dll` beside the exe included, typically to keep
custom shaders. In a process whose first window is on GL, a later window whose
GL fails returns `ErrNoGPUContext` rather than falling back alone, and so does a
GL that comes up but cannot build the built-in shaders: that is a driver bug the
software renderer would hide. `GOGUI_RENDERER=gl` reports the error instead of
falling back. The choice is made once per process: after a session moves between
the console and RDP, restart the app. One case is outside it: a window the app
opens while the first one is still starting, from another thread or from its log
writer, does not wait for the choice. It tries GL with the same fallback on its
own, so it could end on another renderer than the first window on a machine
where the two attempts differ.

On X11 libxkbcommon is optional. Keys, shortcuts and switching between keyboard
layouts work without it; with it, the X11 backend also types letters of scripts
with legacy keysyms (Cyrillic, Greek, Hebrew) and capitalizes them under Caps
Lock as Wayland does. Every desktop that runs GTK or Qt has it installed.
`examples/key_inspector` shows what each key press reports on a machine; see
`docs/specs/keyboard-layouts.md`.

## Step 2: write the app manifest

Keep the app's identity in one file, `appinfo.toml`, beside `main.go`. Then the
name, ID and version are not typed again in each Makefile target, CI workflow
and Go file.

```toml
id      = "com.example.myapp"
name    = "My App"
version = "1.0.0"
icon    = "icon.png"
```

buildapp reads this file (step 3). The app reads the same bytes at run time, so
the bundle ID and the runtime ID are the same. If they differ, macOS keeps
preferences and permission grants under two identities and reports no error.

## Reading the manifest at run time

Embed the file and give it to `gui.WindowCfg.AppInfo`. The window takes its
title from `name`. The app takes its file-access ID, X11 `WM_CLASS` and native
menubar name from the file. A value that you set in Go wins over the file.

```go
//go:embed appinfo.toml
var manifest []byte

// info is parsed once at start-up. MustParse panics on a bad file: the
// file is compiled into the program, so a bad one is a build mistake.
var info = appinfo.MustParse(manifest)

func newWindow() *gui.Window {
	// AppInfo gives the window its title, and the app its file-access
	// ID, X11 WM_CLASS and menubar name. No other place spells them.
	return gui.NewWindow(gui.WindowCfg{
		AppInfo: info,
		State:   &App{},
		Width:   480,
		Height:  320,
		OnInit: func(w *gui.Window) {
			w.SetView(mainView)
		},
	})
}
```

This code is from `examples/app_manifest/main.go`. The format and the flag
precedence are in
[`cmd/buildapp/README.md`](../cmd/buildapp/README.md#app-manifest).

## Step 3: package with buildapp

Run buildapp from the directory that holds `appinfo.toml`, or pass
`-manifest path`. A flag that you give wins over the file. Without a manifest,
give the identity as flags (`-name`, `-id`, `-version`, `-icon`), as below.

A `.png` icon works on every platform. macOS also accepts `.icns`, Windows also
accepts `.ico`.

```bash
# Windows → myapp-1.0.0-windows-amd64.zip
go run ./cmd/buildapp -platform windows -o build -version 1.0.0 \
  -name "My App" -icon icon.png build/myapp.exe

# Linux → myapp-1.0.0-linux-amd64.tar.gz
go run ./cmd/buildapp -platform linux -o build -version 1.0.0 \
  -name "My App" -icon icon.png build/myapp

# macOS → "My App.app" plus my-app-1.0.0.dmg (run on a Mac:
# needs sips, iconutil, codesign, hdiutil for -dmg)
go run ./cmd/buildapp -o build -dmg -version 1.0.0 \
  -name "My App" -icon icon.png build/myapp
```

`-platform` defaults to the host `GOOS`, so the macOS invocation above omits it.
The binary must match the platform: Mach-O for `darwin`, PE for `windows`, ELF
for `linux`. `make release` runs this whole flow over `examples/showcase`.

## Icons

- Windows embeds the icon by appending a `.rsrc` section (`RT_ICON` /
  `RT_GROUP_ICON`). A PNG is embedded verbatim (valid since Vista); a `.ico`
  contributes all of its images.
- Refused when the binary already carries a resource directory, which happens
  when a `.syso` file was linked in. Use one or the other, not both.
- On Linux the `.desktop` entry sets `Terminal=false`, so no terminal emulator
  opens beside the app. Install with `./install.sh` (copies into `~/.local`, no
  privileges needed) or `./install.sh /usr/local` with `sudo` for a system-wide
  install.

## macOS signing

The bundle is always signed, ad-hoc (`-`) by default. Ad-hoc is fine for local
testing and for freshly downloaded releases, but every rebuild changes the
cdhash, which silently revokes TCC-gated permissions (camera, microphone, screen
recording, accessibility, full disk access) granted to the previous build. For a
stable identity during development, create a self-signed Code Signing
certificate (Keychain Access → Certificate Assistant → Create a Certificate) and
pass it explicitly:

```bash
go run ./cmd/buildapp -sign "My Dev Cert" -name "My App" build/myapp
```

or set it once per machine with `BUILDAPP_SIGN_IDENTITY`. Verify with
`codesign -dv --verbose=4 "My App.app"` (ad-hoc prints `Signature=adhoc`; a real
identity prints an `Authority=` line).

`-bundle-deps` copies non-system dylibs into `Contents/Frameworks` and rewrites
their load paths — needed only if the app links libraries outside `/usr/lib` and
`/System`.

Distribution signing (hardened runtime, entitlements, notarization with
`notarytool`) is built in: pass `-entitlements` for the hardened-runtime
signature and add `-notarize -notary-profile <profile>` to submit and staple.
Without `-entitlements`, `-notarize` is an error. See
[`cmd/buildapp/README.md`](../cmd/buildapp/README.md#distribution-signing-and-notarization).
The Windows `.exe` is not Authenticode signed — that is a separate issue; run
`signtool` separately if needed.

## Mobile

Mobile targets build through the standard Go mobile tooling rather than buildapp
— see `make build-ios`
(`CGO_ENABLED=1 GOOS=ios GOARCH=arm64 go build -buildmode=c-archive -tags ios`
for `examples/ios_demo`, needs the Xcode SDK) and `make build-android`
(`gomobile bind -target=android/arm64,android/amd64 -androidapi 24` to an `.aar`
for `examples/android_demo`) in the `Makefile`.

On mobile, the app still reads the embedded `appinfo.toml` at run time, so the
per-app storage ID is the manifest ID. But the package ID comes from the host
project: the Xcode project's `Info.plist` on iOS and the Gradle `applicationId`
on Android. The Go tools build a library (`c-archive` or `.aar`) and do not
write those files, so buildapp cannot set them. Copy the manifest `id` into the
host project by hand.

## Worked example

`examples/app_manifest` has an `appinfo.toml`, so buildapp needs no identity
flags:

```bash
go build -o /tmp/app_manifest ./examples/app_manifest/
cd examples/app_manifest
go run ../../cmd/buildapp -o /tmp /tmp/app_manifest
open "/tmp/App Manifest Demo.app"  # macOS; on Linux/Windows add -platform
```
