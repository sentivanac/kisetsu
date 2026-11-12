import {useCallback, useEffect, useRef, useState} from 'react';
import {
    AddFolder,
    AddToFavorites,
    AddToCollection,
    CreateCollection,
    DeleteCollection,
    DeleteFromLibrary,
    GetFetchedStats,
    GetLibraryStats,
    HideImage,
    KeepFetched,
    ListCollections,
    ListFetched,
    ListFolder,
    ListFolders,
    ListLibrary,
    RemoveFolder,
    RemoveFromFavorites,
    RemoveFromCollection,
    RenameCollection,
    SetWallpaper,
    ShowImageAgain,
    SyncFolder,
} from '../../wailsjs/go/main/App';
import {main} from '../../wailsjs/go/models';
import {BackendError, describeError, toBackendError} from '../lib/backend';
import {formatFileSize} from '../lib/images';
import {fetchedStatsLabel, folderSyncLabel, formatAddedDate, libraryStatsLabel, removeItem} from '../lib/library';
import {CollectionBar, FetchedView, folderOf, folderView, isFolderView} from './CollectionBar';
import {CollectionPicker} from './CollectionPicker';
import {Gallery} from './Gallery';
import {PreviewDialog} from './PreviewDialog';

const pageSize = 60;

// fromFolder: the image is a file in one of the user's folders, which
// Kisetsu never deletes; a collection can hold such images too.
const fromFolder = (item: main.LibraryItem) => item.image.provider === 'folder';

interface Props {
    // The view reloads when it becomes visible, to show new downloads.
    active: boolean;
    sourceName: (providerId: string) => string;
}

