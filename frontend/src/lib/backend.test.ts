import {describe, expect, it} from 'vitest';
import {describeError, isCanceled, toBackendError} from './backend';

describe('isCanceled', () => {
    it('recognizes canceled operations only', () => {
        expect(isCanceled({code: 'canceled', message: 'The search was canceled.'})).toBe(true);
        expect(isCanceled({code: 'internal', message: 'Boom.'})).toBe(false);
    });
});

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
    // What the Wails v2 runtime really does with the ErrorFormatter's value.
    it('reads the JSON text that Wails wraps in an Error', () => {
        const sent = {code: 'unsupported', message: 'Kisetsu cannot set the wallpaper on this desktop yet.'};

        expect(toBackendError(new Error(JSON.stringify(sent)))).toEqual(sent);
    });

    it('keeps the retry delay through the Error wrapping', () => {
        const sent = {code: 'rate_limited', message: 'Too many searches.', retryAfterSeconds: 30};

        expect(toBackendError(new Error(JSON.stringify(sent)))).toEqual(sent);
    });

    it('reads canceled errors, so they stay hidden', () => {
        const err = toBackendError(new Error('{"code":"canceled","message":"The search was canceled."}'));

        expect(isCanceled(err)).toBe(true);
    });

    it('reads JSON text that arrives without the Error wrapping', () => {
        expect(toBackendError('{"code":"invalid_input","message":"Name is empty."}')).toEqual({
            code: 'invalid_input',
            message: 'Name is empty.',
        });
    });

    it('keeps errors sent by the Go ErrorFormatter as objects', () => {
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
        ['an Error wrapping an object', new Error(String({code: 'x', message: 'y'}))],
        ['an Error with JSON of the wrong shape', new Error('{"code": 1}')],
    ])('falls back to a generic message for %s', (_name, value) => {
        const result = toBackendError(value);

        expect(result.code).toBe('internal');
        expect(result.message).toMatch(/log file/);
    });
});
