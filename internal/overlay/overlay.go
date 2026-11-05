// Package overlay draws text on a wallpaper (ADR 0047): the image's
// author and license, and a clock. Kisetsu then sets the composed image
// instead of the original, as Variety does for its clock; the original
// file and the history are never changed.
//
// The composed image has exactly the size of the screen, cropped around
// the centre like the "Zoom"/"Fill" picture option, so the text lands in
// the chosen corner whatever option the desktop uses.
//
// The text uses the Go fonts (BSD licensed, part of golang.org/x/image),
// which cover Latin, Greek and Cyrillic; Drawable tells whether a text can
// be drawn without missing glyphs.
package overlay

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"io"
	"strings"
	"sync"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"

	"github.com/sentivanac/kisetsu/internal/imaging"
)

// Corner is where a block of text is drawn.
type Corner string

const (
	TopLeft     Corner = "top-left"
	TopRight    Corner = "top-right"
	BottomLeft  Corner = "bottom-left"
	BottomRight Corner = "bottom-right"
)

// Valid reports whether c is one of the four corners.
func (c Corner) Valid() bool {
	switch c {
	case TopLeft, TopRight, BottomLeft, BottomRight:
		return true
	}
	return false
}

func (c Corner) right() bool  { return c == TopRight || c == BottomRight }
func (c Corner) bottom() bool { return c == BottomLeft || c == BottomRight }

// Block is text drawn in one corner. Blocks in the same corner are stacked
// in the order given, from the top down.
type Block struct {
	Lines  []string
	Corner Corner
	// Size is the font size in pixels.
	Size int
}

const (
	// Margins as a share of the screen height. The vertical one is larger
	// so the text stays clear of panels and the Windows taskbar, which are
	// drawn over the wallpaper.
	sideMargin     = 0.03
	verticalMargin = 0.07
	// maxLineWidth is the widest a line may be, as a share of the screen
	// width; longer lines are shortened with "…".
	maxLineWidth = 0.6
	// blockGap is the space between stacked blocks, in lines.
	blockGap = 0.4
	// jpegQuality of the composed image.
	jpegQuality = 90
)

var (
	textColor   = image.NewUniform(color.White)
	shadowColor = image.NewUniform(color.NRGBA{A: 110})
)

// Base reduces img to exactly w×h: scaled to cover it and cropped around
// the centre. It is computed once per image; Draw copies it.
func Base(img image.Image, w, h int) (*image.RGBA, error) {
	if w <= 0 || h <= 0 {
		return nil, fmt.Errorf("invalid screen size %dx%d", w, h)
	}
	b := img.Bounds()
	if b.Dx() <= 0 || b.Dy() <= 0 {
		return nil, fmt.Errorf("empty image")
	}
	crop := cropRect(b, w, h)
	if crop.Dx() == w && crop.Dy() == h {
		// Fetched images are already reduced to cover the screen
		// (imaging.Options.Fit), so usually only the crop is left.
		dst := image.NewRGBA(image.Rect(0, 0, w, h))
		draw.Draw(dst, dst.Bounds(), img, crop.Min, draw.Src)
		return dst, nil
	}
	// Scaling a large photo takes a second or two, once per wallpaper.
	return imaging.Resize(subImage(img, crop), w, h), nil
}

// cropRect returns the largest centred part of b with the aspect ratio
// w:h.
func cropRect(b image.Rectangle, w, h int) image.Rectangle {
	cw, ch := b.Dx(), b.Dy()
	// Compare cw/ch with w/h without floating point.
	if int64(cw)*int64(h) > int64(ch)*int64(w) {
		cw = max(1, int(int64(ch)*int64(w)/int64(h)))
	} else {
		ch = max(1, int(int64(cw)*int64(h)/int64(w)))
	}
	x := b.Min.X + (b.Dx()-cw)/2
	y := b.Min.Y + (b.Dy()-ch)/2
	return image.Rect(x, y, x+cw, y+ch)
}

func subImage(img image.Image, r image.Rectangle) image.Image {
	if s, ok := img.(interface {
		SubImage(image.Rectangle) image.Image
	}); ok {
		return s.SubImage(r)
	}
	dst := image.NewRGBA(image.Rect(0, 0, r.Dx(), r.Dy()))
	draw.Draw(dst, dst.Bounds(), img, r.Min, draw.Src)
	return dst
}

