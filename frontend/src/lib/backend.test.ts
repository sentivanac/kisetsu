import {describe, expect, it} from 'vitest';
import {toBackendError} from './backend';

describe('toBackendError', () => {
    it('keeps errors sent by the Go ErrorFormatter', () => {
        const err = {code: 'unavailable', message: 'The local database is not available.'};

        expect(toBackendError(err)).toEqual(err);
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
