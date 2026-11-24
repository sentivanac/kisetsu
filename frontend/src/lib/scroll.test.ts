import {describe, expect, it} from 'vitest';
import {keyScrollTarget} from './scroll';

describe('keyScrollTarget', () => {
    const el = {scrollTop: 500, clientHeight: 1000, scrollHeight: 3000};
    it('moves by pages and to the ends', () => {
        expect(keyScrollTarget('PageDown', el)).toBe(1400);
        expect(keyScrollTarget('PageUp', el)).toBe(0);
        expect(keyScrollTarget('Home', el)).toBe(0);
        expect(keyScrollTarget('End', el)).toBe(2000);
    });
    it('stays in range and ignores other keys', () => {
        expect(keyScrollTarget('PageDown', {...el, scrollTop: 1900})).toBe(2000);
        expect(keyScrollTarget('End', {...el, scrollHeight: 400})).toBe(0);
        expect(keyScrollTarget('a', el)).toBeNull();
    });
});
