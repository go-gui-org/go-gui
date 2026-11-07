package gui

import (
	"errors"

	"github.com/go-gui-org/go-gui/gui/internal/imgdecode"
)

// RegisterEncodedImage decodes PNG, JPEG or GIF data, registers its pixels
// under key, and returns the Src string naming it. The result is accepted
// anywhere ImageCfg.Src or DrawContext.Image accepts a source. It returns an
// error for invalid data or an image that exceeds the in-memory registry's
// limits.
//
// A GIF decodes to its first frame, placed on the GIF's full canvas;
// animation is not played. Other formats decode when the app imports their
// decoder package, which registers itself with the image package:
// golang.org/x/image/webp or golang.org/x/image/bmp, for example.
//
// The key is a content key, under the same contract as [UseImage]: callers
// must use a new key when the image changes. A key that is already registered
// returns its Src without decoding data again. The encoded data is read during
// this call and is not retained.
//
// The decoded pixels stay registered until [DropImage]. Unlike UseImage
// entries, they neither count toward the in-memory image budget nor are
// evicted by it, so a Src kept from startup stays valid.
func RegisterEncodedImage(key string, data []byte) (string, error) {
	if key == "" {
		return "", errors.New("image key is empty")
	}
	if src, ok := pinImage(key); ok {
		return src, nil
	}
	img, err := imgdecode.DecodeNRGBABytes(key, data, imgdecode.Limits{
		MaxWidth:  maxMemImageDim,
		MaxHeight: maxMemImageDim,
		MaxPixels: int64(maxMemImageDim) * int64(maxMemImageDim),
	})
	if err != nil {
		return "", err
	}
	bounds := img.Bounds()
	src := useImage(key, bounds.Dx(), bounds.Dy(), img.Pix, true)
	if src == "" {
		return "", errors.New("decoded image cannot be registered")
	}
	return src, nil
}
