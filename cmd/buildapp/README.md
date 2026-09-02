# buildapp

Packages a compiled go-gui binary as a release artefact. One binary in, one
artefact out.

| Platform | Output                 | What it adds                                                        |
| -------- | ---------------------- | ------------------------------------------------------------------- |
| macOS    | signed `.app`          | `Info.plist`, `.icns` icon, code signature, optional bundled dylibs |
| Windows  | `.zip` with the `.exe` | icon resource in the PE image                                       |
| Linux    | `.tar.gz`              | `.desktop` entry, icon, `install.sh`                                |

The macOS bundle can be double-clicked, dragged to `/Applications`, and shows in
the Dock with a proper name and icon. The Linux archive installs into `~/.local`
and shows in the application menu. The Windows `.exe` shows its icon in
Explorer, the taskbar and Alt-Tab.

## Install

```
go install github.com/go-gui-org/go-gui/cmd/buildapp@latest
```

Or run directly from the repo:

```
go run ./cmd/buildapp [flags] <binary>
```

## Usage

```
buildapp [-platform darwin|windows|linux] [-o outdir] [-name Name] [-id bundle.id]
         [-icon icon.png|.icns|.ico] [-version 1.0] [-sign identity] <binary>
```

Positional arg: path to the compiled executable. It must match `-platform`:
Mach-O for `darwin`, PE for `windows`, ELF for `linux`.

**The executable's basename becomes the installed program name** (the `Exec=`
line on Linux, the file inside the `.zip` on Windows, `Contents/MacOS/<name>` on
macOS). Stage it under a clean name before packaging; `showcase-linux` in,
`Exec=showcase-linux` out.

| Flag           | Default                        | Purpose                                                    |
| -------------- | ------------------------------ | ---------------------------------------------------------- |
| `-platform`    | host `GOOS`                    | Target packager: `darwin`, `windows` or `linux`            |
| `-o`           | `.`                            | Output directory                                           |
| `-name`        | binary basename, capped        | Bundle display name                                        |
| `-id`          | `local.gogui.<name>`           | `CFBundleIdentifier`                                       |
| `-icon`        | none                           | `.png` everywhere; also `.icns` (macOS), `.ico` (Windows)  |
| `-version`     | `1.0`                          | `CFBundleVersion` / short version                          |
| `-bundle-deps` | `false`                        | macOS: bundle non-system dylibs into `Contents/Frameworks` |
| `-sign`        | `$BUILDAPP_SIGN_IDENTITY`, `-` | macOS: `codesign` identity. `-` is ad-hoc                  |

## macOS

### Signing

`buildapp` always signs the bundle. An unsigned bundle makes Gatekeeper report
the app as "damaged", even when every binary inside is individually signed. An
unsigned Mach-O does not load at all on Apple Silicon.

The default identity is `-`, which is **ad-hoc**: no certificate, no team
identifier.

```
Signature=adhoc
TeamIdentifier=not set
CDHash=6ec1c5c95e15276640294221cfd868ab6e073487
```

An ad-hoc signature gives TCC (the macOS privacy database) no designated
requirement to key a grant against, so TCC falls back to the **cdhash**. Every
rebuild produces a new cdhash, so **every rebuild silently revokes every
TCC-gated permission the app holds**. The permissions are screen recording,
microphone, camera, accessibility, input monitoring, and full disk access.

The failure is actively misleading. The System Settings row survives, because
that list is keyed by bundle id for display. The authorization check is keyed by
cdhash. The permission looks granted and the API returns denied, with nothing in
any log connecting the two. Recovery is `tccutil reset <service> <bundle-id>`, a
relaunch, and a re-grant — after every build.

Pass a stable identity to key the grant on the certificate instead of the hash:

```
buildapp -sign "My Dev Cert" ...
```

or set it once per machine (fish):

```fish
set -Ux BUILDAPP_SIGN_IDENTITY "My Dev Cert"
```

`-sign` wins over the environment variable. A **self-signed** code-signing
certificate is enough. No Apple Developer account is needed. Create one in
Keychain Access → Certificate Assistant → Create a Certificate, with type "Code
Signing". Then verify that it is visible to `codesign`:

```
security find-identity -v -p codesigning
```

Verify what a bundle actually carries:

```
codesign -dv --verbose=4 Foo.app
```

Ad-hoc prints `Signature=adhoc`. A real identity prints an `Authority=` line. To
verify the TCC fix end to end, grant the app a permission and rebuild. Then
verify that the permission still works without a re-grant.

Verified on macOS 26 with a self-signed certificate. Falcon
(`github.com.go-gui-org.go-term`) kept its Screen Recording grant across a
rebuild that moved the cdhash from `573a437d…` to `bfa13ddf…`. It had
`TeamIdentifier=not set` throughout. A certificate is enough. An Apple-issued
one is not required.

