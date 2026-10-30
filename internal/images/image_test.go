package images

import (
	"strings"
	"testing"
)

func validImage() Image {
	return Image{
		Provider:     "test",
		ProviderID:   "abc",
		PageURL:      "https://example.com/w/abc",
		ThumbnailURL: "https://example.com/th/abc.jpg",
		PreviewURL:   "https://example.com/lg/abc.jpg",
		DownloadURL:  "https://example.com/full/abc.jpg",
		Width:        1920,
		Height:       1080,
		License:      License{Status: LicenseUnknown},
	}
}

func TestOrientation(t *testing.T) {
	tests := []struct {
		width, height int
		want          Orientation
	}{
		{1920, 1080, Landscape},
		{1080, 1920, Portrait},
		{1000, 1000, Square},
		{1040, 1000, Square}, // within 5%
		{1000, 1040, Square},
		{1920, 0, ""},
	}
	for _, tt := range tests {
		img := Image{Width: tt.width, Height: tt.height}
		if got := img.Orientation(); got != tt.want {
			t.Errorf("%dx%d: Orientation() = %q, want %q", tt.width, tt.height, got, tt.want)
		}
	}
}

func TestAspectRatio(t *testing.T) {
	if got := (Image{Width: 1920, Height: 1200}).AspectRatio(); got != 1.6 {
		t.Errorf("AspectRatio() = %v, want 1.6", got)
	}
	if got := (Image{Width: 1920}).AspectRatio(); got != 0 {
		t.Errorf("AspectRatio() with unknown height = %v, want 0", got)
	}
}

func TestValidateAcceptsCompleteImage(t *testing.T) {
	if err := validImage().Validate(); err != nil {
		t.Fatal(err)
	}

	known := validImage()
	known.License = License{Status: LicenseKnown, Name: "CC BY 4.0", URL: "https://creativecommons.org/licenses/by/4.0/"}
	if err := known.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestValidateRejectsIncompleteImages(t *testing.T) {
	tests := map[string]func(*Image){
		"missing provider":       func(img *Image) { img.Provider = "" },
		"missing provider ID":    func(img *Image) { img.ProviderID = "" },
		"zero width":             func(img *Image) { img.Width = 0 },
		"missing thumbnail":      func(img *Image) { img.ThumbnailURL = "" },
		"relative download URL":  func(img *Image) { img.DownloadURL = "/full/abc.jpg" },
		"javascript page URL":    func(img *Image) { img.PageURL = "javascript:alert(1)" },
		"file thumbnail URL":     func(img *Image) { img.ThumbnailURL = "file:///etc/passwd" },
		"license status not set": func(img *Image) { img.License = License{} },
		"known license, no name": func(img *Image) { img.License = License{Status: LicenseKnown} },
		"invalid license status": func(img *Image) { img.License.Status = "free" },
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			img := validImage()
			mutate(&img)
			if err := img.Validate(); err == nil {
				t.Error("expected a validation error")
			}
		})
	}
}

func TestValidateReportsAllProblems(t *testing.T) {
	err := Image{Provider: "test", ProviderID: "abc"}.Validate()
	if err == nil {
		t.Fatal("expected a validation error")
	}
	for _, part := range []string{"dimensions", "thumbnail URL", "license status"} {
		if !strings.Contains(err.Error(), part) {
			t.Errorf("error %q does not mention %q", err, part)
		}
	}
}
