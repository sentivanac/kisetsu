package overlay

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"strings"
	"testing"
)

func uniform(w, h int, c color.Color) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, c)
		}
	}
	return img
}

// changed returns the bounding box of the pixels that differ.
func changed(a, b *image.RGBA) image.Rectangle {
	var r image.Rectangle
	for y := a.Rect.Min.Y; y < a.Rect.Max.Y; y++ {
		for x := a.Rect.Min.X; x < a.Rect.Max.X; x++ {
			if a.RGBAAt(x, y) != b.RGBAAt(x, y) {
				r = r.Union(image.Rect(x, y, x+1, y+1))
			}
		}
	}
	return r
}

func near(t *testing.T, got color.RGBA, want color.RGBA) {
	t.Helper()
	d := func(a, b uint8) int { return max(int(a)-int(b), int(b)-int(a)) }
	if d(got.R, want.R) > 8 || d(got.G, want.G) > 8 || d(got.B, want.B) > 8 {
		t.Errorf("colour %v, want about %v", got, want)
	}
}

var (
	red   = color.RGBA{255, 0, 0, 255}
	green = color.RGBA{0, 255, 0, 255}
	blue  = color.RGBA{0, 0, 255, 255}
	grey  = color.RGBA{128, 128, 128, 255}
)

// stripes is red, green and blue along the long side, with the green
// middle exactly as long as the short side.
func stripes(long, short int, wide bool) *image.RGBA {
	w, h := long, short
	if !wide {
		w, h = short, long
	}
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	lo := (long - short) / 2
	for y := range h {
		for x := range w {
			p := x
			if !wide {
				p = y
			}
			c := green
			if p < lo {
				c = red
			} else if p >= lo+short {
				c = blue
			}
			img.SetRGBA(x, y, c)
		}
	}
	return img
}

func TestBaseCropsAroundTheCentre(t *testing.T) {
	for _, wide := range []bool{true, false} {
		base, err := Base(stripes(400, 100, wide), 50, 50)
		if err != nil {
			t.Fatal(err)
		}
		if got := base.Bounds(); got != image.Rect(0, 0, 50, 50) {
			t.Fatalf("wide=%v: bounds %v", wide, got)
		}
		for _, p := range []image.Point{{1, 1}, {48, 1}, {1, 48}, {48, 48}, {25, 25}} {
			near(t, base.RGBAAt(p.X, p.Y), green)
		}
	}
}

func TestBaseCropsAFittedImageWithoutScaling(t *testing.T) {
	for _, wide := range []bool{true, false} {
		src := stripes(400, 100, wide)
		base, err := Base(src, 100, 100)
		if err != nil {
			t.Fatal(err)
		}
		for _, p := range []image.Point{{0, 0}, {99, 0}, {0, 99}, {99, 99}} {
			if got := base.RGBAAt(p.X, p.Y); got != green {
				t.Errorf("wide=%v: %v is %v, want exactly green", wide, p, got)
			}
		}
	}
}

func TestBaseHasTheScreenSize(t *testing.T) {
	for _, tc := range []struct{ srcW, srcH, w, h int }{
		{5000, 3000, 1920, 1080}, // larger, wider
		{1000, 1000, 1920, 1080}, // smaller: enlarged
		{1080, 1920, 1920, 1080}, // portrait on a landscape screen
		{1920, 1080, 1080, 1920}, // landscape on a portrait screen
		{1920, 1080, 1920, 1080}, // the same
	} {
		base, err := Base(uniform(tc.srcW, tc.srcH, grey), tc.w, tc.h)
		if err != nil {
			t.Fatal(err)
		}
		if got := base.Bounds(); got != image.Rect(0, 0, tc.w, tc.h) {
			t.Errorf("%dx%d to %dx%d: bounds %v", tc.srcW, tc.srcH, tc.w, tc.h, got)
		}
		near(t, base.RGBAAt(tc.w-1, tc.h-1), grey)
	}
}

func TestBaseAcceptsAnOffsetImage(t *testing.T) {
	img := stripes(400, 100, true).SubImage(image.Rect(100, 0, 300, 100))
	base, err := Base(img, 20, 10)
	if err != nil {
		t.Fatal(err)
	}
	near(t, base.RGBAAt(10, 5), green)
}

func TestBaseRejectsAnInvalidSize(t *testing.T) {
	if _, err := Base(uniform(10, 10, grey), 0, 10); err == nil {
		t.Error("no error for width 0")
	}
}

func TestDrawPutsTextInItsCorner(t *testing.T) {
	const w, h = 800, 450
	base := uniform(w, h, grey)
	for _, tc := range []struct {
		corner  Corner
		quarter image.Rectangle
	}{
		{TopLeft, image.Rect(0, 0, w/2, h/2)},
		{TopRight, image.Rect(w/2, 0, w, h/2)},
		{BottomLeft, image.Rect(0, h/2, w/2, h)},
		{BottomRight, image.Rect(w/2, h/2, w, h)},
	} {
		out, err := Draw(base, []Block{{Lines: []string{"12:34"}, Corner: tc.corner, Size: 40}})
		if err != nil {
			t.Fatal(err)
		}
		r := changed(base, out)
		if r.Empty() {
			t.Fatalf("%s: nothing drawn", tc.corner)
		}
		if !r.In(tc.quarter) {
			t.Errorf("%s: text at %v, outside %v", tc.corner, r, tc.quarter)
		}
		// Clear of panels and docks: at least the margin from every edge
		// (2 px for the shadow).
		var fh float64 = h
		m := int(fh * margin)
		if r.Min.Y < m-2 || r.Max.Y > h-m+2 || r.Min.X < m-2 || r.Max.X > w-m+2 {
			t.Errorf("%s: text at %v, inside the margin of %d px", tc.corner, r, m)
		}
	}
}

