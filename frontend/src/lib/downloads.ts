// Download state for the UI, built from "download:progress" events.

import {BackendError, describeError} from './backend';

export const downloadProgressEvent = 'download:progress';

export type DownloadState =
    | 'queued'
    | 'downloading'
    | 'processing'
    | 'done'
    | 'duplicate'
    | 'failed'
    | 'canceled';

// DownloadProgress mirrors main.DownloadProgress.
export interface DownloadProgress {
    key: string;
    state: DownloadState;
    received: number;
    total: number;
    entryId?: number;
    similarIds?: number[];
    error?: BackendError;
}

export type Downloads = Record<string, DownloadProgress>;

export function isActive(p: DownloadProgress | undefined): boolean {
    return p?.state === 'queued' || p?.state === 'downloading' || p?.state === 'processing';
}

// applyProgress returns the download map with one event applied. Progress
// events do not repeat the expected size, so it is kept from earlier ones.
export function applyProgress(downloads: Downloads, p: DownloadProgress): Downloads {
    const previous = downloads[p.key];
    const total = p.total || previous?.total || 0;
    const received = p.state === 'processing' ? Math.max(p.received, previous?.received ?? 0) : p.received;
    return {...downloads, [p.key]: {...p, total, received}};
}

// percent returns the downloaded share from 0 to 100, or null when the size
// is unknown.
export function percent(p: DownloadProgress): number | null {
    if (p.state === 'processing') return 100;
    if (!p.total || p.total <= 0) return null;
    return Math.min(100, Math.floor((p.received / p.total) * 100));
}

// downloadLabel is the text of the download button.
export function downloadLabel(p: DownloadProgress | undefined, inLibrary: boolean): string {
    if (inLibrary || p?.state === 'done' || p?.state === 'duplicate') return 'In your library';
    switch (p?.state) {
        case 'queued':
            return 'Waiting…';
        case 'downloading': {
            const pct = percent(p);
            return pct === null ? 'Downloading…' : `Downloading ${pct}%`;
        }
        case 'processing':
            return 'Checking image…';
        case 'failed':
            return 'Try again';
    }
    return 'Download';
}

// downloadMessage explains the outcome of a finished download, if needed.
export function downloadMessage(p: DownloadProgress | undefined): string | null {
    if (!p) return null;
    if (p.state === 'failed' && p.error) return describeError(p.error);
    if (p.state === 'duplicate') return 'An identical file was already in your library.';
    const similar = p.similarIds?.length ?? 0;
    if (p.state === 'done' && similar > 0) {
        return similar === 1
            ? 'Saved. It looks like an image already in your library.'
            : `Saved. It looks like ${similar} images already in your library.`;
    }
    if (p.state === 'done') return 'Saved to your library.';
    return null;
}
