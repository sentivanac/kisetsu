package overlay

import (
	"image"
	"image/color"
	"image/draw"
	"math"
	"sort"

	"golang.org/x/image/math/fixed"
)

// Style redraws the whole image in another manner (ADR 0058). Unlike the
// effects of ADR 0050, which keep text legible, a style changes how the
// picture looks; only one is used at a time.
type Style int

const (
	// NoStyle keeps the picture as it is.
	NoStyle Style = iota
	// Oil paints it with brush strokes (a Kuwahara filter) on a canvas
	// with a light relief.
	Oil
	// Mosaic builds it from square tiles of one colour, with thin dark
	// joints between them.
	Mosaic
	// Neon keeps only its edges, in the picture's colours, on black.
	Neon
	// Pencil draws its edges as dark lines on paper.
	Pencil
	// ASCII writes it with characters, denser where it is brighter, in the
	// picture's colours on black.
	ASCII
	// Emboss shades its colours as a relief lit from the top left.
	Emboss
)

// asciiRamp goes from the emptiest to the densest character.
const asciiRamp = " .:-=+*#%@"

// Strengths, tuned by eye on photographs and paintings at 1920×1080.
const (
	neonGain     = 2.5 // edge strength that makes a full-brightness line
	neonBoost    = 1.6 // how much brighter a line is than the picture
	pencilGain   = 1.5
	embossDepth  = 1.5
	canvasDepth  = 0.6 // the relief under an oil painting
	asciiLineGap = 1.15
)

// applyStyle redraws img in style s. size is a share of the image height:
// the brush, tile, letter or line size. keep is the share of the original
// that shows through.
func applyStyle(img *image.RGBA, s Style, size, keep float64) {
	w, h := img.Rect.Dx(), img.Rect.Dy()
	if w == 0 || h == 0 || s == NoStyle {
		return
	}
	px := max(1, int(math.Round(float64(h)*size)))
	orig := append([]uint8(nil), img.Pix...)
	switch s {
	case Oil:
		oil(img, orig, px)
		emboss(img, max(1, h/1080), canvasDepth)
	case Mosaic:
		mosaic(img, orig, px, max(1, int(math.Round(float64(h)/1080))))
	case Neon, Pencil:
		edges(img, orig, px, s == Neon)
	case ASCII:
		if !ascii(img, orig, px) {
			copy(img.Pix, orig) // no font: leave the picture as it was
			return
		}
	case Emboss:
		emboss(img, px, embossDepth)
	}
	if keep > 0 {
		blend(img, orig, min(keep, 1))
	}
}

// blend mixes keep of orig back into img.
func blend(img *image.RGBA, orig []uint8, keep float64) {
	k := int(math.Round(keep * 256))
	w := img.Rect.Dx()
	parallel(img.Rect.Dy(), func(from, to int) {
		for y := from; y < to; y++ {
			row := img.Pix[y*img.Stride : y*img.Stride+w*4]
			o := orig[y*img.Stride : y*img.Stride+w*4]
			for i := 0; i < len(row); i += 4 {
				for c := range 3 {
					row[i+c] = uint8((int(o[i+c])*k + int(row[i+c])*(256-k) + 128) >> 8)
				}
			}
		}
	})
}

// lumaPlane returns the brightness of every pixel of pix, one byte each.
func lumaPlane(pix []uint8, w, h, stride int) []uint8 {
	l := make([]uint8, w*h)
	parallel(h, func(from, to int) {
		for y := from; y < to; y++ {
			row := pix[y*stride:]
			for x := range w {
				l[y*w+x] = uint8(luma(row[x*4 : x*4+3]))
			}
		}
	})
	return l
}

// neighbours reads the brightness d pixels around a pixel: the rows above,
// at and below it, and the columns left of, at and right of it, repeating
// the edge of the image beyond it.
type neighbours struct {
	l    []uint8
	w, h int
	d    int
}

func (n neighbours) rows(y int) (up, mid, down []uint8) {
	row := func(y int) []uint8 {
		y = min(max(y, 0), n.h-1)
		return n.l[y*n.w : (y+1)*n.w]
	}
	return row(y - n.d), row(y), row(y + n.d)
}