func TestDrawDoesNotChangeTheBase(t *testing.T) {
	base := uniform(400, 300, grey)
	before := append([]uint8(nil), base.Pix...)
	if _, err := Draw(base, []Block{{Lines: []string{"Hello"}, Corner: TopLeft, Size: 30}}); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, base.Pix) {
		t.Error("the base was changed")
	}
}

func TestDrawUsesWhiteTextWithAShadow(t *testing.T) {
	base := uniform(400, 300, grey)
	out, err := Draw(base, []Block{{Lines: []string{"HHHH"}, Corner: TopLeft, Size: 60}})
	if err != nil {
		t.Fatal(err)
	}
	var white, darker bool
	for i := 0; i < len(out.Pix); i += 4 {
		switch r := out.Pix[i]; {
		case r > 240:
			white = true
		case r < 100:
			darker = true
		}
	}
	if !white || !darker {
		t.Errorf("white text %v, darker shadow %v", white, darker)
	}
}

func TestDrawStacksBlocksInOneCorner(t *testing.T) {
	base := uniform(800, 450, grey)
	one, err := Draw(base, []Block{{Lines: []string{"12:34"}, Corner: BottomRight, Size: 40}})
	if err != nil {
		t.Fatal(err)
	}
	two, err := Draw(base, []Block{
		{Lines: []string{"12:34"}, Corner: BottomRight, Size: 40},
		{Lines: []string{"Photo: Ana", "CC BY 4.0"}, Corner: BottomRight, Size: 16},
	})
	if err != nil {
		t.Fatal(err)
	}
	r1, r2 := changed(base, one), changed(base, two)
	if r2.Dy() <= r1.Dy()+20 {
		t.Errorf("two blocks %v are not taller than one %v", r2, r1)
	}
	if r2.Min.Y >= r1.Min.Y {
		t.Errorf("the stack did not grow upwards: one %v, two %v", r1, r2)
	}
}

func TestDrawShortensLongLines(t *testing.T) {
	const w = 800
	base := uniform(w, 450, grey)
	var fw float64 = w
	out, err := Draw(base, []Block{{Lines: []string{strings.Repeat("Very long author name ", 20)}, Corner: TopLeft, Size: 20}})
	if err != nil {
		t.Fatal(err)
	}
	if r := changed(base, out); r.Dx() > int(fw*maxLineWidth)+4 {
		t.Errorf("line is %d px wide, more than %v of %d", r.Dx(), maxLineWidth, w)
	}
}

func TestDrawRejectsBadBlocks(t *testing.T) {
	base := uniform(100, 100, grey)
	if _, err := Draw(base, []Block{{Lines: []string{"x"}, Corner: "middle", Size: 20}}); err == nil {
		t.Error("no error for an invalid corner")
	}
	if _, err := Draw(base, []Block{{Lines: []string{"x"}, Corner: TopLeft, Size: 0}}); err == nil {
		t.Error("no error for size 0")
	}
	out, err := Draw(base, []Block{{Corner: TopLeft, Size: 20}})
	if err != nil || !changed(base, out).Empty() {
		t.Errorf("a block without lines: err %v, drew %v", err, changed(base, out))
	}
}

func TestDrawable(t *testing.T) {
	for s, want := range map[string]bool{
		"Photo: Ana Ćirić · CC BY-SA 4.0": true,
		"Иван Петров":                     true,
		"Ελένη":                           true,
		"12:34 Thu 5 Nov":                 true,
		"山田太郎":                            false,
		"Ana 山田":                          false,
	} {
		if got := Drawable(s); got != want {
			t.Errorf("Drawable(%q) = %v, want %v", s, got, want)
		}
	}
}

func TestEncode(t *testing.T) {
	var buf bytes.Buffer
	if err := Encode(&buf, uniform(64, 48, grey)); err != nil {
		t.Fatal(err)
	}
	img, err := jpeg.Decode(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if img.Bounds() != image.Rect(0, 0, 64, 48) {
		t.Errorf("bounds %v", img.Bounds())
	}
}

// BenchmarkMinute is the clock's work every minute on a 4K screen.
func BenchmarkMinute(b *testing.B) {
	base := uniform(3840, 2160, grey)
	blocks := []Block{
		{Lines: []string{"12:34", "Thursday, 5 November"}, Corner: BottomRight, Size: 120},
		{Lines: []string{"Photo: Ana Ćirić · CC BY-SA 4.0"}, Corner: BottomLeft, Size: 36},
	}
	for b.Loop() {
		out, err := Draw(base, blocks)
		if err != nil {
			b.Fatal(err)
		}
		var buf bytes.Buffer
		if err := Encode(&buf, out); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkBase is the work once per wallpaper: a 24 MP photo for 4K.
func BenchmarkBase(b *testing.B) {
	src := uniform(6000, 4000, grey)
	for b.Loop() {
		if _, err := Base(src, 3840, 2160); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkBaseFitted is the usual case: a fetched image, already reduced
// to cover the screen.
func BenchmarkBaseFitted(b *testing.B) {
	src := uniform(4320, 2160, grey)
	for b.Loop() {
		if _, err := Base(src, 3840, 2160); err != nil {
			b.Fatal(err)
		}
	}
}
