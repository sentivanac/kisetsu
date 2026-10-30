import {FormEvent, useEffect, useRef, useState} from 'react';
import {ListProviders, Search} from '../../wailsjs/go/main/App';
import {providers, search} from '../../wailsjs/go/models';
import {BackendError, describeError, toBackendError} from '../lib/backend';

// Minimum resolutions offered in the filter, as [width, height].
const resolutions: Record<string, [number, number]> = {
    any: [0, 0],
    fhd: [1920, 1080],
    qhd: [2560, 1440],
    uhd: [3840, 2160],
};

// Temporary search UI: shows result counts and sizes. The image gallery
// replaces the result list in the next step.
export function SearchPanel() {
    const [sources, setSources] = useState<search.ProviderInfo[]>([]);
    const [providerID, setProviderID] = useState('');
    const [text, setText] = useState('');
    const [resolution, setResolution] = useState('any');
    const [orientation, setOrientation] = useState('');
    const [page, setPage] = useState<providers.Page | null>(null);
    const [error, setError] = useState<BackendError | null>(null);
    const [loading, setLoading] = useState(false);
    // Only the latest search may update the UI.
    const latestRequest = useRef(0);

    useEffect(() => {
        ListProviders()
            .then((list) => {
                setSources(list);
                setProviderID((current) => current || list[0]?.id || '');
            })
            .catch((err: unknown) => setError(toBackendError(err)));
    }, []);

    function runSearch(event: FormEvent) {
        event.preventDefault();
        const [minWidth, minHeight] = resolutions[resolution];
        const query = new providers.Query({text, page: 1, minWidth, minHeight, orientation});
        const request = ++latestRequest.current;

        setLoading(true);
        setError(null);
        Search(providerID, query)
            .then((result) => {
                if (request === latestRequest.current) setPage(result);
            })
            .catch((err: unknown) => {
                if (request !== latestRequest.current) return;
                setPage(null);
                setError(toBackendError(err));
            })
            .finally(() => {
                if (request === latestRequest.current) setLoading(false);
            });
    }

    return (
        <section className="card search" aria-busy={loading}>
            <h2>Search</h2>
            <form className="search-form" onSubmit={runSearch}>
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
                <button type="submit" className="btn" disabled={loading || !providerID}>
                    {loading ? 'Searching…' : 'Search'}
                </button>
            </form>

            <div aria-live="polite">
                {error && (
                    <p className="error" role="alert">
                        {describeError(error)}
                    </p>
                )}
                {page && (
                    <>
                        <p>
                            {page.images.length} results on page {page.page}
                            {page.hasMore ? ' (more available)' : ''}
                        </p>
                        <ul className="result-list">
                            {page.images.map((img) => (
                                <li key={`${img.provider}/${img.providerId}`}>
                                    {img.providerId} · {img.width} × {img.height}
                                    {img.license.status === 'unknown' ? ' · license unknown' : ` · ${img.license.name}`}
                                </li>
                            ))}
                        </ul>
                    </>
                )}
            </div>
        </section>
    );
}
