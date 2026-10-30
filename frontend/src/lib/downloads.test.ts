import {describe, expect, it} from 'vitest';
import {applyProgress, DownloadProgress, downloadLabel, downloadMessage, isActive, percent} from './downloads';

const progress = (over: Partial<DownloadProgress>): DownloadProgress => ({
    key: 'wallhaven/abc',
    state: 'downloading',
    received: 0,
    total: 0,
    ...over,
});

describe('applyProgress', () => {
    it('keeps the expected size from the queued event', () => {
        let state = applyProgress({}, progress({state: 'queued', total: 1000}));
        state = applyProgress(state, progress({received: 250}));
        expect(state['wallhaven/abc']).toMatchObject({state: 'downloading', received: 250, total: 1000});
    });

    it('keeps the received bytes while processing', () => {
        let state = applyProgress({}, progress({received: 900, total: 900}));
        state = applyProgress(state, progress({state: 'processing'}));
        expect(state['wallhaven/abc'].received).toBe(900);
    });

    it('tracks downloads independently', () => {
        let state = applyProgress({}, progress({key: 'a/1', received: 1}));
        state = applyProgress(state, progress({key: 'b/2', state: 'done', entryId: 7}));
        expect(Object.keys(state)).toEqual(['a/1', 'b/2']);
        expect(state['b/2'].entryId).toBe(7);
    });
});

describe('percent', () => {
    it('computes the share and handles unknown sizes', () => {
        expect(percent(progress({received: 250, total: 1000}))).toBe(25);
        expect(percent(progress({received: 2000, total: 1000}))).toBe(100);
        expect(percent(progress({received: 10, total: 0}))).toBeNull();
        expect(percent(progress({state: 'processing'}))).toBe(100);
    });
});

describe('isActive', () => {
    it('is true only while a download runs', () => {
        expect(isActive(progress({state: 'queued'}))).toBe(true);
        expect(isActive(progress({state: 'processing'}))).toBe(true);
        expect(isActive(progress({state: 'done'}))).toBe(false);
        expect(isActive(undefined)).toBe(false);
    });
});

describe('downloadLabel', () => {
    it('describes each state', () => {
        expect(downloadLabel(undefined, false)).toBe('Download');
        expect(downloadLabel(undefined, true)).toBe('In your library');
        expect(downloadLabel(progress({state: 'queued'}), false)).toBe('Waiting…');
        expect(downloadLabel(progress({received: 50, total: 200}), false)).toBe('Downloading 25%');
        expect(downloadLabel(progress({received: 50}), false)).toBe('Downloading…');
        expect(downloadLabel(progress({state: 'processing'}), false)).toBe('Checking image…');
        expect(downloadLabel(progress({state: 'duplicate'}), false)).toBe('In your library');
        expect(downloadLabel(progress({state: 'failed'}), false)).toBe('Try again');
        expect(downloadLabel(progress({state: 'canceled'}), false)).toBe('Download');
    });
});

describe('downloadMessage', () => {
    it('explains outcomes', () => {
        expect(downloadMessage(progress({state: 'done'}))).toBe('Saved to your library.');
        expect(downloadMessage(progress({state: 'done', similarIds: [3]}))).toMatch(/looks like an image/);
        expect(downloadMessage(progress({state: 'done', similarIds: [3, 4]}))).toMatch(/2 images/);
        expect(downloadMessage(progress({state: 'duplicate', entryId: 3}))).toMatch(/identical file/);
        expect(downloadMessage(progress({state: 'failed', error: {code: 'invalid_input', message: 'Too big.'}}))).toBe(
            'Too big.',
        );
        expect(downloadMessage(progress({state: 'downloading'}))).toBeNull();
        expect(downloadMessage(undefined)).toBeNull();
    });
});
