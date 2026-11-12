package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/sentivanac/kisetsu/internal/apperr"
	"github.com/sentivanac/kisetsu/internal/folders"
	"github.com/sentivanac/kisetsu/internal/library"
)

// FolderInfo describes a local folder for the frontend (ADR 0053).
type FolderInfo struct {
	ID     int64  `json:"id"`
	Path   string `json:"path"`
	Name   string `json:"name"` // the last part of the path
	Images int    `json:"images"`
	// Missing is set when the folder cannot be read now, for example a
	// drive that is not connected.
	Missing bool `json:"missing"`
}

// FolderSync says what reading a folder changed.
type FolderSync struct {
	Folder  FolderInfo `json:"folder"`
	Added   int        `json:"added"`
	Updated int        `json:"updated"`
	Removed int        `json:"removed"`
	Skipped int        `json:"skipped"`
}

// ListFolders returns the local folders, in the order they were added.
func (a *App) ListFolders() (list []FolderInfo, err error) {
	defer a.recoverPanic("ListFolders", &err)
	fs, err := a.library.Folders(a.ctx)
	if err != nil {
		return nil, apperr.New(apperr.Unavailable, "The folders could not be loaded.", err)
	}
	counts, err := a.library.FolderCounts(a.ctx)
	if err != nil {
		return nil, apperr.New(apperr.Unavailable, "The folders could not be loaded.", err)
	}
	list = make([]FolderInfo, 0, len(fs))
	for _, f := range fs {
		list = append(list, folderInfo(f, counts[f.ID]))
	}
	return list, nil
}

func folderInfo(f library.Folder, images int) FolderInfo {
	info, err := os.Stat(f.Path)
	return FolderInfo{
		ID: f.ID, Path: f.Path, Name: filepath.Base(f.Path), Images: images,
		Missing: err != nil || !info.IsDir(),
	}
}

// AddFolder asks for a folder in the system's dialog, adds it and reads
// it. It returns an empty result when the dialog was closed.
func (a *App) AddFolder() (result FolderSync, err error) {
	defer a.recoverPanic("AddFolder", &err)
	chosen, err := runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Choose a folder with images",
	})
	if err != nil {
		return FolderSync{}, apperr.New(apperr.Unavailable, "The folder dialog could not be opened.", err)
	}
	if chosen == "" {
		return FolderSync{}, nil
	}
	path, err := folders.Check(chosen, a.ownDirs)
	switch {
	case errors.Is(err, folders.ErrOwnDirectory):
		return FolderSync{}, apperr.New(apperr.InvalidInput,
			"This folder belongs to Kisetsu; its images are in your library already.", err)
	case err != nil:
		return FolderSync{}, apperr.New(apperr.InvalidInput, "This folder cannot be read.", err)
	}
	f, err := a.library.AddFolder(a.ctx, path)
	if errors.Is(err, library.ErrFolderExists) {
		return FolderSync{}, apperr.New(apperr.InvalidInput, "This folder was added already.", err)
	}
	if err != nil {
		return FolderSync{}, apperr.New(apperr.Unavailable, "The folder could not be added.", err)
	}
	a.log.Info("folder added", "folder", f.ID, "path", f.Path)
	return a.syncFolder(f)
}

// SyncFolder reads a folder again: new, changed and removed files.
func (a *App) SyncFolder(id int64) (result FolderSync, err error) {
	defer a.recoverPanic("SyncFolder", &err)
	f, err := a.library.Folder(a.ctx, id)
	if errors.Is(err, library.ErrNotFound) {
		return FolderSync{}, apperr.New(apperr.InvalidInput, "This folder is no longer in Kisetsu.", err)
	}
	if err != nil {
		return FolderSync{}, apperr.New(apperr.Unavailable, "The folder could not be read.", err)
	}
	return a.syncFolder(f)
}

func (a *App) syncFolder(f library.Folder) (FolderSync, error) {
	res, err := syncFolder(a.ctx, a.library, a.files, f, a.ownDirs, a.log)
	counts, cerr := a.library.FolderCounts(a.ctx)
	if cerr != nil {
		return FolderSync{}, apperr.New(apperr.Unavailable, "The folder could not be read.", cerr)
	}
	out := FolderSync{Folder: folderInfo(f, counts[f.ID]),
		Added: res.Added, Updated: res.Updated, Removed: res.Removed, Skipped: res.Skipped}
	if errors.Is(err, folders.ErrMissing) {
		// Its images stay; the view says the folder is not there.
		return out, nil
	}
	if err != nil {
		return FolderSync{}, apperr.New(apperr.Unavailable, "The folder could not be read.", err)
	}
	return out, nil
}

// thumbnailRemover removes thumbnails Kisetsu made (libfiles).
type thumbnailRemover interface {
	Remove(relPath string) error
}

// syncFolder reads a folder and removes the thumbnails of changed and
// removed files; shared by the window and rotation.
func syncFolder(ctx context.Context, store *library.Store, files thumbnailRemover, f library.Folder, own []string, log interface {
	Info(string, ...any)
	Warn(string, ...any)
}) (folders.Result, error) {
	res, err := folders.Sync(ctx, store, f, own...)
	for _, thumb := range res.Thumbnails {
		if rerr := files.Remove(thumb); rerr != nil {
			log.Warn("old folder thumbnail not removed", "path", thumb, "err", rerr)
		}
	}
	if err != nil {
		return res, err
	}
	if res.Added+res.Updated+res.Removed > 0 {
		log.Info("folder read", "folder", f.ID, "added", res.Added, "updated", res.Updated,
			"removed", res.Removed, "skipped", res.Skipped)
	}
	return res, nil
}

// ListFolder returns a folder's images, newest first.
func (a *App) ListFolder(id int64, offset, limit int) (page LibraryPage, err error) {
	defer a.recoverPanic("ListFolder", &err)
	if id <= 0 || offset < 0 || limit < 1 || limit > maxLibraryPage {
		return LibraryPage{}, apperr.New(apperr.InvalidInput, "This part of the folder cannot be shown.", nil)
	}
	entries, err := a.library.List(a.ctx, library.ListOptions{
		Origin: library.FromFolder, FolderID: id, Limit: limit + 1, Offset: offset,
	})
	if err != nil {
		return LibraryPage{}, apperr.New(apperr.Unavailable, "The folder could not be loaded.", err)
	}
	page.HasMore = len(entries) > limit
	entries = entries[:min(len(entries), limit)]
	page.Items = make([]LibraryItem, len(entries))
	for i, e := range entries {
		page.Items[i] = libraryItem(e)
	}
	return page, nil
}

// RemoveFolder forgets a folder and its images in Kisetsu (collections,
// history). The files in the folder are not touched.
func (a *App) RemoveFolder(id int64) (err error) {
	defer a.recoverPanic("RemoveFolder", &err)
	thumbs, err := a.library.RemoveFolder(a.ctx, id)
	if errors.Is(err, library.ErrNotFound) {
		return nil
	}
	if err != nil {
		return apperr.New(apperr.Unavailable, "The folder could not be removed.", err)
	}
	a.files.ForgetFolder(id)
	for _, thumb := range thumbs {
		if err := a.files.Remove(thumb); err != nil {
			a.log.Warn("folder thumbnail not removed", "path", thumb, "err", err)
		}
	}
	a.log.Info("folder removed", "folder", id)
	return nil
}
