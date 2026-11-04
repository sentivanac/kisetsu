import {describe, expect, it} from 'vitest';
import {
    anyResolution,
    categoryChecked,
    parseResolutionKey,
    resolutionKey,
    resolutionLabel,
    resolutionOptions,
    toggleCategory,
} from './filters';

describe('resolution keys', () => {
    it('round-trips', () => {
        expect(parseResolutionKey(resolutionKey({width: 2560, height: 1440}))).toEqual({width: 2560, height: 1440});
    });

    it('falls back to any for malformed keys', () => {
        expect(parseResolutionKey('wide')).toEqual(anyResolution);
        expect(parseResolutionKey('-1x5')).toEqual(anyResolution);
    });
});

describe('resolutionLabel', () => {
    it('labels the empty resolution as Any', () => {
        expect(resolutionLabel(anyResolution)).toBe('Any');
        expect(resolutionLabel({width: 1920, height: 1080})).toBe('1920 × 1080');
    });
});

describe('resolutionOptions', () => {
    it('returns only presets for a preset value', () => {
        const options = resolutionOptions({width: 1920, height: 1080});
        expect(options.map(resolutionKey)).toEqual(['0x0', '1920x1080', '2560x1440', '3840x2160']);
    });

    it('inserts a custom value in size order', () => {
        const options = resolutionOptions({width: 2880, height: 1800});
        expect(options.map(resolutionKey)).toEqual(['0x0', '1920x1080', '2560x1440', '2880x1800', '3840x2160']);
    });
});

describe('categories', () => {
    it('treats an empty choice as all', () => {
        expect(categoryChecked([], 'anime')).toBe(true);
        expect(categoryChecked(['general'], 'anime')).toBe(false);
    });

    it('turns categories off and on in the standard order', () => {
        expect(toggleCategory([], 'anime')).toEqual(['general', 'people']);
        expect(toggleCategory(['general', 'people'], 'people')).toEqual(['general']);
        expect(toggleCategory(['people'], 'general')).toEqual(['general', 'people']);
    });

    it('stores all as no restriction', () => {
        expect(toggleCategory(['general', 'people'], 'anime')).toEqual([]);
    });

    it('keeps at least one category', () => {
        expect(toggleCategory(['general'], 'general')).toEqual(['general']);
    });
});
