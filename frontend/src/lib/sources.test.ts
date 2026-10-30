import {describe, expect, it} from 'vitest';
import {initialSources, mergeStatuses, nextPages, SourceInfo, SourceStatus, statusLabel, toggleSource} from './sources';

const available: SourceInfo[] = [
    {id: 'wallhaven', name: 'Wallhaven', enabledByDefault: true},
    {id: 'commons', name: 'Commons', enabledByDefault: false, contentWarning: 'No SFW rating.'},
];

const status = (over: Partial<SourceStatus>): SourceStatus => ({
    providerId: 'wallhaven',
    name: 'Wallhaven',
    page: 1,
    hasMore: false,
    count: 0,
    ...over,
});

describe('initialSources', () => {
    it('uses only sources enabled by default when nothing is remembered', () => {
        expect(initialSources(available, null)).toEqual(['wallhaven']);
        expect(initialSources(available, [])).toEqual(['wallhaven']);
    });

    it('restores remembered sources, including opt-in ones', () => {
        expect(initialSources(available, ['commons'])).toEqual(['commons']);
    });

    it('ignores remembered sources that no longer exist', () => {
        expect(initialSources(available, ['gone', 'commons'])).toEqual(['commons']);
        expect(initialSources(available, ['gone'])).toEqual(['wallhaven']);
    });
});

describe('toggleSource', () => {
    it('adds and removes sources in display order', () => {
        expect(toggleSource(available, ['commons'], 'wallhaven')).toEqual(['wallhaven', 'commons']);
        expect(toggleSource(available, ['wallhaven', 'commons'], 'wallhaven')).toEqual(['commons']);
    });
});

describe('nextPages', () => {
    it('asks sources with more results for their next page', () => {
        expect(nextPages([status({page: 2, hasMore: true})])).toEqual([{providerId: 'wallhaven', page: 3}]);
    });

    it('retries failed sources at the same page', () => {
        const failed = status({providerId: 'commons', page: 1, error: {code: 'provider_unavailable', message: 'Down.'}});
        expect(nextPages([failed])).toEqual([{providerId: 'commons', page: 1}]);
    });

    it('skips finished and canceled sources', () => {
        const finished = status({hasMore: false});
        const canceled = status({providerId: 'commons', error: {code: 'canceled', message: 'Canceled.'}});
        expect(nextPages([finished, canceled])).toEqual([]);
    });
});

describe('mergeStatuses', () => {
    it('updates asked sources, adds counts and keeps the others', () => {
        const current = [status({page: 1, count: 24, hasMore: true}), status({providerId: 'commons', name: 'Commons', count: 5})];
        const updates = [status({page: 2, count: 20, hasMore: false})];

        expect(mergeStatuses(current, updates)).toEqual([
            status({page: 2, count: 44, hasMore: false}),
            status({providerId: 'commons', name: 'Commons', count: 5}),
        ]);
    });
});

describe('statusLabel', () => {
    it('summarizes counts and errors', () => {
        expect(statusLabel(status({count: 24, hasMore: true}))).toBe('Wallhaven: 24 images');
        expect(statusLabel(status({count: 1}))).toBe('Wallhaven: 1 image (all shown)');
        expect(statusLabel(status({error: {code: 'provider_unavailable', message: 'Wallhaven could not be reached.'}}))).toBe(
            'Wallhaven: Wallhaven could not be reached.',
        );
        expect(statusLabel(status({error: {code: 'rate_limited', message: 'Too many searches.', retryAfterSeconds: 9}}))).toBe(
            'Wallhaven: Too many searches. Try again in 9 s.',
        );
    });
});
