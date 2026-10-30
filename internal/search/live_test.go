//go:build live

// Multi-source search against the real provider APIs. Excluded from normal
// runs; use `make test-live`.

package search

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"testing"
	"time"

	"github.com/sentivanac/kisetsu/internal/providers"
	"github.com/sentivanac/kisetsu/internal/providers/commons"
	"github.com/sentivanac/kisetsu/internal/providers/wallhaven"
)

func TestLiveSearchMany(t *testing.T) {
	client := &http.Client{Timeout: 15 * time.Second}
	ua := "Kisetsu/live-test (+https://github.com/sentivanac/kisetsu)"
	s, err := New(slog.New(slog.NewTextHandler(io.Discard, nil)),
		wallhaven.New(client, ua), commons.New(client, ua))
	if err != nil {
		t.Fatal(err)
	}

	start := time.Now()
	got, err := s.SearchMany(context.Background(),
		[]SourcePage{{ProviderID: wallhaven.ID, Page: 1}, {ProviderID: commons.ID, Page: 1}},
		providers.Query{Text: "lake -snow", MinWidth: 1920, MinHeight: 1080})
	if err != nil {
		t.Fatal(err)
	}

	for _, out := range got.Sources {
		if out.Err != nil {
			t.Errorf("%s: %v", out.ProviderID, out.Err)
		}
		t.Logf("%s: %d images, more: %v", out.ProviderID, out.Count, out.HasMore)
	}
	if len(got.Images) < 2 || got.Images[0].Provider == got.Images[1].Provider {
		t.Errorf("results are not interleaved: %d images", len(got.Images))
	}
	t.Logf("%d images in %s", len(got.Images), time.Since(start).Round(time.Millisecond))
}
