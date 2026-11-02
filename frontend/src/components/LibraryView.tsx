import {useCallback, useEffect, useRef, useState} from 'react';
import {
    AddToCollection,
    CreateCollection,
    DeleteCollection,
    DeleteFromLibrary,
    GetLibraryStats,
    ListCollections,
    ListLibrary,
    RemoveFromCollection,
    RenameCollection,
} from '../../wailsjs/go/main/App';
import {main} from '../../wailsjs/go/models';
import {BackendError, describeError, toBackendError} from '../lib/backend';
import {formatAddedDate, libraryStatsLabel, removeItem} from '../lib/library';
import {CollectionBar} from './CollectionBar';
import {CollectionPicker} from './CollectionPicker';
import {Gallery} from './Gallery';
import {PreviewDialog} from './PreviewDialog';

const pageSize = 60;

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
    const [collections, setCollections] = useState<main.CollectionInfo[]>([]);
    const [collection, setCollection] = useState(0); // 0 = all images
    const [selected, setSelected] = useState<number | null>(null);
    const [error, setError] = useState<BackendError | null>(null);
    const [loading, setLoading] = useState(false);
    const latestRequest = useRef(0);

    const refreshCounts = useCallback(() => {
        GetLibraryStats().then(setStats).catch(() => {});
        ListCollections()
            .then((list) => {
                setCollections(list);
                // The shown collection may have been deleted meanwhile.
                setCollection((current) => (current === 0 || list.some((c) => c.id === current) ? current : 0));
            })
            .catch((err: unknown) => setError(toBackendError(err)));
    }, []);

    const load = useCallback((collectionID: number, offset: number) => {
        const request = ++latestRequest.current;
        setLoading(true);
        setError(null);
        ListLibrary(collectionID, offset, pageSize)
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
        load(collection, 0);
    }, [active, collection, load]);

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

    const selectedItem = selected === null ? null : items[selected];
    const current = collections.find((c) => c.id === collection);

    return (
        <section className="library" aria-busy={loading}>
            <div className="library-header">
                <h2>{current ? current.name : 'Library'}</h2>
                {stats && <p className="muted">{libraryStatsLabel(stats)}</p>}
            </div>

            <CollectionBar
                collections={collections}
                selected={collection}
                totalImages={stats?.images ?? 0}
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
            {!loading && items.length === 0 && stats !== null && (
                <p className="muted">
                    {stats.images === 0
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
                            <CollectionPicker
                                imageID={selectedItem.id}
                                collections={collections}
                                onToggle={(id, member) => toggleMembership(id, selectedItem.id, member)}
                                onCreateWith={(name) => createAndAdd(name, selectedItem.id)}
                            />
                            <LibraryActions
                                key={selectedItem.id}
                                addedAt={selectedItem.addedAt}
                                onDelete={() => deleteItem(selectedItem.id)}
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
    addedAt: string;
    onDelete: () => void;
}

// LibraryActions asks for confirmation before deleting: the first click
// only reveals the buttons that delete or keep the image.
function LibraryActions({addedAt, onDelete}: ActionsProps) {
    const [confirming, setConfirming] = useState(false);
    const added = formatAddedDate(addedAt);

    return (
        <div className="library-actions">
            {added && <p className="muted">Added {added}</p>}
            {confirming ? (
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
