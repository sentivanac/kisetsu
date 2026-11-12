package main

import (
	"errors"
	"time"

	"github.com/sentivanac/kisetsu/internal/apperr"
	"github.com/sentivanac/kisetsu/internal/folders"
	"github.com/sentivanac/kisetsu/internal/images"
	"github.com/sentivanac/kisetsu/internal/libfiles"
	"github.com/sentivanac/kisetsu/internal/library"
)

// maxLibraryPage limits how many entries one ListLibrary call returns.
const maxLibraryPage = 200

// LibraryItem is one downloaded image for the frontend.
type LibraryItem struct {
	ID int64 `json:"id"`
	// Image keeps the provider metadata and attribution. Its ThumbnailURL
	// and PreviewURL point at the local thumbnail.
	Image   images.Image `json:"image"`
	FileURL string       `json:"fileUrl"` // the local original
	AddedAt string       `json:"addedAt"` // RFC 3339
}

// LibraryPage is one page of the library.
type LibraryPage struct {
	Items   []LibraryItem `json:"items"`
	HasMore bool          `json:"hasMore"`
}

// LibraryStats summarizes the library for the frontend.
type LibraryStats struct {
	Images int   `json:"images"`
	Bytes  int64 `json:"bytes"`
}

// ListLibrary returns downloaded images, newest first. A collectionID of 0
// lists the whole library; otherwise only that collection.
func (a *App) ListLibrary(collectionID int64, offset, limit int) (page LibraryPage, err error) {
	defer a.recoverPanic("ListLibrary", &err)

	if collectionID < 0 || offset < 0 || limit < 1 || limit > maxLibraryPage {
		return LibraryPage{}, apperr.New(apperr.InvalidInput, "This part of the library cannot be shown.", nil)
	}
	// One extra entry tells whether another page exists.
	entries, err := a.library.List(a.ctx, library.ListOptions{
		CollectionID: collectionID, Limit: limit + 1, Offset: offset,
	})
	if err != nil {
		return LibraryPage{}, apperr.New(apperr.Unavailable, "The library could not be loaded.", err)
	}

	page.HasMore = len(entries) > limit
	entries = entries[:min(len(entries), limit)]
	page.Items = make([]LibraryItem, len(entries))
	for i, e := range entries {
		page.Items[i] = libraryItem(e)
	}
	return page, nil
}

func libraryItem(e library.Entry) LibraryItem {
	img := e.Image
	thumbPath := e.ThumbnailPath
	if thumbPath == "" && e.Origin == library.FromFolder {
		// Made when the window first asks for it (ADR 0053).
		thumbPath = folders.ThumbnailPath(e)
	}
	thumb := libfiles.URL(thumbPath)
	img.ThumbnailURL, img.PreviewURL = thumb, thumb
	return LibraryItem{
		ID:      e.ID,
		Image:   img,
		FileURL: libfiles.URL(e.FilePath),
		AddedAt: e.AddedAt.Format(time.RFC3339),
	}
}

// GetLibraryStats returns the number of images and their total size.
func (a *App) GetLibraryStats() (stats LibraryStats, err error) {
	defer a.recoverPanic("GetLibraryStats", &err)

	st, err := a.library.Stats(a.ctx)
	if err != nil {
		return LibraryStats{}, apperr.New(apperr.Unavailable, "The library could not be loaded.", err)
	}
	return LibraryStats{Images: st.Images, Bytes: st.Bytes}, nil
}

// DeleteFromLibrary removes an image from the library, including its file
// and thumbnail. The record is removed first: if a file cannot be removed
// afterwards, only an unused file is left, never a record pointing at a
// missing file.
func (a *App) DeleteFromLibrary(id int64) (err error) {
	defer a.recoverPanic("DeleteFromLibrary", &err)

	e, err := a.library.Get(a.ctx, id)
	if errors.Is(err, library.ErrNotFound) {
		return apperr.New(apperr.InvalidInput, "This image is no longer in your library.", err)
	}
	if err != nil {
		return apperr.New(apperr.Unavailable, "The image could not be deleted.", err)
	}
	if err := a.library.Delete(a.ctx, id); err != nil {
		return apperr.New(apperr.Unavailable, "The image could not be deleted.", err)
	}

	for _, rel := range []string{e.FilePath, e.ThumbnailPath} {
		if rel == "" {
			continue
		}
		if err := a.files.Remove(rel); err != nil {
			a.log.Warn("library file not removed", "entry", id, "path", rel, "err", err)
		}
	}
	a.log.Info("image deleted from library", "entry", id)
	return nil
}
