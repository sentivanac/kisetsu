import {describe, expect, it} from 'vitest';
import {isTagHidden} from './tags';

describe('isTagHidden', () => {
    it('ignores case', () => {
        expect(isTagHidden(['Barefoot'], 'barefoot')).toBe(true);
        expect(isTagHidden(['long hair'], 'long')).toBe(false);
        expect(isTagHidden([], 'x')).toBe(false);
    });
});
