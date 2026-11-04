// Hidden tags are compared case-insensitively, like in the Go code.
export function isTagHidden(hidden: string[], tag: string): boolean {
    const t = tag.toLowerCase();
    return hidden.some((h) => h.toLowerCase() === t);
}
