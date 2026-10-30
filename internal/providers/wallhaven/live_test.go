//go:build live

// Tests against the real Wallhaven API. They are excluded from normal runs;
// use `make test-live` to check that the API still behaves as expected.

package wallhaven

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/sentivanac/kisetsu/internal/images"
	"github.com/sentivanac/kisetsu/internal/providers"
)

func TestLiveSearch(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	p := New(&http.Client{Timeout: 15 * time.Second}, "Kisetsu/live-test (+https://github.com/sentivanac/kisetsu)")

	page, err := p.Search(ctx, providers.Query{
		Text: "mountains -snow", Page: 1, MinWidth: 1920, MinHeight: 1080, Orientation: images.Landscape,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Images) == 0 {
		t.Fatal("no results")
	}
	for _, img := range page.Images {
		if err := img.Validate(); err != nil {
			t.Error(err)
		}
		if img.Width < 1920 || img.Height < 1080 || img.Orientation() != images.Landscape {
			t.Errorf("%s: %dx%d does not match the filters", img.ProviderID, img.Width, img.Height)
		}
	}
	t.Logf("%d results, first: %s (%dx%d)", len(page.Images), page.Images[0].PageURL, page.Images[0].Width, page.Images[0].Height)
}