func (n neighbours) cols(x int) (left, mid, right int) {
	return max(x-n.d, 0), x, min(x+n.d, n.w-1)
}

// oil is a Kuwahara filter: each pixel takes the mean colour of the calmest
// (least varying in brightness) of the four (r+1)×(r+1) squares that have
// it in a corner. Edges stay sharp and surfaces become strokes of colour.
//
// Square statistics are kept for r+1 rows at a time, from running sums
// along columns and then along rows, so memory stays small and the cost
// does not depend on r. Each processor core does a band of rows.
func oil(img *image.RGBA, src []uint8, r int) {
	w, h, stride := img.Rect.Dx(), img.Rect.Dy(), img.Stride
	parallel(h, func(from, to int) {
		n := r + 1
		clampX := func(x int) int { return min(max(x, 0), w-1) }
		clampY := func(y int) int { return min(max(y, 0), h-1) }
		// Column sums over the rows of the current square: R, G, B, luma
		// and luma².
		var col [5][]int64
		for i := range col {
			col[i] = make([]int64, w)
		}
		addRow := func(y int, sign int64) {
			row := src[clampY(y)*stride:]
			for x := range w {
				p := row[x*4 : x*4+3]
				l := int64(luma(p))
				col[0][x] += sign * int64(p[0])
				col[1][x] += sign * int64(p[1])
				col[2][x] += sign * int64(p[2])
				col[3][x] += sign * l
				col[4][x] += sign * l * l
			}
		}
		// Statistics of the squares whose top left corner is (x−r, s),
		// for x in [0, w+r): mean colour and brightness variance. A ring
		// of n rows of them, indexed by s mod n.
		type stat struct {
			c    [3]uint8
			vari float32
		}
		ring := make([][]stat, n)
		for i := range ring {
			ring[i] = make([]stat, w+r)
		}
		area := float64(n * n)
		fillStats := func(s int) {
			out := ring[((s%n)+n)%n]
			var sum [5]int64
			for x := -r; x <= 0; x++ { // the square starting at −r
				for i := range sum {
					sum[i] += col[i][clampX(x)]
				}
			}
			for x := -r; x < w; x++ {
				m := float64(sum[3]) / area
				out[x+r] = stat{
					c:    [3]uint8{uint8(float64(sum[0])/area + 0.5), uint8(float64(sum[1])/area + 0.5), uint8(float64(sum[2])/area + 0.5)},
					vari: float32(float64(sum[4])/area - m*m),
				}
				// Slide right: drop column x, add column x+r+1.
				for i := range sum {
					sum[i] += col[i][clampX(x+r+1)] - col[i][clampX(x)]
				}
			}
		}
		// Squares starting at row s cover rows s..s+r.
		first := from - r
		for y := first; y <= first+r; y++ {
			addRow(y, 1)
		}
		for s := first; s < to; s++ {
			fillStats(s)
			if y := s; y >= from {
				top, bottom := ring[(((y-r)%n)+n)%n], ring[((y%n)+n)%n]
				row := img.Pix[y*stride:]
				for x := range w {
					// Squares starting at x−r and x; index +r.
					best := top[x]
					for _, q := range [3]stat{top[x+r], bottom[x], bottom[x+r]} {
						if q.vari < best.vari {
							best = q
						}
					}
					row[x*4], row[x*4+1], row[x*4+2] = best.c[0], best.c[1], best.c[2]
				}
			}
			addRow(s+r+1, 1)
			addRow(s, -1)
		}
	})
}

