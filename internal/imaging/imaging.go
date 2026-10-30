// Package imaging validates downloaded images and derives what the
// library needs from them: real dimensions and format, a SHA-256 for exact
// duplicates, a perceptual hash for visually similar images, and a
// thumbnail.
package imaging

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	_ "image/png" // registers the PNG decoder
	"io"
	"math/bits"

	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp" // registers the WebP decoder
)

const (
	// ThumbnailSize is the longest side of library thumbnails, in pixels.
	ThumbnailSize = 480
	// MaxPixels rejects images that would need too much memory to decode
	// (decompression bombs). 8K is 33 MP; large panoramas fit as well.
	MaxPixels = 120_000_000
	// SimilarDistance is the largest dHash distance (differing bits out of
	// 64) at which two images count as visually the same.
	SimilarDistance = 10

	thumbnailQuality = 85
)

var (
	// ErrUnsupportedFormat means the data is not a JPEG, PNG or WebP image.
	ErrUnsupportedFormat = errors.New("unsupported image format")
	// ErrTooLarge means the image has more than MaxPixels pixels.
	ErrTooLarge = errors.New("image is too large")
	// ErrCorrupt means the data claims to be an image but cannot be decoded.
	ErrCorrupt = errors.New("image is damaged")
)

// formats maps the names used by the image package to MIME types. Formats
// the WebView cannot show are not accepted, even if a decoder is registered.
var formats = map[string]string{
	"jpeg": "image/jpeg",
	"png":  "image/png",
	"webp": "image/webp",
}

// Result describes a processed image.
type Result struct {
	Width, Height int
	MIMEType      string
	SHA256        string // hex
	DHash         uint64
	Thumbnail     []byte // JPEG
}

// Process reads an image file completely. It is read three times — for the
// SHA-256, the header and the full decode — so it must be seekable.
func Process(r io.ReadSeeker) (Result, error) {
	var res Result

	h := sha256.New()
	if _, err := io.Copy(h, r); err != nil {
		return Result{}, fmt.Errorf("read image: %w", err)
	}
	res.SHA256 = hex.EncodeToString(h.Sum(nil))

	if _, err := r.Seek(0, io.SeekStart); err != nil {
		return Result{}, fmt.Errorf("read image: %w", err)
	}
	cfg, format, err := image.DecodeConfig(r)
	if errors.Is(err, image.ErrFormat) {
		return Result{}, ErrUnsupportedFormat
	}
	if err != nil {
		return Result{}, fmt.Errorf("%w: %v", ErrCorrupt, err)
	}
	mime, ok := formats[format]
	if !ok {
		return Result{}, fmt.Errorf("%w: %s", ErrUnsupportedFormat, format)
	}
	if cfg.Width <= 0 || cfg.Height <= 0 {
		return Result{}, fmt.Errorf("%w: %dx%d", ErrCorrupt, cfg.Width, cfg.Height)
	}
	if int64(cfg.Width)*int64(cfg.Height) > MaxPixels {
		return Result{}, fmt.Errorf("%w: %dx%d", ErrTooLarge, cfg.Width, cfg.Height)
	}
	res.Width, res.Height, res.MIMEType = cfg.Width, cfg.Height, mime

	if _, err := r.Seek(0, io.SeekStart); err != nil {
		return Result{}, fmt.Errorf("read image: %w", err)
	}
	img, _, err := image.Decode(r)
	if err != nil {
		return Result{}, fmt.Errorf("%w: %v", ErrCorrupt, err)
	}

	thumb := thumbnail(img, ThumbnailSize)
	res.DHash = DHash(thumb)
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, thumb, &jpeg.Options{Quality: thumbnailQuality}); err != nil {
		return Result{}, fmt.Errorf("encode thumbnail: %w", err)
	}
	res.Thumbnail = buf.Bytes()
	return res, nil
}

// thumbnail scales img so its longest side is at most size, keeping the
// aspect ratio and never enlarging. Transparent areas become black, the
// background of the app, because JPEG has no alpha channel.
func thumbnail(img image.Image, size int) *image.RGBA {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if longest := max(w, h); longest > size {
		w = max(1, w*size/longest)
		h = max(1, h*size/longest)
	}

	// Catmull-Rom over a 20 MP original takes seconds, so the image is
	// first halved while it stays at least twice the target size. Halving
	// averages 2×2 pixels, which is a good downscaling filter; the final
	// step still uses Catmull-Rom.
	src := img
	for sb := src.Bounds(); sb.Dx()/2 >= 2*w && sb.Dy()/2 >= 2*h; sb = src.Bounds() {
		// RGBA destinations use the fast paths of x/image/draw, including
		// directly from the YCbCr images the JPEG decoder returns.
		half := image.NewRGBA(image.Rect(0, 0, sb.Dx()/2, sb.Dy()/2))
		draw.ApproxBiLinear.Scale(half, half.Bounds(), src, sb, draw.Src, nil)
		src = half
	}

	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(dst, dst.Bounds(), image.NewUniform(color.Black), image.Point{}, draw.Src)
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, src.Bounds(), draw.Over, nil)
	return dst
}

// DHash computes a 64-bit difference hash: the image is reduced to 9×8
// grey pixels and each bit says whether a pixel is brighter than its right
// neighbour. Re-encoding or resizing barely changes it; different images
// differ in many bits.
func DHash(img image.Image) uint64 {
	small := image.NewGray(image.Rect(0, 0, 9, 8))
	draw.CatmullRom.Scale(small, small.Bounds(), img, img.Bounds(), draw.Src, nil)

	var hash uint64
	for y := range 8 {
		for x := range 8 {
			hash <<= 1
			if small.GrayAt(x, y).Y > small.GrayAt(x+1, y).Y {
				hash |= 1
			}
		}
	}
	return hash
}

// Distance returns the number of bits in which two hashes differ.
func Distance(a, b uint64) int {
	return bits.OnesCount64(a ^ b)
}

// Similar reports whether two dHashes describe visually the same image.
func Similar(a, b uint64) bool {
	return Distance(a, b) <= SimilarDistance
}
