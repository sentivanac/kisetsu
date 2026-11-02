import {describe, expect, it} from 'vitest';
import {formatAddedDate, libraryStatsLabel, removeItem} from './library';

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
