import {useCallback, useEffect, useState} from 'react';
import './App.css';
import {GetAppInfo} from '../wailsjs/go/main/App';
import {appinfo} from '../wailsjs/go/models';

function App() {
    const [info, setInfo] = useState<appinfo.Info | null>(null);
    const [error, setError] = useState<string | null>(null);

    const loadInfo = useCallback(() => {
        setError(null);
        GetAppInfo()
            .then(setInfo)
            .catch((err: unknown) => setError(String(err)));
    }, []);

    useEffect(loadInfo, [loadInfo]);

    return (
        <main className="app">
            <h1 className="title">
                Kisetsu <span lang="ja">季節</span>
            </h1>
            <p className="tagline">Wallpaper discovery for Linux</p>

            <section className="card" aria-live="polite">
                <h2>Backend status</h2>
                {error && <p className="error">Go backend unavailable: {error}</p>}
                {!error && !info && <p>Connecting to Go backend…</p>}
                {info && (
                    <dl className="info">
                        <dt>Version</dt>
                        <dd>{info.version}</dd>
                        <dt>Go</dt>
                        <dd>{info.goVersion}</dd>
                        <dt>Platform</dt>
                        <dd>{info.platform}</dd>
                    </dl>
                )}
                <button type="button" className="btn" onClick={loadInfo}>
                    Ask Go again
                </button>
            </section>
        </main>
    );
}

export default App;
