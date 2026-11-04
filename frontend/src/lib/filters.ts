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

// Content categories, as Wallhaven classifies images. An empty choice
// means all categories (no restriction), like in the Go code.
export const allCategories = ['general', 'anime', 'people'] as const;
export type Category = (typeof allCategories)[number];

export const categoryLabels: Record<Category, string> = {
    general: 'General',
    anime: 'Anime',
    people: 'People',
};

export function categoryChecked(chosen: string[], category: Category): boolean {
    return chosen.length === 0 || chosen.includes(category);
}

// toggleCategory turns one category on or off. At least one stays on, and
// all on is stored as [] (no restriction).
export function toggleCategory(chosen: string[], category: Category): string[] {
    const on = allCategories.filter((c) => categoryChecked(chosen, c));
    const next = on.includes(category) ? on.filter((c) => c !== category) : [...on, category];
    if (next.length === 0) return chosen;
    if (next.length === allCategories.length) return [];
    return allCategories.filter((c) => next.includes(c));
}
