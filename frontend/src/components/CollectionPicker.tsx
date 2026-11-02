import {useEffect, useState} from 'react';
import {ImageCollections} from '../../wailsjs/go/main/App';
import {main} from '../../wailsjs/go/models';
import {NameForm} from './NameForm';

interface Props {
    imageID: number;
    collections: main.CollectionInfo[];
    onToggle: (collectionID: number, member: boolean) => Promise<void>;
    // Creates a collection and adds the image to it.
    onCreateWith: (name: string) => Promise<void>;
}

// CollectionPicker shows and changes the collections of one library image.
export function CollectionPicker({imageID, collections, onToggle, onCreateWith}: Props) {
    const [member, setMember] = useState<Set<number> | null>(null);
    const [creating, setCreating] = useState(false);
    const [error, setError] = useState<string | null>(null);

    useEffect(() => {
        setMember(null);
        ImageCollections(imageID)
            .then((ids) => setMember(new Set(ids)))
            .catch(() => setError('Collections could not be loaded.'));
    }, [imageID, collections]);

    function toggle(id: number) {
        if (!member) return;
        const next = !member.has(id);
        setError(null);
        onToggle(id, next)
            .then(() => {
                const updated = new Set(member);
                if (next) updated.add(id);
                else updated.delete(id);
                setMember(updated);
            })
            .catch(() => setError('The collection could not be changed.'));
    }

    return (
        <fieldset className="collection-picker">
            <legend>Collections</legend>
            {collections.length === 0 && !creating && <p className="muted">No collections yet.</p>}
            {member &&
                collections.map((c) => (
                    <label key={c.id} className="source">
                        <input type="checkbox" checked={member.has(c.id)} onChange={() => toggle(c.id)}/>
                        <span>{c.name}</span>
                    </label>
                ))}
            {creating ? (
                <NameForm
                    label="New collection name"
                    submitLabel="Create and add"
                    existing={collections}
                    onSubmit={(name) => onCreateWith(name).then(() => setCreating(false))}
                    onCancel={() => setCreating(false)}
                />
            ) : (
                <button type="button" className="link" onClick={() => setCreating(true)}>
                    + New collection
                </button>
            )}
            {error && (
                <p className="error" role="alert">
                    {error}
                </p>
            )}
        </fieldset>
    );
}
