module github.com/go-gui-org/go-gui

go 1.26.0

require (
	github.com/alecthomas/chroma/v2 v2.23.1
	github.com/go-gl/gl v0.0.0-20260331235117-4566fea9a276
	github.com/go-gui-org/go-glyph v1.8.1
	github.com/go-pdf/fpdf v0.9.0
	github.com/godbus/dbus/v5 v5.2.2
	github.com/tdewolff/parse/v2 v2.8.12
	github.com/veandco/go-sdl2 v0.4.40
	github.com/yuin/goldmark v1.8.2
	github.com/yuin/goldmark-emoji v1.0.6
	golang.org/x/tools v0.42.0
)

require (
	github.com/dlclark/regexp2 v1.11.5 // indirect
	github.com/rivo/uniseg v0.4.7 // indirect
	golang.org/x/mod v0.33.0 // indirect
	golang.org/x/sync v0.20.0 // indirect
	golang.org/x/sys v0.43.0 // indirect
	golang.org/x/text v0.34.0 // indirect
)

replace github.com/go-gui-org/go-glyph => ../go-glyph
