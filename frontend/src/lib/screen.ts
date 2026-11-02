import {orientationOf} from './images';

// Screen information from the Wails runtime.
//
// The TypeScript type shipped with Wails v2.17 lacks the size and
// physicalSize fields that the Go side sends, so they are declared here.

export interface ScreenSize {
    width: number;
    height: number;
}

export interface WailsScreen {
    isCurrent: boolean;
    isPrimary: boolean;
    width: number; // deprecated by Wails, same as size
    height: number;
    size?: ScreenSize; // logical pixels
    physicalSize?: ScreenSize; // device pixels (logical × scale factor)
}

// screenPixels returns the device-pixel size of the screen the window is on,
// falling back to the primary or first screen. Wallpapers need device
// pixels: a 200% HiDPI screen of 1920 × 1080 logical pixels shows 3840 × 2160.
export function screenPixels(screens: WailsScreen[]): ScreenSize | null {
    const screen = screens.find((s) => s.isCurrent) ?? screens.find((s) => s.isPrimary) ?? screens[0];
    if (!screen) return null;
    const size = screen.physicalSize ?? screen.size ?? {width: screen.width, height: screen.height};
    if (!(size.width > 0 && size.height > 0)) return null;
    return {width: size.width, height: size.height};
}

// screenFilters returns the search filters that fit a screen: its size as
// the minimum resolution and its orientation. A square screen leaves the
// orientation open ("").
export function screenFilters(size: ScreenSize): {width: number; height: number; orientation: '' | 'landscape' | 'portrait'} {
    const o = orientationOf(size.width, size.height);
    return {width: size.width, height: size.height, orientation: o === 'landscape' || o === 'portrait' ? o : ''};
}

// largestScreen returns the largest width and the largest height of all
// screens, in device pixels: fetched images are reduced to cover it.
export function largestScreen(screens: WailsScreen[]): ScreenSize | null {
    let width = 0;
    let height = 0;
    for (const s of screens) {
        const size = s.physicalSize ?? s.size ?? {width: s.width, height: s.height};
        width = Math.max(width, size.width || 0);
        height = Math.max(height, size.height || 0);
    }
    return width > 0 && height > 0 ? {width, height} : null;
}
