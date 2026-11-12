import {useCallback, useEffect, useState} from 'react';
import {
    GetRotation,
    ListCollections,
    ListFolders,
    ListSavedSearches,
    NextWallpaper,
    PreviousWallpaper,
    RepairAgent,
    SaveRotation,
    ChooseTextFile,
    SaveWallpaperText,
    SetShowInTray,
    StartAgent,
} from '../../wailsjs/go/main/App';
import {main} from '../../wailsjs/go/models';
import {BackendError, describeError, toBackendError} from '../lib/backend';
import {collectionCount} from '../lib/collections';
import {
    clockSizes,
    clockTime,
    corners,
    effectLevels,
    intervalLabel,
    parseSourceKey,
    relativeTime,
    sourceKey,
    tones,
} from '../lib/rotation';

const refreshInterval = 30_000;

interface Props {
    // The view refreshes while it is visible.
    active: boolean;
}

// WallpaperView sets up automatic wallpaper rotation. Every change is
// applied at once; the background agent picks it up.
export function WallpaperView({active}: Props) {
    const [state, setState] = useState<main.RotationState | null>(null);
    const [collections, setCollections] = useState<main.CollectionInfo[]>([]);
    const [savedSearches, setSavedSearches] = useState<main.SavedSearch[]>([]);
    const [folders, setFolders] = useState<main.FolderInfo[]>([]);
    const [busy, setBusy] = useState<'save' | 'next' | 'previous' | null>(null);
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
        ListFolders()
            .then(setFolders)
            .catch(() => {});
        const timer = window.setInterval(() => {
            refresh();
            setTick((t) => t + 1);
        }, refreshInterval);
        return () => window.clearInterval(timer);
    }, [active, refresh]);

    function apply(action: () => Promise<main.RotationState>, kind: 'save' | 'next' | 'previous') {
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

    function saveText(change: Partial<main.WallpaperText>) {
        if (!state) return;
        const next = new main.WallpaperText({...state.text, ...change});
        apply(() => SaveWallpaperText(next), 'save');
    }

    // chooseTextFile asks for the file and turns the text on with it.
    function chooseTextFile() {
        ChooseTextFile()
            .then((path) => {
                if (path) saveText({textFilePath: path, textFile: true});
            })
            .catch((err: unknown) => setError(toBackendError(err)));
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
    const folderMissing = s.folderId !== 0 && !folders.some((f) => f.id === s.folderId);
    const fromSearch = s.savedSearchId !== 0 && !searchMissing;
    // A saved search downloads images when it has none, so it can always try.
    const nothingToShow = state.sourceImages === 0 && !fromSearch;

    return (
        <section className="wallpaper" aria-busy={busy !== null}>
            <h2>Wallpaper</h2>

            {error && (
                <p className="error" role="alert">
                    {describeError(error)}
                </p>
            )}

            <div className="wallpaper-now">
                <button
                    type="button"
                    className="btn secondary"
                    onClick={() => apply(PreviousWallpaper, 'previous')}
                    disabled={busy !== null || !state.lastChange}
                    title="Go back to the wallpaper before this one"
                >
                    {busy === 'previous' ? 'Changing…' : 'Previous'}
                </button>
                <button
                    type="button"
                    className="btn"
                    onClick={() => apply(NextWallpaper, 'next')}
                    disabled={busy !== null || nothingToShow}
                    aria-describedby="wallpaper-now-note"
                >
                    {busy === 'next' ? 'Changing…' : 'Next wallpaper'}
                </button>
                <p className="muted" id="wallpaper-now-note">
                    {nothingToShow
                        ? s.collectionId !== 0 && !collectionMissing
                            ? 'This collection is empty. Add images to it in the Library tab, or choose other images below.'
                            : 'There are no images yet. Download some in the Search tab, or choose a saved search below.'
                        : state.lastChange
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
                                <option key={c.id} value={sourceKey({collectionId: c.id})}>
                                    {c.name} ({collectionCount(c.images)})
                                </option>
                            ))}
                            {collectionMissing && (
                                <option value={sourceKey(s)}>Deleted collection (whole library is used)</option>
                            )}
                        </optgroup>
                        <optgroup label="New images from a saved search">
                            {savedSearches.map((x) => (
                                <option key={x.id} value={sourceKey({savedSearchId: x.id})}>
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
                        <optgroup label="Your folders">
                            {folders.map((f) => (
                                <option key={f.id} value={sourceKey({folderId: f.id})}>
                                    {f.name} ({collectionCount(f.images)})
                                </option>
                            ))}
                            {folderMissing && (
                                <option value={sourceKey(s)}>Removed folder (whole library is used)</option>
                            )}
                            {folders.length === 0 && !folderMissing && (
                                <option disabled value="no-folder">Add a folder in the Library tab first</option>
                            )}
                        </optgroup>
                    </select>
                </label>
                <p className="muted small">
                    {fromSearch
                        ? `${collectionCount(state.sourceImages)} fetched so far. When fewer than 3 are left unshown, ` +
                          'Kisetsu downloads new ones at screen size before changing the wallpaper.'
                        : s.folderId !== 0 && !folderMissing
                          ? `${collectionCount(state.sourceImages)} to choose from. Kisetsu reads the folder again ` +
                            'before each change, so new images take part.'
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
                {s.enabled && state.agent.running && state.nextChange && (
                    <p>
                        Next change at {clockTime(state.nextChange)} ({relativeTime(state.nextChange)}).
                    </p>
                )}
                {(s.enabled || state.text.clock || state.showInTray) && !state.agent.running && !state.agent.error && (
                    <div className="warning" role="alert">
                        <p>
                            {s.enabled
                                ? 'Rotation is on, but the background process that changes the wallpaper is not running.'
                                : state.text.clock
                                  ? 'The clock is on, but the background process that redraws it is not running.'
                                  : 'The background process that shows the tray icon is not running.'}
                        </p>
                        <button
                            type="button"
                            className="btn secondary"
                            onClick={() => apply(StartAgent, 'save')}
                            disabled={busy !== null}
                        >
                            Start it
                        </button>
                    </div>
                )}
                {state.agent.error && <p className="error">{state.agent.error}</p>}
                {!s.enabled && <p className="muted">Rotation is off.</p>}
                <p className="muted small">
                    Rotation runs in a small background process, also while this window is closed. It starts when you
                    log in.
                </p>
            </div>

            <fieldset className="wallpaper-text" disabled={busy === 'save'}>
                <legend>On the wallpaper</legend>
                <label className="toggle">
                    <input
                        type="checkbox"
                        checked={state.text.credit}
                        onChange={(e) => saveText({credit: e.target.checked})}
                    />
                    <span>Show the author, license and source</span>
                </label>
                <label>
                    Corner
                    <select
                        value={state.text.creditCorner}
                        onChange={(e) => saveText({creditCorner: e.target.value})}
                        disabled={!state.text.credit}
                    >
                        {corners.map((c) => (
                            <option key={c.value} value={c.value}>
                                {c.label}
                            </option>
                        ))}
                    </select>
                </label>

                <label className="toggle">
                    <input
                        type="checkbox"
                        checked={state.text.clock}
                        onChange={(e) => saveText({clock: e.target.checked})}
                    />
                    <span>Show a clock</span>
                </label>
                <div className="clock-options">
                    <label>
                        Corner
                        <select
                            value={state.text.clockCorner}
                            onChange={(e) => saveText({clockCorner: e.target.value})}
                            disabled={!state.text.clock}
                        >
                            {corners.map((c) => (
                                <option key={c.value} value={c.value}>
                                    {c.label}
                                </option>
                            ))}
                        </select>
                    </label>
                    <label>
                        Time
                        <select
                            value={state.text.clock12h ? '12h' : '24h'}
                            onChange={(e) => saveText({clock12h: e.target.value === '12h'})}
                            disabled={!state.text.clock}
                        >
                            <option value="24h">24-hour (15:04)</option>
                            <option value="12h">12-hour (3:04 PM)</option>
                        </select>
                    </label>
                    <label>
                        Size
                        <select
                            value={state.text.clockSize}
                            onChange={(e) => saveText({clockSize: e.target.value})}
                            disabled={!state.text.clock}
                        >
                            {clockSizes.map((c) => (
                                <option key={c.value} value={c.value}>
                                    {c.label}
                                </option>
                            ))}
                        </select>
                    </label>
                </div>
                <label className="toggle">
                    <input
                        type="checkbox"
                        checked={state.text.clockDate}
                        onChange={(e) => saveText({clockDate: e.target.checked})}
                        disabled={!state.text.clock}
                    />
                    <span>Show the date under the time</span>
                </label>

                <p className="muted small">
                    The background process redraws the clock every minute. While it is on, Kisetsu keeps showing the
                    wallpaper it set last.
                </p>

                <label className="toggle">
                    <input
                        type="checkbox"
                        checked={state.text.textFile}
                        disabled={!state.text.textFilePath}
                        onChange={(e) => saveText({textFile: e.target.checked})}
                    />
                    <span>Show text from a file</span>
                </label>
                <div className="clock-options">
                    <p className="text-file-path">
                        {state.text.textFilePath ? <code>{state.text.textFilePath}</code> : 'No file chosen.'}{' '}
                        <button type="button" className="btn secondary" onClick={chooseTextFile}>
                            Choose file…
                        </button>
                    </p>
                    <label>
                        Corner
                        <select
                            value={state.text.textFileCorner}
                            onChange={(e) => saveText({textFileCorner: e.target.value})}
                            disabled={!state.text.textFile}
                        >
                            {corners.map((c) => (
                                <option key={c.value} value={c.value}>
                                    {c.label}
                                </option>
                            ))}
                        </select>
                    </label>
                    <label>
                        Size
                        <select
                            value={state.text.textFileSize}
                            onChange={(e) => saveText({textFileSize: e.target.value})}
                            disabled={!state.text.textFile}
                        >
                            {clockSizes.map((c) => (
                                <option key={c.value} value={c.value}>
                                    {c.label}
                                </option>
                            ))}
                        </select>
                    </label>
                </div>
                <p className="muted small">
                    Up to 10 lines of the file, updated when it changes. Whatever is in it will be on your desktop,
                    also in screenshots and screen sharing: do not choose a file with passwords or private notes.
                    Kisetsu never runs commands; to show the output of one (for example <code>df -h</code>), let
                    your own scheduled task write it to the file. The file must be yours and writable only by you.
                </p>

                <h3 className="wallpaper-text-heading">Effects</h3>
                <div className="clock-options">
                    <label>
                        Darken
                        <select value={state.text.darken} onChange={(e) => saveText({darken: e.target.value})}>
                            {effectLevels.map((l) => (
                                <option key={l.value} value={l.value}>
                                    {l.label}
                                </option>
                            ))}
                        </select>
                    </label>
                    <label>
                        Blur
                        <select value={state.text.blur} onChange={(e) => saveText({blur: e.target.value})}>
                            {effectLevels.map((l) => (
                                <option key={l.value} value={l.value}>
                                    {l.label}
                                </option>
                            ))}
                        </select>
                    </label>
                    <label>
                        Colors
                        <select value={state.text.tone} onChange={(e) => saveText({tone: e.target.value})}>
                            {tones.map((t) => (
                                <option key={t.value} value={t.value}>
                                    {t.label}
                                </option>
                            ))}
                        </select>
                    </label>
                </div>
                <label className="toggle">
                    <input
                        type="checkbox"
                        checked={state.text.vignette}
                        onChange={(e) => saveText({vignette: e.target.checked})}
                    />
                    <span>Darker edges (vignette)</span>
                </label>
                <p className="muted small">
                    Effects make text and desktop icons easier to read on bright or busy images.
                </p>

                <p className="muted small">
                    Kisetsu draws the text and effects on a copy of the image, cropped to your screen; the image in
                    your library is not changed.
                    {state.varietyRunning &&
                        ' While Variety is running, images you choose with “Set as wallpaper” go to Variety, without text.'}
                </p>
            </fieldset>

            <fieldset className="tray" disabled={busy === 'save'}>
                <legend>Tray icon</legend>
                <label className="toggle">
                    <input
                        type="checkbox"
                        checked={state.showInTray}
                        onChange={(e) => {
                            const show = e.target.checked;
                            apply(() => SetShowInTray(show), 'save');
                        }}
                    />
                    <span>Show Kisetsu in the system tray</span>
                </label>
                <p className="muted small">
                    The icon’s menu changes the wallpaper, turns rotation on or off and opens this window or About Kisetsu.
                </p>
                {state.showInTray && !state.trayHost && (
                    <p className="warning" role="note">
                        Your desktop does not show tray icons. On GNOME, turn on the extension “AppIndicator and
                        KStatusNotifierItem Support” (Ubuntu has it on by default). Rotation works either way.
                    </p>
                )}
            </fieldset>

            {state.agent.outdated && (
                <div className="warning" role="alert">
                    <p>
                        The background process starts a different copy of Kisetsu than this one (it was moved or
                        rebuilt).
                    </p>
                    <button
                        type="button"
                        className="btn secondary"
                        onClick={() => apply(RepairAgent, 'save')}
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
