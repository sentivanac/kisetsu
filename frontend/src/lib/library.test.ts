import {describe, expect, it} from 'vitest';
import {fetchedStatsLabel, formatAddedDate, libraryStatsLabel, removeItem, folderSyncLabel} from './library';

describe('libraryStatsLabel', () => {
    it('summarizes count and size', () => {
        expect(libraryStatsLabel({images: 0, bytes: 0})).toBe('No images yet');
        expect(libraryStatsLabel({images: 1, bytes: 1536})).toBe('1 image · 1.5 KB');
        expect(libraryStatsLabel({images: 42, bytes: 186 * 1024 * 1024})).toBe('42 images · 186 MB');
    });
});

describe('formatAddedDate', () => {
    it('formats valid timestamps and ignores invalid ones', () => {
        expect(formatAddedDate('2026-10-30T09:15:00Z', 'en-GB')).toBe('30 Oct 2026');
        expect(formatAddedDate('not a date')).toBe('');
    });
});

describe('removeItem', () => {
    const items = [{id: 1}, {id: 2}, {id: 3}];

    it('selects the following item, or the previous one at the end', () => {
        expect(removeItem(items, 2)).toEqual({items: [{id: 1}, {id: 3}], next: 1});
        expect(removeItem(items, 3)).toEqual({items: [{id: 1}, {id: 2}], next: 1});
    });

    it('handles the last item and unknown IDs', () => {
        expect(removeItem([{id: 1}], 1)).toEqual({items: [], next: null});
        expect(removeItem(items, 9)).toEqual({items, next: null});
    });
});

describe('fetchedStatsLabel', () => {
    it('shows use against the limit', () => {
        expect(fetchedStatsLabel({images: 12, bytes: 84 * 1024 * 1024, maxBytes: 200 * 1024 * 1024})).toBe(
            '12 images · 84 MB of 200 MB',
        );
        expect(fetchedStatsLabel({images: 0, bytes: 0, maxBytes: 200 * 1024 * 1024})).toBe('0 images · 0 B of 200 MB');
    });
});

describe('folderSyncLabel', () => {
    const r = (images: number, added = 0, updated = 0, removed = 0, skipped = 0) => ({
        folder: {images},
        added,
        updated,
        removed,
        skipped,
    });
    it('says how many images, and what changed', () => {
        expect(folderSyncLabel(r(1))).toBe('1 image.');
        expect(folderSyncLabel(r(312, 3, 0, 1))).toBe('312 images. 3 new, 1 gone since last time.');
        expect(folderSyncLabel(r(5, 0, 2, 0, 1))).toBe('5 images. 2 changed since last time. 1 file could not be read.');
    });
});
