// Package folders reads local folders the user added and keeps their
// images in the library up to date (ADR 0053). The files are used where
// they are: this package only reads them, and never writes, moves or
// deletes anything in a folder.
package folders

import (
	"context"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg" // decoders for DecodeConfig
	_ "image/png"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	_ "golang.org/x/image/webp"

	"github.com/sentivanac/kisetsu/internal/images"
	"github.com/sentivanac/kisetsu/internal/imaging"
	"github.com/sentivanac/kisetsu/internal/library"
)

// Provider is the provider ID of folder images.
const Provider = "folder"

// MaxFiles is the most images read from one folder, subfolders included.
const MaxFiles = 50000

// ErrMissing means the folder does not exist or cannot be read, for
// example a drive that is not connected. Its images are kept until it is
// back.
var ErrMissing = errors.New("folder not found")

// mimeTypes are the formats Kisetsu shows, by file extension.
var mimeTypes = map[string]string{
	".jpg": "image/jpeg", ".jpeg": "image/jpeg", ".png": "image/png", ".webp": "image/webp",
}

// FilePath is the stored path of a folder file: "folders/<id>/<path in
// the folder, with slashes>".
func FilePath(folderID int64, rel string) string {
	return "folders/" + strconv.FormatInt(folderID, 10) + "/" + filepath.ToSlash(rel)
}

// Result says what a sync changed.
type Result struct {
	Added, Updated, Removed int
	// Skipped files look like images but could not be read, are too
	// large, or are past MaxFiles.
	Skipped int
	// Thumbnails Kisetsu made for files that changed or are gone; the
	// caller removes them.
	Thumbnails []string
}

// Sync reads the folder and brings its images in the library up to date.
// Subfolders count; hidden folders (".cache") and symbolic links are
// skipped. Only new and changed files are opened, and only their headers
// are read.
func Sync(ctx context.Context, store *library.Store, f library.Folder) (Result, error) {
	var res Result
	info, err := os.Stat(f.Path)
	if err != nil || !info.IsDir() {
		return res, fmt.Errorf("%w: %s", ErrMissing, f.Path)
	}
	known, err := store.FolderFiles(ctx, f.ID)
	if err != nil {
		return res, err
	}

	var changes library.FolderChanges
	seen := map[string]bool{}
	count := 0
	walkErr := filepath.WalkDir(f.Path, func(p string, d fs.DirEntry, err error) error {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		if err != nil {
			// An unreadable subfolder is skipped, not the whole folder;
			// the root itself was checked above.
			if d != nil && d.IsDir() && p != f.Path {
				return fs.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			if p != f.Path && strings.HasPrefix(d.Name(), ".") {
				return fs.SkipDir
			}
			return nil
		}
		mime, ok := mimeTypes[strings.ToLower(filepath.Ext(d.Name()))]
		if !ok || !d.Type().IsRegular() || strings.HasPrefix(d.Name(), ".") {
			return nil
		}
		if count >= MaxFiles {
			res.Skipped++
			return nil
		}
		count++
		rel, err := filepath.Rel(f.Path, p)
		if err != nil {
			return nil
		}
		stored := FilePath(f.ID, rel)
		fi, err := d.Info()
		if err != nil {
			res.Skipped++
			return nil
		}
		old, exists := known[stored]
		if exists {
			seen[stored] = true
			if old.Size == fi.Size() && old.ModTime == fi.ModTime().UnixNano() {
				return nil
			}
		}
		e, ok := readHeader(p, fi, mime)
		if !ok {
			res.Skipped++
			return nil
		}
		e.FilePath = stored
		e.Image.ProviderID = strings.TrimPrefix(stored, "folders/")
		if exists {
			e.ID = old.ID
			changes.Update = append(changes.Update, e)
			if old.ThumbnailPath != "" {
				res.Thumbnails = append(res.Thumbnails, old.ThumbnailPath)
			}
			return nil
		}
		changes.Add = append(changes.Add, e)
		return nil
	})
	if walkErr != nil {
		return res, walkErr
	}
	for stored, old := range known {
		if !seen[stored] {
			changes.Removed = append(changes.Removed, old.ID)
			if old.ThumbnailPath != "" {
				res.Thumbnails = append(res.Thumbnails, old.ThumbnailPath)
			}
		}
	}
	if changes.Empty() {
		return res, nil
	}
	if err := store.ApplyFolderChanges(ctx, f.ID, changes); err != nil {
		return Result{}, err
	}
	res.Added, res.Updated, res.Removed = len(changes.Add), len(changes.Update), len(changes.Removed)
	return res, nil
}

// readHeader reads a file's size and format from its header. Files that
// are not images, are damaged or too large to show are refused.
func readHeader(p string, fi fs.FileInfo, mime string) (library.Entry, bool) {
	if fi.Size() <= 0 {
		return library.Entry{}, false
	}
	file, err := os.Open(p)
	if err != nil {
		return library.Entry{}, false
	}
	defer file.Close()
	cfg, format, err := image.DecodeConfig(file)
	if err != nil || cfg.Width <= 0 || cfg.Height <= 0 ||
		int64(cfg.Width)*int64(cfg.Height) > imaging.MaxPixels {
		return library.Entry{}, false
	}
	if detected := "image/" + format; detected != mime {
		mime = detected // the content decides, not the extension
	}
	return library.Entry{
		Image: images.Image{
			Provider: Provider,
			Width:    cfg.Width,
			Height:   cfg.Height,
			MIMEType: mime,
			FileSize: fi.Size(),
			License: images.License{
				Status: images.LicenseUnknown,
				Notice: "From a folder on your computer. Kisetsu knows nothing about its author or license.",
			},
		},
		FileModTime: fi.ModTime().UnixNano(),
	}, true
}
