package imaging

import (
	"image"
	"testing"
)

// photo is a JPEG-like YCbCr image of w×h with some detail.
func photo(w, h int) *image.YCbCr {
	img := image.NewYCbCr(image.Rect(0, 0, w, h), image.YCbCrSubsampleRatio420)
	for i := range img.Y {
		img.Y[i] = uint8(i * 7)
	}
	for i := range img.Cb {
		img.Cb[i], img.Cr[i] = uint8(i*3), uint8(i*5)
	}
	return img
}

func BenchmarkResize(b *testing.B) {
	for _, c := range []struct {
		name         string
		sw, sh, w, h int
	}{
		{"2560x1600-to-1400x875", 2560, 1600, 1400, 875},
		{"6000x4000-to-1920x1280", 6000, 4000, 1920, 1280},
		{"6000x4000-to-3840x2560", 6000, 4000, 3840, 2560},
	} {
		src := photo(c.sw, c.sh)
		b.Run(c.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				Resize(src, c.w, c.h)
			}
		})
	}
}
