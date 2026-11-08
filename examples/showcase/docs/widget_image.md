Display raster images: PNG, JPEG and GIF (see Formats below). Supports local
paths, remote HTTP/HTTPS URLs with automatic download caching, and encoded bytes
such as `go:embed` data. Defaults to 100x100 when no size is specified.

## Usage

```go
gui.Image(gui.ImageCfg{
    Src:    "photo.png",
    Width:  200,
    Height: 150,
})
```

## Choosing an image source

| Source           | How to get the source                               | Lifetime                                                                                   | What your app does                                        |
| ---------------- | --------------------------------------------------- | ------------------------------------------------------------------------------------------ | --------------------------------------------------------- |
| File             | `Src: "photo.png"`                                  | Loaded on first draw, read again from disk if the GPU cache drops it; native backends only | Nothing                                                   |
| HTTP(S) URL      | `Src: "https://…"`                                  | Downloaded in the background, kept in the OS temp directory                                | Nothing                                                   |
| Data URL         | `Src: "data:image/png;base64,…"`                    | Decoded by the browser; **web backend only**                                               | Nothing; on other backends use `RegisterEncodedImage`     |
| Encoded bytes    | `gui.RegisterEncodedImage(key, data)`               | Kept until `gui.DropImage(key)`; never evicted                                             | Register once, keep the source, drop it when not shown    |
| Generated pixels | `gui.UseImage(key, w, h, pix)`                      | In a 32 MiB in-memory cache; evicted when the cache is full                                | Call it while building every frame; never keep the source |
| Changing pixels  | `Surface:` a `*gui.PixelSurface` instead of a `Src` | Owned by your app                                                                          | `Update` it from any goroutine; `Close` it when done      |

Files and URLs ask nothing of your app. The two in-memory sources do, and their
rules are opposite. `RegisterEncodedImage` is a registration: you own it until
you drop it. `UseImage` is a cache: register again each frame, and if one frame
needs more than 32 MiB of `UseImage` images, raise the cache with
`gui.SetMemImageBudget`, or some of them draw blank.

A file name that comes from the user goes through `gui.ValidateImagePath(name)`
first: it checks that the path is safe to load and has a supported image
extension. `gui.ValidateImageExtension(name)` checks the extension alone.

## Formats

PNG, JPEG and GIF decode from `RegisterEncodedImage` on every backend, and from
a file on the native backends. The browser has no file system, so on the web
backend use `RegisterEncodedImage` or a `data:` URL instead of a path. A GIF
shows its first frame; animation is not played.

BMP, WebP and other formats decode only when your app imports their decoder
package, such as `golang.org/x/image/webp`, with a blank import. The package
registers itself with Go's `image` package.

Remote URLs are stricter: a download is kept only when the server sends it as
PNG, JPEG or SVG, whatever decoders are imported. A `data:` URL is decoded by
the browser, so on the web backend it may use any format the browser supports.

## Encoded Bytes and `go:embed`

Register embedded PNG or JPEG data once during startup, then use the returned
source like a file path:

```go
//go:embed default_icon.png
var logoPNG []byte

var logoSrc string

func loadImages() error {
	var err error
	logoSrc, err = gui.RegisterEncodedImage("myapp/logo-v1", logoPNG)
	return err
}

func logoView() gui.View {
	return gui.Image(gui.ImageCfg{
		Src:    logoSrc,
		Width:  128,
		Height: 128,
	})
}
```

The key identifies immutable content, just as it does for `gui.UseImage`. Use a
new key when the bytes change. The encoded input is decoded immediately and is
not retained. The decoded image stays registered until
`gui.DropImage("myapp/logo-v1")` releases it: the in-memory image budget neither
counts nor evicts it, so the saved source stays valid however many other images
are registered.

## Remote Image

```go
gui.Image(gui.ImageCfg{
    Src:    "https://example.com/logo.png",
    Width:  120,
    Height: 40,
    BgColor: gui.White,
})
```

Remote images are fetched asynchronously, cached locally, and displayed on
completion. A placeholder rectangle is shown during the download.

## Key Properties

| Property  | Type    | Description                       |
| --------- | ------- | --------------------------------- |
| Src       | string  | File path, URL, or registered Src |
| Width     | float32 | Display width (default 100)       |
| Height    | float32 | Display height (default 100)      |
| MinWidth  | float32 | Minimum width                     |
| MaxWidth  | float32 | Maximum width                     |
| MinHeight | float32 | Minimum height                    |
| MaxHeight | float32 | Maximum height                    |
| BgColor   | Color   | Opaque fill behind image          |
| Invisible | bool    | Hide without removing from layout |

## Events

| Callback | Signature      | Fired when         |
| -------- | -------------- | ------------------ |
| OnClick  | func(EventCtx) | Image clicked      |
| OnHover  | func(EventCtx) | Mouse enters image |

## Accessibility

| Property | Type    | Description                          |
| -------- | ------- | ------------------------------------ |
| A11YCfg  | A11YCfg | Embedded: A11YLabel, A11YDescription |

Set the pair through the embed: `A11YCfg: gui.A11YCfg{A11YLabel: "Save"}`.
