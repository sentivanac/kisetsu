package imaging

import (
	"image"
	"image/draw"
	"math"
)

// resample scales src to w×h with a Catmull-Rom filter (ADR 0052), like
// draw.CatmullRom from golang.org/x/image, but in little memory: that one
// keeps the whole image filtered along the rows, 32 bytes per pixel of
// w × source height (530 MB from a large photo to a 4K screen). Here the
// rows are filtered one source row at a time, and only the few rows the
// filter needs for the next output row are kept.
//
// The filter is widened by the scale factor when shrinking, so every
// source pixel counts. Transparent areas become black, as before.
func resample(src image.Image, w, h int) *image.RGBA {
	sb := src.Bounds()
	sw, sh := sb.Dx(), sb.Dy()
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	if w <= 0 || h <= 0 || sw <= 0 || sh <= 0 {
		return dst
	}
	cols := weights(sw, w)
	rows := weights(sh, h)

	// The rows one output row needs; they move down the source as the
	// output does, so a small ring holds them.
	ring := 0
	for _, r := range rows {
		ring = max(ring, len(r.taps))
	}
	cache := make([][]float32, ring)
	cached := make([]int, ring) // the source row in each slot; -1 none
	for i := range cache {
		cache[i] = make([]float32, w*4)
		cached[i] = -1
	}
	line := image.NewRGBA(image.Rect(0, 0, sw, 1))
	filtered := func(y int) []float32 {
		slot := y % ring
		if cached[slot] == y {
			return cache[slot]
		}
		// One source row in RGBA: draw.Draw has fast paths from the
		// YCbCr images the JPEG decoder returns.
		draw.Draw(line, line.Bounds(), src, image.Pt(sb.Min.X, sb.Min.Y+y), draw.Src)
		out := cache[slot]
		for x, c := range cols {
			// Alpha is not needed: colors are premultiplied, and the
			// result is over black.
			var r, g, b float32
			for _, t := range c.taps {
				p := line.Pix[t.at*4 : t.at*4+3]
				r += t.w * float32(p[0])
				g += t.w * float32(p[1])
				b += t.w * float32(p[2])
			}
			out[x*4], out[x*4+1], out[x*4+2] = r, g, b
		}
		cached[slot] = y
		return out
	}

	lines := make([][]float32, 0, ring)
	for y, rw := range rows {
		row := dst.Pix[y*dst.Stride : y*dst.Stride+w*4]
		// The rows of one output row are consecutive source rows, at
		// most ring of them, so they sit in different slots.
		lines = lines[:0]
		for _, t := range rw.taps {
			lines = append(lines, filtered(t.at))
		}
		for x := range w {
			var r, g, b float32
			for i, t := range rw.taps {
				p := lines[i][x*4 : x*4+4]
				r += t.w * p[0]
				g += t.w * p[1]
				b += t.w * p[2]
			}
			// Premultiplied colors over black: the color stays, alpha
			// becomes opaque.
			row[x*4], row[x*4+1], row[x*4+2], row[x*4+3] = clamp8(r), clamp8(g), clamp8(b), 255
		}
	}
	return dst
}

type tap struct {
	at int     // source index
	w  float32 // weight
}

type contribution struct{ taps []tap }

// weights returns, for each of n output pixels, the source pixels of a
// line of size pixels it is made from and their weights, which add up to
// 1. Indexes past the edge repeat the edge pixel.
func weights(size, n int) []contribution {
	scale := float64(size) / float64(n)
	widen := max(scale, 1) // shrinking widens the filter
	radius := 2 * widen    // Catmull-Rom reaches 2 pixels
	out := make([]contribution, n)
	for i := range out {
		center := (float64(i)+0.5)*scale - 0.5
		lo := int(math.Floor(center - radius + 1))
		hi := int(math.Floor(center + radius))
		var taps []tap
		var sum float64
		for j := lo; j <= hi; j++ {
			wt := catmullRom((float64(j) - center) / widen)
			if wt == 0 {
				continue
			}
			at := min(max(j, 0), size-1)
			if k := len(taps); k > 0 && taps[k-1].at == at {
				taps[k-1].w += float32(wt)
			} else {
				taps = append(taps, tap{at: at, w: float32(wt)})
			}
			sum += wt
		}
		for k := range taps {
			taps[k].w /= float32(sum)
		}
		out[i].taps = taps
	}
	return out
}

func catmullRom(x float64) float64 {
	x = math.Abs(x)
	switch {
	case x < 1:
		return (1.5*x-2.5)*x*x + 1
	case x < 2:
		return ((-0.5*x+2.5)*x-4)*x + 2
	}
	return 0
}

func clamp8(v float32) uint8 {
	switch {
	case v <= 0:
		return 0
	case v >= 255:
		return 255
	}
	return uint8(v + 0.5)
}
