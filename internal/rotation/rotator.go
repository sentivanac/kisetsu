package rotation

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"slices"
	"time"

	"github.com/sentivanac/kisetsu/internal/library"
	"github.com/sentivanac/kisetsu/internal/settings"
	"github.com/sentivanac/kisetsu/internal/wallpaper"
)

// ErrNoImages means the rotation source has no usable image.
var ErrNoImages = errors.New("no images to rotate")

const (
	// maxAttempts bounds how many images are tried when files are missing.
	maxAttempts = 10
	// lowWater: with fewer unshown fetched images, new ones are fetched
	// before choosing.
	lowWater = 3
	// topUpCount is how many images one top-up fetches.
	topUpCount = 5
	// topUpTimeout bounds a top-up: a few searches and downloads.
	topUpTimeout = 3 * time.Minute
)

// TopUp fetches up to want new images for a saved search, never removing
// the images in protect, and returns how many it fetched. Package fetched
// provides it; rotation only knows this function.
type TopUp func(ctx context.Context, savedSearchID int64, want int, protect []int64) (int, error)

// Screen shows a library image as the wallpaper; *onscreen.Service, whose
// setter has Direct set.
type Screen interface {
	Show(ctx context.Context, e library.Entry, path string) (wallpaper.Method, error)
}

// Files resolves library paths; *libfiles.Server.
type Files interface {
	OriginalPath(relPath string) (string, error)
}

// Rotator chooses and sets the next wallpaper.
type Rotator struct {
	Library  *library.Store
	Settings *settings.Store
	History  *History
	Files    Files
	Screen   Screen
	Log      *slog.Logger
	Rand     *rand.Rand // nil means a random seed
	// TopUp, if set, fetches new images when a saved search runs low.
	TopUp TopUp
}

// Result describes a wallpaper change.
type Result struct {
	ImageID int64
	Method  wallpaper.Method
}

// Next sets the next image from the configured source. It works whether
// or not rotation is enabled, so the UI can offer "Next wallpaper".
//
// It is meant to run unattended, so it is forgiving: a collection that no
// longer exists falls back to the whole library, and images whose file is
// missing are skipped.
func (r *Rotator) Next(ctx context.Context) (Result, error) {
	cfg, err := LoadSettings(ctx, r.Settings)
	if err != nil {
		return Result{}, err
	}
	st, err := r.History.State(ctx)
	if err != nil {
		return Result{}, err
	}
	candidates, err := r.candidates(ctx, cfg, st)
	if err != nil {
		return Result{}, err
	}
	rng := r.Rand
	if rng == nil {
		rng = rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64()))
	}

	for range maxAttempts {
		id, cycle, ok := Pick(candidates, st, cfg.Order, rng)
		if !ok {
			return Result{}, ErrNoImages
		}
		e, path, err := r.path(ctx, id)
		if err != nil {
			r.Log.Warn("skipping image for rotation", "entry", id, "err", err)
			candidates = slices.DeleteFunc(candidates, func(c int64) bool { return c == id })
			continue
		}
		method, err := r.Screen.Show(ctx, e, path)
		if err != nil {
			return Result{}, err
		}
		if err := r.History.Record(ctx, id, cycle, ByRotation); err != nil {
			return Result{}, err
		}
		r.Log.Info("wallpaper rotated", "entry", id, "cycle", cycle, "order", cfg.Order, "method", method)
		return Result{ImageID: id, Method: method}, nil
	}
	return Result{}, fmt.Errorf("%w: %d images had missing files", ErrNoImages, maxAttempts)
}

// RecordManual notes a wallpaper the user set, so rotation knows what is on
// screen. It joins the current cycle without counting as shown in it.
func (r *Rotator) RecordManual(ctx context.Context, imageID int64) error {
	st, err := r.History.State(ctx)
	if err != nil {
		return err
	}
	return r.History.Record(ctx, imageID, st.Cycle, ByUser)
}

// candidates returns the images to choose from. For a saved search they
// are its fetched images, topped up first when few are left unshown.
func (r *Rotator) candidates(ctx context.Context, cfg Settings, st State) ([]int64, error) {
	if cfg.SavedSearchID == 0 {
		return r.libraryCandidates(ctx, cfg.CollectionID)
	}
	exists, err := r.Library.HasSavedSearch(ctx, cfg.SavedSearchID)
	if err != nil {
		return nil, err
	}
	if !exists {
		r.Log.Warn("rotation saved search no longer exists; using the whole library", "saved_search", cfg.SavedSearchID)
		return r.libraryCandidates(ctx, 0)
	}

	ids, err := r.Library.FetchedIDs(ctx, cfg.SavedSearchID)
	if err != nil {
		return nil, err
	}
	if r.TopUp != nil && len(unshown(ids, st.Shown)) < lowWater {
		tctx, cancel := context.WithTimeout(ctx, topUpTimeout)
		n, err := r.TopUp(tctx, cfg.SavedSearchID, topUpCount, []int64{st.Current})
		cancel()
		if err != nil {
			// Offline or a provider problem: rotate what is there.
			r.Log.Warn("could not fetch new images; using the fetched ones", "saved_search", cfg.SavedSearchID, "err", err)
		}
		if n > 0 {
			if ids, err = r.Library.FetchedIDs(ctx, cfg.SavedSearchID); err != nil {
				return nil, err
			}
		}
	}
	return ids, nil
}

func (r *Rotator) libraryCandidates(ctx context.Context, collectionID int64) ([]int64, error) {
	if collectionID != 0 {
		exists, err := r.Library.HasCollection(ctx, collectionID)
		if err != nil {
			return nil, err
		}
		if !exists {
			r.Log.Warn("rotation collection no longer exists; using the whole library", "collection", collectionID)
			collectionID = 0
		}
	}
	return r.Library.IDs(ctx, collectionID)
}

func (r *Rotator) path(ctx context.Context, id int64) (library.Entry, string, error) {
	e, err := r.Library.Get(ctx, id)
	if err != nil {
		return library.Entry{}, "", err
	}
	path, err := r.Files.OriginalPath(e.FilePath)
	return e, path, err
}
