import {useState} from 'react';
import {main} from '../../wailsjs/go/models';
import {collectionCount} from '../lib/collections';
import {NameForm} from './NameForm';

// FetchedView is the "collection" value that shows fetched images.
export const FetchedView = -1;

// A local folder is shown as the "collection" value folderView(id), below
// FetchedView, so one number still says what the library shows.
export const folderView = (id: number) => -1 - id;
export const isFolderView = (view: number) => view < FetchedView;
export const folderOf = (view: number) => -1 - view;

interface Props {
    collections: main.CollectionInfo[];
    folders: main.FolderInfo[];
    selected: number; // 0 = all images, FetchedView = fetched images, folderView(id) = a folder
    totalImages: number;
    fetchedImages: number;
    // adding is set while a new folder is being read.
    addingFolder: boolean;
    onSelect: (id: number) => void;
    onCreate: (name: string) => Promise<void>;
    onRename: (id: number, name: string) => Promise<void>;
    onDelete: (id: number) => void;
    onAddFolder: () => void;
    onRemoveFolder: (id: number) => void;
}

type Mode = 'idle' | 'creating' | 'renaming' | 'confirmDelete' | 'confirmRemoveFolder';

// CollectionBar picks the collection shown in the library and manages
// collections.
export function CollectionBar({
    collections,
    folders,
    selected,
    totalImages,
    fetchedImages,
    addingFolder,
    onSelect,
    onCreate,
    onRename,
    onDelete,
    onAddFolder,
    onRemoveFolder,
}: Props) {
    const [mode, setMode] = useState<Mode>('idle');
    const current = collections.find((c) => c.id === selected);
    const folder = isFolderView(selected) ? folders.find((f) => f.id === folderOf(selected)) : undefined;

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
                <button
                    type="button"
                    className="chip chip-fetched"
                    aria-pressed={selected === FetchedView}
                    title="Images fetched automatically from saved searches"
                    onClick={() => {
                        setMode('idle');
                        onSelect(FetchedView);
                    }}
                >
                    Fetched <span className="chip-count">{fetchedImages}</span>
                </button>
                {folders.map((f) => (
                    <button
                        key={f.id}
                        type="button"
                        className="chip chip-folder"
                        aria-pressed={selected === folderView(f.id)}
                        title={f.missing ? `${f.path} (not available now)` : f.path}
                        onClick={() => {
                            setMode('idle');
                            onSelect(folderView(f.id));
                        }}
                    >
                        <span aria-hidden="true">📁</span> {f.name} <span className="chip-count">{f.images}</span>
                    </button>
                ))}
                {mode !== 'creating' && (
                    <button type="button" className="chip chip-add" onClick={() => setMode('creating')}>
                        + New collection
                    </button>
                )}
                <button type="button" className="chip chip-add" onClick={onAddFolder} disabled={addingFolder}>
                    {addingFolder ? 'Reading folder…' : '+ Add folder'}
                </button>
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

            {folder && mode === 'idle' && (
                <div className="collection-tools">
                    <button type="button" className="btn secondary" onClick={() => setMode('confirmRemoveFolder')}>
                        Remove folder…
                    </button>
                </div>
            )}
            {folder && mode === 'confirmRemoveFolder' && (
                <div className="confirm" role="group" aria-label="Confirm removing the folder">
                    <p>
                        Remove the folder “{folder.name}” from Kisetsu? Its files stay where they are; Kisetsu only
                        forgets them, with their places in your collections.
                    </p>
                    <div className="download-row">
                        <button
                            type="button"
                            className="btn danger"
                            onClick={() => {
                                setMode('idle');
                                onRemoveFolder(folder.id);
                            }}
                        >
                            Remove folder
                        </button>
                        <button type="button" className="btn secondary" onClick={() => setMode('idle')} autoFocus>
                            Keep
                        </button>
                    </div>
                </div>
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
