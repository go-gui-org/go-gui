# Android Demo

Demonstrates go-gui on Android via OpenGL ES 3.0.

## Prerequisites

- Go 1.23+
- [gomobile](https://pkg.go.dev/golang.org/x/mobile/cmd/gomobile):
  `go install golang.org/x/mobile/cmd/gomobile@latest && gomobile init`
- Android SDK (compileSdk 35) + NDK
- Android device or emulator (arm64, API 24+)

## Build

```bash
# Generate AAR from Go code
make bind

# Build debug APK
make build

# Install on connected device
make install
```

## Architecture

```
demo.go (Go, gomobile bind)
  → android.SetWindow / Start / Render
    → gles_android.c (GLES3 via NDK)

Kotlin host:
  MainActivity → GoGuiGLSurfaceView (EGL3 + stencil)
    → GoGuiRenderer.onDrawFrame → Render()
    → touch events → TouchBegan/Moved/Ended
```

## Troubleshooting

- **gomobile not found**: Make sure that `$GOPATH/bin` is in `$PATH`
- **NDK not found**: Set `ANDROID_NDK_HOME`. Alternatively, install the NDK
  through the Android Studio SDK Manager
- **Linker errors**: Make sure that the NDK includes the GLES3 headers
  (`$NDK/toolchains/llvm/prebuilt/*/sysroot/usr/include/GLES3/`)
- **Black screen**: Make sure that logcat shows no shader compilation errors
  under the `go-gui-gles` tag
