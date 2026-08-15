# buildapp

Wraps a compiled go-gui binary into a macOS `.app` bundle so it can be
double-clicked, dragged to `/Applications`, and shown in the Dock with a proper
name and icon.

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
buildapp [-o outdir] [-name Name] [-id bundle.id] [-icon icon.png|.icns] [-version 1.0] [-sign identity] <binary>
```

Positional arg: path to a compiled Mach-O executable.

| Flag           | Default                        | Purpose                                             |
| -------------- | ------------------------------ | --------------------------------------------------- |
| `-o`           | `.`                            | Output directory                                    |
| `-name`        | binary basename, capped        | Bundle display name                                 |
| `-id`          | `local.gogui.<name>`           | `CFBundleIdentifier`                                |
| `-icon`        | none                           | `.png` (auto-converted) or `.icns`                  |
| `-version`     | `1.0`                          | `CFBundleVersion` / short version                   |
| `-bundle-deps` | `false`                        | Bundle non-system dylibs into `Contents/Frameworks` |
| `-sign`        | `$BUILDAPP_SIGN_IDENTITY`, `-` | `codesign` identity; `-` is ad-hoc                  |

### Signing

`buildapp` always signs the bundle. An unsigned bundle makes Gatekeeper report
the app as "damaged", even when every binary inside is individually signed, and
an unsigned Mach-O does not load at all on Apple Silicon.

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
TCC-gated permission the app holds** — screen recording, microphone, camera,
accessibility, input monitoring, full disk access.

The failure is actively misleading: the System Settings row survives, because
that list is keyed by bundle id for display, while the authorization check is
keyed by cdhash. The permission looks granted and the API returns denied, with
nothing in any log connecting the two. Recovery is
`tccutil reset <service> <bundle-id>`, a relaunch, and a re-grant — after every
build.

Pass a stable identity to key the grant on the certificate instead of the hash:

```
buildapp -sign "My Dev Cert" ...
```

or set it once per machine (fish):

```fish
set -Ux BUILDAPP_SIGN_IDENTITY "My Dev Cert"
```

`-sign` wins over the environment variable. A **self-signed** code-signing
certificate is enough; no Apple Developer account is needed. Create one in
Keychain Access → Certificate Assistant → Create a Certificate, with type "Code
Signing", then confirm it is visible to `codesign`:

```
security find-identity -v -p codesigning
```

Verify what a bundle actually carries:

```
codesign -dv --verbose=4 Foo.app
```

Ad-hoc prints `Signature=adhoc`; a real identity prints an `Authority=` line. To
confirm the TCC fix end to end, grant the app a permission, rebuild, and check
the permission still works without a re-grant.

This was verified on macOS 26 with a self-signed certificate: Falcon
(`github.com.go-gui-org.go-term`) kept its Screen Recording grant across a
rebuild that moved the cdhash from `573a437d…` to `bfa13ddf…`, with
`TeamIdentifier=not set` throughout. A certificate is enough; an Apple-issued
one is not required.

Release builds in CI have no certificate and stay ad-hoc. That is fine — freshly
downloaded apps have no grants to lose.

The bundle-level signature uses `codesign --force --deep`. Apple deprecates
`--deep` for distribution signing; it is kept here because the bundle carries no
entitlements and no nested code beyond `Contents/Frameworks` (already signed
inside-out by `-bundle-deps`), so re-signing those with the same identity costs
nothing. Entitlements and hardened runtime are not supported — run `codesign`
and `notarytool` separately for distribution.

### `-bundle-deps`

When set, `buildapp` walks the binary's `LC_LOAD_DYLIB` entries via `otool -L`,
copies every non-system dylib (anything outside `/usr/lib`, `/System/Library`,
`/Library/Apple`) into `Contents/Frameworks/`, then uses `install_name_tool` to:

- rewrite each bundled dylib's own id to `@rpath/<basename>`
- rewrite every reference in the executable and in bundled dylibs to
  `@rpath/<basename>`
- add `@executable_path/../Frameworks` as an rpath on the executable

Transitive dependencies are followed. `install_name_tool` invalidates each
signature it touches, so every modified Mach-O file is re-signed with the
`-sign` identity (required on Apple Silicon). Requires `otool`,
`install_name_tool`, and `codesign` (Xcode Command Line Tools).

Verify a clean bundle:

```
find Foo.app -type f -perm +111 -exec otool -L {} \; | grep -E '/opt/homebrew|/usr/local'
```

Empty output means no host paths leaked into the bundle.

`.png` icons are converted to `.icns` via `sips` and `iconutil` (both ship with
macOS). Intermediate iconset files live in the system temp directory and are
removed on exit.

## Example

```
go build -o /tmp/getstarted ./examples/get_started
buildapp -o /tmp -name GetStarted -icon assets/icon.png /tmp/getstarted
open /tmp/GetStarted.app
```

## Bundle layout

```
GetStarted.app/
  Contents/
    Info.plist
    MacOS/getstarted
    Resources/getstarted.icns   (only when -icon supplied)
```

## Notes

- macOS only.
- Existing `.app` at the destination is overwritten without prompting.
- The bundle is always signed — ad-hoc by default, see [Signing](#signing).
  Notarization is not performed; run `notarytool` separately for distribution.
- Shared libraries are bundled only with `-bundle-deps`. Without it the target
  machine must have them installed.
