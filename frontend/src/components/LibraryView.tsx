import {useCallback, useEffect, useRef, useState} from 'react';
import {
    AddFolder,
    AddToFavorites,
    AddToCollection,
    CancelDownload,
    CheckOriginal,
    CreateCollection,
    DeleteCollection,
    DeleteFromLibrary,
    DownloadFetched,
    GetDownloadSettings,
    GetFetchedStats,
    GetLibraryItem,
    GetLibraryStats,
    GetOriginal,
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
    SaveDownloadSize,
    SetWallpaper,
    ShowImageAgain,
    SyncFolder,
} from '../../wailsjs/go/main/App';
import {main} from '../../wailsjs/go/models';
import {BackendError, describeError, toBackendError} from '../lib/backend';
import {downloadChoices, DownloadProgress, DownloadSize, isActive, screenCopySize} from '../lib/downloads';
import {formatFileSize, imageKey} from '../lib/images';
import {fetchedStatsLabel, folderSyncLabel, formatAddedDate, libraryStatsLabel, removeItem} from '../lib/library';
import {useDownloads} from '../lib/useDownloads';
import {CollectionBar, FetchedView, folderOf, folderView, isFolderView} from './CollectionBar';
import {CollectionPicker} from './CollectionPicker';
import {DownloadButton} from './DownloadButton';
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
    const downloads = useDownloads();
    const [downloadSettings, setDownloadSettings] = useState<main.DownloadSettings | null>(null);
    // Errors from starting a download, and other notes, by entry ID.
    const [startErrors, setStartErrors] = useState<Record<number, BackendError>>({});
    const [originalNotes, setOriginalNotes] = useState<Record<number, string>>({});
    // Downloads started here that have not finished, by key; before is the
    // state of an earlier download of the same image.
    const pending = useRef(new Map<string, {id: number; before?: DownloadProgress}>());

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
        if (!active) return;
        refreshCounts();
        GetDownloadSettings().then(setDownloadSettings).catch(() => {});
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

    // dropItem removes an item from the shown list. Its preview moves on to
    // the next image; the preview of another image stays.
    function dropItem(id: number) {
        const {items: rest, next} = removeItem(items, id);
        const open = selected === null ? undefined : items[selected]?.id;
        setItems(rest);
        if (open === undefined) return;
        setSelected(open === id ? next : rest.findIndex((it) => it.id === open));
    }

    function refreshItem(id: number) {
        GetLibraryItem(id)
            .then((item) => setItems((list) => list.map((it) => (it.id === id ? item : it))))
            .catch((err: unknown) => setError(toBackendError(err)));
    }

    function startDownload(item: main.LibraryItem, size: DownloadSize) {
        const key = imageKey(item.image);
        const {id} = item;
        setStartErrors(({[id]: _, ...rest}) => rest);
        pending.current.set(key, {id, before: downloads[key]});
        const started = showingFetched ? DownloadFetched(id, size) : GetOriginal(id);
        started
            .then((startedKey) => {
                if (startedKey !== '') return;
                pending.current.delete(key);
                setOriginalNotes((notes) => ({...notes, [id]: 'This file is already the original.'}));
                refreshItem(id);
            })
            .catch((err: unknown) => {
                pending.current.delete(key);
                setStartErrors((current) => ({...current, [id]: toBackendError(err)}));
            });
    }

    useEffect(() => {
        for (const [key, {id, before}] of pending.current) {
            const p = downloads[key];
            if (!p || p === before || isActive(p)) continue;
            pending.current.delete(key);
            if (p.state !== 'done') continue;
            refreshCounts();
            if (showingFetched) dropItem(id);
            else {
                setOriginalNotes((notes) => ({...notes, [id]: 'Replaced by the original.'}));
                refreshItem(id);
            }
        }
        // Only download events matter; the rest is current when they arrive.
    }, [downloads]);

    function chooseDownloadSize(size: string) {
        SaveDownloadSize(size)
            .then(() => GetDownloadSettings())
            .then(setDownloadSettings)
            .catch((err: unknown) => setError(toBackendError(err)));
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

    // Downloads shown for an item: a finished one says nothing about the
    // file that is stored now.
    function shownProgress(item: main.LibraryItem): DownloadProgress | undefined {
        const p = downloads[imageKey(item.image)];
        if (p?.state === 'done' || (p?.state === 'duplicate' && !showingFetched)) return undefined;
        return p;
    }

    function fetchedChoices(item: main.LibraryItem) {
        if (!downloadSettings) return [];
        // An original: the stored file is all there is.
        if (item.stored === 'original') return downloadChoices(item.image, null, 'original');
        const known = item.originalWidth > 0 && item.originalHeight > 0;
        const original = known ? {width: item.originalWidth, height: item.originalHeight} : null;
        const screen = {width: downloadSettings.screenWidth, height: downloadSettings.screenHeight};
        const copy = original
            ? screenCopySize(original, screen)
            : {width: item.image.width, height: item.image.height};
        return downloadChoices(original, copy, downloadSettings.size as DownloadSize);
    }

    const selectedItem = selected === null ? null : items[selected];
    const uncheckedID = selectedItem?.stored === 'unknown' ? selectedItem.id : null;

    // Whether an image stored before 0.9.0 is the original is found out when
    // it is opened. Failing that, it stays unknown and can be downloaded.
    useEffect(() => {
        if (uncheckedID === null) return;
        CheckOriginal(uncheckedID)
            .then((item) => setItems((list) => list.map((it) => (it.id === item.id ? item : it))))
            .catch(() => {});
    }, [uncheckedID]);
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
                {downloadSettings && !folder && (
                    <label className="download-size">
                        Save downloads as
                        <select
                            value={downloadSettings.size}
                            onChange={(e) => chooseDownloadSize(e.target.value)}
                            title="Screen copy: reduced to cover your screen, not cropped. Images already in your library do not change."
                        >
                            <option value="original">Original</option>
                            <option value="screen">Screen copy</option>
                        </select>
                    </label>
                )}
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
                    size; the oldest are removed when that is full. Download an image to keep it in your library.
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
                                <DownloadButton
                                    key={selectedItem.id}
                                    progress={shownProgress(selectedItem)}
                                    inLibrary={false}
                                    startError={startErrors[selectedItem.id]}
                                    choices={fetchedChoices(selectedItem)}
                                    onDownload={(size) => startDownload(selectedItem, size)}
                                    onCancel={() => {
                                        CancelDownload(imageKey(selectedItem.image)).catch(() => {});
                                    }}
                                    onKeepCopy={keepItem}
                                />
                            ) : (
                                <>
                                    {selectedItem.stored !== 'original' && (
                                        <div className="library-actions">
                                            <DownloadButton
                                                key={selectedItem.id}
                                                progress={shownProgress(selectedItem)}
                                                inLibrary={false}
                                                startError={startErrors[selectedItem.id]}
                                                choices={downloadChoices(
                                                    selectedItem.originalWidth > 0
                                                        ? {
                                                              width: selectedItem.originalWidth,
                                                              height: selectedItem.originalHeight,
                                                          }
                                                        : null,
                                                    null,
                                                    'original',
                                                )}
                                                idleLabel="Get original"
                                                onDownload={(size) => startDownload(selectedItem, size)}
                                                onCancel={() => {
                                                    CancelDownload(imageKey(selectedItem.image)).catch(() => {});
                                                }}
                                            />
                                            <p className="muted small">
                                                {selectedItem.stored === 'reduced'
                                                    ? 'This is a screen copy: reduced to cover your screen, not cropped.'
                                                    : 'It is not known yet whether this file is the original.'}
                                            </p>
                                        </div>
                                    )}
                                    {originalNotes[selectedItem.id] && (
                                        <p className="notice" role="status">
                                            {originalNotes[selectedItem.id]}
                                        </p>
                                    )}
                                    <CollectionPicker
                                        imageID={selectedItem.id}
                                        collections={collections}
                                        onToggle={(id, member) => toggleMembership(id, selectedItem.id, member)}
                                        onCreateWith={(name) => createAndAdd(name, selectedItem.id)}
                                    />
                                </>
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
