// Source selection and per-source paging for multi-source search.

import {describeError, isCanceled} from './backend';

export interface SourceInfo {
    id: string;
    name: string;
    contentWarning?: string;
    enabledByDefault: boolean;
    // categories is true for sources that apply the category filter.
    categories?: boolean;
}

// SourceError mirrors apperr.PublicError as sent inside a source status.
export interface SourceError {
    code: string;
    message: string;
    retryAfterSeconds?: number;
}

// SourceStatus mirrors main.SourceStatus.
export interface SourceStatus {
    providerId: string;
    name: string;
    page: number;
    hasMore: boolean;
    count: number;
    error?: SourceError;
}

export interface SourcePage {
    providerId: string;
    page: number;
}

// initialSources picks the sources selected at startup: the remembered ones
// that still exist, or the defaults when none are remembered. Sources with a
// content warning are never on by default.
export function initialSources(available: SourceInfo[], saved: string[] | null | undefined): string[] {
    const known = new Set(available.map((s) => s.id));
    const remembered = (saved ?? []).filter((id) => known.has(id));
    if (remembered.length > 0) return remembered;
    return available.filter((s) => s.enabledByDefault).map((s) => s.id);
}

// toggleSource adds or removes id, keeping the order of the available sources.
export function toggleSource(available: SourceInfo[], selected: string[], id: string): string[] {
    const next = new Set(selected);
    if (next.has(id)) next.delete(id);
    else next.add(id);
    return available.map((s) => s.id).filter((sid) => next.has(sid));
}

// nextPages lists what "Load more" asks for: the next page of every source
// that has more results, and the same page again for sources that failed
// (other than by cancellation).
export function nextPages(statuses: SourceStatus[]): SourcePage[] {
    const pages: SourcePage[] = [];
    for (const s of statuses) {
        if (s.error && !isCanceled(s.error)) {
            pages.push({providerId: s.providerId, page: s.page});
        } else if (!s.error && s.hasMore) {
            pages.push({providerId: s.providerId, page: s.page + 1});
        }
    }
    return pages;
}

// mergeStatuses applies the statuses of a "Load more" request to the
// current ones. Counts add up across pages; sources not asked again keep
// their previous status.
export function mergeStatuses(current: SourceStatus[], updates: SourceStatus[]): SourceStatus[] {
    const byID = new Map(updates.map((u) => [u.providerId, u]));
    return current.map((s) => {
        const u = byID.get(s.providerId);
        return u ? {...u, count: s.count + u.count} : s;
    });
}

// statusLabel is the short per-source summary shown under the search form.
export function statusLabel(s: SourceStatus): string {
    if (s.error) return `${s.name}: ${describeError(s.error)}`;
    const images = s.count === 1 ? '1 image' : `${s.count} images`;
    return `${s.name}: ${images}${s.hasMore ? '' : ' (all shown)'}`;
}