// mosaic fills tiles of size×size pixels with their mean colour and
// darkens a joint of gap pixels on their right and bottom edges.
func mosaic(img *image.RGBA, src []uint8, size, gap int) {
	w, h, stride := img.Rect.Dx(), img.Rect.Dy(), img.Stride
	if size <= gap+1 {
		gap = 0
	}
	tilesDown := (h + size - 1) / size
	parallel(tilesDown, func(from, to int) {
		for ty := from; ty < to; ty++ {
			y0, y1 := ty*size, min((ty+1)*size, h)
			for x0 := 0; x0 < w; x0 += size {
				x1 := min(x0+size, w)
				var sum [3]int
				for y := y0; y < y1; y++ {
					row := src[y*stride:]
					for x := x0; x < x1; x++ {
						sum[0] += int(row[x*4])
						sum[1] += int(row[x*4+1])
						sum[2] += int(row[x*4+2])
					}
				}
				n := (y1 - y0) * (x1 - x0)
				c := [3]uint8{uint8((sum[0] + n/2) / n), uint8((sum[1] + n/2) / n), uint8((sum[2] + n/2) / n)}
				for y := y0; y < y1; y++ {
					row := img.Pix[y*stride:]
					joint := y >= y0+size-gap
					for x := x0; x < x1; x++ {
						p := row[x*4 : x*4+3]
						if joint || x >= x0+size-gap {
							// A dark joint, a fifth of the tile's colour.
							p[0], p[1], p[2] = c[0]/5, c[1]/5, c[2]/5
						} else {
							p[0], p[1], p[2] = c[0], c[1], c[2]
						}
					}
				}
			}
		}
	})
}

// edges finds edges with a Sobel operator on a blurred copy, sampled d
// pixels apart, so thicker lines go with larger d and the result looks
// alike at any resolution. Neon colours the edges with the picture on
// black; pencil draws them dark on paper.
func edges(img *image.RGBA, src []uint8, d int, neon bool) {
	w, h, stride := img.Rect.Dx(), img.Rect.Dy(), img.Stride
	blurred := &image.RGBA{Pix: append([]uint8(nil), src...), Stride: stride, Rect: img.Rect}
	blur(blurred, d)
	nb := neighbours{l: lumaPlane(blurred.Pix, w, h, stride), w: w, h: h, d: d}
	blurred = nil
	gain := pencilGain
	if neon {
		gain = neonGain
	}
	parallel(h, func(from, to int) {
		for y := from; y < to; y++ {
			up, mid, down := nb.rows(y)
			row := img.Pix[y*stride:]
			o := src[y*stride:]
			for x := range w {
				l, c, r := nb.cols(x)
				gx := -int(up[l]) - 2*int(mid[l]) - int(down[l]) + int(up[r]) + 2*int(mid[r]) + int(down[r])
				gy := -int(up[l]) - 2*int(up[c]) - int(up[r]) + int(down[l]) + 2*int(down[c]) + int(down[r])
				e := min(1, math.Sqrt(float64(gx*gx+gy*gy))*gain/255)
				p := row[x*4 : x*4+3]
				if neon {
					k := neonBoost * e
					p[0] = clamp8(float64(o[x*4]) * k)
					p[1] = clamp8(float64(o[x*4+1]) * k)
					p[2] = clamp8(float64(o[x*4+2]) * k)
				} else {
					v := 250 - 230*e
					p[0], p[1], p[2] = clamp8(v), clamp8(v), clamp8(v*0.97)
				}
			}
		}
	})
}

// emboss shades the colours of img as a relief lit from the top left,
// comparing brightness d pixels apart.
func emboss(img *image.RGBA, d int, depth float64) {
	w, h, stride := img.Rect.Dx(), img.Rect.Dy(), img.Stride
	nb := neighbours{l: lumaPlane(img.Pix, w, h, stride), w: w, h: h, d: d}
	parallel(h, func(from, to int) {
		for y := from; y < to; y++ {
			up, mid, down := nb.rows(y)
			row := img.Pix[y*stride:]
			for x := range w {
				l, c, r := nb.cols(x)
				e := float64(-int(up[l])-int(up[c])-int(mid[l])+int(mid[r])+int(down[c])+int(down[r])) * depth
				k := (128 + e) / 128
				p := row[x*4 : x*4+3]
				p[0] = clamp8(float64(p[0]) * k)
				p[1] = clamp8(float64(p[1]) * k)
				p[2] = clamp8(float64(p[2]) * k)
			}
		}
	})
}

