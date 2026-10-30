import {useCallback, useEffect, useState} from 'react';
import './App.css';
import {GetStatus} from '../wailsjs/go/main/App';
import {main} from '../wailsjs/go/models';
import {BackendError, toBackendError} from './lib/backend';

function App() {
    const [status, setStatus] = useState<main.Status | null>(null);
    const [error, setError] = useState<BackendError | null>(null);
    const [loading, setLoading] = useState(false);

    const loadStatus = useCallback(() => {
        setLoading(true);
        setError(null);
        GetStatus()
            .then(setStatus)
            .catch((err: unknown) => {
                setStatus(null);
                setError(toBackendError(err));
            })
            .finally(() => setLoading(false));
    }, []);

    useEffect(loadStatus, [loadStatus]);

    return (
        <main className="app">
            <h1 className="title">
                Kisetsu <span lang="ja">季節</span>
            </h1>
            <p className="tagline">Wallpaper discovery for Linux</p>

            <section className="card" aria-live="polite" aria-busy={loading}>
                <h2>Backend status</h2>
                {error && (
                    <p className="error" role="alert">
                        {error.message}
                    </p>
                )}
                {!error && !status && <p>Connecting to Go backend…</p>}
                {status && (
                    <dl className="info">
                        <dt>Version</dt>
                        <dd>{status.app.version}</dd>
                        <dt>Go</dt>
                        <dd>{status.app.goVersion}</dd>
                        <dt>Platform</dt>
                        <dd>{status.app.platform}</dd>
                        <dt>Schema</dt>
                        <dd>v{status.database.schemaVersion}</dd>
                        <dt>Database</dt>
                        <dd className="path">{status.database.path}</dd>
                    </dl>
                )}
                <button type="button" className="btn" onClick={loadStatus} disabled={loading}>
                    {loading ? 'Checking…' : 'Check again'}
                </button>
            </section>
        </main>
    );
}

export default App;
