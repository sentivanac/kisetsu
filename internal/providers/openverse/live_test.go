//go:build live

// Tests against the real Openverse API. They are excluded from normal
// runs; use `make test-live` to check that the API still behaves as
// expected. Openverse allows 20 requests a minute without a key.

package openverse

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/sentivanac/kisetsu/internal/images"
	"github.com/sentivanac/kisetsu/internal/providers"
)

const liveUserAgent = "Kisetsu/live-test (+https://github.com/sentivanac/kisetsu)"

func TestLiveSearch(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	client := &http.Client{Timeout: 15 * time.Second}
	p := New(client, liveUserAgent)

	page, err := p.Search(ctx, providers.Query{
		Text: "harbour -night", Page: 1, MinWidth: 1600, Orientation: images.Landscape,
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
		if img.Width < 1600 || img.Orientation() != images.Landscape {
			t.Errorf("%s: %dx%d does not match the filters", img.ProviderID, img.Width, img.Height)
		}
	}

	// The thumbnail comes from Openverse, the file from the collection.
	first := page.Images[0]
	for name, u := range map[string]string{"thumbnail": first.ThumbnailURL, "file": first.DownloadURL} {
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		req.Header.Set("User-Agent", liveUserAgent)
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("%s %s: HTTP %d", name, u, resp.StatusCode)
		}
	}
	t.Logf("%d results, first: %s (%dx%d, %s, by %q)", len(page.Images), first.PageURL,
		first.Width, first.Height, first.License.Name, first.Author.Name)
}
