package overlay

import (
	"image"
	"image/color"
	"testing"
)

// split is black on the left half and white on the right.
func split(w, h int) *image.RGBA {
	img := filled(w, h, color.RGBA{0, 0, 0, 255})
	for y := range h {
		for x := w / 2; x < w; x++ {
			img.SetRGBA(x, y, color.RGBA{255, 255, 255, 255})
		}
	}
	return img
}

func TestStylesOnAPlainImage(t *testing.T) {
	c := color.RGBA{180, 120, 60, 255}
	for _, tc := range []struct {
		name  string
		style Style
		want  color.RGBA
	}{
		// Strokes, relief and tiles of one colour are that colour.
		{"oil", Oil, c},
		{"emboss", Emboss, c},
		// No edges: neon leaves black, pencil leaves paper.
		{"neon", Neon, color.RGBA{0, 0, 0, 255}},
		{"pencil", Pencil, color.RGBA{250, 250, 243, 255}},
		// Every cell is as bright as the others: all spaces.
		{"ascii", ASCII, color.RGBA{8, 8, 10, 255}},
	} {
		img := filled(120, 80, c)
		Effects{Style: tc.style, StyleSize: 0.1}.Apply(img)
		for _, p := range []image.Point{{0, 0}, {60, 40}, {119, 79}} {
			if got := img.RGBAAt(p.X, p.Y); got != tc.want {
				t.Errorf("%s at %v: %v, want %v", tc.name, p, got, tc.want)
			}
		}
	}
}

func TestOilKeepsAHardEdge(t *testing.T) {
	img := split(40, 20)
	want := append([]uint8(nil), img.Pix...)
	oil(img, append([]uint8(nil), img.Pix...), 5)
	if string(img.Pix) != string(want) {
		t.Error("the Kuwahara filter blurred a hard edge")
	}
}

func TestOilSmoothsNoise(t *testing.T) {
	// A checkerboard of 1 px squares: every square of the filter holds
	// both colours, so each pixel becomes their mean.
	img := filled(30, 30, color.RGBA{0, 0, 0, 255})
	for y := range 30 {
		for x := range 30 {
			if (x+y)%2 == 0 {
				img.SetRGBA(x, y, color.RGBA{200, 200, 200, 255})
			}
		}
	}
	oil(img, append([]uint8(nil), img.Pix...), 3)
	// A 4×4 square holds 8 of each.
	if got := img.RGBAAt(15, 15); got.R != 100 {
		t.Errorf("middle %v, want 100", got)
	}
}

func TestMosaic(t *testing.T) {
	img := split(40, 20) // tiles of 10: the left two black, the right two white
	img.SetRGBA(25, 5, color.RGBA{0, 0, 0, 255})
	mosaic(img, append([]uint8(nil), img.Pix...), 10, 1)
	// One black pixel of 100 in a white tile.
	if got := img.RGBAAt(22, 2); got.R != 252 {
		t.Errorf("tile %v, want 252", got)
	}
	if got := img.RGBAAt(35, 2); got.R != 255 {
		t.Errorf("white tile %v", got)
	}
	// The joint on the right and bottom of the tile: a fifth of it.
	if got := img.RGBAAt(39, 2); got.R != 51 {
		t.Errorf("joint %v, want 51", got)
	}
	if got := img.RGBAAt(35, 9); got.R != 51 {
		t.Errorf("bottom joint %v, want 51", got)
	}
}

func TestNeonLightsTheEdge(t *testing.T) {
	img := split(60, 20)
	edges(img, append([]uint8(nil), img.Pix...), 1, true)
	if got := img.RGBAAt(30, 10); got.R < 200 {
		t.Errorf("at the edge %v, want bright", got)
	}
	if got := img.RGBAAt(50, 10); got.R != 0 {
		t.Errorf("away from it %v, want black", got)
	}
}

func TestPencilDrawsTheEdge(t *testing.T) {
	img := split(60, 20)
	edges(img, append([]uint8(nil), img.Pix...), 1, false)
	if got := img.RGBAAt(30, 10); got.R > 60 {
		t.Errorf("at the edge %v, want dark", got)
	}
	if got := img.RGBAAt(5, 10); got.R != 250 {
		t.Errorf("away from it %v, want paper", got)
	}
}

func TestEmbossLightsFromTheTopLeft(t *testing.T) {
	// A raised square, lit from the top left: the slope up to it on the
	// left catches the light, the slope down on the right is in shadow.
	img := filled(40, 40, color.RGBA{100, 100, 100, 255})
	for y := 10; y < 30; y++ {
		for x := 10; x < 30; x++ {
			img.SetRGBA(x, y, color.RGBA{150, 150, 150, 255})
		}
	}
	emboss(img, 1, 1)
	before, after := img.RGBAAt(9, 20), img.RGBAAt(30, 20)
	if before.R <= 100 || after.R >= 100 {
		t.Errorf("left of the square %v (want lighter), right %v (want darker)", before, after)
	}
	if got := img.RGBAAt(20, 20); got.R != 150 {
		t.Errorf("flat inside %v", got)
	}
}

func TestASCIIWritesTheBrightHalf(t *testing.T) {
	img := split(200, 60)
	Effects{Style: ASCII, StyleSize: 0.2}.Apply(img)
	bg := color.RGBA{8, 8, 10, 255}
	var left, right int
	for y := range 60 {
		for x := range 200 {
			if img.RGBAAt(x, y) != bg {
				// Cells across the middle are half bright.
				if x < 80 {
					left++
				} else if x >= 120 {
					right++
				}
			}
		}
	}
	if left != 0 || right == 0 {
		t.Errorf("%d drawn pixels on the dark half, %d on the bright one", left, right)
	}
}

func TestStyleKeepsSomeOfTheOriginal(t *testing.T) {
	img := filled(20, 20, color.RGBA{200, 200, 200, 255})
	// Neon makes a plain image black; half of the original shows.
	Effects{Style: Neon, StyleSize: 0.05, Keep: 0.5}.Apply(img)
	if got := img.RGBAAt(10, 10); got.R != 100 {
		t.Errorf("%v, want 100", got)
	}
}

func TestStyleIsAnEffect(t *testing.T) {
	if (Effects{Style: Mosaic}).None() {
		t.Error("a style counts as no effect")
	}
}

// BenchmarkStyles measures each style at the medium size on a 4K screen
// (ADR 0058): go test -bench Styles ./internal/overlay/
func BenchmarkStyles(b *testing.B) {
	src := image.NewRGBA(image.Rect(0, 0, 3840, 2160))
	for i := range src.Pix {
		src.Pix[i] = uint8(i * 7919 >> 3)
	}
	sizes := map[Style]float64{Oil: 0.0065, Mosaic: 0.022, Neon: 0.002, Pencil: 0.002, ASCII: 0.015, Emboss: 0.002}
	names := map[Style]string{Oil: "oil", Mosaic: "mosaic", Neon: "neon", Pencil: "pencil", ASCII: "ascii", Emboss: "emboss"}
	for _, s := range []Style{Oil, Mosaic, Neon, Pencil, ASCII, Emboss} {
		b.Run(names[s], func(b *testing.B) {
			img := image.NewRGBA(src.Rect)
			for b.Loop() {
				copy(img.Pix, src.Pix)
				Effects{Style: s, StyleSize: sizes[s]}.Apply(img)
			}
		})
	}
}
