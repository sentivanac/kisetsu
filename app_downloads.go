package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net"
	"os"
	"strings"

	"github.com/sentivanac/kisetsu/internal/apperr"
	"github.com/sentivanac/kisetsu/internal/downloads"
	"github.com/sentivanac/kisetsu/internal/images"
	"github.com/sentivanac/kisetsu/internal/imaging"
	"github.com/sentivanac/kisetsu/internal/library"
	"github.com/sentivanac/kisetsu/internal/settings"
)

// DownloadProgressEvent is the Wails event that carries DownloadProgress.
const DownloadProgressEvent = "download:progress"

// AlreadyInLibrary is the apperr code for images that were downloaded before.
const AlreadyInLibrary apperr.Code = "already_in_library"

// DownloadProgress is sent to the frontend for every download update.
type DownloadProgress struct {
	Key        string  `json:"key"` // provider "/" provider ID
	State      string  `json:"state"`
	Received   int64   `json:"received"`
	Total      int64   `json:"total"`
	EntryID    int64   `json:"entryId,omitempty"`
	SimilarIDs []int64 `json:"similarIds,omitempty"`
	// FetchedID is a fetched copy that KeepFetched can keep instead.
	FetchedID int64               `json:"fetchedId,omitempty"`
	Error     *apperr.PublicError `json:"error,omitempty"`
}

// DownloadSettings is what downloads store.
type DownloadSettings struct {
	Size string `json:"size"` // "original" or "screen"
	// ScreenWidth and ScreenHeight are what "screen" covers.
	ScreenWidth  int `json:"screenWidth"`
	ScreenHeight int `json:"screenHeight"`
}

// GetDownloadSettings returns what downloads store.
func (a *App) GetDownloadSettings() (s DownloadSettings, err error) {
	defer a.recoverPanic("GetDownloadSettings", &err)

	size, err := a.settings.DownloadSize(a.ctx)
	if err != nil {
		return DownloadSettings{}, apperr.New(apperr.Unavailable, "The download settings could not be loaded.", err)
	}
	screen, err := a.settings.Display(a.ctx)
	if err != nil {
		return DownloadSettings{}, apperr.New(apperr.Unavailable, "The download settings could not be loaded.", err)
	}
	return DownloadSettings{Size: string(size), ScreenWidth: screen.Width, ScreenHeight: screen.Height}, nil
}

// SaveDownloadSize chooses what downloads store. Stored images do not change.
func (a *App) SaveDownloadSize(size string) (err error) {
	defer a.recoverPanic("SaveDownloadSize", &err)

	d := settings.DownloadSize(size)
	if !d.Valid() {
		return apperr.New(apperr.InvalidInput, "This download size is not known.", nil)
	}
	if err := a.settings.SaveDownloadSize(a.ctx, d); err != nil {
		return apperr.New(apperr.Unavailable, "The download size could not be saved.", err)
	}
	return nil
}

// Download starts downloading an image into the library, in the chosen
// size, and returns its key. Progress arrives as DownloadProgressEvent
// events.
func (a *App) Download(img images.Image) (key string, err error) {
	defer a.recoverPanic("Download", &err)

	size, err := a.settings.DownloadSize(a.ctx)
	if err != nil {
		a.log.Warn("download size not read; storing the original", "err", err)
		size = settings.DownloadOriginal
	}
	return a.download(img, size)
}

// DownloadAs is Download in the given size, for this image only.
func (a *App) DownloadAs(img images.Image, size string) (key string, err error) {
	defer a.recoverPanic("DownloadAs", &err)

	d := settings.DownloadSize(size)
	if !d.Valid() {
		return "", apperr.New(apperr.InvalidInput, "This download size is not known.", nil)
	}
	return a.download(img, d)
}

func (a *App) download(img images.Image, size settings.DownloadSize) (key string, err error) {
	if size == settings.DownloadOriginal {
		if key, ok := a.keepFetched(img, imaging.Size{}); ok {
			return key, nil
		}
		key, err = a.downloads.Enqueue(a.ctx, img)
	} else {
		screen, serr := a.settings.Display(a.ctx)
		if serr != nil {
			return "", apperr.New(apperr.Unavailable, "The download could not be started.", serr)
		}
		fit := imaging.Size{Width: screen.Width, Height: screen.Height}
		if key, ok := a.keepFetched(img, fit); ok {
			return key, nil
		}
		key, err = a.downloads.EnqueueFitted(a.ctx, img, fit)
	}
	switch {
	case errors.Is(err, library.ErrAlreadyInLibrary):
		return "", apperr.New(AlreadyInLibrary, "This image is already in your library.", err)
	case err != nil && img.Validate() != nil:
		return "", apperr.New(apperr.InvalidInput, "This image cannot be downloaded.", err)
	case err != nil:
		return "", apperr.New(apperr.Unavailable, "The download could not be started.", err)
	}
	return key, nil
}

