import {describe, expect, it} from 'vitest';
import {cleanCollectionName, collectionCount, collectionNameError, maxCollectionName} from './collections';

const existing = [
    {id: 1, name: 'Autumn'},
    {id: 2, name: 'Edo period'},
];

describe('cleanCollectionName', () => {
    it('collapses whitespace', () => {
        expect(cleanCollectionName('  Edo \t  period ')).toBe('Edo period');
    });
});

describe('collectionNameError', () => {
    it('accepts new names', () => {
        expect(collectionNameError('Winter', existing)).toBeNull();
        expect(collectionNameError('季'.repeat(maxCollectionName), existing)).toBeNull(); // characters, not bytes
    });

    it('rejects empty, long and taken names', () => {
        expect(collectionNameError('   ', existing)).toMatch(/Enter a name/);
        expect(collectionNameError('x'.repeat(maxCollectionName + 1), existing)).toMatch(/at most/);
        expect(collectionNameError('AUTUMN', existing)).toMatch(/already exists/);
        expect(collectionNameError(' edo   PERIOD ', existing)).toMatch(/already exists/);
    });

    it('lets a collection keep its own name in another case', () => {
        expect(collectionNameError('autumn', existing, 1)).toBeNull();
        expect(collectionNameError('autumn', existing, 2)).toMatch(/already exists/);
    });
});

describe('collectionCount', () => {
    it('pluralizes', () => {
        expect(collectionCount(0)).toBe('0 images');
        expect(collectionCount(1)).toBe('1 image');
        expect(collectionCount(12)).toBe('12 images');
    });
});
