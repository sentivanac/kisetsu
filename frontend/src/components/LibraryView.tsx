import {useCallback, useEffect, useRef, useState} from 'react';
import {DeleteFromLibrary, GetLibraryStats, ListLibrary} from '../../wailsjs/go/main/App';
import {main} from '../../wailsjs/go/models';
import {BackendError, describeError, toBackendError} from '../lib/backend';
import {formatAddedDate, libraryStatsLabel, removeItem} from '../lib/library';
import {Gallery} from './Gallery';
import {PreviewDialog} from './PreviewDialog';

const pageSize = 60;

interface Props {
    // The view reloads when it becomes visible, to show new downloads.
    active: boolean;
    sourceName: (providerId: string) => string;
}

// LibraryView shows downloaded images from local files.
export function LibraryView({active, sourceName}: Props) {
    const [items, setItems] = useState<main.LibraryItem[]>([]);
    const [hasMore, setHasMore] = useState(false);
    const [stats, setStats] = useState<main.LibraryStats | null>(null);
    const [selected, setSelected] = useState<number | null>(null);
    const [error, setError] = useState<BackendError | null>(null);
    const [loading, setLoading] = useState(false);
    const latestRequest = useRef(0);

    const load = useCallback((offset: number) => {
        const request = ++latestRequest.current;
        setLoading(true);
        setError(null);
        Promise.all([ListLibrary(offset, pageSize), GetLibraryStats()])
            .then(([page, newStats]) => {
                if (request !== latestRequest.current) return;
                setItems((current) => (offset === 0 ? page.items : [...current, ...page.items]));
                setHasMore(page.hasMore);
                setStats(newStats);
            })
            .catch((err: unknown) => {
                if (request === latestRequest.current) setError(toBackendError(err));
            })
            .finally(() => {
                if (request === latestRequest.current) setLoading(false);
            });
    }, []);

    useEffect(() => {
        if (active) {
            setSelected(null);
            load(0);
        }
    }, [active, load]);

    function deleteItem(id: number) {
        DeleteFromLibrary(id)
            .then(() => {
                const {items: rest, next} = removeItem(items, id);
                setItems(rest);
                setSelected(next);
                GetLibraryStats().then(setStats).catch(() => {});
            })
            .catch((err: unknown) => setError(toBackendError(err)));
    }

    const selectedItem = selected === null ? null : items[selected];

    return (
        <section className="library" aria-busy={loading}>
            <div className="library-header">
                <h2>Library</h2>
                {stats && <p className="muted">{libraryStatsLabel(stats)}</p>}
            </div>

            {error && (
                <p className="error" role="alert">
                    {describeError(error)}
                </p>
            )}
            {stats?.images === 0 && !loading && (
                <p className="muted">Your library is empty. Open an image in Search and choose Download.</p>
            )}

            <Gallery items={items.map((i) => i.image)} sourceName={sourceName} onSelect={setSelected}/>

            {hasMore && (
                <div className="load-more">
                    <button type="button" className="btn secondary" onClick={() => load(items.length)} disabled={loading}>
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
                        <LibraryActions
                            key={selectedItem.id}
                            addedAt={selectedItem.addedAt}
                            onDelete={() => deleteItem(selectedItem.id)}
                        />
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
