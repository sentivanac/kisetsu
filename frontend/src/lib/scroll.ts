// Scrolling the shown tab with the keyboard.

export interface Scrollable {
    scrollTop: number;
    clientHeight: number;
    scrollHeight: number;
}

// keyScrollTarget returns the scroll position a key asks for, or null for
// other keys. A page leaves a little of the previous one visible.
export function keyScrollTarget(key: string, el: Scrollable): number | null {
    const end = Math.max(0, el.scrollHeight - el.clientHeight);
    const page = el.clientHeight * 0.9;
    switch (key) {
        case 'PageDown':
            return Math.min(end, el.scrollTop + page);
        case 'PageUp':
            return Math.max(0, el.scrollTop - page);
        case 'Home':
            return 0;
        case 'End':
            return end;
    }
    return null;
}
