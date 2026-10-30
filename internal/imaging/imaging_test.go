package imaging

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"hash/crc32"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"math"
	"os"
	"testing"

	"golang.org/x/image/draw"
)

// scene draws a test picture with gradients and shapes, so the perceptual
// hash has real structure to work with. variant changes the composition.
func scene(w, h, variant int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			fx, fy := float64(x)/float64(w), float64(y)/float64(h)
			var v float64
			switch variant {
			case 0: // sky above a dark ground, with a sun
				v = 200 - 150*fy
				if fy > 0.65 {
					v = 40
				}
				if math.Hypot(fx-0.7, fy-0.3) < 0.1 {
					v = 255
				}
			default: // vertical stripes
				v = 127 + 127*math.Sin(fx*20)
			}
			img.Set(x, y, color.RGBA{uint8(v), uint8(v * 0.9), uint8(255 - v/2), 255})
		}
	}
	return img
}

func encodeJPEG(t *testing.T, img image.Image, quality int) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: quality}); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func encodePNG(t *testing.T, img image.Image) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func process(t *testing.T, data []byte) (Result, error) {
	t.Helper()
	return Process(bytes.NewReader(data))
}

func decodedSize(t *testing.T, jpegData []byte) (int, int) {
	t.Helper()
	cfg, format, err := image.DecodeConfig(bytes.NewReader(jpegData))
	if err != nil || format != "jpeg" {
		t.Fatalf("thumbnail is not a JPEG: %v, %q", err, format)
	}
	return cfg.Width, cfg.Height
}

func TestProcessJPEG(t *testing.T) {
	data := encodeJPEG(t, scene(1200, 800, 0), 90)

	res, err := process(t, data)
	if err != nil {
		t.Fatal(err)
	}

	sum := sha256.Sum256(data)
	if res.SHA256 != hex.EncodeToString(sum[:]) {
		t.Errorf("SHA256 = %s", res.SHA256)
	}
	if res.Width != 1200 || res.Height != 800 || res.MIMEType != "image/jpeg" {
		t.Errorf("got %dx%d %s", res.Width, res.Height, res.MIMEType)
	}
	if w, h := decodedSize(t, res.Thumbnail); w != ThumbnailSize || h != 320 {
		t.Errorf("thumbnail = %dx%d, want %dx320", w, h, ThumbnailSize)
	}
}

func TestProcessPortraitThumbnail(t *testing.T) {
	res, err := process(t, encodePNG(t, scene(600, 1200, 0)))
	if err != nil {
		t.Fatal(err)
	}
	if res.MIMEType != "image/png" {
		t.Errorf("MIMEType = %s", res.MIMEType)
	}
	if w, h := decodedSize(t, res.Thumbnail); w != 240 || h != ThumbnailSize {
		t.Errorf("thumbnail = %dx%d, want 240x%d", w, h, ThumbnailSize)
	}
}

func TestProcessWebPAndNoUpscaling(t *testing.T) {
	data, err := os.ReadFile("testdata/gradient.webp") // 64x48, made with Pillow
	if err != nil {
		t.Fatal(err)
	}

	res, err := process(t, data)
	if err != nil {
		t.Fatal(err)
	}
	if res.Width != 64 || res.Height != 48 || res.MIMEType != "image/webp" {
		t.Errorf("got %dx%d %s", res.Width, res.Height, res.MIMEType)
	}
	if w, h := decodedSize(t, res.Thumbnail); w != 64 || h != 48 {
		t.Errorf("small image was resized to %dx%d", w, h)
	}
}

func TestProcessTransparentPNG(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 100, 100)) // fully transparent

	res, err := process(t, encodePNG(t, img))
	if err != nil {
		t.Fatal(err)
	}
	thumb, err := jpeg.Decode(bytes.NewReader(res.Thumbnail))
	if err != nil {
		t.Fatal(err)
	}
	if r, g, b, _ := thumb.At(50, 50).RGBA(); r>>8 > 8 || g>>8 > 8 || b>>8 > 8 {
		t.Errorf("transparent area became %d,%d,%d, want black", r>>8, g>>8, b>>8)
	}
}

