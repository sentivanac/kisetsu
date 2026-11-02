package fetched

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"slices"

	"github.com/sentivanac/kisetsu/internal/downloads"
	"github.com/sentivanac/kisetsu/internal/images"
	"github.com/sentivanac/kisetsu/internal/imaging"
	"github.com/sentivanac/kisetsu/internal/providers"
	"github.com/sentivanac/kisetsu/internal/search"
	"github.com/sentivanac/kisetsu/internal/searches"
	"github.com/sentivanac/kisetsu/internal/settings"
)

// maxRounds bounds the searches of one Fetch: each round asks every source
// for one page, so one Fetch never uses many API calls.
const maxRounds = 3

// Searcher runs multi-source searches; *search.Service.
type Searcher interface {
	SearchMany(ctx context.Context, sources []search.SourcePage, q providers.Query) (search.MultiResult, error)
	Providers() []search.ProviderInfo
}

// Downloader fetches one image and waits; *downloads.Manager.
type Downloader interface {
	FetchNow(ctx context.Context, img images.Image, opts downloads.FetchOptions) downloads.Progress
}

// Fetcher downloads new images for saved searches.
type Fetcher struct {
	DB         *sql.DB
	Searches   *searches.Store
	Settings   *settings.Store
	Search     Searcher
	Downloader Downloader
	Service    *Service // prunes after fetching
	Log        *slog.Logger
}

// Result describes one Fetch.
type Result struct {
	Fetched int     // new images stored
	IDs     []int64 // their entries
	Pruned  int     // old fetched images removed to stay within the limit
}

// Fetch downloads up to want new images for a saved search, continuing
// where the previous Fetch stopped for each source. Images already in the
// database are skipped. Afterwards old fetched images are pruned, except
// those in protect (for example the wallpaper on screen).
//
// Network problems are not fatal: sources that fail are logged and the
// images fetched so far are kept.
func (f *Fetcher) Fetch(ctx context.Context, savedSearchID int64, want int, protect ...int64) (Result, error) {
	saved, err := f.Searches.Get(ctx, savedSearchID)
	if err != nil {
		return Result{}, err
	}
	sources := f.availableSources(saved.Query.ProviderIDs)
	if len(sources) == 0 {
		return Result{}, fmt.Errorf("saved search %d: none of its sources is available", savedSearchID)
	}
	display, err := f.Settings.Display(ctx)
	if err != nil {
		return Result{}, err
	}
	opts := downloads.FetchOptions{
		SavedSearchID: savedSearchID,
		Fit:           imaging.Size{Width: display.Width, Height: display.Height},
	}

	var res Result
	for round := 0; round < maxRounds && res.Fetched < want; round++ {
		pages, err := f.cursors(ctx, savedSearchID, sources)
		if err != nil {
			return res, err
		}
		multi, err := f.Search.SearchMany(ctx, pages, saved.Query.Provider(1))
		if err != nil {
			return res, err
		}

		for _, img := range multi.Images {
			if res.Fetched >= want || ctx.Err() != nil {
				break
			}
			p := f.Downloader.FetchNow(ctx, img, opts)
			switch p.State {
			case downloads.Done:
				res.Fetched++
				res.IDs = append(res.IDs, p.EntryID)
			case downloads.Failed:
				f.Log.Warn("fetch: image skipped", "key", p.Key, "err", p.Err)
			}
		}

		progressed := false
		for _, out := range multi.Sources {
			if out.Err != nil {
				f.Log.Warn("fetch: source failed", "saved_search", savedSearchID, "provider", out.ProviderID, "err", out.Err)
				continue
			}
			next := out.Page + 1
			if !out.HasMore {
				next = 1 // start over; old images may have been pruned since
			}
			if err := f.setCursor(ctx, savedSearchID, out.ProviderID, next); err != nil {
				return res, err
			}
			progressed = progressed || out.Count > 0
		}
		if !progressed {
			break // every source failed or returned nothing
		}
	}

	if res.Fetched > 0 {
		pruned, _, err := f.Service.Prune(ctx, append(slices.Clone(protect), res.IDs...)...)
		if err != nil {
			return res, err
		}
		res.Pruned = pruned
	}
	f.Log.Info("fetch done", "saved_search", savedSearchID, "fetched", res.Fetched, "pruned", res.Pruned)
	return res, nil
}

// availableSources keeps the saved search's sources that are registered.
func (f *Fetcher) availableSources(ids []string) []string {
	known := map[string]bool{}
	for _, p := range f.Search.Providers() {
		known[p.ID] = true
	}
	var out []string
	for _, id := range ids {
		if known[id] {
			out = append(out, id)
		}
	}
	return out
}

func (f *Fetcher) cursors(ctx context.Context, savedSearchID int64, sources []string) ([]search.SourcePage, error) {
	pages := make([]search.SourcePage, len(sources))
	for i, provider := range sources {
		page := 1
		err := f.DB.QueryRowContext(ctx,
			`SELECT next_page FROM fetch_cursors WHERE saved_search_id = ? AND provider = ?`,
			savedSearchID, provider).Scan(&page)
		if err != nil && err != sql.ErrNoRows {
			return nil, fmt.Errorf("read fetch cursor: %w", err)
		}
		pages[i] = search.SourcePage{ProviderID: provider, Page: page}
	}
	return pages, nil
}

func (f *Fetcher) setCursor(ctx context.Context, savedSearchID int64, provider string, next int) error {
	_, err := f.DB.ExecContext(ctx, `
		INSERT INTO fetch_cursors (saved_search_id, provider, next_page) VALUES (?, ?, ?)
		ON CONFLICT (saved_search_id, provider) DO UPDATE SET next_page = excluded.next_page`,
		savedSearchID, provider, next)
	if err != nil {
		return fmt.Errorf("save fetch cursor: %w", err)
	}
	return nil
}
