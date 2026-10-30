// Minimum-resolution choices for the search form.

export interface Resolution {
    width: number;
    height: number;
}

export const anyResolution: Resolution = {width: 0, height: 0};

export const resolutionPresets: Resolution[] = [
    {width: 1920, height: 1080},
    {width: 2560, height: 1440},
    {width: 3840, height: 2160},
];

// resolutionKey is the <select> value for a resolution.
export function resolutionKey(r: Resolution): string {
    return `${r.width}x${r.height}`;
}

export function parseResolutionKey(key: string): Resolution {
    const [width, height] = key.split('x').map(Number);
    return Number.isInteger(width) && Number.isInteger(height) && width >= 0 && height >= 0
        ? {width, height}
        : anyResolution;
}

export function resolutionLabel(r: Resolution): string {
    return r.width === 0 && r.height === 0 ? 'Any' : `${r.width} × ${r.height}`;
}

// resolutionOptions lists the presets, plus the current value when it is
// not a preset (for example the user's screen size).
export function resolutionOptions(current: Resolution): Resolution[] {
    const options = [anyResolution, ...resolutionPresets];
    const known = options.some((r) => resolutionKey(r) === resolutionKey(current));
    if (known) return options;
    return [...options, current].sort((a, b) => a.width * a.height - b.width * b.height);
}
