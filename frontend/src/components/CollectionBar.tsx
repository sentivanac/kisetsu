import {useState} from 'react';
import {main} from '../../wailsjs/go/models';
import {collectionCount} from '../lib/collections';
import {NameForm} from './NameForm';

interface Props {
    collections: main.CollectionInfo[];
    selected: number; // 0 = all images
    totalImages: number;
    onSelect: (id: number) => void;
    onCreate: (name: string) => Promise<void>;
    onRename: (id: number, name: string) => Promise<void>;
    onDelete: (id: number) => void;
}

type Mode = 'idle' | 'creating' | 'renaming' | 'confirmDelete';

// CollectionBar picks the collection shown in the library and manages
// collections.
export function CollectionBar({collections, selected, totalImages, onSelect, onCreate, onRename, onDelete}: Props) {
    const [mode, setMode] = useState<Mode>('idle');
    const current = collections.find((c) => c.id === selected);

    return (
        <div className="collections">
            <div className="collection-list" role="group" aria-label="Collections">
                <button
                    type="button"
                    className="chip"
                    aria-pressed={selected === 0}
                    onClick={() => {
                        setMode('idle');
                        onSelect(0);
                    }}
                >
                    All images <span className="chip-count">{totalImages}</span>
                </button>
                {collections.map((c) => (
                    <button
                        key={c.id}
                        type="button"
                        className="chip"
                        aria-pressed={selected === c.id}
                        title={collectionCount(c.images)}
                        onClick={() => {
                            setMode('idle');
                            onSelect(c.id);
                        }}
                    >
                        {c.name} <span className="chip-count">{c.images}</span>
                    </button>
                ))}
                {mode !== 'creating' && (
                    <button type="button" className="chip chip-add" onClick={() => setMode('creating')}>
                        + New collection
                    </button>
                )}
            </div>

            {mode === 'creating' && (
                <NameForm
                    label="New collection name"
                    submitLabel="Create"
                    existing={collections}
                    onSubmit={(name) => onCreate(name).then(() => setMode('idle'))}
                    onCancel={() => setMode('idle')}
                />
            )}

            {current && mode === 'idle' && (
                <div className="collection-tools">
                    <button type="button" className="btn secondary" onClick={() => setMode('renaming')}>
                        Rename
                    </button>
                    <button type="button" className="btn secondary" onClick={() => setMode('confirmDelete')}>
                        Delete collection…
                    </button>
                </div>
            )}
            {current && mode === 'renaming' && (
                <NameForm
                    label={`New name for ${current.name}`}
                    submitLabel="Rename"
                    initial={current.name}
                    existing={collections}
                    ownID={current.id}
                    onSubmit={(name) => onRename(current.id, name).then(() => setMode('idle'))}
                    onCancel={() => setMode('idle')}
                />
            )}
            {current && mode === 'confirmDelete' && (
                <div className="confirm" role="group" aria-label="Confirm deletion">
                    <p>
                        Delete the collection “{current.name}”? Its {collectionCount(current.images)} stay in your
                        library.
                    </p>
                    <div className="download-row">
                        <button
                            type="button"
                            className="btn danger"
                            onClick={() => {
                                setMode('idle');
                                onDelete(current.id);
                            }}
                        >
                            Delete collection
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