// Draw returns a copy of base with the blocks drawn on it; base is not
// changed.
func Draw(base *image.RGBA, blocks []Block) (*image.RGBA, error) {
	out := &image.RGBA{
		Pix:    append([]uint8(nil), base.Pix...),
		Stride: base.Stride,
		Rect:   base.Rect,
	}
	w, h := out.Rect.Dx(), out.Rect.Dy()

	// Lay out each corner as one stack.
	type line struct {
		text string
		face font.Face
	}
	stacks := map[Corner][][]line{}
	for _, bl := range blocks {
		if !bl.Corner.Valid() {
			return nil, fmt.Errorf("invalid corner %q", bl.Corner)
		}
		if len(bl.Lines) == 0 {
			continue
		}
		face, err := faceOf(bl.Size)
		if err != nil {
			return nil, err
		}
		defer face.Close()
		var ls []line
		for _, t := range bl.Lines {
			ls = append(ls, line{shorten(face, t, int(float64(w)*maxLineWidth)), face})
		}
		stacks[bl.Corner] = append(stacks[bl.Corner], ls)
	}

	sx := int(float64(h) * sideMargin)
	sy := int(float64(h) * verticalMargin)
	for corner, stack := range stacks {
		// Total height, to place bottom stacks.
		total := 0
		for i, ls := range stack {
			if i > 0 {
				total += gap(ls[0].face)
			}
			for _, l := range ls {
				total += lineHeight(l.face)
			}
		}
		y := sy
		if corner.bottom() {
			y = h - sy - total
		}
		for i, ls := range stack {
			if i > 0 {
				y += gap(ls[0].face)
			}
			for _, l := range ls {
				m := l.face.Metrics()
				x := sx
				if corner.right() {
					x = w - sx - font.MeasureString(l.face, l.text).Ceil()
				}
				drawText(out, l.face, l.text, x, y+m.Ascent.Ceil())
				y += lineHeight(l.face)
			}
		}
	}
	return out, nil
}

func lineHeight(f font.Face) int { return f.Metrics().Height.Ceil() }

func gap(f font.Face) int { return int(float64(lineHeight(f)) * blockGap) }

// drawText draws s with its baseline at y, over a soft shadow that keeps
// it legible on light and dark images alike.
func drawText(dst *image.RGBA, face font.Face, s string, x, y int) {
	d := &font.Drawer{Dst: dst, Face: face}
	off := max(1, face.Metrics().Height.Ceil()/20)
	d.Src = shadowColor
	for _, o := range [][2]int{{-off, 0}, {off, 0}, {0, -off}, {0, off}, {off, off}} {
		d.Dot = fixed.P(x+o[0], y+o[1])
		d.DrawString(s)
	}
	d.Src = textColor
	d.Dot = fixed.P(x, y)
	d.DrawString(s)
}

// shorten cuts s to at most maxWidth pixels, ending it with "…".
func shorten(face font.Face, s string, maxWidth int) string {
	if font.MeasureString(face, s).Ceil() <= maxWidth {
		return s
	}
	r := []rune(s)
	for len(r) > 0 {
		r = r[:len(r)-1]
		t := strings.TrimRight(string(r), " ") + "…"
		if font.MeasureString(face, t).Ceil() <= maxWidth {
			return t
		}
	}
	return ""
}

// Encode writes img as a JPEG.
func Encode(w io.Writer, img image.Image) error {
	return jpeg.Encode(w, img, &jpeg.Options{Quality: jpegQuality})
}

var (
	fontOnce sync.Once
	goFont   *sfnt.Font
	fontErr  error
)

func parsedFont() (*sfnt.Font, error) {
	fontOnce.Do(func() { goFont, fontErr = opentype.Parse(goregular.TTF) })
	return goFont, fontErr
}

// faceOf returns the font at size pixels. A face is not safe for
// concurrent use, so each Draw makes its own; that is cheap.
func faceOf(size int) (font.Face, error) {
	if size < 6 || size > 1000 {
		return nil, fmt.Errorf("invalid font size %d", size)
	}
	f, err := parsedFont()
	if err != nil {
		return nil, err
	}
	// At 72 DPI, points are pixels.
	return opentype.NewFace(f, &opentype.FaceOptions{Size: float64(size), DPI: 72, Hinting: font.HintingFull})
}

// Drawable reports whether the font has a glyph for every character of s,
// so it is drawn without empty boxes (it has none for Chinese or Japanese,
// for example).
func Drawable(s string) bool {
	f, err := parsedFont()
	if err != nil {
		return false
	}
	var buf sfnt.Buffer
	for _, r := range s {
		if i, err := f.GlyphIndex(&buf, r); err != nil || i == 0 {
			return false
		}
	}
	return true
}
