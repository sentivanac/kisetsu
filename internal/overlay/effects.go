package overlay

import (
	"image"
	"math"
	"runtime"
	"sync"
)

// Tone changes the colors of the whole image.
type Tone int

const (
	// Original keeps the colors.
	Original Tone = iota
	// Gray turns the image black and white.
	Gray
	// Sepia turns it brown, like an old photograph.
	Sepia
)

// Effects change the whole image before text is drawn on it (ADR 0050).
// The zero value changes nothing.
type Effects struct {
	// Blur radius as a share of the screen height; 0 is none.
	Blur float64
	Tone Tone
	// Darken multiplies every color by 1−Darken; 0 is none, 1 black.
	Darken float64
	// Vignette darkens the edges: the corners by this share; 0 is none.
	Vignette float64
}

// None reports whether e changes nothing.
func (e Effects) None() bool {
	return e.Blur <= 0 && e.Tone == Original && e.Darken <= 0 && e.Vignette <= 0
}

// Apply changes img in place, in a fixed order: blur, tone, darken,
// vignette.
func (e Effects) Apply(img *image.RGBA) {
	if e.Blur > 0 {
		blur(img, max(1, int(math.Round(float64(img.Rect.Dy())*e.Blur))))
	}
	switch e.Tone {
	case Gray:
		grayscale(img)
	case Sepia:
		sepia(img)
	}
	if e.Darken > 0 {
		darken(img, min(e.Darken, 1))
	}
	if e.Vignette > 0 {
		vignette(img, min(e.Vignette, 1))
	}
}

// blur approximates a Gaussian blur with three box blurs, each along rows
// and then along columns: linear in the number of pixels whatever the
// radius. Both passes read memory in order (the column pass keeps a
// running sum per column), and alpha is left as it is.
func blur(img *image.RGBA, radius int) {
	w, h := img.Rect.Dx(), img.Rect.Dy()
	if w == 0 || h == 0 {
		return
	}
	tmp := make([]uint8, len(img.Pix))
	copy(tmp, img.Pix) // alpha
	for range 3 {
		// Rows and columns are independent, so each pass is split
		// between the processor's cores: by rows, then by columns.
		parallel(h, func(from, to int) { blurRows(img.Pix, tmp, w, from, to, img.Stride, radius) })
		parallel(w, func(from, to int) { blurColumns(tmp, img.Pix, w, h, from, to, img.Stride, radius) })
	}
}

// parallel calls f on n items split into one range per processor core.
func parallel(n int, f func(from, to int)) {
	parts := min(runtime.GOMAXPROCS(0), n)
	var wg sync.WaitGroup
	for p := range parts {
		from, to := n*p/parts, n*(p+1)/parts
		wg.Go(func() { f(from, to) })
	}
	wg.Wait()
}

// recip lets the box average multiply instead of divide: v/window is
// (v*recip + 1<<23) >> 24 for the sums of 8-bit values that occur here.
func recip(window int) int { return (1 << 24) / window }

// blurRows blurs rows from to to (exclusive) along the rows.
func blurRows(src, dst []uint8, w, from, to, stride, radius int) {
	r := recip(2*radius + 1)
	// line holds one channel of a row, with the edge pixels repeated
	// radius times on either side.
	line := make([]int, w+2*radius+1)
	for y := from; y < to; y++ {
		row := src[y*stride : y*stride+w*4]
		out := dst[y*stride : y*stride+w*4]
		for c := range 3 {
			for i := range line {
				x := min(max(i-radius, 0), w-1)
				line[i] = int(row[x*4+c])
			}
			sum := 0
			for i := range 2*radius + 1 {
				sum += line[i]
			}
			for x := range w {
				out[x*4+c] = uint8((sum*r + 1<<23) >> 24)
				sum += line[x+2*radius+1] - line[x]
			}
		}
	}
}

// blurColumns blurs columns from to to (exclusive) along the columns,
// keeping a running sum per column so memory is read row by row.
func blurColumns(src, dst []uint8, w, h, from, to, stride, radius int) {
	r := recip(2*radius + 1)
	lo, hi := from*4, to*4
	row := func(y int) []uint8 {
		y = min(max(y, 0), h-1)
		return src[y*stride+lo : y*stride+hi]
	}
	sums := make([]int, hi-lo)
	for y := -radius; y <= radius; y++ {
		for i, v := range row(y) {
			sums[i] += int(v)
		}
	}
	for y := range h {
		out := dst[y*stride+lo : y*stride+hi]
		for i := 0; i < len(sums); i += 4 {
			out[i] = uint8((sums[i]*r + 1<<23) >> 24)
			out[i+1] = uint8((sums[i+1]*r + 1<<23) >> 24)
			out[i+2] = uint8((sums[i+2]*r + 1<<23) >> 24)
		}
		add, sub := row(y+radius+1), row(y-radius)
		for i := range sums {
			sums[i] += int(add[i]) - int(sub[i])
		}
	}
}

// luma is the brightness of a pixel, with the Rec. 709 weights.
func luma(p []uint8) int {
	return (2126*int(p[0]) + 7152*int(p[1]) + 722*int(p[2]) + 5000) / 10000
}

func grayscale(img *image.RGBA) {
	forPixels(img, func(p []uint8) {
		y := uint8(luma(p))
		p[0], p[1], p[2] = y, y, y
	})
}

// sepia colors the brightness brown: black stays black, white becomes
// cream (255, 224, 176), and the brightness stays close to the original.
func sepia(img *image.RGBA) {
	forPixels(img, func(p []uint8) {
		y := luma(p)
		p[0] = uint8(min(255, y*274>>8)) // ×1.07
		p[1] = uint8(y * 225 >> 8)       // ×0.88
		p[2] = uint8(y * 177 >> 8)       // ×0.69
	})
}

func darken(img *image.RGBA, amount float64) {
	keep := int(math.Round((1 - amount) * 256))
	forPixels(img, func(p []uint8) {
		p[0] = uint8(int(p[0]) * keep >> 8)
		p[1] = uint8(int(p[1]) * keep >> 8)
		p[2] = uint8(int(p[2]) * keep >> 8)
	})
}

// vignette darkens towards the edges: nothing in the middle, strength at
// the corners, smoothly in between.
func vignette(img *image.RGBA, strength float64) {
	w, h := img.Rect.Dx(), img.Rect.Dy()
	cx, cy := float64(w)/2, float64(h)/2
	maxD2 := cx*cx + cy*cy
	for y := range h {
		dy := float64(y) + 0.5 - cy
		row := img.Pix[y*img.Stride : y*img.Stride+w*4]
		for x := range w {
			dx := float64(x) + 0.5 - cx
			d := (dx*dx + dy*dy) / maxD2 // 0 in the middle, 1 at the corners
			keep := int(math.Round((1 - strength*d*d) * 256))
			p := row[x*4 : x*4+3]
			p[0] = uint8(int(p[0]) * keep >> 8)
			p[1] = uint8(int(p[1]) * keep >> 8)
			p[2] = uint8(int(p[2]) * keep >> 8)
		}
	}
}

func forPixels(img *image.RGBA, f func(p []uint8)) {
	w, h := img.Rect.Dx(), img.Rect.Dy()
	for y := range h {
		row := img.Pix[y*img.Stride : y*img.Stride+w*4]
		for x := 0; x < len(row); x += 4 {
			f(row[x : x+4])
		}
	}
}
