# buildapp

Packages a go-gui app as a release artefact. Give it a compiled binary and it
packs one artefact out; give it `-build-pkg` and it compiles first.

| Platform | Output                               | What it adds                                                        |
| -------- | ------------------------------------ | ------------------------------------------------------------------- |
| macOS    | signed `.app` (+ `.dmg` with `-dmg`) | `Info.plist`, `.icns` icon, code signature, optional bundled dylibs |
| Windows  | `.zip` with the `.exe`               | icon resource in the PE image                                       |
| Linux    | `.tar.gz`                            | `.desktop` entry, icon, `install.sh`                                |

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
buildapp [-platform darwin|windows|linux] [-arch amd64|arm64]
         [-o outdir] [-manifest appinfo.toml]
         [-name Name] [-id bundle.id] [-version 1.0] [-build n]
         [-icon icon.png|.icns|.ico] [-sign identity]
         [-build-pkg ./myapp] [-ldflags flags]
         [-dmg] [-entitlements app.entitlements]
         [-notarize -notary-profile profile] <binary>
```

Positional arg: path to the compiled executable. It must match `-platform`:
Mach-O for `darwin`, PE for `windows`, ELF for `linux`. With `-build-pkg` there
is no positional arg: buildapp compiles the package itself.

**The executable's basename becomes the installed program name** (the `Exec=`
line on Linux, the file inside the `.zip` on Windows, `Contents/MacOS/<name>` on
macOS). Stage it under a clean name before packaging; `showcase-linux` in,
`Exec=showcase-linux` out.

| Flag              | Default                           | Purpose                                                                                        |
| ----------------- | --------------------------------- | ---------------------------------------------------------------------------------------------- |
| `-platform`       | host `GOOS`                       | Target packager: `darwin`, `windows` or `linux`                                                |
| `-o`              | `.`                               | Output directory                                                                               |
| `-manifest`       | `./appinfo.toml` when it is there | App manifest; see [App manifest](#app-manifest)                                                |
| `-name`           | binary basename, capped           | Bundle display name                                                                            |
| `-id`             | `local.gogui.<name>`              | `CFBundleIdentifier`                                                                           |
| `-icon`           | none                              | `.png` everywhere; also `.icns` (macOS), `.ico` (Windows)                                      |
| `-version`        | `1.0`                             | `CFBundleShortVersionString`; also the file name version                                       |
| `-build`          | the version                       | macOS: `CFBundleVersion`                                                                       |
| `-bundle-deps`    | `false`                           | macOS: bundle non-system dylibs into `Contents/Frameworks`                                     |
| `-sign`           | `$BUILDAPP_SIGN_IDENTITY`, `-`    | macOS: `codesign` identity. `-` is ad-hoc                                                      |
| `-build-pkg`      | none                              | Compile this package for `-platform`/`-arch` first, then package the result                    |
| `-arch`           | host arch                         | Target architecture for `-build-pkg`                                                           |
| `-ldflags`        | none                              | Extra linker flags for `-build-pkg`                                                            |
| `-dmg`            | `false`                           | macOS: wrap the `.app` in a UDZO `.dmg`                                                        |
| `-entitlements`   | none                              | macOS: entitlements file for distribution signing (hardened runtime)                           |
| `-notarize`       | `false`                           | macOS: submit to Apple with `notarytool` and staple (needs `-entitlements`, `-notary-profile`) |
| `-notary-profile` | none                              | macOS: `notarytool` keychain profile to submit with                                            |

## Compiling with `-build-pkg`

Without it, buildapp packages the binary you hand it, and you run `go build`
yourself with per-platform flags. With it, one call does both:

```
buildapp -platform windows -arch amd64 -build-pkg ./myapp -o dist
```

Linux and Windows compile `CGO_ENABLED=0`; macOS keeps the host C toolchain for
the Metal backend. On Windows `-H windowsgui` is added to the linker flags when
missing, so no console window opens behind the app; your `-ldflags` are kept.
`-build-pkg` takes no positional binary — it compiles the package itself.
Cross-compiling to macOS from another OS still needs the macOS SDK, so build the
macOS binary on a Mac.

## App manifest

An app keeps its identity in `appinfo.toml` beside its `main.go`. buildapp reads
the file, and the app reads the same bytes at run time through `//go:embed` and
`gui.WindowCfg.AppInfo`. Thus the bundle ID and the ID the app uses for
preferences and file-access bookmarks are the same. If the two IDs differ, macOS
keeps the preferences and permission grants under two identities and reports no
error.

