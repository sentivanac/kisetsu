package imaging

import (
	"image"
	"image/color"
	"math"
	"testing"

	"golang.org/x/image/draw"
)

// detailed is an image with smooth areas, hard edges and fine stripes.
func detailed(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			c := color.RGBA{uint8(x * 255 / w), uint8(y * 255 / h), 128, 255}
			if (x/37+y/23)%2 == 0 {
				c.B = 30
			}
			if x%5 == 0 {
				c.R = 255
			}
			img.SetRGBA(x, y, c)
		}
	}
	return img
}

// meanDiff is the mean absolute difference of the color channels.
func meanDiff(a, b *image.RGBA) float64 {
	var sum float64
	for i := 0; i < len(a.Pix); i += 4 {
		for c := range 3 {
			sum += math.Abs(float64(a.Pix[i+c]) - float64(b.Pix[i+c]))
		}
	}
	return sum / float64(len(a.Pix)/4*3)
}

func TestResampleMatchesCatmullRom(t *testing.T) {
	for _, c := range []struct{ sw, sh, w, h int }{
		{1200, 800, 640, 427},   // shrink by 1.9
		{1000, 700, 1000, 700},  // same size
		{640, 480, 1024, 768},   // enlarge
		{2000, 1000, 1999, 999}, // barely
	} {
		src := detailed(c.sw, c.sh)
		want := image.NewRGBA(image.Rect(0, 0, c.w, c.h))
		draw.CatmullRom.Scale(want, want.Bounds(), src, src.Bounds(), draw.Src, nil)
		got := resample(src, c.w, c.h)
		d := meanDiff(got, want)
		t.Logf("%dx%d to %dx%d: mean difference %.3f", c.sw, c.sh, c.w, c.h, d)
		if d > 1.0 {
			t.Errorf("%dx%d to %dx%d: mean difference %.2f from x/image's Catmull-Rom", c.sw, c.sh, c.w, c.h, d)
		}
	}
}

func TestResampleKeepsAPlainImagePlain(t *testing.T) {
	src := image.NewRGBA(image.Rect(10, 20, 310, 220)) // not at the origin
	for i := 0; i < len(src.Pix); i += 4 {
		src.Pix[i], src.Pix[i+1], src.Pix[i+2], src.Pix[i+3] = 200, 100, 50, 255
	}
	got := resample(src, 77, 51)
	for _, p := range [][2]int{{0, 0}, {76, 50}, {38, 25}} {
		if c := got.RGBAAt(p[0], p[1]); c != (color.RGBA{200, 100, 50, 255}) {
			t.Errorf("at %v: %v", p, c)
		}
	}
}

func TestResampleTransparentBecomesBlack(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 40, 40)) // fully transparent
	if c := resample(src, 10, 10).RGBAAt(5, 5); c != (color.RGBA{0, 0, 0, 255}) {
		t.Errorf("transparent pixel became %v, want opaque black", c)
	}
}