// LibraryView shows downloaded images from local files, by collection.
export function LibraryView({active, sourceName}: Props) {
    const [items, setItems] = useState<main.LibraryItem[]>([]);
    const [hasMore, setHasMore] = useState(false);
    const [stats, setStats] = useState<main.LibraryStats | null>(null);
    const [fetchedStats, setFetchedStats] = useState<main.FetchedStats | null>(null);
    const [collections, setCollections] = useState<main.CollectionInfo[]>([]);
    const [folders, setFolders] = useState<main.FolderInfo[]>([]);
    const [addingFolder, setAddingFolder] = useState(false);
    // What the last reading of the shown folder found.
    const [folderNote, setFolderNote] = useState<string | null>(null);
    const [collection, setCollection] = useState(0); // 0 = all images
    const [selected, setSelected] = useState<number | null>(null);
    const [error, setError] = useState<BackendError | null>(null);
    const [loading, setLoading] = useState(false);
    const latestRequest = useRef(0);

    const refreshCounts = useCallback(() => {
        GetLibraryStats().then(setStats).catch(() => {});
        GetFetchedStats().then(setFetchedStats).catch(() => {});
        Promise.all([ListCollections(), ListFolders()])
            .then(([list, folderList]) => {
                setCollections(list);
                setFolders(folderList);
                // The shown collection or folder may have been removed meanwhile.
                setCollection((current) =>
                    current === 0 ||
                    current === FetchedView ||
                    list.some((c) => c.id === current) ||
                    (isFolderView(current) && folderList.some((f) => f.id === folderOf(current)))
                        ? current
                        : 0,
                );
            })
            .catch((err: unknown) => setError(toBackendError(err)));
    }, []);

    const load = useCallback((collectionID: number, offset: number) => {
        const request = ++latestRequest.current;
        setLoading(true);
        setError(null);
        let page: Promise<main.LibraryPage>;
        if (isFolderView(collectionID)) {
            const id = folderOf(collectionID);
            // Opening a folder reads it again first: new and removed files.
            const read =
                offset === 0
                    ? SyncFolder(id).then((r) => {
                          if (request !== latestRequest.current) return;
                          setFolderNote(folderSyncLabel(r));
                          setFolders((list) => list.map((f) => (f.id === id ? r.folder : f)));
                      })
                    : Promise.resolve();
            page = read.then(() => ListFolder(id, offset, pageSize));
        } else if (collectionID === FetchedView) {
            page = ListFetched(offset, pageSize);
        } else {
            page = ListLibrary(collectionID, offset, pageSize);
        }
        page
            .then((page) => {
                if (request !== latestRequest.current) return;
                setItems((current) => (offset === 0 ? page.items : [...current, ...page.items]));
                setHasMore(page.hasMore);
            })
            .catch((err: unknown) => {
                if (request === latestRequest.current) setError(toBackendError(err));
            })
            .finally(() => {
                if (request === latestRequest.current) setLoading(false);
            });
    }, []);

    useEffect(() => {
        if (active) refreshCounts();
    }, [active, refreshCounts]);

    useEffect(() => {
        if (!active) return;
        setSelected(null);
        setFolderNote(null);
        load(collection, 0);
    }, [active, collection, load]);

    function addFolder() {
        setAddingFolder(true);
        setError(null);
        AddFolder()
            .then((r) => {
                if (!r.folder.id) return; // the dialog was closed
                refreshCounts();
                setCollection(folderView(r.folder.id));
            })
            .catch((err: unknown) => setError(toBackendError(err)))
            .finally(() => setAddingFolder(false));
    }

    function removeFolder(id: number) {
        RemoveFolder(id)
            .then(() => {
                setCollection(0);
                refreshCounts();
            })
            .catch((err: unknown) => setError(toBackendError(err)));
    }

    // dropItem removes an item from the shown list and moves the preview on.
    function dropItem(id: number) {
        const {items: rest, next} = removeItem(items, id);
        setItems(rest);
        setSelected(next);
    }

    function deleteItem(id: number) {
        DeleteFromLibrary(id)
            .then(() => {
                dropItem(id);
                refreshCounts();
            })
            .catch((err: unknown) => setError(toBackendError(err)));
    }

    async function toggleMembership(collectionID: number, imageID: number, member: boolean) {
        await (member ? AddToCollection(collectionID, imageID) : RemoveFromCollection(collectionID, imageID));
        // Leaving the collection that is shown removes the image from view.
        if (!member && collectionID === collection) dropItem(imageID);
        refreshCounts();
    }

    async function createAndAdd(name: string, imageID: number) {
        const created = await CreateCollection(name);
        await AddToCollection(created.id, imageID);
        refreshCounts();
    }

    function deleteCollection(id: number) {
        DeleteCollection(id)
            .then(() => {
                if (collection === id) setCollection(0);
                refreshCounts();
            })
            .catch((err: unknown) => setError(toBackendError(err)));
    }

    function keepItem(id: number) {
        KeepFetched(id)
            .then(() => {
                dropItem(id);
                refreshCounts();
            })
            .catch((err: unknown) => setError(toBackendError(err)));
    }

    const selectedItem = selected === null ? null : items[selected];
    const current = collections.find((c) => c.id === collection);
    const showingFetched = collection === FetchedView;
    const folder = isFolderView(collection) ? folders.find((f) => f.id === folderOf(collection)) : undefined;

    return (
        <section className="library" aria-busy={loading}>
            <div className="library-header">
                <h2>{showingFetched ? 'Fetched' : folder ? folder.name : current ? current.name : 'Library'}</h2>
                {showingFetched
                    ? fetchedStats && <p className="muted">{fetchedStatsLabel(fetchedStats)}</p>
                    : folder
                      ? <p className="muted">{folder.path}</p>
                      : stats && <p className="muted">{libraryStatsLabel(stats)}</p>}
            </div>

            <CollectionBar
                collections={collections}
                folders={folders}
                selected={collection}
                totalImages={stats?.images ?? 0}
                fetchedImages={fetchedStats?.images ?? 0}
                addingFolder={addingFolder}
                onAddFolder={addFolder}
                onRemoveFolder={removeFolder}
                onSelect={setCollection}
                onCreate={async (name) => {
                    const created = await CreateCollection(name);
                    refreshCounts();
                    setCollection(created.id);
                }}
                onRename={async (id, name) => {
                    await RenameCollection(id, name);
                    refreshCounts();
                }}
                onDelete={deleteCollection}
            />

            {error && (
                <p className="error" role="alert">
                    {describeError(error)}
                </p>
            )}
            {showingFetched && (
                <p className="muted small">
                    Images fetched automatically from saved searches. They are kept to{' '}
                    {fetchedStats ? formatFileSize(fetchedStats.maxBytes) : '200 MB'}, stored at your screen
                    size; the oldest are removed when that is full. Keep an image to move it into your library.
                </p>
            )}
            {folder && (
                <p className={folder.missing ? 'notice error' : 'muted small'}>
                    {folder.missing
                        ? 'This folder is not available now (a drive that is not connected?). Its images come back when it is.'
                        : (folderNote ?? 'Reading the folder…') +
                          ' Kisetsu uses the images where they are and never changes or deletes them.'}
                </p>
            )}
            {!loading && items.length === 0 && stats !== null && !folder?.missing && (
                <p className="muted">
                    {folder
                        ? 'No images in this folder yet. Kisetsu shows JPEG, PNG and WebP files, subfolders included.'
                        : showingFetched
                        ? 'No fetched images yet. Choose a saved search as the rotation source in the Wallpaper tab.'
                        : stats.images === 0
                          ? 'Your library is empty. Open an image in Search and choose Download.'
                          : 'This collection is empty. Open an image and tick the collection to add it.'}
                </p>
            )}

            <Gallery items={items.map((i) => i.image)} sourceName={sourceName} onSelect={setSelected}/>

            {hasMore && (
                <div className="load-more">
                    <button
                        type="button"
                        className="btn secondary"
                        onClick={() => load(collection, items.length)}
                        disabled={loading}
                    >
                        {loading ? 'Loading…' : 'Load more'}
                    </button>
                </div>
            )}

            {selectedItem && selected !== null && (
                <PreviewDialog
                    image={selectedItem.image}
                    fullUrl={selectedItem.fileUrl}
                    providerName={sourceName(selectedItem.image.provider)}
                    actions={
                        <>
                            {showingFetched ? (
                                <div className="library-actions">
                                    <button type="button" className="btn" onClick={() => keepItem(selectedItem.id)}>
                                        Keep in my library
                                    </button>
                                    <p className="muted small">
                                        Stored at {selectedItem.image.width} × {selectedItem.image.height} for your
                                        screen. The original stays available on its source page.
                                    </p>
                                </div>
                            ) : (
                                <CollectionPicker
                                    imageID={selectedItem.id}
                                    collections={collections}
                                    onToggle={(id, member) => toggleMembership(id, selectedItem.id, member)}
                                    onCreateWith={(name) => createAndAdd(name, selectedItem.id)}
                                />
                            )}
                            <LibraryActions
                                key={selectedItem.id}
                                imageID={selectedItem.id}
                                favorite={selectedItem.favorite}
                                onFavoriteChange={(favorite) => {
                                    // A fetched favorite is kept in the library, so it leaves this view.
                                    if (showingFetched && favorite) dropItem(selectedItem.id);
                                    else
                                        setItems((list) =>
                                            list.map((it) =>
                                                it.id === selectedItem.id ? new main.LibraryItem({...it, favorite}) : it,
                                            ),
                                        );
                                    refreshCounts();
                                }}
                                hidden={selectedItem.hidden}
                                onHiddenChange={(hidden) =>
                                    setItems((list) =>
                                        list.map((it) =>
                                            it.id === selectedItem.id ? new main.LibraryItem({...it, hidden}) : it,
                                        ),
                                    )
                                }
                                addedAt={fromFolder(selectedItem) ? '' : selectedItem.addedAt}
                                onDelete={fromFolder(selectedItem) ? undefined : () => deleteItem(selectedItem.id)}
                            />
                        </>
                    }
                    onClose={() => setSelected(null)}
                    onPrevious={selected > 0 ? () => setSelected(selected - 1) : undefined}
                    onNext={selected < items.length - 1 ? () => setSelected(selected + 1) : undefined}
                />
            )}
        </section>
    );
}