func TestProcessRejectsUnsupportedFormats(t *testing.T) {
	var gifData bytes.Buffer
	if err := gif.Encode(&gifData, scene(20, 20, 0), nil); err != nil {
		t.Fatal(err)
	}
	inputs := map[string][]byte{
		"GIF":  gifData.Bytes(),
		"text": []byte("<html>not an image</html>"),
	}
	for name, data := range inputs {
		if _, err := process(t, data); !errors.Is(err, ErrUnsupportedFormat) {
			t.Errorf("%s: err = %v, want ErrUnsupportedFormat", name, err)
		}
	}
}

func TestProcessRejectsDamagedImage(t *testing.T) {
	data := encodeJPEG(t, scene(400, 300, 0), 90)

	if _, err := process(t, data[:len(data)/2]); !errors.Is(err, ErrCorrupt) {
		t.Errorf("truncated JPEG: err = %v, want ErrCorrupt", err)
	}
}

// pngHeader returns only a PNG signature and IHDR chunk, which is all
// DecodeConfig reads. It lets the test claim huge dimensions cheaply.
func pngHeader(w, h uint32) []byte {
	ihdr := make([]byte, 13)
	binary.BigEndian.PutUint32(ihdr[0:], w)
	binary.BigEndian.PutUint32(ihdr[4:], h)
	ihdr[8], ihdr[9] = 8, 2 // 8-bit RGB

	var buf bytes.Buffer
	buf.WriteString("\x89PNG\r\n\x1a\n")
	binary.Write(&buf, binary.BigEndian, uint32(len(ihdr)))
	chunk := append([]byte("IHDR"), ihdr...)
	buf.Write(chunk)
	binary.Write(&buf, binary.BigEndian, crc32.ChecksumIEEE(chunk))
	return buf.Bytes()
}

func TestProcessRejectsDecompressionBombs(t *testing.T) {
	// 50 000 × 50 000 = 2.5 gigapixels in a few dozen bytes.
	if _, err := process(t, pngHeader(50_000, 50_000)); !errors.Is(err, ErrTooLarge) {
		t.Errorf("err = %v, want ErrTooLarge", err)
	}
}

func TestDHashFindsSameImageAfterReencoding(t *testing.T) {
	original := scene(1600, 1000, 0)
	base, err := process(t, encodeJPEG(t, original, 95))
	if err != nil {
		t.Fatal(err)
	}

	half := image.NewRGBA(image.Rect(0, 0, 800, 500))
	draw.CatmullRom.Scale(half, half.Bounds(), original, original.Bounds(), draw.Src, nil)
	variants := map[string][]byte{
		"low JPEG quality": encodeJPEG(t, original, 25),
		"half size":        encodeJPEG(t, half, 80),
		"PNG":              encodePNG(t, half),
	}
	for name, data := range variants {
		res, err := process(t, data)
		if err != nil {
			t.Fatal(err)
		}
		if d := Distance(base.DHash, res.DHash); !Similar(base.DHash, res.DHash) {
			t.Errorf("%s: distance %d, want at most %d", name, d, SimilarDistance)
		}
		if res.SHA256 == base.SHA256 {
			t.Errorf("%s: a different file has the same SHA-256", name)
		}
	}

	other, err := process(t, encodeJPEG(t, scene(1600, 1000, 1), 95))
	if err != nil {
		t.Fatal(err)
	}
	if d := Distance(base.DHash, other.DHash); Similar(base.DHash, other.DHash) {
		t.Errorf("different images: distance %d, want more than %d", d, SimilarDistance)
	}
}

func TestDistance(t *testing.T) {
	if d := Distance(0, math.MaxUint64); d != 64 {
		t.Errorf("Distance(0, max) = %d", d)
	}
	if d := Distance(0b1011, 0b0010); d != 2 {
		t.Errorf("Distance = %d, want 2", d)
	}
}
