// Formatting helpers for the library view.

import {formatFileSize} from './images';

// libraryStatsLabel summarizes the library, e.g. "42 images · 186 MB".
export function libraryStatsLabel(stats: {images: number; bytes: number}): string {
    if (stats.images === 0) return 'No images yet';
    const count = stats.images === 1 ? '1 image' : `${stats.images} images`;
    const size = formatFileSize(stats.bytes);
    return size ? `${count} · ${size}` : count;
}

// formatAddedDate turns an RFC 3339 timestamp into a short local date,
// or "" if it cannot be read.
export function formatAddedDate(rfc3339: string, locale?: string): string {
    const date = new Date(rfc3339);
    if (Number.isNaN(date.getTime())) return '';
    return date.toLocaleDateString(locale, {year: 'numeric', month: 'short', day: 'numeric'});
}

// removeItem returns the list without the item with the given ID, and the
// index of the item that should be selected next (or null if none remain).
export function removeItem<T extends {id: number}>(items: T[], id: number): {items: T[]; next: number | null} {
    const index = items.findIndex((item) => item.id === id);
    if (index < 0) return {items, next: null};
    const rest = [...items.slice(0, index), ...items.slice(index + 1)];
    if (rest.length === 0) return {items: rest, next: null};
    return {items: rest, next: Math.min(index, rest.length - 1)};
}

// fetchedStatsLabel shows the space fetched images use against the limit,
// e.g. "12 images · 84 MB of 200 MB".
export function fetchedStatsLabel(stats: {images: number; bytes: number; maxBytes: number}): string {
    const count = stats.images === 1 ? '1 image' : `${stats.images} images`;
    const used = formatFileSize(stats.bytes) || '0 B';
    return `${count} · ${used} of ${formatFileSize(stats.maxBytes)}`;
}
