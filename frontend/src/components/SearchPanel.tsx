import {FormEvent, useEffect, useRef, useState} from 'react';
import {ListProviders, Search} from '../../wailsjs/go/main/App';
import {images, providers, search} from '../../wailsjs/go/models';
import {BackendError, describeError, toBackendError} from '../lib/backend';
import {appendUnique} from '../lib/images';
import {Gallery} from './Gallery';
import {PreviewDialog} from './PreviewDialog';

// Minimum resolutions offered in the filter, as [width, height].
const resolutions: Record<string, [number, number]> = {
    any: [0, 0],
    fhd: [1920, 1080],
    qhd: [2560, 1440],
    uhd: [3840, 2160],
};

// A search whose results are on screen, kept to request further pages.
interface ActiveSearch {
    providerID: string;
    query: providers.Query;
    page: number;
    hasMore: boolean;
}

export function SearchPanel() {
    const [sources, setSources] = useState<search.ProviderInfo[]>([]);
    const [providerID, setProviderID] = useState('');
    const [text, setText] = useState('');
    const [resolution, setResolution] = useState('any');
    const [orientation, setOrientation] = useState('');

    const [results, setResults] = useState<images.Image[]>([]);
    const [active, setActive] = useState<ActiveSearch | null>(null);
    const [selected, setSelected] = useState<number | null>(null);
    const [error, setError] = useState<BackendError | null>(null);
    const [loading, setLoading] = useState<'search' | 'more' | null>(null);
    // Only the latest request may update the UI.
    const latestRequest = useRef(0);

    useEffect(() => {
        ListProviders()
            .then((list) => {
                setSources(list);
                setProviderID((current) => current || list[0]?.id || '');
            })
            .catch((err: unknown) => setError(toBackendError(err)));
    }, []);

    function load(next: ActiveSearch, append: boolean) {
        const request = ++latestRequest.current;
        setLoading(append ? 'more' : 'search');
        setError(null);

        Search(next.providerID, new providers.Query({...next.query, page: next.page}))
            .then((page) => {
                if (request !== latestRequest.current) return;
                setResults((current) => (append ? appendUnique(current, page.images) : page.images));
                setActive({...next, page: page.page || next.page, hasMore: page.hasMore});
            })
            .catch((err: unknown) => {
                if (request !== latestRequest.current) return;
                setError(toBackendError(err));
            })
            .finally(() => {
                if (request === latestRequest.current) setLoading(null);
            });
    }

    function runSearch(event: FormEvent) {
        event.preventDefault();
        const [minWidth, minHeight] = resolutions[resolution];
        setResults([]);
        setSelected(null);
        setActive(null);
        load({
            providerID,
            query: new providers.Query({text, page: 1, minWidth, minHeight, orientation}),
            page: 1,
            hasMore: false,
        }, false);
    }

    function loadMore() {
        if (active?.hasMore) load({...active, page: active.page + 1}, true);
    }

    const providerName = (id: string) => sources.find((p) => p.id === id)?.name ?? id;
    const selectedImage = selected === null ? null : results[selected];

    return (
        <section className="search" aria-busy={loading !== null}>
            <form className="search-form" onSubmit={runSearch} role="search">
                <label className="search-text">
                    <span className="sr-only">Search text</span>
                    <input
                        type="search"
                        value={text}
                        onChange={(e) => setText(e.target.value)}
                        placeholder="Misty mountains, Japanese village…"
                        maxLength={256}
                    />
                </label>
                <label>
                    Source
                    <select value={providerID} onChange={(e) => setProviderID(e.target.value)}>
                        {sources.map((p) => (
                            <option key={p.id} value={p.id}>
                                {p.name}
                            </option>
                        ))}
                    </select>
                </label>
                <label>
                    Minimum size
                    <select value={resolution} onChange={(e) => setResolution(e.target.value)}>
                        <option value="any">Any</option>
                        <option value="fhd">1920 × 1080</option>
                        <option value="qhd">2560 × 1440</option>
                        <option value="uhd">3840 × 2160</option>
                    </select>
                </label>
                <label>
                    Orientation
                    <select value={orientation} onChange={(e) => setOrientation(e.target.value)}>
                        <option value="">Any</option>
                        <option value="landscape">Landscape</option>
                        <option value="portrait">Portrait</option>
                        <option value="square">Square</option>
                    </select>
                </label>
                <button type="submit" className="btn" disabled={loading === 'search' || !providerID}>
                    {loading === 'search' ? 'Searching…' : 'Search'}
                </button>
            </form>

            <div aria-live="polite">
                {error && (
                    <p className="error" role="alert">
                        {describeError(error)}
                    </p>
                )}
                {active && results.length === 0 && !error && (
                    <p className="muted">No images match this search. Try other words or fewer filters.</p>
                )}
                {results.length > 0 && <p className="muted">{results.length} images</p>}
            </div>

            <Gallery items={results} onSelect={setSelected}/>

            {active?.hasMore && (
                <div className="load-more">
                    <button type="button" className="btn secondary" onClick={loadMore} disabled={loading !== null}>
                        {loading === 'more' ? 'Loading…' : 'Load more'}
                    </button>
                </div>
            )}

            {selectedImage && selected !== null && (
                <PreviewDialog
                    image={selectedImage}
                    providerName={providerName(selectedImage.provider)}
                    onClose={() => setSelected(null)}
                    onPrevious={selected > 0 ? () => setSelected(selected - 1) : undefined}
                    onNext={selected < results.length - 1 ? () => setSelected(selected + 1) : undefined}
                />
            )}
        </section>
    );
}
