Display raster image files (PNG, JPEG, GIF, BMP, WebP).
Supports local paths and remote HTTP/HTTPS URLs with automatic
download caching. Defaults to 100x100 when no size is specified.

## Usage

```go
gui.Image(gui.ImageCfg{
    Src:    "photo.png",
    Width:  200,
    Height: 150,
})
```

## Remote Image

```go
gui.Image(gui.ImageCfg{
    Src:    "https://example.com/logo.png",
    Width:  120,
    Height: 40,
    BgColor: gui.White,
})
```

Remote images are fetched asynchronously, cached locally, and
displayed on completion. A placeholder rectangle is shown while
downloading.

## Key Properties

| Property  | Type    | Description                          |
|-----------|---------|--------------------------------------|
| Src       | string  | File path or HTTP/HTTPS URL          |
| Width     | float32 | Display width (default 100)          |
| Height    | float32 | Display height (default 100)         |
| MinWidth  | float32 | Minimum width                        |
| MaxWidth  | float32 | Maximum width                        |
| MinHeight | float32 | Minimum height                       |
| MaxHeight | float32 | Maximum height                       |
| BgColor   | Color   | Opaque fill behind image             |
| Invisible | bool    | Hide without removing from layout    |

## Events

| Callback | Signature                          | Fired when         |
|----------|------------------------------------|--------------------|
| OnClick  | func(*Layout, *Event, *Window)     | Image clicked      |
| OnHover  | func(*Layout, *Event, *Window)     | Mouse enters image |

## Accessibility

| Property        | Type   | Description                          |
|-----------------|--------|--------------------------------------|
| A11YLabel       | string | Accessible label                     |
| A11YDescription | string | Accessible description               |