```toml
# appinfo.toml
id      = "org.go-gui.falcon"
name    = "Falcon"
version = "1.4.0"
build   = "42"                 # optional; default is the version
icon    = "assets/icon.png"    # relative to this file

[darwin]
category = "public.app-category.developer-tools"  # LSApplicationCategoryType

[linux]
categories = "Development;"    # .desktop Categories; default "Utility;"
```

buildapp finds the file like this:

1. If you give `-manifest path`, buildapp reads that file. If the file is
   missing, buildapp stops with an error.
2. If you do not give `-manifest`, buildapp reads `appinfo.toml` from the
   working directory, if that file exists.
3. If there is no file, buildapp uses only the flags, as before.

A flag that you give on the command line wins over the file. A flag that you do
not give does not win, so the `-version` default `1.0` does not hide the file's
version.

The format is a strict subset of TOML. Every file that buildapp accepts is also
valid TOML:

- Each line is `key = "value"`. Values are always double-quoted strings. The
  only escapes are `\"` and `\\`.
- A `[section]` header starts a platform block. Sections do not nest.
- `#` starts a comment.
- There are no arrays, numbers or multi-line values.
- An unknown key or section is an error. The error gives the file and line.

### Version from a git tag

A release script can keep `-version "$(git describe --tags)"`. The flag wins in
the package. But the running app reads the embedded file, so it shows the file's
version, not the flag. If the app shows its version, bump the file too, or set
the version at build time with `-ldflags -X`.

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
`--deep` for distribution signing. buildapp keeps it for local builds because
the bundle carries no entitlements and no nested code beyond
`Contents/Frameworks` (already signed inside-out by `-bundle-deps`). Re-signing
those with the same identity costs nothing. For distribution, pass
`-entitlements` instead: the signature switches to the hardened-runtime form
(`--options runtime --entitlements <file> --timestamp`, no `--deep`), which is
what notarization requires.

### `-dmg`

With `-dmg`, buildapp wraps the `.app` in a UDZO disk image named
`<slug>-<version>.dmg`, the same slug-version form as the Windows and Linux
archives. The Finder volume shows as `<Name> <Version>`. The `.app` inside
carries the signature; the image itself is left unsigned, which Gatekeeper
accepts.

```
buildapp -o dist -dmg -name "My App" -version 1.0.0 build/myapp
# dist/my-app-1.0.0.dmg (plus dist/My App.app)
```

### Distribution signing and notarization

For an app downloaded from the web, sign with a Developer ID identity and pass
an entitlements file, then notarize:

```
buildapp -sign "Developer ID Application: Example" \
  -entitlements app.entitlements \
  -notarize -notary-profile ACME \
  -dmg -name "My App" build/myapp
```

`-notarize` submits the artefact to Apple (`notarytool submit --wait`) and
staples the ticket (`stapler staple`): the `.dmg` when `-dmg` is set, else the
`.app`. Without `-entitlements` it is an error, because notarization requires
the hardened runtime. The profile is a `notarytool` keychain profile holding
your Apple ID, team and app-specific password —
`xcrun notarytool store-credentials` creates it. Secrets never travel as flags.
Needs a Developer ID; release CI that has none stays ad-hoc and skips
`-notarize`.

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
- The bundle is always signed — ad-hoc by default, see [Signing](#signing). Pass
  `-entitlements` and `-notarize` for distribution.
- Shared libraries are bundled only with `-bundle-deps`. Without it, the target
  machine must have them installed.
- The macOS packager runs on macOS only: it needs `sips`, `iconutil` and
  `codesign` (`hdiutil` for `-dmg`, `xcrun` for `-notarize`).

## Windows

Two things make a Go binary look like an application on Windows. Only the second
is buildapp's job.

**1. Build with the GUI subsystem.** Without this the loader gives the process a
console, so an empty terminal window appears behind the app window. `-build-pkg`
adds `-H windowsgui` automatically; building by hand:

```
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 \
  go build -ldflags "-H windowsgui" -o build/showcase.exe ./examples/showcase/
```

or in one step:

```
buildapp -platform windows -arch amd64 -build-pkg ./examples/showcase/ \
  -o dist -name "Go-Gui Showcase" -version 1.2.3 -icon gui/default_icon.png
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

`examples/app_manifest` has an `appinfo.toml`, so it needs no identity flags:

```
go build -o /tmp/app_manifest ./examples/app_manifest
cd examples/app_manifest
buildapp -o /tmp /tmp/app_manifest
open "/tmp/App Manifest Demo.app"
```

Without a manifest, give the identity as flags:

```
go build -o /tmp/getstarted ./examples/get_started
buildapp -o /tmp -name GetStarted -icon gui/default_icon.png /tmp/getstarted
open /tmp/GetStarted.app
```

`make release` runs all three packagers over `examples/showcase`.
