package main

import (
	"context"
	"errors"
	"net"
	"strings"

	"github.com/sentivanac/kisetsu/internal/apperr"
	"github.com/sentivanac/kisetsu/internal/downloads"
	"github.com/sentivanac/kisetsu/internal/images"
	"github.com/sentivanac/kisetsu/internal/imaging"
	"github.com/sentivanac/kisetsu/internal/library"
)

// DownloadProgressEvent is the Wails event that carries DownloadProgress.
const DownloadProgressEvent = "download:progress"

// AlreadyInLibrary is the apperr code for images that were downloaded before.
const AlreadyInLibrary apperr.Code = "already_in_library"

// DownloadProgress is sent to the frontend for every download update.
type DownloadProgress struct {
	Key        string         `json:"key"` // provider "/" provider ID
	State      string         `json:"state"`
	Received   int64          `json:"received"`
	Total      int64          `json:"total"`
	EntryID    int64          `json:"entryId,omitempty"`
	SimilarIDs []int64        `json:"similarIds,omitempty"`
	Error      *apperr.Public `json:"error,omitempty"`
}

// Download starts downloading an image into the library and returns its
// key. Progress arrives as DownloadProgressEvent events.
func (a *App) Download(img images.Image) (key string, err error) {
	defer a.recoverPanic("Download", &err)

	key, err = a.downloads.Enqueue(a.ctx, img)
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
