import {useCallback, useEffect, useState} from 'react';
import {
    GetRotation,
    ListCollections,
    ListSavedSearches,
    NextWallpaper,
    RepairRotationTimer,
    SaveRotation,
} from '../../wailsjs/go/main/App';
import {main} from '../../wailsjs/go/models';
import {BackendError, describeError, toBackendError} from '../lib/backend';
import {collectionCount} from '../lib/collections';
import {clockTime, intervalLabel, parseSourceKey, relativeTime, sourceKey} from '../lib/rotation';

const refreshInterval = 30_000;

interface Props {
    // The view refreshes while it is visible.
    active: boolean;
}

// WallpaperView sets up automatic wallpaper rotation. Every change is
// applied at once: the timer is installed, updated or removed.
export function WallpaperView({active}: Props) {
    const [state, setState] = useState<main.RotationState | null>(null);
    const [collections, setCollections] = useState<main.CollectionInfo[]>([]);
    const [savedSearches, setSavedSearches] = useState<main.SavedSearch[]>([]);
    const [busy, setBusy] = useState<'save' | 'next' | null>(null);
    const [error, setError] = useState<BackendError | null>(null);
    const [, setTick] = useState(0); // re-renders relative times

    const refresh = useCallback(() => {
        GetRotation()
            .then(setState)
            .catch((err: unknown) => setError(toBackendError(err)));
    }, []);

    useEffect(() => {
        if (!active) return;
        refresh();
        ListCollections()
            .then(setCollections)
            .catch(() => {}); // the whole library is still offered
        ListSavedSearches()
            .then(setSavedSearches)
            .catch(() => {});
        const timer = window.setInterval(() => {
            refresh();
            setTick((t) => t + 1);
        }, refreshInterval);
        return () => window.clearInterval(timer);
    }, [active, refresh]);

    function apply(action: () => Promise<main.RotationState>, kind: 'save' | 'next') {
        setBusy(kind);
        setError(null);
        action()
            .then(setState)
            .catch((err: unknown) => {
                setError(toBackendError(err));
                refresh(); // show what is actually in effect
            })
            .finally(() => setBusy(null));
    }

    function save(change: Partial<main.RotationSettings>) {
        if (!state) return;
        const next = new main.RotationSettings({...state.settings, ...change});
        apply(() => SaveRotation(next), 'save');
    }

    if (!state) {
        return (
            <section className="wallpaper">
                <h2>Wallpaper</h2>
                {error ? <p className="error">{describeError(error)}</p> : <p className="muted">Loading…</p>}
            </section>
        );
    }

    const s = state.settings;
    const collectionMissing = s.collectionId !== 0 && !collections.some((c) => c.id === s.collectionId);
    const searchMissing = s.savedSearchId !== 0 && !savedSearches.some((x) => x.id === s.savedSearchId);
    const fromSearch = s.savedSearchId !== 0 && !searchMissing;

    return (
        <section className="wallpaper" aria-busy={busy !== null}>
            <h2>Wallpaper</h2>

            {error && (
                <p className="error" role="alert">
                    {describeError(error)}
                </p>
            )}

            <div className="wallpaper-now">
                <button type="button" className="btn" onClick={() => apply(NextWallpaper, 'next')} disabled={busy !== null}>
                    {busy === 'next' ? 'Changing…' : 'Next wallpaper'}
                </button>
                <p className="muted">
                    {state.lastChange
                        ? `Last changed ${relativeTime(state.lastChange)}.`
                        : 'Kisetsu has not changed the wallpaper yet.'}
                </p>
            </div>

            <fieldset className="rotation" disabled={busy === 'save'}>
                <legend>Automatic rotation</legend>

                <label className="toggle">
                    <input type="checkbox" checked={s.enabled} onChange={(e) => save({enabled: e.target.checked})}/>
                    <span>Change the wallpaper automatically</span>
                </label>

                <label>
                    How often
                    <select value={s.interval} onChange={(e) => save({interval: e.target.value})}>
                        {state.intervals.map((i) => (
                            <option key={i} value={i}>
                                {intervalLabel(i)}
                            </option>
                        ))}
                    </select>
                </label>

                <label>
                    Images from
                    <select
                        value={sourceKey(s)}
                        onChange={(e) => save(parseSourceKey(e.target.value))}
                    >
                        <optgroup label="Your library">
                            <option value="library">Whole library</option>
                            {collections.map((c) => (
                                <option key={c.id} value={sourceKey({collectionId: c.id, savedSearchId: 0})}>
                                    {c.name} ({collectionCount(c.images)})
                                </option>
                            ))}
                            {collectionMissing && (
                                <option value={sourceKey(s)}>Deleted collection (whole library is used)</option>
                            )}
                        </optgroup>
                        <optgroup label="New images from a saved search">
                            {savedSearches.map((x) => (
                                <option key={x.id} value={sourceKey({collectionId: 0, savedSearchId: x.id})}>
                                    {x.name}
                                </option>
                            ))}
                            {searchMissing && (
                                <option value={sourceKey(s)}>Deleted saved search (whole library is used)</option>
                            )}
                            {savedSearches.length === 0 && !searchMissing && (
                                <option disabled value="none">Save a search in the Search tab first</option>
                            )}
                        </optgroup>
                    </select>
                </label>
                <p className="muted small">
                    {fromSearch
                        ? `${collectionCount(state.sourceImages)} fetched so far. When fewer than 3 are left unshown, ` +
                          'Kisetsu downloads new ones at screen size before changing the wallpaper.'
                        : `${collectionCount(state.sourceImages)} to choose from.`}
                </p>

                <div role="radiogroup" aria-label="Order" className="order">
                    <label className="toggle">
                        <input
                            type="radio"
                            name="order"
                            checked={s.order === 'random'}
                            onChange={() => save({order: 'random'})}
                        />
                        <span>Random, each image once before any repeats</span>
                    </label>
                    <label className="toggle">
                        <input
                            type="radio"
                            name="order"
                            checked={s.order === 'sequential'}
                            onChange={() => save({order: 'sequential'})}
                        />
                        <span>In the order they were added</span>
                    </label>
                </div>
            </fieldset>

            <div className="rotation-status" aria-live="polite">
                {s.enabled && state.timer.active && state.timer.nextRun && (
                    <p>
                        Next change at {clockTime(state.timer.nextRun)} ({relativeTime(state.timer.nextRun)}).
                    </p>
                )}
                {s.enabled && !state.timer.active && !state.timer.error && (
                    <p className="error">Rotation is on, but its timer is not running. Turn rotation off and on again.</p>
                )}
                {state.timer.error && <p className="error">{state.timer.error}</p>}
                {!s.enabled && <p className="muted">Rotation is off.</p>}
                <p className="muted small">
                    Rotation runs as a systemd user timer, also while this window is closed, and stops when you log out.
                </p>
            </div>

            {state.timer.outdated && (
                <div className="warning" role="alert">
                    <p>The rotation timer starts a different copy of Kisetsu than this one (it was moved or rebuilt).</p>
                    <button
                        type="button"
                        className="btn secondary"
                        onClick={() => apply(RepairRotationTimer, 'save')}
                        disabled={busy !== null}
                    >
                        Use this copy
                    </button>
                </div>
            )}

            {state.varietyRunning && (
                <div className="warning" role="note">
                    <p>
                        Variety is running and changes the wallpaper too. To let Kisetsu rotate, turn off Variety’s
                        automatic change (Variety preferences → “Change wallpaper every…”), or quit Variety.
                    </p>
                </div>
            )}
        </section>
    );
}