Release builds in CI have no certificate and stay ad-hoc. That is fine — freshly
downloaded apps have no grants to lose.

The bundle-level signature uses `codesign --force --deep`. Apple deprecates
`--deep` for distribution signing. buildapp keeps it because the bundle carries
no entitlements and no nested code beyond `Contents/Frameworks` (already signed
inside-out by `-bundle-deps`). Re-signing those with the same identity costs
nothing. Entitlements and hardened runtime are not supported — run `codesign`
and `notarytool` separately for distribution.

### `-bundle-deps`

When set, `buildapp` walks the binary's `LC_LOAD_DYLIB` entries via `otool -L`.
It copies every non-system dylib (anything outside `/usr/lib`,
`/System/Library`, `/Library/Apple`) into `Contents/Frameworks/`. Then it uses
`install_name_tool` to:

- rewrite each bundled dylib's own id to `@rpath/<basename>`
- rewrite every reference in the executable and in bundled dylibs to
  `@rpath/<basename>`
- add `@executable_path/../Frameworks` as an rpath on the executable

It follows transitive dependencies. `install_name_tool` invalidates each
signature it touches, so buildapp re-signs every modified Mach-O file with the
`-sign` identity (required on Apple Silicon). Requires `otool`,
`install_name_tool`, and `codesign` (Xcode Command Line Tools).

Verify a clean bundle:

```
find Foo.app -type f -perm +111 -exec otool -L {} \; | grep -E '/opt/homebrew|/usr/local'
```

Empty output means no host paths leaked into the bundle.

buildapp converts `.png` icons to `.icns` via `sips` and `iconutil` (both ship
with macOS). Intermediate iconset files live in the system temp directory, and
the tool removes them on exit.

### macOS bundle layout

```
GetStarted.app/
  Contents/
    Info.plist
    MacOS/getstarted
    Resources/getstarted.icns   (only when -icon supplied)
```

Notes:

- The tool overwrites an existing `.app` at the destination without prompting.
- The bundle is always signed — ad-hoc by default, see [Signing](#signing). It
  does not perform notarization. Run `notarytool` separately for distribution.
- Shared libraries are bundled only with `-bundle-deps`. Without it, the target
  machine must have them installed.
- The macOS packager runs on macOS only: it needs `sips`, `iconutil` and
  `codesign`.

## Windows

Two things make a Go binary look like an application on Windows. Only the second
is buildapp's job.

**1. Build with the GUI subsystem.** Without this the loader gives the process a
console, so an empty terminal window appears behind the app window:

```
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 \
  go build -ldflags "-H windowsgui" -o build/showcase.exe ./examples/showcase/
```

**2. Embed an icon.** `buildapp -platform windows` appends a `.rsrc` section
carrying `RT_ICON` and `RT_GROUP_ICON` resources, then zips the result:

```
buildapp -platform windows -o dist -name "Go-Gui Showcase" -version 1.2.3 \
  -icon gui/default_icon.png build/showcase.exe
# dist/go-gui-showcase-1.2.3-windows-amd64.zip
```

`-icon` takes a `.png` or a `.ico`. A PNG is embedded as-is: since Vista an icon
directory entry may hold a PNG stream verbatim, so no conversion and no external
tool is needed. A `.ico` contributes all of its images.

Injection is refused when the binary already carries a resource directory, which
happens when a `.syso` was linked in. Use one or the other, not both.

Notarization has no Windows equivalent here: the `.exe` is not Authenticode
signed. Run `signtool` separately if you need that.

## Linux

`buildapp -platform linux` writes a tarball with the freedesktop.org layout, so
the app can appear in the application menu:

```
buildapp -platform linux -o dist -name "Go-Gui Showcase" -version 1.2.3 \
  -icon gui/default_icon.png build/showcase
# dist/go-gui-showcase-1.2.3-linux-amd64.tar.gz
```

```
go-gui-showcase-1.2.3-linux-amd64/
  bin/showcase
  share/applications/local.gogui.go-gui-showcase.desktop
  share/icons/hicolor/256x256/apps/local.gogui.go-gui-showcase.png
  install.sh
```

The user runs `./install.sh` to copy those into `~/.local` (no privileges
needed), or `./install.sh /usr/local` with `sudo` for a system-wide install.

`-icon` must be a `.png` here. The `.desktop` entry sets `Terminal=false`, which
is what stops a terminal emulator opening beside the app.

## Example

```
go build -o /tmp/getstarted ./examples/get_started
buildapp -o /tmp -name GetStarted -icon gui/default_icon.png /tmp/getstarted
open /tmp/GetStarted.app
```

`make release` runs all three packagers over `examples/showcase`.
