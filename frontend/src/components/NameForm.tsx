import {FormEvent, KeyboardEvent, useState} from 'react';
import {toBackendError} from '../lib/backend';
import {cleanCollectionName, collectionNameError, maxCollectionName} from '../lib/collections';

interface Props {
    label: string; // accessible name of the field, e.g. "New collection name"
    submitLabel: string;
    initial?: string;
    existing: {id: number; name: string}[];
    ownID?: number; // when renaming, the collection's own ID
    onSubmit: (name: string) => Promise<void>;
    onCancel: () => void;
}

// NameForm asks for a collection name and checks it before submitting.
// Escape cancels. Errors from the backend are shown under the field.
export function NameForm({label, submitLabel, initial = '', existing, ownID, onSubmit, onCancel}: Props) {
    const [name, setName] = useState(initial);
    const [error, setError] = useState<string | null>(null);
    const [busy, setBusy] = useState(false);

    function submit(event: FormEvent) {
        event.preventDefault();
        const problem = collectionNameError(name, existing, ownID);
        if (problem) {
            setError(problem);
            return;
        }
        setBusy(true);
        onSubmit(cleanCollectionName(name))
            .catch((err: unknown) => setError(toBackendError(err).message))
            .finally(() => setBusy(false));
    }

    function onKeyDown(event: KeyboardEvent) {
        if (event.key === 'Escape') {
            // Do not close the surrounding preview dialog as well.
            event.preventDefault();
            event.stopPropagation();
            onCancel();
        }
    }

    return (
        <form className="name-form" onSubmit={submit} onKeyDown={onKeyDown}>
            <input
                type="text"
                aria-label={label}
                aria-invalid={error !== null}
                value={name}
                maxLength={maxCollectionName * 2}
                onChange={(e) => {
                    setName(e.target.value);
                    setError(null);
                }}
                autoFocus
            />
            <button type="submit" className="btn" disabled={busy}>
                {submitLabel}
            </button>
            <button type="button" className="btn secondary" onClick={onCancel}>
                Cancel
            </button>
            {error && (
                <p className="error" role="alert">
                    {error}
                </p>
            )}
        </form>
    );
}
