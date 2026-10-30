import {describe, expect, it} from 'vitest';
import {describeError, toBackendError} from './backend';

describe('describeError', () => {
    it('adds the retry delay for rate limits', () => {
        expect(
            describeError({code: 'rate_limited', message: 'Too many searches.', retryAfterSeconds: 30}),
        ).toBe('Too many searches. Try again in 30 s.');
    });

    it('returns the message unchanged otherwise', () => {
        expect(describeError({code: 'provider_unavailable', message: 'Offline.'})).toBe('Offline.');
        expect(describeError({code: 'rate_limited', message: 'Wait.', retryAfterSeconds: 0})).toBe('Wait.');
    });
});

describe('toBackendError', () => {
    it('keeps errors sent by the Go ErrorFormatter', () => {
        const err = {code: 'unavailable', message: 'The local database is not available.'};

        expect(toBackendError(err)).toEqual(err);
    });

    it('keeps the retry delay of rate-limit errors', () => {
        const err = {code: 'rate_limited', message: 'Too many searches.', retryAfterSeconds: 30};

        expect(toBackendError(err).retryAfterSeconds).toBe(30);
    });

    it('wraps plain strings sent by Wails as internal errors', () => {
        expect(toBackendError('error parsing arguments: bad input')).toEqual({
            code: 'internal',
            message: 'error parsing arguments: bad input',
        });
    });

    it.each([
        ['undefined', undefined],
        ['null', null],
        ['an empty string', '  '],
        ['an Error instance', new Error('boom')],
        ['an object with the wrong shape', {code: 42, message: 'nope'}],
    ])('falls back to a generic message for %s', (_name, value) => {
        const result = toBackendError(value);

        expect(result.code).toBe('internal');
        expect(result.message).toMatch(/log file/);
    });
});
