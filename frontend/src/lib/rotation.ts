// Formatting for the Wallpaper tab.

const intervalLabels: Record<string, string> = {
    '15m': 'Every 15 minutes',
    '30m': 'Every 30 minutes',
    '1h': 'Every hour',
    '3h': 'Every 3 hours',
    '24h': 'Once a day',
};

export function intervalLabel(interval: string): string {
    return intervalLabels[interval] ?? interval;
}

// relativeTime describes an RFC 3339 time relative to now, e.g.
// "in 34 min", "5 min ago", "in 2 h 10 min". It returns "" for an empty or
// unreadable time.
export function relativeTime(rfc3339: string, now: Date = new Date()): string {
    if (!rfc3339) return '';
    const t = new Date(rfc3339);
    if (Number.isNaN(t.getTime())) return '';

    const diffMs = t.getTime() - now.getTime();
    // Decide the direction before rounding: Math.round(-0.3) is -0, and
    // -0 >= 0 is true.
    if (Math.abs(diffMs) < 30_000) return diffMs >= 0 ? 'in less than a minute' : 'just now';
    const diffMin = Math.round(diffMs / 60000);
    const abs = Math.max(1, Math.abs(diffMin));

    let span: string;
    if (abs < 60) {
        span = `${abs} min`;
    } else if (abs < 24 * 60) {
        const h = Math.floor(abs / 60);
        const m = abs % 60;
        span = m === 0 ? `${h} h` : `${h} h ${m} min`;
    } else {
        const d = Math.round(abs / (24 * 60));
        span = d === 1 ? '1 day' : `${d} days`;
    }
    return diffMin > 0 ? `in ${span}` : `${span} ago`;
}

// clockTime formats an RFC 3339 time as a local time of day.
export function clockTime(rfc3339: string, locale?: string): string {
    const t = new Date(rfc3339);
    if (!rfc3339 || Number.isNaN(t.getTime())) return '';
    return t.toLocaleTimeString(locale, {hour: '2-digit', minute: '2-digit'});
}

// Rotation source as one <select> value: "library", "collection:7",
// "search:3" or "folder:2".
export interface SourceChoice {
    collectionId: number;
    savedSearchId: number;
    folderId: number;
}

const noSource: SourceChoice = {collectionId: 0, savedSearchId: 0, folderId: 0};

export function sourceKey(s: Partial<SourceChoice>): string {
    if (s.folderId) return `folder:${s.folderId}`;
    if (s.savedSearchId) return `search:${s.savedSearchId}`;
    if (s.collectionId) return `collection:${s.collectionId}`;
    return 'library';
}

export function parseSourceKey(key: string): SourceChoice {
    const [kind, raw] = key.split(':');
    const id = Number(raw);
    if (Number.isInteger(id) && id > 0) {
        if (kind === 'search') return {...noSource, savedSearchId: id};
        if (kind === 'collection') return {...noSource, collectionId: id};
        if (kind === 'folder') return {...noSource, folderId: id};
    }
    return {...noSource};
}

// Corners for text on the wallpaper, in the order offered.
export const corners: {value: string; label: string}[] = [
    {value: 'top-left', label: 'Top left'},
    {value: 'top-right', label: 'Top right'},
    {value: 'bottom-left', label: 'Bottom left'},
    {value: 'bottom-right', label: 'Bottom right'},
];

// Text sizes (the clock, the text file), in the order offered.
export const clockSizes: {value: string; label: string}[] = [
    {value: 'small', label: 'Small'},
    {value: 'medium', label: 'Medium'},
    {value: 'large', label: 'Large'},
];

// Strengths of an effect on the wallpaper, in the order offered.
export const effectLevels: {value: string; label: string}[] = [
    {value: 'off', label: 'Off'},
    {value: 'light', label: 'Light'},
    {value: 'medium', label: 'Medium'},
    {value: 'strong', label: 'Strong'},
];

// Colors of the wallpaper, in the order offered.
export const tones: {value: string; label: string}[] = [
    {value: 'original', label: 'Original'},
    {value: 'black-and-white', label: 'Black and white'},
    {value: 'sepia', label: 'Sepia'},
];
