import {KeyboardEvent, useEffect, useLayoutEffect, useRef, useState} from 'react';
import './App.css';
import {GetStatus, ListProviders, SaveDisplaySize, TakeAboutRequest} from '../wailsjs/go/main/App';
import {BrowserOpenURL, EventsOn, ScreenGetAll} from '../wailsjs/runtime/runtime';
import {keyScrollTarget} from './lib/scroll';
import {largestScreen, WailsScreen} from './lib/screen';
import {useUpdate} from './lib/useUpdate';
import {main, search} from '../wailsjs/go/models';
import {BackendError, describeError, toBackendError} from './lib/backend';
import {SearchPanel} from './components/SearchPanel';
import {LibraryView} from './components/LibraryView';
import {WallpaperView} from './components/WallpaperView';
import {AboutDialog} from './components/AboutDialog';

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
    const [showAbout, setShowAbout] = useState(false);
    const update = useUpdate();

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
        // The tray's "About Kisetsu" opens the window with About, or shows
        // it in the open window.
        TakeAboutRequest()
            .then((show) => show && setShowAbout(true))
            .catch(() => {});
        return EventsOn('about:show', () => setShowAbout(true));
    }, []);

    // Images from the user's own folders have no search source (ADR 0053).
    const sourceName = (id: string) =>
        id === 'folder' ? 'Local folder' : (sources.find((p) => p.id === id)?.name ?? id);

    // Every tab keeps its own scroll position. It is stored when leaving the
    // tab, because an element that is not shown may lose it.
    const scrollTops = useRef<Partial<Record<Tab, number>>>({});
    function showTab(next: Tab) {
        scrollTops.current[tab] = document.getElementById(`panel-${tab}`)?.scrollTop ?? 0;
        setTab(next);
    }
    useLayoutEffect(() => {
        const panel = document.getElementById(`panel-${tab}`);
        if (panel) panel.scrollTop = scrollTops.current[tab] ?? 0;
    }, [tab]);

    // With the focus outside the panel (on a tab, or nowhere yet), the
    // browser would not know what to scroll.
    useEffect(() => {
        function onKey(event: globalThis.KeyboardEvent) {
            if (event.defaultPrevented || event.ctrlKey || event.altKey || event.metaKey || event.shiftKey) return;
            const panel = document.getElementById(`panel-${tab}`);
            if (!panel || panel.contains(event.target as Node) || document.querySelector('dialog[open]')) return;
            const top = keyScrollTarget(event.key, panel);
            if (top === null) return;
            event.preventDefault();
            panel.scrollTop = top;
        }
        document.addEventListener('keydown', onKey);
        return () => document.removeEventListener('keydown', onKey);
    }, [tab]);

    const panelProps = (id: Tab) => ({
        id: `panel-${id}`,
        className: 'tab-panel',
        role: 'tabpanel',
        tabIndex: 0,
        'aria-labelledby': `tab-${id}`,
        hidden: tab !== id,
    });

    // Arrow keys move between tabs, as in the WAI-ARIA tabs pattern.
    function onTabKey(event: KeyboardEvent) {
        if (event.key !== 'ArrowLeft' && event.key !== 'ArrowRight') return;
        const index = tabs.findIndex((t) => t.id === tab);
        const next = tabs[(index + (event.key === 'ArrowRight' ? 1 : tabs.length - 1)) % tabs.length];
        showTab(next.id);
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
                            onClick={() => t.id !== tab && showTab(t.id)}
                        >
                            {t.label}
                        </button>
                    ))}
                </nav>
            </header>

            {/* Both views stay mounted, so switching keeps search results. */}
            <main className="app-main">
                <div {...panelProps('search')}>
                    <SearchPanel active={tab === 'search'}/>
                </div>
                <div {...panelProps('library')}>
                    <LibraryView active={tab === 'library'} sourceName={sourceName}/>
                </div>
                <div {...panelProps('wallpaper')}>
                    <WallpaperView active={tab === 'wallpaper'}/>
                </div>
            </main>

            <footer className="app-footer" aria-live="polite">
                {error && <span className="error">{describeError(error)}</span>}
                {status && (
                    // Details for bug reports stay available on hover.
                    <button
                        type="button"
                        className="footer-about"
                        title={`About Kisetsu · ${status.app.platform} · database schema v${status.database.schemaVersion} · ${status.database.path}`}
                        onClick={() => setShowAbout(true)}
                    >
                        {status.app.name} {status.app.version}
                    </button>
                )}
                {update?.available && (
                    <button
                        type="button"
                        className="footer-about footer-update"
                        title="Opens the release page in your browser"
                        onClick={() => BrowserOpenURL(update.url)}
                    >
                        Version {update.available} is available{update.viaSystem && " in your system's updates"}
                    </button>
                )}
            </footer>

            {showAbout && <AboutDialog onClose={() => setShowAbout(false)}/>}
        </div>
    );
}

export default App;
