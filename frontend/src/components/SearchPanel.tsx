import {FormEvent, useEffect, useRef, useState} from 'react';
import {
    CancelSearch,
    GetSearchFilters,
    ListProviders,
    SaveSearchFilters,
    SearchSources,
} from '../../wailsjs/go/main/App';
import {images, providers, search, settings} from '../../wailsjs/go/models';
import {ScreenGetAll} from '../../wailsjs/runtime/runtime';
import {BackendError, describeError, isCanceled, toBackendError} from '../lib/backend';
import {
    anyResolution,
    parseResolutionKey,
    Resolution,
    resolutionKey,
    resolutionLabel,
    resolutionOptions,
} from '../lib/filters';
import {appendUnique} from '../lib/images';
import {screenPixels, WailsScreen} from '../lib/screen';
import {
    initialSources,
    mergeStatuses,
    nextPages,
    SourceInfo,
    SourcePage,
    SourceStatus,
    statusLabel,
    toggleSource,
} from '../lib/sources';
import {Gallery} from './Gallery';
import {PreviewDialog} from './PreviewDialog';

export function SearchPanel() {
    const [available, setAvailable] = useState<SourceInfo[]>([]);
    const [selectedSources, setSelectedSources] = useState<string[]>([]);
    const [text, setText] = useState('');
    const [resolution, setResolution] = useState<Resolution>(anyResolution);
    const [orientation, setOrientation] = useState('');
    // Filters are saved only after the remembered ones were loaded, so the
    // defaults shown during startup never overwrite them.
    const [filtersLoaded, setFiltersLoaded] = useState(false);

    const [results, setResults] = useState<images.Image[]>([]);
    // The query of the search on screen, reused by "Load more".
    const [activeQuery, setActiveQuery] = useState<providers.Query | null>(null);
    const [statuses, setStatuses] = useState<SourceStatus[]>([]);
    const [selected, setSelected] = useState<number | null>(null);
    const [error, setError] = useState<BackendError | null>(null);
    const [loading, setLoading] = useState<'search' | 'more' | null>(null);
    // Only the latest request may update the UI.
    const latestRequest = useRef(0);

    useEffect(() => {
        ListProviders()
            .then(async (list: search.ProviderInfo[]) => {
                setAvailable(list);
                // Remembered filters are a convenience: if they cannot be
                // loaded, searching still works with the defaults.
                const saved = await GetSearchFilters().catch(() => new settings.SearchFilters({}));
                setSelectedSources(initialSources(list, saved.providerIds));
                setResolution({width: saved.minWidth, height: saved.minHeight});
                setOrientation(saved.orientation);
                setFiltersLoaded(true);
            })
            .catch((err: unknown) => setError(toBackendError(err)));
    }, []);

    useEffect(() => {
        if (!filtersLoaded) return;
        SaveSearchFilters(
            new settings.SearchFilters({
                providerIds: selectedSources,
                minWidth: resolution.width,
                minHeight: resolution.height,
                orientation,
            }),
        ).catch((err: unknown) => setError(toBackendError(err)));
    }, [filtersLoaded, selectedSources, resolution, orientation]);

    function applyScreenSize() {
        ScreenGetAll()
            .then((screens) => {
                const size = screenPixels(screens as WailsScreen[]);
                if (size) setResolution(size);
            })
            .catch((err: unknown) => setError(toBackendError(err)));
    }

    function load(pages: SourcePage[], query: providers.Query, append: boolean) {
        const request = ++latestRequest.current;
        setLoading(append ? 'more' : 'search');
        setError(null);

        SearchSources(pages.map((p) => new search.SourcePage(p)), query)
            .then((result) => {
                if (request !== latestRequest.current) return;
                setResults((current) => (append ? appendUnique(current, result.images) : result.images));
                setStatuses((current) => (append ? mergeStatuses(current, result.sources) : result.sources));
            })
            .catch((err: unknown) => {
                if (request !== latestRequest.current) return;
                const backendError = toBackendError(err);
                if (!isCanceled(backendError)) setError(backendError);
            })
            .finally(() => {
                if (request === latestRequest.current) setLoading(null);
            });
    }

    function runSearch(event: FormEvent) {
        event.preventDefault();
        const query = new providers.Query({
            text,
            page: 1,
            minWidth: resolution.width,
            minHeight: resolution.height,
            orientation,
        });
        setResults([]);
        setStatuses([]);
        setSelected(null);
        setActiveQuery(query);
        load(selectedSources.map((providerId) => ({providerId, page: 1})), query, false);
    }

    function loadMore() {
        const pages = nextPages(statuses);
        if (activeQuery && pages.length > 0) load(pages, activeQuery, true);
    }

    function cancel() {
        // The pending search then rejects with "canceled", which is ignored.
        CancelSearch().catch((err: unknown) => setError(toBackendError(err)));
    }

    const sourceName = (id: string) => available.find((p) => p.id === id)?.name ?? id;
    const selectedImage = selected === null ? null : results[selected];
    const morePages = nextPages(statuses);
    const finished = activeQuery !== null && loading === null && statuses.length > 0;

    return (
        <section className="search" aria-busy={loading !== null}>
            <form className="search-form" onSubmit={runSearch} role="search">
                <label className="search-text">
                    <span className="sr-only">Search text</span>
                    <input
                        type="search"
                        value={text}
                        onChange={(e) => setText(e.target.value)}
                        placeholder="Misty mountains, Japanese village…  (-word excludes)"
                        maxLength={256}
                    />
                </label>
                <label>
                    Minimum size
                    <select
                        value={resolutionKey(resolution)}
                        onChange={(e) => setResolution(parseResolutionKey(e.target.value))}
                    >
                        {resolutionOptions(resolution).map((r) => (
                            <option key={resolutionKey(r)} value={resolutionKey(r)}>
                                {resolutionLabel(r)}
                            </option>
                        ))}
                    </select>
                </label>
                <button type="button" className="btn secondary" onClick={applyScreenSize}>
                    Use my screen size
                </button>
                <label>
                    Orientation
                    <select value={orientation} onChange={(e) => setOrientation(e.target.value)}>
                        <option value="">Any</option>
                        <option value="landscape">Landscape</option>
                        <option value="portrait">Portrait</option>
                        <option value="square">Square</option>
                    </select>
                </label>
                {loading === 'search' ? (
                    <button type="button" className="btn secondary" onClick={cancel}>
                        Cancel
                    </button>
                ) : (
                    <button type="submit" className="btn" disabled={selectedSources.length === 0}>
                        Search
                    </button>
                )}

                <fieldset className="sources">
                    <legend>Sources</legend>
                    {available.map((p) => (
                        <label key={p.id} className="source">
                            <input
                                type="checkbox"
                                checked={selectedSources.includes(p.id)}
                                onChange={() => setSelectedSources((s) => toggleSource(available, s, p.id))}
                            />
                            <span>{p.name}</span>
                            {p.contentWarning && <span className="source-warning">{p.contentWarning}</span>}
                        </label>
                    ))}
                </fieldset>
            </form>

            <div aria-live="polite">
                {error && (
                    <p className="error" role="alert">
                        {describeError(error)}
                    </p>
                )}
                {selectedSources.length === 0 && filtersLoaded && (
                    <p className="muted">Select at least one source to search.</p>
                )}
                {statuses.length > 0 && (
                    <ul className="source-status">
                        {statuses.map((s) => (
                            <li key={s.providerId} className={s.error ? 'error' : undefined}>
                                {statusLabel(s)}
                            </li>
                        ))}
                    </ul>
                )}
                {finished && results.length === 0 && statuses.every((s) => !s.error) && (
                    <p className="muted">No images match this search. Try other words or fewer filters.</p>
                )}
            </div>

            <Gallery items={results} sourceName={sourceName} onSelect={setSelected}/>

            {activeQuery && morePages.length > 0 && (
                <div className="load-more">
                    <button type="button" className="btn secondary" onClick={loadMore} disabled={loading !== null}>
                        {loading === 'more' ? 'Loading…' : 'Load more'}
                    </button>
                    {loading === 'more' && (
                        <button type="button" className="btn secondary" onClick={cancel}>
                            Cancel
                        </button>
                    )}
                </div>
            )}

            {selectedImage && selected !== null && (
                <PreviewDialog
                    image={selectedImage}
                    providerName={sourceName(selectedImage.provider)}
                    onClose={() => setSelected(null)}
                    onPrevious={selected > 0 ? () => setSelected(selected - 1) : undefined}
                    onNext={selected < results.length - 1 ? () => setSelected(selected + 1) : undefined}
                />
            )}
        </section>
    );
}