// ascii writes the picture with characters of Go Mono, size pixels high,
// on black. Each cell takes the character for its brightness, ranked
// among all cells so the whole ramp is used, and its colour brightened to
// full strength (the character's density already shows how bright the
// cell is). It reports false when the font cannot be loaded.
func ascii(img *image.RGBA, src []uint8, size int) bool {
	w, h, stride := img.Rect.Dx(), img.Rect.Dy(), img.Stride
	face, err := faceOf(max(size, 6), true)
	if err != nil {
		return false
	}
	adv, ok := face.GlyphAdvance('M')
	if !ok {
		return false
	}
	cw := max(1, adv.Ceil())
	ch := max(1, int(math.Ceil(float64(size)*asciiLineGap)))
	m := face.Metrics()
	baseline := m.Ascent + (fixed.I(ch)-m.Height)/2

	// The glyphs, drawn once.
	ramp := []rune(asciiRamp)
	masks := make([]*image.Alpha, len(ramp))
	offsets := make([]image.Point, len(ramp))
	for i, r := range ramp {
		dr, mask, mp, _, ok := face.Glyph(fixed.Point26_6{Y: baseline}, r)
		if !ok || dr.Empty() {
			continue
		}
		a := image.NewAlpha(image.Rect(0, 0, dr.Dx(), dr.Dy()))
		draw.Draw(a, a.Bounds(), mask, mp, draw.Src)
		masks[i], offsets[i] = a, dr.Min
	}

	cols, rows := (w+cw-1)/cw, (h+ch-1)/ch
	cells := make([]color.RGBA, cols*rows)
	lumas := make([]int, cols*rows)
	parallel(rows, func(from, to int) {
		for cy := from; cy < to; cy++ {
			for cx := range cols {
				x0, y0 := cx*cw, cy*ch
				x1, y1 := min(x0+cw, w), min(y0+ch, h)
				var sum [3]int
				for y := y0; y < y1; y++ {
					row := src[y*stride:]
					for x := x0; x < x1; x++ {
						sum[0] += int(row[x*4])
						sum[1] += int(row[x*4+1])
						sum[2] += int(row[x*4+2])
					}
				}
				n := (y1 - y0) * (x1 - x0)
				c := []uint8{uint8(sum[0] / n), uint8(sum[1] / n), uint8(sum[2] / n)}
				cells[cy*cols+cx] = color.RGBA{c[0], c[1], c[2], 255}
				lumas[cy*cols+cx] = luma(c)
			}
		}
	})
	sorted := append([]int(nil), lumas...)
	sort.Ints(sorted)

	draw.Draw(img, img.Rect, image.NewUniform(color.RGBA{8, 8, 10, 255}), image.Point{}, draw.Src)
	last := len(ramp) - 1
	parallel(rows, func(from, to int) {
		for cy := from; cy < to; cy++ {
			for cx := range cols {
				i := cy*cols + cx
				rank := float64(sort.SearchInts(sorted, lumas[i])) / float64(len(sorted))
				g := int(rank*float64(last) + 0.5)
				if masks[g] == nil {
					continue // a space
				}
				c := cells[i]
				top := max(int(c.R), int(c.G), int(c.B), 1)
				k := math.Min(255/float64(top), 2.2)
				fg := color.RGBA{clamp8(float64(c.R) * k), clamp8(float64(c.G) * k), clamp8(float64(c.B) * k), 255}
				cell := image.Rect(cx*cw, cy*ch, cx*cw+cw, cy*ch+ch)
				at := cell.Min.Add(offsets[g])
				// Clipped to the cell: rows of other cells belong to
				// other cores.
				r := image.Rectangle{Min: at, Max: at.Add(masks[g].Rect.Size())}.Intersect(cell).Intersect(img.Rect)
				draw.DrawMask(img, r, image.NewUniform(fg), image.Point{}, masks[g], r.Min.Sub(at), draw.Over)
			}
		}
	})
	return true
}

func clamp8(v float64) uint8 {
	if v <= 0 {
		return 0
	}
	if v >= 255 {
		return 255
	}
	return uint8(v + 0.5)
}
