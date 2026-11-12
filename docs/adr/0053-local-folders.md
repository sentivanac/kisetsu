# ADR 0053: Local folders

- **Status:** accepted, for 0.7.0
- **Date:** 2026-11-12

## Context

A wallpaper manager is expected to use images the user already has, as
Variety does with its folders. Kisetsu's library only knew images it had
downloaded itself, stored in its own directory, and the window can show
files only from there (a guard of `libfiles`).

The user chose (2026-11-12): folders are used **in place**, never copied,
with **subfolders**.

## Decision

### The data

- Table `folders`: the absolute path of every folder the user added.
- A folder's images are rows of `images` with origin `folder`, the
  folder's ID and the file's modification time. Collections, history,
  "Set as wallpaper", rotation and the text on the wallpaper treat them
  as any image. The user's library and the fetched images exclude them,
  as they exclude each other.
- Stored path: `folders/<folder id>/<path in the folder>`; provider
  `folder`. No author, license unknown, with a notice that the file is
  from the user's own folder.
- **No SHA-256.** Hashing a large folder would read gigabytes the first
  time, and a picture may be in the library and in a folder. Folder
  images also have no difference hash, and duplicate detection leaves
  them out.

Migration 0009 rebuilds `images`, because SQLite cannot change a CHECK
(the origins) or make a `UNIQUE NOT NULL` column optional in place. The
runner gained migrations marked `-- kisetsu: foreign keys off`: they run
on one connection with foreign keys off (otherwise dropping the old
table would delete collection memberships and history through
`ON DELETE CASCADE`), and are committed only if `PRAGMA
foreign_key_check` finds no broken link. Tried on a copy of a real
database: 118 images identical in every column, 186 history entries, no
broken link, `integrity_check` ok.

### Syncing

`folders.Sync` walks the folder and its subfolders and compares with the
stored rows:

- new files are added, changed ones (size or modification time) are read
  again, and the rows of removed ones are deleted, in one transaction;
- only JPEG, PNG and WebP, recognized by content; only the header is
  read (size and format), so a large folder syncs fast;
- hidden folders and files, symbolic links, damaged files, images over
  `imaging.MaxPixels` and files past 50 000 are skipped;
- a folder that does not exist or cannot be read (a drive that is not
  connected) is left as it is: its images are not forgotten.

Kisetsu never writes, moves or deletes anything in a folder. Removing a
folder forgets its rows (and their collection memberships and history);
the files stay.

### Showing folder files

- `libfiles` serves `folders/<id>/…`: the folder is opened with
  `os.Root`, like the library, which refuses paths and symbolic links
  that lead outside it. The folder's path comes from the database.
- A folder is opened for one operation and closed again, unlike the
  library's own directories. On Windows an open folder cannot be renamed
  or deleted, and the agent runs all session: keeping folders open made
  the user's own folders immovable (found by CI on Windows, where a test
  could not remove its temporary folder). A Linux test checks through
  `/proc/self/fd` that nothing in a folder stays open.
- `Remove` refuses folder files (`ErrReadOnly`); `OriginalPath` gives
  them to the wallpaper as any image.
- Folder files are served with `no-cache`: the user may replace a file
  under the same name, so the window checks the modification time.
- Thumbnails are made when the window first asks for one
  (`thumbs/folders/<folder>/<image>-<modification time>.jpg`, in
  Kisetsu's data directory), at most two at a time, and recorded. The
  modification time in the name means a changed file gets a new
  thumbnail rather than a cached old one.

### In the window

- The Library tab shows each folder as a chip next to "Fetched", with
  its image count, and "+ Add folder" opens the system's folder dialog.
  Kisetsu's own directories (data, cache, configuration, state) are
  refused; a folder that contains one is allowed, and syncing skips it.
- Opening a folder reads it again first, then shows its images; the view
  says what changed, or that the folder is not available now.
- The preview offers "Set as wallpaper" and collections, but no
  "Delete…" and no source page; `DeleteFromLibrary` refuses folder
  images as well. "Remove folder…" asks first and says the files stay.

### Rotation

- "Images from" offers the folders. The rotator reads the folder again
  before choosing (`Rotator.SyncFolder`), so new files take part and
  removed ones are not tried; the agent and `kisetsu rotate` do the
  same.
- A folder that was removed falls back to the whole library, like a
  deleted collection. A folder that cannot be read now changes nothing:
  none of its files could be shown.

### Collections (0.7.1)

A collection is its members, whatever their origin: images from folders
can be added to collections, and a collection's view and rotation now
include them. 0.7.0 still filtered collections by origin `library`, so a
folder image added to a collection was counted but neither shown nor
rotated, the forgotten condition the rejected alternative below warns
about, in the other direction. Without a collection, the library still
lists only its own images.

## Verification

Unit tests for the migration (on test data and on a copy of a real
database), syncing (new, changed, removed, hidden, damaged, symbolic
links, a read-only folder, a missing folder, Kisetsu's own directories),
the file server (serving, paths and links out of the folder, refusing to
delete, thumbnails on request), the App (listing, refusing to delete,
setting as wallpaper, removing) and rotation (reading before the first
change, a new file, a missing and a removed folder). In the window on
Linux: adding a folder in the system dialog, thumbnails, preview, set as
wallpaper, removing (the files stayed), and a folder as the rotation
source. On Windows 10 (the release candidate in the VM): upgrading
from 0.6.1 with migration 0009 (library, collections and history kept),
adding a folder, rotation from it with a new file picked up, renaming a
subfolder and moving a file while Kisetsu ran, and removing the folder.

## Alternatives considered

- **Importing (copying) into the library:** simpler, but every image
  twice on disk, and new files in the folder not picked up.
- **Keeping origin `library` and marking folder images with a column:**
  no table rebuild, but every library query would have to exclude them;
  one forgotten condition would mix them into the library.
- **Watching folders for changes (inotify):** instant, but a second
  mechanism on every platform; syncing when the folder is opened and
  before a wallpaper change is enough.
