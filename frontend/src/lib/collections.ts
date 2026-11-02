// Helpers for collections; the rules match internal/library.

export const maxCollectionName = 100;

// cleanCollectionName collapses whitespace like the backend does.
export function cleanCollectionName(name: string): string {
    return name.trim().split(/\s+/).filter(Boolean).join(' ');
}

// collectionNameError returns why a name cannot be used, or null if it can.
// Names are compared case-insensitively, like in the database.
export function collectionNameError(name: string, existing: {id: number; name: string}[], ownID = 0): string | null {
    const clean = cleanCollectionName(name);
    if (clean === '') return 'Enter a name.';
    if ([...clean].length > maxCollectionName) return `Use at most ${maxCollectionName} characters.`;
    const lower = clean.toLocaleLowerCase();
    if (existing.some((c) => c.id !== ownID && c.name.toLocaleLowerCase() === lower)) {
        return 'A collection with this name already exists.';
    }
    return null;
}

// collectionCount is the short image count shown next to a collection.
export function collectionCount(images: number): string {
    return images === 1 ? '1 image' : `${images} images`;
}