interface ActionsProps {
    imageID: number;
    addedAt: string;
    // favorite: in the Favorites collection.
    favorite: boolean;
    onFavoriteChange: (favorite: boolean) => void;
    // hidden: kept out of rotation ("Don't show again").
    hidden: boolean;
    onHiddenChange: (hidden: boolean) => void;
    // Not set for images in the user's folders, which are never deleted.
    onDelete?: () => void;
}

// LibraryActions sets the image as wallpaper and deletes it. Deleting asks
// for confirmation: the first click only reveals the buttons that delete
// or keep the image.
function LibraryActions({
    imageID,
    addedAt,
    favorite,
    onFavoriteChange,
    hidden,
    onHiddenChange,
    onDelete,
}: ActionsProps) {
    const [confirming, setConfirming] = useState(false);
    const [settingWallpaper, setSettingWallpaper] = useState(false);
    const [wallpaperResult, setWallpaperResult] = useState<{ok: boolean; message: string} | null>(null);
    const added = formatAddedDate(addedAt);

    function setAsWallpaper() {
        setSettingWallpaper(true);
        setWallpaperResult(null);
        SetWallpaper(imageID)
            .then((r) => setWallpaperResult({ok: true, message: r.message}))
            .catch((err: unknown) => setWallpaperResult({ok: false, message: describeError(toBackendError(err))}))
            .finally(() => setSettingWallpaper(false));
    }

    return (
        <div className="library-actions">
            {added && <p className="muted">Added {added}</p>}
            <button type="button" className="btn" onClick={setAsWallpaper} disabled={settingWallpaper}>
                {settingWallpaper ? 'Setting wallpaper…' : 'Set as wallpaper'}
            </button>
            <button
                type="button"
                className={favorite ? 'btn secondary favorite on' : 'btn secondary favorite'}
                aria-pressed={favorite}
                onClick={() => {
                    setWallpaperResult(null);
                    (favorite ? RemoveFromFavorites(imageID) : AddToFavorites(imageID))
                        .then(() => onFavoriteChange(!favorite))
                        .catch((err: unknown) =>
                            setWallpaperResult({ok: false, message: describeError(toBackendError(err))}),
                        );
                }}
            >
                <span aria-hidden="true">{favorite ? '♥' : '♡'}</span> {favorite ? 'In favorites' : 'Add to favorites'}
            </button>
            <button
                type="button"
                className="btn secondary"
                onClick={() => {
                    setWallpaperResult(null);
                    (hidden ? ShowImageAgain(imageID) : HideImage(imageID))
                        .then(() => onHiddenChange(!hidden))
                        .catch((err: unknown) =>
                            setWallpaperResult({ok: false, message: describeError(toBackendError(err))}),
                        );
                }}
            >
                {hidden ? 'Show in rotation again' : "Don't show again"}
            </button>
            {hidden && <p className="muted small">Rotation and Previous skip this image.</p>}
            {wallpaperResult && (
                <p className={wallpaperResult.ok ? 'notice' : 'notice error'} role={wallpaperResult.ok ? 'status' : 'alert'}>
                    {wallpaperResult.message}
                </p>
            )}
            {!onDelete ? null : confirming ? (
                <div className="confirm" role="group" aria-label="Confirm deletion">
                    <p>Delete this image and its file from your library? This cannot be undone.</p>
                    <div className="download-row">
                        <button type="button" className="btn danger" onClick={onDelete}>
                            Delete permanently
                        </button>
                        <button type="button" className="btn secondary" onClick={() => setConfirming(false)} autoFocus>
                            Keep
                        </button>
                    </div>
                </div>
            ) : (
                <button type="button" className="btn secondary" onClick={() => setConfirming(true)}>
                    Delete…
                </button>
            )}
        </div>
    );
}
