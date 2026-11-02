package rotation

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"slices"

	"github.com/sentivanac/kisetsu/internal/library"
	"github.com/sentivanac/kisetsu/internal/settings"
	"github.com/sentivanac/kisetsu/internal/wallpaper"
)

// ErrNoImages means the rotation source has no usable image.
var ErrNoImages = errors.New("no images to rotate")

// maxAttempts bounds how many images are tried when files are missing.
const maxAttempts = 10

// Setter sets the wallpaper; *wallpaper.Setter with Direct set.
type Setter interface {
	Set(ctx context.Context, path string) (wallpaper.Method, error)
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
	Setter   Setter
	Log      *slog.Logger
	Rand     *rand.Rand // nil means a random seed
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
	candidates, err := r.candidates(ctx, cfg.CollectionID)
	if err != nil {
		return Result{}, err
	}
	st, err := r.History.State(ctx)
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
		path, err := r.path(ctx, id)
		if err != nil {
			r.Log.Warn("skipping image for rotation", "entry", id, "err", err)
			candidates = slices.DeleteFunc(candidates, func(c int64) bool { return c == id })
			continue
		}
		method, err := r.Setter.Set(ctx, path)
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

func (r *Rotator) candidates(ctx context.Context, collectionID int64) ([]int64, error) {
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

func (r *Rotator) path(ctx context.Context, id int64) (string, error) {
	e, err := r.Library.Get(ctx, id)
	if err != nil {
		return "", err
	}
	return r.Files.OriginalPath(e.FilePath)
}
