import {KeyboardEvent, useEffect, useState} from 'react';
import './App.css';
import {GetStatus, ListProviders, SaveDisplaySize} from '../wailsjs/go/main/App';
import {ScreenGetAll} from '../wailsjs/runtime/runtime';
import {largestScreen, WailsScreen} from './lib/screen';
import {main, search} from '../wailsjs/go/models';
import {BackendError, describeError, toBackendError} from './lib/backend';
import {SearchPanel} from './components/SearchPanel';
import {LibraryView} from './components/LibraryView';
import {WallpaperView} from './components/WallpaperView';

type Tab = 'search' | 'library' | 'wallpaper';

const tabs: {id: Tab; label: string}[] = [
    {id: 'search', label: 'Search'},
    {id: 'library', label: 'Library'},
    {id: 'wallpaper', label: 'Wallpaper'},
];

function App() {
    const [status, setStatus] = useState<main.Status | null>(null);
    const [error, setError] = useState<BackendError | null>(null);
    const [tab, setTab] = useState<Tab>('search');
    const [sources, setSources] = useState<search.ProviderInfo[]>([]);

    useEffect(() => {
        GetStatus()
            .then(setStatus)
            .catch((err: unknown) => setError(toBackendError(err)));
        ListProviders()
            .then(setSources)
            .catch(() => {}); // names fall back to provider IDs
        // The rotation timer runs without a window, so the screen size that
        // fetched images must cover is stored here.
        ScreenGetAll()
            .then((screens) => {
                const size = largestScreen(screens as WailsScreen[]);
                if (size) return SaveDisplaySize(size.width, size.height);
            })
            .catch(() => {}); // the default size is used instead
    }, []);

    const sourceName = (id: string) => sources.find((p) => p.id === id)?.name ?? id;

    // Arrow keys move between tabs, as in the WAI-ARIA tabs pattern.
    function onTabKey(event: KeyboardEvent) {
        if (event.key !== 'ArrowLeft' && event.key !== 'ArrowRight') return;
        const index = tabs.findIndex((t) => t.id === tab);
        const next = tabs[(index + (event.key === 'ArrowRight' ? 1 : tabs.length - 1)) % tabs.length];
        setTab(next.id);
        document.getElementById(`tab-${next.id}`)?.focus();
    }

    return (
        <div className="app">
            <header className="app-header">
                <h1 className="title">
                    Kisetsu <span lang="ja">季節</span>
                </h1>
                <nav className="tabs" role="tablist" aria-label="Views" onKeyDown={onTabKey}>
                    {tabs.map((t) => (
                        <button
                            key={t.id}
                            id={`tab-${t.id}`}
                            type="button"
                            role="tab"
                            aria-selected={tab === t.id}
                            aria-controls={`panel-${t.id}`}
                            tabIndex={tab === t.id ? 0 : -1}
                            className="tab"
                            onClick={() => setTab(t.id)}
                        >
                            {t.label}
                        </button>
                    ))}
                </nav>
            </header>

            {/* Both views stay mounted, so switching keeps search results. */}
            <main className="app-main">
                <div id="panel-search" role="tabpanel" aria-labelledby="tab-search" hidden={tab !== 'search'}>
                    <SearchPanel active={tab === 'search'}/>
                </div>
                <div id="panel-library" role="tabpanel" aria-labelledby="tab-library" hidden={tab !== 'library'}>
                    <LibraryView active={tab === 'library'} sourceName={sourceName}/>
                </div>
                <div id="panel-wallpaper" role="tabpanel" aria-labelledby="tab-wallpaper" hidden={tab !== 'wallpaper'}>
                    <WallpaperView active={tab === 'wallpaper'}/>
                </div>
            </main>

            <footer className="app-footer" aria-live="polite">
                {error && <span className="error">{describeError(error)}</span>}
                {status && (
                    // Details for bug reports stay available on hover.
                    <span
                        title={`${status.app.platform} · database schema v${status.database.schemaVersion} · ${status.database.path}`}
                    >
                        {status.app.name} {status.app.version}
                    </span>
                )}
            </footer>
        </div>
    );
}

export default App;
