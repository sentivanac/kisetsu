import {useState} from 'react';
import {main} from '../../wailsjs/go/models';
import {NameForm} from './NameForm';

interface Props {
    saved: main.SavedSearch[];
    // The saved search last run from this bar, if the form still shows it.
    activeID: number | null;
    // Suggested name for saving the current search.
    suggestedName: string;
    canSave: boolean;
    onRun: (saved: main.SavedSearch) => void;
    onSave: (name: string) => Promise<void>;
    onRename: (id: number, name: string) => Promise<void>;
    onDelete: (id: number) => void;
}

type Mode = 'idle' | 'saving' | 'renaming' | 'confirmDelete';

// SavedSearchBar runs, saves and manages saved searches.
export function SavedSearchBar({saved, activeID, suggestedName, canSave, onRun, onSave, onRename, onDelete}: Props) {
    const [mode, setMode] = useState<Mode>('idle');
    const active = saved.find((s) => s.id === activeID);

    return (
        <div className="saved-searches">
            <div className="collection-list" role="group" aria-label="Saved searches">
                {saved.length > 0 && <span className="muted small">Saved:</span>}
                {saved.map((s) => (
                    <button
                        key={s.id}
                        type="button"
                        className="chip"
                        aria-pressed={s.id === activeID}
                        title={s.query.text || 'Popular images'}
                        onClick={() => {
                            setMode('idle');
                            onRun(s);
                        }}
                    >
                        {s.name}
                    </button>
                ))}
                {mode !== 'saving' && (
                    <button type="button" className="chip chip-add" onClick={() => setMode('saving')} disabled={!canSave}>
                        + Save this search
                    </button>
                )}
            </div>

            {mode === 'saving' && (
                <NameForm
                    label="Name for this search"
                    submitLabel="Save"
                    initial={suggestedName}
                    existing={saved}
                    what="saved search"
                    onSubmit={(name) => onSave(name).then(() => setMode('idle'))}
                    onCancel={() => setMode('idle')}
                />
            )}

            {active && mode === 'idle' && (
                <div className="collection-tools">
                    <button type="button" className="btn secondary" onClick={() => setMode('renaming')}>
                        Rename
                    </button>
                    <button type="button" className="btn secondary" onClick={() => setMode('confirmDelete')}>
                        Delete saved search…
                    </button>
                </div>
            )}
            {active && mode === 'renaming' && (
                <NameForm
                    label={`New name for ${active.name}`}
                    submitLabel="Rename"
                    initial={active.name}
                    existing={saved}
                    ownID={active.id}
                    what="saved search"
                    onSubmit={(name) => onRename(active.id, name).then(() => setMode('idle'))}
                    onCancel={() => setMode('idle')}
                />
            )}
            {active && mode === 'confirmDelete' && (
                <div className="confirm" role="group" aria-label="Confirm deletion">
                    <p>Delete the saved search “{active.name}”? Your library is not affected.</p>
                    <div className="download-row">
                        <button
                            type="button"
                            className="btn danger"
                            onClick={() => {
                                setMode('idle');
                                onDelete(active.id);
                            }}
                        >
                            Delete saved search
                        </button>
                        <button type="button" className="btn secondary" onClick={() => setMode('idle')} autoFocus>
                            Keep
                        </button>
                    </div>
                </div>
            )}
        </div>
    );
}
