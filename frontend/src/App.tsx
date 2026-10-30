import {useEffect, useState} from 'react';
import './App.css';
import {GetStatus} from '../wailsjs/go/main/App';
import {main} from '../wailsjs/go/models';
import {BackendError, describeError, toBackendError} from './lib/backend';
import {SearchPanel} from './components/SearchPanel';

function App() {
    const [status, setStatus] = useState<main.Status | null>(null);
    const [error, setError] = useState<BackendError | null>(null);

    useEffect(() => {
        GetStatus()
            .then(setStatus)
            .catch((err: unknown) => setError(toBackendError(err)));
    }, []);

    return (
        <div className="app">
            <header className="app-header">
                <h1 className="title">
                    Kisetsu <span lang="ja">季節</span>
                </h1>
                <p className="tagline">Wallpaper discovery for Linux</p>
            </header>

            <main className="app-main">
                <SearchPanel/>
            </main>

            <footer className="app-footer" aria-live="polite">
                {error && <span className="error">{describeError(error)}</span>}
                {status && (
                    <span title={status.database.path}>
                        {status.app.name} {status.app.version} · {status.app.platform} · database schema v
                        {status.database.schemaVersion}
                    </span>
                )}
            </footer>
        </div>
    );
}

export default App;