// DownloadFetched is DownloadAs for a fetched image.
func (a *App) DownloadFetched(id int64, size string) (key string, err error) {
	defer a.recoverPanic("DownloadFetched", &err)

	d := settings.DownloadSize(size)
	if !d.Valid() {
		return "", apperr.New(apperr.InvalidInput, "This download size is not known.", nil)
	}
	e, err := a.library.Get(a.ctx, id)
	if errors.Is(err, library.ErrNotFound) {
		return "", apperr.New(apperr.InvalidInput, "This image no longer exists.", err)
	}
	if err != nil {
		return "", apperr.New(apperr.Unavailable, "The download could not be started.", err)
	}
	var fit imaging.Size
	if d == settings.DownloadScreen {
		screen, err := a.settings.Display(a.ctx)
		if err != nil {
			return "", apperr.New(apperr.Unavailable, "The download could not be started.", err)
		}
		fit = imaging.Size{Width: screen.Width, Height: screen.Height}
	}
	if key, ok := a.keepFetched(e.Image, fit); ok {
		return key, nil
	}
	key, err = a.downloads.EnqueueEntry(a.ctx, id, fit)
	if errors.Is(err, library.ErrAlreadyInLibrary) {
		return "", apperr.New(AlreadyInLibrary, "This image is already in your library.", err)
	}
	if err != nil {
		return "", apperr.New(apperr.Unavailable, "The download could not be started.", err)
	}
	return key, nil
}

// keepFetched keeps the fetched copy of img if it is as good as the
// download would be, so nothing is downloaded again: the original, or with
// fit a copy that covers it. It reports false when there is none, or
// keeping it failed.
func (a *App) keepFetched(img images.Image, fit imaging.Size) (string, bool) {
	e, found, err := a.library.FindBySource(a.ctx, img.Provider, img.ProviderID)
	if err != nil || !found || e.Origin != library.Fetched {
		return "", false
	}
	covers := fit != (imaging.Size{}) && e.Image.Width >= fit.Width && e.Image.Height >= fit.Height
	if !covers && a.resolveStored(&e) != library.StoredOriginal {
		return "", false
	}
	if _, err := a.fetched.Keep(a.ctx, e.ID); err != nil {
		a.log.Warn("fetched copy not kept; downloading instead", "entry", e.ID, "err", err)
		return "", false
	}
	key := downloads.Key(img)
	a.notifyDownload(downloads.Progress{Key: key, State: downloads.Done, EntryID: e.ID})
	return key, true
}

// resolveStored returns which version e's file is. For an image stored
// before 0.9.0 it reads the file; an original is then recorded, also in e.
// A reduced one is not: the size of its original is not known.
func (a *App) resolveStored(e *library.Entry) library.Stored {
	if e.Stored() != library.StoredUnknown {
		return e.Stored()
	}
	original, err := a.isOriginal(*e)
	if err != nil {
		a.log.Warn("cannot tell whether the file is the original", "entry", e.ID, "err", err)
		return library.StoredUnknown
	}
	if !original {
		return library.StoredReduced
	}
	if err := a.library.MarkOriginal(a.ctx, e.ID); err != nil {
		a.log.Warn("original not recorded", "entry", e.ID, "err", err)
	}
	e.OriginalWidth, e.OriginalHeight = e.Image.Width, e.Image.Height
	return library.StoredOriginal
}

// CheckOriginal returns a stored image after finding out, if that is not
// known yet, whether its file is the original.
func (a *App) CheckOriginal(id int64) (item LibraryItem, err error) {
	defer a.recoverPanic("CheckOriginal", &err)

	e, err := a.library.Get(a.ctx, id)
	if errors.Is(err, library.ErrNotFound) {
		return LibraryItem{}, apperr.New(apperr.InvalidInput, "This image no longer exists.", err)
	}
	if err != nil {
		return LibraryItem{}, apperr.New(apperr.Unavailable, "The image could not be loaded.", err)
	}
	stored := a.resolveStored(&e)
	items := []LibraryItem{libraryItem(e)}
	items[0].Stored = string(stored)
	a.markHidden(items)
	return items[0], nil
}

