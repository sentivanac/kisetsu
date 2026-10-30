import {describe, expect, it} from 'vitest';
import {screenPixels, WailsScreen} from './screen';

const screen = (over: Partial<WailsScreen>): WailsScreen => ({
    isCurrent: false,
    isPrimary: false,
    width: 1920,
    height: 1080,
    size: {width: 1920, height: 1080},
    physicalSize: {width: 1920, height: 1080},
    ...over,
});

describe('screenPixels', () => {
    it('uses the physical size of the current screen', () => {
        const screens = [
            screen({isPrimary: true}),
            screen({isCurrent: true, size: {width: 1920, height: 1080}, physicalSize: {width: 3840, height: 2160}}),
        ];
        expect(screenPixels(screens)).toEqual({width: 3840, height: 2160});
    });

    it('falls back to the primary, then the first screen', () => {
        const primary = screen({isPrimary: true, physicalSize: {width: 2560, height: 1440}});
        expect(screenPixels([screen({}), primary])).toEqual({width: 2560, height: 1440});
        expect(screenPixels([screen({physicalSize: {width: 1366, height: 768}})])).toEqual({width: 1366, height: 768});
    });

    it('falls back to logical sizes when physical sizes are missing', () => {
        expect(screenPixels([screen({isCurrent: true, physicalSize: undefined})])).toEqual({width: 1920, height: 1080});
        expect(screenPixels([screen({physicalSize: undefined, size: undefined, width: 1280, height: 800})])).toEqual({
            width: 1280,
            height: 800,
        });
    });

    it('returns null when nothing usable is known', () => {
        expect(screenPixels([])).toBeNull();
        expect(screenPixels([screen({physicalSize: {width: 0, height: 0}})])).toBeNull();
    });
});
