import {describe, expect, it} from 'vitest';
import {appendUnique, formatFileSize, formatType, imageKey, licenseLabel} from './images';

const img = (provider: string, providerId: string) => ({provider, providerId});

describe('imageKey', () => {
    it('combines provider and ID', () => {
        expect(imageKey(img('wallhaven', 'abc'))).toBe('wallhaven/abc');
    });
});

describe('appendUnique', () => {
    it('appends new images in order', () => {
        const result = appendUnique([img('w', 'a')], [img('w', 'b'), img('w', 'c')]);
        expect(result.map(imageKey)).toEqual(['w/a', 'w/b', 'w/c']);
    });

    it('skips images already shown and duplicates within the page', () => {
        const result = appendUnique([img('w', 'a')], [img('w', 'a'), img('w', 'b'), img('w', 'b')]);
        expect(result.map(imageKey)).toEqual(['w/a', 'w/b']);
    });

    it('treats equal IDs from different providers as different images', () => {
        const result = appendUnique([img('w', 'a')], [img('u', 'a')]);
        expect(result).toHaveLength(2);
    });

    it('returns the same array when nothing is new', () => {
        const existing = [img('w', 'a')];
        expect(appendUnique(existing, [img('w', 'a')])).toBe(existing);
    });
});

describe('formatFileSize', () => {
    it.each([
        [0, ''],
        [-5, ''],
        [512, '512 B'],
        [1536, '1.5 KB'],
        [1208033, '1.2 MB'],
        [15 * 1024 * 1024, '15 MB'],
    ])('formats %d bytes as %j', (bytes, want) => {
        expect(formatFileSize(bytes)).toBe(want);
    });
});

describe('formatType', () => {
    it('shows the subtype in capitals', () => {
        expect(formatType('image/jpeg')).toBe('JPEG');
        expect(formatType('image/png')).toBe('PNG');
        expect(formatType('image/x-webp')).toBe('WEBP');
        expect(formatType('')).toBe('');
    });
});

describe('licenseLabel', () => {
    it('shows the license name when known', () => {
        expect(licenseLabel({status: 'known', name: 'CC BY 4.0'})).toBe('CC BY 4.0');
    });

    it('says when the license is unknown', () => {
        expect(licenseLabel({status: 'unknown', name: ''})).toBe('License unknown');
        expect(licenseLabel({status: 'known', name: ''})).toBe('License unknown');
    });
});
