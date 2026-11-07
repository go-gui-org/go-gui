// Package imgdecode provides bounded raster decoding shared by gui and its
// rendering backends.
//
// It registers the PNG, JPEG and GIF decoders, so those three formats
// decode in every go-gui binary; a GIF decodes to its first frame. Decoding
// goes through image.Decode, so any other format whose decoder the app
// imports (golang.org/x/image/webp, for example) decodes as well.
package imgdecode

import (
	"bytes"
	"fmt"
	"image"
	"image/draw"
	_ "image/gif"  // Register GIF decoder.
	_ "image/jpeg" // Register JPEG decoder.
	_ "image/png"  // Register PNG decoder.
	"io"
	"os"
)

const (
	// DefaultMaxImageBytes is the default maximum encoded image size accepted
	// by the decoders in this package.
	DefaultMaxImageBytes = int64(16 * 1024 * 1024)
	// DefaultMaxImagePixels is the default maximum decoded pixel count accepted
	// by the decoders in this package.
	DefaultMaxImagePixels = int64(40_000_000)
)

// Limits bounds an encoded image before and after decoding. A non-positive
// byte or pixel limit takes the package default. Non-positive edge limits are
// unbounded.
type Limits struct {
	MaxBytes            int64
	MaxPixels           int64
	MaxWidth, MaxHeight int
}

// DecodeNRGBA validates image limits, decodes the file, and returns the
// pixels as *image.NRGBA. The file is seeked to 0 before decoding.
func DecodeNRGBA(
	path string, f *os.File,
	maxBytes, maxPixels int64,
) (*image.NRGBA, error) {
	cfg, err := validateFile(path, f, maxBytes, maxPixels)
	if err != nil {
		return nil, fmt.Errorf("decode image: validate: %w", err)
	}
	return decodeNRGBA(path, f, Limits{MaxPixels: maxPixels}, cfg)
}

// DecodeNRGBABytes validates and decodes an encoded in-memory image. It
// returns a zero-origin, tightly packed NRGBA image.
func DecodeNRGBABytes(
	name string, data []byte, limits Limits,
) (*image.NRGBA, error) {
	limits = limits.withDefaults()
	if int64(len(data)) > limits.MaxBytes {
		return nil, fmt.Errorf("decode image: validate: image data too large: %s", name)
	}
	reader := bytes.NewReader(data)
	cfg, _, configErr := image.DecodeConfig(reader)
	if configErr != nil {
		return nil, fmt.Errorf(
			"decode image: validate: validate image %s: decode config: %w",
			name, configErr)
	}
	if dimensionsErr := validateDimensions(
		name, cfg.Width, cfg.Height, limits,
	); dimensionsErr != nil {
		return nil, fmt.Errorf("decode image: validate: %w", dimensionsErr)
	}
	nrgba, err := decodeNRGBA(name, reader, limits, cfg)
	if err != nil {
		return nil, err
	}
	return tightlyPacked(nrgba), nil
}

// decodeNRGBA decodes input, whose header reported cfg.
func decodeNRGBA(
	name string, input io.ReadSeeker, limits Limits, cfg image.Config,
) (*image.NRGBA, error) {
	if _, err := input.Seek(0, 0); err != nil {
		return nil, fmt.Errorf("decode image: seek: %w", err)
	}
	src, _, err := image.Decode(input)
	if err != nil {
		return nil, fmt.Errorf("decode image: %w", err)
	}
	src = onCanvas(src, cfg)
	bounds := src.Bounds()
	if dimensionsErr := validateDimensions(
		name, bounds.Dx(), bounds.Dy(), limits,
	); dimensionsErr != nil {
		return nil, dimensionsErr
	}
	if existing, ok := src.(*image.NRGBA); ok {
		return existing, nil
	}
	nrgba := image.NewNRGBA(bounds)
	draw.Draw(nrgba, bounds, src, bounds.Min, draw.Src)
	return nrgba, nil
}

// onCanvas puts a decoded frame that covers only part of its image's
// canvas onto a transparent canvas of the size the header reported. A GIF
// decodes to its first frame, whose bounds are canvas coordinates and may
// cover less than the canvas; using the frame alone would crop the image
// and move it to the origin. The GIF decoder rejects a frame that extends
// past the canvas, so a frame always fits. Any other result is returned
// unchanged.
func onCanvas(src image.Image, cfg image.Config) image.Image {
	canvas := image.Rect(0, 0, cfg.Width, cfg.Height)
	bounds := src.Bounds()
	if bounds == canvas || !bounds.In(canvas) {
		return src
	}
	dst := image.NewNRGBA(canvas)
	draw.Draw(dst, bounds, src, bounds.Min, draw.Src)
	return dst
}

func validateFile(
	path string, f *os.File,
	maxBytes, maxPixels int64,
) (image.Config, error) {
	if maxBytes <= 0 {
		maxBytes = DefaultMaxImageBytes
	}
	info, err := f.Stat()
	if err != nil {
		return image.Config{}, fmt.Errorf("validate image %s: stat: %w", path, err)
	}
	if info.Size() > maxBytes {
		return image.Config{}, fmt.Errorf("image file too large: %s", path)
	}
	if maxPixels <= 0 {
		maxPixels = DefaultMaxImagePixels
	}
	cfg, _, err := image.DecodeConfig(f)
	if err != nil {
		return image.Config{}, fmt.Errorf("validate image %s: decode config: %w", path, err)
	}
	return cfg, validateDimensions(path, cfg.Width, cfg.Height,
		Limits{MaxPixels: maxPixels})
}

func (limits Limits) withDefaults() Limits {
	if limits.MaxBytes <= 0 {
		limits.MaxBytes = DefaultMaxImageBytes
	}
	if limits.MaxPixels <= 0 {
		limits.MaxPixels = DefaultMaxImagePixels
	}
	return limits
}

func validateDimensions(
	name string, width, height int, limits Limits,
) error {
	limits = limits.withDefaults()
	if width <= 0 || height <= 0 {
		return fmt.Errorf("invalid image dimensions: %s", name)
	}
	if int64(width)*int64(height) > limits.MaxPixels ||
		(limits.MaxWidth > 0 && width > limits.MaxWidth) ||
		(limits.MaxHeight > 0 && height > limits.MaxHeight) {
		return fmt.Errorf("image dimensions too large: %s", name)
	}
	return nil
}

func tightlyPacked(src *image.NRGBA) *image.NRGBA {
	bounds := src.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	if bounds.Min == (image.Point{}) && src.Stride == width*4 &&
		len(src.Pix) == width*height*4 {
		return src
	}
	dst := image.NewNRGBA(image.Rect(0, 0, width, height))
	draw.Draw(dst, dst.Bounds(), src, bounds.Min, draw.Src)
	return dst
}
