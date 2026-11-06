import {describe, expect, it} from 'vitest';
import {corners, intervalLabel, parseSourceKey, relativeTime, sourceKey} from './rotation';

describe('intervalLabel', () => {
    it('names the offered intervals', () => {
        expect(intervalLabel('15m')).toBe('Every 15 minutes');
        expect(intervalLabel('24h')).toBe('Once a day');
        expect(intervalLabel('7m')).toBe('7m');
    });
});

describe('relativeTime', () => {
    const now = new Date('2026-11-02T08:00:00Z');
    const at = (minutes: number) => new Date(now.getTime() + minutes * 60000).toISOString();

    it('describes future and past times', () => {
        expect(relativeTime(at(34), now)).toBe('in 34 min');
        expect(relativeTime(at(-5), now)).toBe('5 min ago');
        expect(relativeTime(at(130), now)).toBe('in 2 h 10 min');
        expect(relativeTime(at(180), now)).toBe('in 3 h');
        expect(relativeTime(at(-24 * 60), now)).toBe('1 day ago');
        expect(relativeTime(at(-3 * 24 * 60), now)).toBe('3 days ago');
    });

    it('handles moments and missing values', () => {
        expect(relativeTime(at(0), now)).toBe('in less than a minute');
        expect(relativeTime(new Date(now.getTime() - 20000).toISOString(), now)).toBe('just now');
        expect(relativeTime('', now)).toBe('');
        expect(relativeTime('soon', now)).toBe('');
    });
});

describe('rotation source keys', () => {
    it('round-trips every kind of source', () => {
        for (const choice of [
            {collectionId: 0, savedSearchId: 0},
            {collectionId: 7, savedSearchId: 0},
            {collectionId: 0, savedSearchId: 3},
        ]) {
            expect(parseSourceKey(sourceKey(choice))).toEqual(choice);
        }
    });

    it('falls back to the whole library for unknown keys', () => {
        expect(parseSourceKey('search:x')).toEqual({collectionId: 0, savedSearchId: 0});
        expect(parseSourceKey('album:4')).toEqual({collectionId: 0, savedSearchId: 0});
    });
});

describe('corners', () => {
    it('offers the four corners the backend accepts', () => {
        expect(corners.map((c) => c.value)).toEqual(['top-left', 'top-right', 'bottom-left', 'bottom-right']);
    });
});
