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

// Rotation source as one <select> value: "library", "collection:7" or
// "search:3".
export interface SourceChoice {
    collectionId: number;
    savedSearchId: number;
}

export function sourceKey(s: SourceChoice): string {
    if (s.savedSearchId) return `search:${s.savedSearchId}`;
    if (s.collectionId) return `collection:${s.collectionId}`;
    return 'library';
}

export function parseSourceKey(key: string): SourceChoice {
    const [kind, raw] = key.split(':');
    const id = Number(raw);
    if (Number.isInteger(id) && id > 0) {
        if (kind === 'search') return {collectionId: 0, savedSearchId: id};
        if (kind === 'collection') return {collectionId: id, savedSearchId: 0};
    }
    return {collectionId: 0, savedSearchId: 0};
}

// Corners for text on the wallpaper, in the order offered.
export const corners: {value: string; label: string}[] = [
    {value: 'top-left', label: 'Top left'},
    {value: 'top-right', label: 'Top right'},
    {value: 'bottom-left', label: 'Bottom left'},
    {value: 'bottom-right', label: 'Bottom right'},
];