// GetOriginal replaces the stored copy of a library image by its original.
// It returns the download's key, or "" when the file already is the
// original.
func (a *App) GetOriginal(id int64) (key string, err error) {
	defer a.recoverPanic("GetOriginal", &err)

	e, err := a.library.Get(a.ctx, id)
	if errors.Is(err, library.ErrNotFound) {
		return "", apperr.New(apperr.InvalidInput, "This image is no longer in your library.", err)
	}
	if err != nil {
		return "", apperr.New(apperr.Unavailable, "The download could not be started.", err)
	}
	if e.Origin == library.FromFolder {
		return "", apperr.New(apperr.InvalidInput, "This image is in one of your folders; it has no other original.", nil)
	}
	if a.resolveStored(&e) == library.StoredOriginal {
		return "", nil
	}
	key, err = a.downloads.EnqueueOriginal(a.ctx, id)
	if err != nil {
		return "", apperr.New(apperr.Unavailable, "The download could not be started.", err)
	}
	return key, nil
}

// isOriginal reports whether the stored file is the downloaded one: its
// SHA-256 is that of the original, also when a reduced copy is stored.
func (a *App) isOriginal(e library.Entry) (bool, error) {
	path, err := a.files.OriginalPath(e.FilePath)
	if err != nil {
		return false, err
	}
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return false, err
	}
	return hex.EncodeToString(h.Sum(nil)) == e.SHA256, nil
}

// LibraryStatus returns, for each key ("provider/providerId") of an image
// that is already in the library, its library entry ID.
func (a *App) LibraryStatus(keys []string) (ids map[string]int64, err error) {
	defer a.recoverPanic("LibraryStatus", &err)

	refs := make([]library.SourceRef, 0, len(keys))
	for _, key := range keys {
		// Provider IDs never contain "/", but image IDs might.
		provider, id, ok := strings.Cut(key, "/")
		if ok {
			refs = append(refs, library.SourceRef{Provider: provider, ProviderID: id})
		}
	}
	found, err := a.library.IDsBySource(a.ctx, refs)
	if err != nil {
		return nil, apperr.New(apperr.Unavailable, "The library could not be checked.", err)
	}
	ids = make(map[string]int64, len(found))
	for ref, id := range found {
		ids[ref.Provider+"/"+ref.ProviderID] = id
	}
	return ids, nil
}

// CancelDownload stops a running download.
func (a *App) CancelDownload(key string) (err error) {
	defer a.recoverPanic("CancelDownload", &err)

	a.downloads.Cancel(key)
	return nil
}

// notifyDownload forwards download progress to the frontend.
func (a *App) notifyDownload(p downloads.Progress) {
	event := DownloadProgress{
		Key:        p.Key,
		State:      string(p.State),
		Received:   p.Received,
		Total:      p.Total,
		EntryID:    p.EntryID,
		SimilarIDs: p.SimilarIDs,
		FetchedID:  p.FetchedID,
	}
	if p.Err != nil {
		public := apperr.ToPublic(downloadError(p.Err))
		event.Error = &public
	}
	a.emit(DownloadProgressEvent, event)
}

// downloadError turns a download failure into a message for the user.
func downloadError(err error) error {
	var netErr net.Error
	switch {
	case errors.Is(err, downloads.ErrChanged):
		return apperr.New(apperr.Unavailable,
			"The source now has a different or smaller image, so your copy was kept.", err)
	case errors.Is(err, downloads.ErrFileTooLarge):
		return apperr.New(apperr.InvalidInput, "The file is larger than 100 MB.", err)
	case errors.Is(err, imaging.ErrUnsupportedFormat):
		return apperr.New(apperr.InvalidInput, "The file is not a JPEG, PNG or WebP image.", err)
	case errors.Is(err, imaging.ErrCorrupt):
		return apperr.New(apperr.InvalidInput, "The downloaded file is damaged.", err)
	case errors.Is(err, imaging.ErrTooLarge):
		return apperr.New(apperr.InvalidInput, "The image has too many pixels to process.", err)
	case errors.Is(err, downloads.ErrHTTP):
		return apperr.New(apperr.ProviderUnavailable, "The source did not provide the file. Please try again later.", err)
	case errors.Is(err, context.DeadlineExceeded), errors.As(err, &netErr):
		return apperr.New(apperr.ProviderUnavailable,
			"The image could not be downloaded. Check your internet connection and try again.", err)
	}
	return apperr.InternalError(err)
}
