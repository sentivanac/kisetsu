import {FormEvent, useEffect, useRef, useState} from 'react';
import {
    CancelDownload,
    CancelSearch,
    DeleteSavedSearch,
    Download,
    GetSearchFilters,
    LibraryStatus,
    ListProviders,
    ListSavedSearches,
    RenameSavedSearch,
    SaveSearch,
    SaveSearchFilters,
    SearchSources,
} from '../../wailsjs/go/main/App';
import {images, main, providers, search, searches, settings} from '../../wailsjs/go/models';
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
import {isActive, percent} from '../lib/downloads';
import {appendUnique, imageKey} from '../lib/images';
import {screenFilters, screenPixels, WailsScreen} from '../lib/screen';
import {useDownloads} from '../lib/useDownloads';
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
import {DownloadButton} from './DownloadButton';
import {Gallery} from './Gallery';
import {PreviewDialog} from './PreviewDialog';
import {SavedSearchBar} from './SavedSearchBar';

interface Props {
    // The panel rechecks which results are in the library when it becomes
    // visible again, so images deleted in the library lose their mark.
    active: boolean;
}

export function SearchPanel({active}: Props) {
    const [available, setAvailable] = useState<SourceInfo[]>([]);
    const [selectedSources, setSelectedSources] = useState<string[]>([]);
    const [text, setText] = useState('');
    const [resolution, setResolution] = useState<Resolution>(anyResolution);
    const [orientation, setOrientation] = useState('');
    // Filters are saved only after the remembered ones were loaded, so the
    // defaults shown during startup never overwrite them.
    const [filtersLoaded, setFiltersLoaded] = useState(false);

    const [saved, setSaved] = useState<main.SavedSearch[]>([]);
    // The saved search shown in the form, until the form is changed.
    const [activeSaved, setActiveSaved] = useState<number | null>(null);

    const [results, setResults] = useState<images.Image[]>([]);
    // The query of the search on screen, reused by "Load more".
    const [activeQuery, setActiveQuery] = useState<providers.Query | null>(null);
    const [statuses, setStatuses] = useState<SourceStatus[]>([]);
    const [selected, setSelected] = useState<number | null>(null);
    const [error, setError] = useState<BackendError | null>(null);
    const [loading, setLoading] = useState<'search' | 'more' | null>(null);
    // Only the latest request may update the UI.
    const latestRequest = useRef(0);

    const downloads = useDownloads();
    // Library entry IDs of shown images that are already downloaded.
    const [inLibrary, setInLibrary] = useState<Record<string, number>>({});
    // Errors from starting a download, by image key.
    const [startErrors, setStartErrors] = useState<Record<string, BackendError>>({});

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
                if (!size) return;
                // A wallpaper must match the screen's shape as well as its size.
                const filters = screenFilters(size);
                setActiveSaved(null);
                setResolution({width: filters.width, height: filters.height});
                setOrientation(filters.orientation);
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
                checkLibrary(result.images.map(imageKey));
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
        startSearch(text, selectedSources, resolution, orientation);
    }

    // startSearch runs a search with explicit values, so a saved search can
    // run before the form state has been updated.
    function startSearch(searchText: string, sources: string[], res: Resolution, orient: string) {
        const query = new providers.Query({
            text: searchText,
            page: 1,
            minWidth: res.width,
            minHeight: res.height,
            orientation: orient,
        });
        setResults([]);
        setStatuses([]);
        setSelected(null);
        setActiveQuery(query);
        load(sources.map((providerId) => ({providerId, page: 1})), query, false);
    }

    useEffect(() => {
        ListSavedSearches()
            .then(setSaved)
            .catch(() => {}); // searching works without saved searches
    }, []);

    function refreshSaved() {
        ListSavedSearches()
            .then(setSaved)
            .catch((err: unknown) => setError(toBackendError(err)));
    }

    // runSaved fills the form with a saved search and runs it. Sources that
    // no longer exist are skipped.
    function runSaved(s: main.SavedSearch) {
        const known = new Set(available.map((p) => p.id));
        const sources = (s.query.providerIds ?? []).filter((id) => known.has(id));
        if (sources.length === 0) {
            setError({code: 'invalid_input', message: 'None of the sources of this saved search are available.'});
            return;
        }
        const res = {width: s.query.minWidth, height: s.query.minHeight};
        setText(s.query.text);
        setSelectedSources(sources);
        setResolution(res);
        setOrientation(s.query.orientation);
        setActiveSaved(s.id);
        startSearch(s.query.text, sources, res, s.query.orientation);
    }

    async function saveCurrent(name: string) {
        const created = await SaveSearch(
            name,
            new searches.Query({
                text,
                providerIds: selectedSources,
                minWidth: resolution.width,
                minHeight: resolution.height,
                orientation,
            }),
        );
        refreshSaved();
        setActiveSaved(created.id);
    }

    // Changing the form means it no longer shows the saved search.
    function edited<T>(set: (value: T) => void) {
        return (value: T) => {
            setActiveSaved(null);
            set(value);
        };
    }

    function loadMore() {
        const pages = nextPages(statuses);
        if (activeQuery && pages.length > 0) load(pages, activeQuery, true);
    }

    // checkLibrary marks which of the given images are already downloaded.
    // It only decorates results, so a failure is not shown.
    function checkLibrary(keys: string[]) {
        if (keys.length === 0) return;
        LibraryStatus(keys)
            .then((found) => setInLibrary((current) => ({...current, ...found})))
            .catch(() => {});
    }

    function startDownload(img: images.Image) {
        const key = imageKey(img);
        setStartErrors(({[key]: _, ...rest}) => rest);
        Download(img).catch((err: unknown) => {
            const backendError = toBackendError(err);
            if (backendError.code === 'already_in_library') {
                checkLibrary([key]);
            } else {
                setStartErrors((current) => ({...current, [key]: backendError}));
            }
        });
    }

    useEffect(() => {
        if (!active || results.length === 0) return;
        LibraryStatus(results.map(imageKey))
            .then((found) => setInLibrary(found))
            .catch(() => {});
        // Only on becoming visible; new results are checked when they arrive.
    }, [active]);

    const savedToLibrary = (key: string) =>
        key in inLibrary || downloads[key]?.state === 'done' || downloads[key]?.state === 'duplicate';
    const galleryPercent = (key: string) => {
        const p = downloads[key];
        return isActive(p) ? percent(p) : undefined;
    };

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
            <SavedSearchBar
                saved={saved}
                activeID={activeSaved}
                suggestedName={text.trim() || 'Popular images'}
                canSave={selectedSources.length > 0}
                onRun={runSaved}
                onSave={saveCurrent}
                onRename={async (id, name) => {
                    await RenameSavedSearch(id, name);
                    refreshSaved();
                }}
                onDelete={(id) => {
                    DeleteSavedSearch(id)
                        .then(() => {
                            setActiveSaved(null);
                            refreshSaved();
                        })
                        .catch((err: unknown) => setError(toBackendError(err)));
                }}
            />

            <form className="search-form" onSubmit={runSearch} role="search">
                <label className="search-text">
                    <span className="sr-only">Search text</span>
                    <input
                        type="search"
                        value={text}
                        onChange={(e) => edited(setText)(e.target.value)}
                        placeholder="Misty mountains, Japanese village…  (-word excludes)"
                        maxLength={256}
                    />
                </label>
                <label>
                    Minimum size
                    <select
                        value={resolutionKey(resolution)}
                        onChange={(e) => edited(setResolution)(parseResolutionKey(e.target.value))}
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
                    <select value={orientation} onChange={(e) => edited(setOrientation)(e.target.value)}>
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
                                onChange={() => {
                                    setActiveSaved(null);
                                    setSelectedSources((s) => toggleSource(available, s, p.id));
                                }}
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
                {!activeQuery && loading === null && !error && selectedSources.length > 0 && (
                    <div className="search-intro muted">
                        <p>
                            Describe what you would like to see, for example <q>misty mountains -snow</q>, and press
                            Search. A word with a minus in front is left out.
                        </p>
                        <p>Search without words to see popular images.</p>
                    </div>
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

            <Gallery
                items={results}
                sourceName={sourceName}
                inLibrary={savedToLibrary}
                downloadPercent={galleryPercent}
                onSelect={setSelected}
            />

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
                    actions={
                        <DownloadButton
                            progress={downloads[imageKey(selectedImage)]}
                            inLibrary={imageKey(selectedImage) in inLibrary}
                            startError={startErrors[imageKey(selectedImage)]}
                            onDownload={() => startDownload(selectedImage)}
                            onCancel={() => {
                                CancelDownload(imageKey(selectedImage)).catch(() => {});
                            }}
                        />
                    }
                    onClose={() => setSelected(null)}
                    onPrevious={selected > 0 ? () => setSelected(selected - 1) : undefined}
                    onNext={selected < results.length - 1 ? () => setSelected(selected + 1) : undefined}
                />
            )}
        </section>
    );
}
