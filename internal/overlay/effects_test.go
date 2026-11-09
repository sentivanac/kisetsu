package overlay

import (
	"image"
	"image/color"
	"testing"
)

func filled(w, h int, c color.RGBA) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for i := 0; i < len(img.Pix); i += 4 {
		img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = c.R, c.G, c.B, c.A
	}
	return img
}

func TestNoEffectsChangeNothing(t *testing.T) {
	img := filled(8, 8, color.RGBA{200, 100, 50, 255})
	want := append([]uint8(nil), img.Pix...)
	var e Effects
	if !e.None() {
		t.Error("zero Effects is not None")
	}
	e.Apply(img)
	if string(img.Pix) != string(want) {
		t.Error("zero Effects changed the image")
	}
}

func TestDarken(t *testing.T) {
	img := filled(4, 4, color.RGBA{200, 100, 40, 255})
	Effects{Darken: 0.5}.Apply(img)
	if got := img.RGBAAt(1, 1); got != (color.RGBA{100, 50, 20, 255}) {
		t.Errorf("darkened by half: %v", got)
	}
	Effects{Darken: 1}.Apply(img)
	if got := img.RGBAAt(1, 1); got != (color.RGBA{0, 0, 0, 255}) {
		t.Errorf("darkened fully: %v", got)
	}
}

func TestGrayscale(t *testing.T) {
	img := filled(2, 2, color.RGBA{255, 0, 0, 255})
	Effects{Tone: Gray}.Apply(img)
	// Rec. 709: red counts 21 %.
	if got := img.RGBAAt(0, 0); got != (color.RGBA{54, 54, 54, 255}) {
		t.Errorf("red in black and white: %v", got)
	}
}

func TestSepia(t *testing.T) {
	img := filled(2, 2, color.RGBA{255, 255, 255, 255})
	Effects{Tone: Sepia}.Apply(img)
	if got := img.RGBAAt(0, 0); got != (color.RGBA{255, 224, 176, 255}) {
		t.Errorf("white in sepia: %v", got)
	}
	img = filled(2, 2, color.RGBA{0, 0, 0, 255})
	Effects{Tone: Sepia}.Apply(img)
	if got := img.RGBAAt(0, 0); got != (color.RGBA{0, 0, 0, 255}) {
		t.Errorf("black in sepia: %v", got)
	}
}

func TestVignetteDarkensTheCornersOnly(t *testing.T) {
	img := filled(101, 101, color.RGBA{200, 200, 200, 255})
	Effects{Vignette: 0.5}.Apply(img)
	if mid := img.RGBAAt(50, 50); mid.R < 199 {
		t.Errorf("middle darkened: %v", mid)
	}
	corner := img.RGBAAt(0, 0)
	if corner.R > 105 || corner.R < 95 {
		t.Errorf("corner %v, want about half of 200", corner)
	}
	if edge := img.RGBAAt(50, 0); edge.R <= corner.R || edge.R >= 200 {
		t.Errorf("edge %v should lie between the corner %v and the middle", edge, corner)
	}
}

func TestBlur(t *testing.T) {
	// A plain image stays plain, edges included.
	img := filled(20, 10, color.RGBA{120, 60, 30, 255})
	Effects{Blur: 0.2}.Apply(img)
	for _, p := range [][2]int{{0, 0}, {19, 9}, {10, 5}} {
		if got := img.RGBAAt(p[0], p[1]); got != (color.RGBA{120, 60, 30, 255}) {
			t.Errorf("plain image changed at %v: %v", p, got)
		}
	}

	// A hard edge becomes a gradient, symmetric around it.
	img = filled(40, 10, color.RGBA{0, 0, 0, 255})
	for y := range 10 {
		for x := 20; x < 40; x++ {
			img.SetRGBA(x, y, color.RGBA{255, 255, 255, 255})
		}
	}
	Effects{Blur: 0.3}.Apply(img)
	left, right := img.RGBAAt(19, 5).R, img.RGBAAt(20, 5).R
	if left == 0 || right == 255 || int(left)+int(right) < 250 || int(left)+int(right) > 260 {
		t.Errorf("edge after blur: %d | %d", left, right)
	}
	if img.RGBAAt(0, 5).R != 0 || img.RGBAAt(39, 5).R != 255 {
		t.Errorf("far from the edge: %d, %d", img.RGBAAt(0, 5).R, img.RGBAAt(39, 5).R)
	}
}

func BenchmarkEffects4K(b *testing.B) {
	img := filled(3840, 2160, color.RGBA{120, 160, 200, 255})
	e := Effects{Blur: 0.008, Tone: Sepia, Darken: 0.3, Vignette: 0.4}
	for b.Loop() {
		e.Apply(img)
	}
}

func BenchmarkEach4K(b *testing.B) {
	for name, e := range map[string]Effects{
		"blur": {Blur: 0.008}, "gray": {Tone: Gray}, "sepia": {Tone: Sepia}, "darken": {Darken: 0.3}, "vignette": {Vignette: 0.4},
	} {
		b.Run(name, func(b *testing.B) {
			img := filled(3840, 2160, color.RGBA{120, 160, 200, 255})
			for b.Loop() {
				e.Apply(img)
			}
		})
	}
}
